package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultIgnorePatterns(t *testing.T) {
	patterns := GetDefaultIgnorePatterns()

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
	cfg, err := NewDefaultConfig()

	assert.NoError(t, err, "NewDefaultConfig() error")
	assert.True(t, cfg.GitEnabled, "GitEnabled should be true by default")
	assert.NotEmpty(t, cfg.IgnorePatterns, "IgnorePatterns should not be empty")
}

func TestNewDefaultConfigLoggerNotNil(t *testing.T) {
	cfg, err := NewDefaultConfig()
	if err != nil {
		t.Fatalf("NewDefaultConfig failed: %v", err)
	}
	if cfg.Logger == nil {
		t.Error("Logger should not be nil in NewDefaultConfig")
	}
}

func TestSaveConfigWithNilLogger(t *testing.T) {
	cfg := &Config{
		Logger: nil,
	}

	err := cfg.SaveConfig()
	if err != nil {
		t.Logf("SaveConfig returned error: %v", err)
	}
}

func TestExpandGlobErrorHandling(t *testing.T) {
	tests := []string{
		"",
		"[",
	}

	for _, pattern := range tests {
		result, err := ExpandGlob(pattern)
		if err == nil {
			t.Errorf("ExpandGlob(%q) should return error, got: %v", pattern, result)
		}
	}
}

func TestSaveConfig_AtomicWrite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	configDir := filepath.Join(tempDir, ".dotcor")
	configPath := filepath.Join(configDir, ".dotcorrc")

	cfg := &Config{
		GitEnabled:     true,
		IgnorePatterns: GetDefaultIgnorePatterns(),
	}

	t.Setenv("HOME", tempDir)

	err = cfg.SaveConfig()

	assert.NoError(t, err, "SaveConfig() should not error")
	assert.FileExists(t, configPath, "SaveConfig() should create config file")

	data, err := os.ReadFile(configPath)
	assert.NoError(t, err, "failed to read config file")
	assert.Contains(t, string(data), "git_enabled:", "SaveConfig() should write git_enabled")
}

func TestLoadConfig_CorruptFile_ReturnsError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	configDir := filepath.Join(tempDir, ".dotcor")
	err = os.MkdirAll(configDir, 0755)
	assert.NoError(t, err, "failed to create config dir")

	configPath := filepath.Join(configDir, ".dotcorrc")
	err = os.WriteFile(configPath, []byte("corrupt: yaml: content: [invalid"), 0644)
	assert.NoError(t, err, "failed to write corrupt config")

	t.Setenv("HOME", tempDir)

	_, err = LoadConfig()

	assert.Error(t, err, "LoadConfig() should error for corrupt YAML")
	assert.Contains(t, err.Error(), "parsing config file", "Error should indicate parsing failure")
}

func TestLoadConfigFromPath_LoadsCustomPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	assert.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	customConfigPath := filepath.Join(tempDir, "custom-config.yaml")
	customConfigContent := "git_enabled: true\ngit_remote: git@github.com:user/dotfiles.git\n"
	err = os.WriteFile(customConfigPath, []byte(customConfigContent), 0644)
	assert.NoError(t, err, "failed to write custom config")

	cfg, err := LoadConfigFromPath(customConfigPath)

	assert.NoError(t, err, "LoadConfigFromPath() should not error")
	assert.NotNil(t, cfg, "LoadConfigFromPath() should return config")
	assert.True(t, cfg.GitEnabled, "LoadConfigFromPath() should load git_enabled")
	assert.Equal(t, "git@github.com:user/dotfiles.git", cfg.GitRemote, "LoadConfigFromPath() should load git_remote")
}

func TestLoadConfigFromPath_NonExistentPath_ReturnsError(t *testing.T) {
	nonExistentPath := "/tmp/nonexistent-config-12345.yaml"

	_, err := LoadConfigFromPath(nonExistentPath)

	assert.Error(t, err, "LoadConfigFromPath() should error for non-existent file")
	assert.Contains(t, err.Error(), "reading config file", "Error should indicate file reading failure")
}

func TestConfigFilePermissions(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.Setenv("HOME", tempDir); err != nil {
		t.Fatalf("failed to set HOME: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("HOME"); err != nil {
			t.Logf("failed to unset HOME: %v", err)
		}
	}()

	cfg, err := NewDefaultConfig()
	require.NoError(t, err)

	err = cfg.SaveConfig()
	require.NoError(t, err)

	configPath, _ := GetConfigPath()
	info, err := os.Stat(configPath)
	require.NoError(t, err)

	mode := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), mode, "Config should be owner-only readable")
}

func TestGetConfigDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configDir, err := GetConfigDir()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, ".dotcor"), configDir)
}

func TestGetConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	configPath, err := GetConfigPath()
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, ".dotcor", ".dotcorrc"), configPath)
}

func TestLoadConfig_NoFile_ReturnsDefault(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	cfg, err := LoadConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.GitEnabled)
}
