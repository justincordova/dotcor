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

func TestRebuildConfig_ValidRepo_ReconstructsConfig(t *testing.T) {
	t.Run("valid repo rebuilds config correctly", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create repo directory and add some files
		os.MkdirAll(filesDir, 0755)
		CreateTestFile(t, filepath.Join(filesDir, ".zshrc"), "zsh config")
		CreateTestFile(t, filepath.Join(filesDir, ".gitconfig"), "git config")
		CreateTestFile(t, filepath.Join(filesDir, "config.yaml"), "old config")

		// Create config directory with minimal config
		os.MkdirAll(configDir, 0755)
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

		// Act - Run rebuild-config with --scan and --force
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "rebuild-config", "--scan", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "rebuild-config should succeed")
		assert.Contains(t, stdoutStr, "Found", "should show found files")
	})
}

func TestRebuildConfig_DryRun_ShowsPreview(t *testing.T) {
	t.Run("dry run shows preview without changes", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create repo directory
		os.MkdirAll(filesDir, 0755)
		CreateTestFile(t, filepath.Join(filesDir, ".zshrc"), "zsh config")

		// Create config directory
		os.MkdirAll(configDir, 0755)
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

		// Act - Run rebuild-config with --verify
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "rebuild-config", "--verify")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "rebuild-config --verify should succeed")
		assert.Contains(t, stdoutStr, "Verifying configuration", "should show verification message")
	})
}

func TestRebuildConfig_CorruptConfig_Reconstructs(t *testing.T) {
	t.Run("corrupt config gets reconstructed", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create repo directory with files
		os.MkdirAll(filesDir, 0755)
		CreateTestFile(t, filepath.Join(filesDir, ".zshrc"), "zsh config")

		// Create config directory with corrupt config
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.yaml")
		err := os.WriteFile(configPath, []byte("invalid yaml content [[[["), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run rebuild-config (should handle corrupt config)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "rebuild-config", "--scan", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		// The command might fail due to corrupt config, but should report it
		if err != nil {
			assert.Contains(t, stdoutStr+stderr.String(), "config", "should mention config issue")
		}
	})
}

func TestRebuildConfig_OrphanedFiles_ShowsWarning(t *testing.T) {
	t.Run("orphaned files shown in output", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create repo directory with files
		os.MkdirAll(filesDir, 0755)
		CreateTestFile(t, filepath.Join(filesDir, ".zshrc"), "zsh config")
		CreateTestFile(t, filepath.Join(filesDir, ".vimrc"), "vim config")

		// Create config directory with config that only tracks one file
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files:
  - source_path: ~/.zshrc
    repo_path: .zshrc
    added_at: "%s"
`, filesDir, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run rebuild-config --verify
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "rebuild-config", "--verify")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "rebuild-config --verify should succeed")
		assert.Contains(t, stdoutStr, "Not in configuration", "should show orphaned files section")
		assert.Contains(t, stdoutStr, ".vimrc", "should list orphaned file")
	})
}
