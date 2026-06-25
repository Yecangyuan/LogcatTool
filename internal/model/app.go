package model

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Yecangyuan/LogcatTool/internal/adb"
	"github.com/Yecangyuan/LogcatTool/internal/anomaly"
	"github.com/Yecangyuan/LogcatTool/internal/config"
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
	ModeTagExcludeFilter
	ModePkgFilter
	ModePidFilter
	ModeProcessFilter
	ModeAlertKeyword
	ModeStatsPanel
	ModeDevicePicker
	ModePkgPicker
	ModeGotoTime
	ModeAnomalyPanel
)

func isFilterInputMode(mode InputMode) bool {
	switch mode {
	case ModeSearch, ModeTagFilter, ModeTagExcludeFilter, ModePkgFilter, ModePidFilter, ModeProcessFilter, ModeAlertKeyword:
		return true
	default:
		return false
	}
}

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

type statsKind string

const (
	statsLevel   statsKind = "level"
	statsTag     statsKind = "tag"
	statsPackage statsKind = "package"
	statsProcess statsKind = "process"
)

type statsRow struct {
	Kind     statsKind
	Section  string
	Label    string
	Value    string
	Count    int
	Level    logentry.Level
	Favorite bool
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
	pausedBuffer  []*logentry.Entry

	filePath     string // non-empty when reading from file
	presetSerial string // preset device serial from CLI

	// Package picker state
	allPackages      []string
	filteredPackages []string
	pkgPickerIdx     int
	pkgPickerSearch  string
	statsSelection   int

	// Logcat buffer selection
	logBuffer LogcatBuffer

	// Auto-reconnect
	reconnecting  bool
	reconnectSecs int

	// Filter presets
	activePreset int
	presetSlots  [3]filterPreset

	// Cached stats
	cachedStatsRows []statsRow
	statsDirty      bool

	// Favorites
	favoritePackages  map[string]bool
	favoriteProcesses map[string]bool
	processByPID      map[int]string
	packageByPID      map[int]string

	// Alerts
	alertKeyword string
	lastAlert    string

	// Search history
	searchHistory []string
	historyIdx    int // -1 means not browsing history

	// Crash stack folding
	crashFolded map[int]bool // entry.Index -> folded

	// Sparkline
	sparklineBins [20]int
	sparklineIdx  int

	// Anomaly detection
	anomalyDetector *anomaly.Detector
	anomaly         anomalyState
	anomalyEventsCh chan []anomaly.Event

	// Config persistence
	cfg config.Config
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

type SparklineTickMsg struct{}

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

func sparklineTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return SparklineTickMsg{}
	})
}

func waitForAnomalyEvents(ch <-chan []anomaly.Event) tea.Cmd {
	return func() tea.Msg {
		events, ok := <-ch
		if !ok {
			return nil
		}
		return AnomalyEventsMsg(events)
	}
}

