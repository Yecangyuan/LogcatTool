package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds user-persistent settings.
type Config struct {
	FavoritePackages  map[string]bool `json:"favorite_packages"`
	FavoriteProcesses map[string]bool `json:"favorite_processes"`
	AlertKeyword      string          `json:"alert_keyword"`
	SearchHistory     []string        `json:"search_history"`
	Presets           [3]Preset       `json:"presets"`
	CollapseDupes     bool            `json:"collapse_dupes"`
	WrapLines         bool            `json:"wrap_lines"`
	ShowDetails       bool            `json:"show_details"`
	AutoScroll        bool            `json:"auto_scroll"`
}

// Preset mirrors filterPreset for JSON serialization.
type Preset struct {
	Used      bool   `json:"used"`
	MinLevel  string `json:"min_level,omitempty"`
	Package   string `json:"package,omitempty"`
	Process   string `json:"process,omitempty"`
	Tag       string `json:"tag,omitempty"`
	TagExclude string `json:"tag_exclude,omitempty"`
	PID       int    `json:"pid,omitempty"`
	SearchText string `json:"search_text,omitempty"`
	CrashOnly bool   `json:"crash_only,omitempty"`
	TimeWindowSec int `json:"time_window_sec,omitempty"`
}

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
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
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
	if err := os.WriteFile(p, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
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
	if len(c.SearchHistory) > 20 {
		c.SearchHistory = c.SearchHistory[:20]
	}
}
