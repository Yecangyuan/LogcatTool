package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type anomalyContext struct {
	Event         anomaly.Event
	WindowStart   time.Time
	WindowEnd     time.Time
	Entries       []*logentry.Entry
	LevelCounts   map[logentry.Level]int
	TagCounts     map[string]int
	ProcessCounts map[string]int
	PackageCounts map[string]int
}

func (m AppModel) selectedAnomalyContext() (anomalyContext, bool) {
	event, ok := m.selectedAnomalyEvent()
	if !ok {
		return anomalyContext{}, false
	}
	windowSec := m.selectedAnomalyWindowSec()
	if windowSec <= 0 {
		return anomalyContext{}, false
	}

	window := time.Duration(windowSec) * time.Second
	start := event.LogTime.Add(-window)
	end := event.LogTime.Add(window)
	ctx := anomalyContext{
		Event:         event,
		WindowStart:   start,
		WindowEnd:     end,
		LevelCounts:   make(map[logentry.Level]int),
		TagCounts:     make(map[string]int),
		ProcessCounts: make(map[string]int),
		PackageCounts: make(map[string]int),
	}

	m.allEntries.ForEach(func(entry *logentry.Entry) bool {
		if entry == nil || entry.Timestamp.Before(start) || entry.Timestamp.After(end) {
			return true
		}
		ctx.Entries = append(ctx.Entries, entry)
		ctx.LevelCounts[entry.Level]++
		if entry.Tag != "" {
			ctx.TagCounts[entry.Tag]++
		}
		if process := m.processNameForEntry(entry); process != "" {
			ctx.ProcessCounts[process]++
		}
		if pkg := m.packageNameForEntry(entry); pkg != "" {
			ctx.PackageCounts[pkg]++
		}
		return true
	})
	return ctx, true
}

func (m AppModel) selectedAnomalyEvent() (anomaly.Event, bool) {
	if len(m.anomaly.events) == 0 || m.anomaly.selection < 0 || m.anomaly.selection >= len(m.anomaly.events) {
		return anomaly.Event{}, false
	}
	return m.anomaly.events[m.anomaly.selection], true
}

func (m AppModel) selectedAnomalyWindowSec() int {
	if m.anomaly.highlightSec > 0 {
		return m.anomaly.highlightSec
	}
	return m.cfg.Anomaly.HighlightWindowSec
}

func (m AppModel) processNameForEntry(entry *logentry.Entry) string {
	if entry == nil {
		return ""
	}
	if process := m.processByPID[entry.PID]; process != "" {
		return process
	}
	return m.packageByPID[entry.PID]
}

func (m AppModel) packageNameForEntry(entry *logentry.Entry) string {
	if entry == nil {
		return ""
	}
	if process := m.processByPID[entry.PID]; process != "" {
		return packageNameFromProcess(process)
	}
	return m.packageByPID[entry.PID]
}

func anomalyContextSummaryLines(ctx anomalyContext) []string {
	lines := []string{
		fmt.Sprintf("  窗口: %s - %s  日志:%d",
			ctx.WindowStart.Format("15:04:05.000"),
			ctx.WindowEnd.Format("15:04:05.000"),
			len(ctx.Entries),
		),
	}
	if parts := levelCountLabels(ctx.LevelCounts, 4); len(parts) > 0 {
		lines = append(lines, "  级别: "+strings.Join(parts, "  "))
	}
	if parts := topCountLabels(ctx.TagCounts, 3); len(parts) > 0 {
		lines = append(lines, "  Tag: "+strings.Join(parts, "  "))
	}
	if parts := topCountLabels(ctx.ProcessCounts, 3); len(parts) > 0 {
		lines = append(lines, "  进程: "+strings.Join(parts, "  "))
	}
	if parts := topCountLabels(ctx.PackageCounts, 3); len(parts) > 0 {
		lines = append(lines, "  应用: "+strings.Join(parts, "  "))
	}
	return lines
}

func topCountLabels(counts map[string]int, limit int) []string {
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		if name == "" || count <= 0 {
			continue
		}
		items = append(items, item{name: name, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].name < items[j].name
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, fmt.Sprintf("%s×%d", truncateLabel(item.name, 18), item.count))
	}
	return labels
}

func levelCountLabels(counts map[logentry.Level]int, limit int) []string {
	type item struct {
		level logentry.Level
		count int
	}
	items := make([]item, 0, len(counts))
	for level, count := range counts {
		if count <= 0 {
			continue
		}
		items = append(items, item{level: level, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		return items[i].level < items[j].level
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, fmt.Sprintf("%s×%d", item.level.Label(), item.count))
	}
	return labels
}
