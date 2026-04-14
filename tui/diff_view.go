package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/git"
)

type diffMsg struct {
	content string
	err     error
}

func viewDiff(m Model) string {
	header := renderDiffHeader(m)
	body := m.viewport.View()
	footer := renderDiffFooter(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func renderDiffHeader(m Model) string {
	title := accentStyle.Bold(true).Render("Diff")

	var subtitle string
	if m.selectedPkg < len(m.packages) && m.selectedFile < len(m.packages[m.selectedPkg].Files) {
		f := m.packages[m.selectedPkg].Files[m.selectedFile]
		subtitle = dimStyle.Render(f.RelPath)
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(muted)).
		Padding(0, 1).
		Render(
			lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", subtitle),
		)
}

func renderDiffFooter(m Model) string {
	esc := keyStyle.Render("esc")
	back := descStyle.Render("back")
	commit := keyStyle.Render("c")
	commitDesc := descStyle.Render("commit")

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(muted)).
		Render(
			fmt.Sprintf("%s %s    %s %s", esc, back, commit, commitDesc),
		)
}

func (m Model) updateDiff(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case diffMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.viewport.SetContent(colorizeDiff(msg.content))
		m.viewport.GotoTop()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.activeView = DashboardView
			m.err = nil
			return m, nil

		case msg.String() == "c":
			return m, m.commitDiff()
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func getDiff(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.selectedPkg >= len(m.packages) {
			return diffMsg{err: fmt.Errorf("no package selected")}
		}

		pkg := m.packages[m.selectedPkg]

		if !m.expanded[m.selectedPkg] || m.selectedFile >= len(pkg.Files) {
			content, err := git.GetDiff(m.repoDir)
			if err != nil {
				return diffMsg{err: err}
			}
			if content == "" {
				return diffMsg{content: "No changes detected"}
			}
			return diffMsg{content: content}
		}

		f := pkg.Files[m.selectedFile]
		content, err := git.GetFileDiff(m.repoDir, f.RelPath)
		if err != nil {
			return diffMsg{err: err}
		}
		if content == "" {
			return diffMsg{content: fmt.Sprintf("No changes for %s", f.RelPath)}
		}
		return diffMsg{content: content}
	}
}

func (m Model) commitDiff() tea.Cmd {
	repoDir := m.repoDir
	logger := m.cfg.Logger
	return func() tea.Msg {
		if err := git.AutoCommit(repoDir, "Changes from diff view", logger); err != nil {
			return errMsg{err: err}
		}
		return statusMsg("Changes committed")
	}
}

func colorizeDiff(content string) string {
	if content == "" {
		return dimStyle.Render("No diff available")
	}

	var b strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if len(line) == 0 {
			b.WriteString("\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			b.WriteString(highlightStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(successStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString(errorStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString(highlightStyle.Render(line))
			b.WriteString("\n")
		default:
			b.WriteString(dimStyle.Render(line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 02, 2006")
	}
}
