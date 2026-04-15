package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yecangyuan/LogcatTool/internal/adb"
	"github.com/Yecangyuan/LogcatTool/internal/logentry"
	"github.com/Yecangyuan/LogcatTool/internal/source"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.helpModel.SetWidth(msg.Width)
		m.viewHeight = m.calcViewHeight()
		m.ready = true
		m.clampScroll()

	case SourceStartedMsg:
		if m.source != nil {
			m.source.Stop()
		}
		m.source = msg.Source
		m.entryChan = msg.Entries
		m.errorChan = msg.Errors
		m.reconnecting = false
		m.statusMsg = "已连接"
		cmds = append(cmds, waitForEntries(m.entryChan))

	case LogEntriesMsg:
		if !m.paused {
			for _, entry := range msg {
				entry.Index = m.totalCount
				m.totalCount++
				m.preRenderEntry(entry)
				m.allEntries.Push(entry)
				if m.filter.Match(entry) {
					m.filtered = append(m.filtered, entry)
				}
			}
			m.filteredCount = len(m.filtered)
			// Prune filtered list if ring buffer overflowed
			if m.totalCount > m.allEntries.Cap() && len(m.filtered) > m.allEntries.Cap()*2 {
				m.refilter()
			}
			if m.autoScroll {
				m.scrollToBottom()
			}
		}
		if m.entryChan != nil {
			cmds = append(cmds, waitForEntries(m.entryChan))
		}

	case LogStreamEndedMsg:
		if m.filePath == "" && !m.reconnecting {
			m.reconnecting = true
			m.reconnectSecs = 3
			m.statusMsg = fmt.Sprintf("连接断开，%d秒后重连...", m.reconnectSecs)
			cmds = append(cmds, reconnectTickCmd())
		} else {
			m.statusMsg = "日志流已结束"
		}

	case ReconnectTickMsg:
		if m.reconnecting {
			m.reconnectSecs--
			if m.reconnectSecs <= 0 {
				m.reconnecting = false
				m.statusMsg = "正在重连..."
				if m.deviceIdx < len(m.devices) {
					cmds = append(cmds, m.connectDevice(m.devices[m.deviceIdx]))
				} else {
					cmds = append(cmds, listDevicesCmd(m.adbPath))
				}
			} else {
				m.statusMsg = fmt.Sprintf("连接断开，%d秒后重连...", m.reconnectSecs)
				cmds = append(cmds, reconnectTickCmd())
			}
		}

	case LogErrorMsg:
		m.statusMsg = fmt.Sprintf("错误: %v", msg.Err)

	case DeviceListMsg:
		m.devices = []adb.Device(msg)
		if len(m.devices) == 0 {
			m.statusMsg = "未发现 Android 设备"
			return m, nil
		}
		if m.presetSerial != "" {
			for i, d := range m.devices {
				if d.Serial == m.presetSerial {
					m.deviceIdx = i
					return m, m.connectDevice(d)
				}
			}
			m.statusMsg = fmt.Sprintf("设备 %s 未找到", m.presetSerial)
			return m, nil
		}
		if len(m.devices) == 1 {
			m.deviceIdx = 0
			return m, m.connectDevice(m.devices[0])
		}
		m.inputMode = ModeDevicePicker
		m.deviceIdx = 0

	case PackagePIDMsg:
		m.filter.PIDsByPkg = map[string][]int(msg)
		if m.filter.Package != "" {
			m.refilter()
			m.scrollToBottom()
			m.autoScroll = false
			m.statusMsg = fmt.Sprintf("包名过滤: %s", m.filter.Package)
		}

	case PackageListMsg:
		pkgs := []string(msg)
		m.allPackages = append([]string{"(清除过滤)"}, pkgs...)
		m.filteredPackages = m.allPackages
		m.pkgPickerIdx = 0
		m.pkgPickerSearch = ""
		m.inputMode = ModePkgPicker
		m.statusMsg = fmt.Sprintf("共 %d 个应用", len(pkgs))

	case ClearDeviceDoneMsg:
		m.statusMsg = "设备日志缓冲区已清除"

	case ExportDoneMsg:
		m.statusMsg = fmt.Sprintf("已导出到 %s", msg.Path)

	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.scrollOffset -= 3
			m.autoScroll = false
			m.clampScroll()
		case tea.MouseWheelDown:
			m.scrollOffset += 3
			m.clampScroll()
			if m.isAtBottom() {
				m.autoScroll = true
			}
		}

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Input mode handling
	switch m.inputMode {
	case ModeDevicePicker:
		return m.handleDevicePickerKey(msg)
	case ModePkgPicker:
		return m.handlePkgPickerKey(msg)
	case ModeSearch, ModeTagFilter, ModePkgFilter, ModePidFilter:
		return m.handleFilterInputKey(msg)
	}

	// Normal mode
	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.source != nil {
			m.source.Stop()
		}
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return m, nil

	case key.Matches(msg, m.keys.Pause):
		m.paused = !m.paused
		if m.paused {
			m.statusMsg = "⏸ 已暂停"
		} else {
			m.statusMsg = "▶ 已恢复"
		}
		return m, nil

	case key.Matches(msg, m.keys.Clear):
		m.allEntries.Clear()
		m.filtered = nil
		m.totalCount = 0
		m.filteredCount = 0
		m.scrollOffset = 0
		m.bookmarks = make(map[int]bool)
		m.statusMsg = "日志已清除"
		// Also clear device logcat buffer
		if m.filePath == "" && m.adbPath != "" {
			serial := ""
			if m.deviceIdx < len(m.devices) {
				serial = m.devices[m.deviceIdx].Serial
			}
			return m, clearDeviceCmd(m.adbPath, serial)
		}
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.inputMode = ModeSearch
		m.filterInput.Placeholder = "搜索 (支持正则)..."
		m.filterInput.SetValue(m.filter.SearchText)
		m.filterInput.Focus()
		return m, nil

	case key.Matches(msg, m.keys.TagFilter):
		m.inputMode = ModeTagFilter
		m.filterInput.Placeholder = "输入 Tag..."
		m.filterInput.SetValue(m.filter.Tag)
		m.filterInput.Focus()
		return m, nil

	case key.Matches(msg, m.keys.PkgFilter):
		if m.filePath != "" {
			m.statusMsg = "文件模式不支持包名过滤"
			return m, nil
		}
		serial := ""
		if m.deviceIdx < len(m.devices) {
			serial = m.devices[m.deviceIdx].Serial
		}
		m.statusMsg = "正在获取应用列表..."
		return m, listPackagesCmd(m.adbPath, serial)

	case key.Matches(msg, m.keys.PidFilter):
		m.inputMode = ModePidFilter
		m.filterInput.Placeholder = "输入 PID..."
		if m.filter.PID > 0 {
			m.filterInput.SetValue(strconv.Itoa(m.filter.PID))
		} else {
			m.filterInput.SetValue("")
		}
		m.filterInput.Focus()
		return m, nil

	case key.Matches(msg, m.keys.DevicePicker):
		if m.filePath == "" {
			m.inputMode = ModeDevicePicker
			m.deviceIdx = 0
			return m, listDevicesCmd(m.adbPath)
		}
		return m, nil

	case key.Matches(msg, m.keys.Export):
		if len(m.filtered) > 0 {
			return m, exportLogsCmd(m.filtered)
		}
		m.statusMsg = "没有日志可导出"
		return m, nil

	case key.Matches(msg, m.keys.Bookmark):
		return m.toggleBookmark(), nil

	case key.Matches(msg, m.keys.NextBookmark):
		m.gotoNextBookmark(true)
		return m, nil

	case key.Matches(msg, m.keys.PrevBookmark):
		m.gotoNextBookmark(false)
		return m, nil

	case key.Matches(msg, m.keys.AutoScroll):
		m.autoScroll = !m.autoScroll
		if m.autoScroll {
			m.scrollToBottom()
			m.statusMsg = "自动滚动: 开"
		} else {
			m.statusMsg = "自动滚动: 关"
		}
		return m, nil

	case key.Matches(msg, m.keys.WrapToggle):
		m.wrapLines = !m.wrapLines
		if m.wrapLines {
			m.statusMsg = "换行: 开"
		} else {
			m.statusMsg = "换行: 关"
		}
		return m, nil

	case key.Matches(msg, m.keys.BufferSelect):
		if m.filePath != "" {
			m.statusMsg = "文件模式不支持切换缓冲区"
			return m, nil
		}
		m.logBuffer = (m.logBuffer + 1) % 5
		m.statusMsg = fmt.Sprintf("日志缓冲区: %s", m.logBuffer.Label())
		// Reconnect with new buffer
		if m.deviceIdx < len(m.devices) {
			return m, m.connectDevice(m.devices[m.deviceIdx])
		}
		return m, nil

	case key.Matches(msg, m.keys.CopyLine):
		return m, m.copyCurrentLine()

	// Navigation
	case key.Matches(msg, m.keys.Up):
		m.scrollOffset--
		m.autoScroll = false
		m.clampScroll()
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.scrollOffset++
		m.clampScroll()
		if m.isAtBottom() {
			m.autoScroll = true
		}
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.scrollOffset -= m.viewHeight
		m.autoScroll = false
		m.clampScroll()
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.scrollOffset += m.viewHeight
		m.clampScroll()
		if m.isAtBottom() {
			m.autoScroll = true
		}
		return m, nil

	case key.Matches(msg, m.keys.HalfPageUp):
		m.scrollOffset -= m.viewHeight / 2
		m.autoScroll = false
		m.clampScroll()
		return m, nil

	case key.Matches(msg, m.keys.HalfPageDown):
		m.scrollOffset += m.viewHeight / 2
		m.clampScroll()
		if m.isAtBottom() {
			m.autoScroll = true
		}
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.scrollOffset = 0
		m.autoScroll = false
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.scrollToBottom()
		m.autoScroll = true
		return m, nil

	// Level selection
	case key.Matches(msg, m.keys.LevelV):
		m.filter.SetMinLevel(logentry.LevelVerbose)
		m.refilter()
		m.statusMsg = "日志级别: ≥Verbose (全部)"
		return m, nil
	case key.Matches(msg, m.keys.LevelD):
		m.filter.SetMinLevel(logentry.LevelDebug)
		m.refilter()
		m.statusMsg = "日志级别: ≥Debug"
		return m, nil
	case key.Matches(msg, m.keys.LevelI):
		m.filter.SetMinLevel(logentry.LevelInfo)
		m.refilter()
		m.statusMsg = "日志级别: ≥Info"
		return m, nil
	case key.Matches(msg, m.keys.LevelW):
		m.filter.SetMinLevel(logentry.LevelWarn)
		m.refilter()
		m.statusMsg = "日志级别: ≥Warn"
		return m, nil
	case key.Matches(msg, m.keys.LevelE):
		m.filter.SetMinLevel(logentry.LevelError)
		m.refilter()
		m.statusMsg = "日志级别: ≥Error"
		return m, nil
	case key.Matches(msg, m.keys.LevelF):
		m.filter.SetMinLevel(logentry.LevelFatal)
		m.refilter()
		m.statusMsg = "日志级别: ≥Fatal"
		return m, nil
	}

	return m, nil
}

