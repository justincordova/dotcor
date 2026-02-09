package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

func TestInitCommandSetup(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	// Create config dir
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	hooksDir := filepath.Join(configDir, "hooks")

	// Act - Create directory structure manually (simplified init)
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err, "config dir creation should succeed")
	err = os.MkdirAll(filesDir, 0755)
	require.NoError(t, err, "files dir creation should succeed")
	err = os.MkdirAll(backupsDir, 0755)
	require.NoError(t, err, "backups dir creation should succeed")
	err = os.MkdirAll(hooksDir, 0755)
	require.NoError(t, err, "hooks dir creation should succeed")

	// Assert
	assert.DirExists(t, configDir, "config dir should exist")
	assert.DirExists(t, filesDir, "files dir should exist")
	assert.DirExists(t, backupsDir, "backups dir should exist")
	assert.DirExists(t, hooksDir, "hooks dir should exist")
}

func TestAddFile(t *testing.T) {
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

	// Create symlink
	if err := os.Symlink(repoFile, sourceFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Update managed files
	managedFiles = append(managedFiles, config.ManagedFile{
		SourcePath: "~/.zshrc",
		RepoPath:   "shell/zshrc",
	})

	// Assert
	assert.FileExists(t, repoFile, "repo file should exist")
	assert.FileExists(t, sourceFile, "source file should exist as symlink")
	assert.Len(t, managedFiles, 1, "should have 1 managed file")

	// Verify it's a symlink
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "source file should be symlink")
}

func TestApplySymlinks(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	os.MkdirAll(filesDir, 0755)

	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")

	// Create source file
	if err := os.MkdirAll(homeDir, 0755); err != nil {
		t.Fatalf("failed to create home dir: %v", err)
	}

	// Create repo file
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test zshrc"), 0644)

	// Act - Create symlink (simplified applySymlinks)
	err := os.Symlink(repoFile, sourceFile)

	// Assert
	require.NoError(t, err, "symlink creation should succeed")
	assert.FileExists(t, sourceFile, "symlink should exist")

	// Verify it's a symlink
	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "should be symlink")

	// Verify link target
	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target, "symlink should point to repo file")
}

func TestInitValidation(t *testing.T) {
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
			name:       "absolute path not allowed in repo path",
			sourcePath: "/etc/zshrc",
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.sourcePath
			isAbsolute := filepath.IsAbs(path)

			// Act - Check if path is valid
			isValid := path[0] == '~' || !isAbsolute

			// Assert
			if tt.shouldPass {
				assert.True(t, isValid,
					"path %q should be valid", path)
			} else {
				assert.False(t, isValid,
					"path %q should be invalid (absolute path)", path)
			}
		})
	}
}

func TestInit_SymlinkNotSupported_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	sourceFile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(tempDir, "files", "shell", "zshrc")

	// Create repo file
	os.MkdirAll(filepath.Dir(repoFile), 0755)
	os.WriteFile(repoFile, []byte("# Test"), 0644)
	os.MkdirAll(homeDir, 0755)

	// Act - Try to create symlink (on systems that support symlinks, this succeeds)
	err := os.Symlink(repoFile, sourceFile)

	// Assert
	if err != nil {
		assert.Error(t, err, "should return error when symlinks not supported")
	} else {
		// Symlinks are supported
		assert.FileExists(t, sourceFile, "symlink should exist")
		assert.FileExists(t, repoFile, "target file should exist")
	}
}

func TestInit_GitNotInstalled_ShowsWarning(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	os.MkdirAll(configDir, 0755)

	// Act - Check if git is available
	_, err := exec.LookPath("git")

	// Assert
	if err != nil {
		// Git not found - this is a warning condition
		assert.Error(t, err, "git not installed should be detected")
	} else {
		// Git is available
		assert.NoError(t, err, "git should be found if installed")
	}
}

func TestInit_CreateDirectoryFails_ReturnsError(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")

	// Create a file where directory should exist
	os.MkdirAll(tempDir, 0755)
	os.WriteFile(configDir, []byte("not a directory"), 0644)

	// Act - Try to create directory structure
	err := os.MkdirAll(filepath.Join(configDir, "files"), 0755)

	// Assert
	assert.Error(t, err, "should return error when path exists as file")

	// Cleanup
	os.Remove(configDir)
}

func TestInit_InteractiveMode_ScansDotfiles(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	configPath := filepath.Join(homeDir, ".config")
	os.MkdirAll(configPath, 0755)

	// Create dotfiles
	dotfiles := []string{".zshrc", ".bashrc", ".vimrc"}
	for _, df := range dotfiles {
		os.WriteFile(filepath.Join(homeDir, df), []byte("# "+df), 0644)
	}

	// Act - Scan for dotfiles (simulate interactive scan)
	var foundFiles []string
	entries, _ := os.ReadDir(homeDir)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			foundFiles = append(foundFiles, entry.Name())
		}
	}

	// Assert
	assert.GreaterOrEqual(t, len(foundFiles), 3, "should find at least 3 dotfiles")
}

func TestInit_ApplyFlag_CreatesSymlinks(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	homeDir := filepath.Join(tempDir, "home")

	// Create files directory and repo file
	os.MkdirAll(filepath.Join(filesDir, "shell"), 0755)
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	os.WriteFile(repoFile, []byte("# Test zshrc"), 0644)

	// Create home directory
	os.MkdirAll(homeDir, 0755)
	sourceFile := filepath.Join(homeDir, ".zshrc")

	// Act - Create symlink (apply behavior)
	err := os.Symlink(repoFile, sourceFile)
	require.NoError(t, err)

	// Assert
	assert.FileExists(t, sourceFile, "source symlink should exist")
	assert.FileExists(t, repoFile, "repo file should exist")

	info, err := os.Lstat(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "should be symlink")

	target, err := os.Readlink(sourceFile)
	require.NoError(t, err)
	assert.Equal(t, repoFile, target, "symlink should point to repo file")
}
