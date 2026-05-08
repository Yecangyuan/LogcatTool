package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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
		if m.paused {
			m.pausedBuffer = append(m.pausedBuffer, msg...)
		} else {
			m.ingestEntries(msg)
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
		m.rebuildPIDLookups()
		if m.filter.Package != "" || m.filter.Process != "" {
			m.refilter()
			m.scrollToBottom()
			m.autoScroll = false
			m.statusMsg = m.activeNameFilterStatus()
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
	case ModeStatsPanel:
		return m.handleStatsPanelKey(msg)
	default:
		if isFilterInputMode(m.inputMode) {
			return m.handleFilterInputKey(msg)
		}
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
			buffered := len(m.pausedBuffer)
			if buffered > 0 {
				m.ingestEntries(m.pausedBuffer)
				m.pausedBuffer = nil
				m.statusMsg = fmt.Sprintf("▶ 已恢复，补入 %d 条日志", buffered)
			} else {
				m.statusMsg = "▶ 已恢复"
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Clear):
		m.allEntries.Clear()
		m.filtered = nil
		m.displayRows = nil
		m.pausedBuffer = nil
		m.totalCount = 0
		m.filteredCount = 0
		m.displayCount = 0
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

	case key.Matches(msg, m.keys.TagExclude):
		m.inputMode = ModeTagExcludeFilter
		m.filterInput.Placeholder = "输入要排除的 Tag..."
		m.filterInput.SetValue(m.filter.TagExclude)
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

	case key.Matches(msg, m.keys.ProcFilter):
		if m.filePath != "" {
			m.statusMsg = "文件模式不支持进程名过滤"
			return m, nil
		}
		m.inputMode = ModeProcessFilter
		m.filterInput.Placeholder = "输入进程名..."
		m.filterInput.SetValue(m.filter.Process)
		m.filterInput.Focus()
		return m, nil

	case key.Matches(msg, m.keys.AlertKeyword):
		m.inputMode = ModeAlertKeyword
		m.filterInput.Placeholder = "输入告警关键词..."
		m.filterInput.SetValue(m.alertKeyword)
		m.filterInput.Focus()
		return m, nil

	case key.Matches(msg, m.keys.TimeRange):
		m.cycleTimeRange()
		m.refilter()
		return m, nil

	case key.Matches(msg, m.keys.StatsPanel):
		m.inputMode = ModeStatsPanel
		m.statsSelection = 0
		return m, nil

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

	case key.Matches(msg, m.keys.Favorite):
		m.toggleCurrentFavorite()
		return m, nil

	case key.Matches(msg, m.keys.PresetPrev):
		m.activePreset = (m.activePreset + len(m.presetSlots) - 1) % len(m.presetSlots)
		return m, m.applyActivePreset()

	case key.Matches(msg, m.keys.PresetNext):
		m.activePreset = (m.activePreset + 1) % len(m.presetSlots)
		return m, m.applyActivePreset()

	case key.Matches(msg, m.keys.PresetSave):
		m.presetSlots[m.activePreset] = filterPreset{
			Used:     true,
			Snapshot: m.filter.Snapshot(),
		}
		m.statusMsg = fmt.Sprintf("已保存到预设 %d: %s", m.activePreset+1, m.presetSummary(m.filter.Snapshot()))
		return m, nil

	case key.Matches(msg, m.keys.PresetClear):
		m.presetSlots[m.activePreset] = filterPreset{}
		m.statusMsg = fmt.Sprintf("已清空预设 %d", m.activePreset+1)
		return m, nil

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

	case key.Matches(msg, m.keys.Collapse):
		m.collapseDupes = !m.collapseDupes
		m.rebuildDisplayRows()
		if m.autoScroll {
			m.scrollToBottom()
		} else {
			m.clampScroll()
		}
		if m.collapseDupes {
			m.statusMsg = "重复折叠: 开"
		} else {
			m.statusMsg = "重复折叠: 关"
		}
		return m, nil

	case key.Matches(msg, m.keys.ToggleDetail):
		m.showDetails = !m.showDetails
		m.viewHeight = m.calcViewHeight()
		if m.autoScroll {
			m.scrollToBottom()
		} else {
			m.clampScroll()
		}
		if m.showDetails {
			m.statusMsg = "详情面板: 开"
		} else {
			m.statusMsg = "详情面板: 关"
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

	case key.Matches(msg, m.keys.CrashMode):
		m.filter.CrashOnly = !m.filter.CrashOnly
		m.refilter()
		if m.filter.CrashOnly {
			m.statusMsg = "崩溃模式: 开"
		} else {
			m.statusMsg = "崩溃模式: 关"
		}
		return m, nil

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

	case key.Matches(msg, m.keys.Favorite):
		if len(m.filteredPackages) == 0 || m.pkgPickerIdx >= len(m.filteredPackages) {
			return m, nil
		}
		selected := m.filteredPackages[m.pkgPickerIdx]
		if selected == "(清除过滤)" {
			return m, nil
		}
		if m.favoritePackages[selected] {
			delete(m.favoritePackages, selected)
			m.statusMsg = fmt.Sprintf("已取消收藏应用: %s", selected)
		} else {
			m.favoritePackages[selected] = true
			m.statusMsg = fmt.Sprintf("已收藏应用: %s", selected)
		}
		m.filterPackageList()
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

func (m AppModel) handleStatsPanelKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.buildStatsRows()
	if m.statsSelection >= len(rows) && len(rows) > 0 {
		m.statsSelection = len(rows) - 1
	}
	switch {
	case key.Matches(msg, m.keys.Cancel, m.keys.StatsPanel):
		m.inputMode = ModeNormal
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.statsSelection > 0 {
			m.statsSelection--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.statsSelection < len(rows)-1 {
			m.statsSelection++
		}
		return m, nil
	case key.Matches(msg, m.keys.Favorite):
		if len(rows) == 0 {
			return m, nil
		}
		m.toggleFavoriteForStatsRow(rows[m.statsSelection])
		return m, nil
	case key.Matches(msg, m.keys.Confirm):
		if len(rows) == 0 {
			m.inputMode = ModeNormal
			return m, nil
		}
		cmd := m.applyStatsRow(rows[m.statsSelection])
		m.inputMode = ModeNormal
		return m, cmd
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	}
	return m, nil
}

func (m *AppModel) filterPackageList() {
	if m.pkgPickerSearch == "" {
		m.filteredPackages = append([]string(nil), m.allPackages...)
	} else {
		search := strings.ToLower(m.pkgPickerSearch)
		m.filteredPackages = nil
		for _, pkg := range m.allPackages {
			if strings.Contains(strings.ToLower(pkg), search) {
				m.filteredPackages = append(m.filteredPackages, pkg)
			}
		}
	}
	sort.SliceStable(m.filteredPackages, func(i, j int) bool {
		left, right := m.filteredPackages[i], m.filteredPackages[j]
		if left == "(清除过滤)" || right == "(清除过滤)" {
			return left == "(清除过滤)"
		}
		lf, rf := m.favoritePackages[left], m.favoritePackages[right]
		if lf != rf {
			return lf
		}
		return left < right
	})
	m.pkgPickerIdx = 0
}

func (m AppModel) handleFilterInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Cancel):
		m.inputMode = ModeNormal
		m.filterInput.Blur()
		return m, nil

	case key.Matches(msg, m.keys.Confirm):
		mode := m.inputMode
		m.applyFilterInput()
		m.inputMode = ModeNormal
		m.filterInput.Blur()
		if mode == ModeProcessFilter {
			if m.filter.Process == "" {
				m.refilter()
				m.statusMsg = "已清除进程过滤"
				return m, nil
			}
			serial := ""
			if m.deviceIdx < len(m.devices) {
				serial = m.devices[m.deviceIdx].Serial
			}
			m.statusMsg = fmt.Sprintf("进程过滤: %s (正在刷新PID...)", m.filter.Process)
			return m, loadPackagePIDs(m.adbPath, serial)
		}
		if mode == ModeAlertKeyword {
			if m.alertKeyword == "" {
				m.statusMsg = "已清除告警关键词"
			} else {
				m.statusMsg = fmt.Sprintf("告警关键词: %s", m.alertKeyword)
			}
			return m, nil
		}
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
	case ModeTagExcludeFilter:
		m.filter.TagExclude = val
	case ModePkgFilter:
		m.filter.Package = val
	case ModePidFilter:
		if pid, err := strconv.Atoi(val); err == nil {
			m.filter.PID = pid
		} else {
			m.filter.PID = 0
		}
	case ModeProcessFilter:
		m.filter.Process = val
	case ModeAlertKeyword:
		m.alertKeyword = val
	}
}

