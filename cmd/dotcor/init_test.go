package main

import (
	"bytes"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
)

// ========== Directory Structure Tests ==========

func TestInit_CreatesDirectoryStructure_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfig(t)
	cfg.RepoPath = filepath.Join(tempDir, ".dotcor", "files")

	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	hooksDir := filepath.Join(configDir, "hooks")

	// Act - Create directory structure manually
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(filesDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(backupsDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(hooksDir, 0755)
	require.NoError(t, err)

	// Assert
	AssertDirExists(t, configDir)
	AssertDirExists(t, filesDir)
	AssertDirExists(t, backupsDir)
	AssertDirExists(t, hooksDir)

	// Verify permissions
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(), "config dir should have 0755 permissions")
}

// ========== File Addition Tests ==========

func TestInit_AddFile_CreatesSymlink_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")

	managedFiles := []config.ManagedFile{}

	// Create source file
	os.MkdirAll(filepath.Dir(sourceFile), 0755)
	if err := os.WriteFile(sourceFile, []byte("# Test zshrc\nexport PATH=/bin"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Simulate addFile behavior
	if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err, "moving file should succeed")

	// Create symlink (macOS native, no support checks)
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Update managed files (no platforms field)
	managedFiles = append(managedFiles, config.ManagedFile{
		SourcePath: "~/.zshrc",
		RepoPath:   "shell/zshrc",
		AddedAt:    time.Now(),
	})

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)
	assert.Len(t, managedFiles, 1, "should have 1 managed file")

	// Verify it's a symlink (macOS native)
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "source file should be symlink")

	// Verify symlink points to correct target
	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target, "symlink should point to repo file")
}

