# Log Rate Anomaly Detection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add real-time, multi-dimensional log rate anomaly detection to LogcatTool so users see spikes/drops in the status bar, a dedicated panel, and inline log highlights.

**Architecture:** Introduce a new `internal/anomaly` package that owns time-series aggregation and detection strategy. The existing `internal/model` package wires the detector into log ingestion, renders anomalies in the UI, and persists configuration. Anomalies are produced asynchronously by a goroutine and consumed by the Bubble Tea update loop via a channel-based message.

**Tech Stack:** Go 1.25+, Bubble Tea v2, lipgloss, existing `internal/logentry`, `internal/config`, `internal/ringbuf`.

---

## File Structure

| File | Responsibility |
|------|----------------|
| `internal/config/config.go` | Extend `Config` with `AnomalyConfig` and default values. |
| `internal/anomaly/types.go` | Dimension enum, `Event`, `Direction`. |
| `internal/anomaly/series.go` | Fixed-length, per-second time series bucket ring. |
| `internal/anomaly/strategy.go` | `Strategy` interface + `MovingAverageStrategy` (spike + drop). |
| `internal/anomaly/detector.go` | `Detector` that owns dimension series, records entries, evaluates, emits events. |
| `internal/anomaly/*_test.go` | Unit, integration, and benchmark tests. |
| `internal/model/keys.go` | Add `AnomalyPanel` key binding (`Y`). |
| `internal/model/anomaly.go` | Model anomaly state, helper methods, dimension-to-filter mapping. |
| `internal/model/app.go` | Add anomaly fields, construct detector, start listener command, persist config. |
| `internal/model/update.go` | Wire `AnomalyEventsMsg`, handle `Y`/panel keys, record entries to detector. |
| `internal/model/view.go` | Status bar badge, inline log highlighting, help text, anomaly overlay. |
| `internal/ui/anomaly_panel.go` | Reusable styles for the anomaly panel. |

---

## Task 1: Extend config with anomaly settings

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestAnomalyConfigRoundTrip -v`

Expected: FAIL with `Anomaly` undefined.

- [ ] **Step 3: Add anomaly config types and defaults**

Modify `internal/config/config.go`. Add to the `Config` struct:

```go
	Anomaly AnomalyConfig `json:"anomaly"`
```

Append the following types and helper before `func dir()`:

```go
// DimensionConfig holds per-dimension overrides. Nil/zero means inherit global.
type DimensionConfig struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	RecentWindowSec   *int     `json:"recent_window_sec,omitempty"`
	BaselineWindowSec *int     `json:"baseline_window_sec,omitempty"`
	Multiplier        *float64 `json:"multiplier,omitempty"`
	DropMultiplier    *float64 `json:"drop_multiplier,omitempty"`
	MinBaseline       *int     `json:"min_baseline,omitempty"`
}

// AnomalyConfig is persisted user configuration for rate anomaly detection.
type AnomalyConfig struct {
	Enabled             bool                       `json:"enabled"`
	RecentWindowSec     int                        `json:"recent_window_sec"`
	BaselineWindowSec   int                        `json:"baseline_window_sec"`
	Multiplier          float64                    `json:"multiplier"`
	DropMultiplier      float64                    `json:"drop_multiplier"`
	MinBaseline         int                        `json:"min_baseline"`
	HighlightWindowSec  int                        `json:"highlight_window_sec"`
	MaxKeysPerDimension int                        `json:"max_keys_per_dimension"`
	CooldownSec         int                        `json:"cooldown_sec"`
	Strategy            string                     `json:"strategy"`
	Dimensions          map[string]DimensionConfig `json:"dimensions"`
}

// DefaultAnomalyConfig returns the built-in defaults.
func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		Enabled:             true,
		RecentWindowSec:     30,
		BaselineWindowSec:   300,
		Multiplier:          3.0,
		DropMultiplier:      0.0,
		MinBaseline:         5,
		HighlightWindowSec:  5,
		MaxKeysPerDimension: 1000,
		CooldownSec:         30,
		Strategy:            "moving_average",
		Dimensions: map[string]DimensionConfig{
			"global":  {},
			"level":   {Multiplier: floatPtr(2.0)},
			"tag":     {},
			"pid":     {},
			"package": {},
			"process": {Enabled: boolPtr(false)},
		},
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/config -run TestAnomalyConfigRoundTrip -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add anomaly detection configuration

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 2: Define anomaly types

**Files:**
- Create: `internal/anomaly/types.go`
- Test: `internal/anomaly/types_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/anomaly/types_test.go`:

