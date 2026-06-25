package model

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestSelectedAnomalyContextCollectsEntriesAndSummaries(t *testing.T) {
	m, _ := anomalyContextTestModel(t)

	ctx, ok := m.selectedAnomalyContext()
	if !ok {
		t.Fatal("expected selected anomaly context")
	}
	got := entryMessages(ctx.Entries)
	want := []string{"network warning", "network error", "db error"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("context entries = %v, want %v", got, want)
	}
	if ctx.LevelCounts[logentry.LevelError] != 2 {
		t.Fatalf("error count = %d, want 2", ctx.LevelCounts[logentry.LevelError])
	}
	if ctx.TagCounts["Network"] != 2 {
		t.Fatalf("Network tag count = %d, want 2", ctx.TagCounts["Network"])
	}
	if ctx.PackageCounts["com.example.app"] != 3 {
		t.Fatalf("package count = %d, want 3", ctx.PackageCounts["com.example.app"])
	}
	if ctx.ProcessCounts["com.example.app"] != 2 {
		t.Fatalf("process count = %d, want 2", ctx.ProcessCounts["com.example.app"])
	}
}

func TestAnomalyPanelRendersSelectedContextSummary(t *testing.T) {
	m, _ := anomalyContextTestModel(t)
	m.width = 100
	m.height = 40

	panel := m.overlayAnomalyPanel("")
	for _, want := range []string{"上下文", "Network×2", "com.example.app×3"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel missing %q:\n%s", want, panel)
		}
	}
}

func TestExportSelectedAnomalyContextIncludesMetadataAndEntries(t *testing.T) {
	m, _ := anomalyContextTestModel(t)
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

	msg := m.exportSelectedAnomalyContextCmd()()
	done, ok := msg.(ExportDoneMsg)
	if !ok {
		t.Fatalf("export msg = %#v, want ExportDoneMsg", msg)
	}
	data, err := os.ReadFile(done.Path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Event   map[string]any   `json:"event"`
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("context export should decode: %v\n%s", err, string(data))
	}
	if decoded.Event["key"] != "Network" {
		t.Fatalf("event key = %#v, want Network", decoded.Event["key"])
	}
	if len(decoded.Entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(decoded.Entries))
	}
}

func anomalyContextTestModel(t *testing.T) (AppModel, time.Time) {
	t.Helper()
	base := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	m := New(Options{BufferSize: 8})
	t.Cleanup(func() {
		m.anomalyDetector.Stop()
	})
	m.packageByPID = map[int]string{
		1001: "com.example.app",
		1002: "com.example.app",
	}
	m.processByPID = map[int]string{
		1001: "com.example.app:remote",
		1002: "com.example.app",
	}
	m.ingestEntries([]*logentry.Entry{
		anomalyContextEntry(base.Add(0*time.Second), 1001, logentry.LevelInfo, "Noise", "noise"),
		anomalyContextEntry(base.Add(1*time.Second), 1001, logentry.LevelWarn, "Network", "network warning"),
		anomalyContextEntry(base.Add(2*time.Second), 1002, logentry.LevelError, "Network", "network error"),
		anomalyContextEntry(base.Add(3*time.Second), 1002, logentry.LevelError, "DB", "db error"),
		anomalyContextEntry(base.Add(4*time.Second), 1002, logentry.LevelInfo, "Noise", "later noise"),
	})
	m.anomaly.events = []anomaly.Event{{
		Dimension:    anomaly.DimTag,
		Key:          "Network",
		Direction:    anomaly.DirectionSpike,
		RecentRate:   12,
		BaselineRate: 3,
		Ratio:        4,
		LogTime:      base.Add(2 * time.Second),
		TriggeredAt:  base.Add(2 * time.Second),
	}}
	m.anomaly.highlightSec = 1
	return m, base
}

func anomalyContextEntry(ts time.Time, pid int, level logentry.Level, tag, msg string) *logentry.Entry {
	return &logentry.Entry{
		Timestamp: ts,
		PID:       pid,
		TID:       pid + 100,
		Level:     level,
		Tag:       tag,
		Message:   msg,
		Raw:       tag + ": " + msg,
	}
}
