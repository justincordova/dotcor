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
	default:
		return viewSettingsMain(m)
	}
}

func viewSettingsMain(m Model) string {
	header := subviewHeader(m.width, "Settings", nil)

	var b strings.Builder

	// Git section
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
		for _, p := range m.cfg.IgnorePatterns {
			fmt.Fprintf(&b, "  %s %s\n", dimStyle.Render("•"), textStyle.Render(p))
		}
	}

	body := subviewContent(m.width, m.height-3, b.String())
	footer := subviewFooter(m.width,
		kbd("e", "edit remote"),
		kbd("b", "backups"),
		kbd("esc", "back"),
	)

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
			size := dimStyle.Render(fmt.Sprintf("%d B", backup.Size))
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

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.settingsStep {
	case 1:
		return updateSettingsEditRemote(m, msg)
	case 3:
		return updateSettingsBackups(m, msg)
	default:
		return updateSettingsMain(m, msg)
	}
}

func updateSettingsMain(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Esc):
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
		return settingsMsg{msg: fmt.Sprintf("Cleaned %d backups (%d failed, freed %d B)", deleted, failed, freed)}
	}
}
