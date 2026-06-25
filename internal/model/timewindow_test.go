package model

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestIngestEntriesIncrementalTimeWindowMatchesFullRefilter(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	seed := []*logentry.Entry{
		testWindowEntry(base.Add(0*time.Second), "NetworkManager", "old network"),
		testWindowEntry(base.Add(1*time.Second), "ActivityManager", "old activity"),
		testWindowEntry(base.Add(2*time.Second), "NetworkManager", "kept network"),
		testWindowEntry(base.Add(3*time.Second), "NetworkManager", "kept network 2"),
	}
	batch := []*logentry.Entry{
		testWindowEntry(base.Add(4*time.Second), "ActivityManager", "new activity"),
		testWindowEntry(base.Add(5*time.Second), "NetworkManager", "new network"),
		testWindowEntry(base.Add(6*time.Second), "NetworkManager", "new network 2"),
	}

	incremental := New(Options{BufferSize: 16})
	defer incremental.anomalyDetector.Stop()
	oracle := New(Options{BufferSize: 16})
	defer oracle.anomalyDetector.Stop()

	for _, m := range []*AppModel{&incremental, &oracle} {
		m.ingestEntries(cloneEntries(seed))
		m.filter.TimeWindow = 3 * time.Second
		m.filter.Tag = "NetworkManager"
		m.refilter()
	}

	incremental.ingestEntriesIncrementalTimeWindow(cloneEntries(batch))
	oracle.ingestEntries(cloneEntries(batch))
	oracle.refilter()

	got := entryMessages(incremental.filtered)
	want := entryMessages(oracle.filtered)
	if len(got) != len(want) {
		t.Fatalf("filtered len = %d, want %d; got %v want %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("filtered[%d] = %q, want %q; got %v want %v", i, got[i], want[i], got, want)
		}
	}
	if incremental.filteredCount != oracle.filteredCount {
		t.Fatalf("filteredCount = %d, want %d", incremental.filteredCount, oracle.filteredCount)
	}
	if incremental.displayCount != oracle.displayCount {
		t.Fatalf("displayCount = %d, want %d", incremental.displayCount, oracle.displayCount)
	}
}

func testWindowEntry(ts time.Time, tag, msg string) *logentry.Entry {
	return &logentry.Entry{
		Timestamp: ts,
		PID:       1000,
		TID:       1001,
		Level:     logentry.LevelInfo,
		Tag:       tag,
		Message:   msg,
		Raw:       tag + ": " + msg,
	}
}

func cloneEntries(entries []*logentry.Entry) []*logentry.Entry {
	out := make([]*logentry.Entry, len(entries))
	for i, entry := range entries {
		cp := *entry
		out[i] = &cp
	}
	return out
}

func entryMessages(entries []*logentry.Entry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = entry.Message
	}
	return out
}
