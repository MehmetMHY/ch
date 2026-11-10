package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScrapeYouTube(t *testing.T) {
	const fakeYtDlp = `#!/bin/sh
printf '%s\n' '---CALL---' >> "$YOUTUBE_TEST_ARGS_LOG"
metadata=0
output_base=""
while [ "$#" -gt 0 ]; do
    printf '%s\n' "$1" >> "$YOUTUBE_TEST_ARGS_LOG"
    case "$1" in
        -j) metadata=1 ;;
        -o)
            shift
            printf '%s\n' "$1" >> "$YOUTUBE_TEST_ARGS_LOG"
            output_base="${1%.%(ext)s}"
            ;;
    esac
    shift
done
if [ "$metadata" = 1 ]; then
    if [ "$YOUTUBE_TEST_MODE" = "metadata-error" ]; then
        printf '%s\n' 'ERROR: Sign in to confirm you are not a bot' >&2
        exit 1
    fi
    printf '%s\n' '{"title": "Test video", "duration": 903}'
    exit 0
fi
output="$output_base.en.srt"
case "$YOUTUBE_TEST_MODE" in
    rate-limit)
        printf '%s' 'partial download' > "$output.part"
        printf '%s\n' "ERROR: Unable to download video subtitles for 'en': HTTP Error 429: Too Many Requests" >&2
        exit 1
        ;;
    missing) exit 0 ;;
    empty) printf '\n' > "$output" ;;
    unreadable) mkdir "$output" ;;
    exit-only) exit 1 ;;
    success)
        printf '1\n00:00:00,160 --> 00:00:04,880\n>> Hello world.\n' > "$output"
        printf '1\n00:00:00,160 --> 00:00:04,880\nOriginal English variant.\n' > "$output_base.en-orig.srt"
        printf '%s' 'extra temporary file' > "$output.part"
        ;;
esac
`
	tests := []struct {
		mode    string
		want    string
		wantErr bool
	}{
		{"success", "00:00:00 - 00:00:04\n>> Hello world.", false},
		{"rate-limit", "HTTP Error 429: Too Many Requests", false},
		{"missing", "yt-dlp returned no English SRT subtitles", false},
		{"empty", "the English subtitle file is empty", false},
		{"unreadable", "failed to read subtitle file", false},
		{"exit-only", "exit status 1", false},
		{"metadata-error", "Sign in to confirm you are not a bot", true},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("YOUTUBE_TEST_MODE", tt.mode)
			t.Setenv("YOUTUBE_TEST_ARGS_LOG", filepath.Join(home, "yt-dlp-args.log"))
			binDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(binDir, "yt-dlp"), []byte(fakeYtDlp), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			terminal := &Terminal{}
			got, err := terminal.scrapeYouTube("https://www.youtube.com/watch?v=OHiKsF0JXPk")
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("expected error containing %q, got %v", tt.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, "title: Test video") || !strings.Contains(got, tt.want) {
				t.Fatalf("expected metadata and %q, got %q", tt.want, got)
			}
			if tt.mode != "success" && !strings.Contains(got, "Subtitles unavailable:") {
				t.Fatalf("missing subtitle failure diagnostic: %q", got)
			}
			if tt.mode == "success" && strings.Contains(got, "Original English variant.") {
				t.Fatalf("selected a lower-priority English subtitle variant: %q", got)
			}
			args, err := os.ReadFile(os.Getenv("YOUTUBE_TEST_ARGS_LOG"))
			if err != nil {
				t.Fatal(err)
			}
			for _, flag := range []string{"--js-runtimes\nnode\n", "--remote-components\nejs:github\n"} {
				if count := strings.Count(string(args), flag); count != 2 {
					t.Fatalf("expected %q in both yt-dlp calls, found %d times in:\n%s", flag, count, args)
				}
			}
			for _, flag := range []string{"--write-subs\n", "--write-auto-subs\n", "--sub-langs\nen.*,en\n", "--sub-format\nsrt\n"} {
				if !strings.Contains(string(args), flag) {
					t.Fatalf("subtitle invocation missing %q:\n%s", flag, args)
				}
			}
			entries, err := os.ReadDir(filepath.Join(home, ".ch", "tmp"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("subtitle downloads left temporary files: %v", entries)
			}
		})
	}
}

