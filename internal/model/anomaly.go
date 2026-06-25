package model

import (
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
)

// AnomalyEventsMsg is delivered by the detector listener command.
type AnomalyEventsMsg []anomaly.Event

const maxAnomalyEvents = 100

type anomalyState struct {
	events        []anomaly.Event
	panelOpen     bool
	selection     int
	highlightSec  int
	flashingUntil time.Time
}

func (a *anomalyState) applyEvents(events []anomaly.Event, highlightSec int) {
	if len(events) == 0 {
		return
	}
	now := time.Now()
	a.flashingUntil = now.Add(2 * time.Second)
	a.highlightSec = highlightSec
	for _, e := range events {
		found := false
		for i := range a.events {
			if a.events[i].Dimension == e.Dimension && a.events[i].Key == e.Key {
				a.events[i] = e
				found = true
				break
			}
		}
		if !found {
			a.events = append(a.events, e)
		}
	}
	if len(a.events) > maxAnomalyEvents {
		a.events = a.events[len(a.events)-maxAnomalyEvents:]
		if a.selection >= len(a.events) {
			a.selection = len(a.events) - 1
		}
	}
}

func (a *anomalyState) clear() {
	a.events = nil
	a.selection = 0
}

func (a *anomalyState) isHighlighted(entry *logentry.Entry) bool {
	if entry == nil || a.highlightSec <= 0 {
		return false
	}
	window := time.Duration(a.highlightSec) * time.Second
	for _, e := range a.events {
		if entry.Timestamp.After(e.LogTime.Add(-window)) &&
			entry.Timestamp.Before(e.LogTime.Add(window)) {
			return true
		}
	}
	return false
}

func (a *anomalyState) isFlashing(now time.Time) bool {
	return now.Before(a.flashingUntil) && len(a.events) > 0
}

func (a *anomalyState) worst() anomaly.Event {
	if len(a.events) == 0 {
		return anomaly.Event{}
	}
	w := a.events[0]
	for _, e := range a.events[1:] {
		if e.Ratio > w.Ratio {
			w = e
		}
	}
	return w
}

func (m *AppModel) applySelectedAnomalyFilter() tea.Cmd {
	if m.anomaly.selection >= len(m.anomaly.events) {
		return nil
	}
	e := m.anomaly.events[m.anomaly.selection]
	switch e.Dimension {
	case anomaly.DimLevel:
		if lvl := logentry.ParseLevelString(e.Key); lvl > 0 {
			m.filter.SetMinLevel(lvl)
			m.statusMsg = fmt.Sprintf("异常过滤 级别: ≥%s", e.Key)
		}
	case anomaly.DimTag:
		m.filter.Tag = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 Tag: %s", e.Key)
	case anomaly.DimPID:
		if pid, err := strconv.Atoi(e.Key); err == nil {
			m.filter.PID = pid
			m.statusMsg = fmt.Sprintf("异常过滤 PID: %d", pid)
		}
	case anomaly.DimPackage:
		m.filter.Package = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 包名: %s", e.Key)
		if m.filePath == "" {
			return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
		}
	case anomaly.DimProcess:
		m.filter.Process = e.Key
		m.statusMsg = fmt.Sprintf("异常过滤 进程: %s", e.Key)
		if m.filePath == "" {
			return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
		}
	}
	m.refilter()
	return nil
}
