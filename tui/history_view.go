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
	header := renderHistoryHeader(m)
	body := renderHistoryBody(m)
	footer := renderHistoryFooter(m)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		body,
		footer,
	)
}

func renderHistoryHeader(m Model) string {
	title := accentStyle.Bold(true).Render("History")

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

func renderHistoryBody(m Model) string {
	if len(m.commits) == 0 {
		return lipgloss.NewStyle().
			Width(m.width).
			Padding(1, 2).
			Render(dimStyle.Render("No commit history found"))
	}

	var b strings.Builder
	maxHeight := m.height - 6
	if maxHeight < 5 {
		maxHeight = 5
	}

	for i, commit := range m.commits {
		if i >= maxHeight {
			break
		}

		cursor := "  "
		name := textStyle
		if i == m.selectedCommit {
			cursor = selectedStyle.Render("▶ ")
			name = selectedStyle
		}

		shortHash := commit.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		hash := highlightStyle.Render(shortHash)
		time := dimStyle.Render(formatRelativeTime(commit.Date))
		msg := name.Render(commit.Message)

		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, hash, msg, time))
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(1, 2).
		Render(b.String())
}

func renderHistoryFooter(m Model) string {
	esc := keyStyle.Render("esc")
	back := descStyle.Render("back")
	enter := keyStyle.Render("enter")
	restoreDesc := descStyle.Render("restore")
	diffKey := keyStyle.Render("D")
	diffDesc := descStyle.Render("diff")
	count := dimStyle.Render(fmt.Sprintf("(%d commits)", len(m.commits)))

	return lipgloss.NewStyle().
		Width(m.width).
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color(muted)).
		Render(
			fmt.Sprintf("%s %s    %s %s    %s %s    %s", esc, back, enter, restoreDesc, diffKey, diffDesc, count),
		)
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
