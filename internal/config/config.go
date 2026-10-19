package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
)

// Config holds user-persistent settings.
type Config struct {
	FavoritePackages  map[string]bool `json:"favorite_packages"`
	FavoriteProcesses map[string]bool `json:"favorite_processes"`
	AlertKeyword      string          `json:"alert_keyword"`
	SearchHistory     []string        `json:"search_history"`
	Presets           [3]Preset       `json:"presets"`
	Profiles          []Profile       `json:"profiles"`
	CollapseDupes     bool            `json:"collapse_dupes"`
	WrapLines         bool            `json:"wrap_lines"`
	ShowDetails       bool            `json:"show_details"`
	AutoScroll        bool            `json:"auto_scroll"`
	Anomaly           AnomalyConfig   `json:"anomaly"`
}

// Preset mirrors filterPreset for JSON serialization.
type Preset struct {
	Used          bool   `json:"used"`
	MinLevel      string `json:"min_level,omitempty"`
	Package       string `json:"package,omitempty"`
	Process       string `json:"process,omitempty"`
	Tag           string `json:"tag,omitempty"`
	TagExclude    string `json:"tag_exclude,omitempty"`
	PID           int    `json:"pid,omitempty"`
	SearchText    string `json:"search_text,omitempty"`
	CrashOnly     bool   `json:"crash_only,omitempty"`
	TimeWindowSec int    `json:"time_window_sec,omitempty"`
}

// Profile is a named, persistent filter snapshot.
type Profile struct {
	Name          string `json:"name"`
	MinLevel      string `json:"min_level,omitempty"`
	Package       string `json:"package,omitempty"`
	Process       string `json:"process,omitempty"`
	Tag           string `json:"tag,omitempty"`
	TagExclude    string `json:"tag_exclude,omitempty"`
	PID           int    `json:"pid,omitempty"`
	SearchText    string `json:"search_text,omitempty"`
	IsRegex       bool   `json:"is_regex,omitempty"`
	CrashOnly     bool   `json:"crash_only,omitempty"`
	TimeWindowSec int    `json:"time_window_sec,omitempty"`
}

// AnomalyConfig is persisted user configuration for rate anomaly detection.
type AnomalyConfig struct {
	Enabled             bool                               `json:"enabled"`
	RecentWindowSec     int                                `json:"recent_window_sec"`
	BaselineWindowSec   int                                `json:"baseline_window_sec"`
	Multiplier          float64                            `json:"multiplier"`
	DropMultiplier      float64                            `json:"drop_multiplier"`
	MinBaseline         int                                `json:"min_baseline"`
	HighlightWindowSec  int                                `json:"highlight_window_sec"`
	MaxKeysPerDimension int                                `json:"max_keys_per_dimension"`
	CooldownSec         int                                `json:"cooldown_sec"`
	Strategy            string                             `json:"strategy"`
	Dimensions          map[string]anomaly.DimensionConfig `json:"dimensions"`
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
		Dimensions: map[string]anomaly.DimensionConfig{
			"global":  {Enabled: boolPtr(true)},
			"level":   {Enabled: boolPtr(true), Multiplier: floatPtr(2.0)},
			"tag":     {Enabled: boolPtr(true)},
			"pid":     {Enabled: boolPtr(true)},
			"package": {Enabled: boolPtr(true)},
			"process": {Enabled: boolPtr(false)},
		},
	}
}

func boolPtr(b bool) *bool        { return &b }
func floatPtr(f float64) *float64 { return &f }

const maxSearchHistory = 20

func dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "logcatool")
}

func path() string {
	return filepath.Join(dir(), "config.json")
}

// Load reads the config file, returning a zero Config if missing or invalid.
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
	c.Anomaly = mergeAnomalyConfig(c.Anomaly, DefaultAnomalyConfig(), anomalyKeysPresent(data))
	return c, nil
}

// anomalyKeysPresent reports which JSON keys the user's `anomaly` object actually
// contains.
//
// Zero values alone are ambiguous: `"enabled": false` and a config that omits
// `enabled` entirely both decode to false, yet only the first is an explicit
// choice. Tracking key presence lets merging respect what the user wrote instead
// of silently switching anomaly detection back on.
func anomalyKeysPresent(data []byte) map[string]bool {
	var envelope struct {
		Anomaly map[string]json.RawMessage `json:"anomaly"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil
	}
	present := make(map[string]bool, len(envelope.Anomaly))
	for key := range envelope.Anomaly {
		present[key] = true
	}
	return present
}

func mergeAnomalyConfig(user, def AnomalyConfig, present map[string]bool) AnomalyConfig {
	// An explicit key wins even when its value is the zero value. Without
	// presence information (a Config that never came from disk) fall back to
	// filling in zero fields, which matches the historical behaviour.
	specified := func(key string) bool {
		if present == nil {
			return false
		}
		return present[key]
	}

	if !specified("enabled") && !user.Enabled {
		user.Enabled = def.Enabled
	}
	if !specified("recent_window_sec") && user.RecentWindowSec == 0 {
		user.RecentWindowSec = def.RecentWindowSec
	}
	if !specified("baseline_window_sec") && user.BaselineWindowSec == 0 {
		user.BaselineWindowSec = def.BaselineWindowSec
	}
	if !specified("multiplier") && user.Multiplier == 0 {
		user.Multiplier = def.Multiplier
	}
	if !specified("drop_multiplier") && user.DropMultiplier == 0 {
		user.DropMultiplier = def.DropMultiplier
	}
	if !specified("min_baseline") && user.MinBaseline == 0 {
		user.MinBaseline = def.MinBaseline
	}
	if !specified("highlight_window_sec") && user.HighlightWindowSec == 0 {
		user.HighlightWindowSec = def.HighlightWindowSec
	}
	if !specified("max_keys_per_dimension") && user.MaxKeysPerDimension == 0 {
		user.MaxKeysPerDimension = def.MaxKeysPerDimension
	}
	if !specified("cooldown_sec") && user.CooldownSec == 0 {
		user.CooldownSec = def.CooldownSec
	}
	if !specified("strategy") && user.Strategy == "" {
		user.Strategy = def.Strategy
	}
	if user.Dimensions == nil {
		user.Dimensions = def.Dimensions
	}
	return user
}

// Save writes the config to disk, creating directories if needed.
func Save(c Config) error {
	p := path()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := writeFileAtomic(p, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// writeFileAtomic replaces path via a temporary file in the same directory that
// is fsynced and then renamed into place.
//
// Writing the config in place risks leaving truncated JSON behind if the process
// dies mid-write; the next Load would fail to parse it and drop every saved
// favourite, preset, and search-history entry.
func writeFileAtomic(target string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(target)

	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp config: %w", err)
	}
	// Flush to disk before renaming so a crash cannot publish an empty file.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp config: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp config: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

// AddSearchHistory appends a query to the front, deduplicating and capping at 20.
func (c *Config) AddSearchHistory(query string) {
	if query == "" {
		return
	}
	// deduplicate: remove existing same query
	filtered := make([]string, 0, len(c.SearchHistory))
	for _, h := range c.SearchHistory {
		if h != query {
			filtered = append(filtered, h)
		}
	}
	c.SearchHistory = append([]string{query}, filtered...)
	if len(c.SearchHistory) > maxSearchHistory {
		c.SearchHistory = c.SearchHistory[:maxSearchHistory]
	}
}
