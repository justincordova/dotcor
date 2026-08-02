package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldIgnore(t *testing.T) {
	// Arrange
	patterns := []string{
		"*.key",
		".env",
		".env.*",
		"id_rsa",
		"*.swp",
		".DS_Store",
	}

	tests := []struct {
		name        string
		path        string
		wantIgnored bool
	}{
		{
			name:        "matches key pattern",
			path:        "/home/user/secret.key",
			wantIgnored: true,
		},
		{
			name:        "matches exact env",
			path:        "/home/user/.env",
			wantIgnored: true,
		},
		{
			name:        "matches env.local",
			path:        "/home/user/.env.local",
			wantIgnored: true,
		},
		{
			name:        "matches id_rsa",
			path:        "/home/user/.ssh/id_rsa",
			wantIgnored: true,
		},
		{
			name:        "matches swp",
			path:        "/home/user/.zshrc.swp",
			wantIgnored: true,
		},
		{
			name:        "matches DS_Store",
			path:        "/home/user/.DS_Store",
			wantIgnored: true,
		},
		{
			name:        "normal file not ignored",
			path:        "/home/user/.zshrc",
			wantIgnored: false,
		},
		{
			name:        "gitconfig not ignored",
			path:        "/home/user/.gitconfig",
			wantIgnored: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, _ := ShouldIgnore(tt.path, patterns)

			// Assert
			assert.Equal(t, tt.wantIgnored, got)
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		{
			name:    "glob asterisk match",
			path:    "/home/user/secret.key",
			pattern: "*.key",
			want:    true,
		},
		{
			name:    "exact match",
			path:    "/home/user/.env",
			pattern: ".env",
			want:    true,
		},
		{
			name:    "no match",
			path:    "/home/user/.zshrc",
			pattern: "*.env",
			want:    false,
		},
		{
			name:    "question mark match",
			path:    "/home/user/file1.txt",
			pattern: "file?.txt",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := MatchesPattern(tt.path, tt.pattern)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSecretFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"id_rsa", true},
		{"id_rsa.pub", true},
		{"id_ed25519", true},
		{".env", true},
		{".env.local", true},
		{"secret.key", true},
		{"credentials.pem", true},
		{".zshrc", false},
		{".gitconfig", false},
		{"config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := IsSecretFile(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsHistoryFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{".bash_history", true},
		{".zsh_history", true},
		{".mysql_history", true},
		{".node_repl_history", true},
		{".lesshst", true},
		{".zshrc", false},
		{".bashrc", false},
		{"history.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := IsHistoryFile(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsTemporaryFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{".zshrc.swp", true},
		{"file.swo", true},
		{"backup~", true},
		{"file.tmp", true},
		{"file.bak", true},
		{"file.orig", true},
		{".zshrc", false},
		{"config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := IsTemporaryFile(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSystemFile(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"desktop.ini", true},
		{".zshrc", false},
		{"config.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := IsSystemFile(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetFileCategory(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"id_rsa", "secret"},
		{".env", "secret"},
		{".bash_history", "history"},
		{".zshrc.swp", "temporary"},
		{".DS_Store", "system"},
		{".zshrc", "normal"},
		{".gitconfig", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Arrange
			// (test data defined above)

			// Act
			got := GetFileCategory(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterByPatterns(t *testing.T) {
	// Arrange
	paths := []string{
		"/home/user/.zshrc",
		"/home/user/.env",
		"/home/user/.gitconfig",
		"/home/user/secret.key",
		"/home/user/.DS_Store",
	}

	patterns := []string{".env", "*.key", ".DS_Store"}
	expected := []string{
		"/home/user/.zshrc",
		"/home/user/.gitconfig",
	}

	// Act
	got := FilterByPatterns(paths, patterns)

	// Assert
	assert.Equal(t, len(expected), len(got))
	for i, path := range expected {
		assert.Equal(t, path, got[i])
	}
}

func TestMergePatterns(t *testing.T) {
	// Arrange
	list1 := []string{"*.key", ".env", "*.swp"}
	list2 := []string{".env", ".DS_Store", "*.key"}

	// Act
	got := MergePatterns(list1, list2)

	// Assert
	assert.Len(t, got, 4)

	seen := make(map[string]bool)
	for _, p := range got {
		assert.False(t, seen[p], "MergePatterns() has duplicate: %s", p)
		seen[p] = true
	}
}

func TestLoadGitignorePatterns(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	gitignoreContent := `# Comment line
*.swp
.env

# Another comment
*.key
id_rsa
`
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	err = os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	expected := []string{"*.swp", ".env", "*.key", "id_rsa"}

	// Act
	patterns, err := LoadGitignorePatterns(gitignorePath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, len(expected), len(patterns))
	for i, pattern := range expected {
		assert.Equal(t, pattern, patterns[i])
	}
}

// TestShouldIgnore_DirectoryScopedPatterns pins the fix for a silent failure
// of the secret-exclusion mechanism. filepath.Match's `*` never crosses a
// separator, and callers pass absolute paths, so every directory-scoped
// pattern a user could write matched nothing — while still appearing in the
// settings list and saving to .dotcorrc.
func TestShouldIgnore_DirectoryScopedPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{"single-star under a directory", ".ssh/*", "/home/u/.ssh/id_rsa", true},
		{"nested directory prefix", ".config/nvim/*", "/home/u/.config/nvim/init.lua", true},
		{"globstar prefix", "**/*.log", "/home/u/a/b/x.log", true},
		{"globstar suffix", "secrets/**", "/home/u/secrets/a/b.txt", true},
		{"bare directory name excludes subtree", "node_modules", "/home/u/p/node_modules/x.js", true},
		{"bare filename still matches", ".env", "/home/u/.env", true},
		{"bare filename at depth", ".env", "/home/u/app/.env", true},
		{"glob on basename", "*.key", "/home/u/.ssh/server.key", true},

		{"different directory must not match", ".ssh/*", "/home/u/.aws/credentials", false},
		{"single-star does not cross separator", ".ssh/*", "/home/u/.ssh/sub/id_rsa", false},
		{"unrelated path", "secrets/**", "/home/u/public/a.txt", false},
		{"prefix is not a match", "node_modules", "/home/u/node_modules_old/x.js", false},
		{"extension mismatch", "**/*.log", "/home/u/a/b/x.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, pattern := ShouldIgnore(tt.path, []string{tt.pattern})
			assert.Equal(t, tt.want, got)
			if tt.want {
				assert.Equal(t, tt.pattern, pattern, "the matching pattern must be reported")
			}
			assert.Equal(t, tt.want, MatchesPattern(tt.path, tt.pattern), "MatchesPattern must agree with ShouldIgnore")
		})
	}
}

// TestShouldIgnore_EmptyAndDegenerate guards the edges.
func TestShouldIgnore_EmptyAndDegenerate(t *testing.T) {
	got, _ := ShouldIgnore("/home/u/.zshrc", nil)
	assert.False(t, got, "no patterns means nothing is ignored")

	got, _ = ShouldIgnore("/home/u/.zshrc", []string{""})
	assert.False(t, got, "an empty pattern must not match everything")

	got, _ = ShouldIgnore("/home/u/.zshrc", []string{"/"})
	assert.False(t, got, "a bare separator must not match everything")
}

// TestShouldIgnore_DefaultPatternsStillApply is the regression guard for the
// shipped defaults, which are all basename-shaped.
func TestShouldIgnore_DefaultPatternsStillApply(t *testing.T) {
	patterns := []string{"*.key", "*.pem", ".env", ".env.*", "id_rsa", "id_rsa.*", "*_history", ".DS_Store"}

	for _, path := range []string{
		"/home/u/.ssh/id_rsa",
		"/home/u/.ssh/id_rsa.pub",
		"/home/u/.env",
		"/home/u/.env.local",
		"/home/u/certs/server.pem",
		"/home/u/.bash_history",
		"/home/u/.config/.DS_Store",
	} {
		matched, _ := ShouldIgnore(path, patterns)
		assert.True(t, matched, "%s should be ignored by the default patterns", path)
	}

	matched, _ := ShouldIgnore("/home/u/.zshrc", patterns)
	assert.False(t, matched, "an ordinary dotfile must not be ignored")
}

// TestShouldIgnore_GlobstarsDoNotBlowUp pins the fix for a pattern that could
// hang the TUI.
//
// The recursive matcher branched at every "**" with no memoisation, so cost
// grew as segments^globstars: on a 30-segment path, six globstars took 410ms
// and eight took 9 seconds. Patterns come from .dotcorrc and the settings
// view and are evaluated per file during a $HOME walk, so a pasted or
// malformed pattern froze classification with no error and no cancellation.
func TestShouldIgnore_GlobstarsDoNotBlowUp(t *testing.T) {
	path := "/home/u/" + strings.Repeat("a/", 28) + "x.txt"
	pattern := strings.Repeat("**/", 12) + "*.log"

	done := make(chan bool, 1)
	go func() {
		matched, _ := ShouldIgnore(path, []string{pattern})
		done <- matched
	}()

	select {
	case matched := <-done:
		assert.False(t, matched, "*.log must not match x.txt")
	case <-time.After(5 * time.Second):
		t.Fatal("pattern matching did not terminate promptly — the matcher is superlinear in globstar count")
	}
}

// TestMatchSegments_GlobstarSemantics pins the behaviour of the rewritten
// matcher directly.
func TestMatchSegments_GlobstarSemantics(t *testing.T) {
	tests := []struct {
		name     string
		pattern  []string
		segments []string
		want     bool
	}{
		{"globstar matches nothing", []string{"a", "**"}, []string{"a"}, true},
		{"globstar matches one", []string{"a", "**"}, []string{"a", "b"}, true},
		{"globstar matches many", []string{"a", "**"}, []string{"a", "b", "c", "d"}, true},
		{"globstar in the middle", []string{"a", "**", "d"}, []string{"a", "b", "c", "d"}, true},
		{"globstar in the middle, no tail match", []string{"a", "**", "z"}, []string{"a", "b", "c", "d"}, false},
		{"leading globstar", []string{"**", "d"}, []string{"a", "b", "c", "d"}, true},
		{"consecutive globstars", []string{"**", "**", "d"}, []string{"a", "b", "d"}, true},
		{"only globstars", []string{"**", "**"}, []string{"a", "b"}, true},
		{"star does not cross a segment", []string{"a", "*"}, []string{"a", "b", "c"}, false},
		{"exact match", []string{"a", "b"}, []string{"a", "b"}, true},
		{"pattern longer than path", []string{"a", "b", "c"}, []string{"a", "b"}, false},
		{"empty pattern, empty path", nil, nil, true},
		{"empty pattern, non-empty path", nil, []string{"a"}, false},
		{"globstar against empty path", []string{"**"}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchSegments(tt.pattern, tt.segments))
		})
	}
}
