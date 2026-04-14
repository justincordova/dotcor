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
	case 2:
		return viewSettingsEditGit(m)
	case 3:
		return viewSettingsBackups(m)
	default:
		return viewSettingsMain(m)
	}
}

func viewSettingsMain(m Model) string {
	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render("Settings"))
	b.WriteString("\n\n")

	gitEnabled := "disabled"
	gitStyle := errorStyle
	if m.cfg.GitEnabled {
		gitEnabled = "enabled"
		gitStyle = successStyle
	}

	remote := m.cfg.GitRemote
	if remote == "" {
		remote = dimStyle.Render("(not configured)")
	}

	b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Git:"), gitStyle.Render(gitEnabled)))
	b.WriteString(fmt.Sprintf("  %s %s\n", textStyle.Render("Remote:"), dimStyle.Render(remote)))

	b.WriteString("\n")
	b.WriteString(accentStyle.Render("Ignore Patterns:"))
	b.WriteString("\n")
	if len(m.cfg.IgnorePatterns) == 0 {
		b.WriteString(dimStyle.Render("  (none)"))
		b.WriteString("\n")
	} else {
		for _, p := range m.cfg.IgnorePatterns {
			b.WriteString(fmt.Sprintf("  %s %s\n", dimStyle.Render("•"), dimStyle.Render(p)))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("─────────────────────────────────"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("e"), descStyle.Render("edit git remote")))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("t"), descStyle.Render("toggle git enabled")))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("b"), descStyle.Render("manage backups")))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("esc"), descStyle.Render("back to dashboard")))

	content := b.String()
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
}

func viewSettingsEditRemote(m Model) string {
	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render("Edit Git Remote"))
	b.WriteString("\n\n")
	b.WriteString(textStyle.Render("Enter remote URL:"))
	b.WriteString("\n")
	b.WriteString(m.settingsInput.View())
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("Enter to save, Esc to cancel"))

	content := b.String()
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
}

func viewSettingsEditGit(m Model) string {
	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render("Toggle Git"))
	b.WriteString("\n\n")

	status := "disabled"
	if m.cfg.GitEnabled {
		status = "enabled"
	}
	b.WriteString(fmt.Sprintf("Git is currently: %s\n", textStyle.Render(status)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press t to toggle, Esc to cancel"))

	content := b.String()
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
}

func viewSettingsBackups(m Model) string {
	var b strings.Builder

	b.WriteString(accentStyle.Bold(true).Render("Backup Management"))
	b.WriteString("\n\n")

	if len(m.backups) == 0 {
		b.WriteString(dimStyle.Render("No backups found"))
	} else {
		b.WriteString(fmt.Sprintf("Total backups: %s\n\n", textStyle.Render(fmt.Sprintf("%d", len(m.backups)))))
		maxDisplay := 20
		for i, backup := range m.backups {
			if i >= maxDisplay {
				b.WriteString(dimStyle.Render(fmt.Sprintf("  ... and %d more\n", len(m.backups)-maxDisplay)))
				break
			}
			b.WriteString(fmt.Sprintf("  %s %s %s\n",
				dimStyle.Render(backup.Timestamp.Format("2006-01-02 15:04")),
				textStyle.Render(backup.SourcePath),
				dimStyle.Render(fmt.Sprintf("(%d bytes)", backup.Size)),
			))
		}
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("c"), descStyle.Render("clean old backups (>30 days)")))
	b.WriteString(fmt.Sprintf("  %s %s\n", keyStyle.Render("esc"), descStyle.Render("back to settings")))

	content := b.String()
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.activeView = DashboardView
			m.settingsStep = 0
			return m, nil

		case msg.String() == "e":
			m.settingsStep = 1
			m.settingsInput.SetValue(m.cfg.GitRemote)
			m.settingsInput.Focus()
			return m, textinput.Blink

		case msg.String() == "t":
			m.cfg.GitEnabled = !m.cfg.GitEnabled
			if err := m.cfg.SaveConfig(); err != nil {
				m.err = err
			}
			return m, nil

		case msg.String() == "b":
			m.settingsStep = 3
			return m, m.loadBackups()
		}
	}

	return m, nil
}

func updateSettingsEditRemote(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.settingsStep = 0
			m.settingsInput.Blur()
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			m.cfg.GitRemote = m.settingsInput.Value()
			m.settingsInput.Blur()
			m.settingsStep = 0
			if err := m.cfg.SaveConfig(); err != nil {
				m.err = err
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.settingsInput, cmd = m.settingsInput.Update(msg)
	return m, cmd
}

func updateSettingsBackups(m Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case backupsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.backups = msg.backups
		return m, nil

	case settingsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.statusMsg = msg.msg
		}
		return m, m.loadBackups()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Esc):
			m.settingsStep = 0
			m.backups = nil
			return m, nil

		case msg.String() == "c":
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
		return settingsMsg{msg: fmt.Sprintf("Cleaned %d backups (%d failed, freed %d bytes)", deleted, failed, freed)}
	}
}
