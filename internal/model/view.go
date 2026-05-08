package model

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/Yecangyuan/LogcatTool/internal/logentry"
	"github.com/Yecangyuan/LogcatTool/internal/ui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var crashStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("52")).
	Foreground(lipgloss.Color("196")).
	Bold(true)

var searchHighlightStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("214")).
	Foreground(lipgloss.Color("0")).
	Bold(true)

const detailPaneHeight = 7

func (m AppModel) View() tea.View {
	var v tea.View
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion

	if !m.ready {
		v.SetContent("  正在初始化 LogCaTool...")
		return v
	}

	if m.showHelp {
		v.SetContent(m.renderHelp())
		return v
	}

	var sections []string
	sections = append(sections, m.renderTitleBar())
	sections = append(sections, m.renderFilterBar())
	sections = append(sections, m.renderLogView())
	if m.showDetails {
		sections = append(sections, m.renderDetailPane())
	}
	sections = append(sections, m.renderStatusBar())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.inputMode == ModeDevicePicker {
		content = m.overlayDevicePicker(content)
	}
	if m.inputMode == ModePkgPicker {
		content = m.overlayPkgPicker(content)
	}

	v.SetContent(content)
	if m.inputMode >= ModeSearch && m.inputMode <= ModeProcessFilter {
		c := m.filterInput.Cursor()
		if c != nil {
			c.Y += 1 // offset for title bar
			v.Cursor = c
		}
	}
	return v
}

// renderLogView renders only the visible entries via virtual scroll.
func (m AppModel) renderLogView() string {
	if m.viewHeight <= 0 {
		return ""
	}

	n := len(m.displayRows)
	if n == 0 {
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Render("  暂无日志...")
		padLines := m.viewHeight - 1
		if padLines < 0 {
			padLines = 0
		}
		return emptyMsg + strings.Repeat("\n", padLines)
	}

	start := m.scrollOffset
	end := start + m.viewHeight
	if end > n {
		end = n
	}

	var sb strings.Builder
	hasSearch := m.filter.SearchRe != nil || m.filter.SearchText != ""

	for i := start; i < end; i++ {
		row := m.displayRows[i]
		entry := row.Entry

		var line string
		if entry.IsCrash {
			line = m.renderCrashLine(entry)
		} else if hasSearch {
			line = m.highlightSearchInLine(entry)
		} else {
			line = entry.RenderedBase
		}

		if row.Count > 1 {
			line += lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(
				fmt.Sprintf(" ×%d", row.Count),
			)
		}

		line = rowPrefix(i == m.scrollOffset, m.bookmarks[entry.Index]) + line

		if !m.wrapLines && m.width > 0 {
			lineWidth := lipgloss.Width(line)
			if lineWidth > m.width {
				line = truncateToWidth(line, m.width)
			}
		}

		sb.WriteString(line)
		if i < end-1 {
			sb.WriteByte('\n')
		}
	}

	// Pad remaining lines if fewer visible entries than viewHeight
	rendered := end - start
	if rendered < m.viewHeight {
		for i := 0; i < m.viewHeight-rendered; i++ {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func (m AppModel) renderDetailPane() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	row := m.currentDisplayRow()
	if row == nil || row.Entry == nil {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("244")).
			Width(width).
			Height(detailPaneHeight).
			Render("  详情面板：暂无选中日志")
	}

	entry := row.Entry
	header := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Bold(true).
		Width(width).
		Render(fmt.Sprintf("  详情  PID:%d  TID:%d  级别:%s  折叠:%d", entry.PID, entry.TID, entry.Level.Char(), row.Count))

	lines := []string{
		header,
		fmt.Sprintf("  时间: %s", entry.Timestamp.Format("2006-01-02 15:04:05.000")),
		fmt.Sprintf("  Tag : %s", entry.Tag),
	}
	lines = append(lines, wrapText("  原始: "+entry.Raw, width, detailPaneHeight-len(lines))...)
	for len(lines) < detailPaneHeight {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Width(width).
		Render(strings.Join(lines[:detailPaneHeight], "\n"))
}

func (m AppModel) renderCrashLine(e *logentry.Entry) string {
	ts := e.Timestamp.Format("15:04:05.000")
	line := fmt.Sprintf("%-12s %5d %5d %s %-20s  %s",
		ts, e.PID, e.TID, e.Level.Char(), truncTag(e.Tag, 20), e.Message)
	return crashStyle.Render(line)
}

func (m AppModel) highlightSearchInLine(e *logentry.Entry) string {
	if m.filter.SearchRe != nil {
		raw := e.RenderedBase
		// For regex search, highlight in the message portion
		// Since RenderedBase is styled, we re-render with highlights
		ts := e.Timestamp.Format("15:04:05.000")
		prefix := fmt.Sprintf("%-12s %5d %5d %s %-20s  ",
			ts, e.PID, e.TID, e.Level.Char(), truncTag(e.Tag, 20))

		msg := e.Message
		highlighted := m.filter.SearchRe.ReplaceAllStringFunc(msg, func(match string) string {
			return searchHighlightStyle.Render(match)
		})

		if highlighted != msg {
			style := levelStyle(e.Level)
			return style.Render(prefix) + highlighted
		}
		return raw
	}

	if m.filter.SearchText != "" {
		ts := e.Timestamp.Format("15:04:05.000")
		prefix := fmt.Sprintf("%-12s %5d %5d %s %-20s  ",
			ts, e.PID, e.TID, e.Level.Char(), truncTag(e.Tag, 20))

		msg := e.Message
		highlighted := highlightSubstring(msg, m.filter.SearchText)
		if highlighted != msg {
			style := levelStyle(e.Level)
			return style.Render(prefix) + highlighted
		}
		return e.RenderedBase
	}

	return e.RenderedBase
}

// highlightSubstring highlights all case-insensitive occurrences of substr in s.
func highlightSubstring(s, substr string) string {
	lower := strings.ToLower(s)
	lowerSub := strings.ToLower(substr)
	subLen := len(lowerSub)

	var sb strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], lowerSub)
		if idx < 0 {
			sb.WriteString(s[pos:])
			break
		}
		sb.WriteString(s[pos : pos+idx])
		sb.WriteString(searchHighlightStyle.Render(s[pos+idx : pos+idx+subLen]))
		pos += idx + subLen
	}
	return sb.String()
}

