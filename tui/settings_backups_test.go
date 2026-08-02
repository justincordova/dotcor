package tui

import (
	"io"
	"log/slog"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSettingsMsg_ReloadsBackupListWhilePaneIsOpen pins the fix for a stale
// list. cleanBackups reports "Cleaned N backups" via settingsMsg, whose
// handler only ran refreshAll — packages, git status and commits, not the
// backup list. The pane kept showing every deleted entry until the user left
// and re-entered it.
func TestSettingsMsg_ReloadsBackupListWhilePaneIsOpen(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = SettingsView
	m.settingsStep = settingsStepBackups
	m.backups = []core.BackupInfo{{Timestamp: time.Now()}}

	updated, cmd := m.Update(settingsMsg{msg: "Cleaned 12 backups"})
	require.NotNil(t, cmd)

	assert.Equal(t, "Cleaned 12 backups", updated.(Model).statusMsg)
	assert.True(t, batchProduces(cmd, func(msg tea.Msg) bool {
		_, ok := msg.(backupsMsg)
		return ok
	}), "the backup list must be reloaded while the pane is open")
}

// TestSettingsMsg_DoesNotReloadBackupsOutsidePane avoids pointless work when
// the list is not on screen.
func TestSettingsMsg_DoesNotReloadBackupsOutsidePane(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.activeView = SettingsView
	m.settingsStep = settingsStepMain

	_, cmd := m.Update(settingsMsg{msg: "Remote updated"})
	require.NotNil(t, cmd)

	assert.False(t, batchProduces(cmd, func(msg tea.Msg) bool {
		_, ok := msg.(backupsMsg)
		return ok
	}), "the backup list should not be reloaded when the pane is closed")
}

func testCfg() *config.Config {
	return &config.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// batchProduces runs a command (flattening a tea.Batch) and reports whether
// any resulting message satisfies pred.
func batchProduces(cmd tea.Cmd, pred func(tea.Msg) bool) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, sub := range batch {
			if batchProduces(sub, pred) {
				return true
			}
		}
		return false
	}
	return pred(msg)
}
