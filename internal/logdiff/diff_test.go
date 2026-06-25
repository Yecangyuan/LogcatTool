package logdiff

import (
	"strings"
	"testing"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

func TestCompareCapturesDetectsNewErrorSignatures(t *testing.T) {
	base := Capture{
		Entries: []*logentry.Entry{
			diffEntry(1001, logentry.LevelError, "Network", "timeout"),
		},
		ProcessByPID: map[int]string{1001: "com.example.app"},
		PackageByPID: map[int]string{1001: "com.example.app"},
	}
	candidate := Capture{
		Entries: []*logentry.Entry{
			diffEntry(1001, logentry.LevelError, "Network", "timeout"),
			diffEntry(1002, logentry.LevelFatal, "Auth", "token crash"),
		},
		ProcessByPID: map[int]string{
			1001: "com.example.app",
			1002: "com.example.auth:login",
		},
		PackageByPID: map[int]string{
			1001: "com.example.app",
			1002: "com.example.auth",
		},
	}

	report := CompareCaptures(base, candidate)
	if len(report.NewErrors) != 1 {
		t.Fatalf("new error count = %d, want 1: %#v", len(report.NewErrors), report.NewErrors)
	}
	sig := report.NewErrors[0]
	if sig.Level != "F" || sig.Tag != "Auth" || sig.Message != "token crash" {
		t.Fatalf("new signature mismatch: %#v", sig)
	}
	if sig.Process != "com.example.auth:login" || sig.Package != "com.example.auth" {
		t.Fatalf("signature context mismatch: %#v", sig)
	}
}

func TestCompareCapturesReportsChangedTagProcessAndPackageCounts(t *testing.T) {
	base := Capture{
		Entries: []*logentry.Entry{
			diffEntry(1001, logentry.LevelInfo, "Network", "connected"),
			diffEntry(1002, logentry.LevelWarn, "UI", "slow frame"),
		},
		ProcessByPID: map[int]string{
			1001: "com.example.app",
			1002: "com.example.app:ui",
		},
		PackageByPID: map[int]string{
			1001: "com.example.app",
			1002: "com.example.app",
		},
	}
	candidate := Capture{
		Entries: []*logentry.Entry{
			diffEntry(1001, logentry.LevelInfo, "Network", "connected"),
			diffEntry(1001, logentry.LevelInfo, "Network", "retry"),
			diffEntry(1003, logentry.LevelInfo, "Network", "fallback"),
		},
		ProcessByPID: map[int]string{
			1001: "com.example.app",
			1003: "com.example.sync",
		},
		PackageByPID: map[int]string{
			1001: "com.example.app",
			1003: "com.example.sync",
		},
	}

	report := CompareCaptures(base, candidate)
	assertDelta(t, report.Tags, "Network", 1, 3)
	assertDelta(t, report.Processes, "com.example.sync", 0, 1)
	assertDelta(t, report.Packages, "com.example.sync", 0, 1)

	out := FormatReport(report)
	for _, want := range []string{"New error/fatal signatures", "Changed tags", "Network: 1 -> 3 (+2)", "com.example.sync"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report output missing %q:\n%s", want, out)
		}
	}
}

func assertDelta(t *testing.T, deltas []CountDelta, name string, base, candidate int) {
	t.Helper()
	for _, delta := range deltas {
		if delta.Name == name {
			if delta.Baseline != base || delta.Candidate != candidate {
				t.Fatalf("%s delta = %#v, want %d -> %d", name, delta, base, candidate)
			}
			return
		}
	}
	t.Fatalf("missing delta for %s in %#v", name, deltas)
}

func diffEntry(pid int, level logentry.Level, tag, msg string) *logentry.Entry {
	return &logentry.Entry{
		Timestamp: time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC),
		PID:       pid,
		TID:       pid,
		Level:     level,
		Tag:       tag,
		Message:   msg,
		Raw:       tag + ": " + msg,
	}
}
