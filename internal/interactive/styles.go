package interactive

import "github.com/charmbracelet/lipgloss"

func selected(l lipgloss.Style) lipgloss.Style {
	return l.
		Foreground(foreground_color).
		Background(selected_color).
		Bold(true)
}

var keyStyle = lipgloss.NewStyle().Foreground(color_subtle).Bold(true)
var descStyle = lipgloss.NewStyle().Foreground(color_subtle_desc)
var sepStyle = lipgloss.NewStyle().Foreground(color_subtle_separator)