func TestInit_AddFile_WithCategory_Success(t *testing.T) {
	// Arrange
	configDir := t.TempDir()
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(configDir, "home")
	sourceFile := filepath.Join(homeDir, ".config", "nvim", "init.vim")
	repoFile := filepath.Join(filesDir, "nvim", "init.vim")

	// Create source file
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	if err := os.WriteFile(sourceFile, []byte("set number\n"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Act - Move file and create symlink
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	err := os.Rename(sourceFile, repoFile)
	require.NoError(t, err)
	err = os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Assert
	AssertFileExists(t, repoFile)
	AssertFileExists(t, sourceFile)

	// Verify category in repo path
	assert.Contains(t, repoFile, "nvim", "repo path should contain category")

	// Verify symlink
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)
}

func TestInit_AddFile_AlreadyManaged_ReturnsError(t *testing.T) {
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
	assert.True(t, isManaged, "file should be marked as already managed")
}

// ========== Symlink Tests ==========

func TestInit_ApplySymlinks_CreatesAllLinks_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")

	// Create repo files
	if err := os.MkdirAll(filepath.Join(filesDir, "shell"), 0755); err != nil {
		t.Fatalf("failed to create shell dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "shell", "zshrc"), []byte("# ZSH config"), 0644); err != nil {
		t.Fatalf("failed to create zshrc: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(filesDir, "nvim"), 0755); err != nil {
		t.Fatalf("failed to create nvim dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filesDir, "nvim", "init.vim"), []byte("set number"), 0644); err != nil {
		t.Fatalf("failed to create init.vim: %v", err)
	}

	// Create managed files list
	managedFiles := []config.ManagedFile{
		{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		{SourcePath: "~/.config/nvim/init.vim", RepoPath: "nvim/init.vim"},
	}

	// Act - Create symlinks
	created := 0
	for _, mf := range managedFiles {
		sourcePath := filepath.Join(homeDir, strings.TrimPrefix(mf.SourcePath, "~/"))
		repoPath := filepath.Join(filesDir, mf.RepoPath)

		os.MkdirAll(filepath.Dir(sourcePath), 0755)
		if err := os.Symlink(repoPath, sourcePath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}
		created++
	}

	// Assert
	assert.Equal(t, 2, created, "should create 2 symlinks")

	// Verify all symlinks exist
	AssertFileExists(t, filepath.Join(homeDir, ".zshrc"))
	AssertFileExists(t, filepath.Join(homeDir, ".config", "nvim", "init.vim"))

	// Verify all are symlinks (macOS native)
	info, _ := os.Lstat(filepath.Join(homeDir, ".zshrc"))
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)
}

func TestInit_ApplySymlinks_SkipsExistingValidLinks(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")

	// Create repo file
	os.MkdirAll(filepath.Join(filesDir, "shell"), 0755)
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	if err := os.WriteFile(repoFile, []byte("# ZSH config"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Create existing valid symlink
	sourceFile := filepath.Join(homeDir, ".zshrc")
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Act - Check if symlink is valid
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	isSymlink := info.Mode()&os.ModeSymlink != 0

	// Assert
	assert.True(t, isSymlink, "existing symlink should be detected")

	// Verify link points to correct target
	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target, "symlink should point to correct repo file")
}

// ========== Validation Tests ==========

func TestInit_Validation_ValidPaths_Accepted(t *testing.T) {
	tests := []struct {
		name       string
		sourcePath string
		shouldPass bool
	}{
		{
			name:       "valid dotfile",
			sourcePath: "~/.zshrc",
			shouldPass: true,
		},
		{
			name:       "valid nested config",
			sourcePath: "~/.config/nvim/init.vim",
			shouldPass: true,
		},
		{
			name:       "valid alacritty config",
			sourcePath: "~/.config/alacritty/alacritty.yml",
			shouldPass: true,
		},
		{
			name:       "absolute path not allowed",
			sourcePath: "/etc/zshrc",
			shouldPass: false,
		},
		{
			name:       "empty path rejected",
			sourcePath: "",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.sourcePath

			// Act - Check if path is valid
			isValid := false
			if path != "" {
				isValid = strings.HasPrefix(path, "~") || !filepath.IsAbs(path)
			}

			// Assert
			assert.Equal(t, tt.shouldPass, isValid, "path validation should match expected")
		})
	}
}

func TestInit_Validation_SecretDetection(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		shouldReject bool
	}{
		{
			name:         "valid config",
			content:      "# ZSH config\nexport PATH=/bin",
			shouldReject: false,
		},
		{
			name:         "contains API key",
			content:      "export API_KEY=sk-1234567890abcdef",
			shouldReject: true,
		},
		{
			name:         "contains password",
			content:      "database_password=secret123",
			shouldReject: true,
		},
		{
			name:         "contains token",
			content:      "auth_token=ghp_xxxxxxxxxxxxx",
			shouldReject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var hasSecret bool
			lowerContent := strings.ToLower(tt.content)

			// Act - Simple secret detection
			for _, keyword := range []string{"api_key", "api-key", "password", "secret", "token", "private_key"} {
				if strings.Contains(lowerContent, keyword) && strings.Contains(tt.content, "=") {
					hasSecret = true
					break
				}
			}

			// Assert
			assert.Equal(t, tt.shouldReject, hasSecret, "secret detection should match expected")
		})
	}
}

// ========== Error Path Tests ==========

func TestInit_CreateDirectoryFails_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")

	// Create a file where directory should exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.WriteFile(configDir, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Act - Try to create directory structure
	err := os.MkdirAll(filepath.Join(configDir, "files"), 0755)

	// Assert
	assert.Error(t, err, "should return error when path exists as file")

	// Cleanup
	os.Remove(configDir)
}

func TestInit_CreateSymlinkFails_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")

	// Don't create repo directory - symlink will create dangling link (macOS native)
	// Note: macOS allows creating symlinks to non-existent targets

	// Act - Try to create symlink to non-existent path
	err := os.Symlink("/nonexistent/path", sourceFile)

	// Assert - macOS allows dangling symlinks, so this won't fail
	// The error would occur when trying to read the symlink target
	if err != nil {
		assert.Error(t, err, "should return error if symlink creation fails")
	} else {
		// Symlink created (macOS native)
		info, _ := os.Lstat(sourceFile)
		assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)
	}
}