func (m AppModel) renderTitleBar() string {
	title := ui.TitleStyle.Render(" LogCaTool ")

	var deviceInfo string
	if m.source != nil {
		deviceInfo = fmt.Sprintf(" 设备: %s", m.source.Name())
	} else if m.filePath != "" {
		deviceInfo = fmt.Sprintf(" 文件: %s", m.filePath)
	} else {
		deviceInfo = " 未连接"
	}

	// Show buffer selection
	if m.filePath == "" {
		deviceInfo += fmt.Sprintf(" [%s]", m.logBuffer.Label())
	}

	var status string
	if m.paused {
		status = ui.PausedStyle.Render(" ⏸ 暂停 ")
	}
	if m.reconnecting {
		status += lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(
			fmt.Sprintf(" 🔄 %ds ", m.reconnectSecs))
	}

	left := title + deviceInfo
	right := status
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	bar := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("252")).
		Width(m.width).
		Render(left + strings.Repeat(" ", gap) + right)

	return bar
}

func (m AppModel) renderFilterBar() string {
	var parts []string

	// Level selector (single-select: shows minimum level)
	for i, level := range logentry.FilterableLevels {
		num := fmt.Sprintf("%d", i+1)
		char := level.Char()
		if level == m.filter.MinLevel {
			style := ui.LevelBtnActiveStyle.Foreground(levelColor(level))
			parts = append(parts, style.Render(fmt.Sprintf("[%s:%s]", num, char)))
		} else if m.filter.IsLevelEnabled(level) {
			style := lipgloss.NewStyle().Foreground(levelColor(level))
			parts = append(parts, style.Render(fmt.Sprintf(" %s:%s ", num, char)))
		} else {
			parts = append(parts, ui.LevelBtnInactiveStyle.Render(fmt.Sprintf("[%s:%s]", num, char)))
		}
	}

	// Active filter input or display
	if m.inputMode >= ModeSearch && m.inputMode <= ModeProcessFilter {
		label := filterModeLabel(m.inputMode)
		parts = append(parts, ui.FilterLabelStyle.Render(" "+label+": "))
		parts = append(parts, m.filterInput.View())
	} else {
		if m.filter.SearchText != "" {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" 搜索:%s", m.filter.SearchText)))
		}
		if m.filter.Tag != "" {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" Tag:%s", m.filter.Tag)))
		}
		if m.filter.Package != "" {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" 包名:%s", m.filter.Package)))
		}
		if m.filter.Process != "" {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" 进程:%s", m.filter.Process)))
		}
		if m.filter.CrashOnly {
			parts = append(parts, ui.FilterActiveStyle.Render(" 崩溃模式"))
		}
		if m.filter.PID > 0 {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" PID:%d", m.filter.PID)))
		}
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(
		fmt.Sprintf(" 预设:%d%s", m.activePreset+1, presetMark(m.presetSlots[m.activePreset].Used)),
	))

	content := strings.Join(parts, " ")
	return ui.FilterBarStyle.Width(m.width).Render(content)
}

