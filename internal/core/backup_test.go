package core

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBackup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
	}

	sourceContent := []byte("original content")
	sourceFile := filepath.Join(tempDir, "source.txt")
	err := os.WriteFile(sourceFile, sourceContent, 0644)
	require.NoError(t, err, "failed to create source file")

	// Act
	backupPath, err := CreateBackup(sourceFile, cfg)

	// Assert
	require.NoError(t, err, "CreateBackup() should not error")

	_, err = os.Stat(backupPath)
	assert.NoError(t, err, "CreateBackup() backup file not created")

	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err, "failed to read backup file")

	assert.Equal(t, string(sourceContent), string(backupContent), "CreateBackup() content mismatch")
}

func TestCreateBackupNonexistent(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Logger: slog.Default(),
	}

	// Act
	_, err := CreateBackup("/nonexistent/path/file.txt", cfg)

	// Assert
	assert.Error(t, err, "CreateBackup() should error for nonexistent file")
}

// TestCreateBackup_SameSecondNoClobber verifies that two backups of the
// same file created within the same second-granularity timestamp window
// do not overwrite each other. Before the fix the second backup truncated
// the first (shared path + O_TRUNC copy), so a rollback could restore the
// wrong contents.
func TestCreateBackup_SameSecondNoClobber(t *testing.T) {
	// Isolate the backup directory under DOTCOR_DIR so the test doesn't
	// touch the real ~/.dotcor.
	t.Setenv("DOTCOR_DIR", t.TempDir())

	cfg := &config.Config{Logger: slog.Default()}

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	sourceFile := filepath.Join(home, ".dotcor-backup-collision-test")
	require.NoError(t, os.WriteFile(sourceFile, []byte("version one"), 0644))
	defer func() { _ = os.Remove(sourceFile) }()

	// First backup captures "version one".
	backup1, err := CreateBackup(sourceFile, cfg)
	require.NoError(t, err)

	// The file changes and is backed up again, almost certainly within the
	// same second.
	require.NoError(t, os.WriteFile(sourceFile, []byte("version two"), 0644))
	backup2, err := CreateBackup(sourceFile, cfg)
	require.NoError(t, err)

	require.NotEqual(t, backup1, backup2, "the two backups must use distinct paths")

	content1, err := os.ReadFile(backup1)
	require.NoError(t, err)
	assert.Equal(t, "version one", string(content1), "first backup must still hold the original content")

	content2, err := os.ReadFile(backup2)
	require.NoError(t, err)
	assert.Equal(t, "version two", string(content2), "second backup must hold the updated content")
}

func TestRestoreBackup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
	}

	backupContent := []byte("backup content")
	backupFile := filepath.Join(tempDir, "backup.txt")
	err := os.WriteFile(backupFile, backupContent, 0644)
	require.NoError(t, err, "failed to create backup file")

	targetFile := filepath.Join(tempDir, "restored.txt")

	// Act
	err = RestoreBackup(backupFile, targetFile, cfg)

	// Assert
	require.NoError(t, err, "RestoreBackup() should not error")

	targetContent, err := os.ReadFile(targetFile)
	require.NoError(t, err, "failed to read target file")

	assert.Equal(t, string(backupContent), string(targetContent), "RestoreBackup() content mismatch")
}

func TestRestoreBackupCreatesParentDir(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
	}

	backupFile := filepath.Join(tempDir, "backup.txt")
	err := os.WriteFile(backupFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create backup file")

	targetFile := filepath.Join(tempDir, "nested", "dir", "restored.txt")

	// Act
	err = RestoreBackup(backupFile, targetFile, cfg)

	// Assert
	require.NoError(t, err, "RestoreBackup() should not error")

	_, err = os.Stat(targetFile)
	assert.NoError(t, err, "RestoreBackup() target file not created")
}

func TestBackupExists(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Logger: slog.Default(),
	}

	// Act
	exists := BackupExists("random_nonexistent_file_12345.txt", cfg)

	// Assert
	assert.False(t, exists, "BackupExists() should return false for nonexistent backups")
}

