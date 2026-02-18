package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/justincordova/dotcor/internal/config"
)

func MoveFile(src, dst string, cfg *config.Config) error {
	start := time.Now()
	cfg.Logger.Debug("moving file", "src", src, "dst", dst)

	if err := EnsureDir(filepath.Dir(dst), cfg); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	err := os.Rename(src, dst)
	if err == nil {
		cfg.Logger.Info("file moved successfully", "src", src, "dst", dst)
		return nil
	}

	cfg.Logger.Debug("rename failed, trying copy", "src", src, "dst", dst, "error", err)
	if err := CopyWithPermissions(src, dst, cfg); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	dstExisted := PathExists(dst)
	if err := os.Remove(src); err != nil {
		if !dstExisted {
			os.Remove(dst)
		}
		cfg.Logger.Error("failed to remove original file", "error", err)
		return fmt.Errorf("removing original file: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()
	cfg.Logger.Info("file moved", "src", src, "dst", dst, "duration_ms", durationMs)
	return nil
}

func CopyFile(src, dst string, cfg *config.Config) error {
	cfg.Logger.Debug("copying file", "src", src, "dst", dst)
	return CopyWithPermissions(src, dst, cfg)
}

func CopyWithPermissions(src, dst string, cfg *config.Config) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		cfg.Logger.Error("failed to get source file info", "path", src, "error", err)
		return fmt.Errorf("getting source file info: %w", err)
	}

	if err := EnsureDir(filepath.Dir(dst), cfg); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		cfg.Logger.Error("failed to open source file", "path", src, "error", err)
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		cfg.Logger.Error("failed to create destination file", "path", dst, "error", err)
		return fmt.Errorf("creating destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		cfg.Logger.Error("failed to copy file contents", "src", src, "dst", dst, "error", err)
		return fmt.Errorf("copying file contents: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		cfg.Logger.Error("failed to sync file", "dst", dst, "error", err)
		return fmt.Errorf("syncing destination file: %w", err)
	}

	if err := os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		cfg.Logger.Debug("failed to set file times (non-fatal)", "error", err)
	}

	return nil
}

func EnsureDir(path string, cfg *config.Config) error {
	if path == "" {
		return nil
	}

	cfg.Logger.Debug("ensuring directory exists", "path", path)

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			cfg.Logger.Debug("directory already exists", "path", path)
			return nil
		}
		return fmt.Errorf("path exists but is not a directory: %s", path)
	}

	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			cfg.Logger.Error("failed to create directory", "path", path, "error", err)
			return fmt.Errorf("creating directory: %w", err)
		}
		cfg.Logger.Debug("directory created", "path", path)
		return nil
	}

	return fmt.Errorf("checking directory: %w", err)
}

// PathExists checks if path exists (file or directory)
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsDirectory(path string, cfg *config.Config) (bool, error) {
	cfg.Logger.Debug("checking if path is directory", "path", path)
	info, err := os.Stat(path)
	if err != nil {
		cfg.Logger.Error("failed to stat path", "path", path, "error", err)
		return false, fmt.Errorf("checking path: %w", err)
	}
	isDir := info.IsDir()
	cfg.Logger.Debug("directory check result", "path", path, "is_dir", isDir)
	return isDir, nil
}

func GetFileSize(path string, cfg *config.Config) (int64, error) {
	cfg.Logger.Debug("getting file size", "path", path)
	info, err := os.Stat(path)
	if err != nil {
		cfg.Logger.Error("failed to stat file", "path", path, "error", err)
		return 0, fmt.Errorf("getting file info: %w", err)
	}
	size := info.Size()
	cfg.Logger.Debug("file size retrieved", "path", path, "size", size)
	return size, nil
}

func RemoveFile(path string, cfg *config.Config) error {
	cfg.Logger.Debug("removing file", "path", path)
	if err := os.Remove(path); err != nil {
		cfg.Logger.Error("failed to remove file", "path", path, "error", err)
		return fmt.Errorf("removing file: %w", err)
	}
	cfg.Logger.Debug("file removed", "path", path)
	return nil
}

func RemoveAll(path string, cfg *config.Config) error {
	cfg.Logger.Debug("removing path", "path", path)
	if err := os.RemoveAll(path); err != nil {
		cfg.Logger.Error("failed to remove path", "path", path, "error", err)
		return fmt.Errorf("removing path: %w", err)
	}
	cfg.Logger.Debug("path removed", "path", path)
	return nil
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
		defer os.Remove(tempFile)
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
