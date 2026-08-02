package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func homeWithFiles(t *testing.T, n int) string {
	t.Helper()
	home := t.TempDir()
	for i := 0; i < n; i++ {
		sub := filepath.Join(home, "dir", string(rune('a'+i%26)))
		require.NoError(t, os.MkdirAll(sub, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(sub, "f.conf"), []byte("x"), 0644))
	}
	return home
}

// TestAddView_WarmsBrowserCacheOnOpen pins the fix for a memo that never took
// effect. View receives the Model by value, so buildBrowserItems wrote the
// cache to a copy that was discarded on return: every frame re-walked the
// home directory, running Lstat and EvalSymlinks per entry.
func TestAddView_WarmsBrowserCacheOnOpen(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.homeDir = homeWithFiles(t, 20)
	m.width, m.height = 100, 40

	updated, _ := m.updateDashboard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	asModel := updated.(Model)

	require.Equal(t, AddView, asModel.activeView)
	assert.NotNil(t, asModel.browserItems,
		"opening the Add view must populate the browser memo on the update goroutine")
}

// TestAddView_RenderDoesNotInvalidateCache asserts View is a pure reader.
func TestAddView_RenderDoesNotInvalidateCache(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.homeDir = homeWithFiles(t, 20)
	m.width, m.height = 100, 40
	m.activeView = AddView
	m.buildBrowserItems()

	before := len(m.browserItems)
	require.Positive(t, before)

	_ = m.View()
	_ = m.View()

	assert.Len(t, m.browserItems, before, "rendering must not disturb the memo")
}

// TestBrowserItemsForRender_UsesCache proves the render path reads the memo
// rather than rebuilding, by seeding it with a value the filesystem cannot
// produce.
func TestBrowserItemsForRender_UsesCache(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.homeDir = homeWithFiles(t, 5)
	m.browserItems = []browserItem{{path: "/sentinel", name: "sentinel"}}

	got := m.browserItemsForRender()

	require.Len(t, got, 1)
	assert.Equal(t, "sentinel", got[0].name, "the render path must read the cache, not re-walk the filesystem")
}
