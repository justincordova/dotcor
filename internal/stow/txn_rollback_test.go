package stow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStepWriteFile_UndoRestoresPriorContents pins the fix for data loss on
// rollback. The undo used to unconditionally os.Remove(dst), so when dst was
// an already-tracked repo file the rollback destroyed the user's copy — a
// worse outcome than not rolling back at all.
func TestStepWriteFile_UndoRestoresPriorContents(t *testing.T) {
	repoFile := filepath.Join(t.TempDir(), "pkg", ".bashrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(repoFile), 0755))
	require.NoError(t, os.WriteFile(repoFile, []byte("curated original"), 0640))

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(repoFile, []byte("replacement"), 0644)))

	// A later step fails, unwinding the write.
	err := txn.run(fileStep{
		desc: "failing step",
		do:   func() error { return errors.New("boom") },
		undo: func() error { return nil },
	})
	require.Error(t, err)

	data, readErr := os.ReadFile(repoFile)
	require.NoError(t, readErr, "rollback must not delete a pre-existing repo file")
	assert.Equal(t, "curated original", string(data), "rollback must restore the prior contents")

	info, statErr := os.Stat(repoFile)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0640), info.Mode().Perm(), "rollback must restore the prior mode")
}

// TestStepWriteFile_UndoRemovesNewFile keeps the original behaviour for the
// case the undo was actually written for.
func TestStepWriteFile_UndoRemovesNewFile(t *testing.T) {
	repoDir := t.TempDir()
	repoFile := filepath.Join(repoDir, "pkg", ".bashrc")

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(repoFile, []byte("new"), 0644)))
	require.NoError(t, txn.rollback())

	_, err := os.Stat(repoFile)
	assert.True(t, os.IsNotExist(err), "a file the step created must be removed")
}

// TestStepWriteFile_UndoRemovesWholeCreatedChain pins the fix for orphaned
// directories: MkdirAll can create an arbitrarily deep chain but the undo
// only removed one level, leaving an empty package behind in the repo.
func TestStepWriteFile_UndoRemovesWholeCreatedChain(t *testing.T) {
	repoDir := t.TempDir()
	repoFile := filepath.Join(repoDir, "nvim", ".config", "nvim", "lua", "plugins", "init.lua")

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(repoFile, []byte("return {}"), 0644)))
	require.NoError(t, txn.rollback())

	_, err := os.Stat(filepath.Join(repoDir, "nvim"))
	assert.True(t, os.IsNotExist(err), "every directory the step created must be unwound")

	_, err = os.Stat(repoDir)
	assert.NoError(t, err, "pre-existing directories must survive")
}

// TestStepWriteFile_UndoStopsAtNonEmptyDir ensures the chain unwind never
// touches a directory holding somebody else's files.
func TestStepWriteFile_UndoStopsAtNonEmptyDir(t *testing.T) {
	repoDir := t.TempDir()
	pkgDir := filepath.Join(repoDir, "nvim")
	sibling := filepath.Join(pkgDir, "keep.txt")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.WriteFile(sibling, []byte("keep"), 0644))

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(filepath.Join(pkgDir, "sub", "init.lua"), []byte("x"), 0644)))
	require.NoError(t, txn.rollback())

	_, err := os.Stat(sibling)
	assert.NoError(t, err, "a non-empty directory must never be removed")
}

// TestStepWriteFile_RefusesUnreadableDestination pins the fix for a silent
// downgrade to "did not exist". When the prior contents could not be read,
// priorExisted stayed false and the undo then deleted a repo file that
// existed before the transaction started — the exact outcome the step
// promises never to produce.
func TestStepWriteFile_RefusesUnreadableDestination(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}

	dst := filepath.Join(t.TempDir(), "existing.conf")
	require.NoError(t, os.WriteFile(dst, []byte("precious"), 0644))
	require.NoError(t, os.Chmod(dst, 0000))
	t.Cleanup(func() { _ = os.Chmod(dst, 0644) })

	if _, err := os.ReadFile(dst); err == nil {
		t.Skip("filesystem allowed the read; the failure path was not exercised")
	}

	txn := &fileTxn{}
	err := txn.run(stepWriteFile(dst, []byte("replacement"), 0644))

	require.Error(t, err, "a destination we cannot snapshot must not be overwritten")

	require.NoError(t, os.Chmod(dst, 0644))
	data, readErr := os.ReadFile(dst)
	require.NoError(t, readErr)
	assert.Equal(t, "precious", string(data), "the existing file must be left untouched")
}

// TestStepWriteFile_RefusesSymlinkDestination pins the other capture hole.
// os.WriteFile follows a symlink and writes through it, so the undo's
// os.Remove would drop the link and leave the clobbered target behind.
func TestStepWriteFile_RefusesSymlinkDestination(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real.conf")
	link := filepath.Join(tmp, "link.conf")
	require.NoError(t, os.WriteFile(real, []byte("original target"), 0644))
	require.NoError(t, os.Symlink(real, link))

	txn := &fileTxn{}
	err := txn.run(stepWriteFile(link, []byte("replacement"), 0644))

	require.Error(t, err, "a symlink destination must be refused, not written through")
	assert.Contains(t, err.Error(), "non-regular")

	data, readErr := os.ReadFile(real)
	require.NoError(t, readErr)
	assert.Equal(t, "original target", string(data), "the symlink's target must not be clobbered")
}

// TestStepWriteFile_AppliesModeToExistingFile pins the chmod. open(2) ignores
// the mode argument when the file already exists, so re-adding a 0600
// ~/.ssh/config over a 0644 repo copy left the repo copy world-readable.
func TestStepWriteFile_AppliesModeToExistingFile(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0644))

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(dst, []byte("new"), 0600)))
	txn.commit()

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(),
		"the requested mode must be applied even when the destination already existed")
}

// TestStepWriteFile_UndoRestoresPriorMode covers the reverse direction.
func TestStepWriteFile_UndoRestoresPriorMode(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(dst, []byte("old"), 0600))

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(dst, []byte("new"), 0644)))
	require.NoError(t, txn.rollback())

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "rollback must restore the prior mode")
}
