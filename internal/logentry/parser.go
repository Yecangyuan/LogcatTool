package logentry

import (
	"strconv"
	"strings"
	"time"
)

var currentYear = strconv.Itoa(time.Now().Year())

func Parse(line string) *Entry {
	n := len(line)
	if n < 20 {
		return nil
	}

	i := 0
	// Skip leading whitespace
	for i < n && line[i] == ' ' {
		i++
	}

	// Date: MM-DD (5 chars)
	if i+5 > n || line[i+2] != '-' {
		return nil
	}
	if !isDigit(line[i]) || !isDigit(line[i+1]) || !isDigit(line[i+3]) || !isDigit(line[i+4]) {
		return nil
	}
	date := line[i : i+5]
	i += 5

	if i >= n || line[i] != ' ' {
		return nil
	}
	for i < n && line[i] == ' ' {
		i++
	}

	// Time: HH:MM:SS.mmm (12 chars)
	if i+12 > n {
		return nil
	}
	timeStr := line[i : i+12]
	if timeStr[2] != ':' || timeStr[5] != ':' || timeStr[8] != '.' {
		return nil
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
		return nil
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
		return nil
	}
	tid := fastAtoi(line[tidStart:i])

	for i < n && line[i] == ' ' {
		i++
	}

	// Level (single char)
	if i >= n {
		return nil
	}
	level := ParseLevel(line[i])
	if level == LevelUnknown {
		return nil
	}
	i++

	for i < n && line[i] == ' ' {
		i++
	}

	// Tag : Message
	rest := line[i:]
	colonIdx := strings.Index(rest, ": ")
	if colonIdx < 0 {
		return nil
	}
	tag := strings.TrimRight(rest[:colonIdx], " ")
	msg := rest[colonIdx+2:]

	ts := parseTimestampFast(date, timeStr)

	return &Entry{
		Timestamp: ts,
		PID:       pid,
		TID:       tid,
		Level:     level,
		Tag:       tag,
		Message:   msg,
		Raw:       line,
	}
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

func parseTimestampFast(date, timeStr string) time.Time {
	month := fastAtoi(date[0:2])
	day := fastAtoi(date[3:5])
	hour := fastAtoi(timeStr[0:2])
	min := fastAtoi(timeStr[3:5])
	sec := fastAtoi(timeStr[6:8])
	msec := fastAtoi(timeStr[9:12])

	return time.Date(
		time.Now().Year(), time.Month(month), day,
		hour, min, sec, msec*1e6, time.Local,
	)
}
