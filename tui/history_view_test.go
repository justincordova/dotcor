package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/stretchr/testify/assert"
)

// TestHistoryDKey_SwitchesToDiffView pins the fix for issue #6:
// pressing "D" on the history view must transition to DiffView so the
// user sees the diff they requested. The previous implementation dispatched
// diffFromCommit but left activeView on HistoryView.
func TestHistoryDKey_SwitchesToDiffView(t *testing.T) {
	m := defaultModel()
	m.activeView = HistoryView
	m.commits = []git.CommitInfo{{Hash: "abcdef1234567890"}}
	m.selectedCommit = 0

	updated, _ := m.updateHistory(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	asModel := updated.(Model)

	assert.Equal(t, DiffView, asModel.activeView, "D should switch activeView to DiffView")
}

// TestHistoryDKey_NoOpWithoutCommits guards against an index-out-of-bounds
// when the user presses D before history has loaded.
func TestHistoryDKey_NoOpWithoutCommits(t *testing.T) {
	m := defaultModel()
	m.activeView = HistoryView
	m.commits = nil
	m.selectedCommit = 0

	updated, cmd := m.updateHistory(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	asModel := updated.(Model)

	assert.Equal(t, HistoryView, asModel.activeView, "no-op should leave activeView unchanged")
	assert.Nil(t, cmd, "no-op should return nil cmd")
}
