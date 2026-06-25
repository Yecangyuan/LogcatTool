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

// ConfigToMovingAverageOptions converts Config to the strategy options.
func ConfigToMovingAverageOptions(cfg Config) MovingAverageOptions {
	return MovingAverageOptions{
		RecentWindowSec:   cfg.RecentWindowSec,
		BaselineWindowSec: cfg.BaselineWindowSec,
		Multiplier:        cfg.Multiplier,
		DropMultiplier:    cfg.DropMultiplier,
		MinBaseline:       cfg.MinBaseline,
	}
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
		strategy:  NewMovingAverageStrategy(ConfigToMovingAverageOptions(cfg)),
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
