package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
func AcquireLock(cfg *config.Config) error {
	cfg.Logger.Debug("acquiring lock")

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		lockPath, err := getLockPath()
		if err != nil {
			return err
		}

		// Ensure config directory exists
		if err := fs.EnsureDir(filepath.Dir(lockPath), cfg); err != nil {
			cfg.Logger.Error("failed to create config directory", "error", err)
			return fmt.Errorf("failed to create lock directory at %s: %w", filepath.Dir(lockPath), err)
		}

		// Try atomic lock creation with O_EXCL
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsExist(err) {
				// Lock file exists, check if stale
				stale, staleErr := IsStale(lockPath, cfg)
				if staleErr != nil {
					cfg.Logger.Error("failed to check stale lock", "error", staleErr)
					lastErr = fmt.Errorf("checking stale lock: %w", staleErr)
					continue
				}

				if stale {
					// Try to remove stale lock
					if removeErr := os.Remove(lockPath); removeErr != nil {
						info, _ := ReadLockInfo(lockPath)
						cfg.Logger.Error("failed to remove stale lock", "pid", info.PID)
						lastErr = fmt.Errorf("stale lock but cannot remove: PID %d", info.PID)
						continue
					}
					// Successfully removed stale lock, retry
					cfg.Logger.Debug("removed stale lock, retrying")
					continue
				}

				// Lock is held by active process
				info, _ := ReadLockInfo(lockPath)
				age := time.Since(info.Timestamp)
				cfg.Logger.Error("lock held by another process", "pid", info.PID, "hostname", info.Hostname, "age", age)
				lastErr = fmt.Errorf("%w: PID %d on %s (lock held for %v). If this is incorrect, run 'dotcor doctor --fix'", ErrLockHeld, info.PID, info.Hostname, formatAge(age))
				continue
			}
			cfg.Logger.Error("failed to create lock file", "error", err)
			return fmt.Errorf("creating lock file: %w", err)
		}

		// Lock acquired successfully
		defer func() {
			if err := f.Close(); err != nil {
				cfg.Logger.Warn("failed to close lock file", "error", err)
			}
		}()

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
			if closeErr := f.Close(); closeErr != nil {
				cfg.Logger.Warn("failed to close lock file", "error", closeErr)
			}
			_ = os.Remove(lockPath)
			cfg.Logger.Error("failed to write lock file", "error", err)
			return fmt.Errorf("writing lock file: %w", err)
		}

		cfg.Logger.Info("lock acquired")
		return nil
	}
	cfg.Logger.Error("failed to acquire lock after retries", "attempts", maxRetries, "last_error", lastErr)
	return fmt.Errorf("failed to acquire lock after %d attempts: %w", maxRetries, lastErr)
}

// ReleaseLock releases the file lock
func ReleaseLock(cfg *config.Config) error {
	cfg.Logger.Debug("releasing lock")

	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	// Check if we own the lock
	if !fs.PathExists(lockPath) {
		cfg.Logger.Debug("no lock to release")
		return nil // No lock to release
	}

	info, err := ReadLockInfo(lockPath)
	if err != nil {
		// Cannot read lock, try to remove anyway
		cfg.Logger.Debug("cannot read lock info, attempting removal")
		return os.Remove(lockPath)
	}

	// Only remove if we own it
	if info.PID != os.Getpid() {
		cfg.Logger.Error("cannot release lock owned by another process", "pid", info.PID)
		return fmt.Errorf("cannot release lock owned by PID %d", info.PID)
	}

	if err := os.Remove(lockPath); err != nil {
		cfg.Logger.Error("failed to remove lock file", "error", err)
		return err
	}

	cfg.Logger.Info("lock released")
	return nil
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

// IsStale checks if lock file is stale (process dead)
func IsStale(lockPath string, cfg *config.Config) (bool, error) {
	info, err := ReadLockInfo(lockPath)
	if err != nil {
		cfg.Logger.Debug("malformed lock file", "error", err)
		return true, nil // Malformed lock file is considered stale
	}

	// Check if lock is older than configured timeout
	if time.Since(info.Timestamp) > 5*time.Minute {
		cfg.Logger.Debug("lock is stale due to age", "pid", info.PID, "age", time.Since(info.Timestamp))
		return true, nil
	}

	// Check if process is alive
	alive, err := isProcessAlive(info.PID)
	if err != nil {
		cfg.Logger.Debug("cannot check process status, assuming stale", "pid", info.PID, "error", err)
		return true, nil // Cannot check, assume stale
	}

	if !alive {
		cfg.Logger.Debug("lock is stale, process not alive", "pid", info.PID)
	}

	return !alive, nil
}

// isProcessAlive checks if a process with given PID is still running
func isProcessAlive(pid int) (bool, error) {
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

// GetLockInfo returns information about the current lock, if any
func GetLockInfo() (*LockInfo, error) {
	lockPath, err := getLockPath()
	if err != nil {
		return nil, err
	}

	if !fs.PathExists(lockPath) {
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
func ForceReleaseLock(cfg *config.Config) error {
	cfg.Logger.Debug("force releasing lock")

	lockPath, err := getLockPath()
	if err != nil {
		return err
	}

	if !fs.PathExists(lockPath) {
		cfg.Logger.Debug("no lock to release")
		return nil
	}

	if err := os.Remove(lockPath); err != nil {
		cfg.Logger.Error("failed to remove lock file", "error", err)
		return err
	}

	cfg.Logger.Info("lock forcibly released")
	return nil
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

// formatAge formats a duration into a human-readable string
func formatAge(d time.Duration) string {
	if d < time.Minute {
		seconds := int(d.Seconds())
		return fmt.Sprintf("%d second%s", seconds, pluralize(seconds))
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%d minute%s", minutes, pluralize(minutes))
	} else {
		hours := d.Hours()
		return fmt.Sprintf("%.1f hour%s", hours, pluralize(int(hours)))
	}
}

// pluralize returns 's' if n != 1
func pluralize(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
