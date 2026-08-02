package stow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyFiles_AdoptWhenSourceLivesInsideHome pins the fix for an
// unreachable classification.
//
// ~/dotfiles/zshrc with ~/.zshrc → ~/dotfiles/zshrc is a very common layout.
// The in-$HOME check was a tautology — filepath.Join(home, Rel(home, p)) is
// just Clean(p) — so every file under $HOME short-circuited to Add and the
// $HOME symlink index was never consulted. The result was that ~/.zshrc was
// never repointed at the repo, leaving ~/.zshrc → ~/dotfiles/zshrc → repo,
// which breaks silently the moment ~/dotfiles is removed.
func TestClassifyFiles_AdoptWhenSourceLivesInsideHome(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	source := filepath.Join(homeDir, "dotfiles", "zshrc")
	writeFile(t, source, "export ZSH=1")

	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, homeLink))

	plan, err := ClassifyFiles([]string{source}, repoDir, homeDir, nil)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)

	cf := plan.Packages[0].Files[0]
	assert.Equal(t, ClassAdopt, cf.Class,
		"a file reached through an existing $HOME symlink must be adopted, not added")
	assert.Equal(t, homeLink, cf.HomeSymlink,
		"the $HOME symlink must be recorded so execute can repoint it")
}

// TestClassifyFiles_AdoptRepointsHomeSymlink verifies the end state: after
// execution the $HOME symlink points at the repo, not through the old source.
func TestClassifyFiles_AdoptRepointsHomeSymlink(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	source := filepath.Join(homeDir, "dotfiles", "zshrc")
	writeFile(t, source, "export ZSH=1")
	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(source, homeLink))

	plan, err := ClassifyFiles([]string{source}, repoDir, homeDir, nil)
	require.NoError(t, err)

	_, err = ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)

	target, err := os.Readlink(homeLink)
	require.NoError(t, err)
	resolved := target
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(homeLink), target)
	}
	resolved, err = filepath.EvalSymlinks(resolved)
	require.NoError(t, err, "the $HOME symlink must not dangle")

	resolvedRepo, err := filepath.EvalSymlinks(repoDir)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(resolved, resolvedRepo+string(filepath.Separator)),
		"$HOME symlink should now point into the repo, got %s", resolved)
}

// TestClassifyFiles_DirectHomeFileIsStillAdd guards the Add path the depth
// test is actually for.
func TestClassifyFiles_DirectHomeFileIsStillAdd(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	homeFile := filepath.Join(homeDir, ".bashrc")
	writeFile(t, homeFile, "alias ll='ls -la'")

	plan, err := ClassifyFiles([]string{homeFile}, repoDir, homeDir, nil)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassAdd, plan.Packages[0].Files[0].Class,
		"a file directly in $HOME has nothing to adopt")
}

// TestClassifyFiles_InHomeWithNoSymlinkIsTracked covers the remaining branch.
func TestClassifyFiles_InHomeWithNoSymlinkIsTracked(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	source := filepath.Join(homeDir, "dotfiles", "vimrc")
	writeFile(t, source, "set number")

	plan, err := ClassifyFiles([]string{source}, repoDir, homeDir, nil)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassTrack, plan.Packages[0].Files[0].Class,
		"a nested $HOME file with no $HOME symlink pointing at it is tracked")
}
