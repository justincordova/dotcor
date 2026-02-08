package main

import (
	"log/slog"
	"os"
	"path/filepath"
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
