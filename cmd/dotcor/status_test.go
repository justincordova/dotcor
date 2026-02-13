package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_NotInitialized_ReturnsError(t *testing.T) {
	t.Run("no config returns error", func(t *testing.T) {
		// Arrange - Create temp directory without config
		tempDir := t.TempDir()

		// Build dotcor binary (build with original HOME, not tempDir)
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Run status command without init
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = []string{"HOME=" + tempDir, "PATH=" + os.Getenv("PATH")}
		err = cmd.Run()

		// Assert
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		t.Logf("Command exit code: %v", err)
		t.Logf("stderr: %s", stderrStr)
		t.Logf("stdout: %s", stdoutStr)

		require.Error(t, err, "status should fail when not initialized")
		assert.Contains(t, stderrStr, "config file not found", "should mention config file missing")
		assert.Contains(t, stderrStr, "dotcor init", "should suggest init command")
	})
}

func TestStatus_ValidSymlink_ShowsOK(t *testing.T) {
	t.Run("valid symlink shows ok status", func(t *testing.T) {
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

		// Create repo file
		CreateTestFile(t, repoFile, "test content")

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		if err := os.Symlink(relPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

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

		// Act - Run status command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		if err != nil {
			t.Logf("Command failed with exit code: %v", err)
			t.Logf("stderr: %s", stderrStr)
			t.Logf("stdout: %s", stdoutStr)
		}
		require.NoError(t, err, "status command should succeed")
		assert.Contains(t, stdoutStr, "files managed", "should show file count")
		assert.Contains(t, stdoutStr, ".zshrc", "should show managed file")
	})
}

func TestStatus_BrokenSymlink_ShowsError(t *testing.T) {
	t.Run("missing repo file shows error status", func(t *testing.T) {
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

		// Create symlink pointing to non-existent repo file
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		if err := os.Symlink(relPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}
		// Don't create repo file - this simulates missing repo

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

		// Act - Run status command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		if err != nil {
			t.Logf("Command failed with exit code: %v", err)
			t.Logf("stderr: %s", stderrStr)
			t.Logf("stdout: %s", stdoutStr)
		}
		require.NoError(t, err, "status command should succeed")
		assert.Contains(t, stdoutStr, "files managed", "should show file count")
		assert.Contains(t, stdoutStr, "with issues", "should show issues count")
		assert.Contains(t, stdoutStr, ".zshrc", "should show problematic file")
		assert.Contains(t, stdoutStr, "missing from repository", "should show missing-repo problem")
	})
}

