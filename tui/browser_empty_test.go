package tui

import (
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newBrowserModelWithEmptyHome returns a model whose browser is pointed at
// an empty $HOME so buildBrowserItems() yields zero rows — the condition
// that used to drive browserCursor to -1 on any downward navigation.
func newBrowserModelWithEmptyHome(t *testing.T) Model {
	t.Helper()
	emptyHome := t.TempDir()

	m := defaultModel()
	m.homeDir = emptyHome
	m.repoDir = t.TempDir()
	m.activeView = AddView
	m.addStep = addStepSelect
	m.loading = false
	// Fresh browser state so buildBrowserItems walks emptyHome.
	m.browserEntries = make(map[string][]os.DirEntry)
	m.browserExpanded = make(map[string]bool)
	m.browserSelected = make(map[string]bool)
	return m
}

// TestBrowser_EmptyHome_NavigationDoesNotPanic guards the empty-browser
// state: pressing a downward-motion key when there are no items used to set
// browserCursor to len(items)-1 == -1 and browserAdjustScroll mirrored that
// into a negative browserScroll. The renderer early-returns for an empty
// list today, but a negative scroll is incoherent state that becomes an
// items[-1] panic the moment the list is non-empty again. Cursor and scroll
// must stay non-negative, and View() must not panic on the resulting state.
func TestBrowser_EmptyHome_NavigationDoesNotPanic(t *testing.T) {
	for _, keyStr := range []string{"G", "down", "j", "pgdown", "ctrl+d", "end"} {
		t.Run(keyStr, func(t *testing.T) {
			m := newBrowserModelWithEmptyHome(t)

			items := m.buildBrowserItems()
			require.Empty(t, items, "precondition: browser must have no items")

			var key tea.KeyMsg
			switch keyStr {
			case "down":
				key = tea.KeyMsg{Type: tea.KeyDown}
			case "pgdown":
				key = tea.KeyMsg{Type: tea.KeyPgDown}
			case "ctrl+d":
				key = tea.KeyMsg{Type: tea.KeyCtrlD}
			case "end":
				key = tea.KeyMsg{Type: tea.KeyEnd}
			default:
				key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyStr)}
			}

			updated, _ := m.Update(key)
			got := updated.(Model)

			assert.GreaterOrEqual(t, got.browserCursor, 0, "cursor must never go negative")
			assert.GreaterOrEqual(t, got.browserScroll, 0, "scroll must never go negative")

			// Rendering must not panic on the resulting state.
			assert.NotPanics(t, func() {
				_ = got.View()
			}, "View() must not panic after navigating an empty browser")
		})
	}
}
