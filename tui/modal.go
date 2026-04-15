package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func confirmModal(m Model) string {
	if !m.confirmOpen {
		return ""
	}

	borderColor := colMauve
	titleStyle := accentStyle
	if m.confirmDanger {
		borderColor = colRed
		titleStyle = errorStyle.Bold(true)
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.confirmTitle))
	b.WriteString("\n\n")
	b.WriteString(m.confirmBody)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(m.confirmHint))

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 3).
		MaxWidth(max(m.width-8, 30)).
		Render(b.String())

	return dialog
}
