package adb

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

type LogcatReader struct {
	adbPath string
	serial  string
	buffer  string
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
}

func NewLogcatReader(adbPath, serial, buffer string) *LogcatReader {
	return &LogcatReader{
		adbPath: adbPath,
		serial:  serial,
		buffer:  buffer,
	}
}

// Start begins streaming logcat output. Entries are sent to the entries channel.
// Call Stop() to terminate.
func (r *LogcatReader) Start(ctx context.Context) (<-chan *logentry.Entry, <-chan error) {
	entries := make(chan *logentry.Entry, 256)
	errc := make(chan error, 1)

	ctx, r.cancel = context.WithCancel(ctx)

	args := r.buildArgs()
	r.cmd = exec.CommandContext(ctx, r.adbPath, args...)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		errc <- fmt.Errorf("stdout pipe 失败: %w", err)
		close(entries)
		close(errc)
		return entries, errc
	}

	if err := r.cmd.Start(); err != nil {
		errc <- fmt.Errorf("启动 adb logcat 失败: %w", err)
		close(entries)
		close(errc)
		return entries, errc
	}

	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	go r.readLoop(ctx, stdout, entries, errc)
	return entries, errc
}

func (r *LogcatReader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		r.cancel()
	}
	r.running = false
}

func (r *LogcatReader) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

func (r *LogcatReader) buildArgs() []string {
	var args []string
	if r.serial != "" {
		args = append(args, "-s", r.serial)
	}
	args = append(args, "logcat", "-v", "threadtime")
	if r.buffer != "" && r.buffer != "all" {
		args = append(args, "-b", r.buffer)
	}
	return args
}

func (r *LogcatReader) readLoop(ctx context.Context, rd io.Reader, entries chan<- *logentry.Entry, errc chan<- error) {
	defer close(entries)
	defer close(errc)
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	scanner := bufio.NewScanner(rd)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	index := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		entry := logentry.Parse(line)
		if entry == nil {
			continue
		}
		entry.Index = index
		index++

		select {
		case entries <- entry:
		case <-ctx.Done():
			return
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case errc <- err:
		default:
		}
	}
}

// ClearLogcat clears the device logcat buffer.
func ClearLogcat(adbPath, serial string) error {
	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-c")
	return exec.Command(adbPath, args...).Run()
}
