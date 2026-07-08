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

// joinSubview stacks sub-view sections top to bottom, dropping any empty
// ones (e.g. an absent status row) so they don't introduce blank lines.
func joinSubview(sections ...string) string {
	kept := make([]string, 0, len(sections))
	for _, s := range sections {
		if s != "" {
			kept = append(kept, s)
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, kept...)
}

// subviewStatusRow renders a one-line status/error banner shared by every
// sub-view. Without it, errors set on the model (e.g. a failed restore or a
// SaveConfig failure) are stored but never shown, leaving the user with no
// feedback. Returns "" when there is nothing to report.
func subviewStatusRow(m Model) string {
	switch {
	case m.err != nil:
		return errorStyle.Render(" ✗ " + m.err.Error())
	case m.statusMsg != "":
		return successStyle.Render(" ✓ " + m.statusMsg)
	default:
		return ""
	}
}

func contentWidth(termWidth int) int {
	maxW := 90
	if termWidth < maxW {
		return termWidth
	}
	return maxW
}

func bodyWidth(termWidth int) int {
	return contentWidth(termWidth) - 4
}

func subviewContent(width, height int, body string) string {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2).
		Render(body)
}

func plainFooter(width int, hints ...string) string {
	text := joinHints(hints...)
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Align(lipgloss.Center).
		Render(text)
}

func mapStyle(in []string, style lipgloss.Style) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = style.Render(s)
	}
	return out
}
