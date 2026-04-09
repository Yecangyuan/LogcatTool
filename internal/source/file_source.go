package source

import (
	"bufio"
	"fmt"
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

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		index := 0
		for scanner.Scan() {
			select {
			case <-s.done:
				return
			default:
			}

			entry := logentry.Parse(scanner.Text())
			if entry == nil {
				continue
			}
			entry.Index = index
			index++

			select {
			case entries <- entry:
			case <-s.done:
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case errc <- err:
			default:
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
