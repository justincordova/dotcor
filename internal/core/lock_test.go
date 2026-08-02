package core

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

func TestLockInfo(t *testing.T) {
	// Arrange
	info := LockInfo{
		PID:       12345,
		Timestamp: time.Now(),
		Hostname:  "testhost",
	}

	// Act
	// (No action - testing struct initialization)

	// Assert
	assert.Equal(t, 12345, info.PID, "LockInfo.PID should match")
	assert.Equal(t, "testhost", info.Hostname, "LockInfo.Hostname should match")
}

func TestReadLockInfo(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	lockContent := "12345\n2024-01-15T10:30:00Z\ntesthost\n"
	lockFile := filepath.Join(tempDir, ".lock")
	err = os.WriteFile(lockFile, []byte(lockContent), 0644)
	require.NoError(t, err, "failed to create lock file")

	// Act
	info, err := ReadLockInfo(lockFile)

	// Assert
	require.NoError(t, err, "ReadLockInfo() should not error")
	assert.Equal(t, 12345, info.PID, "ReadLockInfo() PID should match")
	assert.Equal(t, "testhost", info.Hostname, "ReadLockInfo() Hostname should match")
}

func TestReadLockInfoMalformed(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "too few lines",
			content: "12345\n",
		},
		{
			name:    "invalid PID",
			content: "not-a-number\n2024-01-15T10:30:00Z\ntesthost\n",
		},
		{
			name:    "invalid timestamp",
			content: "12345\nnot-a-timestamp\ntesthost\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			lockFile := filepath.Join(tempDir, tt.name+".lock")
			err := os.WriteFile(lockFile, []byte(tt.content), 0644)
			require.NoError(t, err, "failed to create lock file")

			// Act
			_, err = ReadLockInfo(lockFile)

			// Assert
			assert.Error(t, err, "ReadLockInfo() should error for malformed content")
		})
	}
}

func TestIsStale(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	oldLockContent := "99999\n" + oldTime + "\ntesthost\n"
	oldLockFile := filepath.Join(tempDir, "old.lock")
	err = os.WriteFile(oldLockFile, []byte(oldLockContent), 0644)
	require.NoError(t, err, "failed to create old lock file")

	cfg := &config.Config{Logger: slog.Default()}

	// Act
	stale, err := IsStale(oldLockFile, cfg)

	// Assert
	require.NoError(t, err, "IsStale() should not error")
	assert.True(t, stale, "IsStale() should return true for old lock")
}

func TestIsLocked(t *testing.T) {
	// Check initial state (should not be locked if tests run in isolation)
	// locked, err := IsLocked()
	// if err != nil {
	// 	t.Fatalf("IsLocked() error = %v", err)
	// }
	// _ = locked
	t.Skip("IsLocked function not implemented")
}

func TestWithLock(t *testing.T) {
	// Test that WithLock executes the function
	// executed := false
	// err := WithLock(func() error {
	// 	executed = true
	// 	return nil
	// })
	// if err == nil && !executed {
	// 	t.Error("WithLock() function not executed")
	// }
	t.Skip("WithLock function not implemented")
}

func TestIsOwnLock(t *testing.T) {
	// Arrange
	// (No arrangement - IsOwnLock has no parameters)

	// Act
	isOwn, err := IsOwnLock()

	// Assert
	require.NoError(t, err, "IsOwnLock() should not error")
	_ = isOwn // The result depends on lock state
}

func TestGetLockInfo(t *testing.T) {
	// Arrange
	// (No arrangement - GetLockInfo has no parameters)

	// Act
	info, err := GetLockInfo()

	// Assert
	require.NoError(t, err, "GetLockInfo() should not error")
	_ = info // info may be nil (no lock) or non-nil (lock exists)
}

func TestLockTimeout(t *testing.T) {
	_, err := config.NewDefaultConfig()
	require.NoError(t, err, "NewDefaultConfig() should not error")
}

func TestErrLockHeld(t *testing.T) {
	// Arrange
	// (No arrangement - testing error constants)

	// Act
	// (No action - testing constants)

	// Assert
	assert.NotNil(t, ErrLockHeld, "ErrLockHeld should not be nil")
	assert.NotNil(t, ErrStaleLock, "ErrStaleLock should not be nil")
}

func TestLockAcquireWithRetry(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, err := config.NewDefaultConfig()
	require.NoError(t, err, "NewDefaultConfig() should not error")
	cfg.Logger = slog.Default()

	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", tmpDir), "failed to set HOME")
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome), "failed to restore HOME")
	}()

	// Test that lock acquisition has bounded retries
	err = AcquireLock(cfg)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	defer func() {
		if err := ReleaseLock(cfg); err != nil {
			t.Logf("Failed to release lock: %v", err)
		}
	}()

	// Try to acquire same lock - should fail after retries
	cfg2, err := config.NewDefaultConfig()
	require.NoError(t, err, "NewDefaultConfig() should not error")
	cfg2.Logger = slog.Default()
	err = AcquireLock(cfg2)
	if err == nil {
		t.Error("Should fail to acquire already held lock")
	}

	// Verify error message mentions retry attempts
	if err != nil && !strings.Contains(err.Error(), "attempts") {
		t.Error("Error should mention retry attempts")
	}
}

