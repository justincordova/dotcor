package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyFiles_SameBasenameSelectionsDoNotCollide pins the fix for
// silent data loss.
//
// Repo-relative paths were measured from the selection directory, and
// derivePkgName collapses a deep path to its leaf. Selecting ~/.config/nvim
// and ~/.local/share/nvim therefore produced the same package AND the same
// relative path, so the second file overwrote the first in the repo and both
// $HOME symlinks pointed at it. One file's only copy was destroyed — it was
// not in $HOME, not in the repo, not in backups — and the operation reported
// success.
func TestClassifyFiles_SameBasenameSelectionsDoNotCollide(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	a := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	b := filepath.Join(homeDir, ".local", "share", "nvim", "init.lua")
	writeFile(t, a, "CONFIG-NVIM")
	writeFile(t, b, "LOCAL-SHARE-NVIM")

	plan, err := ClassifyFiles([]string{filepath.Dir(a), filepath.Dir(b)}, repoDir, homeDir, nil)
	require.NoError(t, err)

	dests := map[string]bool{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			assert.False(t, dests[cf.RepoDest],
				"two source files must never share a repo destination: %s", cf.RepoDest)
			dests[cf.RepoDest] = true
		}
	}

	result, err := ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)
	require.Empty(t, result.Failures)

	gotA, err := os.ReadFile(a)
	require.NoError(t, err)
	gotB, err := os.ReadFile(b)
	require.NoError(t, err)

	assert.Equal(t, "CONFIG-NVIM", string(gotA), "each file must keep its own content")
	assert.Equal(t, "LOCAL-SHARE-NVIM", string(gotB))
}

// TestClassifyFiles_RepoMirrorsHomeLayout pins the contract every other
// component assumes: Link, Unlink and DiscoverPackages all map
// repo/<pkg>/<rel> to $HOME/<rel>, and SPEC.md documents package nvim as
// holding .config/nvim/init.lua.
func TestClassifyFiles_RepoMirrorsHomeLayout(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	src := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	writeFile(t, src, "vim.opt.number = true")

	plan, err := ClassifyFiles([]string{filepath.Dir(src)}, repoDir, homeDir, nil)
	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)

	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"),
		plan.Packages[0].Files[0].RelPath,
		"the package must mirror the $HOME-relative path")
}

// TestClassifyFiles_AddedFileRoundTripsThroughLinkAndUnlink is the end-to-end
// consequence: after adding, discovery must report the package linked, and
// unlink must be able to remove the link it created.
func TestClassifyFiles_AddedFileRoundTripsThroughLinkAndUnlink(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	src := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	writeFile(t, src, "vim.opt.number = true")

	plan, err := ClassifyFiles([]string{filepath.Dir(src)}, repoDir, homeDir, nil)
	require.NoError(t, err)
	_, err = ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)

	packages, err := DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusLinked, packages[0].Status,
		"a file that was just added must show as linked, not unlinked")

	// Stowing again must be a no-op, not create a stray link at $HOME root.
	_, err = Link(repoDir, homeDir, packages[0].Name, nil)
	require.NoError(t, err)
	_, strayErr := os.Lstat(filepath.Join(homeDir, "init.lua"))
	assert.True(t, os.IsNotExist(strayErr), "stow must not create a stray ~/init.lua")

	// And unlink must be able to remove the real link.
	un, err := Unlink(repoDir, homeDir, packages[0].Name)
	require.NoError(t, err)
	assert.Equal(t, 1, un.Unlinked, "unlink must remove the link that was created")
}

// TestClassifyFiles_StowParentLayoutUnchanged guards the other convention:
// inside a Stow-style parent, package contents are already $HOME-relative.
func TestClassifyFiles_StowParentLayoutUnchanged(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	dotfiles := filepath.Join(homeDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(dotfiles, "nvim", ".config", "nvim", "init.lua"), "nvim")
	writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")

	plan, err := ClassifyFiles([]string{dotfiles}, repoDir, homeDir, nil)
	require.NoError(t, err)

	rels := map[string]string{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			rels[pkg.Name] = cf.RelPath
		}
	}
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), rels["nvim"])
	assert.Equal(t, ".zshrc", rels["zsh"])
}

// TestClassifyFiles_OutsideHomeKeepsSelectionRelativePaths guards sources
// that have no $HOME mapping at all.
func TestClassifyFiles_OutsideHomeKeepsSelectionRelativePaths(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	external := filepath.Join(tmp, "external", "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(external, "sub", "app.conf"), "cfg")
	writeFile(t, filepath.Join(external, "top.conf"), "top")

	plan, err := ClassifyFiles([]string{external}, repoDir, homeDir, nil)
	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)

	rels := map[string]bool{}
	for _, cf := range plan.Packages[0].Files {
		rels[cf.RelPath] = true
	}
	assert.True(t, rels[filepath.Join("sub", "app.conf")], "got %v", rels)
	assert.True(t, rels["top.conf"], "got %v", rels)
}
