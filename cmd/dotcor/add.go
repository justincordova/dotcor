package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/utils"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [file]...",
	Short: "Add dotfiles to DotCor management",
	Long: `Add one or more dotfiles or directories to DotCor management.

Files are moved to the repository and replaced with symlinks.
Supports glob patterns for batch operations.

Templates:
  The --template flag allows you to create template files that get
  rendered with machine-specific variables when symlinks are created.
  This is useful for dotfiles that need different values on different machines.

  Available template variables:
    {{ .Hostname }}  - Machine hostname
    {{ .User }}      - Current username
    {{ .Home }}      - User's home directory

  Template files are stored with a .template extension in the repository.
  When you run 'dotcor rebuild-links', templates are rendered and symlinks
  are created with the substituted values.

  Use templates when:
    - Hostnames differ between machines (e.g., work laptop vs personal desktop)
    - Usernames vary across systems
    - Paths need to be dynamic (though ~ expansion is usually preferred)

  Use regular files when:
    - Configuration is the same across all machines
    - Values don't depend on the specific system
    - You want the same file everywhere

  Examples:
  dotcor add ~/.zshrc                    # Add single file
  dotcor add ~/.zshrc ~/.bashrc          # Add multiple files
  dotcor add ~/.config/nvim/*            # Add with glob pattern
  dotcor add ~/.zshrc --template         # Add as template file
  dotcor add ~/.zshrc --force            # Skip validation warnings

  Template example (SSH config with hostname):
    Host {{ .Hostname }}
      HostName {{ .Hostname }}.local
      User {{ .User }}

  After adding templates, run 'dotcor rebuild-links' to render them:
    dotcor add ~/.ssh/config.template --template
    dotcor rebuild-links`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().BoolP("force", "f", false, "Force add, ignoring warnings (not errors)")
	addCmd.Flags().Bool("template", false, "Treat file as template (adds .template extension)")
	addCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	addCmd.Flags().Bool("batch", false, "Batch mode: confirm once for all files, show progress")
}

