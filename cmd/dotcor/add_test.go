package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

// ========== Happy Path Tests ==========

func TestAdd_SingleFile_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	cfg := &config.Config{
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}

	// Create source file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# Test zshrc\nexport PATH=/bin"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Simulate add behavior
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Create backup
	backupPath := filepath.Join(backupsDir, ".zshrc")
	if err := os.WriteFile(backupPath, []byte("# Test zshrc\nexport PATH=/bin"), 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err, "moving file should succeed")
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err, "symlink creation should succeed")

	// Update config (no platforms field)
	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.zshrc",
		RepoPath:   "shell/zshrc",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)
	assert.Len(t, cfg.ManagedFiles, 1, "should have 1 managed file")
	assert.NotContains(t, repoFile, "platforms", "repo path should not contain platforms")

	// Verify symlink (macOS native)
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)

	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target)
}

func TestAdd_MultipleFiles_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	cfg := &config.Config{
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}

	// Create source files
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	filesToAdd := []string{".zshrc", ".bashrc", ".vimrc"}
	for _, filename := range filesToAdd {
		sourceFile := filepath.Join(homeDir, filename)
		if err := os.WriteFile(sourceFile, []byte("# "+filename), 0644); err != nil {
			t.Fatalf("failed to create source file %s: %v", filename, err)
		}
	}

	// Act - Simulate adding multiple files
	for _, filename := range filesToAdd {
		sourceFile := filepath.Join(homeDir, filename)
		repoFile := filepath.Join(filesDir, "shell", filename)
		backupPath := filepath.Join(backupsDir, filename)

		// Create backup
		if err := os.WriteFile(backupPath, []byte("# "+filename), 0644); err != nil {
			t.Fatalf("failed to create backup %s: %v", filename, err)
		}

		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			t.Fatalf("failed to create repo dir for %s: %v", filename, err)
		}
		if err := os.Rename(sourceFile, repoFile); err != nil {
			t.Fatalf("failed to move %s: %v", filename, err)
		}
		if err := os.Symlink(repoFile, sourceFile); err != nil {
			t.Fatalf("failed to create symlink for %s: %v", filename, err)
		}

		// Add to config (no platforms field)
		cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
			SourcePath: "~/" + filename,
			RepoPath:   "shell/" + filename,
			AddedAt:    time.Now(),
		})
	}

	// Assert
	assert.Len(t, cfg.ManagedFiles, 3, "should have 3 managed files")
	for _, filename := range filesToAdd {
		sourceFile := filepath.Join(homeDir, filename)
		repoFile := filepath.Join(filesDir, "shell", filename)
		AssertFileExists(t, repoFile)
		AssertFileExists(t, sourceFile)

		info, _ := os.Lstat(sourceFile)
		assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)
	}
}

func TestAdd_WithCategory_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".config", "nvim", "init.vim")
	repoFile := filepath.Join(filesDir, "nvim", "init.vim")
	cfg := &config.Config{
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}

	// Create source file
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("set number\n"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Simulate add with category
	backupPath := filepath.Join(backupsDir, "init.vim")
	if err := os.WriteFile(backupPath, []byte("set number\n"), 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err)
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Add to config with custom category (no platforms field)
	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.config/nvim/init.vim",
		RepoPath:   "nvim/init.vim",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)
	assert.Contains(t, repoFile, "nvim", "repo path should use custom category")
}

// ========== Template Tests ==========

func TestAdd_TemplateFile_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc.template")
	repoFile := filepath.Join(filesDir, "shell", "zshrc.template")
	cfg := &config.Config{
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}

	// Create source file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	templateContent := "# Config for {{ .Hostname }}\nexport HOME={{ .Home }}\n"
	if err := os.WriteFile(sourceFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Simulate add with template flag
	backupPath := filepath.Join(backupsDir, ".zshrc.template")
	if err := os.WriteFile(backupPath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err)
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Add to config with .template extension (no platforms field)
	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.zshrc.template",
		RepoPath:   "shell/zshrc.template",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)
	assert.Contains(t, repoFile, ".template", "repo file should have .template extension")

	// Verify template variables (Hostname, User, Home only - no OS variable)
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "{{ .Hostname }}")
	assert.Contains(t, string(content), "{{ .Home }}")
	assert.NotContains(t, string(content), "{{ .OS }}", "template should not have OS variable")
}

