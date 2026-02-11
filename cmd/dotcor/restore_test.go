package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
)

func TestRestore_Head_RestoresLatest(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfigWithFileHistory(t, tempDir)

	repoFile := filepath.Join(cfg.RepoPath, "test.txt")

	// Create second commit with different content
	modifiedContent := "modified content"
	err := os.WriteFile(repoFile, []byte(modifiedContent), 0644)
	require.NoError(t, err, "failed to modify file")

	runGit(t, cfg.RepoPath, "add", "test.txt")
	runGit(t, cfg.RepoPath, "commit", "-m", "Second commit")

	// Modify file again to create working tree change
	currentContent := "current working tree content"
	err = os.WriteFile(repoFile, []byte(currentContent), 0644)
	require.NoError(t, err, "failed to create working tree change")

	// Act
	err = restoreFromGit(cfg.RepoPath, "test.txt", repoFile, "HEAD", false, true, cfg)
	require.NoError(t, err, "restore should succeed")

	// Assert
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err, "should read restored file")
	assert.Equal(t, modifiedContent, string(content), "file should be restored to HEAD version")
}

func TestRestore_SpecificCommit_RestoresCorrectVersion(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfigWithFileHistory(t, tempDir)

	repoFile := filepath.Join(cfg.RepoPath, "test.txt")

	// Create second commit with different content
	modifiedContent := "modified content"
	err := os.WriteFile(repoFile, []byte(modifiedContent), 0644)
	require.NoError(t, err, "failed to modify file")

	runGit(t, cfg.RepoPath, "add", "test.txt")
	runGit(t, cfg.RepoPath, "commit", "-m", "Second commit")

	// Get the first commit hash
	output, err := exec.Command("git", "-C", cfg.RepoPath, "rev-parse", "HEAD~1").CombinedOutput()
	require.NoError(t, err, "failed to get commit hash")
	firstCommitHash := strings.TrimSpace(string(output))

	// Modify file to create working tree change
	currentContent := "current working tree content"
	err = os.WriteFile(repoFile, []byte(currentContent), 0644)
	require.NoError(t, err, "failed to create working tree change")

	// Act - restore to first commit
	err = restoreFromGit(cfg.RepoPath, "test.txt", repoFile, firstCommitHash, false, true, cfg)
	require.NoError(t, err, "restore should succeed")

	// Assert
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err, "should read restored file")
	assert.Equal(t, "initial content", string(content), "file should be restored to first commit version")
}

func TestRestore_PreviewFlag_ShowsDiff(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfigWithFileHistory(t, tempDir)

	repoFile := filepath.Join(cfg.RepoPath, "test.txt")

	// Create second commit
	modifiedContent := "modified content"
	err := os.WriteFile(repoFile, []byte(modifiedContent), 0644)
	require.NoError(t, err, "failed to modify file")

	runGit(t, cfg.RepoPath, "add", "test.txt")
	runGit(t, cfg.RepoPath, "commit", "-m", "Second commit")

	// Modify file in working tree
	err = os.WriteFile(repoFile, []byte("current content"), 0644)
	require.NoError(t, err, "failed to modify file")

	// Act - preview mode
	err = restoreFromGit(cfg.RepoPath, "test.txt", repoFile, "HEAD", true, true, cfg)
	require.NoError(t, err, "preview should succeed")

	// Assert - file should not be changed in preview mode
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err, "should read file")
	assert.Equal(t, "current content", string(content), "file should not be changed in preview mode")
}

func TestRestore_NonexistentFile_ReturnsError(t *testing.T) {
	// Arrange
	cfg := CreateTestConfig(t)

	repoFile := filepath.Join(cfg.RepoPath, "test.txt")

	// Act
	err := restoreFromGit(cfg.RepoPath, "test.txt", repoFile, "HEAD", false, true, cfg)

	// Assert
	assert.Error(t, err, "should return error for nonexistent file")
}

func TestRestore_BackupCreated_RestorePointAvailable(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfigWithFileHistory(t, tempDir)

	repoFile := filepath.Join(cfg.RepoPath, "test.txt")

	// Create second commit
	modifiedContent := "modified content"
	err := os.WriteFile(repoFile, []byte(modifiedContent), 0644)
	require.NoError(t, err, "failed to modify file")

	runGit(t, cfg.RepoPath, "add", "test.txt")
	runGit(t, cfg.RepoPath, "commit", "-m", "Second commit")

	// Modify file in working tree to create different state
	currentContent := "current working tree content"
	err = os.WriteFile(repoFile, []byte(currentContent), 0644)
	require.NoError(t, err, "failed to modify file")

	// Act - restore from HEAD (non-preview mode)
	err = restoreFromGit(cfg.RepoPath, "test.txt", repoFile, "HEAD", false, true, cfg)
	require.NoError(t, err, "restore should succeed")

	// Assert - backup should be created
	// Since ListBackups has issues with nested directory structures,
	// we'll verify backup creation by checking that the backup file exists
	backupDir, err := core.GetBackupDir()
	require.NoError(t, err, "should get backup directory")

	// Walk backup directory to find a backup file
	backupFound := false
	var backupPath string
	if err := filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(path, "test.txt") {
			backupFound = true
			backupPath = path
			return filepath.SkipAll
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to walk backup directory: %v", err)
	}

	require.True(t, backupFound, "backup file should exist")

	// Verify backup contains original content
	backupContent, err := os.ReadFile(backupPath)
	require.NoError(t, err, "should read backup")
	assert.Equal(t, currentContent, string(backupContent), "backup should contain original content")
}

// CreateTestConfigWithFileHistory creates a test config with git repo and file history
func CreateTestConfigWithFileHistory(t *testing.T, dir string) *config.Config {
	t.Helper()

	filesDir := filepath.Join(dir, ".dotcor", "files")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create directory and initialize git
	os.MkdirAll(filesDir, 0755)
	runGit(t, filesDir, "init")
	runGit(t, filesDir, "config", "user.email", "test@example.com")
	runGit(t, filesDir, "config", "user.name", "Test User")
	runGit(t, filesDir, "checkout", "-b", "main")

	// Create initial commit with test file
	testFile := filepath.Join(filesDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	runGit(t, filesDir, "add", "test.txt")
	runGit(t, filesDir, "commit", "-m", "Initial commit")

	return &config.Config{
		Logger:         logger,
		RepoPath:       filesDir,
		GitEnabled:     true,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}
}