func (m AppModel) renderStatusBar() string {
	left := fmt.Sprintf(" 总计:%d  匹配:%d  行:%d", m.totalCount, m.filteredCount, m.displayCount)

	// Scroll indicator
	if len(m.displayRows) > 0 && m.viewHeight > 0 {
		pct := 0
		maxOffset := len(m.displayRows) - m.viewHeight
		if maxOffset > 0 {
			pct = (m.scrollOffset * 100) / maxOffset
			if pct > 100 {
				pct = 100
			}
		} else {
			pct = 100
		}
		left += fmt.Sprintf("  位置:%d/%d (%d%%)", m.scrollOffset+1, len(m.displayRows), pct)
	}

	if m.autoScroll {
		left += "  ▼自动滚动"
	} else {
		left += "  ■手动浏览(G回底部)"
	}
	if m.paused {
		left += "  ⏸暂停中"
		if len(m.pausedBuffer) > 0 {
			left += fmt.Sprintf("  缓冲:%d", len(m.pausedBuffer))
		}
	}
	if len(m.bookmarks) > 0 {
		left += fmt.Sprintf("  🔖%d", len(m.bookmarks))
	}
	if m.collapseDupes {
		left += "  ⇣折叠"
	}
	if m.showDetails {
		left += "  ◫详情"
	}

	right := " /:搜索 Space:暂停 ?:帮助 q:退出 "
	if m.statusMsg != "" {
		right = " " + m.statusMsg + " │" + right
	}

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}

	return ui.StatusBarStyle.Width(m.width).Render(
		left + strings.Repeat(" ", gap) + right)
}

func (m AppModel) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(ui.HelpTitleStyle.Render("\n  LogCaTool 帮助\n\n"))
	sb.WriteString("  导航:\n")
	sb.WriteString("    ↑/k         向上滚动\n")
	sb.WriteString("    ↓/j         向下滚动\n")
	sb.WriteString("    PgUp/Ctrl+u 上翻页\n")
	sb.WriteString("    PgDn/Ctrl+d 下翻页\n")
	sb.WriteString("    g/Home      顶部\n")
	sb.WriteString("    G/End       底部\n")
	sb.WriteString("\n  过滤:\n")
	sb.WriteString("    /           搜索 (支持正则)\n")
	sb.WriteString("    t           Tag 过滤\n")
	sb.WriteString("    p           包名过滤 (下拉选择)\n")
	sb.WriteString("    P           进程名过滤\n")
	sb.WriteString("    i           PID 过滤\n")
	sb.WriteString("    x           崩溃模式\n")
	sb.WriteString("    1-6         选择最低日志级别 V/D/I/W/E/F\n")
	sb.WriteString("\n  操作:\n")
	sb.WriteString("    Space       暂停/恢复日志流\n")
	sb.WriteString("    c           清除日志 (同时清除设备缓冲区)\n")
	sb.WriteString("    d           选择设备\n")
	sb.WriteString("    e           导出日志\n")
	sb.WriteString("    b           添加/移除书签\n")
	sb.WriteString("    n/N         下一个/上一个书签\n")
	sb.WriteString("    y           复制当前行到剪贴板\n")
	sb.WriteString("    B           切换日志缓冲区\n")
	sb.WriteString("    z           折叠连续重复日志\n")
	sb.WriteString("    v           切换详情面板\n")
	sb.WriteString("    [ / ]       切换预设槽并加载\n")
	sb.WriteString("    m / M       保存/清空当前预设槽\n")
	sb.WriteString("    w           切换换行模式\n")
	sb.WriteString("    s           切换自动滚动\n")
	sb.WriteString("    ?           显示/隐藏帮助\n")
	sb.WriteString("    q/Ctrl+c    退出\n")
	sb.WriteString("\n  按 ? 返回\n")
	return sb.String()
}