func (m *AppModel) refilter() {
	all := m.allEntries.All()
	if len(all) > 0 {
		m.filter.ReferenceTime = all[len(all)-1].Timestamp
	}
	m.filtered = m.filter.ApplyAll(all)
	m.filteredCount = len(m.filtered)
	m.rebuildDisplayRows()
	if m.autoScroll {
		m.scrollToBottom()
	}
	m.clampScroll()
}

func (m *AppModel) ingestEntries(entries []*logentry.Entry) {
	if len(entries) == 0 {
		return
	}
	latest := entries[len(entries)-1].Timestamp
	for _, entry := range entries {
		entry.Index = m.totalCount
		m.totalCount++
		m.preRenderEntry(entry)
		m.allEntries.Push(entry)
		m.maybeTriggerAlert(entry)
	}
	m.filter.ReferenceTime = latest
	if m.filter.TimeWindow > 0 {
		m.refilter()
		return
	}
	for _, entry := range entries {
		if m.filter.Match(entry) {
			m.filtered = append(m.filtered, entry)
			m.appendDisplayRow(entry)
		}
	}
	m.filteredCount = len(m.filtered)
	m.displayCount = len(m.displayRows)
	if m.totalCount > m.allEntries.Cap() && len(m.filtered) > m.allEntries.Cap()*2 {
		m.refilter()
	}
	if m.autoScroll {
		m.scrollToBottom()
	}
}

