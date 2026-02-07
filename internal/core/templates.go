package core

import (
	"os"
	"os/user"
	"runtime"
	"strings"
)

// TemplateContext holds variables for template substitution
type TemplateContext struct {
	Hostname string
	OS       string
	User     string
	Home     string
}

// GetTemplateContext returns the current template context
func GetTemplateContext() (*TemplateContext, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	currentUser, err := user.Current()
	if err != nil {
		currentUser = &user.User{HomeDir: "~"}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	return &TemplateContext{
		Hostname: hostname,
		OS:       runtime.GOOS,
		User:     currentUser.Username,
		Home:     home,
	}, nil
}

// SubstituteTemplates performs simple {{ variable }} substitution
// Supports: {{ .Hostname }}, {{ .OS }}, {{ .User }}, {{ .Home }}
func SubstituteTemplate(content string, ctx *TemplateContext) string {
	result := content
	result = strings.ReplaceAll(result, "{{ .Hostname }}", ctx.Hostname)
	result = strings.ReplaceAll(result, "{{ .OS }}", ctx.OS)
	result = strings.ReplaceAll(result, "{{ .User }}", ctx.User)
	result = strings.ReplaceAll(result, "{{ .Home }}", ctx.Home)
	return result
}

// IsTemplateFile checks if a file is a template (has .template extension)
func IsTemplateFile(filename string) bool {
	return strings.HasSuffix(filename, ".template")
}

// StripTemplateExtension removes .template extension from filename
func StripTemplateExtension(filename string) string {
	if IsTemplateFile(filename) {
		return strings.TrimSuffix(filename, ".template")
	}
	return filename
}
