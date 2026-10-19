package adb

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

// stopGracePeriod bounds how long Stop waits for the reader goroutine to wind
// down after the child process has been cancelled.
const stopGracePeriod = 2 * time.Second

type LogcatReader struct {
	adbPath string
	serial  string
	buffer  string
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	done    chan struct{}
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
	r.done = make(chan struct{})
	done := r.done
	r.mu.Unlock()

	go r.readLoop(ctx, stdout, entries, errc, done)
	return entries, errc
}

// Stop terminates the stream and waits for the child process to be reaped.
//
// Waiting matters: cancelling the context only signals the process. Without a
// Wait the adb child stays a zombie until the parent exits, and switching
// devices repeatedly accumulates stray processes.
func (r *LogcatReader) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if done != nil {
		timer := time.NewTimer(stopGracePeriod)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
		}
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()
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

func (r *LogcatReader) readLoop(ctx context.Context, rd io.Reader, entries chan<- *logentry.Entry, errc chan<- error, done chan struct{}) {
	defer close(entries)
	defer close(errc)
	defer func() {
		// Reap the child process. A cancellation shows up as "signal: killed",
		// which is expected and intentionally ignored.
		if r.cmd != nil {
			_ = r.cmd.Wait()
		}

		r.mu.Lock()
		r.running = false
		r.mu.Unlock()

		if done != nil {
			close(done)
		}
	}()

	reader := logentry.NewStreamReader(rd)

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

		select {
		case entries <- entry:
		case <-ctx.Done():
			return
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
	_, err := runADB(adbPath, args...)
	return err
}
