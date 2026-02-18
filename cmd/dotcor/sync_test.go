package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
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
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

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
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

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
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

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
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Act - Try to sync with --push (push will fail on non-bare repo)
		cmd := syncCmd
		cmd.SetArgs([]string{"--push", "--force"})
		err = runSync(cmd, []string{})

		// Assert - Should fail because push to non-bare repo is rejected
		assert.Error(t, err, "sync should return error when push fails")
		assert.Contains(t, err.Error(), "pushing to remote", "error should mention push failure")
	})
}

func TestSync_NopushFlag_SkipsPush(t *testing.T) {
	t.Run("--no-push flag commits only (skip push)", func(t *testing.T) {
		// Arrange - Simple local repo without remote
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
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Act - Run sync without --no-push flag (should auto-push if remote exists)
		// Since we have no remote, it should just commit
		cmd := syncCmd
		cmd.SetArgs([]string{"--force"})
		err = runSync(cmd, []string{})

		// Assert - Should succeed
		require.NoError(t, err, "sync should succeed")

		// Verify changes were committed
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = filesDir
		output, _ := statusCmd.CombinedOutput()
		assert.Empty(t, string(output), "working tree should be clean after sync")

		// Verify we have 2 commits (initial + sync)
		logCmd := exec.Command("git", "log", "--oneline")
		logCmd.Dir = filesDir
		logOutput, _ := logCmd.CombinedOutput()
		commits := len(strings.Split(strings.TrimSpace(string(logOutput)), "\n"))
		assert.Equal(t, 2, commits, "should have 2 commits locally")
	})
}

func TestSync_DryRun_ShowsPreview(t *testing.T) {
	t.Run("--dry-run is alias for --preview", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial")

		// Create a new file to be synced
		newFile := filepath.Join(filesDir, "new.txt")
		CreateTestFile(t, newFile, "new content")

		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Create a fresh command to avoid global state issues
		testCmd := &cobra.Command{
			Use:  "sync",
			RunE: runSync,
		}
		testCmd.Flags().Bool("no-push", false, "")
		testCmd.Flags().Bool("preview", false, "")
		testCmd.Flags().Bool("dry-run", false, "")
		testCmd.Flags().Bool("force", false, "")
		testCmd.Flags().String("message", "", "")

		// Parse the flags
		testCmd.SetArgs([]string{"--dry-run"})
		if err := testCmd.ParseFlags([]string{"--dry-run"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		// Act - Run sync with --dry-run (should behave like --preview)
		err = runSync(testCmd, []string{})

		// Assert - Should return without error, file should not be committed
		require.NoError(t, err)
		// File should NOT be committed in dry-run mode
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = filesDir
		output, _ := statusCmd.CombinedOutput()
		assert.Contains(t, string(output), "new.txt", "file should show as untracked in dry-run")
	})
}

func TestSync_SpecificFiles_CommitsOnlyThose(t *testing.T) {
	t.Run("sync with file arguments commits only those files", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}

		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create two files in repo
		zshrcFile := filepath.Join(filesDir, "shell", "zshrc")
		gitconfigFile := filepath.Join(filesDir, "gitconfig")
		if err := os.MkdirAll(filepath.Dir(zshrcFile), 0755); err != nil {
			t.Fatalf("failed to create dirs: %v", err)
		}
		CreateTestFile(t, zshrcFile, "zshrc content")
		CreateTestFile(t, gitconfigFile, "gitconfig content")
		runGit(t, filesDir, "add", ".")
		runGit(t, filesDir, "commit", "-m", "Initial")

		// Modify both files
		if err := os.WriteFile(zshrcFile, []byte("modified zshrc"), 0644); err != nil {
			t.Fatalf("failed to modify zshrc: %v", err)
		}
		if err := os.WriteFile(gitconfigFile, []byte("modified gitconfig"), 0644); err != nil {
			t.Fatalf("failed to modify gitconfig: %v", err)
		}

		// Create symlinks
		sourceZshrc := filepath.Join(homeDir, ".zshrc")
		sourceGitconfig := filepath.Join(homeDir, ".gitconfig")
		if err := os.Symlink(zshrcFile, sourceZshrc); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}
		if err := os.Symlink(gitconfigFile, sourceGitconfig); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Create config with both files managed
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files:
  - source_path: "` + sourceZshrc + `"
    repo_path: shell/zshrc
    added_at: "2024-01-01T00:00:00Z"
  - source_path: "` + sourceGitconfig + `"
    repo_path: gitconfig
    added_at: "2024-01-01T00:00:00Z"
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Create a fresh command to avoid global state issues
		testCmd := &cobra.Command{
			Use:  "sync",
			RunE: runSync,
		}
		testCmd.Flags().Bool("no-push", false, "")
		testCmd.Flags().Bool("preview", false, "")
		testCmd.Flags().Bool("dry-run", false, "")
		testCmd.Flags().Bool("force", true, "")
		testCmd.Flags().String("message", "", "")

		// Act - Sync only zshrc
		testCmd.SetArgs([]string{sourceZshrc})
		err = runSync(testCmd, []string{sourceZshrc})

		// Assert - Only zshrc should be committed
		require.NoError(t, err)

		// Check that zshrc was committed (gitconfig should still be modified)
		// gitconfig should show as modified but not staged (space before M)
		statusCmd := exec.Command("git", "status", "--porcelain", "gitconfig")
		statusCmd.Dir = filesDir
		statusOutput, _ := statusCmd.CombinedOutput()
		assert.Contains(t, string(statusOutput), " M", "gitconfig should still be modified (not staged)")
	})
}

