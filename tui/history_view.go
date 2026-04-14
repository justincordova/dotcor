package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/git"
)

type historyMsg struct {
	commits []git.CommitInfo
	err     error
}

func viewHistory(m Model) string {
	crumbs := []string{}
	if m.selectedPkg < len(m.packages) {
		p := m.packages[m.selectedPkg]
		crumbs = append(crumbs, p.Name)
		if m.expanded[m.selectedPkg] && m.selectedFile < len(p.Files) {
			crumbs = append(crumbs, p.Files[m.selectedFile].RelPath)
		}
	}

	header := subviewHeader(m.width, "History", crumbs)
	body := renderHistoryBody(m)
	footer := subviewFooter(m.width,
		kbd("enter", "restore"),
		kbd("D", "diff"),
		kbd("↑↓", "nav"),
		kbd("esc", "back"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func renderHistoryBody(m Model) string {
	if len(m.commits) == 0 {
		return subviewContent(m.width, m.height-2, dimStyle.Render("No commit history found."))
	}

	var b strings.Builder
	maxHeight := m.height - 4
	if maxHeight < 5 {
		maxHeight = 5
	}

	start, end := visibleRange(m.selectedCommit, len(m.commits), maxHeight)

	for i := start; i < end; i++ {
		c := m.commits[i]
		selected := i == m.selectedCommit

		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		hash := lipgloss.NewStyle().Foreground(lipgloss.Color(colPeach)).Render(shortHash)
		when := dimStyle.Render(formatRelativeTime(c.Date))
		subject := textStyle.Render(truncate(c.Message, m.width-30))

		row := fmt.Sprintf("  %s  %s  %s", hash, subject, when)
		if selected {
			row = selectedRowStyle.Width(m.width - 4).Render(accentStyle.Render("▸ ") + hash + "  " + subject + "  " + when)
		}
		b.WriteString(row)
		b.WriteString("\n")
	}

	return subviewContent(m.width, m.height-2, b.String())
}

func (m Model) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case historyMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.commits = msg.commits
		m.selectedCommit = 0
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.activeView = DashboardView
			m.commits = nil
			m.err = nil
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.selectedCommit > 0 {
				m.selectedCommit--
			}

		case key.Matches(msg, m.keys.Down):
			if m.selectedCommit < len(m.commits)-1 {
				m.selectedCommit++
			}

		case key.Matches(msg, m.keys.Enter):
			if m.selectedCommit < len(m.commits) {
				return m, m.restoreFromCommit(m.commits[m.selectedCommit].Hash)
			}

		case msg.String() == "D":
			if m.selectedCommit < len(m.commits) {
				return m, m.diffFromCommit(m.commits[m.selectedCommit].Hash)
			}
		}
	}

	return m, nil
}

func getFileHistory(m Model) tea.Cmd {
	return func() tea.Msg {
		if m.selectedPkg >= len(m.packages) {
			return historyMsg{err: fmt.Errorf("no package selected")}
		}

		pkg := m.packages[m.selectedPkg]

		var filePath string
		if m.expanded[m.selectedPkg] && m.selectedFile < len(pkg.Files) {
			filePath = pkg.Files[m.selectedFile].RelPath
		} else {
			filePath = pkg.Name
		}

		commits, err := git.GetFileHistory(m.repoDir, filePath, 50)
		if err != nil {
			return historyMsg{err: err}
		}
		return historyMsg{commits: commits}
	}
}

func (m Model) restoreFromCommit(ref string) tea.Cmd {
	repoDir := m.repoDir

	var filePath string
	if m.selectedPkg < len(m.packages) {
		pkg := m.packages[m.selectedPkg]
		if m.expanded[m.selectedPkg] && m.selectedFile < len(pkg.Files) {
			filePath = pkg.Files[m.selectedFile].RelPath
		}
	}

	return func() tea.Msg {
		if err := git.RestoreFile(repoDir, filePath, ref); err != nil {
			return errMsg{err: err}
		}
		return statusMsg(fmt.Sprintf("Restored from %s", ref[:7]))
	}
}

func (m Model) diffFromCommit(ref string) tea.Cmd {
	repoDir := m.repoDir

	var filePath string
	if m.selectedPkg < len(m.packages) {
		pkg := m.packages[m.selectedPkg]
		if m.expanded[m.selectedPkg] && m.selectedFile < len(pkg.Files) {
			filePath = pkg.Files[m.selectedFile].RelPath
		}
	}

	return func() tea.Msg {
		content, err := git.GetFileDiffFromRef(repoDir, filePath, ref)
		if err != nil {
			return diffMsg{err: err}
		}
		if content == "" {
			return diffMsg{content: fmt.Sprintf("No diff for %s vs %s", filePath, ref[:7])}
		}
		return diffMsg{content: content}
	}
}