func (m *AppModel) rebuildDisplayRows() {
	m.displayRows = nil
	if !m.collapseDupes {
		m.displayRows = make([]displayRow, 0, len(m.filtered))
		for _, entry := range m.filtered {
			m.displayRows = append(m.displayRows, displayRow{Entry: entry, Count: 1})
		}
		m.displayCount = len(m.displayRows)
		return
	}

	for _, entry := range m.filtered {
		m.appendDisplayRow(entry)
	}
	m.displayCount = len(m.displayRows)
}

func (m *AppModel) appendDisplayRow(entry *logentry.Entry) {
	if entry == nil {
		return
	}
	if !m.collapseDupes || len(m.displayRows) == 0 {
		m.displayRows = append(m.displayRows, displayRow{Entry: entry, Count: 1})
		m.displayCount = len(m.displayRows)
		return
	}

	last := &m.displayRows[len(m.displayRows)-1]
	if canFold(last.Entry, entry) {
		last.Entry = entry
		last.Count++
	} else {
		m.displayRows = append(m.displayRows, displayRow{Entry: entry, Count: 1})
	}
	m.displayCount = len(m.displayRows)
}

func canFold(a, b *logentry.Entry) bool {
	if a == nil || b == nil {
		return false
	}
	return a.PID == b.PID &&
		a.TID == b.TID &&
		a.Level == b.Level &&
		a.Tag == b.Tag &&
		a.Message == b.Message
}

