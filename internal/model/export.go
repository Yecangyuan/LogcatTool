package model

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type exportScope int

const (
	exportScopeFiltered exportScope = iota
	exportScopeBookmarks
	exportScopeVisible
	exportScopeAnomalyWindow
)

type exportFormat int

const (
	exportFormatText exportFormat = iota
	exportFormatJSON
	exportFormatCSV
	exportFormatNDJSON
)

type exportOption struct {
	Scope  exportScope
	Format exportFormat
}

type exportedLogEntry struct {
	Timestamp string `json:"timestamp"`
	PID       int    `json:"pid"`
	TID       int    `json:"tid"`
	Level     string `json:"level"`
	Tag       string `json:"tag"`
	Message   string `json:"message"`
	Raw       string `json:"raw"`
	IsCrash   bool   `json:"is_crash"`
}

func exportOptions() []exportOption {
	scopes := []exportScope{exportScopeFiltered, exportScopeBookmarks, exportScopeVisible, exportScopeAnomalyWindow}
	formats := []exportFormat{exportFormatText, exportFormatJSON, exportFormatCSV, exportFormatNDJSON}
	options := make([]exportOption, 0, len(scopes)*len(formats))
	for _, scope := range scopes {
		for _, format := range formats {
			options = append(options, exportOption{Scope: scope, Format: format})
		}
	}
	return options
}

func (s exportScope) Label() string {
	switch s {
	case exportScopeBookmarks:
		return "书签"
	case exportScopeVisible:
		return "当前可见"
	case exportScopeAnomalyWindow:
		return "异常窗口"
	default:
		return "当前过滤"
	}
}

func (s exportScope) filenamePart() string {
	switch s {
	case exportScopeBookmarks:
		return "bookmarks"
	case exportScopeVisible:
		return "visible"
	case exportScopeAnomalyWindow:
		return "anomaly"
	default:
		return "filtered"
	}
}

func (f exportFormat) Label() string {
	switch f {
	case exportFormatJSON:
		return "JSON"
	case exportFormatCSV:
		return "CSV"
	case exportFormatNDJSON:
		return "NDJSON"
	default:
		return "Text"
	}
}

func (f exportFormat) extension() string {
	switch f {
	case exportFormatJSON:
		return "json"
	case exportFormatCSV:
		return "csv"
	case exportFormatNDJSON:
		return "ndjson"
	default:
		return "txt"
	}
}

// exportFileType 表示 e 键快速导出时可选的文件类型（扩展名）。
type exportFileType int

const (
	exportFileTypeTXT exportFileType = iota
	exportFileTypeLOG
	exportFileTypeMD
)

func (f exportFileType) Label() string {
	switch f {
	case exportFileTypeLOG:
		return ".log"
	case exportFileTypeMD:
		return ".md"
	default:
		return ".txt"
	}
}

func (f exportFileType) extension() string {
	switch f {
	case exportFileTypeLOG:
		return "log"
	case exportFileTypeMD:
		return "md"
	default:
		return "txt"
	}
}

// quickExportTypes 返回 e 键快速导出可选的文件类型，按展示顺序排列。
func quickExportTypes() []exportFileType {
	return []exportFileType{exportFileTypeTXT, exportFileTypeLOG, exportFileTypeMD}
}

// quickExportDefaultIndex 返回快速导出默认选中的文件类型下标（默认 .log）。
func quickExportDefaultIndex() int {
	for i, ft := range quickExportTypes() {
		if ft == exportFileTypeLOG {
			return i
		}
	}
	return 0
}

