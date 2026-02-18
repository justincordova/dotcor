package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ShouldIgnore checks if file matches any ignore patterns
// Returns (matched, matchedPattern)
func ShouldIgnore(path string, patterns []string) (bool, string) {
	filename := filepath.Base(path)

	for _, pattern := range patterns {
		// Try matching against filename
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true, pattern
		}

		// Also try matching against full path for patterns like ".env.*"
		matched, err = filepath.Match(pattern, path)
		if err == nil && matched {
			return true, pattern
		}

		// Handle patterns with directory separators
		if strings.Contains(pattern, string(filepath.Separator)) || strings.Contains(pattern, "/") {
			// Normalize separators and try match
			normalizedPattern := filepath.FromSlash(pattern)
			matched, err = filepath.Match(normalizedPattern, path)
			if err == nil && matched {
				return true, pattern
			}
		}
	}

	return false, ""
}

// MatchesPattern checks if path matches a single glob pattern
func MatchesPattern(path, pattern string) bool {
	// Try matching filename
	filename := filepath.Base(path)
	matched, err := filepath.Match(pattern, filename)
	if err == nil && matched {
		return true
	}

	// Try matching full path
	matched, err = filepath.Match(pattern, path)
	if err == nil && matched {
		return true
	}

	// Handle patterns with directory separators
	if strings.Contains(pattern, "/") {
		normalizedPattern := filepath.FromSlash(pattern)
		matched, err = filepath.Match(normalizedPattern, path)
		if err == nil && matched {
			return true
		}
	}

	return false
}

// LoadGitignorePatterns loads patterns from a .gitignore-style file
// Supports comments (#) and blank lines
func LoadGitignorePatterns(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	var patterns []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// IsSecretFile checks if filename indicates a secret file
func IsSecretFile(filename string) bool {
	secretPatterns := []string{
		// Private keys
		"id_rsa", "id_rsa.*",
		"id_ed25519", "id_ed25519.*",
		"id_dsa", "id_dsa.*",
		"id_ecdsa", "id_ecdsa.*",
		"*.key", "*.pem", "*.p12", "*.pfx", "*.ppk",

		// Environment files
		".env", ".env.*",

		// Credential files
		"credentials", "credentials.*",
		"*.credentials",
		"secret", "secret.*", "*.secret",
	}

	for _, pattern := range secretPatterns {
		matched, _ := filepath.Match(pattern, filename)
		if matched {
			return true
		}
	}

	return false
}

// IsHistoryFile checks if filename indicates a history file
func IsHistoryFile(filename string) bool {
	historyPatterns := []string{
		"*_history",
		".*_history",
		".bash_history",
		".zsh_history",
		".sh_history",
		".lesshst",
		".mysql_history",
		".psql_history",
		".node_repl_history",
		".python_history",
	}

	for _, pattern := range historyPatterns {
		matched, _ := filepath.Match(pattern, filename)
		if matched {
			return true
		}
	}

	return false
}

// IsTemporaryFile checks if filename indicates a temporary file
func IsTemporaryFile(filename string) bool {
	tempPatterns := []string{
		"*.swp", "*.swo", ".*.swp",
		"*~",
		"*.tmp", "*.temp",
		"*.bak", "*.backup",
		"*.orig",
		"#*#", // Emacs auto-save
	}

	for _, pattern := range tempPatterns {
		matched, _ := filepath.Match(pattern, filename)
		if matched {
			return true
		}
	}

	return false
}

// IsSystemFile checks if filename indicates a system file
func IsSystemFile(filename string) bool {
	systemFiles := map[string]bool{
		".DS_Store":       true,
		"Thumbs.db":       true,
		"desktop.ini":     true,
		".Spotlight-V100": true,
		".Trashes":        true,
		"ehthumbs.db":     true,
	}

	return systemFiles[filename]
}

// GetFileCategory returns the category of a file based on its name
// Returns one of: "secret", "history", "temporary", "system", "normal"
func GetFileCategory(filename string) string {
	if IsSecretFile(filename) {
		return "secret"
	}
	if IsHistoryFile(filename) {
		return "history"
	}
	if IsTemporaryFile(filename) {
		return "temporary"
	}
	if IsSystemFile(filename) {
		return "system"
	}
	return "normal"
}

// FilterByPatterns filters a list of paths by ignore patterns
// Returns paths that do NOT match any pattern
func FilterByPatterns(paths []string, patterns []string) []string {
	var result []string

	for _, path := range paths {
		ignored, _ := ShouldIgnore(path, patterns)
		if !ignored {
			result = append(result, path)
		}
	}

	return result
}

// MergePatterns merges multiple pattern lists, removing duplicates
func MergePatterns(patternLists ...[]string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, patterns := range patternLists {
		for _, pattern := range patterns {
			if !seen[pattern] {
				seen[pattern] = true
				result = append(result, pattern)
			}
		}
	}

	return result
}
