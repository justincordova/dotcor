package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// categoryMap maps dotfile names to their categories for repo organization
var categoryMap = map[string]string{
	// Shell configurations
	".zshrc":        "shell",
	".zshenv":       "shell",
	".zprofile":     "shell",
	".zlogin":       "shell",
	".zlogout":      "shell",
	".bashrc":       "shell",
	".bash_profile": "shell",
	".bash_logout":  "shell",
	".bashenv":      "shell",
	".profile":      "shell",
	".sh_history":   "shell",

	// Git
	".gitconfig":        "git",
	".gitignore":        "git",
	".gitignore_global": "git",

	// SSH
	"ssh/config": "ssh",

	// Editors
	".vimrc":        "vim",
	".vim":          "vim",
	".nvimrc":       "nvim",
	".editorconfig": "editor",

	// Terminal multiplexers
	".tmux.conf": "tmux",
	".screenrc":  "screen",
}

// NormalizePath converts absolute path to ~ notation
// Example: /Users/you/.zshrc -> ~/.zshrc
func NormalizePath(path string) (string, error) {
	// First expand the path to handle any env vars or ~
	expanded, err := ExpandPath(path)
	if err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	// Clean both paths for consistent comparison
	expanded = filepath.Clean(expanded)
	home = filepath.Clean(home)

	// Check if path is under home directory
	if strings.HasPrefix(expanded, home) {
		// Replace home directory with ~
		relative := strings.TrimPrefix(expanded, home)
		if relative == "" {
			return "~", nil
		}
		// Ensure path starts with ~/
		if relative[0] == filepath.Separator {
			return "~" + relative, nil
		}
		return "~" + string(filepath.Separator) + relative, nil
	}

	// Return original path if not under home
	return expanded, nil
}

// ExpandPath converts ~ notation to absolute path
// Example: ~/.zshrc -> /Users/you/.zshrc
// Also handles environment variables: $XDG_CONFIG_HOME, %APPDATA%, etc.
func ExpandPath(path string) (string, error) {
	// First expand environment variables
	path = os.ExpandEnv(path)

	// Handle ~ notation
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}

		if path == "~" {
			return home, nil
		}

		// Replace ~ with home directory
		if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
			path = filepath.Join(home, path[2:])
		}
	}

	// Clean and return absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("getting absolute path: %w", err)
	}

	return filepath.Clean(absPath), nil
}

// GetRepoFilePath returns full path to file in repo
// Example: shell/zshrc -> /Users/you/.dotcor/files/shell/zshrc
func GetRepoFilePath(config *Config, repoPath string) (string, error) {
	expanded, err := ExpandPath(config.RepoPath)
	if err != nil {
		return "", err
	}

	return filepath.Join(expanded, repoPath), nil
}

// GenerateRepoPath creates repo path from source path with optional override
// Example: ~/.config/nvim/init.vim -> nvim/init.vim
// Example: ~/.zshrc -> shell/zshrc
// customPath parameter allows manual override (e.g., "custom/myshell/zshrc")
func GenerateRepoPath(sourcePath string, customPath string) (string, error) {
	// If custom path provided, use it
	if customPath != "" {
		return customPath, nil
	}

	// Expand the source path
	expanded, err := ExpandPath(sourcePath)
	if err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	// Strip home directory prefix
	relPath := strings.TrimPrefix(expanded, home)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	// Get the base filename
	filename := filepath.Base(relPath)

	// Check category map for exact match
	if category, ok := categoryMap[filename]; ok {
		// Strip leading dot from filename for repo
		repoFilename := strings.TrimPrefix(filename, ".")
		return filepath.Join(category, repoFilename), nil
	}

	// Check prefix matching for patterns
	category := getCategoryByPrefix(filename)

	// Handle .config/ directory specially
	if strings.HasPrefix(relPath, ".config"+string(filepath.Separator)) {
		// Strip .config/ prefix
		configPath := strings.TrimPrefix(relPath, ".config"+string(filepath.Separator))
		return configPath, nil
	}

	// Handle .local/share/ directory
	if strings.HasPrefix(relPath, ".local"+string(filepath.Separator)) {
		// Preserve structure but strip leading dot
		return strings.TrimPrefix(relPath, "."), nil
	}

	// If we found a category by prefix, use it
	if category != "misc" {
		repoFilename := strings.TrimPrefix(filename, ".")
		return filepath.Join(category, repoFilename), nil
	}

	// Default: use misc category with original filename (minus dot)
	repoFilename := strings.TrimPrefix(filename, ".")
	return filepath.Join("misc", repoFilename), nil
}

// getCategoryByPrefix returns category based on filename prefix
func getCategoryByPrefix(filename string) string {
	if strings.HasPrefix(filename, ".zsh") {
		return "shell"
	}
	if strings.HasPrefix(filename, ".bash") {
		return "shell"
	}
	if strings.HasPrefix(filename, ".vim") {
		return "vim"
	}
	if strings.HasPrefix(filename, ".nvim") {
		return "nvim"
	}
	if strings.HasPrefix(filename, ".git") {
		return "git"
	}
	if strings.HasPrefix(filename, ".tmux") {
		return "tmux"
	}
	return "misc"
}

// ComputeRelativeSymlink computes relative path from symlink to target
// Example: link=~/.zshrc, target=~/.dotcor/files/shell/zshrc
//
//	returns: .dotcor/files/shell/zshrc
//
// Validates both paths are on same filesystem
func ComputeRelativeSymlink(linkPath, targetPath string) (string, error) {
	// Expand both paths
	expandedLink, err := ExpandPath(linkPath)
	if err != nil {
		return "", fmt.Errorf("expanding link path: %w", err)
	}

	expandedTarget, err := ExpandPath(targetPath)
	if err != nil {
		return "", fmt.Errorf("expanding target path: %w", err)
	}

	// Get the directory containing the symlink
	linkDir := filepath.Dir(expandedLink)

	// Compute relative path from linkDir to target
	relPath, err := filepath.Rel(linkDir, expandedTarget)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}

	return relPath, nil
}

// ExpandGlob expands glob pattern to list of files
// Example: ~/.config/nvim/*.lua -> [~/.config/nvim/init.lua, ~/.config/nvim/plugins.lua]
func ExpandGlob(pattern string) ([]string, error) {
	// First expand ~ and env vars
	expanded, err := ExpandPath(pattern)
	if err != nil {
		// If expansion fails, try using pattern as-is (might still work for globs)
		expanded = pattern
	}

	// Use filepath.Glob to expand the pattern
	matches, err := filepath.Glob(expanded)
	if err != nil {
		return nil, fmt.Errorf("expanding glob pattern: %w", err)
	}

	// Filter out directories, only return files
	files := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we can't stat
		}
		if !info.IsDir() {
			files = append(files, match)
		}
	}

	return files, nil
}

// GetFilesRecursive returns all files in a directory recursively
func GetFilesRecursive(dir string) ([]string, error) {
	expanded, err := ExpandPath(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.Walk(expanded, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return files, nil
}
