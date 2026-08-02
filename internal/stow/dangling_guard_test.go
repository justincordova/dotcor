package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkWithBackup_RefusesToLinkAtMissingRepoFile pins the fix for a
// dangling symlink in $HOME.
//
// Not every conflict comes from the repo walk: linkAutoDetectedFile records
// one precisely when it FAILED to create the repo copy (read-only mount,
// ENOSPC, a root-owned package dir). Resolving that conflict then replaced
// the user's real file with a symlink to a path that does not exist —
// creating a symlink needs no repo write, so it succeeded — and reported
// success while the only remaining copy sat in the backup tree.
func TestLinkWithBackup_RefusesToLinkAtMissingRepoFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}

	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	backupDir := filepath.Join(tmp, "backups")

	// A package with a managed root under .config/app, so the auto-detect
	// pass walks $HOME/.config/app looking for untracked files.
	pkgSub := filepath.Join(repoDir, "cfg", ".config", "app")
	require.NoError(t, os.MkdirAll(pkgSub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgSub, "tracked.conf"), []byte("tracked"), 0644))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	_, err := Link(repoDir, homeDir, "cfg", nil)
	require.NoError(t, err)

	// An untracked $HOME file the auto-detect pass will try to copy in.
	untracked := filepath.Join(homeDir, ".config", "app", "untracked.conf")
	require.NoError(t, os.WriteFile(untracked, []byte("the only copy"), 0644))

	// Make the repo package subdirectory unwritable so the copy fails.
	require.NoError(t, os.Chmod(pkgSub, 0500))
	t.Cleanup(func() { _ = os.Chmod(pkgSub, 0755) })

	result, err := LinkWithBackup(repoDir, homeDir, "cfg", backupDir, nil)
	require.NoError(t, err)

	repoCopy := filepath.Join(pkgSub, "untracked.conf")
	if _, statErr := os.Lstat(repoCopy); statErr == nil {
		t.Skip("filesystem allowed the write; the failure path was not exercised")
	}

	assert.Contains(t, result.Conflicts, filepath.Join(".config", "app", "untracked.conf"),
		"a conflict with no repo copy must stay unresolved")

	// The user's file must be untouched, not a dangling symlink.
	info, statErr := os.Lstat(untracked)
	require.NoError(t, statErr, "the original file must still exist")
	assert.Zero(t, info.Mode()&os.ModeSymlink, "the original must not have become a symlink")

	data, readErr := os.ReadFile(untracked)
	require.NoError(t, readErr, "the original content must still be readable in place")
	assert.Equal(t, "the only copy", string(data))
}

// TestLinkWithBackup_StillResolvesWhenRepoFileExists guards against the new
// check blocking the normal path.
func TestLinkWithBackup_StillResolvesWhenRepoFileExists(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	backupDir := filepath.Join(tmp, "backups")
	pkgDir := filepath.Join(repoDir, "cfg")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".bashrc"), []byte("repo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("home"), 0644))

	result, err := LinkWithBackup(repoDir, homeDir, "cfg", backupDir, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Resolved)
	assert.Empty(t, result.Conflicts)

	link := filepath.Join(homeDir, ".bashrc")
	info, statErr := os.Lstat(link)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the file should now be a symlink into the repo")

	_, evalErr := filepath.EvalSymlinks(link)
	require.NoError(t, evalErr, "the created symlink must not dangle")
}
