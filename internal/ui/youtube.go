package ui

import (
	"strconv"
	"strings"
)

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
