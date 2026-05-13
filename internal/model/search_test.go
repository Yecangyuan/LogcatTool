package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSearchInputAcceptsPrintableKeysWithoutText(t *testing.T) {
	m := New(Options{BufferSize: 8})

	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	m = model.(AppModel)
	if m.inputMode != ModeSearch {
		t.Fatalf("inputMode = %v, want %v", m.inputMode, ModeSearch)
	}

	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'h'}))
	m = model.(AppModel)
	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'i'}))
	m = model.(AppModel)

	if got := m.filterInput.Value(); got != "hi" {
		t.Fatalf("search input value = %q, want %q", got, "hi")
	}

	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(AppModel)

	if m.inputMode != ModeNormal {
		t.Fatalf("inputMode after confirm = %v, want %v", m.inputMode, ModeNormal)
	}
	if got := m.filter.SearchText; got != "hi" {
		t.Fatalf("search text = %q, want %q", got, "hi")
	}
	if m.filter.SearchRe == nil {
		t.Fatal("search regex should be compiled for valid input")
	}
}

func TestNormalizeTextInputKeyHandlesShiftedRuneAndSpace(t *testing.T) {
	shifted := normalizeTextInputKey(tea.KeyPressMsg(tea.Key{Code: 'a', ShiftedCode: 'A'}))
	if shifted.Key().Text != "A" {
		t.Fatalf("shifted key text = %q, want %q", shifted.Key().Text, "A")
	}

	space := normalizeTextInputKey(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	if space.Key().Text != " " {
		t.Fatalf("space key text = %q, want %q", space.Key().Text, " ")
	}
}
