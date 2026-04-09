package source

import "github.com/Yecangyuan/LogcatTool/internal/logentry"

type LogSource interface {
	Start() (<-chan *logentry.Entry, <-chan error)
	Stop()
	IsRunning() bool
	Name() string
}
