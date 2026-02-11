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

func TestDiff_NoArguments_ShowsAllChanges(t *testing.T) {
	t.Run("shows all uncommitted changes when no file specified", func(t *testing.T) {
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

		// Create and commit file
		CreateTestFile(t, repoFile, "original content")
		runGit(t, filesDir, "add", "shell/zshrc")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Modify file (uncommitted change)
		CreateTestFile(t, repoFile, "modified content")

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

		// Act - Run diff without arguments
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "diff")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "-original content")
		assert.Contains(t, outputStr, "+modified content")
	})
}

func TestDiff_WithFileArgument_ShowsFileDiff(t *testing.T) {
	t.Run("shows diff only for specified file", func(t *testing.T) {
		if !gitIsAvailable() {
			t.Skip("git not available")
		}

		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath1 := filepath.Join(homeDir, ".zshrc")
		repoFile1 := filepath.Join(filesDir, "shell", "zshrc")
		sourcePath2 := filepath.Join(homeDir, ".bashrc")
		repoFile2 := filepath.Join(filesDir, "shell", "bashrc")

		// Create directories and init git
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

		// Create and commit both files
		CreateTestFile(t, repoFile1, "zshrc content")
		runGit(t, filesDir, "add", "shell/zshrc")
		CreateTestFile(t, repoFile2, "bashrc content")
		runGit(t, filesDir, "add", "shell/bashrc")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Modify both files
		CreateTestFile(t, repoFile1, "modified zshrc")
		CreateTestFile(t, repoFile2, "modified bashrc")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
  - source_path: %s
    repo_path: shell/bashrc
    added_at: "%s"
`, filesDir, sourcePath1, time.Now().Format(time.RFC3339), sourcePath2, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run diff for specific file
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "diff", sourcePath1)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "-zshrc content")
		assert.Contains(t, outputStr, "+modified zshrc")
		assert.NotContains(t, outputStr, "bashrc")
	})
}

func TestDiff_StatFlag_ShowsSummary(t *testing.T) {
	t.Run("shows diffstat summary with --stat flag", func(t *testing.T) {
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

		// Create and commit file
		CreateTestFile(t, repoFile, "original")
		runGit(t, filesDir, "add", "shell/zshrc")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

		// Modify file
		CreateTestFile(t, repoFile, "modified content here")

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

		// Act - Run diff with --stat
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "diff", "--stat")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "shell/zshrc")
	})
}

func TestDiff_NoChanges_ShowsNothing(t *testing.T) {
	t.Run("shows no changes message when working tree is clean", func(t *testing.T) {
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

		// Create and commit file (clean working tree)
		CreateTestFile(t, repoFile, "content")
		runGit(t, filesDir, "add", "shell/zshrc")
		runGit(t, filesDir, "commit", "-m", "Initial commit")

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

		// Act - Run diff
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "diff")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "No uncommitted changes")
	})
}
