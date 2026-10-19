package logentry

import (
	"bufio"
	"io"
)

// MaxLogLine caps how many bytes of a single raw line are kept. logcat can emit
// very long single lines (bundled JSON, formatted dumps, long SQL), and an
// unbounded read would either exhaust memory or - as with a default
// bufio.Scanner - abort the stream and look like the device went away.
const maxLogLine = 8 << 20

// StreamReader turns a raw `logcat -v threadtime` stream into entries.
//
// It combines the line reader with a Parser so callers do not have to deal with
// multi-line continuations themselves: stack traces are folded into the entry
// they belong to, and oversized lines are truncated instead of terminating the
// stream.
type StreamReader struct {
	br      *bufio.Reader
	parser  *Parser
	readErr error
	atEOF   bool
}

// NewStreamReader wraps r.
func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		br:     bufio.NewReaderSize(r, 256*1024),
		parser: NewParser(),
	}
}

// Next returns the next complete entry. It reports io.EOF once the stream is
// drained, and any read error after the entries already parsed have been
// returned.
func (s *StreamReader) Next() (*Entry, error) {
	for {
		if s.atEOF {
			// Release the entry still being built so the tail of the log is
			// never lost, then surface any deferred error.
			if entry := s.parser.Flush(); entry != nil {
				return entry, nil
			}
			if err := s.readErr; err != nil {
				s.readErr = nil
				return nil, err
			}
			return nil, io.EOF
		}

		line, truncated, err := s.readLine()
		if err != nil {
			s.atEOF = true
			if err != io.EOF {
				s.readErr = err
			}
			// A final chunk can still carry a usable line, so fall through.
		}

		if len(line) > 0 {
			if truncated {
				line = append(line, []byte(" …(该行过长,已截断)")...)
			}
			// The header that ends the previous entry is what gets returned;
			// the new header itself stays buffered inside the parser.
			if entry := s.parser.ParseLine(string(line)); entry != nil {
				return entry, nil
			}
		}
	}
}

// readLine reads one '\n' terminated line, discarding bytes beyond maxLogLine
// while keeping the stream aligned on the next line boundary.
func (s *StreamReader) readLine() ([]byte, bool, error) {
	var buf []byte
	truncated := false

	for {
		chunk, err := s.br.ReadSlice('\n')

		if len(chunk) > 0 {
			if len(buf) < maxLogLine {
				room := maxLogLine - len(buf)
				if len(chunk) > room {
					chunk = chunk[:room]
					truncated = true
				}
				buf = append(buf, chunk...)
			} else {
				truncated = true
			}
		}

		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return trimEOL(buf), truncated, err
		}
		return trimEOL(buf), truncated, nil
	}
}

func trimEOL(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		b = b[:n-1]
		if n = len(b); n > 0 && b[n-1] == '\r' {
			b = b[:n-1]
		}
		return b
	}
	return b
}
