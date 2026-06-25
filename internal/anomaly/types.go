package anomaly

import "time"

// Dimension identifies what is being monitored.
type Dimension int

const (
	DimGlobal Dimension = iota
	DimLevel
	DimTag
	DimPID
	DimPackage
	DimProcess
)

func (d Dimension) String() string {
	switch d {
	case DimGlobal:
		return "global"
	case DimLevel:
		return "level"
	case DimTag:
		return "tag"
	case DimPID:
		return "pid"
	case DimPackage:
		return "package"
	case DimProcess:
		return "process"
	default:
		return "unknown"
	}
}

// Direction indicates whether the anomaly is a spike or a drop.
type Direction int

const (
	DirectionSpike Direction = iota
	DirectionDrop
)

func (d Direction) String() string {
	switch d {
	case DirectionSpike:
		return "spike"
	case DirectionDrop:
		return "drop"
	default:
		return "unknown"
	}
}

// DimensionConfig holds per-dimension overrides. Nil/zero means inherit global.
type DimensionConfig struct {
	Enabled           *bool    `json:"enabled,omitempty"`
	RecentWindowSec   *int     `json:"recent_window_sec,omitempty"`
	BaselineWindowSec *int     `json:"baseline_window_sec,omitempty"`
	Multiplier        *float64 `json:"multiplier,omitempty"`
	DropMultiplier    *float64 `json:"drop_multiplier,omitempty"`
	MinBaseline       *int     `json:"min_baseline,omitempty"`
}

// Event is emitted when a dimension's rate crosses a threshold.
type Event struct {
	Dimension    Dimension
	Key          string
	Direction    Direction
	RecentRate   float64
	BaselineRate float64
	Ratio        float64
	TriggeredAt  time.Time // wall-clock time for cooldown / flashing
	LogTime      time.Time // log timestamp for aligning highlights
}
