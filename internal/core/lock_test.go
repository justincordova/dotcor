package core

import (
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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	// Arrange
	cfg, err := config.NewDefaultConfig()
	require.NoError(t, err, "NewDefaultConfig() should not error")

	// Act
	// (No action - testing config values)

	// Assert
	assert.GreaterOrEqual(t, cfg.LockTimeout, time.Second, "LockTimeout should be at least 1 second")
	assert.LessOrEqual(t, cfg.LockTimeout, time.Hour, "LockTimeout should not exceed 1 hour")
	assert.Equal(t, 5*time.Minute, cfg.LockTimeout, "Default LockTimeout should be 5 minutes")
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
	cfg.RepoPath = filepath.Join(tmpDir, "files")
	cfg.LockTimeout = 5 * time.Minute

	// Set HOME to temp dir so lock is created there
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

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
	cfg2.RepoPath = filepath.Join(tmpDir, "files")
	cfg2.LockTimeout = 5 * time.Minute
	err = AcquireLock(cfg2)
	if err == nil {
		t.Error("Should fail to acquire already held lock")
	}

	// Verify error message mentions retry attempts
	if err != nil && !strings.Contains(err.Error(), "attempts") {
		t.Error("Error should mention retry attempts")
	}
}
