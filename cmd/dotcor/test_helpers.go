package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

// CreateTestConfig creates a test config with temp directory for testing commands
func CreateTestConfig(t *testing.T) *config.Config {
	t.Helper()

	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return &config.Config{
		Logger:         logger,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false, // Disable git for most command tests
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}
}

// CreateTestConfigWithGit creates a test config with temp directory and initialized git repo
func CreateTestConfigWithGit(t *testing.T, dir string) *config.Config {
	t.Helper()

	filesDir := filepath.Join(dir, ".dotcor", "files")
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Create directory and initialize git
	os.MkdirAll(filesDir, 0755)
	runGit(t, filesDir, "init")
	runGit(t, filesDir, "config", "user.email", "test@example.com")
	runGit(t, filesDir, "config", "user.name", "Test User")
	runGit(t, filesDir, "checkout", "-b", "main")

	// Create initial commit
	testFile := filepath.Join(filesDir, "test.txt")
	os.WriteFile(testFile, []byte("initial content"), 0644)
	runGit(t, filesDir, "add", "test.txt")
	runGit(t, filesDir, "commit", "-m", "Initial commit")

	return &config.Config{
		Logger:         logger,
		RepoPath:       filesDir,
		GitEnabled:     true,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}
}

// runGit executes git commands in a directory
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\noutput: %s", args, dir, err, string(output))
	}
}

// CreateTestFile creates a test file with specified content in temp directory
func CreateTestFile(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
}

// AssertFileExists asserts that a file exists at the given path
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("file does not exist: %s", path)
		return
	}
	if err != nil {
		t.Errorf("failed to stat file %s: %v", path, err)
		return
	}
	if info.IsDir() {
		t.Errorf("path is a directory, not a file: %s", path)
	}
}

// AssertFileNotExists asserts that a file does not exist at the given path
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Errorf("file exists but should not: %s", path)
		return
	}
	if !os.IsNotExist(err) {
		t.Errorf("unexpected error checking file %s: %v", path, err)
	}
}

// AssertDirExists asserts that a directory exists at the given path
func AssertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		t.Errorf("directory does not exist: %s", path)
		return
	}
	if err != nil {
		t.Errorf("failed to stat directory %s: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("path is a file, not a directory: %s", path)
	}
}

// AssertFileContent asserts that a file contains the expected content
func AssertFileContent(t *testing.T, path, expectedContent string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", path, err)
	}

	if string(content) != expectedContent {
		t.Errorf("file content mismatch\n  got: %q\n  want: %q", string(content), expectedContent)
	}
}

// CreateTestSymlink creates a symlink pointing to target at link location
func CreateTestSymlink(t *testing.T, target, link string) {
	t.Helper()

	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("failed to create symlink from %s to %s: %v", link, target, err)
	}
}

// AssertSymlinkPointsTo asserts that a symlink points to the expected target
func AssertSymlinkPointsTo(t *testing.T, link, expectedTarget string) {
	t.Helper()

	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("failed to read symlink %s: %v", link, err)
	}

	if target != expectedTarget {
		t.Errorf("symlink %s points to %s, want %s", link, target, expectedTarget)
	}
}

// RunCommand executes dotcor command with arguments and environment variables
func RunCommand(t *testing.T, cmd, args string, env map[string]string) (stdout, stderr string, exitCode int) {
	t.Helper()

	binaryPath, err := os.Executable()
	if err != nil {
		t.Fatalf("failed to get executable path: %v", err)
	}

	command := exec.Command(binaryPath, args)

	if env != nil {
		for k, v := range env {
			command.Env = append(command.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var outBuf, errBuf bytes.Buffer
	command.Stdout = &outBuf
	command.Stderr = &errBuf

	err = command.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// CreateTestLogger creates a logger with a buffer for log capture in tests
func CreateTestLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	return logger, &buf
}

// AssertLogContains asserts that log output contains expected message at specified level
func AssertLogContains(t *testing.T, buf *bytes.Buffer, level, msg string) {
	t.Helper()

	logOutput := buf.String()
	if !strings.Contains(logOutput, level) {
		t.Errorf("log output does not contain level %q\noutput: %s", level, logOutput)
	}
	if !strings.Contains(logOutput, msg) {
		t.Errorf("log output does not contain message %q\noutput: %s", msg, logOutput)
	}
}

// SetupGitRepo creates a test git repository at specified path
func SetupGitRepo(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", path, err)
	}

	runGit(t, path, "init")
	runGit(t, path, "config", "user.email", "test@example.com")
	runGit(t, path, "config", "user.name", "Test User")
	runGit(t, path, "checkout", "-b", "main")
}

// CreateTestConfigFile creates a YAML config file with specified content
func CreateTestConfigFile(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}
}