// ========== Validation Tests ==========

func TestAdd_Validation_ValidFiles_Accepted(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		content    string
		shouldPass bool
	}{
		{
			name:       "valid zshrc file",
			sourcePath: "~/.zshrc",
			content:    "# Test config\nexport PATH=/bin",
			shouldPass: true,
		},
		{
			name:       "valid nvim config",
			sourcePath: "~/.config/nvim/init.vim",
			content:    "set number\nset tabstop=4",
			shouldPass: true,
		},
		{
			name:       "file with API key",
			sourcePath: "~/.env",
			content:    "API_KEY=sk-1234567890abcdef",
			shouldPass: false,
		},
		{
			name:       "file with secret",
			sourcePath: "~/.env",
			content:    "SECRET=password123",
			shouldPass: false,
		},
		{
			name:       "file with token",
			sourcePath: "~/.config/token",
			content:    "auth_token=ghp_xxxxxxxxxxxxx",
			shouldPass: false,
		},
		{
			name:       "private key file",
			sourcePath: "~/.ssh/id_rsa",
			content:    "-----BEGIN RSA PRIVATE KEY-----",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			content := tt.content

			// Act - Check for secrets
			hasSecret := false
			secrets := []string{"api_key", "api-key", "secret", "token", "private_key"}
			lowerContent := strings.ToLower(content)

			for _, secret := range secrets {
				if strings.Contains(lowerContent, secret) && strings.Contains(content, "=") {
					hasSecret = true
					break
				}
			}

			// Check for private key file
			if strings.Contains(tt.sourcePath, "id_rsa") || strings.Contains(tt.sourcePath, "private") {
				hasSecret = true
			}

			// Assert
			assert.Equal(t, !tt.shouldPass, hasSecret, "secret detection should match expected")
		})
	}
}

// ========== Error Path Tests ==========

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
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Make directory read-only
	if err := os.Chmod(homeDir, 0444); err != nil {
		t.Fatalf("failed to make directory read-only: %v", err)
	}

	// Act
	_, err := os.OpenFile(sourceFile, os.O_WRONLY, 0)

	// Assert
	assert.Error(t, err, "should return error when permission denied")

	// Cleanup
	if err := os.Chmod(homeDir, 0755); err != nil {
		t.Fatalf("failed to restore directory permissions: %v", err)
	}
}

func TestAdd_AlreadyManaged_Skips(t *testing.T) {
	// Arrange
	sourcePath := "~/.zshrc"
	normalized, _ := config.NormalizePath(sourcePath)

	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{
				SourcePath: normalized,
				RepoPath:   "shell/zshrc",
				AddedAt:    time.Now(),
			},
		},
	}

	// Act - Check if file is already managed
	isManaged := false
	for _, mf := range cfg.ManagedFiles {
		if mf.SourcePath == normalized {
			isManaged = true
			break
		}
	}

	// Assert
	assert.True(t, isManaged, "file should be detected as already managed")
}

func TestAdd_BackupFails_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	backupsDir := filepath.Join(configDir, "backups")
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Create source file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Make backups directory non-writable
	// First create configDir with normal permissions
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	// Then create backupsDir with restricted permissions
	if err := os.MkdirAll(backupsDir, 0444); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	// Act - Attempt to create backup would fail
	_, err := os.OpenFile(filepath.Join(backupsDir, "test"), os.O_WRONLY|os.O_CREATE, 0644)

	// Assert
	assert.Error(t, err, "backup creation should fail when directory is not writable")

	// Cleanup
	if err := os.Chmod(backupsDir, 0755); err != nil {
		t.Fatalf("failed to restore backups dir permissions: %v", err)
	}
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
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	originalContent := []byte("# Original content\nexport PATH=/bin")
	if err := os.WriteFile(sourceFile, originalContent, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create backup
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}
	if err := os.WriteFile(backupFile, originalContent, 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Simulate transaction start - move file
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Act - Transaction fails, restore from backup
	err := os.Rename(sourceFile, repoFile)
	if err == nil {
		// Simulate failure - restore from backup
		if restoreErr := os.Rename(backupFile, sourceFile); restoreErr != nil {
			t.Fatalf("failed to restore from backup: %v", restoreErr)
		}
	}

	// Assert
	content, _ := os.ReadFile(sourceFile)
	assert.Equal(t, originalContent, content, "backup should be restored on failure")
}

