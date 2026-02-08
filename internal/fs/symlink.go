package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
)

// ErrSymlinkUnsupported is returned when symlinks are not supported on the platform
var ErrSymlinkUnsupported = errors.New("symlink support required - enable Developer Mode on Windows")

// SymlinkStatus represents the detailed status of a symlink
type SymlinkStatus struct {
	Exists       bool   // Whether the symlink path exists
	IsSymlink    bool   // Whether it's actually a symlink (not a regular file)
	TargetExists bool   // Whether the target file exists
	PointsToRepo bool   // Whether it points to our repo
	IsRelative   bool   // Whether the symlink uses relative path
	ActualTarget string // The actual target path of the symlink
}

// CreateSymlink creates a RELATIVE symlink at `link` pointing to `target`.
// The symlink uses a relative path computed from link's location to target.
// Returns error if symlink fails (NO COPY FALLBACK).
func CreateSymlink(target, link string, cfg *config.Config) error {
	// Check if platform supports symlinks
	supported, err := SupportsSymlinks()
	if err != nil {
		return fmt.Errorf("checking symlink support: %w", err)
	}
	if !supported {
		return ErrSymlinkUnsupported
	}

	// Expand paths
	expandedTarget, err := config.ExpandPath(target)
	if err != nil {
		return fmt.Errorf("expanding target path: %w", err)
	}

	expandedLink, err := config.ExpandPath(link)
	if err != nil {
		return fmt.Errorf("expanding link path: %w", err)
	}

	// Ensure parent directory exists
	if err := EnsureDir(filepath.Dir(expandedLink), cfg); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	// Compute RELATIVE path from link to target
	relPath, err := config.ComputeRelativeSymlink(expandedLink, expandedTarget)
	if err != nil {
		return fmt.Errorf("computing relative path: %w", err)
	}

	// Create symlink in temp location first (atomic rename pattern)
	tempLink := expandedLink + ".tmp"
	if err := os.Symlink(relPath, tempLink); err != nil {
		return fmt.Errorf("creating temp symlink: %w", err)
	}

	// Atomically rename to target (works on Unix, Windows supports it too)
	if err := os.Rename(tempLink, expandedLink); err != nil {
		os.Remove(tempLink)
		return fmt.Errorf("moving symlink into place: %w", err)
	}

	return nil
}

// RemoveSymlink removes a symlink (validates it's actually a symlink first)
func RemoveSymlink(link string, cfg *config.Config) error {
	expandedLink, err := config.ExpandPath(link)
	if err != nil {
		return fmt.Errorf("expanding link path: %w", err)
	}

	// Check if it's actually a symlink
	isLink, err := IsSymlink(expandedLink)
	if err != nil {
		return fmt.Errorf("checking if symlink: %w", err)
	}
	if !isLink {
		return fmt.Errorf("path is not a symlink: %s", link)
	}

	if err := os.Remove(expandedLink); err != nil {
		return fmt.Errorf("removing symlink: %w", err)
	}

	return nil
}

// IsSymlink checks if path is a symlink
func IsSymlink(path string) (bool, error) {
	expandedPath, err := config.ExpandPath(path)
	if err != nil {
		return false, fmt.Errorf("expanding path: %w", err)
	}

	info, err := os.Lstat(expandedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting file info: %w", err)
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

// ReadSymlink reads the target of a symlink (returns raw target, may be relative)
func ReadSymlink(link string) (string, error) {
	expandedLink, err := config.ExpandPath(link)
	if err != nil {
		return "", fmt.Errorf("expanding path: %w", err)
	}

	target, err := os.Readlink(expandedLink)
	if err != nil {
		return "", fmt.Errorf("reading symlink: %w", err)
	}

	return target, nil
}

// IsValidSymlink checks if symlink exists and points to existing target
// Resolves relative paths to check target existence
func IsValidSymlink(link string) (bool, error) {
	expandedLink, err := config.ExpandPath(link)
	if err != nil {
		return false, fmt.Errorf("expanding path: %w", err)
	}

	// Check if it's a symlink
	isLink, err := IsSymlink(expandedLink)
	if err != nil {
		return false, err
	}
	if !isLink {
		return false, nil
	}

	// Read the target
	target, err := ReadSymlink(expandedLink)
	if err != nil {
		return false, err
	}

	// If target is relative, resolve it from the symlink's directory
	var fullTarget string
	if !filepath.IsAbs(target) {
		fullTarget = filepath.Join(filepath.Dir(expandedLink), target)
	} else {
		fullTarget = target
	}

	// Check if target exists
	_, err = os.Stat(fullTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Symlink exists but target doesn't
		}
		return false, fmt.Errorf("checking target: %w", err)
	}

	return true, nil
}

// SupportsSymlinks checks if current platform supports symlinks
// Windows: requires admin rights or developer mode
// Returns true on macOS/Linux, checks on Windows
func SupportsSymlinks() (bool, error) {
	if runtime.GOOS != "windows" {
		return true, nil
	}

	// On Windows, test by creating a temporary symlink
	tmpDir := os.TempDir()
	testFile := filepath.Join(tmpDir, "dotcor_test_file")
	testLink := filepath.Join(tmpDir, "dotcor_test_link")

	// Clean up any existing test files
	os.Remove(testFile)
	os.Remove(testLink)

	// Create test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return false, fmt.Errorf("creating test file: %w", err)
	}
	defer os.Remove(testFile)

	// Try to create symlink
	err := os.Symlink(testFile, testLink)
	if err != nil {
		return false, nil // Symlinks not supported
	}
	defer os.Remove(testLink) // Clean up test symlink

	return true, nil
}

