package anomaly

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestDetectorDetectsSpike(t *testing.T) {
	d := NewDetector(Config{
		Enabled:             true,
		RecentWindowSec:     1,
		BaselineWindowSec:   5,
		Multiplier:          3.0,
		MinBaseline:         1,
		CooldownSec:         0,
		MaxKeysPerDimension: 100,
	})
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		d.Record(&logentry.Entry{Timestamp: base.Add(time.Duration(i) * time.Second), Tag: "Net", Level: logentry.LevelInfo}, "", "")
	}
	spikeTime := base.Add(6 * time.Second)
	for i := 0; i < 20; i++ {
		d.Record(&logentry.Entry{Timestamp: spikeTime, Tag: "Net", Level: logentry.LevelInfo}, "", "")
	}

	events := d.Evaluate(spikeTime)
	found := false
	for _, e := range events {
		if e.Dimension == DimTag && e.Key == "Net" && e.Direction == DirectionSpike {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tag Net spike, got %v", events)
	}
}

func TestDetectorCooldown(t *testing.T) {
	d := NewDetector(Config{
		Enabled:             true,
		RecentWindowSec:     1,
		BaselineWindowSec:   5,
		Multiplier:          3.0,
		MinBaseline:         1,
		CooldownSec:         60,
		MaxKeysPerDimension: 100,
	})
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		d.Record(&logentry.Entry{Timestamp: base.Add(time.Duration(i) * time.Second), Tag: "Net", Level: logentry.LevelInfo}, "", "")
	}
	spikeTime := base.Add(6 * time.Second)
	for i := 0; i < 20; i++ {
		d.Record(&logentry.Entry{Timestamp: spikeTime, Tag: "Net", Level: logentry.LevelInfo}, "", "")
	}

	if len(d.Evaluate(spikeTime)) == 0 {
		t.Fatal("expected first event")
	}
	if len(d.Evaluate(spikeTime)) != 0 {
		t.Fatal("cooldown should suppress duplicate")
	}
}
