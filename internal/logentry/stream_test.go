package logentry

import (
	"io"
	"strings"
	"testing"
)

func drain(t *testing.T, r *StreamReader) []*Entry {
	t.Helper()
	var entries []*Entry
	for {
		entry, err := r.Next()
		if err == io.EOF {
			return entries
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		entries = append(entries, entry)
	}
}

func TestStreamReaderFoldsStackTraceAndKeepsTail(t *testing.T) {
	input := strings.Join([]string{
		"10-19 14:00:00.100  1234  1234 E AndroidRuntime: FATAL EXCEPTION: main",
		"    at com.example.app.Main.run(Main.java:42)",
		"10-19 14:00:00.200  1234  1234 I ActivityManager: done",
	}, "\n")

	// No trailing newline: the final entry must still be delivered.
	entries := drain(t, NewStreamReader(strings.NewReader(input)))
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if !strings.Contains(entries[0].Message, "at com.example.app.Main.run") {
		t.Fatalf("stack trace not folded: %q", entries[0].Message)
	}
	if entries[1].Message != "done" {
		t.Fatalf("tail entry message = %q, want %q", entries[1].Message, "done")
	}
}

// TestStreamReaderSurvivesOversizedLine is the regression guard for the old
// 1MB bufio.Scanner limit, which aborted the whole stream on one long line and
// looked like the device had disconnected.
func TestStreamReaderSurvivesOversizedLine(t *testing.T) {
	long := strings.Repeat("A", 2<<20) // 2 MiB single line, far over the old 1MB cap
	input := strings.Join([]string{
		"10-19 14:00:00.100  1234  1234 I Tag: before",
		"10-19 14:00:00.150  1234  1234 I Tag: " + long,
		"10-19 14:00:00.200  1234  1234 I Tag: after",
	}, "\n")

	entries := drain(t, NewStreamReader(strings.NewReader(input)))
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3: an oversized line must not end the stream", len(entries))
	}
	if entries[0].Message != "before" {
		t.Fatalf("first = %q", entries[0].Message)
	}
	if entries[2].Message != "after" {
		t.Fatalf("last = %q, want %q: stream lost after oversized line", entries[2].Message, "after")
	}
	if len(entries[1].Message) == 0 {
		t.Fatal("oversized entry message is empty")
	}
}

func TestStreamReaderHandlesCRLF(t *testing.T) {
	input := "10-19 14:00:00.100  1234  1234 I Tag: one\r\n10-19 14:00:00.200  1234  1234 I Tag: two\r\n"
	entries := drain(t, NewStreamReader(strings.NewReader(input)))
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if strings.HasSuffix(entries[0].Message, "\r") {
		t.Fatalf("carriage return not trimmed: %q", entries[0].Message)
	}
}

func TestStreamReaderClampsLineLength(t *testing.T) {
	long := strings.Repeat("B", maxLogLine+1024)
	input := "10-19 14:00:00.100  1234  1234 I Tag: " + long + "\n10-19 14:00:00.200  1234  1234 I Tag: tail\n"

	entries := drain(t, NewStreamReader(strings.NewReader(input)))
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	if len(entries[0].Message) > maxLogLine+64 {
		t.Fatalf("message kept %d bytes, want at most %d", len(entries[0].Message), maxLogLine+64)
	}
	if entries[1].Message != "tail" {
		t.Fatalf("stream misaligned after clamped line: %q", entries[1].Message)
	}
}
