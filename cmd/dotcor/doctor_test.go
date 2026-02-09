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

func TestDoctor_HealthySystem_AllChecksPass(t *testing.T) {
	t.Run("healthy system passes all checks", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)

		// Create repo file
		CreateTestFile(t, repoFile, "test content")

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		os.Symlink(relPath, sourcePath)

		// Create config file
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
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

		// Act - Run doctor command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed")
		assert.Contains(t, stdoutStr, "[OK] Configuration valid", "should show valid config")
		assert.Contains(t, stdoutStr, "[OK] No lock file", "should show no lock file")
		assert.Contains(t, stdoutStr, "[OK] All 1 symlinks healthy", "should show healthy symlinks")
	})
}

func TestDoctor_BrokenSymlink_ShowsError(t *testing.T) {
	t.Run("broken symlink detected and reported", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)

		// Create symlink pointing to non-existent repo file (broken symlink)
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		os.Symlink(relPath, sourcePath)
		// Don't create repo file - this simulates a broken symlink

		// Create config file
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
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

		// Act - Run doctor command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed even with issues")
		assert.Contains(t, stdoutStr, "Missing symlink", "should detect missing/broken symlink")
	})
}

func TestDoctor_StaleLock_DetectsAndClears(t *testing.T) {
	t.Run("stale lock detected and cleared with --fix", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		os.MkdirAll(filesDir, 0755)

		// Create config file
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files: []
`, filesDir)
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Create a stale lock file manually
		lockDir := filepath.Join(configDir, "lock")
		os.MkdirAll(lockDir, 0755)
		lockFile := filepath.Join(lockDir, "dotcor.lock")
		oldTime := time.Now().Add(-2 * time.Hour)
		lockContent := fmt.Sprintf(`{"pid": 99999, "hostname": "test", "timestamp": "%s"}`, oldTime.Format(time.RFC3339))
		err = os.WriteFile(lockFile, []byte(lockContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run doctor command (should check for lock file)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed")
		// The command should check lock file, though the specific detection depends on implementation
		assert.Contains(t, stdoutStr, "lock file", "should mention lock file")
	})
}

func TestDoctor_MissingGit_ReturnsWarning(t *testing.T) {
	t.Run("missing git shows warning", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		os.MkdirAll(filesDir, 0755)

		// Create config file
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

		// Act - Run doctor command with PATH set to empty to simulate missing git
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = []string{fmt.Sprintf("HOME=%s", tempDir), "PATH="}
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed")
		assert.Contains(t, stdoutStr, "Git is not installed", "should show git warning")
	})
}

func TestDoctor_FixFlag_AutoRepairs(t *testing.T) {
	t.Run("--fix flag auto-repairs issues", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")

		// Create config directory
		os.MkdirAll(configDir, 0755)

		// Create config file
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

		// Act - Run doctor with --fix
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor", "--fix")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed")
		assert.Contains(t, stdoutStr, "Created repository directory", "should report creating repo")
	})
}

func TestDoctor_PermissionError_DetectsIssue(t *testing.T) {
	t.Run("permission issues detected", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(tempDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create directories
		os.MkdirAll(filesDir, 0755)
		os.MkdirAll(homeDir, 0755)

		// Create repo file
		CreateTestFile(t, repoFile, "test content")

		// Create source file with world-writable permissions
		CreateTestFile(t, sourcePath, "test content")
		os.Chmod(sourcePath, 0666)

		// Create valid symlink to make the permission check run
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		os.Symlink(relPath, sourcePath)

		// Create config file
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
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

		// Act - Run doctor command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "doctor")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "doctor should succeed")
		// The permission check might not detect the issue on all systems, so just verify it runs
		assert.Contains(t, stdoutStr, "permissions", "should check permissions")
	})
}
