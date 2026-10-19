package logentry

import (
	"strings"
	"testing"
	"time"
)

func TestParserFoldsMultiLineStackTraces(t *testing.T) {
	lines := []string{
		"10-19 14:00:00.100  1234  1234 E AndroidRuntime: FATAL EXCEPTION: main",
		"    at com.example.app.Main.run(Main.java:42)",
		"    at android.os.Handler.handleCallback(Handler.java:938)",
		"Caused by: java.lang.NullPointerException",
		"    at com.example.app.Repo.load(Repo.java:12)",
		"10-19 14:00:00.200  1234  1234 I ActivityManager: next entry",
	}

	var p Parser
	var entries []*Entry
	for _, line := range lines {
		if e := p.ParseLine(line); e != nil {
			entries = append(entries, e)
		}
	}
	if last := p.Flush(); last != nil {
		entries = append(entries, last)
	}

	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2 (continuation lines must not become entries)", len(entries))
	}

	crash := entries[0]
	for _, fragment := range []string{
		"at com.example.app.Main.run(Main.java:42)",
		"at android.os.Handler.handleCallback(Handler.java:938)",
		"Caused by: java.lang.NullPointerException",
		"at com.example.app.Repo.load(Repo.java:12)",
	} {
		if !strings.Contains(crash.Message, fragment) {
			t.Fatalf("message missing continuation %q; got:\n%s", fragment, crash.Message)
		}
	}
	if !strings.Contains(crash.Message, "FATAL EXCEPTION: main") {
		t.Fatalf("message lost its header text; got:\n%s", crash.Message)
	}

	// The trailing header must not absorb the previous stack.
	if entries[1].Message != "next entry" {
		t.Fatalf("second entry message = %q, want %q", entries[1].Message, "next entry")
	}
}

func TestParserCapsContinuationGrowth(t *testing.T) {
	var p Parser
	if e := p.ParseLine("10-19 14:00:00.100  1234  1234 E Tag: head"); e != nil {
		t.Fatalf("first header returned %v, want nil", e)
	}
	huge := strings.Repeat("x", 4096)
	for i := 0; i < 64; i++ {
		p.ParseLine(huge)
	}
	entry := p.Flush()
	if entry == nil {
		t.Fatal("flush returned nil")
	}
	if len(entry.Message) > maxContinuationBytes {
		t.Fatalf("message grew to %d bytes, cap is %d", len(entry.Message), maxContinuationBytes)
	}
}

func TestParseRejectsNonHeaderLines(t *testing.T) {
	for _, line := range []string{
		"    at com.example.app.Main.run(Main.java:42)",
		"not a log line at all",
		"--------- beginning of main",
		"",
	} {
		if entry := Parse(line); entry != nil {
			t.Fatalf("Parse(%q) = %#v, want nil", line, entry)
		}
	}
}

func TestParseAcceptsThreadtimeHeader(t *testing.T) {
	entry := Parse("10-19 14:00:00.123  1234  5678 W MyTag: hello world")
	if entry == nil {
		t.Fatal("Parse returned nil for a valid header")
	}
	if entry.PID != 1234 || entry.TID != 5678 {
		t.Fatalf("pid/tid = %d/%d, want 1234/5678", entry.PID, entry.TID)
	}
	if entry.Level != LevelWarn {
		t.Fatalf("level = %v, want %v", entry.Level, LevelWarn)
	}
	if entry.Tag != "MyTag" || entry.Message != "hello world" {
		t.Fatalf("tag/message = %q/%q", entry.Tag, entry.Message)
	}
}

// TestParserRollsBackYear covers the New Year boundary: logcat prints no year,
// so a stamp that would land more than a day in the future belongs to the
// previous year.
func TestParserRollsBackYear(t *testing.T) {
	future := time.Now().Add(48 * time.Hour)

	var p Parser
	entry := p.ParseLine(future.Format("01-02") + " 03:04:05.678  111  222 I Tag: msg")
	if entry != nil {
		t.Fatalf("first header returned %v, want nil", entry)
	}
	flushed := p.Flush()
	if flushed == nil {
		t.Fatal("flush returned nil")
	}

	want := future.Year() - 1
	if flushed.Timestamp.Year() != want {
		t.Fatalf("year = %d, want %d (stamp %s must not be in the future)",
			flushed.Timestamp.Year(), want, future.Format("01-02"))
	}
}

func TestParserKeepsCurrentYearForRecentStamps(t *testing.T) {
	now := time.Now()

	var p Parser
	p.ParseLine(now.Format("01-02") + " 00:00:01.000  111  222 I Tag: msg")
	entry := p.Flush()
	if entry == nil {
		t.Fatal("flush returned nil")
	}
	if entry.Timestamp.Year() != now.Year() {
		t.Fatalf("year = %d, want %d", entry.Timestamp.Year(), now.Year())
	}
}
