package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLinkWithBackup_ResolvedCountsOnlyConflicts pins the distinction the
// Resolved field exists for. Linked also counts files the preceding Link pass
// linked cleanly, so reporting it as "resolved N conflicts" overstated the
// work done.
func TestLinkWithBackup_ResolvedCountsOnlyConflicts(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	backupDir := filepath.Join(tmp, "backups")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(pkgDir, 0755))

	// One file that will link cleanly, one that conflicts with a real file.
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo zshrc"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zprofile"), []byte("repo zprofile"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".zprofile"), []byte("home zprofile"), 0644))

	result, err := LinkWithBackup(repoDir, homeDir, "zsh", backupDir, nil)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Resolved, "only the conflicting file was resolved")
	assert.Equal(t, 2, result.Linked, "both files ended up linked")
	assert.Empty(t, result.Conflicts, "the conflict was resolved")

	// The original content must be recoverable from the backup tree.
	var backedUp bool
	require.NoError(t, filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if data, rerr := os.ReadFile(path); rerr == nil && string(data) == "home zprofile" {
			backedUp = true
		}
		return nil
	}))
	assert.True(t, backedUp, "the original $HOME file must be preserved in the backup dir")
}

// TestResolveOneConflict_RestoreFailurePathIsHomeRelative pins the fix for a
// meaningless path in the most dangerous report this package produces. The
// path used to be computed relative to the backup directory, yielding a
// string like "../../../../.config/nvim/init.lua" for a file under $HOME.
//
// The failure is forced deterministically: the target's parent directory does
// not exist, so the symlink swap fails, the target is absent, and the restore
// write fails too — which is exactly the RestoreFailures branch.
func TestResolveOneConflict_RestoreFailurePathIsHomeRelative(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	relPath := filepath.Join(".config", "nvim", "init.lua")
	targetPath := filepath.Join(homeDir, relPath) // parent deliberately absent
	backupPath := filepath.Join(tmp, "backups", "2024-01-01_00-00-00", relPath)

	result := &LinkResult{}
	err := resolveOneConflict(result, homeDir, targetPath, backupPath, "../repo/init.lua", []byte("original"), 0644)

	require.Error(t, err, "the swap must fail when the target directory is absent")
	require.Len(t, result.RestoreFailures, 1, "an unrecoverable restore must be reported")

	report := result.RestoreFailures[0]
	assert.Contains(t, report, relPath, "the report must name the file relative to $HOME")
	assert.NotContains(t, report, "../", "the reported path must not be a relative escape")
	assert.Contains(t, report, backupPath, "the report must point at the recovery copy")

	// The backup is the user's only copy and must survive the failure.
	data, readErr := os.ReadFile(backupPath)
	require.NoError(t, readErr, "the backup must be preserved")
	assert.Equal(t, "original", string(data))
}
