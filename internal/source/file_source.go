package source

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type FileSource struct {
	path    string
	mu      sync.Mutex
	running bool
	done    chan struct{}
}

func NewFileSource(path string) *FileSource {
	return &FileSource{
		path: path,
		done: make(chan struct{}),
	}
}

func (s *FileSource) Start() (<-chan *logentry.Entry, <-chan error) {
	entries := make(chan *logentry.Entry, 256)
	errc := make(chan error, 1)

	s.mu.Lock()
	s.running = true
	s.done = make(chan struct{})
	done := s.done
	s.mu.Unlock()

	go func() {
		defer close(entries)
		defer close(errc)
		defer func() {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		f, err := os.Open(s.path)
		if err != nil {
			errc <- fmt.Errorf("打开日志文件失败: %w", err)
			return
		}
		defer f.Close()

		reader := logentry.NewStreamReader(f)
		index := 0

		for {
			entry, err := reader.Next()
			if err != nil {
				if err != io.EOF {
					select {
					case errc <- err:
					default:
					}
				}
				return
			}

			entry.Index = index
			index++

			select {
			case entries <- entry:
			case <-done:
				return
			}
		}
	}()

	return entries, errc
}

func (s *FileSource) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.done)
		s.running = false
	}
}

func (s *FileSource) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *FileSource) Name() string {
	return fmt.Sprintf("文件: %s", s.path)
}
