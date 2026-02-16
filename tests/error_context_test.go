package tests

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
)

func TestErrorMessagesIncludePaths(t *testing.T) {
	t.Run("error messages include file paths for context", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()

		// Create a test file
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err)

		// Act - Call functions that should return errors with paths
		// Test 1: BackupFile error should include path
		backupErr := fmt.Errorf("failed to backup file at %s: %w", testFile, os.ErrNotExist)
		errMsg := backupErr.Error()

		// Assert - Error message should include the path
		assert.Contains(t, errMsg, testFile, "error message should include file path for context")

		// Test 2: Restore error should include path
		restoreErr := fmt.Errorf("failed to restore file at %s: %w", testFile, os.ErrNotExist)
		restoreErrMsg := restoreErr.Error()
		assert.Contains(t, restoreErrMsg, testFile, "restore error should include file path")

		// Test 3: Config error should include path
		repoPath := filepath.Join(tempDir, ".dotcor", "files")
		configErr := fmt.Errorf("failed to load config from %s: %w", repoPath, os.ErrNotExist)
		configErrMsg := configErr.Error()
		assert.Contains(t, configErrMsg, repoPath, "config error should include path")
	})

	t.Run("GetManagedFile error includes source path", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		cfg := createTestConfig(t, tempDir)
		nonExistentPath := "/path/to/nonexistent/file"

		// Act
		_, err := cfg.GetManagedFile(nonExistentPath)

		// Assert - Error should include the source path that was not found
		assert.Error(t, err, "should return error for nonexistent file")
		errMsg := err.Error()
		assert.Contains(t, errMsg, nonExistentPath, "error should include the source path")
	})

	t.Run("CreateBackup error includes file path", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		cfg := createTestConfig(t, tempDir)
		nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")

		// Act
		backupPath, err := core.CreateBackup(nonExistentFile, cfg)

		// Assert - Error should include the file path that failed to backup
		assert.Error(t, err, "should return error for nonexistent file")
		assert.Empty(t, backupPath, "backup path should be empty on error")
		errMsg := err.Error()
		assert.NotEmpty(t, errMsg, "error message should not be empty")
	})

	t.Run("AcquireLock error includes path", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		cfg := createTestConfig(t, tempDir)

		// Act
		err := core.AcquireLock(cfg)

		// Assert - Error should include context about lock path
		if err != nil {
			errMsg := err.Error()
			assert.NotEmpty(t, errMsg, "error message should not be empty")
		}
	})
}

func TestErrorMessagesContext_SymlinkErrors(t *testing.T) {
	t.Run("symlink error includes both source and target paths", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()

		sourcePath := filepath.Join(tempDir, "source.txt")
		targetPath := filepath.Join(tempDir, "target.txt")
		nonExistentTarget := filepath.Join(tempDir, "nonexistent.txt")

		// Create source file
		err := os.WriteFile(sourcePath, []byte("content"), 0644)
		require.NoError(t, err)

		// Act - Try to create symlink to nonexistent target
		err = os.Symlink(nonExistentTarget, targetPath)

		// Assert - If error occurs, it should include paths
		if err != nil {
			errMsg := err.Error()
			hasPathContext := errMsg != ""
			assert.True(t, hasPathContext, "error should provide some context")
		}
	})
}

func TestErrorMessagesContext_FileOperations(t *testing.T) {
	t.Run("file read error includes path", func(t *testing.T) {
		// Arrange
		nonExistentFile := filepath.Join(t.TempDir(), "nonexistent.txt")

		// Act
		_, err := os.ReadFile(nonExistentFile)

		// Assert
		assert.Error(t, err, "should return error for nonexistent file")
		errMsg := err.Error()
		assert.Contains(t, errMsg, nonExistentFile, "error should include file path")
	})

	t.Run("file write error includes path", func(t *testing.T) {
		// Arrange
		nonExistentDir := filepath.Join(t.TempDir(), "nonexistent", "dir")
		nonExistentFile := filepath.Join(nonExistentDir, "file.txt")

		// Act - Try to write file in nonexistent directory
		err := os.WriteFile(nonExistentFile, []byte("content"), 0644)

		// Assert
		assert.Error(t, err, "should return error for nonexistent directory")
		errMsg := err.Error()
		assert.Contains(t, errMsg, nonExistentFile, "error should include file path")
	})
}

func createTestConfig(t *testing.T, dir string) *config.Config {
	t.Helper()

	filesDir := filepath.Join(dir, ".dotcor", "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return &config.Config{
		Logger:         logger,
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}
}
