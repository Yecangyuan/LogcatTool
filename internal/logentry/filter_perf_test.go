package logentry

import (
	"testing"
	"time"
)

func TestFilterPlainMatchingDoesNotAllocatePerEntry(t *testing.T) {
	entry := &Entry{
		Timestamp: time.Unix(1_800_000_000, 0),
		PID:       42,
		TID:       43,
		Level:     LevelInfo,
		Tag:       "NetworkManager",
		Message:   "Request timeout while syncing account metadata",
	}

	t.Run("search text", func(t *testing.T) {
		f := NewFilter()
		f.SetSearch("TIMEOUT", false)
		if !f.Match(entry) {
			t.Fatal("sanity check: search should match")
		}
		allocs := testing.AllocsPerRun(1000, func() {
			if !f.Match(entry) {
				t.Fatal("search should match")
			}
		})
		if allocs != 0 {
			t.Fatalf("plain search match allocated %.0f times per entry, want 0", allocs)
		}
	})

	t.Run("tag text", func(t *testing.T) {
		f := NewFilter()
		f.Tag = "network"
		if !f.Match(entry) {
			t.Fatal("sanity check: tag should match")
		}
		allocs := testing.AllocsPerRun(1000, func() {
			if !f.Match(entry) {
				t.Fatal("tag should match")
			}
		})
		if allocs != 0 {
			t.Fatalf("tag match allocated %.0f times per entry, want 0", allocs)
		}
	})
}

func TestFilterSetPIDsByPkgBuildsReusableIndexes(t *testing.T) {
	entry := &Entry{
		Timestamp: time.Unix(1_800_000_000, 0),
		PID:       4242,
		TID:       43,
		Level:     LevelInfo,
		Tag:       "NetworkManager",
		Message:   "Request timeout",
	}

	t.Run("package includes subprocess pids", func(t *testing.T) {
		f := NewFilter()
		f.Package = "com.example.app"
		f.SetPIDsByPkg(map[string][]int{
			"com.example.app":        {1111},
			"com.example.app:remote": {entry.PID},
			"com.example.noise":      {9999},
		})
		if !f.Match(entry) {
			t.Fatal("package filter should match subprocess PID")
		}
		if _, ok := f.packagePIDSet[entry.PID]; !ok {
			t.Fatal("package PID index should contain matching subprocess PID")
		}
	})

	t.Run("process fragment builds matching pid set", func(t *testing.T) {
		f := NewFilter()
		f.Process = "REMOTE"
		f.SetPIDsByPkg(map[string][]int{
			"com.example.app":        {1111},
			"com.example.app:remote": {entry.PID},
			"com.example.noise":      {9999},
		})
		if !f.Match(entry) {
			t.Fatal("process filter should match PID by case-insensitive fragment")
		}
		if _, ok := f.processPIDSet[entry.PID]; !ok {
			t.Fatal("process PID index should contain matching process PID")
		}
	})
}