func runAdd(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	isTemplate, _ := cmd.Flags().GetBool("template")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	batch, _ := cmd.Flags().GetBool("batch")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Add pre-flight validation
	result := core.RunPreflightValidation(cfg, "add", []string{})
	if err := core.DisplayValidationResults(result); err != nil {
		return err
	}

	// Acquire lock (skip for dry-run)
	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer func() {
			if err := core.ReleaseLock(cfg); err != nil {
				cfg.Logger.Error("failed to release lock", "error", err)
			}
		}()
	}

	// Expand glob patterns in args
	var files []string
	for _, arg := range args {
		expanded, err := expandGlobArg(arg)
		if err != nil {
			return fmt.Errorf("expanding %s: %w", arg, err)
		}
		files = append(files, expanded...)
	}

	if len(files) == 0 {
		var patternList strings.Builder
		for i, arg := range args {
			if i > 0 {
				patternList.WriteString(", ")
			}
			patternList.WriteString(arg)
		}
		return fmt.Errorf("no files found matching patterns: %s\n\nCommon issues:\n  - Check that file paths are correct\n  - Use ~ for home directory (e.g., ~/.zshrc)\n  - Verify files exist before adding\n  - Use --dry-run to preview what would be added", patternList.String())
	}

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	useProgress := shouldUseProgress(len(files), batch)

	// Show summary and ask for confirmation
	if batch && !force && !dryRun {
		if err := confirmBatchOperation(len(files), "add", force); err != nil {
			return err
		}
		fmt.Println("")
	}

	if !batch && !force && !dryRun {
		// Calculate total size
		var totalSize int64
		for _, file := range files {
			expandedPath, err := config.ExpandPath(file, cfg)
			if err == nil {
				if info, err := os.Stat(expandedPath); err == nil {
					totalSize += info.Size()
				}
			}
		}

		fmt.Printf("  %sSummary:%s\n", colorLightPink, colorReset)
		fmt.Printf("  Files to add: %d\n", len(files))
		fmt.Printf("  Total size: %s\n", utils.FormatSize(totalSize))
		fmt.Println("")
		fmt.Print("Proceed? [Y/n]: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "n" || input == "no" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Process each file
	added := 0
	skipped := 0

	var progress *Progress
	if useProgress {
		progress = NewProgress(len(files), 20)
	}

	for _, file := range files {
		if useProgress && progress != nil {
			progress.Update()
		}

		result, _, err := processAddFile(cfg, file, force, isTemplate, dryRun, useProgress)
		switch result {
		case addResultSuccess:
			added++
		case addResultSkipped:
			skipped++
		case addResultError:
			if err != nil && !useProgress {
				fmt.Fprintf(os.Stderr, "  %s[X]%s %s: %v\n", colorRed, colorReset, file, err)
			}
			skipped++
		}
	}

	if useProgress && progress != nil {
		progress.Done()
		fmt.Println("")
	}

	// Summary
	fmt.Println("")
	if dryRun {
		fmt.Printf("Would add %d file(s)\n", added)
		return nil
	}

	fmt.Printf("Added %d file(s)", added)
	if skipped > 0 {
		fmt.Printf(", skipped %d", skipped)
	}
	fmt.Println("")

	// Git commit
	if git.IsGitInstalled() && added > 0 {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err != nil {
			return fmt.Errorf("expanding repo path: %w", err)
		}
		if err := git.AutoCommit(repoPath, "Add dotfiles"); err != nil {
			fmt.Printf("%s[!]%s Git commit failed: %v (files marked as uncommitted)\n", colorYellow, colorReset, err)
			fmt.Println("Run 'dotcor sync' to commit these changes.")
		} else {
			fmt.Printf("%s[OK]%s Committed to Git\n", colorGreen, colorReset)
		}
	}

	return nil
}

type addResult int

const (
	addResultSuccess addResult = iota
	addResultSkipped
	addResultError
)

// processAddFile handles adding a single file
func processAddFile(cfg *config.Config, sourcePath string, force bool, isTemplate bool, dryRun bool, quiet bool) (addResult, string, error) {
	// Expand source path
	expanded, err := config.ExpandPath(sourcePath, cfg)
	if err != nil {
		return addResultError, "", fmt.Errorf("invalid path: %w", err)
	}

	// Normalize for display and storage
	normalized, err := config.NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	// Check if file exists
	if !fs.PathExists(expanded) {
		return addResultError, "", fmt.Errorf("file does not exist")
	}

	// Check if already managed
	if cfg.IsManaged(sourcePath) {
		if !quiet {
			fmt.Printf("  - %s (already managed)\n", normalized)
		}
		return addResultSkipped, "", nil
	}

	// Check ignore patterns
	shouldIgnore, pattern := core.ShouldIgnore(expanded, cfg.IgnorePatterns)
	if shouldIgnore {
		if !quiet {
			fmt.Printf("  - %s (ignored - matches %s)\n", normalized, pattern)
		}
		return addResultSkipped, "", nil
	}

	// Run validation
	if err := core.ValidateSourceFile(expanded, cfg); err != nil {
		// Check if it's a warning vs error
		if isWarning(err) && force {
			if !quiet {
				fmt.Printf("  %s[!]%s %s: %v (forced)\n", colorYellow, colorReset, normalized, err)
			}
		} else {
			return addResultError, "", err
		}
	}

	// Check for potential secrets
	secrets, _ := core.DetectSecrets(expanded)
	if len(secrets) > 0 {
		if !force {
			return addResultError, "", fmt.Errorf("potential secrets detected: %v\nUse --force to add anyway", secrets)
		}
		if !quiet {
			fmt.Printf("  %s[!]%s %s: potential secrets detected (forced)\n", colorYellow, colorReset, normalized)
		}
	}

	// Generate repo path
	repoPath, err := config.GenerateRepoPath(sourcePath, cfg)

	// Add .template extension if template flag is set
	if isTemplate {
		repoPath += ".template"
	}
	if err != nil {
		return addResultError, "", fmt.Errorf("generating repo path: %w", err)
	}

	// Validate repo file path can be constructed
	if _, err := config.GetRepoFilePath(cfg, repoPath); err != nil {
		return addResultError, "", err
	}

	if dryRun {
		fmt.Printf("  + %s → %s\n", normalized, repoPath)
		return addResultSuccess, repoPath, nil
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-add", FilePath: sourcePath}, cfg); err != nil {
		if !quiet {
			fmt.Printf("  %s[!]%s Pre-add hook warning: %v\n", colorYellow, colorReset, err)
		}
	}

	// Create backup
	backupPath, err := core.CreateBackup(expanded, cfg)
	if err != nil {
		// Backup creation failed, abort operation
		return addResultError, "", fmt.Errorf("backup creation failed for %s: %w", normalized, err)
	}

	// Verify backup was created successfully
	if backupPath == "" {
		return addResultError, "", fmt.Errorf("backup creation failed, no backup path returned for %s", normalized)
	}

	// Create managed file entry
	mf := config.ManagedFile{
		SourcePath: normalized,
		RepoPath:   repoPath,
		AddedAt:    time.Now(),
	}

	// Use transaction for atomic operation
	tx, err := core.AddFileTransaction(cfg, sourcePath, repoPath, mf)
	if err != nil {
		return addResultError, "", fmt.Errorf("creating transaction: %w", err)
	}

	// Execute transaction
	if err := tx.ExecuteAll(); err != nil {
		// Rollback already happened in ExecuteAll
		// Try to restore from backup if we have one
		if backupPath != "" {
			if restoreErr := core.RestoreBackup(backupPath, expanded, cfg); restoreErr != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "  %s[!]%s Failed to restore backup: %v\n", colorYellow, colorReset, restoreErr)
				}
			}
		}
		return addResultError, "", err
	}

	tx.Commit()
	if !quiet {
		fmt.Printf("  %s[OK]%s %s\n", colorGreen, colorReset, normalized)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-add", FilePath: sourcePath, RepoPath: repoPath}, cfg); err != nil {
		if !quiet {
			fmt.Printf("  %s[!]%s Post-add hook warning: %v\n", colorYellow, colorReset, err)
		}
	}

	// Return relative repoPath (consistent with dry-run return)
	return addResultSuccess, repoPath, nil
}

// expandGlobArg expands a single argument that may contain glob patterns
func expandGlobArg(arg string) ([]string, error) {
	// First expand ~ if present
	expanded, err := config.ExpandPath(arg, nil)
	if err != nil {
		return nil, err
	}

	// Check if it contains glob characters
	if !containsGlob(expanded) {
		return []string{arg}, nil
	}

	// Expand glob
	matches, err := filepath.Glob(expanded)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files match pattern: %s", arg)
	}

	// Filter out directories (only add files)
	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			// Convert back to normalized path with ~
			normalized, _ := config.NormalizePath(match)
			if normalized != "" {
				files = append(files, normalized)
			} else {
				files = append(files, match)
			}
		}
	}

	return files, nil
}

// containsGlob checks if a string contains glob metacharacters
func containsGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// isWarning checks if an error is a warning vs a hard error
func isWarning(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "warning") ||
		strings.Contains(msg, "large file") ||
		strings.Contains(msg, "unusual permissions")
}
