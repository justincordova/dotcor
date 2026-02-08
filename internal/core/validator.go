package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

// Secret detection patterns
var secretPatterns = []*regexp.Regexp{
	// API keys and tokens
	regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),
	regexp.MustCompile(`(?i)api[_-]?secret\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),
	regexp.MustCompile(`(?i)access[_-]?token\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),
	regexp.MustCompile(`(?i)auth[_-]?token\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),

	// Passwords
	regexp.MustCompile(`(?i)password\s*[:=]\s*['"]?[^\s'";]{8,}['"]?`),
	regexp.MustCompile(`(?i)passwd\s*[:=]\s*['"]?[^\s'";]{8,}['"]?`),

	// Secrets
	regexp.MustCompile(`(?i)secret\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),
	regexp.MustCompile(`(?i)private[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),

	// Private key headers
	regexp.MustCompile(`-----BEGIN\s+.*PRIVATE\s+KEY-----`),
	regexp.MustCompile(`-----BEGIN\s+RSA\s+PRIVATE\s+KEY-----`),
	regexp.MustCompile(`-----BEGIN\s+EC\s+PRIVATE\s+KEY-----`),
	regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`),

	// Cloud provider credentials
	regexp.MustCompile(`(?i)aws[_-]?access[_-]?key[_-]?id\s*[:=]\s*['"]?[A-Z0-9]{20}['"]?`),
	regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9/+=]{40}['"]?`),
	regexp.MustCompile(`(?i)azure[_-]?.*secret`),
	regexp.MustCompile(`(?i)gcp[_-]?.*secret`),

	// Database connection strings with passwords
	regexp.MustCompile(`(?i)postgres://[^:]+:[^@]+@`),
	regexp.MustCompile(`(?i)mysql://[^:]+:[^@]+@`),
	regexp.MustCompile(`(?i)mongodb://[^:]+:[^@]+@`),

	// Generic credentials
	regexp.MustCompile(`(?i)credentials\s*[:=]\s*['"]?[^\s'";]{10,}['"]?`),
}

// Large file warning threshold (100MB)
const LargeFileThreshold = 100 * 1024 * 1024

// ValidateSourceFile checks if source file is valid for adding
func ValidateSourceFile(path string, cfg *config.Config) error {
	cfg.Logger.Debug("validating source file", "file", path)

	// Expand path
	expanded, err := config.ExpandPath(path, cfg)
	if err != nil {
		cfg.Logger.Debug("failed to expand path", "file", path, "error", err)
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check if file exists
	info, err := os.Stat(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			cfg.Logger.Debug("file does not exist", "file", path)
			return fmt.Errorf("file does not exist: %s", path)
		}
		cfg.Logger.Debug("failed to check file", "file", path, "error", err)
		return fmt.Errorf("checking file: %w", err)
	}

	// Must be a regular file (not directory for single file add)
	if info.IsDir() {
		cfg.Logger.Warn("path is a directory", "path", path)
		return fmt.Errorf("path is a directory, use --recursive flag: %s", path)
	}

	// Check if file is readable
	if !fs.IsReadable(expanded) {
		cfg.Logger.Debug("file is not readable", "file", path)
		return fmt.Errorf("file is not readable: %s", path)
	}

	// Check if file is inside dotcor directory
	if err := ValidateNotInDotcorDir(path, cfg); err != nil {
		cfg.Logger.Debug("file is in dotcor directory", "file", path, "error", err)
		return err
	}

	// Check if already a symlink pointing to our repo
	isLink, err := fs.IsSymlink(expanded)
	if err != nil {
		cfg.Logger.Debug("failed to check symlink", "file", path, "error", err)
		return fmt.Errorf("checking symlink: %w", err)
	}
	if isLink {
		pointsToRepo, err := fs.SymlinkPointsToRepo(expanded, cfg.RepoPath)
		if err != nil {
			cfg.Logger.Debug("failed to check symlink target", "file", path, "error", err)
			return fmt.Errorf("checking symlink target: %w", err)
		}
		if pointsToRepo {
			cfg.Logger.Debug("file already managed by dotcor", "file", path)
			return fmt.Errorf("file is already a symlink pointing to dotcor repo: %s", path)
		}
		// It's a symlink but points elsewhere - suggest using adopt
		cfg.Logger.Debug("file is symlink pointing elsewhere", "file", path)
		return fmt.Errorf("file is a symlink pointing elsewhere, use 'dotcor adopt' instead: %s", path)
	}

	cfg.Logger.Debug("validation complete", "file", path, "valid", true)
	return nil
}

// ValidateRepoPath checks if repo path is valid
func ValidateRepoPath(path string) error {
	if path == "" {
		return fmt.Errorf("repo path cannot be empty")
	}

	// Must be relative path (no leading /)
	if filepath.IsAbs(path) {
		return fmt.Errorf("repo path must be relative, not absolute: %s", path)
	}

	// Must not contain path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("repo path cannot contain '..': %s", path)
	}

	// Must not start with /
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, string(filepath.Separator)) {
		return fmt.Errorf("repo path cannot start with separator: %s", path)
	}

	return nil
}

