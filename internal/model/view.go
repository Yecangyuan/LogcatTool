package model

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/simley/logcatool/internal/logentry"
	"github.com/simley/logcatool/internal/ui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
	sections = append(sections, m.viewport.View())
	sections = append(sections, m.renderStatusBar())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.inputMode == ModeDevicePicker {
		content = m.overlayDevicePicker(content)
	}

	v.SetContent(content)
	if m.inputMode >= ModeSearch && m.inputMode <= ModePidFilter {
		c := m.filterInput.Cursor()
		if c != nil {
			c.Y += 1 // offset for title bar
			v.Cursor = c
		}
	}
	return v
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

	var status string
	if m.paused {
		status = ui.PausedStyle.Render(" ⏸ 暂停 ")
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

	// Level toggles
	for i, level := range logentry.FilterableLevels {
		num := fmt.Sprintf("%d", i+1)
		char := level.Char()
		if m.filter.IsLevelEnabled(level) {
			style := ui.LevelBtnActiveStyle.Foreground(levelColor(level))
			parts = append(parts, style.Render(fmt.Sprintf("[%s:%s]", num, char)))
		} else {
			parts = append(parts, ui.LevelBtnInactiveStyle.Render(fmt.Sprintf("[%s:%s]", num, char)))
		}
	}

	// Active filter input or display
	if m.inputMode >= ModeSearch && m.inputMode <= ModePidFilter {
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
		if m.filter.PID > 0 {
			parts = append(parts, ui.FilterActiveStyle.Render(
				fmt.Sprintf(" PID:%d", m.filter.PID)))
		}
	}

	content := strings.Join(parts, " ")
	return ui.FilterBarStyle.Width(m.width).Render(content)
}

func (m AppModel) renderStatusBar() string {
	left := fmt.Sprintf(" 总计:%d  显示:%d", m.totalCount, m.filteredCount)

	if m.autoScroll {
		left += "  ▼自动滚动"
	}
	if m.paused {
		left += "  ⏸暂停中"
	}
	if len(m.bookmarks) > 0 {
		left += fmt.Sprintf("  🔖%d", len(m.bookmarks))
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
	sb.WriteString("    p           包名过滤\n")
	sb.WriteString("    i           PID 过滤\n")
	sb.WriteString("    1-6         切换日志级别 V/D/I/W/E/F\n")
	sb.WriteString("\n  操作:\n")
	sb.WriteString("    Space       暂停/恢复日志流\n")
	sb.WriteString("    c           清除日志\n")
	sb.WriteString("    d           选择设备\n")
	sb.WriteString("    e           导出日志\n")
	sb.WriteString("    b           添加/移除书签\n")
	sb.WriteString("    n/N         下一个/上一个书签\n")
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

	// Center the picker overlay
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
			// Overwrite: take left part + overlay + right part
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

func filterModeLabel(mode InputMode) string {
	switch mode {
	case ModeSearch:
		return "搜索"
	case ModeTagFilter:
		return "Tag"
	case ModePkgFilter:
		return "包名"
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
