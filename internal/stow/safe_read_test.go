package stow

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSafeReadFile_NamedPipeDoesNotBlock pins the fix for a TUI freeze.
// A FIFO reports Size() == 0 so the size guard never fires, and os.Open on a
// FIFO blocks forever waiting for a writer. Without the regular-file check
// this test hangs until the go test timeout rather than failing.
func TestSafeReadFile_NamedPipeDoesNotBlock(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "agent.sock")
	require.NoError(t, syscall.Mkfifo(fifo, 0600))

	done := make(chan error, 1)
	go func() {
		_, _, err := safeReadFile(fifo)
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err, "a FIFO must be refused, not read")
		assert.Contains(t, err.Error(), "not a regular file")
	case <-time.After(3 * time.Second):
		t.Fatal("safeReadFile blocked on a named pipe — this freezes the TUI")
	}
}

// TestSafeReadFile_DirectoryRefused guards the other non-regular case that
// reaches this helper via the classify walk.
func TestSafeReadFile_DirectoryRefused(t *testing.T) {
	_, _, err := safeReadFile(t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a regular file")
}

// TestSafeReadFile_RegularFileStillWorks is the guard against over-tightening.
func TestSafeReadFile_RegularFileStillWorks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte("value"), 0640))

	data, perm, err := safeReadFile(path)

	require.NoError(t, err)
	assert.Equal(t, "value", string(data))
	assert.Equal(t, os.FileMode(0640), perm)
}