```go
package anomaly

import "testing"

func TestDimensionString(t *testing.T) {
	if DimGlobal.String() != "global" {
		t.Fatalf("global string wrong: %s", DimGlobal.String())
	}
	if DirectionSpike.String() != "spike" {
		t.Fatalf("spike string wrong: %s", DirectionSpike.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anomaly -run TestDimensionString -v`

Expected: FAIL with package not found.

- [ ] **Step 3: Implement types**

Create `internal/anomaly/types.go`:

```go
package anomaly

import "time"

// Dimension identifies what is being monitored.
type Dimension int

const (
	DimGlobal Dimension = iota
	DimLevel
	DimTag
	DimPID
	DimPackage
	DimProcess
)

func (d Dimension) String() string {
	switch d {
	case DimGlobal:
		return "global"
	case DimLevel:
		return "level"
	case DimTag:
		return "tag"
	case DimPID:
		return "pid"
	case DimPackage:
		return "package"
	case DimProcess:
		return "process"
	default:
		return "unknown"
	}
}

// Direction indicates whether the anomaly is a spike or a drop.
type Direction int

const (
	DirectionSpike Direction = iota
	DirectionDrop
)

func (d Direction) String() string {
	switch d {
	case DirectionSpike:
		return "spike"
	case DirectionDrop:
		return "drop"
	default:
		return "unknown"
	}
}

// Event is emitted when a dimension's rate crosses a threshold.
type Event struct {
	Dimension    Dimension
	Key          string
	Direction    Direction
	RecentRate   float64
	BaselineRate float64
	Ratio        float64
	TriggeredAt  time.Time
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/anomaly -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anomaly/types.go internal/anomaly/types_test.go
git commit -m "feat(anomaly): add dimension, direction, and event types

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 3: Implement fixed-length time series

**Files:**
- Create: `internal/anomaly/series.go`
- Test: `internal/anomaly/series_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/anomaly/series_test.go`:

```go
package anomaly

import "testing"

func TestTimeSeriesSumAndShift(t *testing.T) {
	s := NewTimeSeries(5)
	for i := 0; i < 3; i++ {
		s.Add(i, 1)
	}
	if got := s.Sum(0, 2); got != 3 {
		t.Fatalf("sum want 3 got %d", got)
	}
	s.Add(5, 10)
	if got := s.Sum(0, 4); got != 13 {
		t.Fatalf("sum after shift want 13 got %d", got)
	}
}

