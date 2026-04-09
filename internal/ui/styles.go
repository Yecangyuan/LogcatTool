package ui

import "charm.land/lipgloss/v2"

var (
	ColorVerbose = lipgloss.Color("247") // gray
	ColorDebug   = lipgloss.Color("39")  // blue
	ColorInfo    = lipgloss.Color("42")  // green
	ColorWarn    = lipgloss.Color("214") // yellow
	ColorError   = lipgloss.Color("196") // red
	ColorFatal   = lipgloss.Color("196") // red bold

	LevelStyleV = lipgloss.NewStyle().Foreground(ColorVerbose)
	LevelStyleD = lipgloss.NewStyle().Foreground(ColorDebug)
	LevelStyleI = lipgloss.NewStyle().Foreground(ColorInfo)
	LevelStyleW = lipgloss.NewStyle().Foreground(ColorWarn)
	LevelStyleE = lipgloss.NewStyle().Foreground(ColorError)
	LevelStyleF = lipgloss.NewStyle().Foreground(ColorFatal).Bold(true)

	TitleStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true).
			Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	StatusKeyStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("42")).
			Bold(true)

	FilterBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	FilterLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("42")).
				Bold(true)

	FilterActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true)

	LevelBtnActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1)

	LevelBtnInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Padding(0, 1)

	SearchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("214")).
				Foreground(lipgloss.Color("0")).
				Bold(true)

	PausedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	BookmarkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	DevicePickerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2)

	DeviceSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230")).
				Bold(true)

	DeviceNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	HelpTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	TimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	PidStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	TagStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
)
