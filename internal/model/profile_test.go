package model

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/config"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"

	tea "charm.land/bubbletea/v2"
)

func TestProfileSnapshotConversion(t *testing.T) {
	snapshot := logentry.Snapshot{
		MinLevel:   logentry.LevelWarn,
		Package:    "com.example.app",
		Process:    "com.example.app:remote",
		Tag:        "Network",
		TagExclude: "Noise",
		PID:        1234,
		SearchText: "timeout",
		IsRegex:    true,
		CrashOnly:  true,
		TimeWindow: 2 * time.Minute,
	}

	profile := profileFromSnapshot("network errors", snapshot)
	if profile.Name != "network errors" || profile.MinLevel != "W" || profile.SearchText != "timeout" || profile.TimeWindowSec != 120 {
		t.Fatalf("profile mismatch: %#v", profile)
	}

	roundTrip := snapshotFromProfile(profile)
	if roundTrip != snapshot {
		t.Fatalf("snapshot roundtrip = %#v, want %#v", roundTrip, snapshot)
	}
}

func TestProfileModelSaveApplyRenameDelete(t *testing.T) {
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.filter.SetMinLevel(logentry.LevelError)
	m.filter.Tag = "Network"
	m.filter.SearchText = "timeout"
	m.filter.IsRegex = true
	m.filter.CrashOnly = true
	m.filter.TimeWindow = time.Minute

	m.saveProfile("network errors")
	if len(m.cfg.Profiles) != 1 {
		t.Fatalf("profile count = %d, want 1", len(m.cfg.Profiles))
	}
	if got := m.cfg.Profiles[0].Name; got != "network errors" {
		t.Fatalf("profile name = %q", got)
	}

	m.filter = logentry.NewFilter()
	m.applyProfile(0)
	if m.filter.MinLevel != logentry.LevelError || m.filter.Tag != "Network" || m.filter.SearchText != "timeout" || !m.filter.IsRegex || !m.filter.CrashOnly || m.filter.TimeWindow != time.Minute {
		t.Fatalf("applied filter mismatch: %#v", m.filter.Snapshot())
	}

	m.renameProfile(0, "renamed")
	if got := m.cfg.Profiles[0].Name; got != "renamed" {
		t.Fatalf("renamed profile = %q", got)
	}

	m.deleteProfile(0)
	if len(m.cfg.Profiles) != 0 {
		t.Fatalf("profile count after delete = %d, want 0", len(m.cfg.Profiles))
	}
}

func TestNewRestoresConfiguredProfiles(t *testing.T) {
	cfg := config.Config{
		Profiles: []config.Profile{{Name: "stored", MinLevel: "E", Tag: "AndroidRuntime"}},
		Anomaly:  config.DefaultAnomalyConfig(),
	}
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.cfg = cfg

	m.applyProfile(0)
	if m.filter.MinLevel != logentry.LevelError || m.filter.Tag != "AndroidRuntime" {
		t.Fatalf("stored profile not applied: %#v", m.filter.Snapshot())
	}
}

func TestProfilePanelSaveAndApplyFlow(t *testing.T) {
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.filter.Tag = "Network"
	m.filter.SetMinLevel(logentry.LevelWarn)

	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'U', Text: "U"}))
	m = model.(AppModel)
	if m.inputMode != ModeProfilePanel {
		t.Fatalf("inputMode = %v, want %v", m.inputMode, ModeProfilePanel)
	}

	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 's', Text: "s"}))
	m = model.(AppModel)
	if m.inputMode != ModeProfileName {
		t.Fatalf("inputMode = %v, want %v", m.inputMode, ModeProfileName)
	}
	for _, r := range "net" {
		model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
		m = model.(AppModel)
	}
	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(AppModel)
	if len(m.cfg.Profiles) != 1 || m.cfg.Profiles[0].Name != "net" {
		t.Fatalf("profiles = %#v, want saved net profile", m.cfg.Profiles)
	}
	if m.inputMode != ModeProfilePanel {
		t.Fatalf("inputMode after save = %v, want profile panel", m.inputMode)
	}

	m.filter = logentry.NewFilter()
	model, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	m = model.(AppModel)
	if m.filter.Tag != "Network" || m.filter.MinLevel != logentry.LevelWarn {
		t.Fatalf("applied filter = %#v, want saved profile", m.filter.Snapshot())
	}
}