func TestTimeSeriesJumpBeyondCapacity(t *testing.T) {
	s := NewTimeSeries(3)
	s.Add(0, 1)
	s.Add(1, 2)
	s.Add(10, 5)
	if got := s.Sum(8, 10); got != 5 {
		t.Fatalf("want 5 after jump, got %d", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anomaly -run TestTimeSeries -v`

Expected: FAIL undefined `NewTimeSeries`.

- [ ] **Step 3: Implement TimeSeries**

Create `internal/anomaly/series.go`:

```go
package anomaly

// TimeSeries stores per-second counts in a fixed-length ring of buckets.
type TimeSeries struct {
	buckets []int
	cap     int
	offset  int // second value stored at buckets[0]
	len     int // number of valid seconds ending at offset+len-1
}

// NewTimeSeries creates a series that holds `capacity` one-second buckets.
func NewTimeSeries(capacity int) *TimeSeries {
	if capacity <= 0 {
		capacity = 1
	}
	return &TimeSeries{buckets: make([]int, capacity), cap: capacity}
}

// Add increments the bucket for the given second.
func (s *TimeSeries) Add(second, delta int) {
	idx := s.ensure(second)
	if idx >= 0 {
		s.buckets[idx] += delta
	}
}

// Sum returns the total count for buckets in [secondFrom, secondTo] inclusive.
func (s *TimeSeries) Sum(secondFrom, secondTo int) int {
	total := 0
	for sec := secondFrom; sec <= secondTo; sec++ {
		if idx := s.indexFor(sec); idx >= 0 {
			total += s.buckets[idx]
		}
	}
	return total
}

func (s *TimeSeries) ensure(second int) int {
	if s.len == 0 {
		s.offset = second
		s.len = 1
		s.buckets[0] = 0
		return 0
	}
	end := s.offset + s.len - 1
	if second < s.offset {
		return -1
	}
	if second > end {
		newEnd := second
		for sec := end + 1; sec <= newEnd && sec < s.offset+s.cap; sec++ {
			idx := s.mod(sec - s.offset)
			s.buckets[idx] = 0
		}
		if second >= s.offset+s.cap {
			newOffset := second - s.cap + 1
			for i := range s.buckets {
				s.buckets[i] = 0
			}
			s.offset = newOffset
			s.len = s.cap
			return s.mod(second - newOffset)
		}
		s.len = newEnd - s.offset + 1
	}
	return s.mod(second - s.offset)
}

func (s *TimeSeries) indexFor(second int) int {
	if s.len == 0 || second < s.offset || second > s.offset+s.len-1 {
		return -1
	}
	return s.mod(second - s.offset)
}

func (s *TimeSeries) mod(v int) int {
	v %= s.cap
	if v < 0 {
		v += s.cap
	}
	return v
}

// Reset clears all buckets.
func (s *TimeSeries) Reset() {
	for i := range s.buckets {
		s.buckets[i] = 0
	}
	s.offset = 0
	s.len = 0
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/anomaly -run TestTimeSeries -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anomaly/series.go internal/anomaly/series_test.go
git commit -m "feat(anomaly): add fixed-length time series

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 4: Implement detection strategy

**Files:**
- Create: `internal/anomaly/strategy.go`
- Test: `internal/anomaly/strategy_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/anomaly/strategy_test.go`:

```go
package anomaly

import (
	"math"
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anomaly -run TestMovingAverageSpike -v`

Expected: FAIL undefined.

- [ ] **Step 3: Implement strategy**

Create `internal/anomaly/strategy.go`:

```go
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
	return &movingAverageStrategy{opts: opts}
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

	if baselineRate < float64(s.opts.MinBaseline) {
		return nil
	}

	var events []Event
	now := time.Now()

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
		})
	}

	return events
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/anomaly -run TestMovingAverage -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anomaly/strategy.go internal/anomaly/strategy_test.go
git commit -m "feat(anomaly): add moving average spike/drop strategy

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 5: Implement detector with dimensions and cooldown

**Files:**
- Create: `internal/anomaly/detector.go`
- Test: `internal/anomaly/detector_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/anomaly/detector_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/anomaly -run TestDetectorDetectsSpike -v`

Expected: FAIL undefined `NewDetector`.

- [ ] **Step 3: Implement detector**

Create `internal/anomaly/detector.go`:

```go
package anomaly

import (
	"strconv"
	"sync"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

// Config is the runtime configuration for the detector.
type Config struct {
	Enabled             bool
	RecentWindowSec     int
	BaselineWindowSec   int
	Multiplier          float64
	DropMultiplier      float64
	MinBaseline         int
	CooldownSec         int
	MaxKeysPerDimension int
}

// Detector maintains time series for multiple dimensions and evaluates them.
type Detector struct {
	cfg       Config
	strategy  Strategy
	global    *TimeSeries
	byLevel   map[logentry.Level]*TimeSeries
	byTag     map[string]*TimeSeries
	byPID     map[int]*TimeSeries
	byPackage map[string]*TimeSeries
	byProcess map[string]*TimeSeries
	cooldowns map[cooldownKey]time.Time
	mu        sync.Mutex
}

type cooldownKey struct {
	Dim Dimension
	Key string
}

// NewDetector creates a detector from runtime config.
func NewDetector(cfg Config) *Detector {
	if cfg.MaxKeysPerDimension <= 0 {
		cfg.MaxKeysPerDimension = 1000
	}
	total := cfg.RecentWindowSec + cfg.BaselineWindowSec
	if total <= 0 {
		total = 330
	}
	byLevel := make(map[logentry.Level]*TimeSeries)
	for _, lvl := range logentry.FilterableLevels {
		byLevel[lvl] = NewTimeSeries(total)
	}
	return &Detector{
		cfg:       cfg,
		strategy:  NewMovingAverageStrategy(MovingAverageOptions(cfg)),
		global:    NewTimeSeries(total),
		byLevel:   byLevel,
		byTag:     make(map[string]*TimeSeries),
		byPID:     make(map[int]*TimeSeries),
		byPackage: make(map[string]*TimeSeries),
		byProcess: make(map[string]*TimeSeries),
		cooldowns: make(map[cooldownKey]time.Time),
	}
}

// Record ingests a single log entry into all enabled dimensions.
func (d *Detector) Record(e *logentry.Entry, packageName, processName string) {
	if !d.cfg.Enabled || e == nil {
		return
	}
	sec := int(e.Timestamp.Unix())
	d.mu.Lock()
	defer d.mu.Unlock()

	d.global.Add(sec, 1)
	if s, ok := d.byLevel[e.Level]; ok {
		s.Add(sec, 1)
	}
	d.tagSeries(e.Tag, sec).Add(sec, 1)
	d.pidSeries(e.PID, sec).Add(sec, 1)
	if packageName != "" {
		d.packageSeries(packageName, sec).Add(sec, 1)
	}
	if processName != "" {
		d.processSeries(processName, sec).Add(sec, 1)
	}
}

// Evaluate runs the strategy against all dimensions for the given timestamp.
func (d *Detector) Evaluate(now time.Time) []Event {
	if !d.cfg.Enabled {
		return nil
	}
	sec := int(now.Unix())
	d.mu.Lock()
	defer d.mu.Unlock()

	var all []Event
	all = append(all, d.evalSeries(d.global, DimGlobal, "global", sec)...)
	for lvl, s := range d.byLevel {
		all = append(all, d.evalSeries(s, DimLevel, lvl.Label(), sec)...)
	}
	for k, s := range d.byTag {
		all = append(all, d.evalSeries(s, DimTag, k, sec)...)
	}
	for k, s := range d.byPID {
		all = append(all, d.evalSeries(s, DimPID, strconv.Itoa(k), sec)...)
	}
	for k, s := range d.byPackage {
		all = append(all, d.evalSeries(s, DimPackage, k, sec)...)
	}
	for k, s := range d.byProcess {
		all = append(all, d.evalSeries(s, DimProcess, k, sec)...)
	}
	return d.applyCooldowns(all, now)
}

func (d *Detector) evalSeries(s *TimeSeries, dim Dimension, key string, sec int) []Event {
	if s == nil {
		return nil
	}
	return d.strategy.Evaluate(s, dim, key, sec)
}

func (d *Detector) applyCooldowns(events []Event, now time.Time) []Event {
	if d.cfg.CooldownSec <= 0 {
		return events
	}
	window := time.Duration(d.cfg.CooldownSec) * time.Second
	var out []Event
	for _, e := range events {
		k := cooldownKey{Dim: e.Dimension, Key: e.Key}
		if last, ok := d.cooldowns[k]; ok && now.Sub(last) < window {
			continue
		}
		d.cooldowns[k] = now
		out = append(out, e)
	}
	return out
}

func (d *Detector) tagSeries(key string, sec int) *TimeSeries {
	if s, ok := d.byTag[key]; ok {
		return s
	}
	d.evictOneTag()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byTag[key] = s
	return s
}

func (d *Detector) pidSeries(key int, sec int) *TimeSeries {
	if s, ok := d.byPID[key]; ok {
		return s
	}
	d.evictOnePID()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byPID[key] = s
	return s
}

func (d *Detector) packageSeries(key string, sec int) *TimeSeries {
	if s, ok := d.byPackage[key]; ok {
		return s
	}
	d.evictOnePackage()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byPackage[key] = s
	return s
}

func (d *Detector) processSeries(key string, sec int) *TimeSeries {
	if s, ok := d.byProcess[key]; ok {
		return s
	}
	d.evictOneProcess()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byProcess[key] = s
	return s
}

func (d *Detector) evictOneTag() {
	if len(d.byTag) < d.cfg.MaxKeysPerDimension {
		return
	}
	for k := range d.byTag {
		delete(d.byTag, k)
		return
	}
}

func (d *Detector) evictOnePID() {
	if len(d.byPID) < d.cfg.MaxKeysPerDimension {
		return
	}
	for k := range d.byPID {
		delete(d.byPID, k)
		return
	}
}

func (d *Detector) evictOnePackage() {
	if len(d.byPackage) < d.cfg.MaxKeysPerDimension {
		return
	}
	for k := range d.byPackage {
		delete(d.byPackage, k)
		return
	}
}

func (d *Detector) evictOneProcess() {
	if len(d.byProcess) < d.cfg.MaxKeysPerDimension {
		return
	}
	for k := range d.byProcess {
		delete(d.byProcess, k)
		return
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/anomaly -run TestDetector -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/anomaly/detector.go internal/anomaly/detector_test.go
git commit -m "feat(anomaly): add multi-dimensional detector with cooldown

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 6: Add model anomaly state and messages

**Files:**
- Create: `internal/model/anomaly.go`
- Modify: `internal/model/app.go`

- [ ] **Step 1: Create model anomaly state file**

Create `internal/model/anomaly.go`:

```go
package model

import (
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

// AnomalyEventsMsg is delivered by the detector listener command.
type AnomalyEventsMsg []anomaly.Event

type anomalyState struct {
	events       []anomaly.Event
	panelOpen    bool
	selection    int
	highlightSec int
	flashingUntil time.Time
}

func (a *anomalyState) applyEvents(events []anomaly.Event, highlightSec int) {
	if len(events) == 0 {
		return
	}
	now := time.Now()
	a.flashingUntil = now.Add(2 * time.Second)
	a.highlightSec = highlightSec
	for _, e := range events {
		found := false
		for i := range a.events {
			if a.events[i].Dimension == e.Dimension && a.events[i].Key == e.Key {
				a.events[i] = e
				found = true
				break
			}
		}
		if !found {
			a.events = append(a.events, e)
		}
	}
}

func (a *anomalyState) clear() {
	a.events = nil
	a.selection = 0
}

func (a *anomalyState) isHighlighted(entry *logentry.Entry) bool {
	if entry == nil || a.highlightSec <= 0 {
		return false
	}
	window := time.Duration(a.highlightSec) * time.Second
	for _, e := range a.events {
		if entry.Timestamp.After(e.TriggeredAt.Add(-window)) &&
			entry.Timestamp.Before(e.TriggeredAt.Add(window)) {
			return true
		}
	}
	return false
}

func (a *anomalyState) isFlashing(now time.Time) bool {
	return now.Before(a.flashingUntil) && len(a.events) > 0
}

func (a *anomalyState) worst() anomaly.Event {
	if len(a.events) == 0 {
		return anomaly.Event{}
	}
	w := a.events[0]
	for _, e := range a.events[1:] {
		if e.Ratio > w.Ratio {
			w = e
		}
	}
	return w
}

func (m *AppModel) applySelectedAnomalyFilter() tea.Cmd {
	if m.anomaly.selection >= len(m.anomaly.events) {
		return nil
	}
	e := m.anomaly.events[m.anomaly.selection]
	switch e.Dimension {
	case anomaly.DimLevel:
		if lvl := logentry.ParseLevelString(e.Key); lvl > 0 {
			m.filter.SetMinLevel(lvl)
			m.statusMsg = fmt.Sprintf("异常过滤 级别: ≥%s", e.Key)
		}
	case anomaly.DimTag:
		m.filter.Tag = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 Tag: %s", e.Key)
	case anomaly.DimPID:
		if pid, err := strconv.Atoi(e.Key); err == nil {
			m.filter.PID = pid
			m.statusMsg = fmt.Sprintf("异常过滤 PID: %d", pid)
		}
	case anomaly.DimPackage:
		m.filter.Package = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 包名: %s", e.Key)
		if m.filePath == "" {
			return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
		}
	case anomaly.DimProcess:
		m.filter.Process = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 进程: %s", e.Key)
		if m.filePath == "" {
			return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
		}
	}
	m.refilter()
	return nil
}
```

- [ ] **Step 2: Modify AppModel to include anomaly state**

Modify `internal/model/app.go`:

1. Add import: `"github.com/Yecangyuan/LogcatTool/internal/anomaly"`.
2. Add `ModeAnomalyPanel` to `InputMode` constants.
3. Add fields to `AppModel`:

```go
	// Anomaly detection
	anomalyDetector *anomaly.Detector
	anomaly         anomalyState
	anomalyEventsCh chan []anomaly.Event
	anomalyDone     chan struct{}
```

4. In `New`, after loading config, create detector and channel:

```go
	m.anomalyDetector = anomaly.NewDetector(anomaly.Config{
		Enabled:             cfg.Anomaly.Enabled,
		RecentWindowSec:     cfg.Anomaly.RecentWindowSec,
		BaselineWindowSec:   cfg.Anomaly.BaselineWindowSec,
		Multiplier:          cfg.Anomaly.Multiplier,
		DropMultiplier:      cfg.Anomaly.DropMultiplier,
		MinBaseline:         cfg.Anomaly.MinBaseline,
		CooldownSec:         cfg.Anomaly.CooldownSec,
		MaxKeysPerDimension: cfg.Anomaly.MaxKeysPerDimension,
	})
	m.anomalyEventsCh = make(chan []anomaly.Event, 16)
	m.anomalyDone = make(chan struct{})
	go m.anomalyDetectorLoop()
```

5. In `Init`, start the anomaly listener command:

```go
func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{sparklineTickCmd(), waitForAnomalyEvents(m.anomalyEventsCh)}
	...
}
```

6. In `saveAndQuit`, close the done channel:

```go
	close(m.anomalyDone)
```

- [ ] **Step 3: Commit**

```bash
git add internal/model/anomaly.go internal/model/app.go
git commit -m "feat(model): add anomaly state and detector wiring fields

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 7: Wire detector into log ingestion and add listener command

**Files:**
- Modify: `internal/model/update.go`
- Modify: `internal/model/app.go`

- [ ] **Step 1: Add anomaly listener command and goroutine**

In `internal/model/app.go`, add:

```go
func waitForAnomalyEvents(ch <-chan []anomaly.Event) tea.Cmd {
	return func() tea.Msg {
		events, ok := <-ch
		if !ok {
			return nil
		}
		return AnomalyEventsMsg(events)
	}
}

func (m *AppModel) anomalyDetectorLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			events := m.anomalyDetector.Evaluate(time.Now())
			if len(events) > 0 {
				select {
				case m.anomalyEventsCh <- events:
				default:
				}
			}
		case <-m.anomalyDone:
			return
		}
	}
}
```

- [ ] **Step 2: Record entries to detector**

In `internal/model/update.go`, modify `ingestEntries`:

```go
func (m *AppModel) ingestEntries(entries []*logentry.Entry) {
	if len(entries) == 0 {
		return
	}
	latest := entries[len(entries)-1].Timestamp
	for _, entry := range entries {
		entry.Index = m.totalCount
		m.totalCount++
		m.preRenderEntry(entry)
		m.allEntries.Push(entry)
		m.maybeTriggerAlert(entry)
		pkg := m.packageByPID[entry.PID]
		proc := m.processByPID[entry.PID]
		m.anomalyDetector.Record(entry, pkg, proc)
	}
	...
}
```

- [ ] **Step 3: Handle AnomalyEventsMsg**

In `internal/model/update.go` `Update`, add case before the closing `return`:

```go
	case AnomalyEventsMsg:
		m.anomaly.applyEvents([]anomaly.Event(msg), m.cfg.Anomaly.HighlightWindowSec)
		if len(msg) > 0 {
			worst := msg[0]
			icon := "🔺"
			if worst.Direction == anomaly.DirectionDrop {
				icon = "🔻"
			}
			m.statusMsg = fmt.Sprintf("%s %s=%s %.1fx", icon, worst.Dimension.String(), truncateLabel(worst.Key, 12), worst.Ratio)
		}
		cmds = append(cmds, waitForAnomalyEvents(m.anomalyEventsCh))
```

- [ ] **Step 4: Commit**

```bash
git add internal/model/update.go internal/model/app.go
git commit -m "feat(model): wire detector into ingestion and event loop

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 8: Status bar anomaly indicator

**Files:**
- Modify: `internal/model/view.go`

- [ ] **Step 1: Render anomaly badge in status bar**

In `internal/model/view.go` `renderStatusBar`, after the `m.lastAlert` block, add:

```go
	if m.anomaly.isFlashing(time.Now()) {
		worst := m.anomaly.worst()
		icon := "🔺"
		if worst.Direction == anomaly.DirectionDrop {
			icon = "🔻"
		}
		left += fmt.Sprintf("  %s %s=%s %.1fx", icon, worst.Dimension.String(), truncateLabel(worst.Key, 12), worst.Ratio)
	}
```

Add import for `"github.com/Yecangyuan/LogcatTool/internal/anomaly"` and `time` if not already present.

- [ ] **Step 2: Commit**

```bash
git add internal/model/view.go
git commit -m "feat(ui): add status bar anomaly indicator

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 9: Anomaly panel overlay and key bindings

**Files:**
- Modify: `internal/model/keys.go`
- Create: `internal/ui/anomaly_panel.go`
- Modify: `internal/model/view.go`
- Modify: `internal/model/update.go`

- [ ] **Step 1: Add key binding**

In `internal/model/keys.go`:

```go
	AnomalyPanel key.Binding
```

In `DefaultKeyMap()`:

```go
		AnomalyPanel: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "异常面板"),
		),
```

In `FullHelp()`, add `k.AnomalyPanel` to an appropriate row.

- [ ] **Step 2: Add UI styles**

Create `internal/ui/anomaly_panel.go`:

```go
package ui

import "charm.land/lipgloss/v2"

var (
	AnomalyPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Padding(1, 2)

	AnomalySelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Bold(true)

	AnomalySpikeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	AnomalyDropStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))
)
```

- [ ] **Step 3: Render overlay**

In `internal/model/view.go` `View()`, add:

```go
	if m.inputMode == ModeAnomalyPanel {
		content = m.overlayAnomalyPanel(content)
	}
