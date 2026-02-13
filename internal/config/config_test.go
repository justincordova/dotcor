package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetDefaultIgnorePatterns(t *testing.T) {
	// Act
	patterns := GetDefaultIgnorePatterns()

	// Assert
	assert.NotEmpty(t, patterns, "GetDefaultIgnorePatterns() returned empty slice")

	expected := []string{"*.key", ".env", "id_rsa", "*.swp", ".DS_Store"}
	for _, exp := range expected {
		found := false
		for _, p := range patterns {
			if p == exp {
				found = true
				break
			}
		}
		assert.True(t, found, "GetDefaultIgnorePatterns() missing expected pattern: %s", exp)
	}
}

func TestNewDefaultConfig(t *testing.T) {
	// Act
	cfg, err := NewDefaultConfig()

	// Assert
	assert.NoError(t, err, "NewDefaultConfig() error")
	assert.Equal(t, CurrentConfigVersion, cfg.Version, "Version")
	assert.True(t, cfg.GitEnabled, "GitEnabled should be true by default")
	assert.NotEmpty(t, cfg.IgnorePatterns, "IgnorePatterns should not be empty")
	assert.Empty(t, cfg.ManagedFiles, "ManagedFiles should be empty initially")
}

func TestConfigManagedFiles(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []ManagedFile{},
	}

	// Act & Assert
	assert.False(t, cfg.IsManaged("~/.zshrc"), "IsManaged() should return false for unmanaged file")

	mf := ManagedFile{
		SourcePath: "~/.zshrc",
		RepoPath:   "shell/zshrc",
		AddedAt:    time.Now(),
	}
	cfg.ManagedFiles = append(cfg.ManagedFiles, mf)

	assert.True(t, cfg.IsManaged("~/.zshrc"), "IsManaged() should return true for managed file")

	got, err := cfg.GetManagedFile("~/.zshrc")
	assert.NoError(t, err, "GetManagedFile() error")
	assert.Equal(t, "shell/zshrc", got.RepoPath, "GetManagedFile().RepoPath")

	_, err = cfg.GetManagedFile("~/.nonexistent")
	assert.Error(t, err, "GetManagedFile() should return error for non-existent file")
}

func TestGetUncommittedFiles(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:    CurrentConfigVersion,
		RepoPath:   "~/.dotcor/files",
		GitEnabled: true,
		ManagedFiles: []ManagedFile{
			{
				SourcePath:     "~/.zshrc",
				RepoPath:       "shell/zshrc",
				HasUncommitted: false,
			},
			{
				SourcePath:     "~/.bashrc",
				RepoPath:       "shell/bashrc",
				HasUncommitted: true,
			},
			{
				SourcePath:     "~/.vimrc",
				RepoPath:       "vim/vimrc",
				HasUncommitted: true,
			},
		},
	}

	// Act
	uncommitted := cfg.GetUncommittedFiles()

	// Assert
	assert.Equal(t, 2, len(uncommitted), "GetUncommittedFiles() returned wrong number of files")

	for _, f := range uncommitted {
		assert.True(t, f.HasUncommitted, "GetUncommittedFiles() returned file without uncommitted flag: %s", f.SourcePath)
	}
}

func TestMarkAsUncommitted_UpdatesFileFlag(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles: []ManagedFile{
			{
				SourcePath:     "~/.zshrc",
				RepoPath:       "shell/zshrc",
				AddedAt:        time.Now(),
				HasUncommitted: false,
			},
		},
	}

	// Act
	err = cfg.MarkAsUncommitted("~/.zshrc")

	// Assert
	assert.NoError(t, err, "MarkAsUncommitted() should not error")

	mf, err := cfg.GetManagedFile("~/.zshrc")
	assert.NoError(t, err, "GetManagedFile() should not error")
	assert.True(t, mf.HasUncommitted, "MarkAsUncommitted() should set HasUncommitted to true")
}

func TestClearUncommitted_ClearsFileFlag(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles: []ManagedFile{
			{
				SourcePath:     "~/.zshrc",
				RepoPath:       "shell/zshrc",
				AddedAt:        time.Now(),
				HasUncommitted: true,
			},
		},
	}

	// Act
	err = cfg.ClearUncommitted("~/.zshrc")

	// Assert
	assert.NoError(t, err, "ClearUncommitted() should not error")

	mf, err := cfg.GetManagedFile("~/.zshrc")
	assert.NoError(t, err, "GetManagedFile() should not error")
	assert.False(t, mf.HasUncommitted, "ClearUncommitted() should set HasUncommitted to false")
}

