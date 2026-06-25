package model

import (
	"regexp"
	"testing"
)

var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestTruncateToWidthPreservesANSISequences(t *testing.T) {
	got := truncateToWidth("\x1b[31mabcdef\x1b[0m", 3)
	visible := sgrPattern.ReplaceAllString(got, "")
	if visible != "abc" {
		t.Fatalf("visible truncated text = %q, want %q; raw=%q", visible, "abc", got)
	}
	if got != "\x1b[31mabc\x1b[0m" {
		t.Fatalf("truncated ANSI output = %q, want complete SGR-wrapped text", got)
	}
}