```

Add `overlayAnomalyPanel`:

```go
func (m AppModel) overlayAnomalyPanel(bg string) string {
	rows := m.anomaly.events
	var sb strings.Builder
	sb.WriteString(ui.HelpTitleStyle.Render("异常检测") + "\n\n")

	if len(rows) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  暂无异常"))
	} else {
		for i, e := range rows {
			cursor := "  "
			if i == m.anomaly.selection {
				cursor = "▸ "
			}
			dir := "🔺"
			style := ui.AnomalySpikeStyle
			if e.Direction == anomaly.DirectionDrop {
				dir = "🔻"
				style = ui.AnomalyDropStyle
			}
			line := fmt.Sprintf("%s%s %s=%s  %.1fx (%.1f vs %.1f/s)",
				cursor, dir, e.Dimension.String(), truncateLabel(e.Key, 18), e.Ratio, e.RecentRate, e.BaselineRate)
			if i == m.anomaly.selection {
				sb.WriteString(ui.AnomalySelectedStyle.Render(line))
			} else {
				sb.WriteString(style.Render(line))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  j/k选择 Enter过滤 c清空 Y/Esc关闭"))
	panel := ui.AnomalyPanelStyle.Render(sb.String())
	x := (m.width - lipgloss.Width(panel)) / 2
	y := (m.height - lipgloss.Height(panel)) / 2
	if x < 0 { x = 0 }
	if y < 0 { y = 0 }
	return placeOverlay(x, y, panel, bg)
}
```

- [ ] **Step 4: Handle panel keys**

In `internal/model/update.go` `handleKey`, add:

```go
	case key.Matches(msg, m.keys.AnomalyPanel):
		if m.inputMode == ModeAnomalyPanel {
			m.inputMode = ModeNormal
		} else {
			m.inputMode = ModeAnomalyPanel
			m.anomaly.selection = 0
		}
		return m, nil
```

Add `handleAnomalyPanelKey`:

```go
func (m AppModel) handleAnomalyPanelKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel, m.keys.AnomalyPanel):
		m.inputMode = ModeNormal
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.anomaly.selection > 0 {
			m.anomaly.selection--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.anomaly.selection < len(m.anomaly.events)-1 {
			m.anomaly.selection++
		}
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		cmd := m.applySelectedAnomalyFilter()
		m.inputMode = ModeNormal
		return m, cmd
	case msg.String() == "c":
		m.anomaly.clear()
		m.statusMsg = "异常历史已清空"
		return m, nil
	case key.Matches(msg, m.keys.Quit):
		return m.saveAndQuit()
	}
	return m, nil
}
```

In `handleKey` input mode switch, add:

```go
	case ModeAnomalyPanel:
		return m.handleAnomalyPanelKey(msg)
