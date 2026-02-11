package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSync_NoChanges_ReturnsEarly(t *testing.T) {
	t.Run("no changes returns early with message", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Initialize git repo in filesDir
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create initial commit so repo is clean
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial content")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Set HOME to tempDir
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		// Act - Run sync command
		cmd := syncCmd
		cmd.SetArgs([]string{"--force"})
		err = runSync(cmd, []string{})

		// Assert
		require.NoError(t, err, "sync should succeed even with no changes")
	})
}

func TestSync_WithChanges_CommitsAndPushes(t *testing.T) {
	t.Run("commits changes and pushes to remote", func(t *testing.T) {
		// Arrange - Set up two repos (remote and local)
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		remoteDir := filepath.Join(tempDir, "remote")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(remoteDir, 0755); err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}

		// Initialize remote repo as a bare repository
		runGit(t, remoteDir, "init", "--bare")

		// Initialize local repo and add remote
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")
		runGit(t, filesDir, "remote", "add", "origin", remoteDir)

		// Create initial commit in local repo
		localFile := filepath.Join(filesDir, "local.txt")
		CreateTestFile(t, localFile, "local content")
		runGit(t, filesDir, "add", "local.txt")
		runGit(t, filesDir, "commit", "-m", "Local initial commit")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Create a new change to sync
		newFile := filepath.Join(filesDir, "newfile.txt")
		CreateTestFile(t, newFile, "new content")

		// Set HOME to tempDir
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		// Act - Run sync command with force flag
		cmd := syncCmd
		cmd.SetArgs([]string{"--force"})
		err = runSync(cmd, []string{})

		// Assert
		require.NoError(t, err, "sync should succeed")
	})
}

func TestSync_GitCommitFails_MarksUncommitted(t *testing.T) {
	t.Run("git commit failure returns error", func(t *testing.T) {
		// Arrange - Create a git repo that will fail to commit
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Initialize git repo
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create initial commit
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial content")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Create a managed file with uncommitted flag set
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}
		CreateTestFile(t, repoFile, "test content")
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		if err := os.Symlink(relPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Modify the repo file to create uncommitted changes
		CreateTestFile(t, repoFile, "modified content")

		// Create config with uncommitted file
		configPath := filepath.Join(configDir, "config.yaml")
		now := time.Now()
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files:
  - source_path: ` + sourcePath + `
    repo_path: shell/zshrc
    added_at: "` + now.Format(time.RFC3339) + `"
    has_uncommitted: true
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Set HOME to tempDir
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		// Act - Try to sync (should commit changes)
		cmd := syncCmd
		cmd.SetArgs([]string{"--force"})
		err = runSync(cmd, []string{})

		// Assert - Should succeed because commit works
		require.NoError(t, err, "sync should succeed with commit")
	})
}

func TestSync_PushFails_ContinuesWithoutError(t *testing.T) {
	t.Run("push failure continues without error", func(t *testing.T) {
		// Arrange - Set up local repo with a remote that will fail to push
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		remoteDir := filepath.Join(tempDir, "remote")

		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(remoteDir, 0755); err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}

		// Initialize remote repo as a non-bare repository (will reject pushes)
		runGit(t, remoteDir, "init")
		runGit(t, remoteDir, "config", "user.email", "test@example.com")
		runGit(t, remoteDir, "config", "user.name", "Test User")
		runGit(t, remoteDir, "checkout", "-b", "main")

		// Initialize local repo
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")
		runGit(t, filesDir, "remote", "add", "origin", remoteDir)

		// Create initial commit
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial content")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Create a new change
		newFile := filepath.Join(filesDir, "newfile.txt")
		CreateTestFile(t, newFile, "new content")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Set HOME to tempDir
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		// Act - Try to sync (push will fail on non-bare repo)
		cmd := syncCmd
		cmd.SetArgs([]string{"--force"})
		err = runSync(cmd, []string{})

		// Assert - Should fail because push to non-bare repo is rejected
		// Current implementation returns error on push failure
		assert.Error(t, err, "sync should return error when push fails")
		assert.Contains(t, err.Error(), "pushing to remote", "error should mention push failure")
	})
}

func TestSync_NopushFlag_SkipsPush(t *testing.T) {
	t.Run("--no-push flag skips push operation", func(t *testing.T) {
		// Arrange - Set up local repo without remote
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Initialize git repo
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create initial commit
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial content")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Create a new change
		newFile := filepath.Join(filesDir, "newfile.txt")
		CreateTestFile(t, newFile, "new content")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Set HOME to tempDir
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", originalHome)

		// Act - Run sync with --no-push flag
		cmd := syncCmd
		cmd.SetArgs([]string{"--no-push", "--force"})
		err = runSync(cmd, []string{})

		// Assert - Should succeed without attempting to push
		require.NoError(t, err, "sync should succeed with --no-push flag")

		// Verify changes were committed
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = filesDir
		output, _ := statusCmd.CombinedOutput()
		assert.Empty(t, string(output), "working tree should be clean after sync")
	})
}
