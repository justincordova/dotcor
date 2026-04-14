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
				formatBinding("enter", "expand/collapse package"),
				formatBinding("tab", "switch panel"),
				formatBinding("esc", "back"),
			},
		},
		{
			title: "Actions",
			bindings: []string{
				formatBinding("s", "stow package"),
				formatBinding("u", "unstow package"),
				formatBinding("a", "add file"),
				formatBinding("d", "remove file"),
			},
		},
		{
			title: "Git",
			bindings: []string{
				formatBinding("S", "sync (commit + push)"),
				formatBinding("p", "push to remote"),
				formatBinding("P", "pull from remote"),
				formatBinding("D", "view diff"),
				formatBinding("H", "view history"),
				formatBinding("r", "restore file"),
			},
		},
		{
			title: "Other",
			bindings: []string{
				formatBinding("/", "search packages"),
				formatBinding("L", "view logs"),
				formatBinding("?", "toggle help"),
				formatBinding("q", "quit"),
			},
		},
	}

	var sections []string
	maxWidth := 0

	for _, cat := range categories {
		var lines []string
		lines = append(lines, accentStyle.Bold(true).Render(cat.title))
		lines = append(lines, strings.Repeat("─", 30))
		lines = append(lines, cat.bindings...)
		section := strings.Join(lines, "\n")
		sections = append(sections, section)
		if lipgloss.Width(section) > maxWidth {
			maxWidth = lipgloss.Width(section)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	content += fmt.Sprintf("\n\n%s", dimStyle.Render("Press ? or esc to close"))

	dialogWidth := maxWidth + 4
	dialogHeight := len(strings.Split(content, "\n")) + 2

	dialog := lipgloss.NewStyle().
		Width(dialogWidth).
		Height(dialogHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(iris)).
		Padding(1, 2).
		Render(content)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(
			lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, dialog),
		)
}

func formatBinding(key, desc string) string {
	return fmt.Sprintf("  %s  %s", keyStyle.Render(key), descStyle.Render(desc))
}