func TestStatus_UncommittedChanges_ShowsWarning(t *testing.T) {
	t.Run("uncommitted files show in output", func(t *testing.T) {
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

		// Create repo file
		CreateTestFile(t, repoFile, "test content")

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		if err := os.Symlink(relPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Create config file with uncommitted files
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: false
managed_files:
  - source_path: %s
    repo_path: shell/zshrc
    added_at: "%s"
    has_uncommitted: true
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

		// Act - Run status command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		stderrStr := stderr.String()
		stdoutStr := stdout.String()
		if err != nil {
			t.Logf("Command failed with exit code: %v", err)
			t.Logf("stderr: %s", stderrStr)
			t.Logf("stdout: %s", stdoutStr)
		}
		require.NoError(t, err, "status command should succeed")
		assert.Contains(t, stdoutStr, "Uncommitted Files:", "should show uncommitted files section")
		assert.Contains(t, stdoutStr, ".zshrc", "should list uncommitted file")
		assert.Contains(t, stdoutStr, "dotcor sync", "should suggest sync command")
	})
}

func TestStatus_GitAheadBehind_ShowsCounts(t *testing.T) {
	t.Run("git status shows ahead/behind counts", func(t *testing.T) {
		if !git.IsGitInstalled() {
			t.Skip("git not installed, skipping test")
		}

		// Arrange - Create two git repos to simulate ahead/behind
		tempDir := t.TempDir()
		remoteDir := filepath.Join(tempDir, "remote")
		localDir := filepath.Join(tempDir, "local")
		configDir := filepath.Join(localDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		homeDir := filepath.Join(localDir, "home")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(filesDir, "shell", "zshrc")

		// Create remote repo
		if err := os.MkdirAll(remoteDir, 0755); err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}
		runGit(t, remoteDir, "init")
		runGit(t, remoteDir, "config", "user.email", "test@example.com")
		runGit(t, remoteDir, "config", "user.name", "Test User")
		runGit(t, remoteDir, "checkout", "-b", "main")

		// Create local repo and add remote
		if err := os.MkdirAll(localDir, 0755); err != nil {
			t.Fatalf("failed to create local dir: %v", err)
		}
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
		runGit(t, filesDir, "remote", "add", "origin", remoteDir)

		// Create a file and commit in remote
		remoteFile := filepath.Join(remoteDir, "test.txt")
		CreateTestFile(t, remoteFile, "remote content")
		runGit(t, remoteDir, "add", "test.txt")
		runGit(t, remoteDir, "commit", "-m", "Initial commit")

		// Create an initial commit in local repo so we can set up tracking
		localFile := filepath.Join(filesDir, "local.txt")
		CreateTestFile(t, localFile, "local content")
		runGit(t, filesDir, "add", "local.txt")
		runGit(t, filesDir, "commit", "-m", "Local initial commit")
		// Fetch in local repo and set up tracking
		runGit(t, filesDir, "fetch", "origin")
		runGit(t, filesDir, "branch", "--set-upstream-to=origin/main", "main")
		// Create managed file in local repo
		CreateTestFile(t, repoFile, "test content")
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		if err := os.Symlink(relPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Fetch from remote so we can be behind
		runGit(t, filesDir, "fetch", "origin")

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

		// Test 1: Behind by 1 commit (local needs to pull)
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", localDir))
		err = cmd.Run()
		if err != nil {
			t.Logf("Test 1 failed with exit code: %v", err)
			t.Logf("Test 1 stderr: %s", stderr.String())
			t.Logf("Test 1 stdout: %s", stdout.String())
		}
		require.NoError(t, err)
		outputStr := stdout.String()

		// Should show behind status
		assert.Contains(t, outputStr, "behind remote", "should show behind status")

		// Test 2: Ahead by 1 commit (local needs to push)
		// Commit in local repo
		runGit(t, filesDir, "add", ".")
		runGit(t, filesDir, "commit", "-m", "Local commit")

		// Run status again
		stdout.Reset()
		cmd = exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", localDir))
		err = cmd.Run()
		if err != nil {
			t.Logf("Test 2 failed with exit code: %v", err)
			t.Logf("Test 2 stderr: %s", stderr.String())
			t.Logf("Test 2 stdout: %s", stdout.String())
		}
		require.NoError(t, err)
		outputStr = stdout.String()

		// Should show ahead status
		assert.Contains(t, outputStr, "ahead of remote", "should show ahead status")
	})
}

func TestStatus_SpecificFiles_ShowsOnlyThose(t *testing.T) {
	t.Run("status with file arguments shows only those files", func(t *testing.T) {
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

		// Create two managed files
		zshrcFile := filepath.Join(filesDir, "shell", "zshrc")
		gitconfigFile := filepath.Join(filesDir, "gitconfig")
		if err := os.MkdirAll(filepath.Dir(zshrcFile), 0755); err != nil {
			t.Fatalf("failed to create dirs: %v", err)
		}
		CreateTestFile(t, zshrcFile, "zshrc content")
		CreateTestFile(t, gitconfigFile, "gitconfig content")

		// Create symlinks
		sourceZshrc := filepath.Join(homeDir, ".zshrc")
		sourceGitconfig := filepath.Join(homeDir, ".gitconfig")
		if err := os.Symlink(zshrcFile, sourceZshrc); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}
		if err := os.Symlink(gitconfigFile, sourceGitconfig); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Create config with both files
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: "%s"
    repo_path: shell/zshrc
    added_at: "%s"
  - source_path: "%s"
    repo_path: gitconfig
    added_at: "%s"
`, filesDir, sourceZshrc, time.Now().Format(time.RFC3339), sourceGitconfig, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Status only for zshrc
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status", sourceZshrc)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err, "status command should succeed")
		outputStr := stdout.String()
		assert.Contains(t, outputStr, ".zshrc", "output should contain zshrc")
		assert.NotContains(t, outputStr, ".gitconfig", "output should not contain gitconfig")
		assert.Contains(t, outputStr, "files managed", "should show file count")
	})
}

func TestStatus_NoArgs_ShowsAll(t *testing.T) {
	t.Run("status without arguments shows all files", func(t *testing.T) {
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

		testFile := filepath.Join(filesDir, "test.txt")
		CreateTestFile(t, testFile, "content")
		sourceFile := filepath.Join(homeDir, ".test")
		if err := os.Symlink(testFile, sourceFile); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		configPath := filepath.Join(configDir, "config.yaml")
		configContent := fmt.Sprintf(`version: "1.0"
repo_path: %s
git_enabled: true
managed_files:
  - source_path: "%s"
    repo_path: test.txt
    added_at: "%s"
`, filesDir, sourceFile, time.Now().Format(time.RFC3339))
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Build dotcor binary
		buildPath := filepath.Join(tempDir, "dotcor-test")
		buildCmd := exec.Command("go", "build", "-o", buildPath, "github.com/justincordova/dotcor/cmd/dotcor")
		output, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("building test binary failed: %v\noutput: %s", err, string(output))
		}

		// Act - Status without arguments
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(buildPath, "status")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
		err = cmd.Run()

		// Assert
		require.NoError(t, err, "status command should succeed")
		outputStr := stdout.String()
		assert.Contains(t, outputStr, ".test", "output should contain file")
		assert.Contains(t, outputStr, "files managed", "should show file count")
	})
}
