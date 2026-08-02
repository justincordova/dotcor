package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkWithBackup_BackupDirsArePrivate pins the fix for an information
// leak. The backup tree mirrors $HOME, so backing up ~/.ssh/config creates a
// .ssh directory inside it. At 0755 that directory is world-traversable: the
// copied file keeps its own 0600, but the filenames under ~/.ssh, ~/.gnupg
// and ~/.aws still leak on a shared host.
func TestLinkWithBackup_BackupDirsArePrivate(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	backupDir := filepath.Join(tmp, "backups")
	pkgDir := filepath.Join(repoDir, "ssh")

	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".ssh"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".ssh", "config"), []byte("repo"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".ssh", "config"), []byte("original"), 0600))

	result, err := LinkWithBackup(repoDir, homeDir, "ssh", backupDir, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.Resolved, "the conflict must have been backed up")

	var checked int
	require.NoError(t, filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return err
		}
		checked++
		assert.Zero(t, info.Mode().Perm()&0o077,
			"backup directory %s must not be group- or world-accessible (mode %v)", path, info.Mode().Perm())
		return nil
	}))
	assert.Positive(t, checked, "the walk must have inspected some directories")
}
