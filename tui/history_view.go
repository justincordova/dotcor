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
	base := viewHistoryBase(m)
	if m.confirmOpen {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			confirmModal(m),
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color(colBase)),
		)
	}
	return base
}

func viewHistoryBase(m Model) string {
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
		if m.confirmOpen {
			switch {
			case key.Matches(msg, m.keys.Enter):
				ref := m.confirmRestoreRef
				m.clearConfirm()
				return m, m.restoreFromCommit(ref)
			default:
				m.clearConfirm()
				return m, nil
			}
		}
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
				c := m.commits[m.selectedCommit]
				m.confirmOpen = true
				m.confirmAction = "restore"
				m.confirmRestoreRef = c.Hash
				filePath := historyFilePath(m)
				shortHash := shortRef(c.Hash)
				m.confirmTitle = fmt.Sprintf("Restore %s?", filePath)
				m.confirmBody = fmt.Sprintf("from %s · %s\n\nThis replaces the current version.", shortHash, formatRelativeTime(c.Date))
				m.confirmHint = "enter confirm · esc cancel"
				m.confirmDanger = true
			}

		case msg.String() == "D":
			if m.selectedCommit < len(m.commits) {
				// Switch to DiffView before the diff content arrives so the
				// user sees the transition immediately. The diffMsg handler
				// populates the viewport without changing views.
				m.activeView = DiffView
				return m, m.diffFromCommit(m.commits[m.selectedCommit].Hash)
			}
		}
	}

	return m, nil
}

func shortRef(ref string) string {
	if len(ref) > 7 {
		return ref[:7]
	}
	return ref
}

func historyFilePath(m Model) string {
	if m.selectedPkg < len(m.packages) {
		pkg := m.packages[m.selectedPkg]
		if m.expanded[m.selectedPkg] && m.selectedFile < len(pkg.Files) {
			return pkg.Files[m.selectedFile].RelPath
		}
		return pkg.Name
	}
	return ""
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
		} else {
			filePath = pkg.Name
		}
	}

	return func() tea.Msg {
		if err := git.RestoreFile(repoDir, filePath, ref); err != nil {
			return errMsg{err: err}
		}
		return statusMsg(fmt.Sprintf("Restored from %s", shortRef(ref)))
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
			return diffMsg{content: fmt.Sprintf("No diff for %s vs %s", filePath, shortRef(ref))}
		}
		return diffMsg{content: content}
	}
}
