package logentry

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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

	pidIndexesEnabled bool
	packagePIDFilter  string
	packagePIDSet     map[int]struct{}
	processPIDFilter  string
	processPIDSet     map[int]struct{}
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
	if f.pidIndexesEnabled {
		if f.packagePIDSet == nil || f.packagePIDFilter != f.Package {
			f.rebuildPackagePIDSet()
		}
		_, ok := f.packagePIDSet[pid]
		return ok
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
	if f.pidIndexesEnabled {
		if f.processPIDSet == nil || f.processPIDFilter != f.Process {
			f.rebuildProcessPIDSet()
		}
		_, ok := f.processPIDSet[pid]
		return ok
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

func (f *Filter) SetPIDsByPkg(pids map[string][]int) {
	f.PIDsByPkg = pids
	f.pidIndexesEnabled = true
	f.packagePIDFilter = ""
	f.packagePIDSet = nil
	f.processPIDFilter = ""
	f.processPIDSet = nil
}

func (f *Filter) rebuildPackagePIDSet() {
	set := make(map[int]struct{})
	for name, pids := range f.PIDsByPkg {
		if name != f.Package && !strings.HasPrefix(name, f.Package+":") {
			continue
		}
		for _, pid := range pids {
			set[pid] = struct{}{}
		}
	}
	f.packagePIDFilter = f.Package
	f.packagePIDSet = set
}

func (f *Filter) rebuildProcessPIDSet() {
	set := make(map[int]struct{})
	for name, pids := range f.PIDsByPkg {
		if !containsFold(name, f.Process) {
			continue
		}
		for _, pid := range pids {
			set[pid] = struct{}{}
		}
	}
	f.processPIDFilter = f.Process
	f.processPIDSet = set
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
	if substr == "" {
		return true
	}
	if isASCIIString(s) && isASCIIString(substr) {
		return containsFoldASCII(s, substr)
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func containsFoldASCII(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	first := asciiLower(substr[0])
	for i := 0; i <= len(s)-len(substr); i++ {
		if asciiLower(s[i]) != first {
			continue
		}
		matched := true
		for j := 1; j < len(substr); j++ {
			if asciiLower(s[i+j]) != asciiLower(substr[j]) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func isASCIIString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func asciiLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
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
