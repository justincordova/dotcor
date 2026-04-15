package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justincordova/dotcor/internal/core"
)

type settingsMsg struct {
	msg string
	err error
}

type backupsMsg struct {
	backups []core.BackupInfo
	err     error
}

func viewSettings(m Model) string {
	switch m.settingsStep {
	case 1:
		return viewSettingsEditRemote(m)
	case 3:
		return viewSettingsBackups(m)
	case 4:
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
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)))
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
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)))
	b.WriteString("\n")
	if len(m.cfg.IgnorePatterns) == 0 {
		b.WriteString(dimStyle.Render("  (none)\n"))
	} else {
		for i, p := range m.cfg.IgnorePatterns {
			if m.settingsEditingIgnore && i == m.settingsIgnoreIdx {
				fmt.Fprintf(&b, "  %s %s\n", accentStyle.Render(">"), textStyle.Render(p))
			} else {
				fmt.Fprintf(&b, "  %s %s\n", dimStyle.Render("•"), textStyle.Render(p))
			}
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

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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
	return lipgloss.JoinVertical(lipgloss.Left, header, subviewContent(m.width, m.height-3, body), footer)
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
			ts := dimStyle.Render(backup.Timestamp.Format("2006-01-02 15:04"))
			src := textStyle.Render(truncate(backup.SourcePath, m.width-40))
			size := dimStyle.Render(humanSize(backup.Size))
			fmt.Fprintf(&b, "  %s  %s  %s\n", ts, src, size)
		}
	}

	body := subviewContent(m.width, m.height-3, b.String())
	footer := subviewFooter(m.width,
		kbd("c", "clean >30d"),
		kbd("esc", "back"),
	)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
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
	return lipgloss.JoinVertical(lipgloss.Left, header, subviewContent(m.width, m.height-3, body), footer)
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
			m.settingsStep = 0
			return m, nil
		}

		switch keyMsg.String() {
		case "e":
			m.settingsStep = 1
			m.settingsInput.SetValue(m.cfg.GitRemote)
			m.settingsInput.Focus()
			return m, textinput.Blink

		case "b":
			m.settingsStep = 3
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
				m.settingsStep = 4
				return m, textinput.Blink
			}

		case "d":
			if m.settingsEditingIgnore && m.settingsIgnoreIdx < len(m.cfg.IgnorePatterns) {
				m.cfg.IgnorePatterns = append(
					m.cfg.IgnorePatterns[:m.settingsIgnoreIdx],
					m.cfg.IgnorePatterns[m.settingsIgnoreIdx+1:]...,
				)
				_ = m.cfg.SaveConfig()
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
			m.settingsStep = 0
			m.settingsInput.Blur()
			return m, nil

		case key.Matches(keyMsg, m.keys.Enter):
			m.cfg.GitRemote = m.settingsInput.Value()
			m.settingsInput.Blur()
			m.settingsStep = 0
			if err := m.cfg.SaveConfig(); err != nil {
				m.err = err
				return m, nil
			}
			m.statusMsg = "Remote saved"
			return m, clearStatusAfter(3 * time.Second)
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
			m.settingsStep = 0
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
				_ = m.cfg.SaveConfig()
			}
			m.settingsInput.Blur()
			m.settingsStep = 0
			m.settingsEditingIgnore = false
			return m, nil
		case key.Matches(keyMsg, m.keys.Esc):
			m.settingsInput.Blur()
			m.settingsStep = 0
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	return m, cmd
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
