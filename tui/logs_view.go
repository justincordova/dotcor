package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func viewLogs(m Model) string {
	header := subviewHeader(m.width, "Logs", []string{"~/.dotcor/logs/dotcor.log"})
	filter := renderLogFilter(m)
	body := m.viewport.View()
	footer := subviewFooter(m.width,
		kbd("1", "debug"),
		kbd("2", "info"),
		kbd("3", "warn"),
		kbd("4", "error"),
		kbd("↑↓/jk", "scroll"), kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
		kbd("esc", "back"),
		dimStyle.Render(fmt.Sprintf("%d lines", len(m.logs))),
	)

	parts := []string{header, filter, body}
	if status := subviewStatusRow(m); status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func renderLogFilter(m Model) string {
	levels := []string{"debug", "info", "warn", "error"}
	colors := map[string]string{
		"debug": colOverlay0,
		"info":  colBlue,
		"warn":  colYellow,
		"error": colRed,
	}

	var parts []string
	for _, lv := range levels {
		label := lv
		if lv == m.logLevel {
			parts = append(parts, pill(label, colBase, colors[lv]))
		} else {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(lipgloss.Color(colors[lv])).
				Padding(0, 1).
				Render(label))
		}
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Render(dimStyle.Render("level: ") + lipgloss.JoinHorizontal(lipgloss.Center, parts...))
}
