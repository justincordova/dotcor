package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCtrlC_QuitsFromEveryView pins the fix for an app that could not be
// exited. The Quit binding was only matched in the dashboard, help and logs
// handlers, and Bubble Tea neither handles ctrl+c itself nor leaves SIGINT
// deliverable (it puts the terminal in raw mode) — so the Add wizard,
// Settings, Diff, History and the init flow had no escape hatch.
func TestCtrlC_QuitsFromEveryView(t *testing.T) {
	views := map[string]View{
		"dashboard": DashboardView,
		"add":       AddView,
		"diff":      DiffView,
		"history":   HistoryView,
		"settings":  SettingsView,
		"logs":      LogsView,
		"help":      HelpView,
	}

	for name, view := range views {
		t.Run(name, func(t *testing.T) {
			m := NewModel(testCfg(), "test")
			m.loading = false
			m.activeView = view

			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

			require.NotNil(t, cmd, "ctrl+c must produce a command")
			assert.Equal(t, tea.Quit(), cmd(), "ctrl+c must quit from %s", name)
		})
	}
}

// TestCtrlC_QuitsFromTextInputStates covers the states that swallow keys.
func TestCtrlC_QuitsFromTextInputStates(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		m := NewModel(testCfg(), "test")
		m.loading = false
		m.searching = true

		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		require.NotNil(t, cmd)
		assert.Equal(t, tea.Quit(), cmd())
	})

	t.Run("init wizard", func(t *testing.T) {
		m := NewModel(testCfg(), "test")
		m.loading = false
		m.initStep = 2

		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		require.NotNil(t, cmd)
		assert.Equal(t, tea.Quit(), cmd())
	})
}

// TestStowConflicts_NoInvisibleModalOutsideDashboard pins the fix for a
// dialog armed in a view that neither draws nor consumes it. The user would
// return to the dashboard later to find a stale prompt, or press enter in a
// view that dispatches a different action entirely.
func TestStowConflicts_NoInvisibleModalOutsideDashboard(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = AddView

	updated, _ := m.Update(stowResultMsg{pkgName: "zsh", conflicts: []string{".zshrc"}})
	asModel := updated.(Model)

	assert.False(t, asModel.confirmOpen,
		"no modal may be opened in a view that does not render one")
	assert.Contains(t, asModel.statusMsg, "conflicts",
		"the user must still be told about the conflicts")
}

// TestStowConflicts_OpensModalOnDashboard keeps the normal path working.
func TestStowConflicts_OpensModalOnDashboard(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DashboardView

	updated, _ := m.Update(stowResultMsg{pkgName: "zsh", conflicts: []string{".zshrc"}})
	asModel := updated.(Model)

	assert.True(t, asModel.confirmOpen)
	assert.Equal(t, "resolve-conflicts", asModel.confirmAction)
	assert.Equal(t, "zsh", asModel.confirmTarget)
}

// TestStowConflicts_ClearsStaleDialogState ensures a restore dialog's ref and
// path cannot leak into the conflict action.
func TestStowConflicts_ClearsStaleDialogState(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = DashboardView
	m.confirmOpen = true
	m.confirmAction = "restore"
	m.confirmRestoreRef = "abc1234"
	m.confirmFilePath = "some/file"

	updated, _ := m.Update(stowResultMsg{pkgName: "zsh", conflicts: []string{".zshrc"}})
	asModel := updated.(Model)

	assert.Equal(t, "resolve-conflicts", asModel.confirmAction)
	assert.Empty(t, asModel.confirmRestoreRef, "the stale restore ref must not survive")
	assert.Empty(t, asModel.confirmFilePath, "the stale file path must not survive")
}
