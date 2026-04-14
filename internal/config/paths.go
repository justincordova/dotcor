package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath converts absolute path to ~ notation
// Example: /Users/you/.zshrc -> ~/.zshrc
func NormalizePath(path string) (string, error) {
	// First expand the path to handle any env vars or ~
	expanded, err := ExpandPath(path, nil)
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
func ExpandPath(path string, cfg *Config) (string, error) {
	if cfg != nil && cfg.Logger != nil {
		cfg.Logger.Debug("expanding path", "path", path)
	}

	// First expand environment variables
	path = os.ExpandEnv(path)

	// Handle ~ notation
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}

		if path == "~" {
			if cfg != nil && cfg.Logger != nil {
				cfg.Logger.Debug("path expanded", "path", "~", "expanded", home)
			}
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

	expanded := filepath.Clean(absPath)
	if cfg != nil && cfg.Logger != nil {
		cfg.Logger.Debug("path expanded", "path", path, "expanded", expanded)
	}

	return expanded, nil
}

// GenerateRepoPath generates repository path for a source file
// Returns path relative to repository's files directory
// Example: ~/.zshrc -> .zshrc
//
// Only accepts paths under the user's home directory. Returns error for:
// - Paths outside home directory (e.g., /etc/hosts)
// - Nil Config parameter
// - Empty paths or paths that resolve to home directory
func GenerateRepoPath(sourcePath string, cfg *Config) (string, error) {
	// Validate cfg parameter
	if cfg == nil {
		return "", fmt.Errorf("config cannot be nil")
	}

	cfg.Logger.Debug("generating repo path", "source", sourcePath)

	// Expand source path to absolute
	expanded, err := ExpandPath(sourcePath, cfg)
	if err != nil {
		return "", fmt.Errorf("expanding path: %w", err)
	}

	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	// Validate path is under home directory
	if !strings.HasPrefix(expanded, home) {
		return "", fmt.Errorf("path must be under home directory: %s", sourcePath)
	}

	// Strip home directory prefix
	relPath := strings.TrimPrefix(expanded, home)
	relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

	// Handle edge case: path equals home directory
	if relPath == "" {
		return "", fmt.Errorf("path resolves to home directory: %s", sourcePath)
	}

	cfg.Logger.Debug("repo path generated", "source", sourcePath, "expanded", expanded, "repo", relPath)
	return relPath, nil
}

// ComputeRelativeSymlink computes relative path from symlink to target
// Example: link=~/.zshrc, target=~/.dotcor/files/shell/zshrc
//
//	returns: .dotcor/files/shell/zshrc
//
// Validates both paths are on same filesystem
func ComputeRelativeSymlink(linkPath, targetPath string) (string, error) {
	// Expand both paths
	expandedLink, err := ExpandPath(linkPath, nil)
	if err != nil {
		return "", fmt.Errorf("expanding link path: %w", err)
	}

	expandedTarget, err := ExpandPath(targetPath, nil)
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
	// Validate pattern is not empty
	if pattern == "" {
		return nil, fmt.Errorf("glob pattern cannot be empty")
	}

	// First expand ~ and env vars
	expanded, err := ExpandPath(pattern, nil)
	if err != nil {
		return nil, fmt.Errorf("expanding path: %w", err)
	}

	// Use filepath.Glob to expand the pattern
	matches, err := filepath.Glob(expanded)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files match pattern: %s", pattern)
	}

	// Filter out directories, only return files
	files := make([]string, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue // Skip files we cannot stat
		}
		if !info.IsDir() {
			normalized, _ := NormalizePath(match)
			if normalized != "" {
				files = append(files, normalized)
			} else {
				files = append(files, match)
			}
		}
	}

	return files, nil
}

// GetFilesRecursive returns all files in a directory recursively
func GetFilesRecursive(dir string) ([]string, error) {
	expanded, err := ExpandPath(dir, nil)
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
