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

func TestAdopt_ValidSymlink_AdoptsFile(t *testing.T) {
	t.Run("adopts valid symlink pointing to repo file", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}

		// Create repo file
		CreateTestFile(t, repoFile, "repo content")

		// Create valid symlink to repo file
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		err := os.Symlink(relPath, sourcePath)
		require.NoError(t, err)

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files: []
`, filesDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run adopt command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "adopt", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "[OK]")
		assert.Contains(t, outputStr, "Adopted 1 symlink(s)")

		// Verify file is now in config
		configData, err := os.ReadFile(configPath)
		require.NoError(t, err)
		configStr := string(configData)
		assert.Contains(t, configStr, "managed_files:")
		assert.Contains(t, configStr, ".zshrc")
	})
}

func TestAdopt_Nonsymlink_ReturnsError(t *testing.T) {
	t.Run("returns error for regular file (not a symlink)", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}

		// Create regular file (not a symlink)
		CreateTestFile(t, sourcePath, "regular file content")

		// Create config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
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

		// Act - Run adopt command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "adopt", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		// Command should succeed but file should be skipped
		assert.Contains(t, stdoutStr, "skipped")
		assert.Contains(t, stderrStr, "not a symlink")
	})
}

func TestAdopt_AlreadyManaged_ReturnsError(t *testing.T) {
	t.Run("returns error for file already in managed list", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}

		// Create repo file
		CreateTestFile(t, repoFile, "repo content")

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		err := os.Symlink(relPath, sourcePath)
		require.NoError(t, err)

		// Create config with file already managed
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
`, filesDir, sourcePath, time.Now().Format(time.RFC3339))
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run adopt command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "adopt", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "already managed")
		assert.Contains(t, outputStr, "skipped")
	})
}

func TestAdopt_PointingToRepo_ReturnsError(t *testing.T) {
	t.Run("accepts symlink pointing inside dotcor repo", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}

		// Create repo file
		CreateTestFile(t, repoFile, "repo content")

		// Create symlink pointing to repo file (this should succeed)
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		err := os.Symlink(relPath, sourcePath)
		require.NoError(t, err)

		// Create empty config
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files: []
`, filesDir)
		err = os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run adopt command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "adopt", sourcePath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err)
		outputStr := stdout.String()
		// Should successfully adopt since it points to repo
		assert.Contains(t, outputStr, "[OK]")
		assert.Contains(t, outputStr, "Adopted 1 symlink(s)")
	})
}
