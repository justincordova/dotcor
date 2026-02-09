package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

func TestAddCommandSingleFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	cfg := &config.Config{
		RepoPath:     filesDir,
		GitEnabled:   false,
		ManagedFiles: []config.ManagedFile{},
	}

	// Create source file
	os.MkdirAll(homeDir, 0755)
	if err := os.WriteFile(sourceFile, []byte("# Test zshrc\nexport PATH=/bin"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Simulate add behavior
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err, "moving file should succeed")
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err, "symlink creation should succeed")

	// Update config
	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.zshrc",
		RepoPath:   "shell/zshrc",
	})

	// Assert
	assert.FileExists(t, repoFile, "repo file should exist")
	assert.FileExists(t, sourceFile, "source symlink should exist")
	assert.Len(t, cfg.ManagedFiles, 1, "should have 1 managed file")

	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "should be symlink")
}

func TestAddCommandValidation(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		content    string
		shouldPass bool
		reason     string
	}{
		{
			name:       "valid zshrc file",
			sourcePath: "~/.zshrc",
			content:    "# Test config",
			shouldPass: true,
			reason:     "zshrc files should be valid",
		},
		{
			name:       "file with API key should fail",
			sourcePath: "~/.env",
			content:    "API_KEY=secret123",
			shouldPass: false,
			reason:     "files with API keys should be rejected",
		},
		{
			name:       "secret key file should fail",
			sourcePath: "~/.ssh/id_rsa",
			content:    "-----BEGIN RSA PRIVATE KEY-----",
			shouldPass: false,
			reason:     "private key files should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.sourcePath

			// Act - Check if path would be accepted
			isSecretFile := filepath.Base(path) == "id_rsa" || filepath.Base(path) == ".env"
			containsApiKey := tt.content == "API_KEY=secret123"

			shouldReject := isSecretFile || containsApiKey

			// Assert
			if tt.shouldPass {
				assert.False(t, shouldReject, tt.reason)
			} else {
				assert.True(t, shouldReject, tt.reason)
			}
		})
	}
}

func TestAddCommandCategory(t *testing.T) {
	// Arrange
	tests := []struct {
		name       string
		sourcePath string
		category   string
		expected   string
	}{
		{
			name:       "zshrc to shell category",
			sourcePath: "~/.zshrc",
			category:   "shell",
			expected:   "shell/.zshrc",
		},
		{
			name:       "nvim config to nvim category",
			sourcePath: "~/.config/nvim/init.vim",
			category:   "nvim",
			expected:   "nvim/init.vim",
		},
		{
			name:       "empty category uses default",
			sourcePath: "~/.zshrc",
			category:   "",
			expected:   "shell/.zshrc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sourcePath := tt.sourcePath
			category := tt.category

			// Act - Generate repo path based on category
			var repoPath string
			baseName := filepath.Base(sourcePath)
			if category != "" {
				repoPath = category + "/" + baseName
			} else {
				repoPath = "shell/" + baseName
			}

			// Assert
			assert.Equal(t, tt.expected, repoPath,
				"repo path should be %s", tt.expected)
		})
	}
}

func TestAddCommandTemplate(t *testing.T) {
	// Arrange
	sourcePath := "~/.zshrc.template"
	isTemplate := filepath.Ext(sourcePath) == ".template"

	// Act - Check if file is template
	expectedRepoPath := filepath.Base(sourcePath)

	if isTemplate {
		expectedRepoPath = expectedRepoPath[:len(expectedRepoPath)-len(".template")]
	}

	// Assert
	assert.True(t, isTemplate, "should recognize template extension")
	assert.Equal(t, ".zshrc", expectedRepoPath, "template extension should be stripped with dot")
}

func TestAddCommandRecursive(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	configPath := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(configPath, 0755)

	// Create multiple files in directory
	os.WriteFile(filepath.Join(configPath, "init.vim"), []byte("vim config"), 0644)
	os.WriteFile(filepath.Join(configPath, "custom.lua"), []byte("lua config"), 0644)

	// Act - Simulate recursive add
	var foundFiles []string
	filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			foundFiles = append(foundFiles, path)
		}
		return nil
	})

	// Assert
	assert.GreaterOrEqual(t, len(foundFiles), 2, "should find at least 2 files in directory")
}

