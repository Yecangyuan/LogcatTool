package model

import (
	"os"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func benchmarkModel(b *testing.B, capacity int) AppModel {
	b.Helper()
	m := New(Options{BufferSize: capacity})
	m.width = 160
	m.height = 48
	m.viewHeight = 40
	b.Cleanup(func() {
		if m.anomalyDetector != nil {
			m.anomalyDetector.Stop()
		}
	})
	return m
}

func benchmarkEntries(n int, base time.Time) []*logentry.Entry {
	entries := make([]*logentry.Entry, n)
	levels := []logentry.Level{logentry.LevelVerbose, logentry.LevelDebug, logentry.LevelInfo, logentry.LevelWarn, logentry.LevelError}
	tags := []string{"NetworkManager", "ActivityManager", "RenderThread", "SQLite", "AndroidRuntime"}
	for i := 0; i < n; i++ {
		tag := tags[i%len(tags)]
		level := levels[i%len(levels)]
		msg := "background sync completed"
		if i%11 == 0 {
			msg = "request timeout while syncing account metadata"
		}
		entries[i] = &logentry.Entry{
			Timestamp: base.Add(time.Duration(i) * time.Millisecond),
			PID:       1000 + i%64,
			TID:       2000 + i%128,
			Level:     level,
			Tag:       tag,
			Message:   msg,
			Raw:       tag + ": " + msg,
		}
	}
	return entries
}

func BenchmarkAppModelRefilterSearch(b *testing.B) {
	m := benchmarkModel(b, 50_000)
	m.ingestEntries(benchmarkEntries(50_000, time.Unix(1_800_000_000, 0)))
	m.filter.SetSearch("timeout", false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.refilter()
	}
}

func BenchmarkAppModelIngestTimeWindow(b *testing.B) {
	m := benchmarkModel(b, 50_000)
	base := time.Unix(1_800_000_000, 0)
	m.ingestEntries(benchmarkEntries(10_000, base))
	m.filter.TimeWindow = 2 * time.Second
	m.refilter()

	batches := make([][]*logentry.Entry, b.N)
	for i := 0; i < b.N; i++ {
		batches[i] = benchmarkEntries(50, base.Add(20*time.Second+time.Duration(i)*time.Second))
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.ingestEntries(batches[i])
	}
}

func BenchmarkExportTextCommand(b *testing.B) {
	entries := benchmarkEntries(2000, time.Unix(1_800_000_000, 0))
	withBenchmarkWorkingDir(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if msg := exportLogsCmd(entries)(); isLogError(msg) {
			b.Fatalf("export failed: %v", msg)
		}
	}
}

func BenchmarkExportJSONCommand(b *testing.B) {
	entries := benchmarkEntries(2000, time.Unix(1_800_000_000, 0))
	withBenchmarkWorkingDir(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if msg := exportJSONCmd(entries)(); isLogError(msg) {
			b.Fatalf("export failed: %v", msg)
		}
	}
}

func isLogError(msg any) bool {
	_, ok := msg.(LogErrorMsg)
	return ok
}

func withBenchmarkWorkingDir(b *testing.B) {
	b.Helper()
	wd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	tmp := b.TempDir()
	if err := os.Chdir(tmp); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}