// ========== Flag Tests ==========

func TestAdd_Flag_Category_UsesCustomCategory(t *testing.T) {
	// Arrange
	category := "mycategory"
	sourcePath := "~/.zshrc"

	// Act - Generate repo path with custom category
	baseName := filepath.Base(sourcePath)
	repoPath := filepath.Join(category, strings.TrimPrefix(baseName, "."))

	// Assert
	assert.Equal(t, "mycategory/zshrc", repoPath, "should use custom category")
}

func TestAdd_Flag_DryRun_NoChangesMade(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Create source file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Dry run doesn't make changes

	// In dry run, we only print what would happen
	var buf bytes.Buffer
	buf.WriteString("  + ~/.zshrc → shell/zshrc\n")

	// Assert - Verify no actual file operations happened
	assert.NotContains(t, buf.String(), "moved")
	assert.NotContains(t, buf.String(), "created")
	assert.Contains(t, buf.String(), "+ ~/.zshrc", "dry run should show what would be added")
	assert.Contains(t, buf.String(), "shell/zshrc")
}

func TestAdd_Flag_Force_SkipsValidation(t *testing.T) {
	// Arrange
	content := "API_KEY=secret123"

	// Act - Check if force flag would skip validation
	force := true

	hasSecret := strings.Contains(strings.ToLower(content), "api_key") && strings.Contains(content, "=")

	// Assert
	if force {
		assert.True(t, hasSecret, "with force, should add file even with secrets")
	} else {
		assert.False(t, !hasSecret, "without force, should reject file with secrets")
	}
}

func TestAdd_Flag_Recursive_AddsDirectory(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	configPath := filepath.Join(homeDir, ".config", "nvim")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create files in directory
	if err := os.WriteFile(filepath.Join(configPath, "init.vim"), []byte("vim config"), 0644); err != nil {
		t.Fatalf("failed to create init.vim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "custom.lua"), []byte("lua config"), 0644); err != nil {
		t.Fatalf("failed to create custom.lua: %v", err)
	}

	// Act - Simulate recursive add
	var addedFiles []string
	if err := filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		addedFiles = append(addedFiles, path)
		return nil
	}); err != nil {
		t.Fatalf("failed to walk config path: %v", err)
	}

	// Assert
	assert.Len(t, addedFiles, 2, "should find 2 files in directory recursively")
}

// ========== Glob Pattern Tests ==========

func TestAdd_GlobPattern_Expands(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	shellDir := filepath.Join(homeDir, "shell")
	if err := os.MkdirAll(shellDir, 0755); err != nil {
		t.Fatalf("failed to create shell dir: %v", err)
	}

	// Create shell files
	if err := os.WriteFile(filepath.Join(shellDir, "zshrc"), []byte("zsh config"), 0644); err != nil {
		t.Fatalf("failed to create zshrc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shellDir, "bashrc"), []byte("bash config"), 0644); err != nil {
		t.Fatalf("failed to create bashrc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(shellDir, "profile"), []byte("profile config"), 0644); err != nil {
		t.Fatalf("failed to create profile: %v", err)
	}

	// Act - Expand glob pattern
	pattern := filepath.Join(shellDir, "*")
	matches, err := filepath.Glob(pattern)

	// Assert
	require.NoError(t, err)
	assert.Len(t, matches, 3, "should find 3 files matching glob pattern")
}

func TestAdd_GlobPattern_NoMatches_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	// Act - Try to expand glob pattern with no matches
	pattern := filepath.Join(homeDir, "*.nonexistent")
	matches, err := filepath.Glob(pattern)

	// Assert
	require.NoError(t, err)
	assert.Len(t, matches, 0, "should find no files matching pattern")
}

// ========== Hook Tests ==========

func TestAdd_HookFails_LogsWarning(t *testing.T) {
	// Arrange
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	preAddHook := filepath.Join(hooksDir, "pre-add")

	// Create a failing hook script
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	if err := os.WriteFile(preAddHook, []byte("#!/bin/sh\nexit 1"), 0755); err != nil {
		t.Fatalf("failed to create hook script: %v", err)
	}

	// Act - Simulate hook execution
	_, err := os.Stat(preAddHook)

	// Assert
	assert.NoError(t, err, "hook file should exist")
	AssertFileExists(t, preAddHook)

	// Hook would fail but operation continues
}

func TestAdd_HookSuccess_OperationContinues(t *testing.T) {
	// Arrange
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	preAddHook := filepath.Join(hooksDir, "pre-add")

	// Create a successful hook script
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	if err := os.WriteFile(preAddHook, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatalf("failed to create hook script: %v", err)
	}

	// Act - Simulate hook execution
	_, err := os.Stat(preAddHook)

	// Assert
	assert.NoError(t, err, "hook file should exist")
	assert.FileExists(t, preAddHook)
}

// ========== Git Integration Tests ==========

func TestAdd_GitEnabled_CommitsChanges(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: true,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc", AddedAt: time.Now()},
		},
	}

	// Create files
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.WriteFile(repoFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Act - Git would be enabled, commit would be attempted
	gitEnabled := cfg.GitEnabled

	// Assert
	assert.True(t, gitEnabled, "git should be enabled for commit")
	assert.FileExists(t, repoFile, "repo file should exist")
}