// TestIsStale_ForeignHostJudgedByAgeOnly pins the fix for a lock on a shared
// or synced $HOME. The hostname was recorded but never compared, so a lock
// written by PID 4321 on another machine was judged by whether PID 4321
// happens to exist locally — an unrelated process.
func TestIsStale_ForeignHostJudgedByAgeOnly(t *testing.T) {
	cfg := testConfig()
	lockPath := filepath.Join(t.TempDir(), ".lock")

	// A fresh lock from another host, claiming this process's own PID so the
	// local liveness probe would definitely say "alive".
	content := fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), time.Now().Format(time.RFC3339), "some-other-host")
	require.NoError(t, os.WriteFile(lockPath, []byte(content), 0644))

	stale, err := IsStale(lockPath, cfg)
	require.NoError(t, err)
	assert.False(t, stale, "a fresh foreign-host lock must be respected")

	// A dead PID from another host must still be respected until it ages out,
	// because the local process table says nothing about that host.
	content = fmt.Sprintf("%d\n%s\n%s\n", 999999, time.Now().Format(time.RFC3339), "some-other-host")
	require.NoError(t, os.WriteFile(lockPath, []byte(content), 0644))

	stale, err = IsStale(lockPath, cfg)
	require.NoError(t, err)
	assert.False(t, stale, "a foreign-host lock must not be judged by the local process table")

	// Once it ages past the timeout it becomes stale regardless of host.
	old := time.Now().Add(-10 * time.Minute).Format(time.RFC3339)
	content = fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), old, "some-other-host")
	require.NoError(t, os.WriteFile(lockPath, []byte(content), 0644))

	stale, err = IsStale(lockPath, cfg)
	require.NoError(t, err)
	assert.True(t, stale, "an aged-out foreign lock is stale")
}

// TestIsStale_LocalHostStillUsesProcessTable is the guard against the
// hostname check disabling stale detection on the normal single-machine path.
func TestIsStale_LocalHostStillUsesProcessTable(t *testing.T) {
	cfg := testConfig()
	lockPath := filepath.Join(t.TempDir(), ".lock")

	content := fmt.Sprintf("%d\n%s\n%s\n", 999999, time.Now().Format(time.RFC3339), localHostname())
	require.NoError(t, os.WriteFile(lockPath, []byte(content), 0644))

	stale, err := IsStale(lockPath, cfg)
	require.NoError(t, err)
	assert.True(t, stale, "a dead PID on this host is stale")
}

// TestReclaimStaleLock_RefusesToDeleteALiveLock pins the fix for a TOCTOU
// that could delete another process's live lock. Two processes both observe
// the same stale lock; the first reclaims it and creates its own, and the
// second must not then destroy that live lock.
func TestReclaimStaleLock_RefusesToDeleteALiveLock(t *testing.T) {
	cfg := testConfig()
	lockPath := filepath.Join(t.TempDir(), ".lock")

	// Simulate the interleaving: by the time we reclaim, the file at
	// lockPath is a fresh, live lock owned by the process that won.
	live := fmt.Sprintf("%d\n%s\n%s\n", os.Getpid(), time.Now().Format(time.RFC3339), localHostname())
	require.NoError(t, os.WriteFile(lockPath, []byte(live), 0644))

	err := reclaimStaleLock(lockPath, cfg)

	require.ErrorIs(t, err, ErrLockHeld, "reclaim must refuse when the file is a live lock")
	data, readErr := os.ReadFile(lockPath)
	require.NoError(t, readErr, "the live lock must be put back, not left moved aside")
	assert.Equal(t, live, string(data), "the live lock's contents must be intact")
}

// TestReclaimStaleLock_RemovesGenuinelyStaleLock keeps the happy path working.
func TestReclaimStaleLock_RemovesGenuinelyStaleLock(t *testing.T) {
	cfg := testConfig()
	lockPath := filepath.Join(t.TempDir(), ".lock")

	dead := fmt.Sprintf("%d\n%s\n%s\n", 999999, time.Now().Format(time.RFC3339), localHostname())
	require.NoError(t, os.WriteFile(lockPath, []byte(dead), 0644))

	require.NoError(t, reclaimStaleLock(lockPath, cfg))

	_, err := os.Stat(lockPath)
	assert.True(t, os.IsNotExist(err), "a genuinely stale lock must be removed")

	matches, _ := filepath.Glob(lockPath + ".stale.*")
	assert.Empty(t, matches, "no staged file may be left behind")
}
