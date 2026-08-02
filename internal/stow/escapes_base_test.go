package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEscapesBase pins the distinction a plain HasPrefix(rel, "..") loses.
func TestEscapesBase(t *testing.T) {
	inside := []string{
		"..foo/bar.conf",
		"..config",
		"...hidden",
		"a/b.conf",
		".",
		".config/nvim/init.lua",
	}
	for _, rel := range inside {
		assert.False(t, escapesBase(rel), "%q is inside the base", rel)
	}

	outside := []string{
		"..",
		"../sibling",
		"../../up/two",
	}
	for _, rel := range outside {
		assert.True(t, escapesBase(rel), "%q escapes the base", rel)
	}
}

// TestClassifyFiles_DotDotPrefixedDirKeepsItsPath pins the fix end to end.
// A directory whose name begins with ".." is a legal path inside the
// selection; treating it as an escape collapsed every file to its bare
// basename, so two files with the same name in different subdirectories
// mapped onto one repo destination and one silently overwrote the other.
func TestClassifyFiles_DotDotPrefixedDirKeepsItsPath(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	selDir := filepath.Join(tmp, "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// A loose file at the root stops this being treated as a Stow parent, so
	// the whole selection becomes one package and relative paths matter.
	writeFile(t, filepath.Join(selDir, "top.conf"), "top")
	writeFile(t, filepath.Join(selDir, "..stash", "config"), "stashed")
	writeFile(t, filepath.Join(selDir, "normal", "config"), "normal")

	plan, err := ClassifyFiles([]string{selDir}, repoDir, homeDir, nil)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 3)

	rels := map[string]bool{}
	dests := map[string]bool{}
	for _, cf := range plan.Packages[0].Files {
		rels[cf.RelPath] = true
		dests[cf.RepoDest] = true
	}

	assert.True(t, rels[filepath.Join("..stash", "config")], "the ..stash path must be preserved, got %v", rels)
	assert.True(t, rels[filepath.Join("normal", "config")], "got %v", rels)
	assert.Len(t, dests, 3, "three source files must map to three distinct repo destinations")
}
