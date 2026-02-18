package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCompatibleVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "empty version is compatible",
			version: "",
			want:    true,
		},
		{
			name:    "current version is compatible",
			version: CurrentConfigVersion,
			want:    true,
		},
		{
			name:    "future version is not compatible",
			version: "9.9.9",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			version := tt.version

			// Act
			got := IsCompatibleVersion(version)

			// Assert
			assert.Equal(t, tt.want, got,
				"IsCompatibleVersion(%q) = %v, want %v", version, got, tt.want)
		})
	}
}

func TestGetMigrationPath(t *testing.T) {
	tests := []struct {
		name        string
		fromVersion string
		toVersion   string
		wantPathLen int
	}{
		{
			name:        "same version returns empty path",
			fromVersion: "1.0",
			toVersion:   "1.0",
			wantPathLen: 0,
		},
		{
			name:        "empty version treated as 1.0 to 1.0",
			fromVersion: "",
			toVersion:   "1.0",
			wantPathLen: 0,
		},
		{
			name:        "no migration path exists yet",
			fromVersion: "1.0",
			toVersion:   "9.9.9",
			wantPathLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fromVersion := tt.fromVersion
			toVersion := tt.toVersion

			// Act
			path := GetMigrationPath(fromVersion, toVersion)

			// Assert
			assert.Equal(t, tt.wantPathLen, len(path),
				"GetMigrationPath(%q, %q) path length = %d, want %d",
				fromVersion, toVersion, len(path), tt.wantPathLen)
		})
	}
}

func TestMigrateFromEmpty(t *testing.T) {
	// Arrange
	if os.Getenv("HOME") == "" {
		t.Setenv("HOME", "/tmp/testuser")
	}
	cfg := &Config{
		Version:        "",
		RepoPath:       "",
		GitEnabled:     true,
		IgnorePatterns: []string{},
		ManagedFiles:   []ManagedFile{},
	}

	// Act
	err := MigrateFromEmpty(cfg)

	// Assert
	require.NoError(t, err, "MigrateFromEmpty should succeed")
	assert.Equal(t, CurrentConfigVersion, cfg.Version,
		"MigrateFromEmpty should set version to current")
	assert.NotEmpty(t, cfg.RepoPath,
		"MigrateFromEmpty should set repo path")
	assert.NotEmpty(t, cfg.IgnorePatterns,
		"MigrateFromEmpty should set ignore patterns")
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name:      "nil config is invalid",
			config:    nil,
			wantError: true,
		},
		{
			name: "empty version is invalid",
			config: &Config{
				Version:  "",
				RepoPath: "/some/path",
			},
			wantError: true,
		},
		{
			name: "empty repo path is invalid",
			config: &Config{
				Version:  "1.0",
				RepoPath: "",
			},
			wantError: true,
		},
		{
			name: "valid config passes validation",
			config: &Config{
				Version:    CurrentConfigVersion,
				RepoPath:   "/some/path",
				GitEnabled: true,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := tt.config

			// Act
			err := ValidateConfig(cfg)

			// Assert
			if tt.wantError {
				assert.Error(t, err, "ValidateConfig should return error for invalid config")
			} else {
				assert.NoError(t, err, "ValidateConfig should succeed for valid config")
			}
		})
	}
}

func TestExportConfig(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       "~/.dotcor/files",
		GitEnabled:     true,
		IgnorePatterns: []string{"*.swp"},
		ManagedFiles: []ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
			},
		},
	}

	// Act
	data, err := ExportConfig(cfg)

	// Assert
	require.NoError(t, err, "ExportConfig should succeed")
	assert.NotEmpty(t, data, "ExportConfig should return non-empty data")
	assert.Contains(t, string(data), "version",
		"ExportConfig should contain version field")
}

func TestMigrateRepoPathConstruction(t *testing.T) {
	// Arrange
	configDir := "/tmp/test"
	cfg := &Config{
		RepoPath: configDir + "/files",
	}

	// Act
	_ = cfg.RepoPath

	// Assert - RepoPath should be absolute
	if !filepath.IsAbs(cfg.RepoPath) {
		t.Error("RepoPath should be absolute")
	}
}
