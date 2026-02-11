package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

// BackupInfo represents information about a backup
type BackupInfo struct {
	Timestamp  time.Time
	SourcePath string // Original file path (normalized)
	BackupPath string // Full path to backup file
	Size       int64
}

// TimestampFormat is the format used for backup directory names
// Format: YYYY-MM-DD_HH-MM-SS (sortable, filesystem-safe)
const TimestampFormat = "2006-01-02_15-04-05"

// GetBackupDir returns the backup directory path (~/.dotcor/backups)
func GetBackupDir() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "backups"), nil
}

// CreateBackup creates a timestamped backup of a file before destructive operations
// Returns backup path and error
func CreateBackup(sourcePath string, cfg *config.Config) (string, error) {
	start := time.Now()
	cfg.Logger.Debug("creating backup", "file", sourcePath)

	expanded, err := config.ExpandPath(sourcePath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand path", "file", sourcePath, "error", err)
		return "", fmt.Errorf("expanding source path: %w", err)
	}

	if !fs.PathExists(expanded) {
		cfg.Logger.Error("source file does not exist", "file", sourcePath)
		return "", fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Get backup directory
	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return "", err
	}

	// Create timestamped subdirectory
	timestamp := time.Now().Format(TimestampFormat)
	timestampDir := filepath.Join(backupDir, timestamp)

	// Check if path exists and is a file (not directory)
	if info, err := os.Stat(timestampDir); err == nil && !info.IsDir() {
		return "", fmt.Errorf("backup path exists as file, not directory: %s", timestampDir)
	}

	if err := fs.EnsureDir(timestampDir, cfg); err != nil {
		cfg.Logger.Error("failed to create backup directory", "error", err)
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	// Normalize source path and use as relative path in backup to preserve uniqueness
	normalized, err := config.NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	// Strip leading ~ and convert to relative path for storage
	// e.g., ~/.zshrc -> zshrc, ~/.config/nvim/init.lua -> config/nvim/init.lua
	backupRelativePath := strings.TrimPrefix(normalized, "~/")
	backupPath := filepath.Join(timestampDir, backupRelativePath)

	// Ensure parent directory exists
	if err := fs.EnsureDir(filepath.Dir(backupPath), cfg); err != nil {
		cfg.Logger.Error("failed to create backup subdirectory", "error", err)
		return "", fmt.Errorf("creating backup subdirectory: %w", err)
	}

	if err := fs.CopyWithPermissions(expanded, backupPath, cfg); err != nil {
		cfg.Logger.Error("failed to copy to backup", "src", expanded, "dst", backupPath, "error", err)
		return "", fmt.Errorf("copying to backup: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()
	cfg.Logger.Info("backup created",
		"file", sourcePath,
		"path", backupPath,
		"duration_ms", durationMs,
	)

	return backupPath, nil
}

// RestoreBackup restores a file from backup to target path
func RestoreBackup(backupPath, targetPath string, cfg *config.Config) error {
	start := time.Now()
	cfg.Logger.Debug("restoring from backup",
		"backup", backupPath,
		"target", targetPath,
	)

	expandedBackup, err := config.ExpandPath(backupPath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand backup path", "error", err)
		return fmt.Errorf("expanding backup path: %w", err)
	}

	expandedTarget, err := config.ExpandPath(targetPath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand target path", "error", err)
		return fmt.Errorf("expanding target path: %w", err)
	}

	if !fs.PathExists(expandedBackup) {
		cfg.Logger.Error("backup file does not exist", "path", backupPath)
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	if err := fs.EnsureDir(filepath.Dir(expandedTarget), cfg); err != nil {
		cfg.Logger.Error("failed to create target directory", "error", err)
		return fmt.Errorf("creating target directory: %w", err)
	}

	if err := fs.CopyWithPermissions(expandedBackup, expandedTarget, cfg); err != nil {
		cfg.Logger.Error("failed to restore from backup", "src", expandedBackup, "dst", expandedTarget, "error", err)
		return fmt.Errorf("restoring from backup: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()
	cfg.Logger.Info("backup restored",
		"backup", backupPath,
		"target", targetPath,
		"duration_ms", durationMs,
	)

	return nil
}

// ListBackups returns list of all backups with timestamps
func ListBackups(cfg *config.Config) ([]BackupInfo, error) {
	cfg.Logger.Debug("listing backups")

	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return nil, err
	}

	// Check if backup directory exists
	if !fs.PathExists(backupDir) {
		return []BackupInfo{}, nil
	}

	var backups []BackupInfo

	// Walk through backup directory
	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get the timestamp directory name
		relPath, err := filepath.Rel(backupDir, path)
		if err != nil {
			return nil // Skip files we cannot process
		}

		// Get the parent directory (timestamp directory)
		timestampStr := filepath.Dir(relPath)
		if timestampStr == "." {
			return nil // Skip files directly in backup dir
		}

		// Extract source path (everything after the timestamp directory)
		// e.g., "2024-01-15_10-30-00/config/nvim/init.lua" -> "config/nvim/init.lua"
		parts := strings.SplitN(relPath, string(filepath.Separator), 2)
		if len(parts) != 2 {
			return nil // Skip if path doesn't have timestamp prefix
		}

		// Parse timestamp from the first directory component
		timestamp, err := time.Parse(TimestampFormat, timestampStr)
		if err != nil {
			return nil // Skip if we cannot parse timestamp
		}

		// Reconstruct original source path with ~ prefix
		sourcePath := "~/" + parts[1]

		backups = append(backups, BackupInfo{
			Timestamp:  timestamp,
			SourcePath: sourcePath,
			BackupPath: path,
			Size:       info.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking backup directory: %w", err)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// CleanupCandidate represents a backup directory that can be cleaned up
type CleanupCandidate struct {
	Path      string
	Timestamp time.Time
	Size      int64
}

// PreviewCleanup returns what would be deleted without actually deleting
func PreviewCleanup(olderThan time.Duration, keepLast int, cfg *config.Config) ([]CleanupCandidate, int64, error) {
	cfg.Logger.Debug("previewing backup cleanup", "older_than", olderThan, "keep_last", keepLast)

	candidates, _, err := getCleanupCandidates(olderThan, keepLast, cfg)
	if err != nil {
		cfg.Logger.Error("failed to get cleanup candidates", "error", err)
		return nil, 0, err
	}

	var totalSize int64
	for _, c := range candidates {
		totalSize += c.Size
	}

	return candidates, totalSize, nil
}

// CleanOldBackups removes backups older than specified duration, keeping at least keepLast.
// Returns: number deleted, number of errors, total freed size, first error encountered.
// Continues deleting even if some deletions fail.
func CleanOldBackups(olderThan time.Duration, keepLast int, cfg *config.Config) (deleted int, failed int, freedSize int64, err error) {
	cfg.Logger.Debug("cleaning old backups", "older_than", olderThan, "keep_last", keepLast)

	candidates, _, err := getCleanupCandidates(olderThan, keepLast, cfg)
	if err != nil {
		return 0, 0, 0, err
	}

	var firstErr error
	var actualFreed int64

	for _, candidate := range candidates {
		if err := fs.RemoveAll(candidate.Path, cfg); err != nil {
			failed++
			cfg.Logger.Error("failed to remove backup directory", "path", candidate.Path, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("removing %s: %w", candidate.Path, err)
			}
			continue
		}
		deleted++
		actualFreed += candidate.Size
		cfg.Logger.Debug("deleted backup directory", "path", candidate.Path)
	}

	cfg.Logger.Info("backup cleanup complete",
		"deleted", deleted,
		"failed", failed,
		"freed_bytes", actualFreed,
	)

	return deleted, failed, actualFreed, firstErr
}

// getCleanupCandidates returns backup directories that match cleanup criteria
func getCleanupCandidates(olderThan time.Duration, keepLast int, cfg *config.Config) ([]CleanupCandidate, int64, error) {
	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return nil, 0, err
	}

	// Check if backup directory exists
	if !fs.PathExists(backupDir) {
		return nil, 0, nil
	}

	// Get list of timestamp directories
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, 0, fmt.Errorf("reading backup directory: %w", err)
	}

	// Parse and sort directories by timestamp
	type timestampDir struct {
		name      string
		timestamp time.Time
		path      string
	}

	var dirs []timestampDir
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		timestamp, err := time.Parse(TimestampFormat, entry.Name())
		if err != nil {
			continue // Skip directories that don't match timestamp format
		}

		dirs = append(dirs, timestampDir{
			name:      entry.Name(),
			timestamp: timestamp,
			path:      filepath.Join(backupDir, entry.Name()),
		})
	}

	// Sort by timestamp (newest first)
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].timestamp.After(dirs[j].timestamp)
	})

	// Determine which directories to delete
	cutoff := time.Now().Add(-olderThan)
	var candidates []CleanupCandidate
	var totalSize int64

	for i, dir := range dirs {
		// Keep at least keepLast backups
		if i < keepLast {
			continue
		}

		// Check if older than cutoff
		if dir.timestamp.Before(cutoff) {
			// Calculate size
			size, _ := getDirSize(dir.path)
			totalSize += size
			candidates = append(candidates, CleanupCandidate{
				Path:      dir.path,
				Timestamp: dir.timestamp,
				Size:      size,
			})
		}
	}

	return candidates, totalSize, nil
}

// getDirSize calculates the total size of a directory
func getDirSize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// GetBackupsForFile returns backups for a specific file (by normalized source path)
func GetBackupsForFile(sourcePath string, cfg *config.Config) ([]BackupInfo, error) {
	cfg.Logger.Debug("getting backups for file", "file", sourcePath)

	normalized, err := config.NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	backupRelativePath := strings.TrimPrefix(normalized, "~/")

	allBackups, err := ListBackups(cfg)
	if err != nil {
		return nil, err
	}

	var fileBackups []BackupInfo
	for _, backup := range allBackups {
		// Match on the relative path portion
		backupDir, dirErr := GetBackupDir()
		if dirErr == nil {
			backupRelPath, relErr := filepath.Rel(backupDir, backup.BackupPath)
			if relErr == nil {
				// Path will be like "2025-01-04_10-30-15/config/nvim/init.lua"
				// We need to match after the timestamp directory
				if parts := strings.SplitN(backupRelPath, string(filepath.Separator), 2); len(parts) == 2 && parts[1] == backupRelativePath {
					fileBackups = append(fileBackups, backup)
				}
			}
		}
	}

	return fileBackups, nil
}

// GetLatestBackup returns the most recent backup for a file
func GetLatestBackup(sourcePath string, cfg *config.Config) (*BackupInfo, error) {
	backups, err := GetBackupsForFile(sourcePath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to get backups for file", "file", sourcePath, "error", err)
		return nil, err
	}

	if len(backups) == 0 {
		cfg.Logger.Debug("no backups found for file", "file", sourcePath)
		return nil, fmt.Errorf("no backups found for: %s", sourcePath)
	}

	return &backups[0], nil
}

// BackupExists checks if any backup exists for a file
func BackupExists(sourcePath string, cfg *config.Config) bool {
	backups, err := GetBackupsForFile(sourcePath, cfg)
	if err != nil {
		return false
	}
	return len(backups) > 0
}

// GetBackupCount returns the total number of backups
func GetBackupCount(cfg *config.Config) (int, error) {
	backups, err := ListBackups(cfg)
	if err != nil {
		cfg.Logger.Error("failed to list backups", "error", err)
		return 0, err
	}
	return len(backups), nil
}

// GetTotalBackupSize returns the total size of all backups
func GetTotalBackupSize(cfg *config.Config) (int64, error) {
	cfg.Logger.Debug("calculating total backup size")

	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return 0, err
	}

	if !fs.PathExists(backupDir) {
		return 0, nil
	}

	size, err := getDirSize(backupDir)
	if err != nil {
		cfg.Logger.Error("failed to calculate backup size", "error", err)
	}
	cfg.Logger.Info("total backup size calculated", "bytes", size)
	return size, err
}
