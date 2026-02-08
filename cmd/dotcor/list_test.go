package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/justincordova/dotcor/internal/config"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestList_NoFiles_PrintsEmptyMessage(t *testing.T) {
	t.Run("no managed files shows empty message", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		cfg.ManagedFiles = []config.ManagedFile{}

		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			if len(files) == 0 {
				fmt.Println("No files managed by DotCor.")
				fmt.Println("Run 'dotcor add <file>' to start managing dotfiles.")
			}
		})

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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			outputSimple(files)
		})

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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			outputLong(cfg, files, false)
		})

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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			outputByCategory(cfg, files, false)
		})

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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			outputLong(cfg, files, true)
		})

		// Assert
		assert.Contains(t, output, "SOURCE")
		assert.Contains(t, output, "STATUS")
		assert.Contains(t, output, sourceFile)
		assert.Contains(t, output, "not-symlink")
	})
}

func TestList_PathsOnly(t *testing.T) {
	t.Run("paths-only outputs only file paths", func(t *testing.T) {
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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			for _, f := range files {
				fmt.Println(f.SourcePath)
			}
		})

		// Assert
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Len(t, lines, 2)
		assert.Contains(t, lines[0], "~/.zshrc")
		assert.Contains(t, lines[1], "~/.config/nvim/init.vim")
	})
}

func TestList_JSONOutput(t *testing.T) {
	t.Run("JSON format outputs valid JSON", func(t *testing.T) {
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
		files := cfg.GetManagedFilesForPlatform()

		// Act
		output := captureOutput(func() {
			outputJSON(cfg, files, false)
		})

		// Assert
		assert.Contains(t, output, "[")
		assert.Contains(t, output, "]")
		assert.Contains(t, output, "\"source\":")
		assert.Contains(t, output, "\"repo\":")
		assert.Contains(t, output, "~/.zshrc")
		assert.Contains(t, output, "shell/zshrc")
	})
}

func TestList_GetCategory(t *testing.T) {
	tests := []struct {
		name     string
		repoPath string
		expected string
	}{
		{
			name:     "extracts category from path",
			repoPath: "shell/zshrc",
			expected: "shell",
		},
		{
			name:     "extracts category from nested path",
			repoPath: "nvim/init.vim",
			expected: "nvim",
		},
		{
			name:     "handles top-level file",
			repoPath: "README.md",
			expected: "README.md",
		},
		{
			name:     "handles empty path",
			repoPath: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := getCategory(tt.repoPath)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestList_ResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			baseDir:  "/home/user",
			path:     "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			baseDir:  "/home/user",
			path:     "relative/path",
			expected: "/home/user/relative/path",
		},
		{
			name:     "relative path with ..",
			baseDir:  "/home/user",
			path:     "../parent/file",
			expected: "/home/parent/file",
		},
		{
			name:     "current directory",
			baseDir:  "/home/user",
			path:     ".",
			expected: "/home/user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := resolvePath(tt.baseDir, tt.path)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestList_GetDir(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "extracts directory from path",
			path:     "/home/user/.zshrc",
			expected: "/home/user",
		},
		{
			name:     "handles file without directory",
			path:     "file.txt",
			expected: ".",
		},
		{
			name:     "handles nested path",
			path:     "/a/b/c/d/file.txt",
			expected: "/a/b/c/d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := getDir(tt.path)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
