package logentry

import (
	"regexp"
	"strings"
)

type Filter struct {
	MinLevel   Level
	Tag        string
	PID        int
	Package    string
	SearchText string
	SearchRe   *regexp.Regexp
	IsRegex    bool
	PIDsByPkg  map[string][]int // package name -> PIDs mapping
	Levels     map[Level]bool   // deprecated: kept for compatibility, unused
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

	if f.PID > 0 && e.PID != f.PID {
		return false
	}

	if f.Package != "" && !f.matchPackage(e.PID) {
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
	pids, ok := f.PIDsByPkg[f.Package]
	if !ok {
		return false
	}
	for _, p := range pids {
		if p == pid {
			return true
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
	if f.Tag != "" || f.PID > 0 || f.Package != "" {
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
