package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
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
			got, err := ExpandPath(tt.input, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("ExpandPath() = %v, want %v", got, tt.want)
			}
			if tt.want == "" && !filepath.IsAbs(got) {
				t.Errorf("ExpandPath() = %v, expected absolute path", got)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Normalize separators for comparison
			got = strings.ReplaceAll(got, string(filepath.Separator), "/")
			want := strings.ReplaceAll(tt.want, string(filepath.Separator), "/")
			if got != want {
				t.Errorf("NormalizePath() = %v, want %v", got, want)
			}
		})
	}
}

func TestGenerateRepoPath(t *testing.T) {
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
			got, err := GenerateRepoPath(tt.sourcePath, tt.customPath, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateRepoPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Normalize separators for comparison
			got = strings.ReplaceAll(got, string(filepath.Separator), "/")
			if got != tt.want {
				t.Errorf("GenerateRepoPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeRelativeSymlink(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
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
			got, err := ComputeRelativeSymlink(tt.linkPath, tt.targetPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeRelativeSymlink() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("ComputeRelativeSymlink() = %v, want prefix %v", got, tt.wantPrefix)
			}
		})
	}
}

func TestGetCategoryByPrefix(t *testing.T) {
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
			got := getCategoryByPrefix(tt.filename)
			if got != tt.want {
				t.Errorf("getCategoryByPrefix(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}
