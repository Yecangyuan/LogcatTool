package anomaly

import "testing"

func TestDimensionString(t *testing.T) {
	tests := []struct {
		dim  Dimension
		want string
	}{
		{DimGlobal, "global"},
		{DimLevel, "level"},
		{DimTag, "tag"},
		{DimPID, "pid"},
		{DimPackage, "package"},
		{DimProcess, "process"},
		{Dimension(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.dim.String(); got != tc.want {
			t.Errorf("Dimension(%d).String() = %q, want %q", tc.dim, got, tc.want)
		}
	}
}

func TestDirectionString(t *testing.T) {
	tests := []struct {
		dir  Direction
		want string
	}{
		{DirectionSpike, "spike"},
		{DirectionDrop, "drop"},
		{Direction(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.dir.String(); got != tc.want {
			t.Errorf("Direction(%d).String() = %q, want %q", tc.dir, got, tc.want)
		}
	}
}