// GetSymlinkStatus returns detailed status of a symlink
func GetSymlinkStatus(linkPath string, expectedTarget string) (SymlinkStatus, error) {
	status := SymlinkStatus{}

	expandedLink, err := config.ExpandPath(linkPath)
	if err != nil {
		return status, fmt.Errorf("expanding link path: %w", err)
	}

	// Check if path exists
	info, err := os.Lstat(expandedLink)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil // Path doesn't exist
		}
		return status, fmt.Errorf("checking path: %w", err)
	}
	status.Exists = true

	// Check if it's a symlink
	status.IsSymlink = info.Mode()&os.ModeSymlink != 0
	if !status.IsSymlink {
		return status, nil // Not a symlink
	}

	// Read symlink target
	target, err := os.Readlink(expandedLink)
	if err != nil {
		return status, fmt.Errorf("reading symlink: %w", err)
	}
	status.ActualTarget = target

	// Check if target is relative
	status.IsRelative = !filepath.IsAbs(target)

	// Resolve target path
	var fullTarget string
	if status.IsRelative {
		fullTarget = filepath.Join(filepath.Dir(expandedLink), target)
	} else {
		fullTarget = target
	}

	// Check if target exists
	_, err = os.Stat(fullTarget)
	status.TargetExists = err == nil

	// Check if target points to our repo
	if expectedTarget != "" {
		expandedExpected, err := config.ExpandPath(expectedTarget)
		if err != nil {
			return status, fmt.Errorf("expanding expected target path: %w", err)
		}

		// Clean both paths for comparison
		cleanTarget := filepath.Clean(fullTarget)
		cleanExpected := filepath.Clean(expandedExpected)
		status.PointsToRepo = cleanTarget == cleanExpected
	}

	return status, nil
}

// ResolveSymlink returns the absolute path that a symlink points to
func ResolveSymlink(link string) (string, error) {
	expandedLink, err := config.ExpandPath(link)
	if err != nil {
		return "", fmt.Errorf("expanding path: %w", err)
	}

	// Read the symlink target
	target, err := os.Readlink(expandedLink)
	if err != nil {
		return "", fmt.Errorf("reading symlink: %w", err)
	}

	// If target is relative, resolve from symlink's directory
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(expandedLink), target)
	}

	return filepath.Clean(target), nil
}

// IsRelativeSymlink checks if a symlink uses a relative path
func IsRelativeSymlink(link string) (bool, error) {
	target, err := ReadSymlink(link)
	if err != nil {
		return false, err
	}
	return !filepath.IsAbs(target), nil
}

// SymlinkPointsToRepo checks if a symlink points to a file in the dotcor repo
func SymlinkPointsToRepo(link string, repoPath string) (bool, error) {
	resolved, err := ResolveSymlink(link)
	if err != nil {
		return false, err
	}

	expandedRepo, err := config.ExpandPath(repoPath)
	if err != nil {
		return false, fmt.Errorf("expanding repo path: %w", err)
	}

	// Check if resolved path is under repo
	return strings.HasPrefix(resolved, expandedRepo), nil
}
