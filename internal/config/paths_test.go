package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testConfig() *Config {
	return &Config{
		Logger:         slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
		IgnorePatterns: GetDefaultIgnorePatterns(),
	}
}

func TestExpandPath(t *testing.T) {
	// Arrange
	home := os.Getenv("HOME")
	if home == "" {
		t.Setenv("HOME", "/tmp/testuser")
		home = "/tmp/testuser"
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "tilde only",
			input: "~",
			want:  home,
		},
		{
			name:  "tilde with path",
			input: "~/.zshrc",
			want:  filepath.Join(home, ".zshrc"),
		},
		{
			name:  "tilde with nested path",
			input: "~/.config/nvim/init.lua",
			want:  filepath.Join(home, ".config", "nvim", "init.lua"),
		},
		{
			name:  "absolute path unchanged",
			input: "/etc/hosts",
			want:  "/etc/hosts",
		},
		{
			name:  "relative path becomes absolute",
			input: "foo/bar",
			want:  "", // Will check it's absolute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := ExpandPath(tt.input, nil)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "ExpandPath() should return error")
			} else {
				assert.NoError(t, err, "ExpandPath() should not return error")
			}

			if tt.want != "" {
				assert.Equal(t, tt.want, got, "ExpandPath() result")
			} else {
				assert.True(t, filepath.IsAbs(got), "ExpandPath() should return absolute path")
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	// Arrange
	home := os.Getenv("HOME")
	if home == "" {
		t.Setenv("HOME", "/tmp/testuser")
		home = "/tmp/testuser"
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "home dir becomes tilde",
			input: filepath.Join(home, ".zshrc"),
			want:  "~/.zshrc",
		},
		{
			name:  "tilde stays tilde",
			input: "~/.zshrc",
			want:  "~/.zshrc",
		},
		{
			name:  "nested path normalized",
			input: filepath.Join(home, ".config", "nvim", "init.lua"),
			want:  "~/.config/nvim/init.lua",
		},
		{
			name:  "outside home stays absolute",
			input: "/etc/hosts",
			want:  "/etc/hosts",
		},
		{
			// Regression: HasPrefix without a separator boundary used to
			// treat "/tmp/testuser-other" as living under home
			// "/tmp/testuser" and emit a malformed "~-other" path.
			name:  "sibling sharing home prefix stays absolute",
			input: home + "-other/file.txt",
			want:  home + "-other/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := NormalizePath(tt.input)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "NormalizePath() should return error")
			} else {
				assert.NoError(t, err, "NormalizePath() should not return error")
			}

			// Normalize separators for comparison
			got = strings.ReplaceAll(got, string(filepath.Separator), "/")
			want := strings.ReplaceAll(tt.want, string(filepath.Separator), "/")
			assert.Equal(t, want, got, "NormalizePath() result")
		})
	}
}

func TestGenerateRepoPath(t *testing.T) {
	// Arrange
	if os.Getenv("HOME") == "" {
		t.Setenv("HOME", "/tmp/testuser")
	}
	tests := []struct {
		name       string
		sourcePath string
		wantRepo   string
		wantErr    bool
		nilConfig  bool
	}{
		{
			name:       "flat dotfile in home",
			sourcePath: "~/.zshrc",
			wantRepo:   ".zshrc",
			wantErr:    false,
			nilConfig:  false,
		},
		{
			name:       "nested config file",
			sourcePath: "~/.config/nvim/init.vim",
			wantRepo:   ".config/nvim/init.vim",
			wantErr:    false,
			nilConfig:  false,
		},
		{
			name:       "ssh config file",
			sourcePath: "~/.ssh/config",
			wantRepo:   ".ssh/config",
			wantErr:    false,
			nilConfig:  false,
		},
		{
			name:       "system file outside home",
			sourcePath: "/etc/hosts",
			wantRepo:   "",
			wantErr:    true,
			nilConfig:  false,
		},
		{
			name:       "nil config returns error",
			sourcePath: "~/.zshrc",
			wantRepo:   "",
			wantErr:    true,
			nilConfig:  true,
		},
		{
			name:       "path resolves to home directory returns error",
			sourcePath: "~",
			wantRepo:   "",
			wantErr:    true,
			nilConfig:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			var cfg *Config
			if !tt.nilConfig {
				cfg = testConfig()
			}

			// Act
			got, err := GenerateRepoPath(tt.sourcePath, cfg)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "GenerateRepoPath() should return error")
			} else {
				assert.NoError(t, err, "GenerateRepoPath() should not return error")
			}
			assert.Equal(t, tt.wantRepo, got, "GenerateRepoPath() result")
		})
	}
}

func TestComputeRelativeSymlink(t *testing.T) {
	// Arrange
	home := os.Getenv("HOME")
	if home == "" {
		t.Setenv("HOME", "/tmp/testuser")
		home = "/tmp/testuser"
	}

	tests := []struct {
		name       string
		linkPath   string
		targetPath string
		wantPrefix string // Check prefix since exact path varies
		wantErr    bool
	}{
		{
			name:       "home to dotcor files",
			linkPath:   filepath.Join(home, ".zshrc"),
			targetPath: filepath.Join(home, ".dotcor", "files", "shell", "zshrc"),
			wantPrefix: ".dotcor",
		},
		{
			name:       "nested config path",
			linkPath:   filepath.Join(home, ".config", "nvim", "init.lua"),
			targetPath: filepath.Join(home, ".dotcor", "files", "nvim", "init.lua"),
			wantPrefix: "..", // Goes up from .config/nvim
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := ComputeRelativeSymlink(tt.linkPath, tt.targetPath)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "ComputeRelativeSymlink() should return error")
			} else {
				assert.NoError(t, err, "ComputeRelativeSymlink() should not return error")
			}

			assert.True(t, strings.HasPrefix(got, tt.wantPrefix), "ComputeRelativeSymlink() result should have prefix")
		})
	}
}
