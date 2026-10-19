package logentry

import (
	"strings"
	"time"
)

const (
	// maxContinuationBytes caps how much multi-line payload (Java/Kotlin stack
	// traces, pre-formatted dumps) is folded into a single entry, so one runaway
	// message cannot grow without bound.
	maxContinuationBytes = 64 << 10

	// yearRolloverGrace tolerates devices whose clock runs slightly ahead of the
	// host when deciding whether a MM-DD stamp belongs to the previous year.
	yearRolloverGrace = 24 * time.Hour
)

// Parser converts raw `logcat -v threadtime` lines into entries.
//
// It is stateful because threadtime output emits stack traces and other
// multi-line payloads as bare continuation lines that carry no
// timestamp/PID/TID/level header. Those lines belong to the entry that preceded
// them, so the parser buffers the entry it is building and releases it when the
// next header line (or Flush) arrives.
//
// A Parser is not safe for concurrent use; use one Parser per log stream.
type Parser struct {
	year      int
	lastMonth int
	lastDay   int
	pending   *Entry
}

// NewParser returns a Parser anchored to the current year.
func NewParser() *Parser {
	return &Parser{year: time.Now().Year()}
}

// ParseLine consumes a single raw line.
//
// For a header line it returns the entry buffered so far (nil for the very
// first one); for a continuation line it appends the text to the buffered entry
// and returns nil. Call Flush when the stream ends to drain the last entry.
func (p *Parser) ParseLine(line string) *Entry {
	entry, ok := p.parseHeader(line)
	if !ok {
		p.appendContinuation(line)
		return nil
	}
	prev := p.pending
	p.pending = entry
	return prev
}

// Flush returns the entry still buffered by the parser, clearing it.
func (p *Parser) Flush() *Entry {
	entry := p.pending
	p.pending = nil
	return entry
}

// appendContinuation folds a headerless line into the pending entry.
func (p *Parser) appendContinuation(line string) {
	if p.pending == nil || line == "" {
		return
	}
	if len(p.pending.Message) >= maxContinuationBytes {
		return
	}

	var b strings.Builder
	b.Grow(len(p.pending.Message) + len(line) + 2)
	b.WriteString(p.pending.Message)
	b.WriteString("\n")
	b.WriteString(line)

	msg := b.String()
	if len(msg) > maxContinuationBytes {
		msg = msg[:maxContinuationBytes]
	}
	p.pending.Message = msg

	if len(p.pending.Raw) < maxContinuationBytes {
		p.pending.Raw += "\n" + line
	}
}

// Parse converts a single threadtime header line into an Entry, returning nil
// when the line is not a log header. Continuation lines are not folded; use a
// Parser for streams that may contain multi-line payloads.
func Parse(line string) *Entry {
	var p Parser
	entry, ok := p.parseHeader(line)
	if !ok {
		return nil
	}
	return entry
}

// yearFor resolves the calendar year for a MM-DD stamp.
//
// logcat prints no year, so it is inferred from the host clock. When the stamp
// is more than a day ahead of "now" it must belong to the previous year, which
// keeps filters correct across New Year rollover and when the device clock is
// ahead of the host. The result is memoised per date to keep the hot path
// allocation free.
func (p *Parser) yearFor(month, day int) int {
	if p.year != 0 && month == p.lastMonth && day == p.lastDay {
		return p.year
	}

	year := p.year
	if year == 0 {
		year = time.Now().Year()
	}
	if time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local).
		After(time.Now().Add(yearRolloverGrace)) {
		year--
	}

	p.year, p.lastMonth, p.lastDay = year, month, day
	return year
}

// parseHeader parses one `MM-DD HH:MM:SS.mmm PID TID L Tag: Message` line.
func (p *Parser) parseHeader(line string) (*Entry, bool) {
	n := len(line)
	if n < 20 {
		return nil, false
	}

	i := 0
	// Skip leading whitespace
	for i < n && line[i] == ' ' {
		i++
	}

	// Date: MM-DD (5 chars)
	if i+5 > n || line[i+2] != '-' {
		return nil, false
	}
	if !isDigit(line[i]) || !isDigit(line[i+1]) || !isDigit(line[i+3]) || !isDigit(line[i+4]) {
		return nil, false
	}
	date := line[i : i+5]
	i += 5

	if i >= n || line[i] != ' ' {
		return nil, false
	}
	for i < n && line[i] == ' ' {
		i++
	}

	// Time: HH:MM:SS.mmm (12 chars)
	if i+12 > n {
		return nil, false
	}
	timeStr := line[i : i+12]
	if timeStr[2] != ':' || timeStr[5] != ':' || timeStr[8] != '.' {
		return nil, false
	}
	i += 12

	for i < n && line[i] == ' ' {
		i++
	}

	// PID (digits)
	pidStart := i
	for i < n && isDigit(line[i]) {
		i++
	}
	if i == pidStart {
		return nil, false
	}
	pid := fastAtoi(line[pidStart:i])

	for i < n && line[i] == ' ' {
		i++
	}

	// TID (digits)
	tidStart := i
	for i < n && isDigit(line[i]) {
		i++
	}
	if i == tidStart {
		return nil, false
	}
	tid := fastAtoi(line[tidStart:i])

	for i < n && line[i] == ' ' {
		i++
	}

	// Level (single char)
	if i >= n {
		return nil, false
	}
	level := ParseLevel(line[i])
	if level == LevelUnknown {
		return nil, false
	}
	i++

	for i < n && line[i] == ' ' {
		i++
	}

	// Tag : Message
	rest := line[i:]
	colonIdx := strings.Index(rest, ": ")
	if colonIdx < 0 {
		return nil, false
	}
	tag := strings.TrimRight(rest[:colonIdx], " ")
	msg := rest[colonIdx+2:]

	month := fastAtoi(date[0:2])
	day := fastAtoi(date[3:5])
	ts := parseTimestampFast(timeStr, p.yearFor(month, day), month, day)

	return &Entry{
		Timestamp: ts,
		PID:       pid,
		TID:       tid,
		Level:     level,
		Tag:       tag,
		Message:   msg,
		Raw:       line,
	}, true
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func fastAtoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func parseTimestampFast(timeStr string, year, month, day int) time.Time {
	hour := fastAtoi(timeStr[0:2])
	min := fastAtoi(timeStr[3:5])
	sec := fastAtoi(timeStr[6:8])
	msec := fastAtoi(timeStr[9:12])

	return time.Date(
		year, time.Month(month), day,
		hour, min, sec, msec*1e6, time.Local,
	)
}
