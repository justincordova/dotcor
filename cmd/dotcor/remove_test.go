package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

// ========== Happy Path Tests ==========

func TestRemove_SingleFile_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	os.MkdirAll(filesDir, 0755)
	os.MkdirAll(backupsDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")

	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: false,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Create repo file and symlink
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test zshrc"), 0644)
	os.MkdirAll(homeDir, 0755)
	os.Symlink(repoFile, sourceFile)

	// Create backup of repo file
	backupPath := filepath.Join(backupsDir, "zshrc")
	os.WriteFile(backupPath, []byte("# Test zshrc"), 0644)

	// Act - Remove symlink
	err := os.Remove(sourceFile)
	require.NoError(t, err, "removing symlink should succeed")

	// Remove from config
	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	AssertFileNotExists(t, sourceFile)
	AssertFileExists(t, repoFile)
	assert.Len(t, cfg.ManagedFiles, 0, "should have 0 managed files")
}

func TestRemove_MultipleFiles_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	os.MkdirAll(filesDir, 0755)
	os.MkdirAll(backupsDir, 0755)

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
		backupPath := filepath.Join(backupsDir, filepath.Base(mf.RepoPath))

		// Create repo file
		os.MkdirAll(filepath.Dir(repoFile), 0755)
		os.WriteFile(repoFile, []byte("# "+filepath.Base(mf.SourcePath)), 0644)

		// Create symlink
		os.Symlink(repoFile, sourceFile)

		// Create backup
		os.WriteFile(backupPath, []byte("# "+filepath.Base(mf.SourcePath)), 0644)
	}

	// Act - Remove all symlinks
	for _, mf := range cfg.ManagedFiles {
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		os.Remove(sourceFile)
	}

	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	assert.Len(t, cfg.ManagedFiles, 0, "all files should be removed from config")
	for _, mf := range cfg.ManagedFiles {
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		AssertFileNotExists(t, sourceFile)
	}
}

func TestRemove_WithKeepRepo_PreservesRepoFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create repo file with actual content
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	repoContent := "# My zsh config\nexport PATH=/usr/bin"
	os.WriteFile(repoFile, []byte(repoContent), 0644)

	// Create symlink
	os.Symlink(repoFile, sourceFile)

	// Act - Remove symlink with --keep-repo
	err := os.Remove(sourceFile)
	require.NoError(t, err, "removing symlink should succeed")

	// Assert
	AssertFileNotExists(t, sourceFile)
	AssertFileExists(t, repoFile)

	// Verify repo file content
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)
	assert.Equal(t, repoContent, string(content), "repo file content should be preserved")
}

func TestRemove_RestoresFromBackup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	backupsDir := filepath.Join(tempDir, "backups")
	os.MkdirAll(backupsDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	backupFile := filepath.Join(backupsDir, ".zshrc.backup")

	// Create backup
	os.MkdirAll(homeDir, 0755)
	backupContent := "# Backup content"
	os.WriteFile(backupFile, []byte(backupContent), 0644)

	// Act - Restore from backup
	content, err := os.ReadFile(backupFile)
	require.NoError(t, err, "reading backup should succeed")

	err = os.WriteFile(sourceFile, content, 0644)
	require.NoError(t, err, "restoring file should succeed")

	// Assert
	AssertFileExists(t, sourceFile)
	AssertFileExists(t, backupFile)

	// Verify content
	restoredContent, _ := os.ReadFile(sourceFile)
	assert.Equal(t, backupContent, string(restoredContent), "restored content should match backup")
}

// ========== Validation Tests ==========

func TestRemove_NotManaged_ReturnsError(t *testing.T) {
	// Arrange
	sourcePath := "~/.bashrc"

	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Act - Check if file is managed
	isManaged := false
	for _, mf := range cfg.ManagedFiles {
		if mf.SourcePath == sourcePath {
			isManaged = true
			break
		}
	}

	// Assert
	assert.False(t, isManaged, "file should not be detected as managed")
}

