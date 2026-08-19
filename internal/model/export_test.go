package model

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"

	tea "charm.land/bubbletea/v2"
)

func TestWriteTextLogs(t *testing.T) {
	var out bytes.Buffer
	entries := exportTestEntries()
	if err := writeTextLogs(&out, entries); err != nil {
		t.Fatalf("writeTextLogs: %v", err)
	}
	if got, want := out.String(), "raw one\nraw two\n"; got != want {
		t.Fatalf("text export = %q, want %q", got, want)
	}
}

func TestWriteJSONLogs(t *testing.T) {
	var out bytes.Buffer
	entries := exportTestEntries()
	if err := writeJSONLogs(&out, entries); err != nil {
		t.Fatalf("writeJSONLogs: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json should decode: %v\n%s", err, out.String())
	}
	if len(decoded) != 2 {
		t.Fatalf("json entry count = %d, want 2", len(decoded))
	}
	if decoded[0]["tag"] != "NetworkManager" || decoded[1]["is_crash"] != true {
		t.Fatalf("decoded json missing expected fields: %#v", decoded)
	}
}

func TestWriteCSVLogsEscapesFields(t *testing.T) {
	var out bytes.Buffer
	entries := exportTestEntries()
	if err := writeCSVLogs(&out, entries); err != nil {
		t.Fatalf("writeCSVLogs: %v", err)
	}
	rows, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("csv should parse: %v\n%s", err, out.String())
	}
	if len(rows) != 3 {
		t.Fatalf("csv row count = %d, want 3", len(rows))
	}
	if rows[0][0] != "timestamp" || rows[0][6] != "raw" {
		t.Fatalf("unexpected csv header: %#v", rows[0])
	}
	if rows[1][5] != "request, timeout" {
		t.Fatalf("csv message field = %q, want escaped comma field", rows[1][5])
	}
}

func TestWriteNDJSONLogsOneObjectPerLine(t *testing.T) {
	var out bytes.Buffer
	entries := exportTestEntries()
	if err := writeNDJSONLogs(&out, entries); err != nil {
		t.Fatalf("writeNDJSONLogs: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson line count = %d, want 2", len(lines))
	}
	for i, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("line %d should decode as JSON: %v", i, err)
		}
	}
}

func exportTestEntries() []*logentry.Entry {
	base := time.Date(2026, 6, 25, 10, 30, 0, 123_000_000, time.UTC)
	return []*logentry.Entry{
		{
			Timestamp: base,
			PID:       100,
			TID:       101,
			Level:     logentry.LevelInfo,
			Tag:       "NetworkManager",
			Message:   "request, timeout",
			Raw:       "raw one",
		},
		{
			Timestamp: base.Add(time.Second),
			PID:       200,
			TID:       201,
			Level:     logentry.LevelError,
			Tag:       "AndroidRuntime",
			Message:   "FATAL EXCEPTION",
			Raw:       "raw two",
			IsCrash:   true,
		},
	}
}

