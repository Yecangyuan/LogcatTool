package model

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/adb"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
	"github.com/Yecangyuan/LogcatTool/internal/ringbuf"
	"github.com/Yecangyuan/LogcatTool/internal/source"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
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
	ModeProcessFilter
	ModeDevicePicker
	ModePkgPicker
)

// Logcat buffer types
type LogcatBuffer int

const (
	BufferAll LogcatBuffer = iota
	BufferMain
	BufferSystem
	BufferCrash
	BufferEvents
)

func (b LogcatBuffer) String() string {
	switch b {
	case BufferMain:
		return "main"
	case BufferSystem:
		return "system"
	case BufferCrash:
		return "crash"
	case BufferEvents:
		return "events"
	default:
		return "all"
	}
}

func (b LogcatBuffer) Label() string {
	switch b {
	case BufferMain:
		return "Main"
	case BufferSystem:
		return "System"
	case BufferCrash:
		return "Crash"
	case BufferEvents:
		return "Events"
	default:
		return "All"
	}
}

type displayRow struct {
	Entry *logentry.Entry
	Count int
}

type filterPreset struct {
	Used     bool
	Snapshot logentry.Snapshot
}

type AppModel struct {
	allEntries  *ringbuf.RingBuffer[*logentry.Entry]
	filtered    []*logentry.Entry
	displayRows []displayRow
	filter      *logentry.Filter

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
	showDetails   bool
	collapseDupes bool
	inputMode     InputMode

	// Virtual scroll state (replaces viewport)
	scrollOffset int // index of first visible entry in filtered
	viewHeight   int // number of visible log lines

	filterInput textinput.Model
	helpModel   help.Model
	keys        KeyMap

	totalCount    int
	filteredCount int
	displayCount  int
	bookmarks     map[int]bool
	statusMsg     string

	filePath     string // non-empty when reading from file
	presetSerial string // preset device serial from CLI

	// Package picker state
	allPackages      []string
	filteredPackages []string
	pkgPickerIdx     int
	pkgPickerSearch  string

	// Logcat buffer selection
	logBuffer LogcatBuffer

	// Auto-reconnect
	reconnecting  bool
	reconnectSecs int

	// Filter presets
	activePreset int
	presetSlots  [3]filterPreset
}

// --- Messages ---

type LogEntriesMsg []*logentry.Entry

type LogStreamEndedMsg struct{}

type LogErrorMsg struct{ Err error }

type DeviceListMsg []adb.Device

type PackagePIDMsg map[string][]int

type ExportDoneMsg struct{ Path string }

type PackageListMsg []string

type ClearDeviceDoneMsg struct{}

type ReconnectTickMsg struct{}

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

func listPackagesCmd(adbPath, serial string) tea.Cmd {
	return func() tea.Msg {
		pkgs, err := adb.ListPackages(adbPath, serial)
		if err != nil {
			return LogErrorMsg{Err: err}
		}
		return PackageListMsg(pkgs)
	}
}

func clearDeviceCmd(adbPath, serial string) tea.Cmd {
	return func() tea.Msg {
		_ = adb.ClearLogcat(adbPath, serial)
		return ClearDeviceDoneMsg{}
	}
}

func reconnectTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return ReconnectTickMsg{}
	})
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
		logBuffer:    BufferAll,
	}
}

func (m AppModel) Init() tea.Cmd {
	if m.filePath != "" {
		src := source.NewFileSource(m.filePath)
		return startSourceCmd(src)
	}
	return listDevicesCmd(m.adbPath)
}
