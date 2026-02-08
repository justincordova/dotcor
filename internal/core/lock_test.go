package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
)

func TestLockInfo(t *testing.T) {
	info := LockInfo{
		PID:       12345,
		Timestamp: time.Now(),
		Hostname:  "testhost",
	}

	if info.PID != 12345 {
		t.Errorf("LockInfo.PID = %d, want 12345", info.PID)
	}
	if info.Hostname != "testhost" {
		t.Errorf("LockInfo.Hostname = %s, want testhost", info.Hostname)
	}
}

func TestReadLockInfo(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a valid lock file
	lockContent := "12345\n2024-01-15T10:30:00Z\ntesthost\n"
	lockFile := filepath.Join(tempDir, ".lock")
	if err := os.WriteFile(lockFile, []byte(lockContent), 0644); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	// Read lock info
	info, err := ReadLockInfo(lockFile)
	if err != nil {
		t.Fatalf("ReadLockInfo() error = %v", err)
	}

	if info.PID != 12345 {
		t.Errorf("ReadLockInfo() PID = %d, want 12345", info.PID)
	}
	if info.Hostname != "testhost" {
		t.Errorf("ReadLockInfo() Hostname = %s, want testhost", info.Hostname)
	}
}

func TestReadLockInfoMalformed(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
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
			lockFile := filepath.Join(tempDir, tt.name+".lock")
			if err := os.WriteFile(lockFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create lock file: %v", err)
			}

			_, err := ReadLockInfo(lockFile)
			if err == nil {
				t.Error("ReadLockInfo() should error for malformed content")
			}
		})
	}
}

func TestIsStale(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an old lock file (timestamp > 1 hour ago)
	oldTime := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)
	oldLockContent := "99999\n" + oldTime + "\ntesthost\n"
	oldLockFile := filepath.Join(tempDir, "old.lock")
	if err := os.WriteFile(oldLockFile, []byte(oldLockContent), 0644); err != nil {
		t.Fatalf("failed to create old lock file: %v", err)
	}

	// Check if stale (old lock with non-existent PID should be stale)
	cfg := &config.Config{Logger: slog.Default()}
	stale, err := IsStale(oldLockFile, cfg)
	if err != nil {
		t.Fatalf("IsStale() error = %v", err)
	}
	if !stale {
		t.Error("IsStale() should return true for old lock")
	}
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
	// Without acquiring a lock, IsOwnLock should return false
	isOwn, err := IsOwnLock()
	if err != nil {
		t.Fatalf("IsOwnLock() error = %v", err)
	}

	// The result depends on lock state
	_ = isOwn
}

func TestGetLockInfo(t *testing.T) {
	// GetLockInfo should not error even if no lock exists
	info, err := GetLockInfo()
	if err != nil {
		t.Fatalf("GetLockInfo() error = %v", err)
	}

	// info may be nil (no lock) or non-nil (lock exists)
	_ = info
}

func TestLockTimeout(t *testing.T) {
	// Verify LockTimeout constant is reasonable
	if LockTimeout < time.Second {
		t.Error("LockTimeout is too short")
	}
	if LockTimeout > time.Hour {
		t.Error("LockTimeout is too long")
	}
}

func TestErrLockHeld(t *testing.T) {
	// Verify error constants exist
	if ErrLockHeld == nil {
		t.Error("ErrLockHeld should not be nil")
	}
	if ErrStaleLock == nil {
		t.Error("ErrStaleLock should not be nil")
	}
}