func (m AppModel) overlayDevicePicker(bg string) string {
	if len(m.devices) == 0 {
		return bg
	}

	var sb strings.Builder
	sb.WriteString(ui.HelpTitleStyle.Render("选择设备") + "\n\n")

	for i, d := range m.devices {
		cursor := "  "
		if i == m.deviceIdx {
			cursor = "▸ "
			sb.WriteString(ui.DeviceSelectedStyle.Render(cursor + d.DisplayName()))
		} else {
			sb.WriteString(ui.DeviceNormalStyle.Render(cursor + d.DisplayName()))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("↑↓选择 Enter确认 Esc取消"))

	picker := ui.DevicePickerStyle.Render(sb.String())

	pickerW := lipgloss.Width(picker)
	pickerH := lipgloss.Height(picker)
	x := (m.width - pickerW) / 2
	y := (m.height - pickerH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return placeOverlay(x, y, picker, bg)
}

func (m AppModel) overlayPkgPicker(bg string) string {
	maxVisible := 15
	var sb strings.Builder

	sb.WriteString(ui.HelpTitleStyle.Render("选择应用包名") + "\n")

	searchDisplay := m.pkgPickerSearch
	if searchDisplay == "" {
		searchDisplay = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("输入过滤...")
	}
	sb.WriteString(fmt.Sprintf("  🔍 %s\n\n", searchDisplay))

	if len(m.filteredPackages) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  没有匹配的应用"))
		sb.WriteString("\n")
	} else {
		startIdx := 0
		if m.pkgPickerIdx >= maxVisible {
			startIdx = m.pkgPickerIdx - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(m.filteredPackages) {
			endIdx = len(m.filteredPackages)
		}

		if startIdx > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  ▲ 更多...\n"))
		}

		for i := startIdx; i < endIdx; i++ {
			pkg := m.filteredPackages[i]
			cursor := "  "
			if i == m.pkgPickerIdx {
				cursor = "▸ "
				sb.WriteString(ui.DeviceSelectedStyle.Render(cursor + pkg))
			} else {
				sb.WriteString(ui.DeviceNormalStyle.Render(cursor + pkg))
			}
			sb.WriteString("\n")
		}

		if endIdx < len(m.filteredPackages) {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  ▼ 更多...\n"))
		}

		sb.WriteString(fmt.Sprintf("\n  %d/%d", m.pkgPickerIdx+1, len(m.filteredPackages)))
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("  j/k选择 Enter确认 Esc取消 输入搜索"))

	picker := ui.DevicePickerStyle.Render(sb.String())

	pickerW := lipgloss.Width(picker)
	pickerH := lipgloss.Height(picker)
	x := (m.width - pickerW) / 2
	y := (m.height - pickerH) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return placeOverlay(x, y, picker, bg)
}

func placeOverlay(x, y int, overlay, bg string) string {
	bgLines := strings.Split(bg, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, line := range overlayLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bgLine := bgLines[row]
		bgWidth := lipgloss.Width(bgLine)
		overlayWidth := lipgloss.Width(line)

		if x >= bgWidth {
			bgLines[row] = bgLine + strings.Repeat(" ", x-bgWidth) + line
		} else {
			left := truncateToWidth(bgLine, x)
			rightStart := x + overlayWidth
			right := ""
			if rightStart < bgWidth {
				right = skipToWidth(bgLine, rightStart)
			}
			bgLines[row] = left + line + right
		}
	}
	return strings.Join(bgLines, "\n")
}

func truncateToWidth(s string, w int) string {
	runes := []rune(s)
	if w >= len(runes) {
		return s
	}
	return string(runes[:w])
}

func skipToWidth(s string, w int) string {
	runes := []rune(s)
	if w >= len(runes) {
		return ""
	}
	return string(runes[w:])
}

func wrapText(s string, width, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if width < 8 {
		width = 8
	}
	runes := []rune(s)
	chunk := width - 2
	if chunk < 1 {
		chunk = 1
	}

	lines := make([]string, 0, maxLines)
	for len(runes) > 0 && len(lines) < maxLines {
		take := chunk
		if take > len(runes) {
			take = len(runes)
		}
		lines = append(lines, string(runes[:take]))
		runes = runes[take:]
	}
	if len(runes) > 0 && len(lines) > 0 {
		last := []rune(lines[len(lines)-1])
		if len(last) > chunk-1 {
			last = last[:chunk-1]
		}
		lines[len(lines)-1] = string(last) + "…"
	}
	return lines
}

func rowPrefix(selected, bookmarked bool) string {
	switch {
	case selected && bookmarked:
		return "❯🔖 "
	case selected:
		return "❯ "
	case bookmarked:
		return "🔖 "
	default:
		return "  "
	}
}

func presetMark(used bool) string {
	if used {
		return "★"
	}
	return "·"
}

func filterModeLabel(mode InputMode) string {
	switch mode {
	case ModeSearch:
		return "搜索"
	case ModeTagFilter:
		return "Tag"
	case ModePkgFilter:
		return "包名"
	case ModeProcessFilter:
		return "进程"
	case ModePidFilter:
		return "PID"
	default:
		return ""
	}
}

func levelColor(l logentry.Level) color.Color {
	switch l {
	case logentry.LevelVerbose:
		return ui.ColorVerbose
	case logentry.LevelDebug:
		return ui.ColorDebug
	case logentry.LevelInfo:
		return ui.ColorInfo
	case logentry.LevelWarn:
		return ui.ColorWarn
	case logentry.LevelError:
		return ui.ColorError
	case logentry.LevelFatal:
		return ui.ColorFatal
	default:
		return lipgloss.Color("252")
	}
}
