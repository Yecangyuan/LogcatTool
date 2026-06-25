package anomaly

import (
	"math"
	"time"
)

// Strategy evaluates a time series and returns zero or more events.
type Strategy interface {
	Evaluate(series *TimeSeries, dim Dimension, key string, nowSecond int) []Event
}

// MovingAverageOptions configures the moving-average strategy.
type MovingAverageOptions struct {
	RecentWindowSec   int
	BaselineWindowSec int
	Multiplier        float64
	DropMultiplier    float64 // <= 0 disables drop detection
	MinBaseline       int
}

// NewMovingAverageStrategy creates the default strategy.
func NewMovingAverageStrategy(opts MovingAverageOptions) Strategy {
	return &movingAverageStrategy{opts: normalizeMovingAverageOptions(opts)}
}

// normalizeMovingAverageOptions fills in sane defaults without mutating the input.
func normalizeMovingAverageOptions(opts MovingAverageOptions) MovingAverageOptions {
	if opts.RecentWindowSec <= 0 {
		opts.RecentWindowSec = 30
	}
	if opts.BaselineWindowSec <= 0 {
		opts.BaselineWindowSec = 300
	}
	if opts.Multiplier <= 0 {
		opts.Multiplier = 3.0
	}
	if opts.MinBaseline < 0 {
		opts.MinBaseline = 0
	}
	return opts
}

type movingAverageStrategy struct {
	opts MovingAverageOptions
}

func (s *movingAverageStrategy) Evaluate(series *TimeSeries, dim Dimension, key string, nowSecond int) []Event {
	recentFrom := nowSecond - s.opts.RecentWindowSec + 1
	recentCount := series.Sum(recentFrom, nowSecond)
	recentRate := float64(recentCount) / float64(s.opts.RecentWindowSec)

	baselineTo := nowSecond - s.opts.RecentWindowSec
	baselineFrom := baselineTo - s.opts.BaselineWindowSec + 1
	baselineCount := series.Sum(baselineFrom, baselineTo)
	baselineRate := float64(baselineCount) / float64(s.opts.BaselineWindowSec)

	if baselineCount < s.opts.MinBaseline {
		return nil
	}

	var events []Event
	now := time.Now()
	logTime := time.Unix(int64(nowSecond), 0)

	if recentRate >= baselineRate*s.opts.Multiplier {
		ratio := recentRate / baselineRate
		if math.IsInf(ratio, 1) || math.IsNaN(ratio) {
			ratio = 0
		}
		events = append(events, Event{
			Dimension:    dim,
			Key:          key,
			Direction:    DirectionSpike,
			RecentRate:   recentRate,
			BaselineRate: baselineRate,
			Ratio:        ratio,
			TriggeredAt:  now,
			LogTime:      logTime,
		})
	}

	if s.opts.DropMultiplier > 0 && recentRate > 0 && recentRate <= baselineRate*s.opts.DropMultiplier {
		ratio := recentRate / baselineRate
		events = append(events, Event{
			Dimension:    dim,
			Key:          key,
			Direction:    DirectionDrop,
			RecentRate:   recentRate,
			BaselineRate: baselineRate,
			Ratio:        ratio,
			TriggeredAt:  now,
			LogTime:      logTime,
		})
	}

	return events
}