func writeTextLogs(w io.Writer, entries []*logentry.Entry) error {
	for _, e := range entries {
		if _, err := io.WriteString(w, e.Raw); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeMarkdownLogs(w io.Writer, entries []*logentry.Entry) error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Logcat 日志导出\n\n")
	fmt.Fprintf(&sb, "- 导出时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "- 日志条数: %d\n\n", len(entries))
	sb.WriteString("```\n")
	for _, e := range entries {
		sb.WriteString(e.Raw)
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")
	_, err := io.WriteString(w, sb.String())
	return err
}

func writeJSONLogs(w io.Writer, entries []*logentry.Entry) error {
	data, err := json.MarshalIndent(exportLogEntries(entries), "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func writeCSVLogs(w io.Writer, entries []*logentry.Entry) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"timestamp", "pid", "tid", "level", "tag", "message", "raw", "is_crash"}); err != nil {
		return err
	}
	for _, e := range entries {
		if err := cw.Write([]string{
			e.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
			strconv.Itoa(e.PID),
			strconv.Itoa(e.TID),
			e.Level.Char(),
			e.Tag,
			e.Message,
			e.Raw,
			strconv.FormatBool(e.IsCrash),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeNDJSONLogs(w io.Writer, entries []*logentry.Entry) error {
	enc := json.NewEncoder(w)
	for _, e := range exportLogEntries(entries) {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func writeLogsByFormat(w io.Writer, entries []*logentry.Entry, format exportFormat) error {
	switch format {
	case exportFormatJSON:
		return writeJSONLogs(w, entries)
	case exportFormatCSV:
		return writeCSVLogs(w, entries)
	case exportFormatNDJSON:
		return writeNDJSONLogs(w, entries)
	default:
		return writeTextLogs(w, entries)
	}
}

func (m AppModel) exportScopedLogsCmd(scope exportScope, format exportFormat) tea.Cmd {
	return func() tea.Msg {
		entries := m.entriesForExportScope(scope)
		filename := fmt.Sprintf("logcat_%s_%s.%s",
			scope.filenamePart(),
			time.Now().Format("20060102_150405"),
			format.extension(),
		)
		f, err := os.Create(filename)
		if err != nil {
			return LogErrorMsg{Err: fmt.Errorf("导出失败: %w", err)}
		}
		defer f.Close()
		if err := writeLogsByFormat(f, entries, format); err != nil {
			return LogErrorMsg{Err: fmt.Errorf("导出失败: %w", err)}
		}
		return ExportDoneMsg{Path: filename}
	}
}

func (m AppModel) entriesForExportScope(scope exportScope) []*logentry.Entry {
	switch scope {
	case exportScopeBookmarks:
		return m.bookmarkedExportEntries()
	case exportScopeVisible:
		return m.visibleExportEntries()
	case exportScopeAnomalyWindow:
		return m.anomalyWindowExportEntries()
	default:
		return append([]*logentry.Entry(nil), m.filtered...)
	}
}

func (m AppModel) bookmarkedExportEntries() []*logentry.Entry {
	if len(m.bookmarks) == 0 {
		return nil
	}
	entries := make([]*logentry.Entry, 0, len(m.bookmarks))
	m.allEntries.ForEach(func(entry *logentry.Entry) bool {
		if entry != nil && m.bookmarks[entry.Index] {
			entries = append(entries, entry)
		}
		return true
	})
	return entries
}

func (m AppModel) visibleExportEntries() []*logentry.Entry {
	if len(m.displayRows) == 0 || m.viewHeight <= 0 {
		return nil
	}
	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(m.displayRows) {
		return nil
	}
	end := start + m.viewHeight
	if end > len(m.displayRows) {
		end = len(m.displayRows)
	}
	entries := make([]*logentry.Entry, 0, end-start)
	for _, row := range m.displayRows[start:end] {
		if row.Entry != nil {
			entries = append(entries, row.Entry)
		}
	}
	return entries
}

func (m AppModel) anomalyWindowExportEntries() []*logentry.Entry {
	ctx, ok := m.selectedAnomalyContext()
	if !ok {
		return nil
	}
	return ctx.Entries
}

func exportLogEntries(entries []*logentry.Entry) []exportedLogEntry {
	out := make([]exportedLogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, exportedLogEntry{
			Timestamp: e.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
			PID:       e.PID,
			TID:       e.TID,
			Level:     e.Level.Char(),
			Tag:       e.Tag,
			Message:   e.Message,
			Raw:       e.Raw,
			IsCrash:   e.IsCrash,
		})
	}
	return out
}

type exportedAnomalyEvent struct {
	Dimension    string  `json:"dimension"`
	Key          string  `json:"key"`
	Direction    string  `json:"direction"`
	RecentRate   float64 `json:"recent_rate"`
	BaselineRate float64 `json:"baseline_rate"`
	Ratio        float64 `json:"ratio"`
	TriggeredAt  string  `json:"triggered_at"`
	LogTime      string  `json:"log_time"`
}

type exportedAnomalyWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type exportedAnomalyCounts struct {
	Levels    map[string]int `json:"levels"`
	Tags      map[string]int `json:"tags"`
	Processes map[string]int `json:"processes"`
	Packages  map[string]int `json:"packages"`
}

type exportedAnomalyContext struct {
	Event   exportedAnomalyEvent  `json:"event"`
	Window  exportedAnomalyWindow `json:"window"`
	Counts  exportedAnomalyCounts `json:"counts"`
	Entries []exportedLogEntry    `json:"entries"`
}

func (m AppModel) exportSelectedAnomalyContextCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, ok := m.selectedAnomalyContext()
		if !ok {
			return LogErrorMsg{Err: fmt.Errorf("没有异常上下文可导出")}
		}
		filename := fmt.Sprintf("logcat_anomaly_context_%s.json", time.Now().Format("20060102_150405"))
		f, err := os.Create(filename)
		if err != nil {
			return LogErrorMsg{Err: fmt.Errorf("导出失败: %w", err)}
		}
		defer f.Close()
		if err := writeAnomalyContextJSON(f, ctx); err != nil {
			return LogErrorMsg{Err: fmt.Errorf("导出失败: %w", err)}
		}
		return ExportDoneMsg{Path: filename}
	}
}

func writeAnomalyContextJSON(w io.Writer, ctx anomalyContext) error {
	data, err := json.MarshalIndent(exportAnomalyContext(ctx), "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func exportAnomalyContext(ctx anomalyContext) exportedAnomalyContext {
	return exportedAnomalyContext{
		Event: exportedAnomalyEvent{
			Dimension:    ctx.Event.Dimension.String(),
			Key:          ctx.Event.Key,
			Direction:    ctx.Event.Direction.String(),
			RecentRate:   ctx.Event.RecentRate,
			BaselineRate: ctx.Event.BaselineRate,
			Ratio:        ctx.Event.Ratio,
			TriggeredAt:  ctx.Event.TriggeredAt.Format("2006-01-02T15:04:05.000Z07:00"),
			LogTime:      ctx.Event.LogTime.Format("2006-01-02T15:04:05.000Z07:00"),
		},
		Window: exportedAnomalyWindow{
			Start: ctx.WindowStart.Format("2006-01-02T15:04:05.000Z07:00"),
			End:   ctx.WindowEnd.Format("2006-01-02T15:04:05.000Z07:00"),
		},
		Counts: exportedAnomalyCounts{
			Levels:    exportLevelCounts(ctx.LevelCounts),
			Tags:      copyStringCounts(ctx.TagCounts),
			Processes: copyStringCounts(ctx.ProcessCounts),
			Packages:  copyStringCounts(ctx.PackageCounts),
		},
		Entries: exportLogEntries(ctx.Entries),
	}
}

func exportLevelCounts(counts map[logentry.Level]int) map[string]int {
	out := make(map[string]int, len(counts))
	for level, count := range counts {
		if count > 0 {
			out[level.Label()] = count
		}
	}
	return out
}

func copyStringCounts(counts map[string]int) map[string]int {
	out := make(map[string]int, len(counts))
	for key, count := range counts {
		if key != "" && count > 0 {
			out[key] = count
		}
	}
	return out
}