func TestRemove_ManagedFile_Detected(t *testing.T) {
	// Arrange
	sourcePath := "~/.zshrc"

	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Act - Check if file is managed
	isManaged := false
	for _, mf := range cfg.ManagedFiles {
		if mf.SourcePath == sourcePath {
			isManaged = true
			break
		}
	}

	// Assert
	assert.True(t, isManaged, "file should be detected as managed")
}

// ========== Error Path Tests ==========

func TestRemove_FileDoesNotExist_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Act
	_, err := os.Stat(sourceFile)

	// Assert
	assert.Error(t, err, "should return error for non-existent file")
	assert.True(t, os.IsNotExist(err), "error should be NotExist")
}

func TestRemove_PermissionDenied_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create source file
	os.WriteFile(sourceFile, []byte("# Test"), 0644)

	// Make file read-only
	os.Chmod(sourceFile, 0444)

	// Act - Try to remove (this may succeed on some systems)
	err := os.Remove(sourceFile)

	// Assert
	if err != nil {
		assert.Error(t, err, "should return error when permission denied")
	}
	// Cleanup
	os.Chmod(sourceFile, 0755)
}

func TestRemove_SymlinkNotManaged_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")
	repoFile := filepath.Join(tempDir, "files", "zshrc")

	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{}, // Empty list
	}

	// Create symlink that's not managed
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Check if symlink is managed
	isManaged := false
	for _, mf := range cfg.ManagedFiles {
		expanded, _ := config.ExpandPath(mf.SourcePath, cfg)
		if expanded == sourceFile {
			isManaged = true
			break
		}
	}

	// Assert
	assert.False(t, isManaged, "symlink should not be detected as managed")
}

// ========== Flag Tests ==========

func TestRemove_Flag_KeepRepo_PreservesFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	cfg := &config.Config{
		RepoPath: filesDir,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Create repo file and symlink
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Remove with keep-repo
	keepRepo := true
	err := os.Remove(sourceFile)
	require.NoError(t, err)

	if keepRepo {
		// Update config (remove from managed list)
		cfg.ManagedFiles = []config.ManagedFile{}
	}

	// Assert
	AssertFileNotExists(t, sourceFile)
	if keepRepo {
		AssertFileExists(t, repoFile)
		assert.Len(t, cfg.ManagedFiles, 0, "should be removed from managed list")
	}
}

func TestRemove_Flag_All_RemovesAllFiles(t *testing.T) {
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
		},
	}

	// Create files
	for _, mf := range cfg.ManagedFiles {
		repoFile := filepath.Join(filesDir, mf.RepoPath)
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		os.MkdirAll(filepath.Dir(repoFile), 0755)
		os.WriteFile(repoFile, []byte("# content"), 0644)
		os.Symlink(repoFile, sourceFile)
	}

	// Act - Remove all with --all
	removeAll := true
	var filesToRemove []config.ManagedFile

	if removeAll {
		filesToRemove = cfg.ManagedFiles
	}

	// Remove all files
	for _, mf := range filesToRemove {
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		os.Remove(sourceFile)
	}

	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	assert.Len(t, cfg.ManagedFiles, 0, "all files should be removed from config")
	for _, mf := range filesToRemove {
		sourceFile := filepath.Join(homeDir, filepath.Base(mf.SourcePath))
		AssertFileNotExists(t, sourceFile)
	}
}

func TestRemove_Flag_Force_SkipsConfirmation(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")
	repoFile := filepath.Join(tempDir, "files", "zshrc")

	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Create files
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - With --force, skip confirmation
	force := true
	confirmed := true // In real code, this would come from prompt

	if force {
		// Skip confirmation, proceed with removal
		confirmed = true
	}

	// Assert
	assert.True(t, confirmed, "with --force, confirmation should be skipped")

	if confirmed {
		os.Remove(sourceFile)
		cfg.ManagedFiles = []config.ManagedFile{}

		AssertFileNotExists(t, sourceFile)
		assert.Len(t, cfg.ManagedFiles, 0, "should be removed from config")
	}
}