func TestSelectEnglishSubtitle(t *testing.T) {
	baseName := filepath.Join("tmp", "subtitles")
	tests := []struct {
		name    string
		matches []string
		want    string
	}{
		{name: "no matches"},
		{
			name: "exact English before original and regional variants",
			matches: []string{
				filepath.Join("tmp", "subtitles.en-US.srt"),
				filepath.Join("tmp", "subtitles.en-orig.srt"),
				filepath.Join("tmp", "subtitles.en.srt"),
			},
			want: filepath.Join("tmp", "subtitles.en.srt"),
		},
		{
			name: "original before regional English",
			matches: []string{
				filepath.Join("tmp", "subtitles.en-GB.srt"),
				filepath.Join("tmp", "subtitles.en-orig.srt"),
			},
			want: filepath.Join("tmp", "subtitles.en-orig.srt"),
		},
		{
			name: "stable regional fallback",
			matches: []string{
				filepath.Join("tmp", "subtitles.en-US.srt"),
				filepath.Join("tmp", "subtitles.en-GB.srt"),
			},
			want: filepath.Join("tmp", "subtitles.en-GB.srt"),
		},
		{
			name: "rejects non-English tracks",
			matches: []string{
				filepath.Join("tmp", "subtitles.fr.srt"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectEnglishSubtitle(baseName, tt.matches); got != tt.want {
				t.Fatalf("selectEnglishSubtitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompactSRT(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "single cue",
			in: "1\n" +
				"00:00:00,160 --> 00:00:04,880\n" +
				"We know what Uber's 2017 was like.\n",
			want: "00:00:00 - 00:00:04\n" +
				"We know what Uber's 2017 was like.\n",
		},
		{
			name: "drops cue indices and blank lines",
			in: "1\n" +
				"00:00:00,160 --> 00:00:04,880\n" +
				"first line\n" +
				"\n" +
				"2\n" +
				"00:00:04,880 --> 00:00:06,879\n" +
				"second line\n" +
				"\n",
			want: "00:00:00 - 00:00:04\n" +
				"first line\n" +
				"00:00:04 - 00:00:06\n" +
				"second line\n",
		},
		{
			name: "preserves >> speaker markers verbatim",
			in: "3\n" +
				"00:00:02,879 --> 00:00:06,879\n" +
				">> Travis has stepped down from his\n" +
				"\n" +
				"4\n" +
				"00:00:04,880 --> 00:00:10,240\n" +
				">> role as chief executive.\n",
			want: "00:00:02 - 00:00:06\n" +
				">> Travis has stepped down from his\n" +
				"00:00:04 - 00:00:10\n" +
				">> role as chief executive.\n",
		},
		{
			name: "multi-line subtitle text passes through",
			in: "5\n" +
				"00:00:10,240 --> 00:00:13,920\n" +
				"or Mark was on the board. You're in this\n" +
				"hell. You're dealing with the lawsuits.\n" +
				"\n",
			want: "00:00:10 - 00:00:13\n" +
				"or Mark was on the board. You're in this\n" +
				"hell. You're dealing with the lawsuits.\n",
		},
		{
			name: "timestamp without millis stays intact",
			in: "7\n" +
				"00:01:02 --> 00:01:05\n" +
				"no millis here\n",
			want: "00:01:02 - 00:01:05\n" +
				"no millis here\n",
		},
		{
			name: "trailing newline only is not echoed",
			in: "1\n" +
				"00:00:00,000 --> 00:00:01,000\n" +
				"hi\n" +
				"\n",
			want: "00:00:00 - 00:00:01\n" +
				"hi\n",
		},
		{
			name: "non-numeric text line that looks like a number is not dropped",
			in: "1\n" +
				"00:00:00,000 --> 00:00:01,000\n" +
				"chapter 12\n" +
				"\n",
			want: "00:00:00 - 00:00:01\n" +
				"chapter 12\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compactSRT(tt.in)
			if got != tt.want {
				t.Errorf("compactSRT mismatch\nwant: %q\ngot:  %q", tt.want, got)
			}
		})
	}
}

func TestCompactTimestampLine(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"00:00:00,160 --> 00:00:04,880", "00:00:00 - 00:00:04"},
		{"00:01:02,000 --> 00:01:05,500", "00:01:02 - 00:01:05"},
		{" 00:00:00,160 --> 00:00:04,880 ", "00:00:00 - 00:00:04"},
		{"00:01:02 --> 00:01:05", "00:01:02 - 00:01:05"},
	}

	for _, tt := range tests {
		got := compactTimestampLine(tt.in)
		if got != tt.want {
			t.Errorf("compactTimestampLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStripMillis(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"00:00:00,160", "00:00:00"},
		{" 00:00:00,160 ", "00:00:00"},
		{"00:01:02", "00:01:02"},
		{"  00:01:02  ", "00:01:02"},
		{"", ""},
	}

	for _, tt := range tests {
		got := stripMillis(tt.in)
		if got != tt.want {
			t.Errorf("stripMillis(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
