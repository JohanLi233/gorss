package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	alert     = lipgloss.AdaptiveColor{Light: "#FD4659", Dark: "#FF5A6E"}

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(highlight).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(subtle).
			Padding(0, 1)

	feedListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2).
			Width(30)

	selectedFeedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(special)

	normalFeedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#DDDDDD"})

	articleListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2)

	selectedArticleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(special)

	normalArticleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#DDDDDD"})

	articleViewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2)

	articleTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	articleMetaStyle = lipgloss.NewStyle().
			Foreground(subtle).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(subtle)

	// 配置界面样式
	configViewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(1, 2)

	configTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	configContentStyle = lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1)

	selectedConfigItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(special)

	normalConfigItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#DDDDDD"})

	selectedConfigFormStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(special)

	normalConfigFormStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#DDDDDD"})

	configHelpStyle = lipgloss.NewStyle().
			Foreground(subtle)

	configMessageStyle = lipgloss.NewStyle().
			Foreground(alert)
)
