package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/simley/logcatool/internal/adb"
	"github.com/simley/logcatool/internal/logentry"
	"github.com/simley/logcatool/internal/source"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
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
		if !m.ready {
			m.viewport = viewport.New(
				viewport.WithWidth(msg.Width),
				viewport.WithHeight(m.viewportHeight()),
			)
			m.ready = true
			m.rebuildContent()
		} else {
			m.viewport.SetWidth(msg.Width)
			m.viewport.SetHeight(m.viewportHeight())
		}

	case SourceStartedMsg:
		if m.source != nil {
			m.source.Stop()
		}
		m.source = msg.Source
		m.entryChan = msg.Entries
		m.errorChan = msg.Errors
		m.statusMsg = "已连接"
		cmds = append(cmds, waitForEntries(m.entryChan))

	case LogEntriesMsg:
		if !m.paused {
			for _, entry := range msg {
				entry.Index = m.totalCount
				m.totalCount++
				m.allEntries.Push(entry)
				if m.filter.Match(entry) {
					m.filtered = append(m.filtered, entry)
				}
			}
			m.filteredCount = len(m.filtered)
			m.rebuildContent()
			if m.autoScroll {
				m.viewport.GotoBottom()
			}
		}
		if m.entryChan != nil {
			cmds = append(cmds, waitForEntries(m.entryChan))
		}

	case LogStreamEndedMsg:
		m.statusMsg = "日志流已结束"

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

	case PackageListMsg:
		pkgs := []string(msg)
		m.allPackages = append([]string{"(清除过滤)"}, pkgs...)
		m.filteredPackages = m.allPackages
		m.pkgPickerIdx = 0
		m.pkgPickerSearch = ""
		m.inputMode = ModePkgPicker
		m.statusMsg = fmt.Sprintf("共 %d 个应用", len(pkgs))

	case ExportDoneMsg:
		m.statusMsg = fmt.Sprintf("已导出到 %s", msg.Path)

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	var vpCmd tea.Cmd
	if m.ready && m.inputMode == ModeNormal {
		m.viewport, vpCmd = m.viewport.Update(msg)
		cmds = append(cmds, vpCmd)
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
		m.bookmarks = make(map[int]bool)
		m.rebuildContent()
		m.statusMsg = "日志已清除"
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
			m.viewport.GotoBottom()
			m.statusMsg = "自动滚动: 开"
		} else {
			m.statusMsg = "自动滚动: 关"
		}
		return m, nil

	case key.Matches(msg, m.keys.WrapToggle):
		m.wrapLines = !m.wrapLines
		m.rebuildContent()
		if m.wrapLines {
			m.statusMsg = "换行: 开"
		} else {
			m.statusMsg = "换行: 关"
		}
		return m, nil

	case key.Matches(msg, m.keys.LevelV):
		m.filter.ToggleLevel(logentry.LevelVerbose)
		m.refilter()
		return m, nil
	case key.Matches(msg, m.keys.LevelD):
		m.filter.ToggleLevel(logentry.LevelDebug)
		m.refilter()
		return m, nil
	case key.Matches(msg, m.keys.LevelI):
		m.filter.ToggleLevel(logentry.LevelInfo)
		m.refilter()
		return m, nil
	case key.Matches(msg, m.keys.LevelW):
		m.filter.ToggleLevel(logentry.LevelWarn)
		m.refilter()
		return m, nil
	case key.Matches(msg, m.keys.LevelE):
		m.filter.ToggleLevel(logentry.LevelError)
		m.refilter()
		return m, nil
	case key.Matches(msg, m.keys.LevelF):
		m.filter.ToggleLevel(logentry.LevelFatal)
		m.refilter()
		return m, nil
	}

	// Pass to viewport
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
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
			} else {
				m.filter.Package = selected
				m.statusMsg = fmt.Sprintf("包名过滤: %s", selected)
			}
			m.inputMode = ModeNormal
			m.refilter()
		} else {
			m.inputMode = ModeNormal
		}
		return m, nil

	default:
		// Handle typing for fuzzy search
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
	m.rebuildContent()
}

func (m *AppModel) rebuildContent() {
	if !m.ready {
		return
	}
	content := renderLogEntries(m.filtered, m.filter, m.bookmarks, m.width)
	m.viewport.SetContent(content)
}

func (m AppModel) connectDevice(dev adb.Device) tea.Cmd {
	src := source.NewADBSource(m.adbPath, dev)
	return tea.Batch(
		startSourceCmd(src),
		loadPackagePIDs(m.adbPath, dev.Serial),
	)
}

func (m AppModel) toggleBookmark() AppModel {
	if !m.ready || len(m.filtered) == 0 {
		return m
	}
	// Bookmark the entry at the current viewport top line
	topLine := m.viewport.YOffset()
	if topLine < len(m.filtered) {
		idx := m.filtered[topLine].Index
		if m.bookmarks[idx] {
			delete(m.bookmarks, idx)
		} else {
			m.bookmarks[idx] = true
		}
		m.rebuildContent()
	}
	return m
}

func (m *AppModel) gotoNextBookmark(forward bool) {
	if len(m.bookmarks) == 0 || len(m.filtered) == 0 {
		return
	}

	current := m.viewport.YOffset()
	if forward {
		for i := current + 1; i < len(m.filtered); i++ {
			if m.bookmarks[m.filtered[i].Index] {
				m.viewport.SetYOffset(i)
				return
			}
		}
		// Wrap around
		for i := 0; i <= current; i++ {
			if m.bookmarks[m.filtered[i].Index] {
				m.viewport.SetYOffset(i)
				return
			}
		}
	} else {
		for i := current - 1; i >= 0; i-- {
			if m.bookmarks[m.filtered[i].Index] {
				m.viewport.SetYOffset(i)
				return
			}
		}
		for i := len(m.filtered) - 1; i >= current; i-- {
			if m.bookmarks[m.filtered[i].Index] {
				m.viewport.SetYOffset(i)
				return
			}
		}
	}
}

func (m AppModel) viewportHeight() int {
	h := m.height - 3 // title(1) + filter(1) + status(1)
	if h < 1 {
		h = 1
	}
	return h
}

// renderLogEntries builds the viewport content string.
func renderLogEntries(entries []*logentry.Entry, f *logentry.Filter, bookmarks map[int]bool, width int) string {
	if len(entries) == 0 {
		return "  等待日志..."
	}

	var b strings.Builder
	b.Grow(len(entries) * 120)

	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderEntry(e, f, bookmarks[e.Index], width))
	}
	return b.String()
}

func renderEntry(e *logentry.Entry, f *logentry.Filter, bookmarked bool, _ int) string {
	style := levelStyle(e.Level)

	ts := e.Timestamp.Format("15:04:05.000")
	line := fmt.Sprintf("%s %5d %5d %s %-20s: %s",
		ts, e.PID, e.TID, e.Level.Char(), e.Tag, e.Message)

	rendered := style.Render(line)

	if bookmarked {
		rendered = bookmarkMarker + rendered
	}

	// Highlight search terms
	if f != nil && f.SearchRe != nil {
		rendered = highlightSearch(rendered, f)
	}

	return rendered
}

func highlightSearch(line string, f *logentry.Filter) string {
	if f.SearchRe == nil {
		return line
	}
	// Simple highlight - just return as-is since ANSI-aware replacement is complex
	// The search match is indicated by the filter bar
	return line
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
