package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandPath(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get home dir")

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
	home, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get home dir")

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
	tests := []struct {
		name       string
		sourcePath string
		customPath string
		want       string
		wantErr    bool
	}{
		{
			name:       "zshrc goes to shell",
			sourcePath: "~/.zshrc",
			customPath: "",
			want:       "shell/zshrc",
		},
		{
			name:       "bashrc goes to shell",
			sourcePath: "~/.bashrc",
			customPath: "",
			want:       "shell/bashrc",
		},
		{
			name:       "gitconfig goes to git",
			sourcePath: "~/.gitconfig",
			customPath: "",
			want:       "git/gitconfig",
		},
		{
			name:       "vimrc goes to vim",
			sourcePath: "~/.vimrc",
			customPath: "",
			want:       "vim/vimrc",
		},
		{
			name:       "tmux.conf goes to tmux",
			sourcePath: "~/.tmux.conf",
			customPath: "",
			want:       "tmux/tmux.conf",
		},
		{
			name:       "config dir stripped",
			sourcePath: "~/.config/nvim/init.lua",
			customPath: "",
			want:       "nvim/init.lua",
		},
		{
			name:       "custom path override",
			sourcePath: "~/.zshrc",
			customPath: "custom/myshell",
			want:       "custom/myshell",
		},
		{
			name:       "unknown file goes to misc",
			sourcePath: "~/.obscurefile",
			customPath: "",
			want:       "misc/obscurefile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := GenerateRepoPath(tt.sourcePath, tt.customPath, nil)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "GenerateRepoPath() should return error")
			} else {
				assert.NoError(t, err, "GenerateRepoPath() should not return error")
			}

			// Normalize separators for comparison
			got = strings.ReplaceAll(got, string(filepath.Separator), "/")
			assert.Equal(t, tt.want, got, "GenerateRepoPath() result")
		})
	}
}

func TestComputeRelativeSymlink(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get home dir")

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

func TestGetCategoryByPrefix(t *testing.T) {
	// Arrange
	tests := []struct {
		filename string
		want     string
	}{
		{".zshrc", "shell"},
		{".zsh_history", "shell"},
		{".bashrc", "shell"},
		{".bash_profile", "shell"},
		{".vimrc", "vim"},
		{".vim", "vim"},
		{".nvimrc", "nvim"},
		{".gitconfig", "git"},
		{".gitignore", "git"},
		{".tmux.conf", "tmux"},
		{".randomfile", "misc"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Act
			got := getCategoryByPrefix(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got, "getCategoryByPrefix() result")
		})
	}
}
