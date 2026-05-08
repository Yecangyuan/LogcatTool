package logentry

import (
	"testing"
	"time"
)

func TestParseThreadtime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantPID int
		wantTID int
		wantLvl Level
		wantTag string
		wantMsg string
	}{
		{
			name:    "standard debug line",
			input:   "04-09 10:42:01.234  1234  5678 D MyTag   : Some debug message",
			wantPID: 1234,
			wantTID: 5678,
			wantLvl: LevelDebug,
			wantTag: "MyTag",
			wantMsg: "Some debug message",
		},
		{
			name:    "error line",
			input:   "04-09 10:42:01.234  1234  5678 E AndroidRuntime: FATAL EXCEPTION: main",
			wantPID: 1234,
			wantTID: 5678,
			wantLvl: LevelError,
			wantTag: "AndroidRuntime",
			wantMsg: "FATAL EXCEPTION: main",
		},
		{
			name:    "info with spaces in message",
			input:   "01-01 00:00:00.000     1     1 I Tag     : message with spaces",
			wantPID: 1,
			wantTID: 1,
			wantLvl: LevelInfo,
			wantTag: "Tag",
			wantMsg: "message with spaces",
		},
		{
			name:    "not a log line",
			input:   "--------- beginning of main",
			wantNil: true,
		},
		{
			name:    "empty line",
			input:   "",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := Parse(tt.input)
			if tt.wantNil {
				if entry != nil {
					t.Errorf("expected nil, got %+v", entry)
				}
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			if entry.PID != tt.wantPID {
				t.Errorf("PID = %d, want %d", entry.PID, tt.wantPID)
			}
			if entry.TID != tt.wantTID {
				t.Errorf("TID = %d, want %d", entry.TID, tt.wantTID)
			}
			if entry.Level != tt.wantLvl {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLvl)
			}
			if entry.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", entry.Tag, tt.wantTag)
			}
			if entry.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", entry.Message, tt.wantMsg)
			}
			if entry.Timestamp.IsZero() {
				t.Error("Timestamp should not be zero")
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input byte
		want  Level
	}{
		{'V', LevelVerbose},
		{'D', LevelDebug},
		{'I', LevelInfo},
		{'W', LevelWarn},
		{'E', LevelError},
		{'F', LevelFatal},
		{'S', LevelSilent},
		{'X', LevelUnknown},
	}
	for _, tt := range tests {
		got := ParseLevel(tt.input)
		if got != tt.want {
			t.Errorf("ParseLevel(%c) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFilterMatch(t *testing.T) {
	entry := &Entry{
		Timestamp: time.Now(),
		PID:       1234,
		TID:       5678,
		Level:     LevelDebug,
		Tag:       "MyTag",
		Message:   "hello world",
	}

	t.Run("empty filter matches all", func(t *testing.T) {
		f := NewFilter()
		if !f.Match(entry) {
			t.Error("empty filter should match")
		}
	})

	t.Run("level filter - MinLevel", func(t *testing.T) {
		f := NewFilter()
		f.SetMinLevel(LevelInfo)
		// Debug entry should not match when MinLevel is Info
		if f.Match(entry) {
			t.Error("debug should not match when MinLevel is Info")
		}
		// Info entry should match
		infoEntry := &Entry{
			Timestamp: time.Now(),
			PID:       1234,
			TID:       5678,
			Level:     LevelInfo,
			Tag:       "MyTag",
			Message:   "info message",
		}
		if !f.Match(infoEntry) {
			t.Error("info should match when MinLevel is Info")
		}
		// Verbose shows all
		f.SetMinLevel(LevelVerbose)
		if !f.Match(entry) {
			t.Error("debug should match when MinLevel is Verbose")
		}
	})

	t.Run("tag filter", func(t *testing.T) {
		f := NewFilter()
		f.Tag = "MyTag"
		if !f.Match(entry) {
			t.Error("should match tag")
		}
		f.Tag = "Other"
		if f.Match(entry) {
			t.Error("should not match different tag")
		}
	})

	t.Run("tag exclude filter", func(t *testing.T) {
		f := NewFilter()
		f.TagExclude = "My"
		if f.Match(entry) {
			t.Error("should exclude matching tag")
		}
		f.TagExclude = "Other"
		if !f.Match(entry) {
			t.Error("should not exclude unmatched tag")
		}
	})

	t.Run("search text", func(t *testing.T) {
		f := NewFilter()
		f.SetSearch("hello", false)
		if !f.Match(entry) {
			t.Error("should match search text")
		}
		f.SetSearch("missing", false)
		if f.Match(entry) {
			t.Error("should not match missing text")
		}
	})

	t.Run("regex search", func(t *testing.T) {
		f := NewFilter()
		f.SetSearch("hel+o", true)
		if !f.Match(entry) {
			t.Error("should match regex")
		}
	})

	t.Run("PID filter", func(t *testing.T) {
		f := NewFilter()
		f.PID = 1234
		if !f.Match(entry) {
			t.Error("should match PID")
		}
		f.PID = 9999
		if f.Match(entry) {
			t.Error("should not match wrong PID")
		}
	})

	t.Run("process filter", func(t *testing.T) {
		f := NewFilter()
		f.Process = "systemui"
		f.PIDsByPkg = map[string][]int{
			"com.android.systemui": {1234},
			"com.android.phone":    {2222},
		}
		if !f.Match(entry) {
			t.Error("should match process name by substring")
		}
		f.Process = "PHONE"
		if f.Match(entry) {
			t.Error("should not match another process name")
		}
	})

	t.Run("package filter matches subprocess", func(t *testing.T) {
		f := NewFilter()
		f.Package = "com.huawei.smarthome.extend"
		f.PIDsByPkg = map[string][]int{
			"com.huawei.smarthome.extend:p9": {1234},
		}
		if !f.Match(entry) {
			t.Error("package filter should match subprocess name with colon suffix")
		}
	})

	t.Run("crash only filter", func(t *testing.T) {
		f := NewFilter()
		f.CrashOnly = true
		crashEntry := &Entry{
			Timestamp: time.Now(),
			PID:       1234,
			TID:       5678,
			Level:     LevelError,
			Tag:       "AndroidRuntime",
			Message:   "FATAL EXCEPTION: main",
			IsCrash:   true,
		}
		if !f.Match(crashEntry) {
			t.Error("crash-only filter should match crash entry")
		}
		if f.Match(entry) {
			t.Error("crash-only filter should reject non-crash entry")
		}
	})

	t.Run("snapshot roundtrip", func(t *testing.T) {
		f := NewFilter()
		f.SetMinLevel(LevelWarn)
		f.Tag = "Audio"
		f.TagExclude = "Noise"
		f.PID = 321
		f.Package = "com.test.app"
		f.Process = "remote"
		f.CrashOnly = true
		f.TimeWindow = time.Minute
		f.SetSearch("fatal", true)

		snap := f.Snapshot()
		other := NewFilter()
		other.ApplySnapshot(snap)

		if other.MinLevel != LevelWarn || other.Tag != "Audio" || other.TagExclude != "Noise" || other.PID != 321 ||
			other.Package != "com.test.app" || other.Process != "remote" || !other.CrashOnly || other.TimeWindow != time.Minute {
			t.Error("snapshot should restore scalar filter fields")
		}
		if other.SearchText != "fatal" || !other.IsRegex || other.SearchRe == nil {
			t.Error("snapshot should restore search settings")
		}
	})

	t.Run("time window filter", func(t *testing.T) {
		f := NewFilter()
		f.TimeWindow = time.Minute
		f.ReferenceTime = time.Now()
		recent := &Entry{Timestamp: f.ReferenceTime.Add(-30 * time.Second), PID: 1, TID: 1, Level: LevelInfo}
		old := &Entry{Timestamp: f.ReferenceTime.Add(-2 * time.Minute), PID: 1, TID: 1, Level: LevelInfo}
		if !f.Match(recent) {
			t.Error("recent entry should match time window")
		}
		if f.Match(old) {
			t.Error("old entry should not match time window")
		}
	})

	t.Run("nil entry", func(t *testing.T) {
		f := NewFilter()
		if f.Match(nil) {
			t.Error("nil entry should not match")
		}
	})
}

func TestFilterIsActive(t *testing.T) {
	f := NewFilter()
	if f.IsActive() {
		t.Error("new filter should not be active")
	}
	f.Tag = "test"
	if !f.IsActive() {
		t.Error("filter with tag should be active")
	}
	f = NewFilter()
	f.Process = "systemui"
	if !f.IsActive() {
		t.Error("filter with process should be active")
	}
}
