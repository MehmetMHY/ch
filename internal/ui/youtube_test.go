package ui

import "testing"

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
