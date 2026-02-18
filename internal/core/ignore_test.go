package core

import (
	"os"
	"path/filepath"
	"testing"

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
