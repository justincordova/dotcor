package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
)

// Settings pane steps. viewSettings and the update handlers dispatch on
// these, and app.go needs the backups value to know when to reload the list.
const (
	settingsStepMain       = 0
	settingsStepEditRemote = 1
	settingsStepBackups    = 3
	settingsStepAddPattern = 4
)

type settingsMsg struct {
	msg string
	err error
	// gitRemote carries the value that was actually persisted, so the model
	// is only updated once the write has succeeded. Nil means "unchanged".
	gitRemote *string
}

type backupsMsg struct {
	backups []core.BackupInfo
	err     error
}

func viewSettings(m Model) string {
	switch m.settingsStep {
	case settingsStepEditRemote:
		return viewSettingsEditRemote(m)
	case settingsStepBackups:
		return viewSettingsBackups(m)
	case settingsStepAddPattern:
		return viewSettingsAddPattern(m)
	default:
		return viewSettingsMain(m)
	}
}

func viewSettingsMain(m Model) string {
	header := subviewHeader(m.width, "Settings", nil)

	var b strings.Builder

	b.WriteString(accentStyle.Render("Git"))
	b.WriteString("\n")
	b.WriteString(hRule(bodyWidth(m.width)))
	b.WriteString("\n\n")

	remote := m.cfg.GitRemote
	if remote == "" {
		remote = dimStyle.Render("(not configured)")
	} else {
		remote = textStyle.Render(remote)
	}
	b.WriteString("  " + padRight(textStyle.Render("Remote"), 12) + "  " + remote + "\n")

	b.WriteString("\n")
	b.WriteString(accentStyle.Render("Ignore Patterns"))
	b.WriteString("\n")
	b.WriteString(hRule(bodyWidth(m.width)))
	b.WriteString("\n")
	if len(m.cfg.IgnorePatterns) == 0 {
		b.WriteString(dimStyle.Render("  (none)\n"))
	} else {
		for i, p := range m.cfg.IgnorePatterns {
			selected := m.settingsEditingIgnore && i == m.settingsIgnoreIdx
			b.WriteString(selectableRow(textStyle.Render(p), selected, bodyWidth(m.width)))
			b.WriteString("\n")
		}
	}

	body := subviewContent(m.width, m.height-3, b.String())

	var footerKeys []string
	footerKeys = append(footerKeys, kbd("e", "edit remote"), kbd("b", "backups"))
	if m.settingsEditingIgnore {
		footerKeys = append(footerKeys, kbd("a", "add"), kbd("d", "delete"), kbd("i", "done"), kbd("esc", "back"))
	} else {
		footerKeys = append(footerKeys, kbd("i", "edit patterns"), kbd("esc", "back"))
	}
	footer := subviewFooter(m.width, footerKeys...)

	return joinSubview(header, body, subviewStatusRow(m), footer)
}

func viewSettingsEditRemote(m Model) string {
	header := subviewHeader(m.width, "Settings", []string{"edit remote"})

	body := strings.Join([]string{
		textStyle.Render("Enter git remote URL:"),
		"",
		m.settingsInput.View(),
		"",
		dimStyle.Render("e.g. git@github.com:user/dotfiles.git"),
	}, "\n")

	footer := subviewFooter(m.width, kbd("enter", "save"), kbd("esc", "cancel"))
	return joinSubview(header, subviewContent(m.width, m.height-3, body), subviewStatusRow(m), footer)
}

func viewSettingsBackups(m Model) string {
	header := subviewHeader(m.width, "Settings", []string{"backups"})

	var b strings.Builder
	if len(m.backups) == 0 {
		b.WriteString(dimStyle.Render("No backups found."))
	} else {
		fmt.Fprintf(&b, "%s %s\n\n",
			dimStyle.Render("Total:"),
			accentStyle.Render(fmt.Sprintf("%d backups", len(m.backups))),
		)
		maxDisplay := 20
		for i, backup := range m.backups {
			if i >= maxDisplay {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  … and %d more\n", len(m.backups)-maxDisplay)))
				break
			}
			srcW := m.width - 40
			if srcW < 10 {
				srcW = 10
			}
			ts := dimStyle.Render(backup.Timestamp.Format("2006-01-02 15:04"))
			// Pad the source path to a fixed column so the size lines up in
			// a straight right-hand column instead of jittering per row.
			src := textStyle.Render(padRight(truncate(backup.SourcePath, srcW), srcW))
			size := dimStyle.Render(humanSize(backup.Size))
			fmt.Fprintf(&b, "  %s  %s  %s\n", ts, src, size)
		}
	}

	body := subviewContent(m.width, m.height-3, b.String())
	footer := subviewFooter(m.width,
		kbd("c", "clean >30d"),
		kbd("esc", "back"),
	)
	return joinSubview(header, body, subviewStatusRow(m), footer)
}

