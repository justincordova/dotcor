package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ShouldIgnore reports whether path matches any ignore pattern, and which.
//
// Matching follows .gitignore semantics rather than raw filepath.Match,
// because filepath.Match's `*` never crosses a separator. Callers pass an
// absolute path, so every directory-scoped pattern a user could write —
// `.ssh/*`, `secrets/**`, `**/*.log`, `node_modules` — matched nothing at all.
// That is a silent failure of a secret-exclusion mechanism: the pattern shows
// up in the settings list, saves to .dotcorrc, and filters none of the files
// the user believed they had excluded.
//
// The rules implemented here:
//
//   - A pattern with no separator matches any path SEGMENT, so `node_modules`
//     excludes everything beneath it and `.env` excludes the file itself.
//   - A pattern with separators matches any trailing run of segments, so
//     `.ssh/*` matches /home/u/.ssh/id_rsa without needing to be anchored.
//   - `**` matches zero or more whole segments.
func ShouldIgnore(path string, patterns []string) (bool, string) {
	segments := pathSegments(path)

	for _, pattern := range patterns {
		if matchesIgnorePattern(pattern, segments) {
			return true, pattern
		}
	}

	return false, ""
}

// MatchesPattern reports whether path matches a single ignore pattern, using
// the same semantics as ShouldIgnore.
func MatchesPattern(path, pattern string) bool {
	return matchesIgnorePattern(pattern, pathSegments(path))
}

// pathSegments splits a path into its non-empty components, normalising
// Windows separators so patterns can always be written with "/".
func pathSegments(path string) []string {
	clean := strings.Trim(filepath.ToSlash(filepath.Clean(path)), "/")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}

func matchesIgnorePattern(pattern string, segments []string) bool {
	patternSegments := pathSegments(pattern)
	if len(patternSegments) == 0 {
		return false
	}

	// A bare name (no separator, no globstar) matches at any depth — both the
	// file itself and any ancestor directory, so ignoring `node_modules`
	// excludes its whole subtree.
	if len(patternSegments) == 1 && patternSegments[0] != "**" {
		for _, segment := range segments {
			if matched, err := filepath.Match(patternSegments[0], segment); err == nil && matched {
				return true
			}
		}
		return false
	}

	// Otherwise try to match the pattern against every trailing run of
	// segments, so patterns need not be anchored at the filesystem root.
	for i := 0; i <= len(segments); i++ {
		if matchSegments(patternSegments, segments[i:]) {
			return true
		}
	}
	return false
}

// matchSegments matches pattern segments against path segments, where "**"
// consumes zero or more whole segments and every other segment is matched
// with filepath.Match.
func matchSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}

	if pattern[0] == "**" {
		for i := 0; i <= len(segments); i++ {
			if matchSegments(pattern[1:], segments[i:]) {
				return true
			}
		}
		return false
	}

	if len(segments) == 0 {
		return false
	}

	matched, err := filepath.Match(pattern[0], segments[0])
	if err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], segments[1:])
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
