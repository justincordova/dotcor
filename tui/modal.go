package tui

import (
	"fmt"
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

	// Reserve room for title + 2 blanks + hint + 2 padding + 2 border = ~7
	// rows of chrome. Subtract from terminal height; the body gets the rest.
	maxBodyLines := m.height - 8
	if maxBodyLines < 3 {
		maxBodyLines = 3
	}
	body := truncateBody(m.confirmBody, maxBodyLines)

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.confirmTitle))
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(m.confirmHint))

	maxHeight := m.height - 4
	if maxHeight < 6 {
		maxHeight = 6
	}

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 3).
		MaxWidth(max(m.width-8, 30)).
		MaxHeight(maxHeight).
		Render(b.String())

	return dialog
}

// truncateBody clips body to maxLines, appending "... N more" when content
// is dropped. Preserves trailing instructional text by prioritising the
// first maxLines-1 lines and showing the count.
func truncateBody(body string, maxLines int) string {
	lines := strings.Split(body, "\n")
	if len(lines) <= maxLines {
		return body
	}
	keep := maxLines - 1
	if keep < 1 {
		keep = 1
	}
	dropped := len(lines) - keep
	out := append([]string{}, lines[:keep]...)
	out = append(out, dimStyle.Render(fmt.Sprintf("... %d more", dropped)))
	return strings.Join(out, "\n")
}
