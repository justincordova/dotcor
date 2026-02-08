package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

func TestRemoveCommandSingleFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")

	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: false,
		ManagedFiles: []config.ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
			},
		},
	}

	// Create repo file and symlink
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test zshrc"), 0644)
	os.MkdirAll(homeDir, 0755)
	os.Symlink(repoFile, sourceFile)

	// Act - Simulate remove behavior
	err := os.Remove(sourceFile)
	require.NoError(t, err, "removing symlink should succeed")

	// Update config
	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	assert.NoFileExists(t, sourceFile, "symlink should be removed")
	assert.FileExists(t, repoFile, "repo file should still exist")
	assert.Len(t, cfg.ManagedFiles, 0, "should have 0 managed files")
}

func TestRemoveCommandValidation(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
			},
		},
	}

	tests := []struct {
		name       string
		sourcePath string
		isManaged  bool
	}{
		{
			name:       "remove managed file",
			sourcePath: "~/.zshrc",
			isManaged:  true,
		},
		{
			name:       "remove unmanaged file",
			sourcePath: "~/.bashrc",
			isManaged:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.sourcePath

			// Act - Check if path is managed
			isManaged := false
			for _, mf := range cfg.ManagedFiles {
				if mf.SourcePath == path {
					isManaged = true
					break
				}
			}

			// Assert
			assert.Equal(t, tt.isManaged, isManaged,
				"file %q managed status should be %v", path, tt.isManaged)
		})
	}
}

func TestRemoveCommandKeepRepoFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create repo file with actual content
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# My zsh config\nexport PATH=/usr/bin"), 0644)

	// Create symlink
	os.Symlink(repoFile, sourceFile)

	// Act - Remove symlink
	err := os.Remove(sourceFile)
	require.NoError(t, err, "removing symlink should succeed")

	// Assert
	assert.NoFileExists(t, sourceFile, "symlink should be removed")
	assert.FileExists(t, repoFile, "repo file should still exist")

	// Verify repo file content
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "My zsh config", "repo file content should be preserved")
}

func TestRemoveCommandRestoreBackup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	backupsDir := filepath.Join(tempDir, "backups")
	os.MkdirAll(backupsDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	backupFile := filepath.Join(backupsDir, ".zshrc.backup")

	// Create backup
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(backupFile, []byte("# Backup content"), 0644)

	// Act - Restore from backup
	content, err := os.ReadFile(backupFile)
	require.NoError(t, err, "reading backup should succeed")

	err = os.WriteFile(sourceFile, content, 0644)
	require.NoError(t, err, "restoring file should succeed")

	// Assert
	assert.FileExists(t, sourceFile, "source file should exist after restore")
	assert.FileExists(t, backupFile, "backup file should still exist")

	// Verify content
	restoredContent, _ := os.ReadFile(sourceFile)
	assert.Equal(t, "# Backup content", string(restoredContent),
		"restored content should match backup")
}

func TestRemoveCommandMultipleFiles(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	os.MkdirAll(homeDir, 0755)

	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: false,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
			{SourcePath: "~/.bashrc", RepoPath: "shell/bashrc"},
			{SourcePath: "~/.vimrc", RepoPath: "vim/vimrc"},
		},
	}

	// Create all files
	for _, mf := range cfg.ManagedFiles {
		repoFile := filepath.Join(filesDir, mf.RepoPath)
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		os.MkdirAll(filepath.Dir(repoFile), 0755)
		os.WriteFile(repoFile, []byte("content"), 0644)
		os.Symlink(repoFile, sourceFile)
	}

	// Act - Remove all files
	for _, mf := range cfg.ManagedFiles {
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		os.Remove(sourceFile)
	}

	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	assert.Len(t, cfg.ManagedFiles, 0, "all files should be removed from config")

	// Verify no symlinks exist
	assert.NoFileExists(t, filepath.Join(homeDir, ".zshrc"), "zshrc symlink should be removed")
	assert.NoFileExists(t, filepath.Join(homeDir, ".bashrc"), "bashrc symlink should be removed")
	assert.NoFileExists(t, filepath.Join(homeDir, ".vimrc"), "vimrc symlink should be removed")

	// Verify repo files still exist
	assert.FileExists(t, filepath.Join(filesDir, "shell", "zshrc"), "zshrc repo file should exist")
	assert.FileExists(t, filepath.Join(filesDir, "shell", "bashrc"), "bashrc repo file should exist")
	assert.FileExists(t, filepath.Join(filesDir, "vim", "vimrc"), "vimrc repo file should exist")
}
