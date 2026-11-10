package ui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// youtubeCommandError preserves yt-dlp diagnostics instead of only an exit code.
func youtubeCommandError(err error, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, detail)
}

// selectEnglishSubtitle prefers the exact English track, then YouTube's
// original-English track, followed by other English variants. Sorting within
// each priority keeps selection stable when yt-dlp returns multiple tracks.
func selectEnglishSubtitle(baseName string, matches []string) string {
	if len(matches) == 0 {
		return ""
	}

	ordered := append([]string(nil), matches...)
	priority := func(path string) int {
		name := filepath.Base(path)
		name = strings.TrimPrefix(name, filepath.Base(baseName)+".")
		language := strings.TrimSuffix(name, filepath.Ext(name))
		language = strings.ToLower(language)
		switch {
		case language == "en":
			return 0
		case language == "en-orig":
			return 1
		case strings.HasPrefix(language, "en-") || strings.HasPrefix(language, "en_"):
			return 2
		case strings.HasPrefix(language, "en"):
			return 3
		default:
			return 4
		}
	}
	sort.Slice(ordered, func(i, j int) bool {
		left, right := priority(ordered[i]), priority(ordered[j])
		if left != right {
			return left < right
		}
		return ordered[i] < ordered[j]
	})
	if priority(ordered[0]) == 4 {
		return ""
	}

	return ordered[0]
}

// compactSRT strips yt-dlp's SRT chrome to cut token usage without altering the
// transcript text itself. Per cue it:
//
//  1. Drops the cue index (a line that is a pure integer).
//  2. Drops the millisecond portion of each timestamp (everything after the comma)
//     and shortens " --> " to " - ".
//  3. Drops blank lines.
//  4. Passes every other line through verbatim, including ">>" speaker markers.
//
// The goal is robustness over cleverness: no merging, no guessing at sentence
// boundaries, no assumptions about ">>". Only pure waste is removed, so the
// output stays valid for auto-subs and manual subs alike.
func compactSRT(srt string) string {
	var out strings.Builder
	out.Grow(len(srt))

	for _, line := range strings.Split(srt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Cue index: a line that is just a number.
		if _, err := strconv.Atoi(line); err == nil {
			continue
		}

		// Timestamp line: "00:00:00,160 --> 00:00:04,880"
		// -> "00:00:00 - 00:00:04" (drop millis, shorten arrow).
		if strings.Contains(line, "-->") {
			out.WriteString(compactTimestampLine(line))
			out.WriteByte('\n')
			continue
		}

		// Any other line (subtitle text, ">>" speaker markers, etc.): verbatim.
		out.WriteString(line)
		out.WriteByte('\n')
	}

	return out.String()
}

// compactTimestampLine turns "00:00:00,160 --> 00:00:04,880" into
// "00:00:00 - 00:00:04". It drops the milliseconds from each side and replaces
// the " --> " separator with " - ".
func compactTimestampLine(line string) string {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return line
	}

	left := stripMillis(parts[0])
	right := stripMillis(parts[1])
	return left + " - " + right
}

// stripMillis trims surrounding whitespace and removes anything after a comma,
// turning " 00:00:00,160 " into "00:00:00". If there is no comma, the trimmed
// value is returned unchanged.
func stripMillis(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ','); i >= 0 {
		return s[:i]
	}
	return s
}