func (m *AppModel) rebuildPIDLookups() {
	m.processByPID = make(map[int]string, len(m.filter.PIDsByPkg))
	m.packageByPID = make(map[int]string, len(m.filter.PIDsByPkg))
	for name, pids := range m.filter.PIDsByPkg {
		pkg := packageNameFromProcess(name)
		for _, pid := range pids {
			m.processByPID[pid] = name
			m.packageByPID[pid] = pkg
		}
	}
}

func packageNameFromProcess(name string) string {
	if idx := strings.IndexByte(name, ':'); idx >= 0 {
		return name[:idx]
	}
	return name
}

func (m *AppModel) cycleTimeRange() {
	ranges := []time.Duration{0, 10 * time.Second, time.Minute, 5 * time.Minute}
	next := 0
	for i, d := range ranges {
		if m.filter.TimeWindow == d {
			next = (i + 1) % len(ranges)
			break
		}
	}
	m.filter.TimeWindow = ranges[next]
	if m.filter.TimeWindow == 0 {
		m.statusMsg = "时间范围: 全部"
		return
	}
	m.statusMsg = fmt.Sprintf("时间范围: 最近 %s", formatDurationLabel(m.filter.TimeWindow))
}

func formatDurationLabel(d time.Duration) string {
	switch d {
	case 10 * time.Second:
		return "10秒"
	case time.Minute:
		return "1分钟"
	case 5 * time.Minute:
		return "5分钟"
	default:
		return d.String()
	}
}

func (m *AppModel) maybeTriggerAlert(entry *logentry.Entry) {
	if entry == nil {
		return
	}
	if entry.IsCrash {
		m.lastAlert = fmt.Sprintf("Crash %s", truncTag(entry.Tag, 18))
		return
	}
	if m.alertKeyword == "" {
		return
	}
	if containsFoldLocal(entry.Tag, m.alertKeyword) || containsFoldLocal(entry.Message, m.alertKeyword) {
		m.lastAlert = fmt.Sprintf("%s 命中 %s", truncTag(entry.Tag, 14), m.alertKeyword)
	}
}

func (m *AppModel) toggleCurrentFavorite() {
	row := m.currentDisplayRow()
	if row == nil || row.Entry == nil {
		m.statusMsg = "没有可收藏的当前日志"
		return
	}
	process := m.processByPID[row.Entry.PID]
	if process != "" {
		if m.favoriteProcesses[process] {
			delete(m.favoriteProcesses, process)
			m.statusMsg = fmt.Sprintf("已取消收藏进程: %s", process)
		} else {
			m.favoriteProcesses[process] = true
			m.statusMsg = fmt.Sprintf("已收藏进程: %s", process)
		}
		return
	}
	pkg := m.packageByPID[row.Entry.PID]
	if pkg != "" {
		if m.favoritePackages[pkg] {
			delete(m.favoritePackages, pkg)
			m.statusMsg = fmt.Sprintf("已取消收藏应用: %s", pkg)
		} else {
			m.favoritePackages[pkg] = true
			m.statusMsg = fmt.Sprintf("已收藏应用: %s", pkg)
		}
		return
	}
	m.statusMsg = "当前日志没有可识别的应用/进程"
}

func (m *AppModel) toggleFavoriteForStatsRow(row statsRow) {
	switch row.Kind {
	case statsPackage:
		if m.favoritePackages[row.Value] {
			delete(m.favoritePackages, row.Value)
			m.statusMsg = fmt.Sprintf("已取消收藏应用: %s", row.Value)
		} else {
			m.favoritePackages[row.Value] = true
			m.statusMsg = fmt.Sprintf("已收藏应用: %s", row.Value)
		}
	case statsProcess:
		if m.favoriteProcesses[row.Value] {
			delete(m.favoriteProcesses, row.Value)
			m.statusMsg = fmt.Sprintf("已取消收藏进程: %s", row.Value)
		} else {
			m.favoriteProcesses[row.Value] = true
			m.statusMsg = fmt.Sprintf("已收藏进程: %s", row.Value)
		}
	default:
		m.statusMsg = "该统计项不支持收藏"
	}
}

