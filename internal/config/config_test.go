package config

import (
	"encoding/json"
	"testing"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
)

func TestAnomalyConfigRoundTrip(t *testing.T) {
	cfg := Config{
		Anomaly: AnomalyConfig{
			Enabled:           true,
			RecentWindowSec:   30,
			BaselineWindowSec: 300,
			Multiplier:        3.0,
			MinBaseline:       5,
			Dimensions: map[string]anomaly.DimensionConfig{
				"tag": {Enabled: boolPtr(true), Multiplier: floatPtr(2.5)},
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !decoded.Anomaly.Enabled {
		t.Fatal("enabled lost")
	}
	if decoded.Anomaly.Multiplier != 3.0 {
		t.Fatalf("multiplier want 3.0 got %v", decoded.Anomaly.Multiplier)
	}
	tag := decoded.Anomaly.Dimensions["tag"]
	if tag.Multiplier == nil || *tag.Multiplier != 2.5 {
		t.Fatalf("tag multiplier want 2.5 got %v", tag.Multiplier)
	}
}

func TestDefaultAnomalyConfig(t *testing.T) {
	cfg := DefaultAnomalyConfig()

	if !cfg.Enabled {
		t.Fatal("expected Enabled == true")
	}
	if cfg.Multiplier != 3.0 {
		t.Fatalf("expected Multiplier == 3.0, got %v", cfg.Multiplier)
	}

	global, ok := cfg.Dimensions["global"]
	if !ok {
		t.Fatal("missing global dimension")
	}
	if global.Enabled == nil || *global.Enabled != true {
		t.Fatalf("expected global.Enabled == true, got %v", global.Enabled)
	}

	level, ok := cfg.Dimensions["level"]
	if !ok {
		t.Fatal("missing level dimension")
	}
	if level.Enabled == nil || *level.Enabled != true {
		t.Fatalf("expected level.Enabled == true, got %v", level.Enabled)
	}
	if level.Multiplier == nil || *level.Multiplier != 2.0 {
		t.Fatalf("expected level.Multiplier == 2.0, got %v", level.Multiplier)
	}

	tag, ok := cfg.Dimensions["tag"]
	if !ok {
		t.Fatal("missing tag dimension")
	}
	if tag.Enabled == nil || *tag.Enabled != true {
		t.Fatalf("expected tag.Enabled == true, got %v", tag.Enabled)
	}

	pid, ok := cfg.Dimensions["pid"]
	if !ok {
		t.Fatal("missing pid dimension")
	}
	if pid.Enabled == nil || *pid.Enabled != true {
		t.Fatalf("expected pid.Enabled == true, got %v", pid.Enabled)
	}

	packageDim, ok := cfg.Dimensions["package"]
	if !ok {
		t.Fatal("missing package dimension")
	}
	if packageDim.Enabled == nil || *packageDim.Enabled != true {
		t.Fatalf("expected package.Enabled == true, got %v", packageDim.Enabled)
	}

	process, ok := cfg.Dimensions["process"]
	if !ok {
		t.Fatal("missing process dimension")
	}
	if process.Enabled == nil || *process.Enabled != false {
		t.Fatalf("expected process.Enabled == false, got %v", process.Enabled)
	}
}

func TestDefaultAnomalyConfigRoundTrip(t *testing.T) {
	cfg := Config{Anomaly: DefaultAnomalyConfig()}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !decoded.Anomaly.Enabled {
		t.Fatal("enabled lost")
	}
	if decoded.Anomaly.Multiplier != 3.0 {
		t.Fatalf("multiplier want 3.0 got %v", decoded.Anomaly.Multiplier)
	}

	global := decoded.Anomaly.Dimensions["global"]
	if global.Enabled == nil || *global.Enabled != true {
		t.Fatalf("global enabled want true got %v", global.Enabled)
	}

	level := decoded.Anomaly.Dimensions["level"]
	if level.Enabled == nil || *level.Enabled != true {
		t.Fatalf("level enabled want true got %v", level.Enabled)
	}
	if level.Multiplier == nil || *level.Multiplier != 2.0 {
		t.Fatalf("level multiplier want 2.0 got %v", level.Multiplier)
	}

	process := decoded.Anomaly.Dimensions["process"]
	if process.Enabled == nil || *process.Enabled != false {
		t.Fatalf("process enabled want false got %v", process.Enabled)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := Config{Anomaly: DefaultAnomalyConfig()}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if !loaded.Anomaly.Enabled {
		t.Fatal("Anomaly.Enabled lost")
	}
	if loaded.Anomaly.Multiplier != 3.0 {
		t.Fatalf("Anomaly.Multiplier want 3.0 got %v", loaded.Anomaly.Multiplier)
	}

	level := loaded.Anomaly.Dimensions["level"]
	if level.Multiplier == nil || *level.Multiplier != 2.0 {
		t.Fatalf("level multiplier want 2.0 got %v", level.Multiplier)
	}
}
