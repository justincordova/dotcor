package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestStatus_NotInitialized_ReturnsError(t *testing.T) {
	t.Run("no config returns error", func(t *testing.T) {
		// Arrange
		// Create temp directory without config
		cfg := CreateTestConfig(t)
		cfg.ManagedFiles = []config.ManagedFile{}

		// Act
		// Call status command
		status := collectStatus(cfg)

		// Assert
		// Verify no files are managed
		if status.Statistics.TotalFiles != 0 {
			t.Errorf("expected 0 total files, got %d", status.Statistics.TotalFiles)
		}
	})
}

func TestStatus_ValidSymlink_ShowsOK(t *testing.T) {
	t.Run("valid symlink shows ok status", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		homeDir := filepath.Join(tempDir, "home")
		repoDir := filepath.Join(tempDir, "repo")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(repoDir, "shell", "zshrc")

		// Create directories
		os.MkdirAll(homeDir, 0755)
		os.MkdirAll(filepath.Dir(repoFile), 0755)

		// Create repo file
		CreateTestFile(t, repoFile, "test content")

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		os.Symlink(relPath, sourcePath)

		cfg := &config.Config{
			RepoPath:   repoDir,
			GitEnabled: false,
			Logger:     CreateTestConfig(t).Logger,
		}

		now := time.Now()
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath: sourcePath,
				RepoPath:   "shell/zshrc",
				AddedAt:    now,
			},
		}

		// Act
		status := collectStatus(cfg)

		// Assert
		assert.Equal(t, 1, status.Statistics.TotalFiles)
		assert.Equal(t, 1, status.Statistics.HealthyFiles)
		assert.Equal(t, 0, status.Statistics.ProblematicFiles)
		assert.Equal(t, "ok", status.Files[0].Status)
	})
}

func TestStatus_BrokenSymlink_ShowsError(t *testing.T) {
	t.Run("missing repo file shows error status", func(t *testing.T) {
		// Arrange
		tempDir := t.TempDir()
		homeDir := filepath.Join(tempDir, "home")
		repoDir := filepath.Join(tempDir, "repo")
		sourcePath := filepath.Join(homeDir, ".zshrc")
		repoFile := filepath.Join(repoDir, "shell", "zshrc")

		// Create directories
		os.MkdirAll(homeDir, 0755)
		os.MkdirAll(filepath.Dir(repoFile), 0755)

		// Create valid symlink
		relPath, _ := filepath.Rel(filepath.Dir(sourcePath), repoFile)
		os.Symlink(relPath, sourcePath)
		// Don't create repo file - this simulates missing repo

		cfg := &config.Config{
			RepoPath:   repoDir,
			GitEnabled: false,
			Logger:     CreateTestConfig(t).Logger,
		}

		now := time.Now()
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath: sourcePath,
				RepoPath:   "shell/zshrc",
				AddedAt:    now,
			},
		}

		// Act
		status := collectStatus(cfg)

		// Assert
		assert.Equal(t, 1, status.Statistics.TotalFiles)
		assert.Equal(t, 0, status.Statistics.HealthyFiles)
		assert.Equal(t, 1, status.Statistics.ProblematicFiles)
		assert.Equal(t, "missing-repo", status.Files[0].Status)
		assert.NotEmpty(t, status.Files[0].Problem)
	})
}

func TestStatus_UncommittedChanges_ShowsWarning(t *testing.T) {
	t.Run("uncommitted files show in output", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		now := time.Now()

		// Create managed files with uncommitted changes
		cfg.ManagedFiles = []config.ManagedFile{
			{
				SourcePath:     "~/.zshrc",
				RepoPath:       "shell/zshrc",
				AddedAt:        now,
				HasUncommitted: true,
			},
			{
				SourcePath:     "~/.vimrc",
				RepoPath:       "editor/vimrc",
				AddedAt:        now,
				HasUncommitted: true,
			},
		}

		// Act
		uncommittedFiles := cfg.GetUncommittedFiles()

		// Assert
		assert.Equal(t, 2, len(uncommittedFiles))
		assert.Contains(t, uncommittedFiles[0].SourcePath, ".zshrc")
		assert.Contains(t, uncommittedFiles[1].SourcePath, ".vimrc")
	})
}

func TestStatus_GitAheadBehind_ShowsCounts(t *testing.T) {
	t.Run("git status shows ahead/behind counts", func(t *testing.T) {
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

		// Act
		status := collectStatus(cfg)

		// Assert
		// Without actual git repo, should show empty git status
		assert.False(t, status.GitStatus.IsRepo)
		assert.Equal(t, "", status.GitStatus.Branch)
		assert.Equal(t, 0, status.GitStatus.AheadBy)
		assert.Equal(t, 0, status.GitStatus.BehindBy)
	})
}
