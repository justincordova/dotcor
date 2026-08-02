package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkStatus_AgreesThroughSymlinkedHomeAncestor pins the fix for three
// code paths disagreeing about one symlink.
//
// When an ancestor of the link is itself a symlink (a very common layout:
// ~/.config → /data/config), a relative link target resolved lexically points
// somewhere else than where the kernel resolves it. Link/DiscoverPackages
// compared lexically while Unlink resolved, so a correctly-linked file was
// reported as a conflict by Link, as foreign by LinkWithBackup ("use 'o' to
// adopt" — on the user's own correctly-linked file), and as unlinked by the
// dashboard, while Unlink removed it just fine.
func TestLinkStatus_AgreesThroughSymlinkedHomeAncestor(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	// Deliberately at a DIFFERENT depth from ~/.config. A relative link
	// target computed from the lexical parent only happens to resolve
	// correctly when the two depths coincide.
	realConfig := filepath.Join(tmp, "data", "deep", "nested", "config")

	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(realConfig, 0755))
	// ~/.config is a symlink to a directory outside $HOME.
	require.NoError(t, os.Symlink(realConfig, filepath.Join(homeDir, ".config")))

	repoFile := filepath.Join(repoDir, "nvim", ".config", "init.lua")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoFile), 0755))
	require.NoError(t, os.WriteFile(repoFile, []byte("vim.opt.number = true"), 0644))

	// Link through the symlinked ancestor.
	result, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)
	require.Empty(t, result.Conflicts, "first link should be clean")
	require.Equal(t, 1, result.Linked)

	target := filepath.Join(homeDir, ".config", "init.lua")
	resolved, err := filepath.EvalSymlinks(target)
	require.NoError(t, err, "the created symlink must not dangle")
	wantResolved, err := filepath.EvalSymlinks(repoFile)
	require.NoError(t, err)
	assert.Equal(t, wantResolved, resolved, "the link must actually reach the repo file")

	// Re-linking must recognise its own work, not report a conflict.
	again, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)
	assert.Empty(t, again.Conflicts, "Link must recognise a link it created")
	assert.Empty(t, again.Foreign, "Link must not call its own link foreign")

	// Discovery must agree.
	packages, err := DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusLinked, packages[0].Status, "discovery must agree the package is linked")

	// And unlink must still remove it.
	un, err := Unlink(repoDir, homeDir, "nvim")
	require.NoError(t, err)
	assert.Equal(t, 1, un.Unlinked)
}

// TestSymlinkTargetsPath_RejectsForeignLink guards against the helper being
// too permissive — a link pointing elsewhere must never be claimed as ours.
func TestSymlinkTargetsPath_RejectsForeignLink(t *testing.T) {
	tmp := t.TempDir()
	repoFile := filepath.Join(tmp, "repo", "zshrc")
	otherFile := filepath.Join(tmp, "elsewhere", "zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoFile), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(otherFile), 0755))
	require.NoError(t, os.WriteFile(repoFile, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(otherFile, []byte("b"), 0644))

	link := filepath.Join(tmp, ".zshrc")
	require.NoError(t, os.Symlink(otherFile, link))

	assert.False(t, symlinkTargetsPath(link, repoFile))
	assert.True(t, symlinkTargetsPath(link, otherFile))
}

// TestSymlinkTargetsPath_NonSymlinkAndMissing covers the degenerate inputs.
func TestSymlinkTargetsPath_NonSymlinkAndMissing(t *testing.T) {
	tmp := t.TempDir()
	regular := filepath.Join(tmp, "regular")
	require.NoError(t, os.WriteFile(regular, []byte("x"), 0644))

	assert.False(t, symlinkTargetsPath(regular, regular), "a regular file is not a link to itself")
	assert.False(t, symlinkTargetsPath(filepath.Join(tmp, "missing"), regular))
}

// TestSymlinkTargetsPath_DanglingLink still compares lexically so a link into
// a deleted repo file is recognised as ours (and therefore removable).
func TestSymlinkTargetsPath_DanglingLink(t *testing.T) {
	tmp := t.TempDir()
	missingRepoFile := filepath.Join(tmp, "repo", "gone")
	require.NoError(t, os.MkdirAll(filepath.Dir(missingRepoFile), 0755))

	link := filepath.Join(tmp, ".gone")
	require.NoError(t, os.Symlink(missingRepoFile, link))

	assert.True(t, symlinkTargetsPath(link, missingRepoFile))
}

// TestSymlinkTargetsPath_SymlinkedHomeWithMissingRepoFile pins the fix for an
// asymmetric comparison.
//
// The link side always had its parent resolved, but a wantPath that does not
// exist fell back to a purely lexical clean. With a symlinked $HOME
// (/home/u → /export/home/u, common on NFS/SSO setups) the two sides could
// never compare equal, so package discovery reported a correctly-linked file
// as foreign and told the user to adopt their own file.
func TestSymlinkTargetsPath_SymlinkedHomeWithMissingRepoFile(t *testing.T) {
	tmp := t.TempDir()
	realHome := filepath.Join(tmp, "export", "home", "u")
	require.NoError(t, os.MkdirAll(realHome, 0755))

	// /home/u is a symlink to the real location.
	aliasParent := filepath.Join(tmp, "home")
	require.NoError(t, os.MkdirAll(aliasParent, 0755))
	aliasHome := filepath.Join(aliasParent, "u")
	require.NoError(t, os.Symlink(realHome, aliasHome))

	// A repo path that does not exist yet, named through the alias.
	repoFile := filepath.Join(aliasHome, ".dotcor", "pkg", "x")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoFile), 0755))

	// A link created against the resolved form, as the link phase would.
	link := filepath.Join(realHome, ".x")
	require.NoError(t, os.Symlink(filepath.Join(realHome, ".dotcor", "pkg", "x"), link))

	assert.True(t, symlinkTargetsPath(link, repoFile),
		"a link into the repo must be recognised even when the repo file is missing and $HOME is a symlink")
}