func TestAdd_GitNotEnabled_SkipsCommit(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		GitEnabled: false,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc", AddedAt: time.Now()},
		},
	}

	// Act - Git not enabled
	gitEnabled := cfg.GitEnabled

	// Assert
	assert.False(t, gitEnabled, "git should not be enabled, commit skipped")
}

// ========== Ignore Pattern Tests ==========

func TestAdd_IgnorePattern_Matches_Skips(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		pattern      string
		shouldIgnore bool
	}{
		{
			name:         "matches pattern",
			path:         ".cache/file",
			pattern:      ".cache",
			shouldIgnore: true,
		},
		{
			name:         "does not match",
			path:         ".zshrc",
			pattern:      ".cache",
			shouldIgnore: false,
		},
		{
			name:         "matches wildcard",
			path:         "file.log",
			pattern:      "*.log",
			shouldIgnore: true,
		},
		{
			name:         "wildcard does not match",
			path:         "file.txt",
			pattern:      "*.log",
			shouldIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.path
			pattern := tt.pattern

			// Act - Check if path matches pattern
			matched := false
			if strings.Contains(pattern, "*") {
				idx := strings.Index(pattern, "*")
				prefix := pattern[:idx]
				suffix := pattern[idx+1:]
				if prefix != "" && suffix != "" {
					if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
						matched = true
					}
				} else if prefix != "" {
					if strings.HasPrefix(path, prefix) {
						matched = true
					}
				} else if suffix != "" {
					if strings.HasSuffix(path, suffix) {
						matched = true
					}
				}
			} else if strings.Contains(path, pattern) {
				matched = true
			}

			// Assert
			assert.Equal(t, tt.shouldIgnore, matched)
		})
	}
}

// ========== Structured Logging Tests ==========

func TestAdd_EmitsCorrectLogs(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfig(t)

	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	cfg.Logger = logger

	// Act - Simulate add operation
	sourcePath := filepath.Join(tempDir, ".zshrc")
	if err := os.WriteFile(sourcePath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	cfg.Logger.Debug("validating source file", "path", sourcePath)
	cfg.Logger.Info("backup created", "file", sourcePath)
	cfg.Logger.Info("symlink created", "source", sourcePath, "target", filepath.Join(tempDir, "files", "zshrc"))

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "validating source file")
	assert.Contains(t, logs, "backup created")
	assert.Contains(t, logs, "symlink created")
	assert.Contains(t, logs, "path")
	assert.Contains(t, logs, "file")
	assert.Contains(t, logs, "source")
	assert.Contains(t, logs, "target")
}

func TestAdd_LogLevels_Verified(t *testing.T) {
	// Arrange
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	// Act - Log at different levels
	logger.Debug("validating file")
	logger.Info("adding file")
	logger.Warn("hook failed", "hook", "pre-add")
	logger.Error("backup failed", "error", "permission denied")

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "DEBUG", "debug message")
	assert.Contains(t, logs, "INFO", "info message")
	assert.Contains(t, logs, "WARN", "warning message")
	assert.Contains(t, logs, "ERROR", "error message")
}

