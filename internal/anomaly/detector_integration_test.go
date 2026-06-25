package anomaly

import (
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestDetectorSimulatedStream(t *testing.T) {
	d := NewDetector(Config{
		Enabled:             true,
		RecentWindowSec:     2,
		BaselineWindowSec:   10,
		Multiplier:          3.0,
		MinBaseline:         5,
		CooldownSec:         0,
		MaxKeysPerDimension: 100,
	})
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 10 seconds of baseline at 10 logs/sec
	for sec := 0; sec < 10; sec++ {
		for i := 0; i < 10; i++ {
			d.Record(
				&logentry.Entry{Timestamp: base.Add(time.Duration(sec) * time.Second), Tag: "Net", Level: logentry.LevelInfo},
				"", "",
			)
		}
	}
	// spike at second 10: 100 logs in one second
	spike := base.Add(10 * time.Second)
	for i := 0; i < 100; i++ {
		d.Record(&logentry.Entry{Timestamp: spike, Tag: "Net", Level: logentry.LevelInfo}, "", "",
		)
	}

	events := d.Evaluate(spike)
	if len(events) == 0 {
		t.Fatal("expected anomaly events")
	}
	found := false
	for _, e := range events {
		if e.Dimension == DimTag && e.Key == "Net" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tag Net anomaly, got %v", events)
	}
}

func BenchmarkDetectorHighThroughput(b *testing.B) {
	d := NewDetector(Config{
		Enabled:             true,
		RecentWindowSec:     30,
		BaselineWindowSec:   300,
		Multiplier:          3.0,
		MinBaseline:         5,
		CooldownSec:         30,
		MaxKeysPerDimension: 1000,
	})
	now := time.Now()
	entry := &logentry.Entry{Timestamp: now, Tag: "Net", Level: logentry.LevelInfo}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Record(entry, "com.example", "com.example:remote")
	}
}