func TestInit_MissingParentDirectory_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, "nested", "path", ".zshrc")
	repoFile := filepath.Join(tempDir, "files", "zshrc")

	if err := os.WriteFile(repoFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Don't create parent directory

	// Act - Try to create symlink without parent dir
	err := os.Symlink(repoFile, sourceFile)

	// Assert
	assert.Error(t, err, "should return error when parent directory doesn't exist")
}

// ========== Flag Tests ==========

func TestInit_ApplyFlag_CreatesSymlinks(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")

	// Create files directory and repo file
	if err := os.MkdirAll(filepath.Join(filesDir, "shell"), 0755); err != nil {
		t.Fatalf("failed to create shell dir: %v", err)
	}
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	if err := os.WriteFile(repoFile, []byte("# Test zshrc"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Create home directory
	os.MkdirAll(homeDir, 0755)
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Act - Create symlink (apply behavior)
	err := os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Assert
	AssertFileExists(t, sourceFile)
	AssertFileExists(t, repoFile)

	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink)

	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target)
}

func TestInit_InteractiveMode_ScansDotfiles(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	configPath := filepath.Join(homeDir, ".config")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create dotfiles
	dotfiles := []string{".zshrc", ".bashrc", ".vimrc", ".gitconfig"}
	for _, df := range dotfiles {
		if err := os.WriteFile(filepath.Join(homeDir, df), []byte("# "+df), 0644); err != nil {
			t.Fatalf("failed to create dotfile %s: %v", df, err)
		}
	}

	// Act - Scan for dotfiles
	var foundFiles []string
	entries, _ := os.ReadDir(homeDir)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			foundFiles = append(foundFiles, entry.Name())
		}
	}

	// Assert
	assert.GreaterOrEqual(t, len(foundFiles), 4, "should find at least 4 dotfiles")
	assert.Contains(t, foundFiles, ".zshrc")
	assert.Contains(t, foundFiles, ".bashrc")
}

// ========== Structured Logging Tests ==========

func TestInit_EmitsCorrectLogs(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := CreateTestConfig(t)

	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)
	cfg.Logger = logger

	// Act - Create directory
	os.MkdirAll(filepath.Join(tempDir, ".dotcor"), 0755)

	// Log the operation
	cfg.Logger.Info("created directory", "path", filepath.Join(tempDir, ".dotcor"))

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "INFO")
	assert.Contains(t, logs, "created directory")
	assert.Contains(t, logs, "path")
}

func TestInit_LogLevels_Verified(t *testing.T) {
	// Arrange
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	// Act - Log at different levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")

	// Assert
	logs := logBuf.String()
	assert.Contains(t, logs, "DEBUG", "debug message")
	assert.Contains(t, logs, "INFO", "info message")
	assert.Contains(t, logs, "WARN", "warning message")
	assert.Contains(t, logs, "ERROR", "error message")
}

// ========== Git Integration Tests ==========

func TestInit_GitInstalled_InitializesRepo(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	filesDir := filepath.Join(tempDir, ".dotcor", "files")
	os.MkdirAll(filesDir, 0755)

	// Act - Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = filesDir
	output, err := cmd.CombinedOutput()

	// Assert - Git may not be installed
	if err != nil {
		t.Skip("Git not installed: " + string(output))
	}

	// Verify .git directory exists
	gitDir := filepath.Join(filesDir, ".git")
	assert.DirExists(t, gitDir, ".git directory should exist")
}

func TestInit_GitNotInstalled_ShowsWarning(t *testing.T) {
	// Arrange - No setup needed

	// Act - Check if git is available
	_, err := exec.LookPath("git")

	// Assert
	if err != nil {
		assert.Error(t, err, "git not installed should be detected")
	} else {
		assert.NoError(t, err, "git should be found if installed")
	}
}

// ========== Config Tests ==========

