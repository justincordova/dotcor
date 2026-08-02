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