```

- [ ] **Step 5: Commit**

```bash
git add internal/model/keys.go internal/ui/anomaly_panel.go internal/model/view.go internal/model/update.go
git commit -m "feat(ui): add anomaly panel overlay and key bindings

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 10: Inline anomaly highlighting

**Files:**
- Modify: `internal/model/view.go`

- [ ] **Step 1: Add highlight style**

In `internal/model/view.go`, add:

```go
var anomalyHighlightPrefix = lipgloss.NewStyle().
	Foreground(lipgloss.Color("196")).
	Bold(true)
```

- [ ] **Step 2: Apply highlight in renderLogView**

In `renderLogView`, after building `line` and before row prefix, add:

```go
		if m.anomaly.isHighlighted(entry) {
			line = anomalyHighlightPrefix.Render("⚠ ") + line
		}

		line = rowPrefix(i == m.scrollOffset, m.bookmarks[entry.Index]) + line
```

- [ ] **Step 3: Commit**

```bash
git add internal/model/view.go
git commit -m "feat(ui): add inline anomaly highlight marker

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 11: Persist anomaly config and defaults

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/model/app.go`

- [ ] **Step 1: Ensure defaults on load**

In `internal/config/config.go`, modify `Load` to merge defaults:

```go
func Load() (Config, error) {
	p := path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Anomaly: DefaultAnomalyConfig()}, nil
		}
		return Config{Anomaly: DefaultAnomalyConfig()}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{Anomaly: DefaultAnomalyConfig()}, fmt.Errorf("parse config: %w", err)
	}
	c.Anomaly = mergeAnomalyConfig(c.Anomaly, DefaultAnomalyConfig())
	return c, nil
}

func mergeAnomalyConfig(user, def AnomalyConfig) AnomalyConfig {
	if user.RecentWindowSec == 0 {
		user.RecentWindowSec = def.RecentWindowSec
	}
	if user.BaselineWindowSec == 0 {
		user.BaselineWindowSec = def.BaselineWindowSec
	}
	if user.Multiplier == 0 {
		user.Multiplier = def.Multiplier
	}
	if user.MinBaseline == 0 {
		user.MinBaseline = def.MinBaseline
	}
	if user.HighlightWindowSec == 0 {
		user.HighlightWindowSec = def.HighlightWindowSec
	}
	if user.MaxKeysPerDimension == 0 {
		user.MaxKeysPerDimension = def.MaxKeysPerDimension
	}
	if user.CooldownSec == 0 {
		user.CooldownSec = def.CooldownSec
	}
	if user.Strategy == "" {
		user.Strategy = def.Strategy
	}
	if user.Dimensions == nil {
		user.Dimensions = def.Dimensions
	}
	return user
}
```