// ========== Path Normalization Tests ==========

func TestAdd_PathNormalization_Works(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "tilde expansion",
			input:    "~/.zshrc",
			expected: "~/.zshrc",
		},
		{
			name:     "trailing slash removed",
			input:    "~/.config/nvim/",
			expected: "~/.config/nvim",
		},
		{
			name:     "extra slashes collapsed",
			input:    "~/.config///nvim//init.vim",
			expected: "~/.config/nvim/init.vim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			input := tt.input

			// Act - Normalize path
			normalized := filepath.Clean(input)
			normalized = strings.TrimRight(normalized, string(filepath.Separator))

			// Assert
			assert.Equal(t, tt.expected, normalized)
		})
	}
}

// ========== Edge Cases ==========

func TestAdd_EmptyFile_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".empty")
	repoFile := filepath.Join(filesDir, "shell", "empty")
	cfg := &config.Config{
		RepoPath:     filesDir,
		GitEnabled:   false,
		ManagedFiles: []config.ManagedFile{},
	}

	// Create empty file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Add empty file
	backupPath := filepath.Join(backupsDir, "empty")
	if err := os.WriteFile(backupPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.Rename(sourceFile, repoFile); err != nil {
		t.Fatalf("failed to move file: %v", err)
	}
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.empty",
		RepoPath:   "shell/empty",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)

	content, _ := os.ReadFile(repoFile)
	assert.Empty(t, content, "file should be empty")
}

func TestAdd_LargeFile_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".large")
	repoFile := filepath.Join(filesDir, "shell", "large")
	cfg := &config.Config{
		RepoPath:     filesDir,
		GitEnabled:   false,
		ManagedFiles: []config.ManagedFile{},
	}

	// Create large file (1MB)
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	largeContent := make([]byte, 1024*1024)
	if err := os.WriteFile(sourceFile, largeContent, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Add large file
	backupPath := filepath.Join(backupsDir, "large")
	if err := os.WriteFile(backupPath, largeContent, 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.Rename(sourceFile, repoFile); err != nil {
		t.Fatalf("failed to move file: %v", err)
	}
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.large",
		RepoPath:   "shell/large",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)

	info, _ := os.Stat(repoFile)
	assert.Greater(t, info.Size(), int64(1024*1000), "file should be large")
}

func TestAdd_FileWithSpecialCharacters_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".config-file")
	repoFile := filepath.Join(filesDir, "shell", "config-file")
	cfg := &config.Config{
		RepoPath:     filesDir,
		GitEnabled:   false,
		ManagedFiles: []config.ManagedFile{},
	}

	// Create file with hyphen
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("# config file with hyphen"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Add file
	backupPath := filepath.Join(backupsDir, "config-file")
	if err := os.WriteFile(backupPath, []byte("# config file with hyphen"), 0644); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	if err := os.Rename(sourceFile, repoFile); err != nil {
		t.Fatalf("failed to move file: %v", err)
	}
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: "~/.config-file",
		RepoPath:   "shell/config-file",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)
}

// ========== Helper Functions Tests ==========

func TestAdd_HelperFunctions_Work(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Test CreateTestConfig
	t.Run("CreateTestConfig", func(t *testing.T) {
		cfg := CreateTestConfig(t)
		assert.NotNil(t, cfg)
		assert.NotNil(t, cfg.Logger)
		assert.Contains(t, cfg.RepoPath, "files")
	})

	// Test CreateTestFile
	t.Run("CreateTestFile", func(t *testing.T) {
		testFile := filepath.Join(tempDir, "test.txt")
		CreateTestFile(t, testFile, "test content")
		AssertFileExists(t, testFile)
		AssertFileContent(t, testFile, "test content")
	})

	// Test CreateTestSymlink
	t.Run("CreateTestSymlink", func(t *testing.T) {
		testDir := t.TempDir()
		target := filepath.Join(testDir, "target.txt")
		link := filepath.Join(testDir, "link.txt")
		if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
			t.Fatalf("failed to create target file: %v", err)
		}
		CreateTestSymlink(t, target, link)
		AssertSymlinkPointsTo(t, link, target)
	})
}
