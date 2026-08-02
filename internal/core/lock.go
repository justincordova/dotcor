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

		err = createLockFile(lockPath)
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
					if reclaimErr := reclaimStaleLock(lockPath, cfg); reclaimErr != nil {
						info, _ := ReadLockInfo(lockPath)
						if errors.Is(reclaimErr, ErrLockHeld) {
							cfg.Logger.Debug("stale lock was reclaimed by another process", "pid", info.PID)
							lastErr = fmt.Errorf("%w: PID %d on %s", ErrLockHeld, info.PID, info.Hostname)
						} else {
							cfg.Logger.Error("failed to remove stale lock", "pid", info.PID, "error", reclaimErr)
							lastErr = fmt.Errorf("stale lock but cannot remove: PID %d", info.PID)
						}
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

		cfg.Logger.Info("lock acquired")
		return nil
	}
	cfg.Logger.Error("failed to acquire lock after retries", "attempts", maxRetries, "last_error", lastErr)
	return fmt.Errorf("failed to acquire lock after %d attempts: %w", maxRetries, lastErr)
}

// createLockFile publishes a fully-populated lock file at lockPath, or
// returns an os.IsExist error if one is already there.
//
// The content is written to a private temporary file first and only then
// linked into place. An O_EXCL create followed by a separate write leaves the
// lock file on disk with zero bytes for the duration of the write — and a
// concurrent acquirer that reads it in that window sees a malformed lock,
// concludes it is stale, and deletes it. Both processes then hold the lock.
//
// os.Link is the right primitive: it is atomic and, unlike os.Rename, it
// fails rather than clobbering an existing destination.
func createLockFile(lockPath string) error {
	content := fmt.Sprintf("%d\n%s\n%s\n",
		os.Getpid(),
		time.Now().Format(time.RFC3339),
		localHostname(),
	)

	tmp, err := os.CreateTemp(filepath.Dir(lockPath), ".lock.tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp lock file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing lock content: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp lock file: %w", err)
	}

	// Fails with EEXIST if another process already published a lock.
	return os.Link(tmpPath, lockPath)
}

// reclaimStaleLock atomically takes ownership of a lock believed to be stale.
//
// Removing the stale file directly is a TOCTOU. Two processes can both
// observe the same stale lock; the first removes it and immediately creates
// its own, and the second's Remove then deletes that live lock — leaving both
// convinced they hold it and mutating $HOME concurrently, which is the exact
// situation the lock exists to prevent.
//
// Renaming instead moves the file aside in a single syscall, under a name
// only this process can produce, so the winner is unambiguous. If what we
// moved turns out to be a live lock (the other process recreated it in
// between), we put it straight back and report the lock as held.
func reclaimStaleLock(lockPath string, cfg *config.Config) error {
	// A PID alone is not unique on a shared ~/.dotcor, which the hostname
	// handling in IsStale explicitly supports. CreateTemp gives a name no
	// other process can collide with.
	staged, err := os.CreateTemp(filepath.Dir(lockPath), ".lock.stale.*")
	if err != nil {
		return fmt.Errorf("staging stale lock: %w", err)
	}
	stagedPath := staged.Name()
	if err := staged.Close(); err != nil {
		_ = os.Remove(stagedPath)
		return fmt.Errorf("staging stale lock: %w", err)
	}

	// Always clean up the staging path, on every exit. Leaving it behind
	// would strand the lock contents in a file nothing ever looks at.
	defer func() { _ = os.Remove(stagedPath) }()

	if err := os.Rename(lockPath, stagedPath); err != nil {
		return err
	}

	stale, err := IsStale(stagedPath, cfg)
	if err != nil || !stale {
		// What we moved aside is a live lock: another process reclaimed and
		// republished between our staleness check and this rename. Put it
		// back with os.Link, which fails rather than clobbering — a third
		// process may already have acquired the lock in the meantime, and
		// overwriting that would hand the lock to two owners at once.
		if linkErr := os.Link(stagedPath, lockPath); linkErr != nil && !os.IsExist(linkErr) {
			cfg.Logger.Error("failed to restore live lock after reclaim attempt",
				"error", linkErr, "staged", stagedPath)
		}
		return ErrLockHeld
	}

	return nil
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

	// Only remove if we own it — same PID AND same host. On a shared or
	// synced ~/.dotcor, a PID on its own is not an identity: host B whose
	// own PID happens to match would otherwise delete host A's live lock.
	if info.PID != os.Getpid() || (info.Hostname != "" && info.Hostname != localHostname()) {
		cfg.Logger.Error("cannot release lock owned by another process",
			"pid", info.PID, "hostname", info.Hostname)
		return fmt.Errorf("cannot release lock owned by PID %d on %s", info.PID, info.Hostname)
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

// staleTimeout bounds how long a lock we cannot evaluate is respected.
//
// It is a fallback for the cases where liveness is genuinely unknowable — a
// lock written on another host, or one we cannot parse. It must NOT be
// applied to a lock whose owner we can see is alive: the timestamp is written
// once at acquisition and never refreshed, while main.go holds the lock for
// the whole TUI session. Checking age first meant leaving the TUI open for
// five minutes was enough for a second instance to declare the lock stale,
// delete it, and run concurrently.
const staleTimeout = 5 * time.Minute

// IsStale reports whether a lock file may be reclaimed.
func IsStale(lockPath string, cfg *config.Config) (bool, error) {
	info, err := ReadLockInfo(lockPath)
	if err != nil {
		// An unparsable lock is usually a truncated leftover, but it is also
		// what a lock being written by another process looks like for an
		// instant. Judge it by the file's age rather than reclaiming
		// immediately, so a live acquisition in progress is never destroyed.
		cfg.Logger.Debug("malformed lock file", "error", err)
		fi, statErr := os.Lstat(lockPath)
		if statErr != nil {
			return true, nil
		}
		return time.Since(fi.ModTime()) > staleTimeout, nil
	}

	// A lock written on a different host says nothing about the local
	// process table — PID 4321 on "laptop" is an unrelated process here.
	// This is reachable whenever ~/.dotcor lives on a shared or synced
	// filesystem (NFS, SMB, a synced folder, or DOTCOR_DIR pointed at one).
	// Liveness is unknowable, so fall back to age.
	if info.Hostname != "" && info.Hostname != localHostname() {
		age := time.Since(info.Timestamp)
		cfg.Logger.Debug("lock owned by another host, judging by age only",
			"pid", info.PID, "hostname", info.Hostname, "age", age)
		return age > staleTimeout, nil
	}

	// Same host: the process table is authoritative, so use it and nothing
	// else. A live owner keeps its lock however long it has held it.
	alive, err := isProcessAlive(info.PID)
	if err != nil {
		cfg.Logger.Debug("cannot check process status, judging by age", "pid", info.PID, "error", err)
		return time.Since(info.Timestamp) > staleTimeout, nil
	}

	if !alive {
		cfg.Logger.Debug("lock is stale, process not alive", "pid", info.PID)
	}

	return !alive, nil
}

// localHostname returns this machine's hostname, or "unknown" when it can't
// be determined. Both the writer and the reader of a lock file must agree on
// this value, so it lives in one place.
func localHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
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
