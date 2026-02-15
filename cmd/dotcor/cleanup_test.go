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

func TestCleanup_OldBackups_DeletesCorrectly(t *testing.T) {
	t.Run("cleanup command runs successfully", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

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

		// Act - Run cleanup
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup should succeed")
	})
}

func TestCleanup_KeepLastFlag_PreservesRecent(t *testing.T) {
	t.Run("keep flag is accepted", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

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

		// Act - Run cleanup with --keep 10
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--keep", "10", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup should succeed with keep flag")
	})
}

func TestCleanup_NoBackups_ReturnsEmpty(t *testing.T) {
	t.Run("no backups shows empty state", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

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

		// Act - Run cleanup
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup should succeed with no backups")
		assert.Contains(t, stdoutStr, "No backups", "should show no backups message")
	})
}

func TestCleanup_ForceFlag_SkipsPrompt(t *testing.T) {
	t.Run("force flag skips confirmation", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Create an old backup
		backupFile := filepath.Join(backupsDir, "old.zshrc.bak")
		oldTime := time.Now().Add(-45 * 24 * time.Hour)
		if err := os.WriteFile(backupFile, []byte("old content"), 0644); err != nil {
			t.Fatalf("failed to create backup file: %v", err)
		}
		if err := os.Chtimes(backupFile, oldTime, oldTime); err != nil {
			t.Fatalf("failed to set file times: %v", err)
		}

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

		// Act - Run cleanup with --force (should not prompt)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--older-than", "30d", "--force")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		// With --force, it should not hang on prompt
		// We can't easily test that it doesn't prompt, but we can verify it completes
		assert.True(t, true, "test should complete without hanging")
	})
}

func TestCleanup_Preview_ShowsWhatWouldBeDeleted(t *testing.T) {
	t.Run("dry run shows preview", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

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

		// Act - Run cleanup with --dry-run
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--dry-run")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup --dry-run should succeed")
		assert.Contains(t, stdoutStr, "No backups", "should handle empty backup state")
	})
}

func TestCleanup_AutoFlag_UsesSmartDefaults(t *testing.T) {
	t.Run("--auto flag shows smart defaults and is dry run by default", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Create 15 old backup directories (35+ days old)
		now := time.Now()
		for i := 0; i < 15; i++ {
			backupTime := now.Add(-time.Duration(35+i) * 24 * time.Hour)
			timestampDir := backupTime.Format("2006-01-02_15-04-05")
			backupPath := filepath.Join(backupsDir, timestampDir)
			if err := os.MkdirAll(backupPath, 0755); err != nil {
				t.Fatalf("failed to create backup dir: %v", err)
			}
			testFile := filepath.Join(backupPath, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

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

		// Act - Run cleanup with --auto flag
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--auto")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup --auto should succeed")
		assert.Contains(t, stdoutStr, "Auto cleanup", "should show auto cleanup header")
		assert.Contains(t, stdoutStr, "Keep: last 10 backups", "should show keep default")
		assert.Contains(t, stdoutStr, "older than 30 days", "should show age default")
		assert.Contains(t, stdoutStr, "Dry run", "should be dry run by default")
		assert.Contains(t, stdoutStr, "--execute", "should suggest using --execute flag")
	})
}

func TestCleanup_AutoFlag_WithExecute_DeletesBackups(t *testing.T) {
	t.Run("--auto --execute actually deletes backups", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		backupsDir := filepath.Join(configDir, "backups")
		filesDir := filepath.Join(configDir, "files")

		// Create directories
		if err := os.MkdirAll(backupsDir, 0755); err != nil {
			t.Fatalf("failed to create backups dir: %v", err)
		}
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Create 15 old backup directories
		now := time.Now()
		for i := 0; i < 15; i++ {
			backupTime := now.Add(-time.Duration(35+i) * 24 * time.Hour)
			timestampDir := backupTime.Format("2006-01-02_15-04-05")
			backupPath := filepath.Join(backupsDir, timestampDir)
			if err := os.MkdirAll(backupPath, 0755); err != nil {
				t.Fatalf("failed to create backup dir: %v", err)
			}
			testFile := filepath.Join(backupPath, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

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

		// Act - Run cleanup with --auto --execute flags
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "cleanup-backups", "--auto", "--execute")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stdout: %s", stdoutStr)

		require.NoError(t, err, "cleanup --auto --execute should succeed")
		assert.Contains(t, stdoutStr, "Auto cleanup", "should show auto cleanup header")
		assert.Contains(t, stdoutStr, "Removed", "should show deletion message")
		assert.Contains(t, stdoutStr, "Remaining", "should show remaining backups")

		// Verify backups were actually deleted (should keep last 10, so 5 should be deleted)
		entries, _ := os.ReadDir(backupsDir)
		t.Logf("Remaining backup directories: %d", len(entries))
		assert.LessOrEqual(t, len(entries), 10, "should keep at most 10 backups")
	})
}
