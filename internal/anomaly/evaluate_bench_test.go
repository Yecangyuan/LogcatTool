package anomaly

import (
	"fmt"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func BenchmarkDetectorEvaluateManyKeys(b *testing.B) {
	enabled := true
	disabled := false
	d := NewDetector(Config{
		Enabled:             true,
		RecentWindowSec:     30,
		BaselineWindowSec:   300,
		Multiplier:          3.0,
		MinBaseline:         5,
		CooldownSec:         30,
		MaxKeysPerDimension: 1000,
		Dimensions: map[string]DimensionConfig{
			"global":  {Enabled: &enabled},
			"level":   {Enabled: &enabled},
			"tag":     {Enabled: &enabled},
			"pid":     {Enabled: &enabled},
			"package": {Enabled: &disabled},
			"process": {Enabled: &disabled},
		},
	})
	base := time.Unix(1_800_000_000, 0)
	for sec := 0; sec < 330; sec++ {
		ts := base.Add(time.Duration(sec) * time.Second)
		for key := 0; key < 1000; key++ {
			d.Record(&logentry.Entry{
				Timestamp: ts,
				PID:       10_000 + key,
				Level:     logentry.LevelInfo,
				Tag:       fmt.Sprintf("Tag%d", key),
			}, "", "")
		}
	}
	now := base.Add(330 * time.Second)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Evaluate(now)
	}
}