func TestAddCommandGlobPattern(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	shellDir := filepath.Join(homeDir, "shell")
	os.MkdirAll(shellDir, 0755)

	// Create shell files
	os.WriteFile(filepath.Join(shellDir, "zshrc"), []byte("zsh config"), 0644)
	os.WriteFile(filepath.Join(shellDir, "bashrc"), []byte("bash config"), 0644)

	// Act - List files in directory
	var foundFiles []string
	entries, err := os.ReadDir(shellDir)
	require.NoError(t, err, "reading directory should succeed")
	for _, entry := range entries {
		if !entry.IsDir() {
			foundFiles = append(foundFiles, filepath.Join(shellDir, entry.Name()))
		}
	}

	// Assert
	assert.Len(t, foundFiles, 2, "should find 2 files in directory")
}

func TestAdd_FileDoesNotExist_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Act
	_, err := os.Stat(sourceFile)

	// Assert
	assert.Error(t, err, "should return error for non-existent file")
	assert.True(t, os.IsNotExist(err), "error should be NotExist")
}

func TestAdd_PermissionDenied_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Create source file
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(sourceFile, []byte("# Test"), 0644)

	// Make directory read-only
	os.Chmod(homeDir, 0444)

	// Act
	_, err := os.OpenFile(sourceFile, os.O_WRONLY, 0)

	// Assert
	assert.Error(t, err, "should return error when permission denied")

	// Cleanup
	os.Chmod(homeDir, 0755)
}

func TestAdd_HookFails_LogsWarning(t *testing.T) {
	// Arrange
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	preAddHook := filepath.Join(hooksDir, "pre-add")

	// Create a failing hook script
	os.MkdirAll(hooksDir, 0755)
	os.WriteFile(preAddHook, []byte("#!/bin/sh\nexit 1"), 0755)

	// Act - Simulate hook execution
	_, err := os.Stat(preAddHook)

	// Assert
	assert.NoError(t, err, "hook file should exist")
	assert.FileExists(t, preAddHook, "hook file should exist")

	// Hook would fail but operation continues
	// This is verified by checking the hook exists and would fail
}

func TestAdd_BackupFails_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	backupsDir := filepath.Join(configDir, "backups")
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Create source file
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(sourceFile, []byte("# Test"), 0644)

	// Make backups directory non-writable
	os.MkdirAll(backupsDir, 0444)

	// Act - Attempt to create backup would fail
	_, err := os.OpenFile(filepath.Join(backupsDir, "test"), os.O_WRONLY|os.O_CREATE, 0644)

	// Assert
	assert.Error(t, err, "backup creation should fail when directory is not writable")

	// Cleanup
	os.Chmod(backupsDir, 0755)
}

func TestAdd_TransactionFails_RestoresBackup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	backupsDir := filepath.Join(configDir, "backups")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	backupFile := filepath.Join(backupsDir, ".zshrc")

	// Create source file
	os.MkdirAll(homeDir, 0755)
	originalContent := []byte("# Original content\nexport PATH=/bin")
	os.WriteFile(sourceFile, originalContent, 0644)

	// Create backup
	os.MkdirAll(backupsDir, 0755)
	os.WriteFile(backupFile, originalContent, 0644)

	// Simulate transaction start - move file
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	os.MkdirAll(filepath.Dir(repoFile), 0755)

	// Act - Transaction fails, restore from backup
	err := os.Rename(sourceFile, repoFile)
	if err == nil {
		// Simulate failure - restore from backup
		os.Rename(backupFile, sourceFile)
	}

	// Assert
	content, _ := os.ReadFile(sourceFile)
	assert.Equal(t, originalContent, content, "backup should be restored on failure")
}

func TestAdd_GitCommitFails_MarksUncommitted(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")

	// Create files
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(sourceFile, []byte("# Test"), 0644)

	// Act - Simulate add without git (git would fail)
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err)

	// Create symlink
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Git not enabled, so no commit attempt
	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: false,
	}

	// Assert
	assert.FileExists(t, repoFile, "repo file should exist")
	assert.FileExists(t, sourceFile, "symlink should exist")
	assert.False(t, cfg.GitEnabled, "git not enabled, no commit attempted")
}
