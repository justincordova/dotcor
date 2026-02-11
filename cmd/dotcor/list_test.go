package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_NoFiles_PrintsEmptyMessage(t *testing.T) {
	t.Run("no managed files shows empty message", func(t *testing.T) {
		// Arrange - Create temp directory with empty config file
		tempDir := t.TempDir()
		configDir := filepath.Join(tempDir, ".dotcor")
		filesDir := filepath.Join(configDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			t.Fatalf("failed to create files dir: %v", err)
		}

		// Create empty config file
		configPath := filepath.Join(configDir, "config.yaml")
		configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: false
managed_files: []
`
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		// Use pre-built binary
		binaryPath := "/tmp/dotcor-test-binary"
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Fatalf("test binary not found at %s. Run 'go build -o %s ./cmd/dotcor' first.", binaryPath, binaryPath)
		}

		// Act - Run list command
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(binaryPath, "list")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Env = append(os.Environ(), "HOME="+tempDir)
		err = cmd.Run()

		// Assert
		require.NoError(t, err, "list command should succeed")
		outputStr := stdout.String()
		assert.Contains(t, outputStr, "No files managed by DotCor.", "should show empty message")
		assert.Contains(t, outputStr, "Run 'dotcor add <file>' to start managing dotfiles.", "should suggest add command")
	})
}

func TestList_SingleFile_DisplaysCorrectly(t *testing.T) {
	t.Run("single file displays in simple format", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		now := time.Now()
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
				AddedAt:    now,
			},
		}

		// Act - call the actual outputSimple function
		files := cfg.ManagedFiles
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		if err := outputSimple(files); err != nil {
			t.Fatalf("failed to output simple: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		if _, err := out.ReadFrom(r); err != nil {
			t.Fatalf("failed to read from pipe: %v", err)
		}
		output := out.String()

		// Assert
		assert.Contains(t, output, "~/.zshrc")
		assert.Contains(t, output, "1 file(s) managed")
	})
}

func TestList_MultipleFiles_DisplaysInTable(t *testing.T) {
	t.Run("multiple files in long format table", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		now := time.Now()
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
				AddedAt:    now,
			},
			{
				SourcePath: "~/.config/nvim/init.vim",
				RepoPath:   "nvim/init.vim",
				AddedAt:    now,
			},
		}

		// Act - call the actual outputLong function
		files := cfg.ManagedFiles
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		if err := outputLong(cfg, files, false); err != nil {
			t.Fatalf("failed to output long: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		if _, err := out.ReadFrom(r); err != nil {
			t.Fatalf("failed to read from pipe: %v", err)
		}
		output := out.String()

		// Assert
		assert.Contains(t, output, "SOURCE")
		assert.Contains(t, output, "REPO PATH")
		assert.Contains(t, output, "ADDED")
		assert.Contains(t, output, "~/.zshrc")
		assert.Contains(t, output, "~/.config/nvim/init.vim")
		assert.Contains(t, output, "2 file(s) managed")
	})
}

func TestList_UncommittedFile_ShowsWarning(t *testing.T) {
	t.Run("files with symlink status display warnings", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		homeDir := filepath.Join(tempDir, "home")
		repoDir := filepath.Join(tempDir, "repo")
		sourceFile := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(repoDir, "shell", "zshrc")

		// Create directories
		if err := os.MkdirAll(homeDir, 0755); err != nil {
			t.Fatalf("failed to create home dir: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir: %v", err)
		}

		// Create repo file
		if err := os.WriteFile(repoFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create repo file: %v", err)
		}

		// Create source file (not a symlink - this will trigger "not-symlink" status)
		if err := os.WriteFile(sourceFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}

		cfg := &config.Config{
			RepoPath:   repoDir,
			GitEnabled: false,
			Logger:     CreateTestConfig(t).Logger,
		}

		now := time.Now()
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath: sourceFile,
				RepoPath:   "shell/zshrc",
				AddedAt:    now,
			},
		}

		// Act - call the actual outputLong function with showStatus=true
		files := cfg.ManagedFiles
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		if err := outputLong(cfg, files, true); err != nil {
			t.Fatalf("failed to output long: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		if _, err := out.ReadFrom(r); err != nil {
			t.Fatalf("failed to read from pipe: %v", err)
		}
		output := out.String()

		// Assert
		assert.Contains(t, output, "SOURCE")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, sourceFile)
		assert.Contains(t, output, "not-symlink")
	})
}

func TestList_OutputFormat_NoPlatformsColumn(t *testing.T) {
	// This test verifies that the list output does NOT contain a PLATFORMS column
	// which was removed from the implementation but may still appear in documentation

	// Read README.md to verify documentation matches implementation
	readmeContent, err := os.ReadFile("../../README.md")
	require.NoError(t, err)

	// Check that the README output example does NOT contain PLATFORMS column
	// The old format was:
	// SOURCE PATH                     REPO PATH              ADDED AT          PLATFORMS
	// The correct format should not have PLATFORMS column

	lines := string(readmeContent)
	hasStaleFormat := false

	// Check if stale format exists in README
	for _, line := range []string{
		"SOURCE PATH                     REPO PATH              ADDED AT          PLATFORMS",
		"SOURCE PATH          REPO PATH         ADDED AT          PLATFORMS",
		"PLATFORMS",
	} {
		if strings.Contains(lines, line) {
			hasStaleFormat = true
			break
		}
	}

	// This test documents that PLATFORMS column should NOT exist
	assert.False(t, hasStaleFormat, "README should not contain stale PLATFORMS column in list output example")
}
