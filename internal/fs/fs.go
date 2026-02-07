package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFile moves a file from src to dst, preserving permissions
// Uses os.Rename when possible, falls back to copy+delete for cross-device moves
func MoveFile(src, dst string) error {
	// Ensure destination directory exists
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Try rename first (fast, atomic on same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If rename failed (likely cross-device), fall back to copy+delete
	if err := CopyWithPermissions(src, dst); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	// Check if dst existed before copying
	dstExisted := FileExists(dst)

	// Remove original after successful copy
	if err := os.Remove(src); err != nil {
		// Only remove dst if we created it (not if it existed before)
		if !dstExisted {
			os.Remove(dst)
		}
		return fmt.Errorf("removing original file: %w", err)
	}

	return nil
}

// CopyFile copies file with permissions preserved
func CopyFile(src, dst string) error {
	return CopyWithPermissions(src, dst)
}

// CopyWithPermissions copies file preserving all metadata (permissions, timestamps)
func CopyWithPermissions(src, dst string) error {
	// Get source file info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("getting source file info: %w", err)
	}

	// Ensure destination directory exists
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file with same permissions
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying file contents: %w", err)
	}

	// Sync to ensure data is written
	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("syncing destination file: %w", err)
	}

	// Preserve timestamps
	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		// Non-fatal, just log or ignore
		// Some filesystems don't support this
	}

	return nil
}

// FileExists checks if file exists (and is not a directory)
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// PathExists checks if a path exists (file or directory)
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureDir creates directory if it doesn't exist (including parents)
func EnsureDir(path string) error {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return nil // Directory already exists
		}
		return fmt.Errorf("path exists but is not a directory: %s", path)
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
		return nil
	}

	return fmt.Errorf("checking directory: %w", err)
}

// IsDirectory checks if path is a directory
func IsDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("checking path: %w", err)
	}
	return info.IsDir(), nil
}

// Exists checks if path exists (file or directory)
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetFileSize returns file size in bytes
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("getting file info: %w", err)
	}
	return info.Size(), nil
}

// RemoveFile removes a file or empty directory
func RemoveFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}
	return nil
}

// RemoveAll removes a file or directory and all its contents
func RemoveAll(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("removing path: %w", err)
	}
	return nil
}

// GetFilesRecursive returns all files in directory recursively
func GetFilesRecursive(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return files, nil
}

// IsReadable checks if a file is readable
func IsReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// IsWritable checks if a path is writable
func IsWritable(path string) bool {
	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Check if parent directory is writable
			parent := filepath.Dir(path)
			return IsWritable(parent)
		}
		return false
	}

	// If it's a directory, try to create a temp file
	if info.IsDir() {
		tempFile := filepath.Join(path, ".dotcor_write_test")
		file, err := os.Create(tempFile)
		if err != nil {
			return false
		}
		file.Close()
		os.Remove(tempFile)
		return true
	}

	// For files, try to open for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// GetFileMode returns the file mode/permissions
func GetFileMode(path string) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("getting file info: %w", err)
	}
	return info.Mode(), nil
}
