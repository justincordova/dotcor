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
	crumbs := []string{"dotfiles"}
	if m.selectedPkg < len(m.packages) {
		p := m.packages[m.selectedPkg]
		crumbs = append(crumbs, p.Name)
		if m.expanded[m.selectedPkg] && m.selectedFile < len(p.Files) {
			crumbs = append(crumbs, p.Files[m.selectedFile].RelPath)
		}
	}

	header := subviewHeader(m.width, "Diff", crumbs)
	body := m.viewport.View()
	footer := subviewFooter(m.width,
		kbd("c", "commit"),
		kbd("↑↓/jk", "scroll"), kbd("pgup/pgdn", "page"), kbd("g/G", "top/bot"),
		kbd("esc", "back"),
	)

	parts := []string{header, body}
	if status := diffStatusRow(m); status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, footer)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func diffStatusRow(m Model) string {
	return subviewStatusRow(m)
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

// getDiff resolves the diff target on the Update goroutine and hands the
// command only immutable values.
//
// The closure used to capture the whole Model and read m.expanded inside the
// goroutine. m.expanded is a map, so it is shared rather than copied with the
// Model, and Update writes it on every enter keypress. A map read racing a
// map write is an unrecoverable Go runtime fatal error — no recover catches
// it — and `git diff` on a large repo holds the window open for seconds:
// press D, esc back to the dashboard, then enter, and the process dies.
func getDiff(m Model) tea.Cmd {
	repoDir := m.repoDir
	hasPackage := m.selectedPkg >= 0 && m.selectedPkg < len(m.packages)
	filePath := selectedFileRelPath(m)

	return func() tea.Msg {
		if !hasPackage {
			return diffMsg{err: fmt.Errorf("no package selected")}
		}

		if filePath == "" {
			content, err := git.GetDiff(repoDir)
			if err != nil {
				// Fresh repo with no commits → `git diff HEAD` errors with
				// "fatal: bad revision 'HEAD'". Surface a friendly hint
				// rather than the raw git output.
				if strings.Contains(err.Error(), "bad revision 'HEAD'") ||
					strings.Contains(err.Error(), "unknown revision") {
					return diffMsg{content: "No commits yet — press s in the package view to stow, or save a file and sync."}
				}
				return diffMsg{err: err}
			}
			if content == "" {
				return diffMsg{content: "No changes detected"}
			}
			return diffMsg{content: content}
		}

		content, err := git.GetFileDiff(repoDir, filePath)
		if err != nil {
			return diffMsg{err: err}
		}
		if content == "" {
			return diffMsg{content: fmt.Sprintf("No changes for %s", filePath)}
		}
		return diffMsg{content: content}
	}
}

// selectedFileRelPath returns the RelPath of the file under the cursor, or ""
// when the cursor is on a package row rather than a file.
//
// Must be called on the Update goroutine: it reads m.expanded, which is
// shared mutable state.
func selectedFileRelPath(m Model) string {
	if m.selectedPkg < 0 || m.selectedPkg >= len(m.packages) {
		return ""
	}
	pkg := m.packages[m.selectedPkg]
	if m.expanded[m.selectedPkg] && m.selectedFile >= 0 && m.selectedFile < len(pkg.Files) {
		return pkg.Files[m.selectedFile].RelPath
	}
	return ""
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

	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colGreen))
	delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colRed))
	hunkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colMauve)).Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colLavender)).Bold(true)

	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		switch {
		case len(line) == 0:
			b.WriteString("\n")
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			b.WriteString(metaStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString(hunkStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString(addStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString(delStyle.Render(line))
			b.WriteString("\n")
		case strings.HasPrefix(line, "diff "):
			b.WriteString(metaStyle.Render(line))
			b.WriteString("\n")
		default:
			b.WriteString(textStyle.Render(line))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		// Clock skew between systems (e.g. a commit pulled from another
		// machine with a faster clock) can produce a negative duration;
		// treat as "just now" rather than rendering "-3m ago".
		return "just now"
	}
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