func (m *AppModel) anomalyDetectorLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			events := m.anomalyDetector.Evaluate(time.Now())
			if len(events) > 0 {
				select {
				case m.anomalyEventsCh <- events:
				default:
				}
			}
		case <-m.anomalyDetector.Done():
			return
		}
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

	cfg, _ := config.Load()

	m := AppModel{
		allEntries:        ringbuf.New[*logentry.Entry](opts.BufferSize),
		filter:            logentry.NewFilter(),
		adbPath:           opts.ADBPath,
		autoScroll:        cfg.AutoScroll,
		wrapLines:         cfg.WrapLines,
		showDetails:       cfg.ShowDetails,
		collapseDupes:     cfg.CollapseDupes,
		keys:              DefaultKeyMap(),
		helpModel:         help.New(),
		filterInput:       ti,
		bookmarks:         make(map[int]bool),
		filePath:          opts.FilePath,
		presetSerial:      opts.Serial,
		logBuffer:         BufferAll,
		favoritePackages:  cfg.FavoritePackages,
		favoriteProcesses: cfg.FavoriteProcesses,
		processByPID:      make(map[int]string),
		packageByPID:      make(map[int]string),
		statsDirty:        true,
		alertKeyword:      cfg.AlertKeyword,
		searchHistory:     cfg.SearchHistory,
		crashFolded:       make(map[int]bool),
		cfg:               cfg,
	}

	m.anomalyDetector = anomaly.NewDetector(anomaly.Config{
		Enabled:             cfg.Anomaly.Enabled,
		RecentWindowSec:     cfg.Anomaly.RecentWindowSec,
		BaselineWindowSec:   cfg.Anomaly.BaselineWindowSec,
		Multiplier:          cfg.Anomaly.Multiplier,
		DropMultiplier:      cfg.Anomaly.DropMultiplier,
		MinBaseline:         cfg.Anomaly.MinBaseline,
		CooldownSec:         cfg.Anomaly.CooldownSec,
		MaxKeysPerDimension: cfg.Anomaly.MaxKeysPerDimension,
		Dimensions:          cfg.Anomaly.Dimensions,
	})
	m.anomalyEventsCh = make(chan []anomaly.Event, 16)
	go m.anomalyDetectorLoop()

	if m.favoritePackages == nil {
		m.favoritePackages = make(map[string]bool)
	}
	if m.favoriteProcesses == nil {
		m.favoriteProcesses = make(map[string]bool)
	}

	// Restore presets
	for i := range cfg.Presets {
		if cfg.Presets[i].Used {
			m.presetSlots[i] = filterPreset{
				Used: true,
				Snapshot: logentry.Snapshot{
					MinLevel:   logentry.ParseLevelString(cfg.Presets[i].MinLevel),
					Package:    cfg.Presets[i].Package,
					Process:    cfg.Presets[i].Process,
					Tag:        cfg.Presets[i].Tag,
					TagExclude: cfg.Presets[i].TagExclude,
					PID:        cfg.Presets[i].PID,
					SearchText: cfg.Presets[i].SearchText,
					CrashOnly:  cfg.Presets[i].CrashOnly,
					TimeWindow: time.Duration(cfg.Presets[i].TimeWindowSec) * time.Second,
				},
			}
		}
	}

	return m
}

func (m AppModel) Init() tea.Cmd {
	cmds := []tea.Cmd{sparklineTickCmd(), waitForAnomalyEvents(m.anomalyEventsCh)}
	if m.filePath != "" {
		src := source.NewFileSource(m.filePath)
		cmds = append(cmds, startSourceCmd(src))
	} else {
		cmds = append(cmds, listDevicesCmd(m.adbPath))
	}
	return tea.Batch(cmds...)
}

func (m AppModel) saveConfig() {
	m.cfg.FavoritePackages = m.favoritePackages
	m.cfg.FavoriteProcesses = m.favoriteProcesses
	m.cfg.AlertKeyword = m.alertKeyword
	m.cfg.SearchHistory = m.searchHistory
	m.cfg.CollapseDupes = m.collapseDupes
	m.cfg.WrapLines = m.wrapLines
	m.cfg.ShowDetails = m.showDetails
	m.cfg.AutoScroll = m.autoScroll

	for i := range m.presetSlots {
		if m.presetSlots[i].Used {
			s := m.presetSlots[i].Snapshot
			m.cfg.Presets[i] = config.Preset{
				Used:          true,
				MinLevel:      s.MinLevel.Char(),
				Package:       s.Package,
				Process:       s.Process,
				Tag:           s.Tag,
				TagExclude:    s.TagExclude,
				PID:           s.PID,
				SearchText:    s.SearchText,
				CrashOnly:     s.CrashOnly,
				TimeWindowSec: int(s.TimeWindow.Seconds()),
			}
		} else {
			m.cfg.Presets[i] = config.Preset{}
		}
	}

	_ = config.Save(m.cfg)
}
