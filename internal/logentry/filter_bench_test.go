package logentry

import (
	"fmt"
	"testing"
	"time"
)

func benchmarkFilterEntry() *Entry {
	return &Entry{
		Timestamp: time.Unix(1_800_000_000, 0),
		PID:       4242,
		TID:       4243,
		Level:     LevelInfo,
		Tag:       "NetworkManager",
		Message:   "Request timeout while syncing account metadata",
		Raw:       "raw",
	}
}

func BenchmarkFilterPlainSearchMatch(b *testing.B) {
	entry := benchmarkFilterEntry()
	f := NewFilter()
	f.SetSearch("TIMEOUT", false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !f.Match(entry) {
			b.Fatal("expected search to match")
		}
	}
}

func BenchmarkFilterTagMatch(b *testing.B) {
	entry := benchmarkFilterEntry()
	f := NewFilter()
	f.Tag = "network"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !f.Match(entry) {
			b.Fatal("expected tag to match")
		}
	}
}

func BenchmarkFilterPackagePIDMatch(b *testing.B) {
	entry := benchmarkFilterEntry()
	f := NewFilter()
	f.Package = "com.example.app"
	f.SetPIDsByPkg(benchmarkPIDMap(1000, "com.example.app", entry.PID))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !f.Match(entry) {
			b.Fatal("expected package to match")
		}
	}
}

func BenchmarkFilterProcessPIDMatch(b *testing.B) {
	entry := benchmarkFilterEntry()
	f := NewFilter()
	f.Process = "remote"
	f.SetPIDsByPkg(benchmarkPIDMap(1000, "com.example.app:remote", entry.PID))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !f.Match(entry) {
			b.Fatal("expected process to match")
		}
	}
}

func benchmarkPIDMap(size int, matchingName string, matchingPID int) map[string][]int {
	pids := make(map[string][]int, size)
	for i := 0; i < size-1; i++ {
		pids[fmt.Sprintf("com.example.noise%d", i)] = []int{10_000 + i}
	}
	pids[matchingName] = []int{matchingPID}
	return pids
}
