package tui

import (
	"sync"
	"testing"

	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
)

// TestGetDiff_DoesNotReadSharedStateOnGoroutine pins the fix for a fatal
// data race. getDiff used to capture the whole Model and read m.expanded —
// a map, so it is shared rather than copied — inside the tea.Cmd goroutine,
// while Update writes that same map on every enter keypress. A Go map read
// racing a map write is an unrecoverable runtime fatal error.
//
// Run with -race to see the original failure.
func TestGetDiff_DoesNotReadSharedStateOnGoroutine(t *testing.T) {
	m := modelWithPackages()
	m.repoDir = t.TempDir()
	cmd := getDiff(m)

	var wg sync.WaitGroup
	wg.Add(2)

	// The command goroutine, exactly as Bubble Tea runs it.
	go func() {
		defer wg.Done()
		_ = cmd()
	}()

	// Update mutating the shared expansion map, as pressing enter does.
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			m.expanded[0] = !m.expanded[0]
		}
	}()

	wg.Wait()
}

// TestGetFileHistory_ResolvesTargetEagerly asserts the target is resolved on
// the caller's goroutine, so the closure carries no shared state.
func TestGetFileHistory_ResolvesTargetEagerly(t *testing.T) {
	m := modelWithPackages()
	m.expanded[0] = true
	m.selectedFile = 0

	// Resolution must already have happened by the time the cmd is built.
	assert.Equal(t, ".zshrc", historyFilePath(m))

	m.repoDir = t.TempDir()
	cmd := getFileHistory(m)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = cmd()
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2000; i++ {
			m.expanded[0] = !m.expanded[0]
		}
	}()
	wg.Wait()
}

// TestSelectedFileRelPath covers the boundaries the helper now guards.
func TestSelectedFileRelPath(t *testing.T) {
	m := modelWithPackages()

	m.expanded[0] = false
	assert.Empty(t, selectedFileRelPath(m), "a collapsed package has no selected file")

	m.expanded[0] = true
	m.selectedFile = 0
	assert.Equal(t, ".zshrc", selectedFileRelPath(m))

	m.selectedFile = 99
	assert.Empty(t, selectedFileRelPath(m), "an out-of-range file index must not index the slice")

	m.selectedFile = -1
	assert.Empty(t, selectedFileRelPath(m), "a negative file index must not index the slice")

	m.selectedFile = 0
	m.selectedPkg = -1
	assert.Empty(t, selectedFileRelPath(m), "a negative package index must not index the slice")

	m.selectedPkg = 99
	assert.Empty(t, selectedFileRelPath(m))
}

// TestHistoryFilePath_EmptyPackages guards the no-package case.
func TestHistoryFilePath_EmptyPackages(t *testing.T) {
	m := defaultModel()
	m.packages = nil
	assert.Empty(t, historyFilePath(m))
}

func modelWithPackages() Model {
	m := defaultModel()
	m.packages = []stow.Package{
		{Name: "zsh", Files: []stow.FileEntry{{RelPath: ".zshrc"}}},
	}
	m.selectedPkg = 0
	m.selectedFile = 0
	return m
}