func viewSettingsAddPattern(m Model) string {
	header := subviewHeader(m.width, "Settings", []string{"add pattern"})

	body := strings.Join([]string{
		textStyle.Render("Enter ignore pattern:"),
		"",
		m.settingsInput.View(),
		"",
		dimStyle.Render("e.g. *.log, .env, secret/"),
	}, "\n")

	footer := subviewFooter(m.width, kbd("enter", "save"), kbd("esc", "cancel"))
	return joinSubview(header, subviewContent(m.width, m.height-3, body), subviewStatusRow(m), footer)
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.settingsStep {
	case 1:
		return updateSettingsEditRemote(m, msg)
	case 3:
		return updateSettingsBackups(m, msg)
	case 4:
		return updateSettingsAddPattern(m, msg)
	default:
		return updateSettingsMain(m, msg)
	}
}

func updateSettingsMain(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
			if m.settingsEditingIgnore {
				m.settingsEditingIgnore = false
				return m, nil
			}
			m.activeView = DashboardView
			m.settingsStep = settingsStepMain
			return m, nil
		}

		switch keyMsg.String() {
		case "e":
			m.settingsStep = settingsStepEditRemote
			m.settingsInput.SetValue(m.cfg.GitRemote)
			m.settingsInput.Focus()
			return m, textinput.Blink

		case "b":
			m.settingsStep = settingsStepBackups
			return m, m.loadBackups()

		case "i":
			if m.settingsEditingIgnore {
				m.settingsEditingIgnore = false
				return m, nil
			}
			m.settingsEditingIgnore = true
			m.settingsIgnoreIdx = 0
			return m, nil

		case "a":
			if m.settingsEditingIgnore {
				m.settingsInput.SetValue("")
				m.settingsInput.Focus()
				m.settingsStep = settingsStepAddPattern
				return m, textinput.Blink
			}

		case "d":
			if m.settingsEditingIgnore && m.settingsIgnoreIdx < len(m.cfg.IgnorePatterns) {
				m.cfg.IgnorePatterns = append(
					m.cfg.IgnorePatterns[:m.settingsIgnoreIdx],
					m.cfg.IgnorePatterns[m.settingsIgnoreIdx+1:]...,
				)
				// Clamp the cursor: deleting the last item leaves
				// settingsIgnoreIdx == len() which then renders out of
				// range and breaks subsequent up/down navigation.
				if m.settingsIgnoreIdx >= len(m.cfg.IgnorePatterns) {
					m.settingsIgnoreIdx = len(m.cfg.IgnorePatterns) - 1
				}
				if m.settingsIgnoreIdx < 0 {
					m.settingsIgnoreIdx = 0
				}
				// Surface SaveConfig failures. Silently swallowing them
				// leaves the in-memory pattern list out of sync with
				// .dotcorrc — the user thinks their edit was persisted
				// but the next dotcor run will resurrect the deleted
				// pattern from disk.
				if err := m.cfg.SaveConfig(); err != nil {
					m.err = fmt.Errorf("saving config: %w", err)
				}
				return m, nil
			}

		case "up", "k":
			if m.settingsEditingIgnore && m.settingsIgnoreIdx > 0 {
				m.settingsIgnoreIdx--
				return m, nil
			}

		case "down", "j":
			if m.settingsEditingIgnore && m.settingsIgnoreIdx < len(m.cfg.IgnorePatterns)-1 {
				m.settingsIgnoreIdx++
				return m, nil
			}
		}
	}

	return m, nil
}