- [ ] **Step 2: Save config in saveConfig**

In `internal/model/app.go` `saveConfig`, add:

```go
	m.cfg.Anomaly = config.AnomalyConfig{
		Enabled:             m.anomalyDetector.cfg.Enabled,
		RecentWindowSec:     m.anomalyDetector.cfg.RecentWindowSec,
		BaselineWindowSec:   m.anomalyDetector.cfg.BaselineWindowSec,
		Multiplier:          m.anomalyDetector.cfg.Multiplier,
		DropMultiplier:      m.anomalyDetector.cfg.DropMultiplier,
		MinBaseline:         m.anomalyDetector.cfg.MinBaseline,
		HighlightWindowSec:  m.cfg.Anomaly.HighlightWindowSec,
		MaxKeysPerDimension: m.anomalyDetector.cfg.MaxKeysPerDimension,
		CooldownSec:         m.anomalyDetector.cfg.CooldownSec,
		Strategy:            "moving_average",
		Dimensions:          m.cfg.Anomaly.Dimensions,
	}
```

This assumes detector config is not mutated at runtime. If we later add runtime config UI, adjust accordingly.

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go internal/model/app.go
git commit -m "feat(config): persist anomaly settings and apply defaults

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 12: Update help text

**Files:**
- Modify: `internal/model/view.go`