func (m *AppModel) applyStatsRow(row statsRow) tea.Cmd {
	switch row.Kind {
	case statsLevel:
		m.filter.SetMinLevel(row.Level)
		m.statusMsg = fmt.Sprintf("统计筛选: ≥%s", row.Level.Label())
	case statsTag:
		m.filter.Tag = row.Value
		m.statusMsg = fmt.Sprintf("统计筛选 Tag: %s", row.Value)
	case statsPackage:
		m.filter.Package = row.Value
		m.statusMsg = fmt.Sprintf("统计筛选包名: %s", row.Value)
	case statsProcess:
		m.filter.Process = row.Value
		m.statusMsg = fmt.Sprintf("统计筛选进程: %s", row.Value)
	}
	m.refilter()
	if m.filePath == "" && (row.Kind == statsPackage || row.Kind == statsProcess) {
		return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
	}
	return nil
}

func (m AppModel) buildStatsRows() []statsRow {
	levelCounts := make(map[logentry.Level]int)
	tagCounts := make(map[string]int)
	processCounts := make(map[string]int)
	packageCounts := make(map[string]int)

	for _, entry := range m.filtered {
		levelCounts[entry.Level]++
		if entry.Tag != "" {
			tagCounts[entry.Tag]++
		}
		if process := m.processByPID[entry.PID]; process != "" {
			processCounts[process]++
			packageCounts[packageNameFromProcess(process)]++
		} else if pkg := m.packageByPID[entry.PID]; pkg != "" {
			packageCounts[pkg]++
		}
	}

	rows := make([]statsRow, 0, 24)
	for _, level := range logentry.FilterableLevels {
		if count := levelCounts[level]; count > 0 {
			rows = append(rows, statsRow{
				Kind:    statsLevel,
				Section: "级别",
				Label:   level.Label(),
				Value:   level.Label(),
				Count:   count,
				Level:   level,
			})
		}
	}
	rows = append(rows, topCountRows("Tag", statsTag, tagCounts, nil)...)
	rows = append(rows, topCountRows("进程", statsProcess, processCounts, m.favoriteProcesses)...)
	rows = append(rows, topCountRows("应用", statsPackage, packageCounts, m.favoritePackages)...)
	return rows
}

func topCountRows(section string, kind statsKind, counts map[string]int, favorites map[string]bool) []statsRow {
	type item struct {
		Name     string
		Count    int
		Favorite bool
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		items = append(items, item{Name: name, Count: count, Favorite: favorites != nil && favorites[name]})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Favorite != items[j].Favorite {
			return items[i].Favorite
		}
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > 5 {
		items = items[:5]
	}
	rows := make([]statsRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, statsRow{
			Kind:     kind,
			Section:  section,
			Label:    item.Name,
			Value:    item.Name,
			Count:    item.Count,
			Favorite: item.Favorite,
		})
	}
	return rows
}