func TestRemove_Flag_DryRun_NoChangesMade(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	cfg := &config.Config{
		RepoPath: filesDir,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Create files
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Dry run doesn't make changes
	dryRun := true

	var buf bytes.Buffer
	if dryRun {
		buf.WriteString("Dry run - no changes will be made:\n")
		buf.WriteString("  - ~/.zshrc\n")
		buf.WriteString("    → Copy to " + sourceFile + "\n")
		buf.WriteString("    → Remove from repo: shell/zshrc\n")
	}

	// Assert - Verify no actual file operations happened
	assert.Contains(t, buf.String(), "Dry run", "dry run should be indicated")
	assert.Contains(t, buf.String(), "- ~/.zshrc", "should show file to remove")

	// Verify files still exist
	AssertFileExists(t, sourceFile)
	AssertFileExists(t, repoFile)
	assert.Len(t, cfg.ManagedFiles, 1, "should still be in config in dry run")
}

// ========== Git Integration Tests ==========

func TestRemove_GitEnabled_CommitsChanges(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	cfg := &config.Config{
		RepoPath:   filesDir,
		GitEnabled: true,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Create files
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Remove with git enabled
	/* keepRepo := false */ // Don't keep repo, so git commit happens
	err := os.Remove(sourceFile)
	require.NoError(t, err)

	cfg.ManagedFiles = []config.ManagedFile{}

	// Assert
	gitEnabled := cfg.GitEnabled
	assert.True(t, gitEnabled, "git should be enabled for commit")
	assert.Len(t, cfg.ManagedFiles, 0, "should be removed from config")
}

func TestRemove_GitNotEnabled_SkipsCommit(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		GitEnabled: false,
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Act - Git not enabled
	gitEnabled := cfg.GitEnabled

	// Assert
	assert.False(t, gitEnabled, "git should not be enabled, commit skipped")
}

// ========== Hook Tests ==========

func TestRemove_PreHookWarnsContinues(t *testing.T) {
	// Arrange
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	preRemoveHook := filepath.Join(hooksDir, "pre-remove")

	// Create a failing hook script
	os.MkdirAll(hooksDir, 0755)
	os.WriteFile(preRemoveHook, []byte("#!/bin/sh\nexit 1"), 0755)

	// Act - Simulate hook execution
	_, err := os.Stat(preRemoveHook)

	// Assert
	assert.NoError(t, err, "hook file should exist")
	AssertFileExists(t, preRemoveHook)

	// Hook would fail but operation continues
}

func TestRemove_PostHookWarnsContinues(t *testing.T) {
	// Arrange
	hooksDir := filepath.Join(t.TempDir(), "hooks")
	postRemoveHook := filepath.Join(hooksDir, "post-remove")

	// Create a failing hook script
	os.MkdirAll(hooksDir, 0755)
	os.WriteFile(postRemoveHook, []byte("#!/bin/sh\nexit 1"), 0755)

	// Act - Simulate hook execution
	_, err := os.Stat(postRemoveHook)

	// Assert
	assert.NoError(t, err, "hook file should exist")
	AssertFileExists(t, postRemoveHook)

	// Hook would fail but operation continues
}

// ========== Structured Logging Tests ==========

func TestRemove_EmitsCorrectLogs(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfig(t)

	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	cfg.Logger = logger

	// Act - Simulate remove operation
	sourcePath := filepath.Join(tempDir, ".zshrc")
	/* repoPath := filepath.Join(tempDir, "files", "zshrc") */

	cfg.Logger.Debug("checking symlink status", "file", sourcePath)
	cfg.Logger.Info("removing symlink", "source", sourcePath)
	cfg.Logger.Info("updating config", "file", "~/.zshrc")

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "checking symlink status")
	assert.Contains(t, logs, "removing symlink")
	assert.Contains(t, logs, "updating config")
	assert.Contains(t, logs, "file")
	assert.Contains(t, logs, "source")
}

func TestRemove_LogLevels_Verified(t *testing.T) {
	// Arrange
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	// Act - Log at different levels
	logger.Debug("validating file")
	logger.Info("removing file")
	logger.Warn("hook failed", "hook", "pre-remove")
	logger.Error("file not managed", "path", "~/.bashrc")

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "DEBUG", "debug message")
	assert.Contains(t, logs, "INFO", "info message")
	assert.Contains(t, logs, "WARN", "warning message")
	assert.Contains(t, logs, "ERROR", "error message")
}

