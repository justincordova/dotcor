package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadme_ProjectStructure_Accuracy(t *testing.T) {
	// Read README.md to verify project structure matches actual codebase
	readmeContent, err := os.ReadFile("../../README.md")
	require.NoError(t, err)

	lines := string(readmeContent)

	// Check for non-existent files mentioned that should be removed
	nonExistentFiles := []string{
		"internal/core/linker.go",
		"internal/core/validator.go",
	}

	for _, file := range nonExistentFiles {
		assert.NotContains(t, lines, file, "README should not reference non-existent file: "+file)
	}

	// Check that the project structure section contains expected commands
	// The structure section should have tree-like format with commands
	expectedInStructure := []string{
		"cmd/dotcor/", // Should have cmd directory
		"main.go",     // Should have main.go
		"internal/",   // Should have internal directory
	}

	for _, item := range expectedInStructure {
		assert.Contains(t, lines, item, "README should contain: "+item)
	}
}

func TestReadme_ProjectStructure_ProperFormatting(t *testing.T) {
	readmeContent, err := os.ReadFile("../../README.md")
	require.NoError(t, err)

	lines := strings.Split(string(readmeContent), "\n")

	// Find the project structure section
	inStructureSection := false
	structureLines := []string{}

	for _, line := range lines {
		if strings.Contains(line, "### Project Structure") {
			inStructureSection = true
			continue
		}
		if inStructureSection {
			// Stop at the next section
			if strings.HasPrefix(line, "### ") && !strings.Contains(line, "Project Structure") {
				break
			}
			structureLines = append(structureLines, line)
		}
	}

	// Verify structure section exists
	assert.Greater(t, len(structureLines), 10, "Project Structure section should exist in README")

	// Verify structure section has tree-like format with backticks
	structureText := strings.Join(structureLines, "\n")
	assert.Contains(t, structureText, "```", "Project Structure should be in code block")

	// Verify all 15 commands are listed in the structure
	expectedCommands := []string{
		"main.go",
		"init.go",
		"add.go",
		"remove.go",
		"list.go",
		"status.go",
		"sync.go",
		"restore.go",
		"history.go",
		"diff.go",
		"adopt.go",
		"doctor.go",
		"rebuild.go",
		"rebuild-links.go",
		"clone.go",
		"cleanup.go",
	}

	for _, cmd := range expectedCommands {
		assert.Contains(t, structureText, cmd, "Project Structure should include: "+cmd)
	}

	// Verify internal packages
	expectedPackages := []string{
		"internal/",
		"config/",
		"core/",
		"fs/",
		"git/",
		"logger/",
	}

	for _, pkg := range expectedPackages {
		assert.Contains(t, structureText, pkg, "Project Structure should include: "+pkg)
	}
}