func TestEntriesForExportScope(t *testing.T) {
	base := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.viewHeight = 2
	m.ingestEntries([]*logentry.Entry{
		testWindowEntry(base.Add(0*time.Second), "NetworkManager", "first"),
		testWindowEntry(base.Add(1*time.Second), "ActivityManager", "second"),
		testWindowEntry(base.Add(2*time.Second), "NetworkManager", "third"),
		testWindowEntry(base.Add(3*time.Second), "AndroidRuntime", "fourth"),
	})
	m.bookmarks[1] = true
	m.bookmarks[3] = true
	m.scrollOffset = 1
	m.anomaly.events = []anomaly.Event{{
		Dimension: anomaly.DimTag,
		Key:       "NetworkManager",
		LogTime:   base.Add(2 * time.Second),
	}}
	m.anomaly.highlightSec = 1

	tests := []struct {
		name  string
		scope exportScope
		want  []string
	}{
		{name: "filtered", scope: exportScopeFiltered, want: []string{"first", "second", "third", "fourth"}},
		{name: "bookmarks", scope: exportScopeBookmarks, want: []string{"second", "fourth"}},
		{name: "visible", scope: exportScopeVisible, want: []string{"second", "third"}},
		{name: "anomaly window", scope: exportScopeAnomalyWindow, want: []string{"second", "third", "fourth"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entryMessages(m.entriesForExportScope(tt.scope))
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Fatalf("scope entries = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportScopedLogsCommandWritesChosenScopeAndFormat(t *testing.T) {
	base := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.ingestEntries([]*logentry.Entry{
		testWindowEntry(base.Add(0*time.Second), "NetworkManager", "first"),
		testWindowEntry(base.Add(1*time.Second), "ActivityManager", "second"),
	})
	m.bookmarks[1] = true

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	msg := m.exportScopedLogsCmd(exportScopeBookmarks, exportFormatCSV)()
	done, ok := msg.(ExportDoneMsg)
	if !ok {
		t.Fatalf("export msg = %#v, want ExportDoneMsg", msg)
	}
	if filepath.Ext(done.Path) != ".csv" {
		t.Fatalf("export path = %q, want .csv", done.Path)
	}
	data, err := os.ReadFile(done.Path)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("csv rows = %d, want header + one bookmarked entry", len(rows))
	}
	if rows[1][5] != "second" {
		t.Fatalf("exported message = %q, want bookmarked entry", rows[1][5])
	}
}

func TestExportPanelBindingOpensPanel(t *testing.T) {
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'O', Text: "O"}))
	updated := model.(AppModel)
	if updated.inputMode != ModeExportPanel {
		t.Fatalf("inputMode = %v, want %v", updated.inputMode, ModeExportPanel)
	}
}

func TestWriteMarkdownLogs(t *testing.T) {
	var out bytes.Buffer
	entries := exportTestEntries()
	if err := writeMarkdownLogs(&out, entries); err != nil {
		t.Fatalf("writeMarkdownLogs: %v", err)
	}
	got := out.String()
	for _, want := range []string{"# Logcat 日志导出", "raw one", "raw two", "```"} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown export missing %q:\n%s", want, got)
		}
	}
}

func TestQuickExportTypesDefaultIsLog(t *testing.T) {
	types := quickExportTypes()
	want := []exportFileType{exportFileTypeTXT, exportFileTypeLOG, exportFileTypeMD}
	if len(types) != len(want) {
		t.Fatalf("quick export types = %d, want %d", len(types), len(want))
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("types[%d] = %v, want %v", i, types[i], want[i])
		}
	}
	if got, want := quickExportDefaultIndex(), 1; got != want {
		t.Fatalf("default index = %d, want %d (.log)", got, want)
	}
	if got := types[quickExportDefaultIndex()].extension(); got != "log" {
		t.Fatalf("default extension = %q, want log", got)
	}
}

func TestExportKeyOpensFormatPickerWithLogDefault(t *testing.T) {
	base := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	m := New(Options{BufferSize: 8})
	defer m.anomalyDetector.Stop()
	m.ingestEntries([]*logentry.Entry{
		testWindowEntry(base, "NetworkManager", "first"),
	})
	model, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: 'e', Text: "e"}))
	updated := model.(AppModel)
	if updated.inputMode != ModeExportFormat {
		t.Fatalf("inputMode = %v, want %v", updated.inputMode, ModeExportFormat)
	}
	if updated.exportSelection != quickExportDefaultIndex() {
		t.Fatalf("exportSelection = %d, want %d (.log)", updated.exportSelection, quickExportDefaultIndex())
	}
}

func TestExportLogsCmdWritesChosenExtension(t *testing.T) {
	entries := exportTestEntries()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	for _, ft := range quickExportTypes() {
		msg := exportLogsCmd(entries, ft)()
		done, ok := msg.(ExportDoneMsg)
		if !ok {
			t.Fatalf("export(%v) msg = %#v, want ExportDoneMsg", ft, msg)
		}
		if got := filepath.Ext(done.Path); got != ft.Label() {
			t.Fatalf("export(%v) path = %q, want %s", ft, done.Path, ft.Label())
		}
		data, err := os.ReadFile(done.Path)
		if err != nil {
			t.Fatal(err)
		}
		if ft == exportFileTypeMD && !strings.Contains(string(data), "```") {
			t.Fatalf("markdown export missing code fence:\n%s", string(data))
		}
	}
}
