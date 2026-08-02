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
		kbd("↑↓", "nav"), kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
		kbd("esc", "back"),
	)

	parts := []string{header, body}
	if status := subviewStatusRow(m); status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
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
		when := dimStyle.Render(padRight(formatRelativeTime(c.Date), 12))
		subject := textStyle.Render(truncate(c.Message, m.width-30))

		content := fmt.Sprintf("%s  %s  %s", hash, when, subject)
		b.WriteString(selectableRow(content, selected, m.width-4))
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
				// Dispatch on confirmAction. The modal shown over History is
				// not always History's own restore prompt — the top-level
				// Update opens a conflict-resolution prompt from any view.
				return m.confirmAccept()
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

		case msg.String() == "pgup" || msg.String() == "ctrl+b":
			m.selectedCommit -= 5
			if m.selectedCommit < 0 {
				m.selectedCommit = 0
			}

		case msg.String() == "pgdown" || msg.String() == "ctrl+f":
			// Clamping to len-1 on an empty list yields -1, which then
			// passes the `< len(m.commits)` guards below and panics on
			// index. Only move the cursor when there is something to move to.
			if len(m.commits) > 0 {
				m.selectedCommit += 5
				if m.selectedCommit > len(m.commits)-1 {
					m.selectedCommit = len(m.commits) - 1
				}
			}

		case msg.String() == "g" || msg.String() == "home":
			m.selectedCommit = 0

		case msg.String() == "G" || msg.String() == "end":
			if len(m.commits) > 0 {
				m.selectedCommit = len(m.commits) - 1
			}

		case key.Matches(msg, m.keys.Enter):
			if m.selectedCommit >= 0 && m.selectedCommit < len(m.commits) {
				c := m.commits[m.selectedCommit]
				m.confirmOpen = true
				m.confirmAction = "restore"
				m.confirmRestoreRef = c.Hash
				// Capture the file path NOW so a packagesMsg arriving
				// before Enter can't shift indices and target the wrong file.
				m.confirmFilePath = historyFilePath(m)
				shortHash := shortRef(c.Hash)
				m.confirmTitle = fmt.Sprintf("Restore %s?", m.confirmFilePath)
				m.confirmBody = fmt.Sprintf("from %s · %s\n\nThis replaces the current version.", shortHash, formatRelativeTime(c.Date))
				m.confirmHint = "enter confirm · esc cancel"
				m.confirmDanger = true
			}

		case msg.String() == "D":
			if m.selectedCommit >= 0 && m.selectedCommit < len(m.commits) {
				// Switch to DiffView before the diff content arrives so the
				// user sees the transition immediately. The diffMsg handler
				// populates the viewport without changing views.
				m.activeView = DiffView
				// Capture file path at key-press to avoid the same TOCTOU
				// shape as the restore Enter handler above.
				return m, m.diffFromCommit(m.commits[m.selectedCommit].Hash, historyFilePath(m))
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

// historyFilePath returns the path history should be shown for: the selected
// file when a package is expanded, otherwise the package itself.
//
// Must be called on the Update goroutine — it reads m.expanded, which is
// shared mutable state.
func historyFilePath(m Model) string {
	if m.selectedPkg < 0 || m.selectedPkg >= len(m.packages) {
		return ""
	}
	pkg := m.packages[m.selectedPkg]
	if rel := selectedFileRelPath(m); rel != "" {
		return rel
	}
	return pkg.Name
}

// getFileHistory resolves the target on the Update goroutine and hands the
// command only immutable values. See getDiff for why capturing the Model and
// reading m.expanded inside the goroutine is a fatal data race.
func getFileHistory(m Model) tea.Cmd {
	repoDir := m.repoDir
	hasPackage := m.selectedPkg >= 0 && m.selectedPkg < len(m.packages)
	filePath := historyFilePath(m)

	return func() tea.Msg {
		if !hasPackage {
			return historyMsg{err: fmt.Errorf("no package selected")}
		}

		commits, err := git.GetFileHistory(repoDir, filePath, 50)
		if err != nil {
			return historyMsg{err: err}
		}
		return historyMsg{commits: commits}
	}
}

// restoreFromCommit takes filePath as a parameter (captured at dialog-open
// time) so that a background packagesMsg arriving between dialog-open and
// Enter can't shift indices and cause RestoreFile to operate on the wrong
// file. See ISSUES.md #7.
func (m Model) restoreFromCommit(ref, filePath string) tea.Cmd {
	repoDir := m.repoDir
	return func() tea.Msg {
		if err := git.RestoreFile(repoDir, filePath, ref); err != nil {
			return errMsg{err: err}
		}
		return statusMsg(fmt.Sprintf("Restored from %s", shortRef(ref)))
	}
}

// diffFromCommit takes filePath as a parameter for the same TOCTOU reason
// as restoreFromCommit.
func (m Model) diffFromCommit(ref, filePath string) tea.Cmd {
	repoDir := m.repoDir
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
