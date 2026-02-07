package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetTemplateContext(t *testing.T) {
	ctx, err := GetTemplateContext()
	if err != nil {
		t.Fatalf("GetTemplateContext() error = %v", err)
	}

	if ctx.Hostname == "" {
		t.Error("GetTemplateContext() Hostname is empty")
	}

	if ctx.OS == "" {
		t.Error("GetTemplateContext() OS is empty")
	}

	if ctx.User == "" {
		t.Error("GetTemplateContext() User is empty")
	}

	if ctx.Home == "" {
		t.Error("GetTemplateContext() Home is empty")
	}
}

func TestSubstituteTemplate(t *testing.T) {
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
			result := SubstituteTemplate(tt.input, ctx)
			if result != tt.expected {
				t.Errorf("SubstituteTemplate() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsTemplateFile(t *testing.T) {
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
			got := IsTemplateFile(tt.filename)
			if got != tt.want {
				t.Errorf("IsTemplateFile(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestStripTemplateExtension(t *testing.T) {
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
			result := StripTemplateExtension(tt.input)
			if result != tt.expected {
				t.Errorf("StripTemplateExtension(%s) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTemplateIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-template-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create template file
	templatePath := filepath.Join(tempDir, "config.template")
	templateContent := `# Config file for {{ .Hostname }}
User: {{ .User }}
Home: {{ .Home }}
OS: {{ .OS }}
`
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to create template file: %v", err)
	}

	// Get context
	ctx := &TemplateContext{
		Hostname: "test-machine",
		OS:       "darwin",
		User:     "alice",
		Home:     "/Users/alice",
	}

	// Read and substitute
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	result := SubstituteTemplate(string(content), ctx)

	// Verify substitutions
	if !strings.Contains(result, "test-machine") {
		t.Error("SubstituteTemplate() did not substitute Hostname")
	}
	if !strings.Contains(result, "alice") {
		t.Error("SubstituteTemplate() did not substitute User")
	}
	if !strings.Contains(result, "/Users/alice") {
		t.Error("SubstituteTemplate() did not substitute Home")
	}
	if !strings.Contains(result, "darwin") {
		t.Error("SubstituteTemplate() did not substitute OS")
	}

	// Verify template syntax is preserved but replaced
	expectedLines := []string{
		"# Config file for test-machine",
		"User: alice",
		"Home: /Users/alice",
		"OS: darwin",
	}

	for _, line := range expectedLines {
		if !strings.Contains(result, line) {
			t.Errorf("Template result missing expected line: %s\nGot:\n%s", line, result)
		}
	}
}
