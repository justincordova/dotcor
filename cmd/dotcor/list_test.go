package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
		os.MkdirAll(filesDir, 0755)

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

		outputSimple(files)

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		out.ReadFrom(r)
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

		outputLong(cfg, files, false)

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		out.ReadFrom(r)
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
		os.MkdirAll(homeDir, 0755)
		os.MkdirAll(filepath.Dir(repoFile), 0755)

		// Create repo file
		os.WriteFile(repoFile, []byte("test content"), 0644)

		// Create source file (not a symlink - this will trigger "not-symlink" status)
		os.WriteFile(sourceFile, []byte("test content"), 0644)

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

		outputLong(cfg, files, true)

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		out.ReadFrom(r)
		output := out.String()

		// Assert
		assert.Contains(t, output, "SOURCE")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, sourceFile)
		assert.Contains(t, output, "not-symlink")
	})
}
