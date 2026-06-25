package anomaly

import (
	"fmt"
	"os"
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
	Dimensions          map[string]DimensionConfig
}

// Detector maintains time series for multiple dimensions and evaluates them.
type Detector struct {
	cfg       Config
	global    *TimeSeries
	byLevel   map[logentry.Level]*TimeSeries
	byTag     map[string]*TimeSeries
	byPID     map[int]*TimeSeries
	byPackage map[string]*TimeSeries
	byProcess map[string]*TimeSeries
	cooldowns map[cooldownKey]time.Time
	done      chan struct{}
	mu        sync.Mutex

	globalOpts MovingAverageOptions
	dimOpts    map[Dimension]MovingAverageOptions
	dimEnabled map[Dimension]bool
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
	if cfg.Dimensions == nil {
		cfg.Dimensions = make(map[string]DimensionConfig)
	}
	total := cfg.RecentWindowSec + cfg.BaselineWindowSec
	if total <= 0 {
		total = 330
	}
	byLevel := make(map[logentry.Level]*TimeSeries)
	for _, lvl := range logentry.FilterableLevels {
		byLevel[lvl] = NewTimeSeries(total)
	}

	d := &Detector{
		cfg:        cfg,
		global:     NewTimeSeries(total),
		byLevel:    byLevel,
		byTag:      make(map[string]*TimeSeries),
		byPID:      make(map[int]*TimeSeries),
		byPackage:  make(map[string]*TimeSeries),
		byProcess:  make(map[string]*TimeSeries),
		cooldowns:  make(map[cooldownKey]time.Time),
		done:       make(chan struct{}),
		globalOpts: normalizeMovingAverageOptions(MovingAverageOptions{
			RecentWindowSec:   cfg.RecentWindowSec,
			BaselineWindowSec: cfg.BaselineWindowSec,
			Multiplier:        cfg.Multiplier,
			DropMultiplier:    cfg.DropMultiplier,
			MinBaseline:       cfg.MinBaseline,
		}),
		dimOpts:    make(map[Dimension]MovingAverageOptions),
		dimEnabled: make(map[Dimension]bool),
	}
	d.buildDimensionConfigs()
	return d
}

func (d *Detector) buildDimensionConfigs() {
	for dim := DimGlobal; dim <= DimProcess; dim++ {
		d.dimEnabled[dim] = d.cfg.Enabled
		d.dimOpts[dim] = d.globalOpts
	}

	for name, dc := range d.cfg.Dimensions {
		dim := parseDimension(name)
		if dim < 0 {
			continue
		}
		opts := d.globalOpts
		if dc.Enabled != nil {
			d.dimEnabled[dim] = *dc.Enabled
		}
		if dc.RecentWindowSec != nil {
			opts.RecentWindowSec = *dc.RecentWindowSec
		}
		if dc.BaselineWindowSec != nil {
			opts.BaselineWindowSec = *dc.BaselineWindowSec
		}
		if dc.Multiplier != nil {
			opts.Multiplier = *dc.Multiplier
		}
		if dc.DropMultiplier != nil {
			opts.DropMultiplier = *dc.DropMultiplier
		}
		if dc.MinBaseline != nil {
			opts.MinBaseline = *dc.MinBaseline
		}
		d.dimOpts[dim] = normalizeMovingAverageOptions(opts)
	}
}

func parseDimension(s string) Dimension {
	switch s {
	case "global":
		return DimGlobal
	case "level":
		return DimLevel
	case "tag":
		return DimTag
	case "pid":
		return DimPID
	case "package":
		return DimPackage
	case "process":
		return DimProcess
	default:
		return Dimension(-1)
	}
}

// Stop signals the background evaluation goroutine to exit.
func (d *Detector) Stop() {
	close(d.done)
}

// Done returns the channel used to signal detector shutdown.
func (d *Detector) Done() <-chan struct{} {
	return d.done
}

// Record ingests a single log entry into all enabled dimensions.
func (d *Detector) Record(e *logentry.Entry, packageName, processName string) {
	if !d.cfg.Enabled || e == nil {
		return
	}
	sec := int(e.Timestamp.Unix())
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.dimEnabled[DimGlobal] {
		d.global.Add(sec, 1)
	}
	if d.dimEnabled[DimLevel] {
		if s, ok := d.byLevel[e.Level]; ok {
			s.Add(sec, 1)
		}
	}
	if d.dimEnabled[DimTag] {
		d.tagSeries(e.Tag).Add(sec, 1)
	}
	if d.dimEnabled[DimPID] {
		d.pidSeries(e.PID).Add(sec, 1)
	}
	if d.dimEnabled[DimPackage] && packageName != "" {
		d.packageSeries(packageName).Add(sec, 1)
	}
	if d.dimEnabled[DimProcess] && processName != "" {
		d.processSeries(processName).Add(sec, 1)
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
	if d.dimEnabled[DimGlobal] {
		all = append(all, d.evalSeries(d.global, DimGlobal, "global", sec, d.dimOpts[DimGlobal])...)
	}
	if d.dimEnabled[DimLevel] {
		for lvl, s := range d.byLevel {
			all = append(all, d.evalSeries(s, DimLevel, lvl.Label(), sec, d.dimOpts[DimLevel])...)
		}
	}
	if d.dimEnabled[DimTag] {
		for k, s := range d.byTag {
			all = append(all, d.evalSeries(s, DimTag, k, sec, d.dimOpts[DimTag])...)
		}
	}
	if d.dimEnabled[DimPID] {
		for k, s := range d.byPID {
			all = append(all, d.evalSeries(s, DimPID, strconv.Itoa(k), sec, d.dimOpts[DimPID])...)
		}
	}
	if d.dimEnabled[DimPackage] {
		for k, s := range d.byPackage {
			all = append(all, d.evalSeries(s, DimPackage, k, sec, d.dimOpts[DimPackage])...)
		}
	}
	if d.dimEnabled[DimProcess] {
		for k, s := range d.byProcess {
			all = append(all, d.evalSeries(s, DimProcess, k, sec, d.dimOpts[DimProcess])...)
		}
	}
	return d.applyCooldowns(all, now)
}

func (d *Detector) evalSeries(s *TimeSeries, dim Dimension, key string, sec int, opts MovingAverageOptions) (events []Event) {
	if s == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "%s [anomaly panic] dimension=%s key=%s: %v\n",
				time.Now().UTC().Format("2006-01-02T15:04:05Z"), dim.String(), key, r)
			events = nil
		}
	}()
	return NewMovingAverageStrategy(opts).Evaluate(s, dim, key, sec)
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
	// Clean expired cooldown entries to prevent unbounded growth.
	for k, last := range d.cooldowns {
		if now.Sub(last) >= window {
			delete(d.cooldowns, k)
		}
	}
	return out
}

func (d *Detector) tagSeries(key string) *TimeSeries {
	if s, ok := d.byTag[key]; ok {
		return s
	}
	d.evictOneTag()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byTag[key] = s
	return s
}

func (d *Detector) pidSeries(key int) *TimeSeries {
	if s, ok := d.byPID[key]; ok {
		return s
	}
	d.evictOnePID()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byPID[key] = s
	return s
}

func (d *Detector) packageSeries(key string) *TimeSeries {
	if s, ok := d.byPackage[key]; ok {
		return s
	}
	d.evictOnePackage()
	s := NewTimeSeries(d.cfg.RecentWindowSec + d.cfg.BaselineWindowSec)
	d.byPackage[key] = s
	return s
}

func (d *Detector) processSeries(key string) *TimeSeries {
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
