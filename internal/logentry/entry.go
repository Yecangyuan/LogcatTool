package logentry

import "time"

type Entry struct {
	Index     int
	Timestamp time.Time
	PID       int
	TID       int
	Level     Level
	Tag       string
	Message   string
	Raw       string
	Bookmarked bool
}