func updateSettingsEditRemote(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
			m.settingsStep = settingsStepMain
			m.settingsInput.Blur()
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			newURL := strings.TrimSpace(m.settingsInput.Value())
			m.settingsInput.Blur()
			m.settingsStep = settingsStepMain

			// Validate here: it is pure string work, so a bad URL is
			// rejected instantly instead of after a round trip through git.
			if err := git.ValidateRemoteURL(newURL); err != nil {
				m.err = fmt.Errorf("invalid remote URL: %w", err)
				return m, nil
			}

			// Do not update the model optimistically. If git or the config
			// write rejects the URL, the pane would otherwise display the
			// rejected value as if configured while .git/config and
			// .dotcorrc both still held the old one — exactly the silent
			// divergence applyGitRemote is written to avoid. The value is
			// applied on the success message instead.
			m.err = nil
			return m, m.applyGitRemote(newURL)
		}
	}

	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	return m, cmd
}

func updateSettingsBackups(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
			m.settingsStep = settingsStepMain
			m.backups = nil
			return m, nil
		case keyMsg.String() == "c":
			return m, m.cleanBackups()
		}
	}
	return m, nil
}

func updateSettingsAddPattern(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Enter):
			val := m.settingsInput.Value()
			if val != "" {
				m.cfg.IgnorePatterns = append(m.cfg.IgnorePatterns, val)
				// Surface SaveConfig failures so the user knows when
				// their addition didn't reach .dotcorrc (disk full,
				// permission denied, etc.) — the previous silent swallow
				// left the in-memory state diverged from on-disk.
				if err := m.cfg.SaveConfig(); err != nil {
					m.err = fmt.Errorf("saving config: %w", err)
				}
			}
			m.settingsInput.Blur()
			m.settingsStep = settingsStepMain
			m.settingsEditingIgnore = false
			return m, nil
		case key.Matches(keyMsg, m.keys.Esc):
			m.settingsInput.Blur()
			m.settingsStep = settingsStepMain
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	return m, cmd
}

// applyGitRemote writes the remote to .git/config and then to .dotcorrc.
//
// This runs off the update loop. Each git call carries a 30s timeout and
// SetRemote forks two processes, so doing the work inline froze input and
// rendering for the whole duration with no spinner and no way to cancel.
//
// The config is snapshotted rather than shared: SaveConfig marshals every
// field, and marshalling m.cfg on this goroutine would race the settings
// view's in-place edits to IgnorePatterns.
func (m Model) applyGitRemote(newURL string) tea.Cmd {
	repoDir := m.repoDir
	// .dotcorrc lives inside the repository and is picked up by `git add -A`,
	// so a remote entered as https://user:ghp_token@host would be committed
	// and pushed, publishing the token. Persist the URL without its secret;
	// .git/config keeps exactly what the user typed and is never staged.
	stored := git.StripURLPassword(newURL)

	return func() tea.Msg {
		// Apply to .git/config FIRST. If git rejects the URL, abort before
		// persisting to .dotcorrc — otherwise the config file claims one URL
		// while push/pull use a different one (or none), and the divergence
		// stays silent until the user notices a sync failing.
		if newURL == "" {
			existing, err := git.GetRemoteURL(repoDir)
			if err != nil {
				return settingsMsg{err: fmt.Errorf("reading remote: %w", err)}
			}
			if existing != "" {
				if err := git.RemoveRemote(repoDir, "origin"); err != nil {
					return settingsMsg{err: fmt.Errorf("removing remote: %w", err)}
				}
			}
		} else if err := git.SetRemote(repoDir, "origin", newURL); err != nil {
			return settingsMsg{err: err}
		}

		// Report the value; the Update handler adopts it and writes the
		// config on the update goroutine. Saving a snapshot captured before
		// the git subprocesses ran would blind-write the whole file and
		// silently revert an ignore-pattern edit made in the meantime —
		// losing a security-relevant setting with no error shown.
		return settingsMsg{msg: "Remote saved", gitRemote: &stored}
	}
}

func (m Model) loadBackups() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		backups, err := core.ListBackups(cfg)
		if err != nil {
			return backupsMsg{err: err}
		}
		return backupsMsg{backups: backups}
	}
}

func (m Model) cleanBackups() tea.Cmd {
	cfg := m.cfg
	return func() tea.Msg {
		deleted, failed, freed, err := core.CleanOldBackups(30*24*time.Hour, 5, cfg)
		if err != nil {
			return settingsMsg{err: err}
		}
		return settingsMsg{msg: fmt.Sprintf("Cleaned %d backups (%d failed, freed %s)", deleted, failed, humanSize(freed))}
	}
}
