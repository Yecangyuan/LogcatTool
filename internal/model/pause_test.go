package model

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

func TestPauseBindingMatchesSpace(t *testing.T) {
	msg := tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "})
	if !key.Matches(msg, DefaultKeyMap().Pause) {
		t.Fatal("space key should match pause binding")
	}
}

func TestPauseBuffersAndFlushesEntries(t *testing.T) {
	m := New(Options{BufferSize: 8})

	msg := tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "})
	model, _ := m.Update(msg)
	m = model.(AppModel)
	if !m.paused {
		t.Fatal("model should enter paused state")
	}

	entries := []*logentry.Entry{
		{Timestamp: time.Now(), PID: 1001, TID: 1001, Level: logentry.LevelInfo, Tag: "Tag", Message: "one", Raw: "one"},
		{Timestamp: time.Now(), PID: 1002, TID: 1002, Level: logentry.LevelWarn, Tag: "Tag", Message: "two", Raw: "two"},
	}

	model, _ = m.Update(LogEntriesMsg(entries))
	m = model.(AppModel)
	if got := len(m.pausedBuffer); got != 2 {
		t.Fatalf("paused buffer len = %d, want 2", got)
	}
	if m.totalCount != 0 {
		t.Fatalf("totalCount while paused = %d, want 0", m.totalCount)
	}

	model, _ = m.Update(msg)
	m = model.(AppModel)
	if m.paused {
		t.Fatal("model should resume after second space")
	}
	if got := len(m.pausedBuffer); got != 0 {
		t.Fatalf("paused buffer len after resume = %d, want 0", got)
	}
	if m.totalCount != 2 {
		t.Fatalf("totalCount after resume = %d, want 2", m.totalCount)
	}
	if m.filteredCount != 2 {
		t.Fatalf("filteredCount after resume = %d, want 2", m.filteredCount)
	}
}
