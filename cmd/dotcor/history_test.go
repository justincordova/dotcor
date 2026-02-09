package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_SingleFile_ShowsCommits(t *testing.T) {
	t.Run("shows git history for a managed file", func(t *testing.T) {
		if !gitIsAvailable() {
			t.Skip("git not available")
		}

		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories and init git
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create repo file and make multiple commits
		CreateTestFile(t, repoFile, "initial content")
		runGit(t, filesDir, "add", "shell/zshrc")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		CreateTestFile(t, repoFile, "second content")
		runGit(t, filesDir, "add", "shell/zshrc")
		runGit(t, filesDir, "commit", "-m", "Second commit")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
`, filesDir, sourcePath, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run history command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "history", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "History for")
		assert.Contains(t, outputStr, "commit")
		assert.Contains(t, outputStr, "Author:")
		assert.Contains(t, outputStr, "Second commit")
		assert.Contains(t, outputStr, "Initial commit")
	})
}

func TestHistory_LimitFlag_ShowsSpecifiedCount(t *testing.T) {
	t.Run("shows only specified number of commits with -n flag", func(t *testing.T) {
		if !gitIsAvailable() {
			t.Skip("git not available")
		}

		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories and init git
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create 5 commits
		for i := 1; i <= 5; i++ {
			CreateTestFile(t, repoFile, fmt.Sprintf("content %d", i))
			runGit(t, filesDir, "add", "shell/zshrc")
			runGit(t, filesDir, "commit", "-m", fmt.Sprintf("Commit %d", i))
		}

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
`, filesDir, sourcePath, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run history with limit of 2
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "history", "-n", "2", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		// Git log shows most recent commits first, so with -n 2 we see commits 5 and 4
		assert.Contains(t, outputStr, "Commit 4")
		assert.Contains(t, outputStr, "Commit 5")
		assert.NotContains(t, outputStr, "Commit 1")
		assert.NotContains(t, outputStr, "Commit 2")
		assert.NotContains(t, outputStr, "Commit 3")
	})
}

func TestHistory_UnmanagedFile_ReturnsError(t *testing.T) {
	t.Run("returns error for file not in managed list", func(t *testing.T) {
		if !gitIsAvailable() {
			t.Skip("git not available")
		}

		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")

		// Create directories and init git
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create empty config (no managed files)
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files: []
`, filesDir)
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run history for unmanaged file
		var stdout, stderr bytes.Buffer
		unmanagedFile := filepath.Join(homeDir, ".bashrc")
		cmd := exec.Command(buildPath, "history", unmanagedFile)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.Error(t, err)
		stderrStr := stderr.String()
		assert.Contains(t, stderrStr, "file not managed")
	})
}

func TestHistory_NoHistory_EmptyOutput(t *testing.T) {
	t.Run("shows no commits message for file with no history", func(t *testing.T) {
		if !gitIsAvailable() {
			t.Skip("git not available")
		}

		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories and init git
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)
		runGit(t, filesDir, "init")
		runGit(t, filesDir, "config", "user.email", "test@example.com")
		runGit(t, filesDir, "config", "user.name", "Test User")
		runGit(t, filesDir, "checkout", "-b", "main")

		// Create and commit a different file (so repo has commits but not for our file)
		otherFile := filepath.Join(filesDir, "other.txt")
		CreateTestFile(t, otherFile, "other content")
		runGit(t, filesDir, "add", "other.txt")
		runGit(t, filesDir, "commit", "-m", "Other commit")

		// Create our file but don't commit it
		CreateTestFile(t, repoFile, "content")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
`, filesDir, sourcePath, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run history command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "history", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "No commits found")
	})
}

func gitIsAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
