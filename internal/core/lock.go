package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

// LockInfo contains information about the current lock
type LockInfo struct {
	PID       int
	Timestamp time.Time
	Hostname  string
}

// LockTimeout is the duration after which a lock is considered stale
const LockTimeout = time.Hour

// maxRetries is the maximum number of lock acquisition attempts
const maxRetries = 3

// ErrLockHeld is returned when lock is already held by another process
var ErrLockHeld = errors.New("lock is held by another process")

// ErrStaleLock is returned when lock appears to be stale
var ErrStaleLock = errors.New("stale lock detected")

// getLockPath returns the path to the lock file
func getLockPath() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ".lock"), nil
}

// AcquireLock acquires file-based lock for dotcor operations
// Uses O_EXCL for atomic lock creation to prevent race conditions
// Returns error if lock is already held
func AcquireLock() error {
	for i := 0; i < maxRetries; i++ {
		lockPath, err := getLockPath()
		if err != nil {
			return err
		}

		// Ensure config directory exists
		if err := fs.EnsureDir(filepath.Dir(lockPath)); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}

		// Try atomic lock creation with O_EXCL
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsExist(err) {
				// Lock file exists, check if stale
				stale, staleErr := IsStale(lockPath)
				if staleErr != nil {
					return fmt.Errorf("checking stale lock: %w", staleErr)
				}

				if stale {
					// Try to remove stale lock and retry
					if removeErr := os.Remove(lockPath); removeErr != nil {
						info, _ := ReadLockInfo(lockPath)
						return fmt.Errorf("%w: PID %d (process appears dead). Run 'dotcor doctor --fix' to clear", ErrStaleLock, info.PID)
					}
					// Retry lock acquisition after removing stale lock
					continue
				}

				// Lock is held by active process
				info, _ := ReadLockInfo(lockPath)
				return fmt.Errorf("%w: PID %d on %s. If this is incorrect, run 'dotcor doctor --fix'", ErrLockHeld, info.PID, info.Hostname)
			}
			return fmt.Errorf("creating lock file: %w", err)
		}
		defer f.Close()

		// Write lock content
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}

		content := fmt.Sprintf("%d\n%s\n%s\n",
			os.Getpid(),
			time.Now().Format(time.RFC3339),
			hostname,
		)

		if _, err := f.WriteString(content); err != nil {
			// Clean up on write failure
			os.Remove(lockPath)
			return fmt.Errorf("writing lock file: %w", err)
		}

		return nil
	}
	return fmt.Errorf("failed to acquire lock after %d attempts", maxRetries)
}

// ReleaseLock releases the file lock
func ReleaseLock() error {
	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	// Check if we own the lock
	if !fs.FileExists(lockPath) {
		return nil // No lock to release
	}

	info, err := ReadLockInfo(lockPath)
	if err != nil {
		// Can't read lock, try to remove anyway
		return os.Remove(lockPath)
	}

	// Only remove if we own it
	if info.PID != os.Getpid() {
		return fmt.Errorf("cannot release lock owned by PID %d", info.PID)
	}

	return os.Remove(lockPath)
}

// WithLock executes a function while holding the lock
// Automatically releases lock on completion or panic
func WithLock(fn func() error) error {
	if err := AcquireLock(); err != nil {
		return err
	}

	defer func() {
		ReleaseLock()
		if r := recover(); r != nil {
			panic(r) // Re-panic after releasing lock
		}
	}()

	return fn()
}

// IsLocked checks if lock is currently held
func IsLocked() (bool, error) {
	lockPath, err := getLockPath()
	if err != nil {
		return false, err
	}

	return fs.FileExists(lockPath), nil
}

// IsStale checks if lock file is stale (process dead)
func IsStale(lockPath string) (bool, error) {
	info, err := ReadLockInfo(lockPath)
	if err != nil {
		return true, nil // Malformed lock file is considered stale
	}

	// Check if lock is older than LockTimeout
	if time.Since(info.Timestamp) > LockTimeout {
		return true, nil
	}

	// Check if process is alive
	alive, err := isProcessAlive(info.PID)
	if err != nil {
		return true, nil // Can't check, assume stale
	}

	return !alive, nil
}

// ClearStaleLock removes stale lock file
func ClearStaleLock() error {
	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	if !fs.FileExists(lockPath) {
		return nil // No lock to clear
	}

	// Read lock info for error messages
	info, infoErr := ReadLockInfo(lockPath)

	// Verify it's actually stale
	stale, err := IsStale(lockPath)
	if err != nil {
		return fmt.Errorf("checking if stale: %w", err)
	}

	if !stale {
		if infoErr == nil {
			return fmt.Errorf("lock is not stale (process %d is still running)", info.PID)
		}
		return fmt.Errorf("lock is not stale")
	}

	return os.Remove(lockPath)
}

// ReadLockInfo reads lock information from lock file
func ReadLockInfo(lockPath string) (LockInfo, error) {
	content, err := os.ReadFile(lockPath)
	if err != nil {
		return LockInfo{}, fmt.Errorf("reading lock file: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 3 {
		return LockInfo{}, fmt.Errorf("malformed lock file: expected 3 lines, got %d", len(lines))
	}

	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil {
		return LockInfo{}, fmt.Errorf("invalid PID in lock file: %w", err)
	}

	timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(lines[1]))
	if err != nil {
		return LockInfo{}, fmt.Errorf("invalid timestamp in lock file: %w", err)
	}

	hostname := strings.TrimSpace(lines[2])

	return LockInfo{
		PID:       pid,
		Timestamp: timestamp,
		Hostname:  hostname,
	}, nil
}

// isProcessAlive checks if a process with given PID is still running
func isProcessAlive(pid int) (bool, error) {
	if runtime.GOOS == "windows" {
		return isProcessAliveWindows(pid)
	}
	return isProcessAliveUnix(pid)
}

// isProcessAliveUnix checks if process is alive on Unix systems
func isProcessAliveUnix(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil // Process doesn't exist
	}

	// On Unix, signal 0 checks if process exists without killing it
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we don't have permission
		return false, nil
	}

	return true, nil
}

// isProcessAliveWindows checks if process is alive on Windows
// Note: On Windows, os.FindProcess always succeeds, so we check
// if we can signal the process. This is imperfect but works for
// most cases where the PID has been reused or the process is gone.
func isProcessAliveWindows(pid int) (bool, error) {
	// On Windows, we try to find and signal the process
	// FindProcess on Windows doesn't actually verify the process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	// On Windows, Signal(0) returns an error if process doesn't exist
	// or we don't have permission. We treat both as "not alive" for
	// lock staleness purposes since either way we can't communicate with it.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, nil
	}

	return true, nil
}

// GetLockInfo returns information about the current lock, if any
func GetLockInfo() (*LockInfo, error) {
	lockPath, err := getLockPath()
	if err != nil {
		return nil, err
	}

	if !fs.FileExists(lockPath) {
		return nil, nil // No lock
	}

	info, err := ReadLockInfo(lockPath)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// ForceReleaseLock forcibly removes the lock file regardless of owner
// Use with caution - only when you're sure the lock is stale
func ForceReleaseLock() error {
	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	if !fs.FileExists(lockPath) {
		return nil
	}

	return os.Remove(lockPath)
}

// IsOwnLock checks if current process owns the lock
func IsOwnLock() (bool, error) {
	info, err := GetLockInfo()
	if err != nil {
		return false, err
	}

	if info == nil {
		return false, nil // No lock exists
	}

	return info.PID == os.Getpid(), nil
}
