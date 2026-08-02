package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHistoryConfirm_DispatchesOnAction pins the fix for the wrong action
// running from a shared modal.
//
// stowResultMsg is handled by the top-level Update regardless of activeView,
// so a "N conflicts detected" prompt can be opened while History is active —
// and viewHistory renders it. History's handler used to ignore confirmAction
// and always run a git restore, using its own (empty) ref and path. The
// conflicts were then silently never resolved.
func TestHistoryConfirm_DispatchesOnAction(t *testing.T) {
	m := defaultModel()
	m.activeView = HistoryView
	m.packages = []stow.Package{{Name: "zsh"}}

	// The conflict prompt as the top-level Update sets it up.
	m.confirmOpen = true
	m.confirmAction = "resolve-conflicts"
	m.confirmTarget = "zsh"
	m.confirmRestoreRef = ""
	m.confirmFilePath = ""

	updated, cmd := m.updateHistory(tea.KeyMsg{Type: tea.KeyEnter})
	asModel := updated.(Model)

	require.NotNil(t, cmd, "confirming must dispatch a command")
	assert.False(t, asModel.confirmOpen, "the modal must close")

	// A restore with an empty ref and path is the bug's signature: it runs
	// `git checkout HEAD -- ""`. Resolving conflicts against a repo that
	// doesn't exist yields a stow error instead.
	msg := cmd()
	switch got := msg.(type) {
	case errMsg:
		t.Fatalf("history confirm ran a restore instead of resolving conflicts: %v", got.err)
	case stowResultMsg:
		// Correct branch: resolveConflicts was dispatched.
	default:
		t.Fatalf("unexpected message type %T", msg)
	}
}

// TestHistoryConfirm_RestoreStillWorks guards the action History owns.
func TestHistoryConfirm_RestoreStillWorks(t *testing.T) {
	m := defaultModel()
	m.activeView = HistoryView
	m.confirmOpen = true
	m.confirmAction = "restore"
	m.confirmRestoreRef = "abc1234"
	m.confirmFilePath = "zsh"

	updated, cmd := m.updateHistory(tea.KeyMsg{Type: tea.KeyEnter})
	asModel := updated.(Model)

	require.NotNil(t, cmd)
	assert.False(t, asModel.confirmOpen)
}

// TestConfirmAccept_UnknownActionIsCancel ensures an unrecognised action
// never falls through to another view's destructive default.
func TestConfirmAccept_UnknownActionIsCancel(t *testing.T) {
	m := defaultModel()
	m.confirmOpen = true
	m.confirmAction = "not-a-real-action"

	updated, cmd := m.confirmAccept()
	asModel := updated.(Model)

	assert.Nil(t, cmd, "an unknown action must not run anything")
	assert.False(t, asModel.confirmOpen, "the modal must still close")
}