func (m AppModel) handleDevicePickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.inputMode = ModeNormal
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.deviceIdx > 0 {
			m.deviceIdx--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.deviceIdx < len(m.devices)-1 {
			m.deviceIdx++
		}
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		if m.deviceIdx < len(m.devices) {
			m.inputMode = ModeNormal
			return m, m.connectDevice(m.devices[m.deviceIdx])
		}
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func (m AppModel) handlePkgPickerKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.inputMode = ModeNormal
		return m, nil

	case key.Matches(msg, m.keys.Quit):
		m.inputMode = ModeNormal
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.pkgPickerIdx > 0 {
			m.pkgPickerIdx--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.pkgPickerIdx < len(m.filteredPackages)-1 {
			m.pkgPickerIdx++
		}
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		if len(m.filteredPackages) > 0 && m.pkgPickerIdx < len(m.filteredPackages) {
			selected := m.filteredPackages[m.pkgPickerIdx]
			if selected == "(清除过滤)" {
				m.filter.Package = ""
				m.statusMsg = "已清除包名过滤"
				m.inputMode = ModeNormal
				m.refilter()
				return m, nil
			}
			m.filter.Package = selected
			m.statusMsg = fmt.Sprintf("包名过滤: %s (正在刷新PID...)", selected)
			m.inputMode = ModeNormal
			// Refresh PIDs then refilter when PackagePIDMsg arrives
			serial := ""
			if m.deviceIdx < len(m.devices) {
				serial = m.devices[m.deviceIdx].Serial
			}
			return m, loadPackagePIDs(m.adbPath, serial)
		} else {
			m.inputMode = ModeNormal
		}
		return m, nil

	default:
		k := msg.String()
		if k == "backspace" {
			if len(m.pkgPickerSearch) > 0 {
				m.pkgPickerSearch = m.pkgPickerSearch[:len(m.pkgPickerSearch)-1]
			}
		} else if len(k) == 1 && k[0] >= 0x20 && k[0] <= 0x7e {
			m.pkgPickerSearch += k
		} else {
			return m, nil
		}
		m.filterPackageList()
		return m, nil
	}
}

