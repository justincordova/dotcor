package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func viewHelp(m Model) string {
	categories := []struct {
		title    string
		bindings []string
	}{
		{
			title: "Navigation",
			bindings: []string{
				formatBinding("↑/k", "move up"),
				formatBinding("↓/j", "move down"),
				formatBinding("enter", "expand / confirm"),
				formatBinding("/", "search packages"),
				formatBinding("esc", "back / cancel"),
			},
		},
		{
			title: "Packages",
			bindings: []string{
				formatBinding("s", "stow package"),
				formatBinding("u", "unstow package"),
				formatBinding("A", "stow all"),
				formatBinding("a", "add file (browser)"),
				formatBinding("o", "adopt foreign symlinks"),
				formatBinding("r", "remove (context-aware)"),
			},
		},
		{
			title: "Git",
			bindings: []string{
				formatBinding("i", "init git"),
				formatBinding("S", "sync (commit + push)"),
				formatBinding("p", "push to remote"),
				formatBinding("P", "pull from remote"),
				formatBinding("D", "view diff"),
				formatBinding("H", "view history"),
			},
		},
		{
			title: "System",
			bindings: []string{
				formatBinding("L", "view logs"),
				formatBinding(",", "settings"),
				formatBinding("?", "toggle help"),
				formatBinding("q", "quit"),
			},
		},
	}

	var sections []string
	maxWidth := 0

	for _, cat := range categories {
		var lines []string
		lines = append(lines, accentStyle.Render(cat.title))
		lines = append(lines, subtleStyle.Render(strings.Repeat("─", 28)))
		lines = append(lines, cat.bindings...)
		section := strings.Join(lines, "\n")
		sections = append(sections, section)
		if lipgloss.Width(section) > maxWidth {
			maxWidth = lipgloss.Width(section)
		}
	}

	// Arrange into 2 columns.
	var rows []string
	for i := 0; i < len(sections); i += 2 {
		if i+1 < len(sections) {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Width(maxWidth+4).Render(sections[i]),
				sections[i+1],
			))
		} else {
			rows = append(rows, sections[i])
		}
	}
	content := strings.Join(rows, "\n\n")

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colMauve)).
		Bold(true).
		Render("◆ DotCor — Keybindings")

	footer := dimStyle.Render("press ? or esc to close")

	body := lipgloss.JoinVertical(lipgloss.Left, title, "", content, "", footer)

	maxH := m.height - 4
	if maxH < 8 {
		maxH = 8
	}
	maxW := m.width - 8
	if maxW < 30 {
		maxW = 30
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colMauve)).
		Padding(1, 3).
		MaxWidth(maxW).
		MaxHeight(maxH).
		Render(body)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog,
		lipgloss.WithWhitespaceChars(" "))
}

func formatBinding(k, desc string) string {
	return fmt.Sprintf("  %s  %s", keyStyle.Render(padRight(k, 7)), descStyle.Render(desc))
}
