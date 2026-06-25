package ui

import "charm.land/lipgloss/v2"

var (
	AnomalyPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("214")).
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Padding(1, 2)

	AnomalySelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Bold(true)

	AnomalySpikeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	AnomalyDropStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39"))
)