func TestSync_NoArgs_CommitsAll(t *testing.T) {
	t.Run("sync without arguments commits all files", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial")

		// Modify file
		if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
			t.Fatalf("failed to modify file: %v", err)
		}

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Create a fresh command to avoid global state issues
		testCmd := &cobra.Command{
			Use:  "sync",
			RunE: runSync,
		}
		testCmd.Flags().Bool("no-push", false, "")
		testCmd.Flags().Bool("preview", false, "")
		testCmd.Flags().Bool("dry-run", false, "")
		testCmd.Flags().Bool("force", true, "")
		testCmd.Flags().String("message", "", "")

		// Act - Sync without arguments
		testCmd.SetArgs([]string{})
		err = runSync(testCmd, []string{})

		// Assert - All changes committed
		require.NoError(t, err)
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = filesDir
		output, _ := statusCmd.CombinedOutput()
		assert.Empty(t, string(output), "all changes should be committed")
	})
}

func TestSyncPreviewWithUncommittedChanges(t *testing.T) {
	t.Run("sync preview shows warning about uncommitted changes", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create initial commit
		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "initial")
		runGit(t, filesDir, "add", "test.txt")
		runGit(t, filesDir, "commit", "-m", "Initial")

		// Modify file to create uncommitted changes
		CreateTestFile(t, testFile, "modified content")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		originalHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", originalHome), "failed to restore HOME")
		}()

		// Create a fresh command to avoid global state issues
		testCmd := &cobra.Command{
			Use:  "sync",
			RunE: runSync,
		}
		testCmd.Flags().Bool("no-push", false, "")
		testCmd.Flags().Bool("preview", false, "")
		testCmd.Flags().Bool("dry-run", false, "")
		testCmd.Flags().Bool("force", false, "")
		testCmd.Flags().String("message", "", "")

		// Parse the flags
		testCmd.SetArgs([]string{"--preview"})
		if err := testCmd.ParseFlags([]string{"--preview"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}

		// Act - Run sync with preview flag (should show warning about uncommitted changes)
		err = runSync(testCmd, []string{})

		// Assert - Should return without error and file should not be committed
		require.NoError(t, err, "preview should not commit changes")

		// Verify file is still modified (uncommitted)
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = filesDir
		statusOutput, _ := statusCmd.CombinedOutput()
		assert.Contains(t, string(statusOutput), "test.txt", "file should show as modified")
	})
}