func TestGetUncommittedFiles_ReturnsOnlyFlagged(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:    CurrentConfigVersion,
		RepoPath:   "~/.dotcor/files",
		GitEnabled: false,
		ManagedFiles: []ManagedFile{
			{
				SourcePath:     "~/.zshrc",
				RepoPath:       "shell/zshrc",
				HasUncommitted: false,
			},
			{
				SourcePath:     "~/.bashrc",
				RepoPath:       "shell/bashrc",
				HasUncommitted: true,
			},
			{
				SourcePath:     "~/.vimrc",
				RepoPath:       "vim/vimrc",
				HasUncommitted: true,
			},
			{
				SourcePath:     "~/.gitconfig",
				RepoPath:       "git/gitconfig",
				HasUncommitted: false,
			},
		},
	}

	// Act
	uncommitted := cfg.GetUncommittedFiles()

	// Assert
	assert.Equal(t, 2, len(uncommitted), "GetUncommittedFiles() returned wrong number of files")

	for _, f := range uncommitted {
		assert.True(t, f.HasUncommitted, "GetUncommittedFiles() returned file without uncommitted flag: %s", f.SourcePath)
	}
}

func TestSaveConfig_AtomicWrite(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, ".dotcor")
	configPath := filepath.Join(configDir, "config.yaml")

	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     true,
		IgnorePatterns: GetDefaultIgnorePatterns(),
		ManagedFiles:   []ManagedFile{},
	}

	t.Setenv("HOME", tempDir)

	// Act
	err = cfg.SaveConfig()

	// Assert
	assert.NoError(t, err, "SaveConfig() should not error")
	assert.FileExists(t, configPath, "SaveConfig() should create config file")

	data, err := os.ReadFile(configPath)
	assert.NoError(t, err, "failed to read config file")
	assert.Contains(t, string(data), "version:", "SaveConfig() should write version")
	assert.Contains(t, string(data), "repo_path:", "SaveConfig() should write repo_path")
}

func TestLoadConfig_CorruptFile_ReturnsError(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, ".dotcor")
	err = os.MkdirAll(configDir, 0755)
	assert.NoError(t, err, "failed to create config dir")

	configPath := filepath.Join(configDir, "config.yaml")
	err = os.WriteFile(configPath, []byte("corrupt: yaml: content: [invalid"), 0644)
	assert.NoError(t, err, "failed to write corrupt config")

	t.Setenv("HOME", tempDir)

	// Act
	_, err = LoadConfig()

	// Assert
	assert.Error(t, err, "LoadConfig() should error for corrupt YAML")
	assert.Contains(t, err.Error(), "parsing config file", "Error should indicate parsing failure")
}

func TestLoadConfig_VersionMismatch_Migrates(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, ".dotcor")
	err = os.MkdirAll(configDir, 0755)
	assert.NoError(t, err, "failed to create config dir")

	configPath := filepath.Join(configDir, "config.yaml")
	oldConfig := "repo_path: ~/.dotcor/files\ngit_enabled: true\n"
	err = os.WriteFile(configPath, []byte(oldConfig), 0644)
	assert.NoError(t, err, "failed to write old config (no version)")

	t.Setenv("HOME", tempDir)

	// Act
	cfg, err := LoadConfig()

	// Assert
	assert.NoError(t, err, "LoadConfig() should not error")
	assert.NotNil(t, cfg, "LoadConfig() should return config")
	assert.Equal(t, CurrentConfigVersion, cfg.Version, "LoadConfig() should migrate empty version to current")
}

func TestRemoveManagedFile_BySourcePath_Deletes(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles: []ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
				AddedAt:    time.Now(),
			},
			{
				SourcePath: "~/.bashrc",
				RepoPath:   "shell/bashrc",
				AddedAt:    time.Now(),
			},
		},
	}

	// Act
	err = cfg.RemoveManagedFile("~/.zshrc")

	// Assert
	assert.NoError(t, err, "RemoveManagedFile() should not error")
	assert.False(t, cfg.IsManaged("~/.zshrc"), "RemoveManagedFile() should remove file from config")
	assert.True(t, cfg.IsManaged("~/.bashrc"), "RemoveManagedFile() should keep other files")

	err = cfg.RemoveManagedFile("~/.nonexistent")
	assert.Error(t, err, "RemoveManagedFile() should error for non-existent file")
}