// ValidateNotAlreadyManaged checks if file is not already managed
func ValidateNotAlreadyManaged(cfg *config.Config, sourcePath string) error {
	if cfg.IsManaged(sourcePath) {
		return fmt.Errorf("file is already managed by dotcor: %s", sourcePath)
	}
	return nil
}

// ValidateNotInDotcorDir checks file isn't inside ~/.dotcor/ (circular reference)
func ValidateNotInDotcorDir(path string, cfg *config.Config) error {
	expanded, err := config.ExpandPath(path, cfg)
	if err != nil {
		return fmt.Errorf("expanding path: %w", err)
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config dir: %w", err)
	}

	// Check if path is under dotcor directory
	if strings.HasPrefix(expanded, configDir) {
		return fmt.Errorf("cannot add files from inside dotcor directory: %s", path)
	}

	return nil
}

// ValidateFileSize checks file isn't unreasonably large (>100MB warning)
func ValidateFileSize(path string, cfg *config.Config) error {
	cfg.Logger.Debug("validating file size", "file", path)

	expanded, err := config.ExpandPath(path, cfg)
	if err != nil {
		cfg.Logger.Debug("failed to expand path for validation", "file", path, "error", err)
		return fmt.Errorf("expanding path: %w", err)
	}

	info, err := os.Stat(expanded)
	if err != nil {
		cfg.Logger.Debug("failed to get file info", "file", path, "error", err)
		return fmt.Errorf("getting file info: %w", err)
	}

	size := info.Size()
	cfg.Logger.Debug("file size check", "file", path, "size", size, "threshold", LargeFileThreshold)

	if size > LargeFileThreshold {
		sizeMB := float64(size) / (1024 * 1024)
		cfg.Logger.Warn("file is very large", "file", path, "size_mb", sizeMB)
		return fmt.Errorf("file is very large (%.1fMB), consider excluding: %s", sizeMB, path)
	}

	return nil
}

// DetectSecrets scans file content for potential secrets
func DetectSecrets(path string) ([]string, error) {
	expanded, err := config.ExpandPath(path, nil)
	if err != nil {
		return nil, fmt.Errorf("expanding path: %w", err)
	}

	content, err := os.ReadFile(expanded)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var warnings []string
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		for _, pattern := range secretPatterns {
			matches := pattern.FindAllString(line, -1)
			for _, match := range matches {
				// Truncate match if too long
				displayMatch := match
				if len(displayMatch) > 50 {
					displayMatch = displayMatch[:50] + "..."
				}
				warning := fmt.Sprintf("Line %d: %s", lineNum+1, displayMatch)
				warnings = append(warnings, warning)
			}
		}
	}

	return warnings, nil
}

// ShouldWarnAboutSecrets returns true if file likely contains secrets
func ShouldWarnAboutSecrets(path string, warnings []string) bool {
	return len(warnings) > 0
}

// ValidateAll runs all validations on a file
func ValidateAll(path string, cfg *config.Config) (warnings []string, err error) {
	cfg.Logger.Debug("running all validations", "file", path)

	// Basic validations
	if err := ValidateSourceFile(path, cfg); err != nil {
		return nil, err
	}

	if err := ValidateNotAlreadyManaged(cfg, path); err != nil {
		return nil, err
	}

	// Check file size (non-fatal, just return warning)
	if err := ValidateFileSize(path, cfg); err != nil {
		warnings = append(warnings, err.Error())
	}

	// Check for secrets
	secretWarnings, err := DetectSecrets(path)
	if err != nil {
		// Non-fatal, just skip secret detection
		cfg.Logger.Warn("secret detection failed, skipping", "file", path, "error", err)
	} else {
		if len(secretWarnings) > 0 {
			cfg.Logger.Warn("potential secrets detected", "file", path, "count", len(secretWarnings))
		}
		warnings = append(warnings, secretWarnings...)
	}

	cfg.Logger.Debug("all validations complete", "file", path, "warnings", len(warnings))
	return warnings, nil
}

// ValidateSymlinkTarget checks if a symlink target is valid for adoption
func ValidateSymlinkTarget(linkPath string, cfg *config.Config) error {
	// Check if it's actually a symlink
	isLink, err := fs.IsSymlink(linkPath)
	if err != nil {
		return fmt.Errorf("checking symlink: %w", err)
	}
	if !isLink {
		return fmt.Errorf("path is not a symlink: %s", linkPath)
	}

	// Check if symlink is valid (target exists)
	valid, err := fs.IsValidSymlink(linkPath)
	if err != nil {
		return fmt.Errorf("validating symlink: %w", err)
	}
	if !valid {
		return fmt.Errorf("symlink target does not exist: %s", linkPath)
	}

	// Check if already points to our repo
	pointsToRepo, err := fs.SymlinkPointsToRepo(linkPath, cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("checking symlink target: %w", err)
	}
	if pointsToRepo {
		return fmt.Errorf("symlink already points to dotcor repo: %s", linkPath)
	}

	return nil
}