func (m *AppModel) filterPackageList() {
	if m.pkgPickerSearch == "" {
		m.filteredPackages = m.allPackages
	} else {
		search := strings.ToLower(m.pkgPickerSearch)
		m.filteredPackages = nil
		for _, pkg := range m.allPackages {
			if strings.Contains(strings.ToLower(pkg), search) {
				m.filteredPackages = append(m.filteredPackages, pkg)
			}
		}
	}
	m.pkgPickerIdx = 0
}

func (m AppModel) handleFilterInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.inputMode = ModeNormal
		m.filterInput.Blur()
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		m.applyFilterInput()
		m.inputMode = ModeNormal
		m.filterInput.Blur()
		m.refilter()
		return m, nil
	}

	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

func (m *AppModel) applyFilterInput() {
	val := m.filterInput.Value()
	switch m.inputMode {
	case ModeSearch:
		m.filter.SetSearch(val, true)
	case ModeTagFilter:
		m.filter.Tag = val
	case ModePkgFilter:
		m.filter.Package = val
	case ModePidFilter:
		if pid, err := strconv.Atoi(val); err == nil {
			m.filter.PID = pid
		} else {
			m.filter.PID = 0
		}
	}
}

func (m *AppModel) refilter() {
	all := m.allEntries.All()
	m.filtered = m.filter.ApplyAll(all)
	m.filteredCount = len(m.filtered)
	if m.autoScroll {
		m.scrollToBottom()
	}
	m.clampScroll()
}

