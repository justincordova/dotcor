package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func browserModel(t *testing.T, entries, height int) Model {
	t.Helper()
	home := t.TempDir()
	for i := 0; i < entries; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(home, fmt.Sprintf("f%03d", i)), []byte("x"), 0644))
	}
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.homeDir = home
	m.activeView = AddView
	m.width, m.height = 100, height
	m.buildBrowserItems()
	return m
}

// TestAddBrowser_FirstEntryIsVisible pins the fix for a dialog that overflowed
// the terminal.
//
// renderAddStep0 measured only the dialog chrome, not the body lines it emits
// itself (the path header, the rule, and the "… N more" row). The dialog came
// out 3 lines too tall, and clampDialogHeight trims from index 5 — exactly
// where the item list starts. The browser opened showing its fourth entry
// while the cursor sat invisibly on the first, so pressing space selected a
// file the user could not see.
func TestAddBrowser_FirstEntryIsVisible(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			m := browserModel(t, 200, height)

			body := stripANSI(m.View())

			assert.Contains(t, body, "f000",
				"the first entry — where the cursor starts — must be on screen")
			assert.Contains(t, body, selectionMarker,
				"the cursor marker must be visible")
		})
	}
}

// TestAddBrowser_FitsTerminalHeight asserts the dialog does not overflow, so
// clampDialogHeight never has rows to trim.
func TestAddBrowser_FitsTerminalHeight(t *testing.T) {
	for _, height := range []int{24, 30, 40, 60} {
		t.Run(fmt.Sprintf("height=%d", height), func(t *testing.T) {
			m := browserModel(t, 200, height)

			lines := strings.Split(m.View(), "\n")
			assert.LessOrEqual(t, len(lines), height,
				"the rendered view must fit the terminal")
		})
	}
}

// TestAddBrowser_CursorClampedWhenTreeShrinks pins the fix for a blank
// browser. Collapsing an ancestor removes every descendant row, so the tree
// can shrink far below the cursor. With cursor and scroll both past the end
// the render loop never runs and j/G appear dead, because they are gated on
// len(items)-1.
func TestAddBrowser_CursorClampedWhenTreeShrinks(t *testing.T) {
	m := browserModel(t, 200, 40)
	m.browserCursor = 150
	m.browserScroll = 140

	// Simulate the list shrinking underneath the cursor, as a collapse does.
	m.browserItems = m.browserItems[:10]
	m.browserAdjustScroll()

	assert.Less(t, m.browserCursor, 10, "the cursor must be clamped into the new list")
	assert.GreaterOrEqual(t, m.browserCursor, 0)
	assert.LessOrEqual(t, m.browserScroll, m.browserCursor)

	body := stripANSI(m.View())
	assert.Contains(t, body, "f00", "the browser must still show entries, not a blank pane")
}

// TestAddBrowser_CollapseKeepsBrowserPopulated drives the real key path.
func TestAddBrowser_CollapseKeepsBrowserPopulated(t *testing.T) {
	home := t.TempDir()
	nested := filepath.Join(home, "adir")
	require.NoError(t, os.MkdirAll(nested, 0755))
	for i := 0; i < 100; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(nested, fmt.Sprintf("c%03d", i)), []byte("x"), 0644))
	}
	for i := 0; i < 5; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(home, fmt.Sprintf("t%03d", i)), []byte("x"), 0644))
	}

	m := NewModel(testCfg(), "test")
	m.loading = false
	m.homeDir = home
	m.activeView = AddView
	m.width, m.height = 100, 40
	m.browserExpanded[nested] = true
	m.buildBrowserItems()

	// Put the cursor on a child of the expanded directory. Entries are
	// walked in sorted order, so "adir" is index 0 and its children follow.
	require.Greater(t, len(m.browserItems), 90)
	m.browserCursor = 90
	require.Equal(t, 1, m.browserItems[m.browserCursor].indent,
		"index 90 should be a child of the expanded directory")
	m.browserAdjustScroll()

	// Collapse from a descendant row: the ancestor folds up.
	updated, _ := m.browserHandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	after := updated.(Model)

	items := after.buildBrowserItems()
	assert.Less(t, after.browserCursor, len(items), "the cursor must remain within the collapsed list")
	assert.GreaterOrEqual(t, after.browserCursor, 0)

	body := stripANSI(after.View())
	assert.Contains(t, body, "adir", "the browser must not be blank after collapsing")
}