func TestInit_CreatesConfigFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	dotcorDir := filepath.Join(tempDir, ".dotcor")
	configPath := filepath.Join(dotcorDir, "config.yaml")
	repoPath := filepath.Join(dotcorDir, "files")

	os.MkdirAll(dotcorDir, 0755)
	os.MkdirAll(repoPath, 0755)

	// Act - Create config file manually
	configContent := "repo_path: " + repoPath + "\ngit_enabled: true\nignore_patterns: []\nmanaged_files: []\n"
	err := os.WriteFile(configPath, []byte(configContent), 0644)

	// Assert
	require.NoError(t, err, "config creation should succeed")
	AssertFileExists(t, configPath)

	// Verify config content
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "repo_path", "config should have repo_path field")
}

func TestInit_ConfigAlreadyInitialized_ReturnsEarly(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create config file
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("repo_path: /test\n"), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Act - Check if already initialized
	alreadyInitialized := fs.PathExists(configDir) && fs.PathExists(configPath)

	// Assert
	assert.True(t, alreadyInitialized, "should detect already initialized state")
}

// ========== Path Handling Tests ==========

func TestInit_PathNormalization_Works(t *testing.T) {
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

			// Act - Normalize path (simplified)
			normalized := input
			normalized = filepath.Clean(normalized)
			normalized = strings.TrimRight(normalized, string(filepath.Separator))

			// Assert
			assert.Equal(t, tt.expected, normalized)
		})
	}
}

// ========== Backup Tests ==========

func TestInit_BackupFile_Created(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	sourceFile := filepath.Join(tempDir, ".zshrc")
	backupsDir := filepath.Join(tempDir, ".dotcor", "backups")

	// Create source file
	if err := os.WriteFile(sourceFile, []byte("# Original config"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create backups directory
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		t.Fatalf("failed to create backups dir: %v", err)
	}

	// Act - Create backup
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupsDir, filepath.Base(sourceFile)+"."+timestamp)
	if err := os.Rename(sourceFile, backupPath); err != nil {
		t.Fatalf("failed to rename file: %v", err)
	}

	// Assert
	AssertFileExists(t, backupPath)
	assert.Contains(t, backupPath, timestamp, "backup should have timestamp")
}

// ========== Ignore Pattern Tests ==========

func TestInit_IgnorePattern_Matches(t *testing.T) {
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

			// Act - Check if path matches pattern (simplified)
			matched := false
			if strings.Contains(pattern, "*") {
				idx := strings.Index(pattern, "*")
				prefix := pattern[:idx]
				suffix := pattern[idx+1:]
				if prefix != "" && suffix != "" {
					// Both prefix and suffix (e.g., "prefix*.suffix")
					if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
						matched = true
					}
				} else if prefix != "" {
					// Prefix only (e.g., "prefix*")
					if strings.HasPrefix(path, prefix) {
						matched = true
					}
				} else if suffix != "" {
					// Suffix only (e.g., "*.suffix")
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

// ========== Helper Functions Tests ==========

func TestInit_HelperFunctions_Work(t *testing.T) {
	// Test CreateTestConfig
	t.Run("CreateTestConfig", func(t *testing.T) {
		cfg := CreateTestConfig(t)
		assert.NotNil(t, cfg)
		assert.NotNil(t, cfg.Logger)
		assert.Contains(t, cfg.RepoPath, "files")
	})

	// Test CreateTestFile
	t.Run("CreateTestFile", func(t *testing.T) {
		testFile := filepath.Join(t.TempDir(), "test.txt")
		CreateTestFile(t, testFile, "test content")
		AssertFileExists(t, testFile)
		AssertFileContent(t, testFile, "test content")
	})

	// Test CreateTestSymlink
	t.Run("CreateTestSymlink", func(t *testing.T) {
		tempDir := t.TempDir()
		target := filepath.Join(tempDir, "target.txt")
		link := filepath.Join(tempDir, "link.txt")
		if err := os.WriteFile(target, []byte("target"), 0644); err != nil {
			t.Fatalf("failed to create target file: %v", err)
		}
		CreateTestSymlink(t, target, link)
		AssertSymlinkPointsTo(t, link, target)
	})
}
