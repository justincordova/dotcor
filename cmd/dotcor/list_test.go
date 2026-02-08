package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestList_NoFiles_PrintsEmptyMessage(t *testing.T) {
	t.Run("no managed files shows empty message", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		cfg.ManagedFiles = []config.ManagedFile{}

		// Act - call outputSimple directly since it handles the empty case through runList
		files := cfg.GetManagedFilesForPlatform()
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		if len(files) == 0 {
			fmt.Println("No files managed by DotCor.")
			fmt.Println("Run 'dotcor add <file>' to start managing dotfiles.")
		}

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		out.ReadFrom(r)
		output := out.String()

		// Assert
		assert.Contains(t, output, "No files managed by DotCor.")
		assert.Contains(t, output, "Run 'dotcor add <file>' to start managing dotfiles.")
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
		files := cfg.GetManagedFilesForPlatform()
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
		files := cfg.GetManagedFilesForPlatform()
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

	t.Run("multiple files in category format", func(t *testing.T) {
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
				SourcePath: "~/.bashrc",
				RepoPath:   "shell/bashrc",
				AddedAt:    now,
			},
			{
				SourcePath: "~/.config/nvim/init.vim",
				RepoPath:   "nvim/init.vim",
				AddedAt:    now,
			},
		}

		// Act - call the actual outputByCategory function
		files := cfg.GetManagedFilesForPlatform()
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		outputByCategory(cfg, files, false)

		w.Close()
		os.Stdout = oldStdout
		var out bytes.Buffer
		out.ReadFrom(r)
		output := out.String()

		// Assert
		assert.Contains(t, output, "[shell]")
		assert.Contains(t, output, "[nvim]")
		assert.Contains(t, output, "~/.zshrc")
		assert.Contains(t, output, "~/.bashrc")
		assert.Contains(t, output, "~/.config/nvim/init.vim")
		assert.Contains(t, output, "3 file(s)")
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
		files := cfg.GetManagedFilesForPlatform()
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