func containsFoldLocal(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func (m AppModel) activeNameFilterStatus() string {
	switch {
	case m.filter.Package != "" && m.filter.Process != "":
		return fmt.Sprintf("包名:%s  进程:%s", m.filter.Package, m.filter.Process)
	case m.filter.Package != "":
		return fmt.Sprintf("包名过滤: %s", m.filter.Package)
	case m.filter.Process != "":
		return fmt.Sprintf("进程过滤: %s", m.filter.Process)
	default:
		return "进程列表已刷新"
	}
}

func (m AppModel) presetSummary(s logentry.Snapshot) string {
	var parts []string
	if s.CrashOnly {
		parts = append(parts, "崩溃")
	}
	if s.TimeWindow > 0 {
		parts = append(parts, "时段:"+formatDurationLabel(s.TimeWindow))
	}
	if s.MinLevel > logentry.LevelVerbose {
		parts = append(parts, "≥"+s.MinLevel.String())
	}
	if s.Package != "" {
		parts = append(parts, "包:"+s.Package)
	}
	if s.Process != "" {
		parts = append(parts, "进程:"+s.Process)
	}
	if s.Tag != "" {
		parts = append(parts, "Tag:"+s.Tag)
	}
	if s.TagExclude != "" {
		parts = append(parts, "排除:"+s.TagExclude)
	}
	if s.PID > 0 {
		parts = append(parts, fmt.Sprintf("PID:%d", s.PID))
	}
	if s.SearchText != "" {
		parts = append(parts, "搜:"+s.SearchText)
	}
	if len(parts) == 0 {
		return "全部日志"
	}
	return strings.Join(parts, " ")
}

func (m *AppModel) applyActivePreset() tea.Cmd {
	slot := m.presetSlots[m.activePreset]
	if !slot.Used {
		m.statusMsg = fmt.Sprintf("预设 %d 为空", m.activePreset+1)
		return nil
	}

	m.filter.ApplySnapshot(slot.Snapshot)
	m.refilter()
	m.statusMsg = fmt.Sprintf("已应用预设 %d: %s", m.activePreset+1, m.presetSummary(slot.Snapshot))

	if m.filePath == "" && (m.filter.Package != "" || m.filter.Process != "") {
		return loadPackagePIDs(m.adbPath, m.currentDeviceSerial())
	}
	return nil
}

func (m AppModel) currentDeviceSerial() string {
	if m.deviceIdx < len(m.devices) {
		return m.devices[m.deviceIdx].Serial
	}
	return ""
}

func (m AppModel) currentDisplayRow() *displayRow {
	if len(m.displayRows) == 0 {
		return nil
	}
	idx := m.scrollOffset
	if idx < 0 {
		idx = 0
	}
	if idx >= len(m.displayRows) {
		idx = len(m.displayRows) - 1
	}
	return &m.displayRows[idx]
}

// --- Scroll helpers ---

func (m *AppModel) scrollToBottom() {
	maxOffset := len(m.displayRows) - m.viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.scrollOffset = maxOffset
}

func (m *AppModel) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := len(m.displayRows) - m.viewHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

func (m *AppModel) isAtBottom() bool {
	maxOffset := len(m.displayRows) - m.viewHeight
	if maxOffset < 0 {
		return true
	}
	return m.scrollOffset >= maxOffset
}

func (m AppModel) calcViewHeight() int {
	h := m.height - 3 // title(1) + filter(1) + status(1)
	if m.showDetails {
		h -= detailPaneHeight
	}
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
	row := m.currentDisplayRow()
	if row == nil || row.Entry == nil {
		return m
	}
	entryIdx := row.Entry.Index
	if m.bookmarks[entryIdx] {
		delete(m.bookmarks, entryIdx)
	} else {
		m.bookmarks[entryIdx] = true
	}
	return m
}

func (m *AppModel) gotoNextBookmark(forward bool) {
	if len(m.bookmarks) == 0 || len(m.displayRows) == 0 {
		return
	}

	current := m.scrollOffset
	if forward {
		for i := current + 1; i < len(m.displayRows); i++ {
			if m.bookmarks[m.displayRows[i].Entry.Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
		for i := 0; i <= current; i++ {
			if m.bookmarks[m.displayRows[i].Entry.Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
	} else {
		for i := current - 1; i >= 0; i-- {
			if m.bookmarks[m.displayRows[i].Entry.Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
		for i := len(m.displayRows) - 1; i >= current; i-- {
			if m.bookmarks[m.displayRows[i].Entry.Index] {
				m.scrollOffset = i
				m.autoScroll = false
				return
			}
		}
	}
}

func (m AppModel) copyCurrentLine() tea.Cmd {
	row := m.currentDisplayRow()
	if row == nil || row.Entry == nil {
		return nil
	}
	line := row.Entry.Raw
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
