package source

import (
	"context"

	"github.com/simley/logcatool/internal/adb"
	"github.com/simley/logcatool/internal/logentry"
)

type ADBSource struct {
	reader *adb.LogcatReader
	ctx    context.Context
	cancel context.CancelFunc
	device adb.Device
}

func NewADBSource(adbPath string, device adb.Device) *ADBSource {
	return &ADBSource{
		reader: adb.NewLogcatReader(adbPath, device.Serial),
		device: device,
	}
}

func (s *ADBSource) Start() (<-chan *logentry.Entry, <-chan error) {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s.reader.Start(s.ctx)
}

func (s *ADBSource) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.reader.Stop()
}

func (s *ADBSource) IsRunning() bool {
	return s.reader.IsRunning()
}

func (s *ADBSource) Name() string {
	return s.device.DisplayName()
}
