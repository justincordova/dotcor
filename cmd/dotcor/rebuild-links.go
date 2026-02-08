package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/spf13/cobra"
)

// containsTemplateVariables checks if file content has any template variables
func containsTemplateVariables(content string) bool {
	variables := []string{
		"{{ .Hostname }}",
		"{{ .OS }}",
		"{{ .User }}",
		"{{ .Home }}",
	}
	for _, v := range variables {
		if strings.Contains(content, v) {
			return true
		}
	}
	return false
}

var rebuildLinksCmd = &cobra.Command{
	Use:   "rebuild-links",
	Short: "Rebuild symlinks from template files",
	Long: `Rebuild symlinks by rendering template files with current context.

This command:
1. Finds all .template files in the repository
2. Renders them with current context (hostname, OS, user, home)
3. Creates or updates symlinks to the rendered files

Examples:
  dotcor rebuild-links                    # Rebuild all template links
  dotcor rebuild-links --dry-run         # Show what would be done`,
	RunE: runRebuildLinks,
}

func init() {
	rebuildLinksCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	rootCmd.AddCommand(rebuildLinksCmd)
}

func runRebuildLinks(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}

	// Get template context
	ctx, err := core.GetTemplateContext()
	if err != nil {
		return fmt.Errorf("getting template context: %w", err)
	}

	// Find all managed files
	managedFiles := cfg.GetManagedFilesForPlatform()
	if len(managedFiles) == 0 {
		fmt.Println("No managed files found.")
		return nil
	}

	// Acquire lock (skip for dry-run)
	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer core.ReleaseLock(cfg)
	}

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	// Process each managed file
	rebuilt := 0
	skipped := 0

	for _, mf := range managedFiles {
		// Check if repo file is a template
		if !core.IsTemplateFile(mf.RepoPath) {
			skipped++
			continue
		}

		// Get paths
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s (invalid source path)\n", mf.SourcePath)
			continue
		}

		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s (invalid repo path)\n", mf.SourcePath)
			continue
		}

		// Check if template file exists
		if !fs.PathExists(repoPath) {
			fmt.Printf("  - %s (template not found in repo)\n", mf.SourcePath)
			continue
		}

		// Read template content
		templateContent, err := os.ReadFile(repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s (failed to read template: %v)\n", mf.SourcePath, err)
			continue
		}

		// Check if file actually contains template variables
		if !containsTemplateVariables(string(templateContent)) {
			fmt.Printf("  [!] Skipping %s: no template variables found\n", mf.SourcePath)
			skipped++
			continue
		}

		// Render template
		renderedContent := core.SubstituteTemplate(string(templateContent), ctx)

		// Strip .template extension for output path
		baseRepoPath := strings.TrimSuffix(repoPath, ".template")

		if dryRun {
			fmt.Printf("  + %s → %s\n", mf.SourcePath, core.StripTemplateExtension(mf.RepoPath))
			continue
		}

		// Get original file permissions
		originalMode := os.FileMode(0644)
		if info, err := os.Stat(baseRepoPath); err == nil {
			originalMode = info.Mode() & 0777
		}

		// Write rendered content with preserved permissions
		if err := os.WriteFile(baseRepoPath, []byte(renderedContent), originalMode); err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s (failed to write: %v)\n", mf.SourcePath, err)
			continue
		}

		// Update or create symlink
		// First, remove existing file/symlink if it exists
		if fs.PathExists(sourcePath) {
			os.Remove(sourcePath)
		}

		// Create symlink to rendered file
		if err := fs.CreateSymlink(baseRepoPath, sourcePath, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s (failed to create symlink: %v)\n", mf.SourcePath, err)
			continue
		}

		fmt.Printf("  [OK] %s\n", mf.SourcePath)
		rebuilt++
	}

	// Summary
	fmt.Println("")
	if dryRun {
		fmt.Printf("Would rebuild %d file(s)\n", rebuilt)
		return nil
	}

	fmt.Printf("Rebuilt %d file(s)", rebuilt)
	if skipped > 0 {
		fmt.Printf(", skipped %d", skipped)
	}
	fmt.Println("")

	return nil
}
