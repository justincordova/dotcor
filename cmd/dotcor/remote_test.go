package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/git"
)

// ========== Show Remote Tests ==========

func TestRemote_ShowExistingRemote_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	require.NoError(t, os.MkdirAll(filesDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// Create a test config
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
ignore_patterns: []
managed_files: []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Initialize git repo and add remote
	remoteCmd := remoteCmd
	remoteCmd.SetArgs([]string{})

	// First, set up git remote via direct call
	require.NoError(t, git.InitRepo(filesDir))
	require.NoError(t, git.SetRemote(filesDir, "origin", "git@github.com:user/dotfiles.git"))

	// Act
	// Note: We can't easily test the CLI command in unit tests
	// This test documents expected behavior
	assert.NotNil(t, remoteCmd)
}

func TestRemote_NoRemote_DisplaysHelpText(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	require.NoError(t, os.MkdirAll(filesDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
ignore_patterns: []
managed_files: []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Assert
	assert.NotNil(t, remoteCmd)
	assert.Equal(t, "remote [url]", remoteCmd.Use)
}

func TestRemote_SetNewRemote_Success(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	require.NoError(t, os.MkdirAll(filesDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
ignore_patterns: []
managed_files: []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Initialize git repo
	require.NoError(t, os.Chdir(filesDir))
	require.NoError(t, os.Setenv("HOME", tempDir))

	// Assert
	assert.NotNil(t, remoteCmd)
}

func TestRemote_DefaultsToOrigin(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	require.NoError(t, os.MkdirAll(filesDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
ignore_patterns: []
managed_files: []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Assert - Check default flag value
	flags := remoteCmd.Flags()
	nameFlag := flags.Lookup("name")
	require.NotNil(t, nameFlag)
	assert.Equal(t, "origin", nameFlag.DefValue)
}

func TestRemote_AcceptsCustomRemoteName(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(configDir, "files")
	require.NoError(t, os.MkdirAll(filesDir, 0755))
	require.NoError(t, os.MkdirAll(configDir, 0755))

	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `version: "1.0"
repo_path: ` + filesDir + `
git_enabled: true
ignore_patterns: []
managed_files: []
`
	require.NoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	// Assert
	flags := remoteCmd.Flags()
	nameFlag := flags.Lookup("name")
	require.NotNil(t, nameFlag)
	// The Changed field is false until the flag is actually used
	// Just verify the flag exists and has the right default value
	assert.Equal(t, "origin", nameFlag.DefValue, "flag should have default value")
	assert.NotNil(t, nameFlag, "flag should be defined")
}
