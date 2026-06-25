package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestReplaySourceEmitsEntriesInTimestampOrder(t *testing.T) {
	path := writeReplayLog(t, []string{
		"06-25 10:00:00.020  1000  1000 E Later: later",
		"06-25 10:00:00.000  1000  1000 I First: first",
		"06-25 10:00:00.010  1000  1000 W Middle: middle",
	})
	src := NewReplaySource(path, 1000, WithReplaySleep(func(time.Duration) bool { return true }))

	entries, errc := src.Start()
	got := collectReplayMessages(t, entries, errc)
	want := []string{"first", "middle", "later"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("messages = %v, want %v", got, want)
	}
}

func TestReplaySourcePauseResumeBeforeEmission(t *testing.T) {
	path := writeReplayLog(t, []string{
		"06-25 10:00:00.000  1000  1000 I First: first",
	})
	src := NewReplaySource(path, 1)
	src.Pause()

	entries, _ := src.Start()
	select {
	case entry := <-entries:
		t.Fatalf("entry emitted while paused: %#v", entry)
	case <-time.After(20 * time.Millisecond):
	}

	src.Resume()
	select {
	case entry := <-entries:
		if entry == nil || entry.Message != "first" {
			t.Fatalf("entry after resume = %#v, want first", entry)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for entry after resume")
	}
}

func TestReplaySourceSpeedChangesAffectSubsequentDelays(t *testing.T) {
	path := writeReplayLog(t, []string{
		"06-25 10:00:00.000  1000  1000 I First: first",
		"06-25 10:00:00.100  1000  1000 I Second: second",
		"06-25 10:00:00.300  1000  1000 I Third: third",
	})

	var src *ReplaySource
	var delays []time.Duration
	src = NewReplaySource(path, 2, WithReplaySleep(func(d time.Duration) bool {
		delays = append(delays, d)
		if len(delays) == 1 {
			src.SetSpeed(4)
		}
		return true
	}))

	entries, errc := src.Start()
	_ = collectReplayMessages(t, entries, errc)
	want := []time.Duration{50 * time.Millisecond, 50 * time.Millisecond}
	if len(delays) != len(want) {
		t.Fatalf("delay count = %d, want %d (%v)", len(delays), len(want), delays)
	}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delay[%d] = %s, want %s", i, delays[i], want[i])
		}
	}
}

func collectReplayMessages(t *testing.T, entries <-chan *logentry.Entry, errc <-chan error) []string {
	t.Helper()
	var got []string
	for entry := range entries {
		got = append(got, entry.Message)
	}
	for err := range errc {
		if err != nil {
			t.Fatalf("unexpected replay error: %v", err)
		}
	}
	return got
}

func writeReplayLog(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "logcat.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