// --- Scroll helpers ---

func (m *AppModel) scrollToBottom() {
	maxOffset := len(m.filtered) - m.viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.scrollOffset = maxOffset
}

func (m *AppModel) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := len(m.filtered) - m.viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m *AppModel) isAtBottom() bool {
	maxOffset := len(m.filtered) - m.viewHeight
	if maxOffset < 0 {
		return true
	}
	return m.scrollOffset >= maxOffset
}

func (m AppModel) calcViewHeight() int {
	h := m.height - 3 // title(1) + filter(1) + status(1)
	if h < 1 {
		h = 1
	}
	return h
}

// --- Entry rendering ---

func (m *AppModel) preRenderEntry(e *logentry.Entry) {
	ts := e.Timestamp.Format("15:04:05.000")
	line := fmt.Sprintf("%-12s %5d %5d %s %-20s  %s",
		ts, e.PID, e.TID, e.Level.Char(), truncTag(e.Tag, 20), e.Message)
	style := levelStyle(e.Level)
	e.RenderedBase = style.Render(line)
	e.IsCrash = detectCrash(e)
}

func truncTag(tag string, maxLen int) string {
	if len(tag) <= maxLen {
		return tag
	}
	return tag[:maxLen-1] + "…"
}

func detectCrash(e *logentry.Entry) bool {
	if e.Level < logentry.LevelError {
		return false
	}
	msg := e.Message
	tag := e.Tag
	return strings.Contains(msg, "FATAL EXCEPTION") ||
		strings.Contains(tag, "AndroidRuntime") ||
		strings.Contains(msg, "ANR in") ||
		strings.HasPrefix(msg, "Process:") ||
		(len(msg) > 0 && msg[0] == '\t' && strings.HasPrefix(msg, "\tat "))
}

func (m AppModel) connectDevice(dev adb.Device) tea.Cmd {
	src := source.NewADBSource(m.adbPath, dev, m.logBuffer.String())
	return tea.Batch(
		startSourceCmd(src),
		loadPackagePIDs(m.adbPath, dev.Serial),
	)
}

func (m AppModel) toggleBookmark() AppModel {
	if len(m.filtered) == 0 {
		return m
	}
	idx := m.scrollOffset
	if idx < len(m.filtered) {
		entryIdx := m.filtered[idx].Index
		if m.bookmarks[entryIdx] {
			delete(m.bookmarks, entryIdx)
		} else {
			m.bookmarks[entryIdx] = true
		}
	}
	return m
}

func (m *AppModel) gotoNextBookmark(forward bool) {
	if len(m.bookmarks) == 0 || len(m.filtered) == 0 {
		return
	}

	current := m.scrollOffset
	if forward {
		for i := current + 1; i < len(m.filtered); i++ {
			if m.bookmarks[m.filtered[i].Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
		for i := 0; i <= current; i++ {
			if m.bookmarks[m.filtered[i].Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
	} else {
		for i := current - 1; i >= 0; i-- {
			if m.bookmarks[m.filtered[i].Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
		for i := len(m.filtered) - 1; i >= current; i-- {
			if m.bookmarks[m.filtered[i].Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
	}
}

func (m AppModel) copyCurrentLine() tea.Cmd {
	if len(m.filtered) == 0 || m.scrollOffset >= len(m.filtered) {
		return nil
	}
	line := m.filtered[m.scrollOffset].Raw
	return func() tea.Msg {
		copyToClipboard(line)
		return nil
	}
}

func levelStyle(l logentry.Level) lipgloss.Style {
	switch l {
	case logentry.LevelVerbose:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	case logentry.LevelDebug:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	case logentry.LevelInfo:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case logentry.LevelWarn:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case logentry.LevelError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case logentry.LevelFatal:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}

const bookmarkMarker = "🔖"
