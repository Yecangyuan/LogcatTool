package model

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/simley/logcatool/internal/adb"
	"github.com/simley/logcatool/internal/logentry"
	"github.com/simley/logcatool/internal/ringbuf"
	"github.com/simley/logcatool/internal/source"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

const defaultBufferSize = 50000

type InputMode int

const (
	ModeNormal InputMode = iota
	ModeSearch
	ModeTagFilter
	ModePkgFilter
	ModePidFilter
	ModeDevicePicker
)

type AppModel struct {
	allEntries *ringbuf.RingBuffer[*logentry.Entry]
	filtered   []*logentry.Entry
	filter     *logentry.Filter

	source    source.LogSource
	entryChan <-chan *logentry.Entry
	errorChan <-chan error
	adbPath   string
	devices   []adb.Device
	deviceIdx int

	width, height int
	ready         bool
	paused        bool
	autoScroll    bool
	wrapLines     bool
	showHelp      bool
	inputMode     InputMode

	viewport    viewport.Model
	filterInput textinput.Model
	helpModel   help.Model
	keys        KeyMap

	totalCount    int
	filteredCount int
	bookmarks     map[int]bool
	statusMsg     string

	filePath     string // non-empty when reading from file
	presetSerial string // preset device serial from CLI
}

// --- Messages ---

type LogEntriesMsg []*logentry.Entry

type LogStreamEndedMsg struct{}

type LogErrorMsg struct{ Err error }

type DeviceListMsg []adb.Device

type PackagePIDMsg map[string][]int

type ExportDoneMsg struct{ Path string }

type SourceStartedMsg struct {
	Source  source.LogSource
	Entries <-chan *logentry.Entry
	Errors  <-chan error
}

// --- Commands ---

func waitForEntries(ch <-chan *logentry.Entry) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-ch
		if !ok {
			return LogStreamEndedMsg{}
		}
		batch := []*logentry.Entry{entry}
		for {
			select {
			case e, ok := <-ch:
				if !ok {
					return LogEntriesMsg(batch)
				}
				batch = append(batch, e)
				if len(batch) >= 200 {
					return LogEntriesMsg(batch)
				}
			default:
				return LogEntriesMsg(batch)
			}
		}
	}
}

func listDevicesCmd(adbPath string) tea.Cmd {
	return func() tea.Msg {
		devices, err := adb.ListDevices(adbPath)
		if err != nil {
			return LogErrorMsg{Err: err}
		}
		return DeviceListMsg(devices)
	}
}

func loadPackagePIDs(adbPath, serial string) tea.Cmd {
	return func() tea.Msg {
		pids, err := adb.GetPackagePIDs(adbPath, serial)
		if err != nil {
			return LogErrorMsg{Err: err}
		}
		return PackagePIDMsg(pids)
	}
}

func startSourceCmd(src source.LogSource) tea.Cmd {
	return func() tea.Msg {
		entries, errc := src.Start()
		return SourceStartedMsg{Source: src, Entries: entries, Errors: errc}
	}
}

func exportLogsCmd(entries []*logentry.Entry) tea.Cmd {
	return func() tea.Msg {
		filename := fmt.Sprintf("logcat_%s.txt", time.Now().Format("20060102_150405"))
		var sb strings.Builder
		for _, e := range entries {
			sb.WriteString(e.Raw)
			sb.WriteByte('\n')
		}
		if err := os.WriteFile(filename, []byte(sb.String()), 0644); err != nil {
			return LogErrorMsg{Err: fmt.Errorf("导出失败: %w", err)}
		}
		return ExportDoneMsg{Path: filename}
	}
}

// --- Constructor ---

type Options struct {
	ADBPath    string
	FilePath   string
	Serial     string
	BufferSize int
}

func New(opts Options) AppModel {
	if opts.BufferSize <= 0 {
		opts.BufferSize = defaultBufferSize
	}

	ti := textinput.New()
	ti.Placeholder = "输入搜索内容..."
	ti.CharLimit = 200
	ti.SetWidth(40)

	return AppModel{
		allEntries:   ringbuf.New[*logentry.Entry](opts.BufferSize),
		filter:       logentry.NewFilter(),
		adbPath:      opts.ADBPath,
		autoScroll:   true,
		keys:         DefaultKeyMap(),
		helpModel:    help.New(),
		filterInput:  ti,
		bookmarks:    make(map[int]bool),
		filePath:     opts.FilePath,
		presetSerial: opts.Serial,
	}
}

func (m AppModel) Init() tea.Cmd {
	if m.filePath != "" {
		src := source.NewFileSource(m.filePath)
		return startSourceCmd(src)
	}
	return listDevicesCmd(m.adbPath)
}
