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

func TestShouldApplyOnPlatform(t *testing.T) {
	tests := []struct {
		name            string
		platforms       []string
		currentPlatform string
		want            bool
	}{
		{
			name:            "empty platforms means all",
			platforms:       []string{},
			currentPlatform: "darwin",
			want:            true,
		},
		{
			name:            "nil platforms means all",
			platforms:       nil,
			currentPlatform: "linux",
			want:            true,
		},
		{
			name:            "matching platform",
			platforms:       []string{"darwin", "linux"},
			currentPlatform: "darwin",
			want:            true,
		},
		{
			name:            "non-matching platform",
			platforms:       []string{"darwin"},
			currentPlatform: "linux",
			want:            false,
		},
		{
			name:            "wsl platform",
			platforms:       []string{"wsl", "linux"},
			currentPlatform: "wsl",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := ShouldApplyOnPlatform(tt.platforms, tt.currentPlatform)

			// Assert
			assert.Equal(t, tt.want, got, "ShouldApplyOnPlatform()")
		})
	}
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
		Platforms:  []string{},
	}
	cfg.ManagedFiles = append(cfg.ManagedFiles, mf)

	assert.True(t, cfg.IsManaged("~/.zshrc"), "IsManaged() should return true for managed file")

	got, err := cfg.GetManagedFile("~/.zshrc")
	assert.NoError(t, err, "GetManagedFile() error")
	assert.Equal(t, "shell/zshrc", got.RepoPath, "GetManagedFile().RepoPath")

	_, err = cfg.GetManagedFile("~/.nonexistent")
	assert.Error(t, err, "GetManagedFile() should return error for non-existent file")
}

func TestGetManagedFilesForPlatform(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:    CurrentConfigVersion,
		RepoPath:   "~/.dotcor/files",
		GitEnabled: false,
		ManagedFiles: []ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
				Platforms:  []string{}, // All platforms
			},
			{
				SourcePath: "~/.bashrc",
				RepoPath:   "shell/bashrc",
				Platforms:  []string{"linux", "darwin"},
			},
			{
				SourcePath: "~/.wslconfig",
				RepoPath:   "wsl/wslconfig",
				Platforms:  []string{"wsl"},
			},
		},
	}

	// Act
	files := cfg.GetManagedFilesForPlatform()

	// Assert
	found := false
	for _, f := range files {
		if f.SourcePath == "~/.zshrc" {
			found = true
			break
		}
	}
	assert.True(t, found, "GetManagedFilesForPlatform() should include universal files")
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

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "foo", false},
		{"Microsoft", "Microsoft", true},
		{"Linux version WSL", "WSL", true},
		{"", "foo", false},
		{"foo", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			// Act
			got := contains(tt.s, tt.substr)

			// Assert
			assert.Equal(t, tt.want, got, "contains(%q, %q)", tt.s, tt.substr)
		})
	}
}
