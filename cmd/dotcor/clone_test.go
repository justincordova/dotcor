package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClone_ValidURL_ClonesAndInitializes(t *testing.T) {
	t.Run("valid URL clones repository", func(t *testing.T) {
		// Arrange - Create a local git repo to clone from
		remoteDir := t.TempDir()
		homeDir := t.TempDir()

		runGit(t, remoteDir, "init")
		runGit(t, remoteDir, "config", "user.email", "test@example.com")
		runGit(t, remoteDir, "config", "user.name", "Test User")
		runGit(t, remoteDir, "checkout", "-b", "main")

		// Add a file to the remote
		remoteFile := filepath.Join(remoteDir, "test.txt")
		CreateTestFile(t, remoteFile, "test content")
		runGit(t, remoteDir, "add", "test.txt")
		runGit(t, remoteDir, "commit", "-m", "Initial commit")

		// Build dotcor binary
		buildPath := filepath.Join(homeDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run clone command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "clone", remoteDir, "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", homeDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)
		t.Logf("stderr: %s", stderrStr)

		require.NoError(t, err, "clone should succeed")
		assert.Contains(t, stdoutStr, "Cloning repository", "should show cloning message")
		assert.Contains(t, stdoutStr, "Clone complete", "should show completion message")
	})
}

func TestClone_InvalidURL_ReturnsError(t *testing.T) {
	t.Run("invalid URL returns error", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()

		// Build dotcor binary
		buildPath := filepath.Join(homeDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run clone with invalid URL
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "clone", "http://invalid-url-that-does-not-exist.example.com/repo.git")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", homeDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)
		t.Logf("stderr: %s", stderrStr)

		// Clone should fail with invalid URL
		assert.Error(t, err, "clone should fail with invalid URL")
		assert.Contains(t, stdoutStr+stderrStr, "cloning repository", "should mention cloning error")
	})
}

func TestClone_DirectoryExists_ReturnsError(t *testing.T) {
	t.Run("existing directory handled correctly", func(t *testing.T) {
		// Arrange
		remoteDir := t.TempDir()
		homeDir := t.TempDir()

		// Create existing .dotcor directory
		configDir := filepath.Join(homeDir, ".dotcor")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}
		existingFile := filepath.Join(configDir, "existing.txt")
		CreateTestFile(t, existingFile, "existing content")

		// Create a local git repo to clone from
		runGit(t, remoteDir, "init")
		runGit(t, remoteDir, "config", "user.email", "test@example.com")
		runGit(t, remoteDir, "config", "user.name", "Test User")
		runGit(t, remoteDir, "checkout", "-b", "main")

		// Build dotcor binary
		buildPath := filepath.Join(homeDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run clone command with --force to handle existing dir
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "clone", remoteDir, "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", homeDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)
		t.Logf("stderr: %s", stderrStr)

		// With --force, it should succeed and overwrite
		if err == nil {
			assert.Contains(t, stdoutStr, "Removing existing", "should mention removing existing directory")
		} else {
			// Or it should report the existing directory
			assert.Contains(t, stdoutStr+stderrStr, "exists", "should mention existing directory")
		}
	})
}

func TestClone_NoGitInstalled_ReturnsError(t *testing.T) {
	t.Run("no git shows helpful error", func(t *testing.T) {
		// Arrange
		homeDir := t.TempDir()

		// Build dotcor binary
		buildPath := filepath.Join(homeDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run clone with PATH set to empty (no git)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "clone", "https://github.com/user/dotfiles.git")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = []string{fmt.Sprintf("HOME=%s", homeDir), "PATH="}
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		stderrStr := stderr.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)
		t.Logf("stderr: %s", stderrStr)

		// Should fail when git is not available
		assert.Error(t, err, "clone should fail without git")
		assert.Contains(t, stdoutStr+stderrStr, "git", "should mention git in error")
	})
}
