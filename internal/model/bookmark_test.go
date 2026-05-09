package model

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestPruneStaleBookmarksAfterBufferWrap(t *testing.T) {
	m := New(Options{BufferSize: 3})

	m.ingestEntries([]*logentry.Entry{
		testEntryWithTag(time.Unix(1, 0), "keep-a"),
		testEntryWithTag(time.Unix(2, 0), "drop"),
		testEntryWithTag(time.Unix(3, 0), "keep-b"),
	})
	m.bookmarks[0] = true
	m.bookmarks[1] = true
	m.bookmarks[2] = true

	m.ingestEntries([]*logentry.Entry{
		testEntryWithTag(time.Unix(4, 0), "new-a"),
		testEntryWithTag(time.Unix(5, 0), "new-b"),
	})

	if len(m.bookmarks) != 1 {
		t.Fatalf("bookmark count after wrap = %d, want 1", len(m.bookmarks))
	}
	if !m.bookmarks[2] {
		t.Fatal("bookmark for newest surviving entry should remain")
	}
	if m.bookmarks[0] || m.bookmarks[1] {
		t.Fatal("bookmarks for evicted entries should be pruned")
	}
}

func TestRefilterKeepsBookmarksForHiddenEntriesStillInBuffer(t *testing.T) {
	m := New(Options{BufferSize: 4})

	m.ingestEntries([]*logentry.Entry{
		testEntryWithTag(time.Unix(1, 0), "Visible"),
		testEntryWithTag(time.Unix(2, 0), "Hidden"),
		testEntryWithTag(time.Unix(3, 0), "Visible"),
	})
	m.bookmarks[1] = true

	m.filter.Tag = "Visible"
	m.refilter()

	if !m.bookmarks[1] {
		t.Fatal("bookmark for filtered-out but still buffered entry should remain")
	}
}

func testEntryWithTag(ts time.Time, tag string) *logentry.Entry {
	return &logentry.Entry{
		Timestamp: ts,
		PID:       1000,
		TID:       1000,
		Level:     logentry.LevelInfo,
		Tag:       tag,
		Message:   tag,
		Raw:       tag,
	}
}
