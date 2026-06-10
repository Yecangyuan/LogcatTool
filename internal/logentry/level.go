package logentry

import "fmt"

type Level int

const (
	LevelVerbose Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
	LevelSilent
	LevelUnknown
)

var levelChars = map[byte]Level{
	'V': LevelVerbose,
	'D': LevelDebug,
	'I': LevelInfo,
	'W': LevelWarn,
	'E': LevelError,
	'F': LevelFatal,
	'S': LevelSilent,
}

func ParseLevel(c byte) Level {
	if l, ok := levelChars[c]; ok {
		return l
	}
	return LevelUnknown
}

func ParseLevelString(s string) Level {
	switch s {
	case "Verbose", "V":
		return LevelVerbose
	case "Debug", "D":
		return LevelDebug
	case "Info", "I":
		return LevelInfo
	case "Warn", "W":
		return LevelWarn
	case "Error", "E":
		return LevelError
	case "Fatal", "F":
		return LevelFatal
	case "Silent", "S":
		return LevelSilent
	default:
		return LevelUnknown
	}
}

func (l Level) Char() string {
	switch l {
	case LevelVerbose:
		return "V"
	case LevelDebug:
		return "D"
	case LevelInfo:
		return "I"
	case LevelWarn:
		return "W"
	case LevelError:
		return "E"
	case LevelFatal:
		return "F"
	case LevelSilent:
		return "S"
	default:
		return "?"
	}
}

func (l Level) Label() string {
	switch l {
	case LevelVerbose:
		return "Verbose"
	case LevelDebug:
		return "Debug"
	case LevelInfo:
		return "Info"
	case LevelWarn:
		return "Warn"
	case LevelError:
		return "Error"
	case LevelFatal:
		return "Fatal"
	case LevelSilent:
		return "Silent"
	default:
		return "Unknown"
	}
}

func (l Level) String() string {
	return fmt.Sprintf("%s(%s)", l.Label(), l.Char())
}

var FilterableLevels = []Level{
	LevelVerbose,
	LevelDebug,
	LevelInfo,
	LevelWarn,
	LevelError,
	LevelFatal,
}
