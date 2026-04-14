package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func viewLogs(m Model) string {
	header := renderLogsHeader(m)
	body := m.viewport.View()
	footer := renderLogsFooter(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func renderLogsHeader(m Model) string {
	title := accentStyle.Bold(true).Render("Logs")

	levelLabel := fmt.Sprintf("level: %s", m.logLevel)
	levelStyle := dimStyle
	switch m.logLevel {
	case "debug":
		levelStyle = dimStyle
	case "info":
		levelStyle = textStyle
	case "warn":
		levelStyle = warningStyle
	case "error":
		levelStyle = errorStyle
	}

	filter := levelStyle.Render(levelLabel)
	filters := dimStyle.Render("[1]debug [2]info [3]warn [4]error")

	return lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(muted)).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(
				lipgloss.Top,
				title,
				strings.Repeat(" ", 2),
				filter,
				strings.Repeat(" ", 4),
				filters,
			),
		)
}

func renderLogsFooter(m Model) string {
	esc := keyStyle.Render("esc")
	back := descStyle.Render("back to dashboard")
	count := dimStyle.Render(fmt.Sprintf("(%d lines)", len(m.logs)))

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(muted)).
		Render(
			fmt.Sprintf("%s %s    %s", esc, back, count),
		)
}
