package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadPreservesExplicitlyDisabledAnomaly covers the regression where an
// explicit `"enabled": false` was overwritten by the default (true) because the
// merge could not tell "false on purpose" from "not configured".
func TestLoadPreservesExplicitlyDisabledAnomaly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "logcatool")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `{"anomaly": {"enabled": false}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Anomaly.Enabled {
		t.Fatal("explicit enabled:false was overridden back to true")
	}
	// Other omitted keys still receive their defaults.
	if loaded.Anomaly.Multiplier != 3.0 {
		t.Fatalf("Multiplier = %v, want default 3.0", loaded.Anomaly.Multiplier)
	}
	if loaded.Anomaly.RecentWindowSec != 30 {
		t.Fatalf("RecentWindowSec = %v, want default 30", loaded.Anomaly.RecentWindowSec)
	}
}

func TestLoadPreservesExplicitZeroDropMultiplier(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "logcatool")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A non-default drop multiplier proves explicit values survive merging.
	content := `{"anomaly": {"enabled": true, "drop_multiplier": 0.5}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Anomaly.DropMultiplier != 0.5 {
		t.Fatalf("DropMultiplier = %v, want 0.5", loaded.Anomaly.DropMultiplier)
	}
}

func TestLoadAppliesDefaultsWhenAnomalyMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".config", "logcatool")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.Anomaly.Enabled {
		t.Fatal("missing anomaly section must default to enabled")
	}
}

// TestSaveLeavesNoTempFiles guards the atomic-write path: temp files must be
// renamed into place, never left behind in the config directory.
func TestSaveLeavesNoTempFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save(Config{Anomaly: DefaultAnomalyConfig()}); err != nil {
		t.Fatalf("save: %v", err)
	}

	dir := filepath.Join(home, ".config", "logcatool")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "\"enabled\"") {
		t.Fatalf("config content missing enabled field: %s", data)
	}
}

func TestSaveOverwriteIsComplete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	first := Config{AlertKeyword: strings.Repeat("k", 4096), Anomaly: DefaultAnomalyConfig()}
	if err := Save(first); err != nil {
		t.Fatalf("first save: %v", err)
	}
	second := Config{AlertKeyword: "short", Anomaly: DefaultAnomalyConfig()}
	if err := Save(second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.AlertKeyword != "short" {
		t.Fatalf("AlertKeyword = %q, want %q (stale bytes left by in-place write)", loaded.AlertKeyword, "short")
	}
}
