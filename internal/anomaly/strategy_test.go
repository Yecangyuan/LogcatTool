package anomaly

import (
	"testing"
)

func TestMovingAverageSpike(t *testing.T) {
	s := NewMovingAverageStrategy(MovingAverageOptions{
		RecentWindowSec:   1,
		BaselineWindowSec: 5,
		Multiplier:        3.0,
		MinBaseline:       1,
	})
	series := NewTimeSeries(10)
	now := 10
	for sec := now - 5; sec <= now-2; sec++ {
		series.Add(sec, 2)
	}
	series.Add(now, 10)

	events := s.Evaluate(series, DimTag, "Net", now)
	if len(events) != 1 || events[0].Direction != DirectionSpike {
		t.Fatalf("want spike got %v", events)
	}
}

func TestMovingAverageDrop(t *testing.T) {
	s := NewMovingAverageStrategy(MovingAverageOptions{
		RecentWindowSec:   1,
		BaselineWindowSec: 5,
		Multiplier:        3.0,
		DropMultiplier:    0.2,
		MinBaseline:       5,
	})
	series := NewTimeSeries(10)
	now := 10
	for sec := now - 5; sec <= now-2; sec++ {
		series.Add(sec, 10)
	}
	series.Add(now, 1)

	events := s.Evaluate(series, DimTag, "Net", now)
	if len(events) != 1 || events[0].Direction != DirectionDrop {
		t.Fatalf("want drop got %v", events)
	}
}

func TestMovingAverageNoEventWhenBaselineTooLow(t *testing.T) {
	s := NewMovingAverageStrategy(MovingAverageOptions{
		RecentWindowSec:   1,
		BaselineWindowSec: 5,
		Multiplier:        3.0,
		MinBaseline:       5,
	})
	series := NewTimeSeries(10)
	series.Add(10, 100)

	events := s.Evaluate(series, DimGlobal, "global", 10)
	if len(events) != 0 {
		t.Fatalf("want no events got %d", len(events))
	}
}