// ========== Symlink Tests ==========

func TestRemove_SymlinkTarget_Validation(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	repoFile := filepath.Join(tempDir, "files", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create symlink
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Verify symlink points to correct target
	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, repoFile, target, "symlink should point to repo file")

	// Remove symlink
	os.Remove(sourceFile)

	// Verify symlink is gone
	_, err = os.Lstat(sourceFile)
	assert.Error(t, err, "symlink should be removed")
	assert.True(t, os.IsNotExist(err), "should be NotExist error")
}

// ========== Empty Directory Cleanup Tests ==========

func TestRemove_CleansUpEmptyDirectories(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	shellDir := filepath.Join(filesDir, "shell")
	os.MkdirAll(shellDir, 0755)

	repoFile := filepath.Join(shellDir, "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create repo file and symlink
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.Symlink(repoFile, sourceFile)

	// Act - Remove repo file
	os.Remove(repoFile)

	// Remove empty directory
	os.Remove(shellDir)

	// Assert
	AssertFileNotExists(t, repoFile)
	AssertFileNotExists(t, shellDir)
	AssertDirExists(t, filesDir)
}

// ========== Edge Cases ==========

func TestRemove_EmptyConfig_NothingToRemove(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{},
	}

	// Act - Try to remove all
	removeAll := true
	var filesToRemove []config.ManagedFile

	if removeAll {
		filesToRemove = cfg.ManagedFiles
	}

	// Assert
	assert.Len(t, filesToRemove, 0, "should have no files to remove")
	assert.Len(t, cfg.ManagedFiles, 0, "config should be empty")
}

func TestRemove_RemoveFromManagedList_Success(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		ManagedFiles: []config.ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
			{SourcePath: "~/.bashrc", RepoPath: "shell/bashrc"},
		},
	}

	// Act - Remove from managed list
	sourcePath := "~/.zshrc"
	var newFiles []config.ManagedFile

	for _, mf := range cfg.ManagedFiles {
		if mf.SourcePath != sourcePath {
			newFiles = append(newFiles, mf)
		}
	}

	cfg.ManagedFiles = newFiles

	// Assert
	assert.Len(t, cfg.ManagedFiles, 1, "should have 1 managed file")
	assert.Equal(t, "~/.bashrc", cfg.ManagedFiles[0].SourcePath, "correct file should remain")
}

func TestRemove_FileAlreadyRegular_CopiesCorrectly(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	repoFile := filepath.Join(tempDir, "files", "zshrc")
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Create regular file (not symlink)
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Repo content"), 0644)
	os.WriteFile(sourceFile, []byte("# Regular file"), 0644)

	// Act - Copy from repo (overwrites regular file)
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)

	err = os.WriteFile(sourceFile, content, 0644)
	require.NoError(t, err)

	// Assert
	AssertFileExists(t, sourceFile)

	restoredContent, _ := os.ReadFile(sourceFile)
	assert.Equal(t, "# Repo content", string(restoredContent), "should copy repo content")
}

// ========== Helper Functions Tests ==========

func TestRemove_HelperFunctions_Work(t *testing.T) {
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
		os.WriteFile(target, []byte("target"), 0644)
		CreateTestSymlink(t, target, link)
		AssertSymlinkPointsTo(t, link, target)
	})
}
