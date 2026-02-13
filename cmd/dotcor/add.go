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
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [file]...",
	Short: "Add dotfiles to DotCor management",
	Long: `Add one or more dotfiles or directories to DotCor management.

Files are moved to the repository and replaced with symlinks.
Supports glob patterns for batch operations.

Examples:
  dotcor add ~/.zshrc                    # Add single file
  dotcor add ~/.zshrc ~/.bashrc          # Add multiple files
  dotcor add ~/.config/nvim/*            # Add with glob pattern
  dotcor add ~/.config/nvim --recursive  # Add directory recursively
  dotcor add ~/.zshrc --template         # Add as template file
  dotcor add ~/.zshrc --category shell   # Add with custom category
  dotcor add ~/.zshrc --force            # Skip validation warnings`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringP("category", "c", "", "Override automatic category detection")
	addCmd.Flags().BoolP("force", "f", false, "Force add, ignoring warnings (not errors)")
	addCmd.Flags().BoolP("recursive", "r", false, "Recursively add all files in directory")
	addCmd.Flags().Bool("template", false, "Treat file as template (adds .template extension)")
	addCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	category, _ := cmd.Flags().GetString("category")
	force, _ := cmd.Flags().GetBool("force")
	recursive, _ := cmd.Flags().GetBool("recursive")
	isTemplate, _ := cmd.Flags().GetBool("template")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

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

	// Handle recursive directories
	if recursive {
		var expandedFiles []string
		totalFiles := 0

		for _, file := range files {
			expandedPath, err := config.ExpandPath(file, cfg)
			if err != nil {
				return fmt.Errorf("expanding %s: %w", file, err)
			}

			isDir, err := fs.IsDirectory(expandedPath, cfg)
			if err == nil && isDir {
				// Get all files in directory recursively
				dirFiles, err := fs.GetFilesRecursive(expandedPath)
				if err != nil {
					return fmt.Errorf("recursively reading directory %s: %w", file, err)
				}

				// Normalize paths and add to list
				for _, dirFile := range dirFiles {
					normalized, _ := config.NormalizePath(dirFile)
					if normalized != "" {
						expandedFiles = append(expandedFiles, normalized)
					} else {
						expandedFiles = append(expandedFiles, dirFile)
					}
				}
				totalFiles += len(dirFiles)
			} else {
				// Not a directory, keep as is
				expandedFiles = append(expandedFiles, file)
				totalFiles++
			}
		}

		if totalFiles > 10 {
			fmt.Printf("Adding %d files from directories...\n", totalFiles)
		}

		files = expandedFiles
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found matching the provided patterns")
	}

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	// Show summary and ask for confirmation
	if !force && !dryRun {
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

		fmt.Println("Summary:")
		fmt.Printf("  Files to add: %d\n", len(files))
		fmt.Printf("  Total size: %s\n", formatSize(totalSize))
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

	for _, file := range files {
		result, _, err := processAddFile(cfg, file, category, force, isTemplate, dryRun)
		switch result {
		case addResultSuccess:
			added++
		case addResultSkipped:
			skipped++
		case addResultError:
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s[X]%s %s: %v\n", colorRed, colorReset, file, err)
			}
			skipped++
		}
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
func processAddFile(cfg *config.Config, sourcePath string, category string, force bool, isTemplate bool, dryRun bool) (addResult, string, error) {
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
		fmt.Printf("  - %s (already managed)\n", normalized)
		return addResultSkipped, "", nil
	}

	// Check ignore patterns
	shouldIgnore, pattern := core.ShouldIgnore(expanded, cfg.IgnorePatterns)
	if shouldIgnore {
		fmt.Printf("  - %s (ignored - matches %s)\n", normalized, pattern)
		return addResultSkipped, "", nil
	}

	// Run validation
	if err := core.ValidateSourceFile(expanded, cfg); err != nil {
		// Check if it's a warning vs error
		if isWarning(err) && force {
			fmt.Printf("  %s[!]%s %s: %v (forced)\n", colorYellow, colorReset, normalized, err)
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
		fmt.Printf("  %s[!]%s %s: potential secrets detected (forced)\n", colorYellow, colorReset, normalized)
	}

	// Generate repo path
	customRepoPath := ""
	if category != "" {
		// Category should be combined with the filename, not replace the entire path
		// e.g., --category shell for ~/.zshrc should produce "shell/zshrc"
		filename := filepath.Base(expanded)
		// Strip leading dot from filename for repo path
		repoFilename := strings.TrimPrefix(filename, ".")
		customRepoPath = filepath.Join(category, repoFilename)
	}
	repoPath, err := config.GenerateRepoPath(sourcePath, customRepoPath, cfg)

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
		fmt.Printf("  %s[!]%s Pre-add hook warning: %v\n", colorYellow, colorReset, err)
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
				fmt.Fprintf(os.Stderr, "  %s[!]%s Failed to restore backup: %v\n", colorYellow, colorReset, restoreErr)
			}
		}
		return addResultError, "", err
	}

	tx.Commit()
	fmt.Printf("  %s[OK]%s %s\n", colorGreen, colorReset, normalized)

	if err := core.RunHook(core.HookContext{HookType: "post-add", FilePath: sourcePath, RepoPath: repoPath}, cfg); err != nil {
		fmt.Printf("  %s[!]%s Post-add hook warning: %v\n", colorYellow, colorReset, err)
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
