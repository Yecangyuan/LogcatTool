package config

import (
	"encoding/json"
	"testing"
)

func TestAnomalyConfigRoundTrip(t *testing.T) {
	cfg := Config{
		Anomaly: AnomalyConfig{
			Enabled:           true,
			RecentWindowSec:   30,
			BaselineWindowSec: 300,
			Multiplier:        3.0,
			MinBaseline:       5,
			Dimensions: map[string]DimensionConfig{
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

func boolPtr(b bool) *bool           { return &b }
func floatPtr(f float64) *float64 { return &f }