func TestGetBackupCount(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Logger: slog.Default(),
	}

	// Act
	count, err := GetBackupCount(cfg)

	// Assert
	require.NoError(t, err, "GetBackupCount() should not error")
	assert.GreaterOrEqual(t, count, 0, "GetBackupCount() should be >= 0")
}

func TestGetTotalBackupSize(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Logger: slog.Default(),
	}

	// Act
	size, err := GetTotalBackupSize(cfg)

	// Assert
	require.NoError(t, err, "GetTotalBackupSize() should not error")
	assert.GreaterOrEqual(t, size, int64(0), "GetTotalBackupSize() should be >= 0")
}

func TestTimestampFormat(t *testing.T) {
	// Arrange
	now := time.Now()

	// Act
	formatted := now.Format(TimestampFormat)
	parsed, err := time.Parse(TimestampFormat, formatted)

	// Assert
	require.NoError(t, err, "TimestampFormat not parseable")

	assert.Equal(t, now.Year(), parsed.Year(), "TimestampFormat lost precision (year)")
	assert.Equal(t, now.Month(), parsed.Month(), "TimestampFormat lost precision (month)")
	assert.Equal(t, now.Day(), parsed.Day(), "TimestampFormat lost precision (day)")
	assert.Equal(t, now.Hour(), parsed.Hour(), "TimestampFormat lost precision (hour)")
	assert.Equal(t, now.Minute(), parsed.Minute(), "TimestampFormat lost precision (minute)")
	assert.Equal(t, now.Second(), parsed.Second(), "TimestampFormat lost precision (second)")
}

func TestPreviewCleanup(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Logger: slog.Default(),
	}

	// Act
	_, _, err := PreviewCleanup(30*24*time.Hour, 5, cfg)

	// Assert
	assert.NoError(t, err, "PreviewCleanup() should not error")
}

func TestCleanupCandidate(t *testing.T) {
	// Arrange
	candidate := CleanupCandidate{
		Path:      "/some/path",
		Timestamp: time.Now(),
		Size:      1024,
	}

	// Act - struct initialization is the action

	// Assert
	assert.Equal(t, "/some/path", candidate.Path, "CleanupCandidate.Path not set correctly")
	assert.Equal(t, int64(1024), candidate.Size, "CleanupCandidate.Size not set correctly")
}

func TestCreateBackup_LargeFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
	}

	sourceContent := make([]byte, 1024*1024)
	for i := range sourceContent {
		sourceContent[i] = byte(i % 256)
	}
	sourceFile := filepath.Join(tempDir, "large_source.txt")
	err := os.WriteFile(sourceFile, sourceContent, 0644)
	require.NoError(t, err, "failed to create source file")

	// Act
	backupPath, err := CreateBackup(sourceFile, cfg)

	// Assert
	require.NoError(t, err, "CreateBackup() should not error for large file")

	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err, "failed to read backup file")

	assert.Equal(t, len(sourceContent), len(backupContent), "CreateBackup() large file content length mismatch")
	assert.Equal(t, sourceContent, backupContent, "CreateBackup() large file content mismatch")
}

func TestRestoreBackup_OverwritesExisting(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
	}

	backupContent := []byte("backup content")
	backupFile := filepath.Join(tempDir, "backup.txt")
	err := os.WriteFile(backupFile, backupContent, 0644)
	require.NoError(t, err, "failed to create backup file")

	targetFile := filepath.Join(tempDir, "target.txt")
	err = os.WriteFile(targetFile, []byte("original content"), 0644)
	require.NoError(t, err, "failed to create target file")

	// Act
	err = RestoreBackup(backupFile, targetFile, cfg)

	// Assert
	require.NoError(t, err, "RestoreBackup() should not error")

	targetContent, err := os.ReadFile(targetFile)
	require.NoError(t, err, "failed to read target file")

	assert.Equal(t, string(backupContent), string(targetContent), "RestoreBackup() should overwrite existing file")
}
