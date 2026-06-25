package source

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type replaySleepFunc func(time.Duration) bool

type ReplayOption func(*ReplaySource)

type ReplaySource struct {
	path    string
	mu      sync.Mutex
	running bool
	done    chan struct{}

	speed    float64
	resumeCh chan struct{}
	sleepFn  replaySleepFunc
}

func NewReplaySource(path string, speed float64, opts ...ReplayOption) *ReplaySource {
	if speed <= 0 {
		speed = 1
	}
	s := &ReplaySource{
		path:  path,
		speed: speed,
		done:  make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithReplaySleep(fn replaySleepFunc) ReplayOption {
	return func(s *ReplaySource) {
		s.sleepFn = fn
	}
}

func (s *ReplaySource) Start() (<-chan *logentry.Entry, <-chan error) {
	entries := make(chan *logentry.Entry, 256)
	errc := make(chan error, 1)

	s.mu.Lock()
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		defer close(entries)
		defer close(errc)
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		parsed, err := readReplayEntries(s.path)
		if err != nil {
			errc <- err
			return
		}
		sort.SliceStable(parsed, func(i, j int) bool {
			return parsed[i].Timestamp.Before(parsed[j].Timestamp)
		})
		for i, entry := range parsed {
			entry.Index = i
		}

		for i, entry := range parsed {
			if !s.waitWhilePaused() {
				return
			}
			if i > 0 {
				delay := replayDelay(entry.Timestamp.Sub(parsed[i-1].Timestamp), s.Speed())
				if delay > 0 && !s.sleep(delay) {
					return
				}
				if !s.waitWhilePaused() {
					return
				}
			}
			select {
			case entries <- entry:
			case <-s.done:
				return
			}
		}
	}()

	return entries, errc
}

func (s *ReplaySource) Stop() {
	s.mu.Lock()
	if s.running {
		close(s.done)
		s.running = false
	}
	resumeCh := s.resumeCh
	s.resumeCh = nil
	s.mu.Unlock()
	if resumeCh != nil {
		close(resumeCh)
	}
}

func (s *ReplaySource) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *ReplaySource) Name() string {
	return fmt.Sprintf("回放: %s (%.2gx)", s.path, s.Speed())
}

func (s *ReplaySource) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resumeCh == nil {
		s.resumeCh = make(chan struct{})
	}
}

func (s *ReplaySource) Resume() {
	s.mu.Lock()
	resumeCh := s.resumeCh
	s.resumeCh = nil
	s.mu.Unlock()
	if resumeCh != nil {
		close(resumeCh)
	}
}

func (s *ReplaySource) SetSpeed(speed float64) {
	if speed <= 0 {
		speed = 1
	}
	s.mu.Lock()
	s.speed = speed
	s.mu.Unlock()
}

func (s *ReplaySource) Speed() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.speed <= 0 {
		return 1
	}
	return s.speed
}

func (s *ReplaySource) waitWhilePaused() bool {
	for {
		s.mu.Lock()
		resumeCh := s.resumeCh
		done := s.done
		s.mu.Unlock()
		if resumeCh == nil {
			return true
		}
		select {
		case <-resumeCh:
		case <-done:
			return false
		}
	}
}

func (s *ReplaySource) sleep(delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	if s.sleepFn != nil {
		return s.sleepFn(delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-s.done:
		return false
	}
}

func replayDelay(gap time.Duration, speed float64) time.Duration {
	if gap <= 0 {
		return 0
	}
	if speed <= 0 {
		speed = 1
	}
	return time.Duration(float64(gap) / speed)
}

func readReplayEntries(path string) ([]*logentry.Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开回放日志失败: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var entries []*logentry.Entry
	for scanner.Scan() {
		entry := logentry.Parse(scanner.Text())
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
