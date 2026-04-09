package source

import "github.com/simley/logcatool/internal/logentry"

type LogSource interface {
	Start() (<-chan *logentry.Entry, <-chan error)
	Stop()
	IsRunning() bool
	Name() string
}
