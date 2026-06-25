package source

import "github.com/Yecangyuan/LogcatTool/internal/logentry"

type LogSource interface {
	Start() (<-chan *logentry.Entry, <-chan error)
	Stop()
	IsRunning() bool
	Name() string
}

type ReplayController interface {
	Pause()
	Resume()
	SetSpeed(speed float64)
	Speed() float64
}
