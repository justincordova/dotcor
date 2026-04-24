package stow

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTxn_RunSuccess_LeavesStepsInExecutedList(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out.txt")
	txn := &fileTxn{}

	// Act
	err := txn.run(stepWriteFile(dst, []byte("hello"), 0644))

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, dst)
	assert.Len(t, txn.steps, 1)
}

func TestFileTxn_RunFailure_RollsBackPriorSteps(t *testing.T) {
	// Two writes: the first succeeds, the second is forced to fail by
	// using an unwritable destination path. The first write must be
	// undone (file removed) by the rollback.
	// Arrange
	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "first.txt")
	// Force step 2 to fail by writing into a path whose parent is a file.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "blocker"), []byte("x"), 0644))
	second := filepath.Join(tmpDir, "blocker", "child.txt")

	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(first, []byte("a"), 0644)))

	// Act
	err := txn.run(stepWriteFile(second, []byte("b"), 0644))

	// Assert: error returned, first file rolled back.
	require.Error(t, err)
	_, statErr := os.Stat(first)
	assert.True(t, os.IsNotExist(statErr), "first file must be removed by rollback")
}

func TestFileTxn_Commit_SkipsRollbackOnLaterCall(t *testing.T) {
	// After commit, manually invoking rollback should be a no-op even
	// though steps were executed.
	tmpDir := t.TempDir()
	dst := filepath.Join(tmpDir, "out.txt")
	txn := &fileTxn{}
	require.NoError(t, txn.run(stepWriteFile(dst, []byte("x"), 0644)))

	txn.commit()
	require.NoError(t, txn.rollback())

	assert.FileExists(t, dst, "committed file must remain on subsequent rollback")
}

func TestFileTxn_RollbackPanic_ReturnsError(t *testing.T) {
	// A step whose undo() panics should not propagate the panic; rollback
	// must convert it to an error.
	txn := &fileTxn{}
	step := fileStep{
		desc: "panicker",
		do:   func() error { return nil },
		undo: func() error { panic("boom") },
	}
	require.NoError(t, txn.run(step))

	// Trigger rollback by running a failing step after the panicker.
	failing := fileStep{
		desc: "failer",
		do:   func() error { return errors.New("nope") },
		undo: func() error { return nil },
	}
	err := txn.run(failing)

	require.Error(t, err)
}

func TestStepReplaceFileWithSymlink_UndoRestoresOriginalBytes(t *testing.T) {
	// Arrange: a regular file with known bytes.
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "regular.txt")
	target := filepath.Join(tmpDir, "real.txt")
	require.NoError(t, os.WriteFile(src, []byte("ORIGINAL"), 0644))
	require.NoError(t, os.WriteFile(target, []byte("target"), 0644))

	step := stepReplaceFileWithSymlink(src, target, []byte("ORIGINAL"), 0644)

	// Act: do then undo.
	require.NoError(t, step.do())
	// Verify it's now a symlink.
	info, err := os.Lstat(src)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)

	require.NoError(t, step.undo())

	// Assert: regular file with original bytes is back.
	info, err = os.Lstat(src)
	require.NoError(t, err)
	assert.Zero(t, info.Mode()&os.ModeSymlink, "src must no longer be a symlink after undo")
	bytes, err := os.ReadFile(src)
	require.NoError(t, err)
	assert.Equal(t, []byte("ORIGINAL"), bytes)
}

func TestStepRepointSymlink_UndoRestoresOriginalTarget(t *testing.T) {
	tmpDir := t.TempDir()
	link := filepath.Join(tmpDir, "link")
	origTarget := filepath.Join(tmpDir, "orig")
	newTarget := filepath.Join(tmpDir, "new")
	require.NoError(t, os.WriteFile(origTarget, []byte("o"), 0644))
	require.NoError(t, os.WriteFile(newTarget, []byte("n"), 0644))
	require.NoError(t, os.Symlink(origTarget, link))

	step := stepRepointSymlink(link, newTarget, origTarget)

	// Act
	require.NoError(t, step.do())
	resolved, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, newTarget, resolved)

	require.NoError(t, step.undo())

	// Assert
	resolved, err = os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, origTarget, resolved, "undo must restore the original symlink target")
}
