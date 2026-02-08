package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTemplateContext(t *testing.T) {
	// Act
	ctx, err := GetTemplateContext()

	// Assert
	require.NoError(t, err, "GetTemplateContext() should not return an error")
	assert.NotEmpty(t, ctx.Hostname, "GetTemplateContext() Hostname should not be empty")
	assert.NotEmpty(t, ctx.OS, "GetTemplateContext() OS should not be empty")
	assert.NotEmpty(t, ctx.User, "GetTemplateContext() User should not be empty")
	assert.NotEmpty(t, ctx.Home, "GetTemplateContext() Home should not be empty")
}

func TestSubstituteTemplate(t *testing.T) {
	// Arrange
	ctx := &TemplateContext{
		Hostname: "testhost",
		OS:       "linux",
		User:     "testuser",
		Home:     "/home/testuser",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "substitute hostname",
			input:    "hostname={{ .Hostname }}",
			expected: "hostname=testhost",
		},
		{
			name:     "substitute os",
			input:    "os={{ .OS }}",
			expected: "os=linux",
		},
		{
			name:     "substitute user",
			input:    "user={{ .User }}",
			expected: "user=testuser",
		},
		{
			name:     "substitute home",
			input:    "home={{ .Home }}",
			expected: "home=/home/testuser",
		},
		{
			name:     "substitute multiple variables",
			input:    "{{ .User }}@{{ .Hostname }}",
			expected: "testuser@testhost",
		},
		{
			name:     "no variables to substitute",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "partial match only",
			input:    "{{ .Hostname }} extra text",
			expected: "testhost extra text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			result := SubstituteTemplate(tt.input, ctx)

			// Assert
			assert.Equal(t, tt.expected, result, "SubstituteTemplate() should return expected result")
		})
	}
}

func TestIsTemplateFile(t *testing.T) {
	// Arrange
	tests := []struct {
		filename string
		want     bool
	}{
		{"file.template", true},
		{"file.txt.template", true},
		{"file.txt", false},
		{"file", false},
		{".template", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// Act
			got := IsTemplateFile(tt.filename)

			// Assert
			assert.Equal(t, tt.want, got, "IsTemplateFile(%s) should return expected result", tt.filename)
		})
	}
}

func TestStripTemplateExtension(t *testing.T) {
	// Arrange
	tests := []struct {
		input    string
		expected string
	}{
		{"file.template", "file"},
		{"file.txt.template", "file.txt"},
		{"path/to/file.template", "path/to/file"},
		{"file.txt", "file.txt"},
		{"file", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Act
			result := StripTemplateExtension(tt.input)

			// Assert
			assert.Equal(t, tt.expected, result, "StripTemplateExtension(%s) should return expected result", tt.input)
		})
	}
}

func TestTemplateIntegration(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-template-test-*")
	require.NoError(t, err, "should create temp dir")
	defer os.RemoveAll(tempDir)

	templatePath := filepath.Join(tempDir, "config.template")
	templateContent := `# Config file for {{ .Hostname }}
User: {{ .User }}
Home: {{ .Home }}
OS: {{ .OS }}
`
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err, "should create template file")

	ctx := &TemplateContext{
		Hostname: "test-machine",
		OS:       "darwin",
		User:     "alice",
		Home:     "/Users/alice",
	}

	// Act
	content, err := os.ReadFile(templatePath)
	require.NoError(t, err, "should read template file")

	result := SubstituteTemplate(string(content), ctx)

	// Assert
	assert.Contains(t, result, "test-machine", "SubstituteTemplate() should substitute Hostname")
	assert.Contains(t, result, "alice", "SubstituteTemplate() should substitute User")
	assert.Contains(t, result, "/Users/alice", "SubstituteTemplate() should substitute Home")
	assert.Contains(t, result, "darwin", "SubstituteTemplate() should substitute OS")

	expectedLines := []string{
		"# Config file for test-machine",
		"User: alice",
		"Home: /Users/alice",
		"OS: darwin",
	}

	for _, line := range expectedLines {
		assert.Contains(t, result, line, "Template result should contain expected line: %s", line)
	}
}