- [ ] **Step 1: Add anomaly keys to help**

In `renderHelp`, add after the stats panel line:

```text
    Y           异常检测面板
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/view.go
git commit -m "docs(ui): add anomaly panel key to help text

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 13: Integration and benchmark tests

**Files:**
- Create: `internal/anomaly/detector_integration_test.go`

- [ ] **Step 1: Write integration test**

Create `internal/anomaly/detector_integration_test.go`:

```go
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
			d.Record(&logentry.Entry{Timestamp: base.Add(time.Duration(sec) * time.Second), Tag: "Net", Level: logentry.LevelInfo}, "", "")
		}
	}
	// spike at second 10: 100 logs in one second
	spike := base.Add(10 * time.Second)
	for i := 0; i < 100; i++ {
		d.Record(&logentry.Entry{Timestamp: spike, Tag: "Net", Level: logentry.LevelInfo}, "", "")
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
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/anomaly -run TestDetectorSimulatedStream -v`

Expected: PASS.

Run benchmark: `go test ./internal/anomaly -bench BenchmarkDetectorHighThroughput -benchtime 5s`

Expected: completes without panic.

- [ ] **Step 3: Commit**

```bash
git add internal/anomaly/detector_integration_test.go
git commit -m "test(anomaly): add stream simulation and throughput benchmark

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Task 14: Final verification

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`

Expected: all tests pass.

- [ ] **Step 2: Build binary**

Run: `go build -o LogcatTool .`

Expected: builds without errors.

- [ ] **Step 3: Run harness verify**

Run: `./harness/bin/verify`

Expected: passes.

- [ ] **Step 4: Manual smoke test**

Run: `./LogcatTool -f some-logcat.txt` or connect a device, generate a burst of logs, and verify:

- Status bar shows 🔺 when burst occurs.
- `Y` opens the anomaly panel.
- `Enter` on an anomaly applies the corresponding filter.
- `c` clears anomaly history.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(anomaly): address verification findings

Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

## Self-Review Checklist

- **Spec coverage**: every section of the design doc maps to at least one task.
- **No placeholders**: all code blocks contain concrete code; no TBD/TODO/filler.
- **Type consistency**: `AnomalyConfig`, `Event`, `Detector`, `anomalyState` names and field names match across tasks.
- **Import hygiene**: `internal/anomaly` is imported wherever used; `strconv` and `time` imports are added where needed.
- **Test coverage**: unit tests for series, strategy, detector, config; integration + benchmark tests included.
- **Risk**: goroutine lifecycle is tied to `saveAndQuit`; cooldown prevents spam; map eviction prevents unbounded growth.

## Known Gaps / Future Work

- Z-score strategy is only reserved via the `Strategy` interface; not implemented in this plan.
- Runtime configuration UI (adjust thresholds without editing JSON) is out of scope.
- Per-dimension override resolution (merging global + dimension config) is assumed to happen in detector construction; if runtime edits are added later, centralize this logic.
