package logentry

import (
	"regexp"
	"strings"
	"time"
)

type Filter struct {
	MinLevel      Level
	Tag           string
	TagExclude    string
	PID           int
	Package       string
	Process       string
	CrashOnly     bool
	TimeWindow    time.Duration
	ReferenceTime time.Time
	SearchText    string
	SearchRe      *regexp.Regexp
	IsRegex       bool
	PIDsByPkg     map[string][]int // package/process name -> PIDs mapping
	Levels        map[Level]bool   // deprecated: kept for compatibility, unused
}

type Snapshot struct {
	MinLevel   Level
	Tag        string
	TagExclude string
	PID        int
	Package    string
	Process    string
	CrashOnly  bool
	TimeWindow time.Duration
	SearchText string
	IsRegex    bool
}

func NewFilter() *Filter {
	return &Filter{
		MinLevel: LevelVerbose,
	}
}

func (f *Filter) Match(e *Entry) bool {
	if e == nil {
		return false
	}

	if !f.matchLevel(e.Level) {
		return false
	}

	if f.Tag != "" && !containsFold(e.Tag, f.Tag) {
		return false
	}

	if f.TagExclude != "" && containsFold(e.Tag, f.TagExclude) {
		return false
	}

	if f.PID > 0 && e.PID != f.PID {
		return false
	}

	if f.Package != "" && !f.matchPackage(e.PID) {
		return false
	}

	if f.Process != "" && !f.matchProcess(e.PID) {
		return false
	}

	if f.CrashOnly && !e.IsCrash {
		return false
	}

	if !f.matchTime(e.Timestamp) {
		return false
	}

	if !f.matchSearch(e) {
		return false
	}

	return true
}

func (f *Filter) matchLevel(level Level) bool {
	return level >= f.MinLevel
}

func (f *Filter) matchPackage(pid int) bool {
	if len(f.PIDsByPkg) == 0 || f.Package == "" {
		return true
	}
	for name, pids := range f.PIDsByPkg {
		if name != f.Package && !strings.HasPrefix(name, f.Package+":") {
			continue
		}
		for _, p := range pids {
			if p == pid {
				return true
			}
		}
	}
	return false
}

func (f *Filter) matchProcess(pid int) bool {
	if len(f.PIDsByPkg) == 0 || f.Process == "" {
		return true
	}
	for name, pids := range f.PIDsByPkg {
		if !containsFold(name, f.Process) {
			continue
		}
		for _, p := range pids {
			if p == pid {
				return true
			}
		}
	}
	return false
}

func (f *Filter) matchSearch(e *Entry) bool {
	if f.SearchText == "" && f.SearchRe == nil {
		return true
	}

	if f.IsRegex && f.SearchRe != nil {
		return f.SearchRe.MatchString(e.Tag) ||
			f.SearchRe.MatchString(e.Message)
	}

	if f.SearchText != "" {
		return containsFold(e.Tag, f.SearchText) ||
			containsFold(e.Message, f.SearchText)
	}

	return true
}

func (f *Filter) matchTime(ts time.Time) bool {
	if f.TimeWindow <= 0 {
		return true
	}
	ref := f.ReferenceTime
	if ref.IsZero() {
		ref = time.Now()
	}
	return !ts.Before(ref.Add(-f.TimeWindow))
}

func (f *Filter) SetSearch(text string, isRegex bool) {
	f.IsRegex = isRegex
	f.SearchText = text
	f.SearchRe = nil

	if isRegex && text != "" {
		if re, err := regexp.Compile("(?i)" + text); err == nil {
			f.SearchRe = re
		}
	}
}

func (f *Filter) SetMinLevel(level Level) {
	f.MinLevel = level
	f.Levels = nil
}

func (f *Filter) IsLevelEnabled(level Level) bool {
	return level >= f.MinLevel
}

func (f *Filter) IsActive() bool {
	if f.Tag != "" || f.TagExclude != "" || f.PID > 0 || f.Package != "" || f.Process != "" || f.CrashOnly || f.TimeWindow > 0 {
		return true
	}
	if f.SearchText != "" || f.SearchRe != nil {
		return true
	}
	if f.MinLevel > LevelVerbose {
		return true
	}
	return false
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// ApplyAll filters a slice of entries, returning only matching ones.
func (f *Filter) ApplyAll(entries []*Entry) []*Entry {
	if !f.IsActive() {
		return entries
	}
	var result []*Entry
	for _, e := range entries {
		if f.Match(e) {
			result = append(result, e)
		}
	}
	return result
}

func (f *Filter) Snapshot() Snapshot {
	return Snapshot{
		MinLevel:   f.MinLevel,
		Tag:        f.Tag,
		TagExclude: f.TagExclude,
		PID:        f.PID,
		Package:    f.Package,
		Process:    f.Process,
		CrashOnly:  f.CrashOnly,
		TimeWindow: f.TimeWindow,
		SearchText: f.SearchText,
		IsRegex:    f.IsRegex,
	}
}

func (f *Filter) ApplySnapshot(s Snapshot) {
	f.MinLevel = s.MinLevel
	f.Tag = s.Tag
	f.TagExclude = s.TagExclude
	f.PID = s.PID
	f.Package = s.Package
	f.Process = s.Process
	f.CrashOnly = s.CrashOnly
	f.TimeWindow = s.TimeWindow
	f.SetSearch(s.SearchText, s.IsRegex)
}
