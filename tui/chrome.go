package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// subviewHeader renders a consistent top bar for sub-views:
//
//	[◆ title]  crumb1 › crumb2                          [ esc back ]
func subviewHeader(width int, title string, crumbs []string) string {
	left := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colMauve)).
		Bold(true).
		Render("◆ " + title)

	if len(crumbs) > 0 {
		sep := dimStyle.Render(" › ")
		crumbText := strings.Join(mapStyle(crumbs, subtitleStyle), sep)
		left = left + "  " + crumbText
	}

	right := dimStyle.Render("esc ") + keyStyle.Render("back")

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	row := left + strings.Repeat(" ", gap) + right
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(lipgloss.Color(colMantle)).
		Render(row)
}

// subviewFooter renders a bottom keybind bar.
func subviewFooter(width int, hints ...string) string {
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 1).
		Background(lipgloss.Color(colMantle)).
		Render(joinHints(hints...))
}

// subviewContent frames the body between header and footer with a fixed height.
func subviewContent(width, height int, body string) string {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2).
		Render(body)
}

func mapStyle(in []string, style lipgloss.Style) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = style.Render(s)
	}
	return out
}
