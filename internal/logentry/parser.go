package logentry

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// threadtime 格式: "04-09 10:42:01.234  1234  5678 D MyTag   : message"
var threadtimeRe = regexp.MustCompile(
	`^(\d{2}-\d{2})\s+(\d{2}:\d{2}:\d{2}\.\d{3})\s+(\d+)\s+(\d+)\s+([VDIWEFS])\s+(.+?)\s*:\s(.*)$`,
)

func Parse(line string) *Entry {
	m := threadtimeRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	ts := parseTimestamp(m[1], m[2])
	pid, _ := strconv.Atoi(m[3])
	tid, _ := strconv.Atoi(m[4])
	level := ParseLevel(m[5][0])
	tag := strings.TrimSpace(m[6])
	msg := m[7]

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

func parseTimestamp(date, timeStr string) time.Time {
	now := time.Now()
	ref := strconv.Itoa(now.Year()) + "-" + date + " " + timeStr
	t, err := time.ParseInLocation("2006-01-02 15:04:05.000", ref, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
