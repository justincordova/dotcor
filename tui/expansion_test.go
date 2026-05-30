package tui

import (
	"testing"

	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
)

// TestPackagesMsg_RemapsExpansionByName guards against the expansion map
// (keyed by slice index) marking the wrong package as expanded after the
// package list changes. Deleting an earlier package shifts every later
// package down one index; without name-based remapping, the expansion
// would stay on the now-vacated index and highlight an unrelated package.
func TestPackagesMsg_RemapsExpansionByName(t *testing.T) {
	m := defaultModel()
	m.loading = false
	m.packages = []stow.Package{
		{Name: "aaa"},
		{Name: "bbb"},
		{Name: "ccc"},
	}
	// User expands "ccc" (index 2).
	m.expanded = map[int]bool{2: true}

	// "aaa" is deleted; a fresh discovery returns the remaining two in
	// alphabetical order, so "ccc" is now at index 1.
	updated, _ := m.Update(packagesMsg{packages: []stow.Package{
		{Name: "bbb"},
		{Name: "ccc"},
	}})
	got := updated.(Model)

	assert.True(t, got.expanded[1], "ccc should still be expanded at its new index 1")
	assert.False(t, got.expanded[2], "the stale index 2 must not be expanded (it no longer exists)")
	assert.False(t, got.expanded[0], "bbb (index 0) was never expanded and must stay collapsed")
}

// TestPackagesMsg_DropsExpansionForRemovedPackage ensures expansion state
// for a package that no longer exists is discarded rather than leaking
// onto whatever package now occupies that index.
func TestPackagesMsg_DropsExpansionForRemovedPackage(t *testing.T) {
	m := defaultModel()
	m.loading = false
	m.packages = []stow.Package{
		{Name: "aaa"},
		{Name: "bbb"},
	}
	// User expanded "aaa" (index 0).
	m.expanded = map[int]bool{0: true}

	// "aaa" is deleted; only "bbb" remains, now at index 0.
	updated, _ := m.Update(packagesMsg{packages: []stow.Package{
		{Name: "bbb"},
	}})
	got := updated.(Model)

	assert.False(t, got.expanded[0], "bbb must not inherit aaa's expansion state")
	assert.Empty(t, got.expanded, "expansion for the removed package should be dropped entirely")
}
