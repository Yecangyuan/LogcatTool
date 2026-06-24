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

// Event is emitted when a dimension's rate crosses a threshold.
type Event struct {
	Dimension    Dimension
	Key          string
	Direction    Direction
	RecentRate   float64
	BaselineRate float64
	Ratio        float64
	TriggeredAt  time.Time
}
