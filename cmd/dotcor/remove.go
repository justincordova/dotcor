package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove [file]...",
	Short:   "Stop managing dotfiles",
	Aliases: []string{"rm"},
	Long: `Remove dotfiles from DotCor management.

By default, the file is removed from management but kept in the repository.
The symlink at the original location is deleted. Use --delete-repo to also
remove the file from the repository (copies it back to source location).

Examples:
  dotcor remove ~/.zshrc              # Remove from management, keep in repo (default)
  dotcor remove ~/.zshrc --delete-repo  # Remove from management and delete from repo
  dotcor remove --all                 # Remove all files from management`,
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().Bool("delete-repo", false, "Delete file from repository (copies back to source location)")
	removeCmd.Flags().Bool("all", false, "Remove all files from management")
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	removeCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	removeCmd.Flags().Bool("batch", false, "Batch mode: confirm once for all files, show progress")
}

func runRemove(cmd *cobra.Command, args []string) error {
	deleteRepo, _ := cmd.Flags().GetBool("delete-repo")
	removeAll, _ := cmd.Flags().GetBool("all")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	batch, _ := cmd.Flags().GetBool("batch")

	if !removeAll && len(args) == 0 {
		return fmt.Errorf("specify files to remove or use --all")
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Add pre-flight validation
	result := core.RunPreflightValidation(cfg, "remove", []string{})
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

	// Determine which files to remove
	var filesToRemove []config.ManagedFile

	if removeAll {
		filesToRemove = cfg.ManagedFiles
		if len(filesToRemove) == 0 {
			fmt.Println("No files to remove.")
			return nil
		}
	} else {
		for _, arg := range args {
			mf, err := cfg.GetManagedFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s[X]%s %s: not managed\n", colorRed, colorReset, arg)
				if !strings.HasPrefix(arg, "~") && !strings.HasPrefix(arg, "/") && !strings.HasPrefix(arg, ".") {
					fmt.Fprintf(os.Stderr, "      %s[!]%s Tip: Use ~ for home directory (e.g., ~/.zshrc)\n", colorYellow, colorReset)
				}
				continue
			}
			filesToRemove = append(filesToRemove, *mf)
		}
	}

	if len(filesToRemove) == 0 {
		return fmt.Errorf("no valid files to remove")
	}

	// Confirmation
	if batch && !force && !dryRun {
		if err := confirmBatchOperation(len(filesToRemove), "remove", force); err != nil {
			return err
		}
		fmt.Println("")
	}

	if !batch && !force && !dryRun {
		fmt.Printf("%sSummary:%s\n", colorLightPink, colorReset)
		fmt.Printf("  Files to remove: %d\n", len(filesToRemove))
		fmt.Println("  Backups will be preserved")
		fmt.Println("")
		for _, f := range filesToRemove {
			fmt.Printf("  - %s\n", f.SourcePath)
		}
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

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	useProgress := shouldUseProgress(len(filesToRemove), batch)

	// Process each file
	removed := 0

	var progress *Progress
	if useProgress {
		progress = NewProgress(len(filesToRemove), 20)
	}

	for _, mf := range filesToRemove {
		if useProgress && progress != nil {
			progress.Update()
		}
		err := processRemoveFile(cfg, mf, !deleteRepo, dryRun, useProgress)
		if err != nil {
			if !useProgress {
				fmt.Fprintf(os.Stderr, "  %s[X]%s %s: %v\n", colorRed, colorReset, mf.SourcePath, err)
			}
			continue
		}
		removed++
	}

	if useProgress && progress != nil {
		progress.Done()
		fmt.Println("")
	}

	// Summary
	fmt.Println("")
	if dryRun {
		fmt.Printf("Would remove %d file(s) from management\n", removed)
		return nil
	}

	fmt.Printf("Removed %d file(s) from management\n", removed)

	// Git commit
	if git.IsGitInstalled() && removed > 0 && deleteRepo {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err != nil {
			fmt.Printf("%s[!]%s Git commit skipped: invalid repo path: %v\n", colorYellow, colorReset, err)
		} else {
			message := fmt.Sprintf("Remove %d file(s) from management", removed)
			if err := git.AutoCommit(repoPath, message); err != nil {
				fmt.Printf("%s[!]%s Git commit failed: %v\n", colorYellow, colorReset, err)
			} else {
				fmt.Printf("%s[OK]%s Committed to Git\n", colorGreen, colorReset)
			}
		}
	}

	return nil
}

// processRemoveFile handles removing a single file
func processRemoveFile(cfg *config.Config, mf config.ManagedFile, keepRepo bool, dryRun bool, quiet bool) error {
	sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
	if err != nil {
		return fmt.Errorf("invalid repo path: %w", err)
	}

	if dryRun {
		fmt.Printf("  - %s\n", mf.SourcePath)
		if !keepRepo {
			fmt.Printf("    → Remove symlink from %s\n", sourcePath)
			fmt.Printf("    → Keep in repo: %s\n", mf.RepoPath)
		} else {
			fmt.Printf("    → Copy to %s\n", sourcePath)
			fmt.Printf("    → Remove from repo: %s\n", mf.RepoPath)
		}
		return nil
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-remove", FilePath: mf.SourcePath}, cfg); err != nil {
		if !quiet {
			fmt.Printf("  %s[!]%s Pre-remove hook warning: %v\n", colorYellow, colorReset, err)
		}
	}

	// Check if source is a symlink
	isLink, err := fs.IsSymlink(sourcePath)
	if err != nil {
		return fmt.Errorf("checking symlink status: %w", err)
	}

	// If keeping repo, just remove symlink and update config
	if keepRepo {
		if isLink {
			if err := os.Remove(sourcePath); err != nil {
				return fmt.Errorf("removing symlink: %w", err)
			}
		}

		// Remove from config
		if err := cfg.RemoveManagedFile(mf.SourcePath); err != nil {
			return fmt.Errorf("updating config: %w", err)
		}

		if err := core.RunHook(core.HookContext{HookType: "post-remove", FilePath: mf.SourcePath}, cfg); err != nil {
			if !quiet {
				fmt.Printf("  %s[!]%s Post-remove hook warning: %v\n", colorYellow, colorReset, err)
			}
		}

		if !quiet {
			fmt.Printf("  %s[OK]%s %s (removed from management, kept in repo)\n", colorGreen, colorReset, mf.SourcePath)
		}
		return nil
	}

	// Full removal: copy back and delete from repo

	// First, create backup of repo file
	if fs.PathExists(repoPath) {
		backupPath, err := core.CreateBackup(repoPath, cfg)
		if err != nil {
			return fmt.Errorf("backup creation failed for %s: %w", mf.RepoPath, err)
		}
		if backupPath == "" {
			return fmt.Errorf("backup creation failed - no backup path returned for %s", mf.SourcePath)
		}
	}

	// Ensure parent directory exists
	if err := fs.EnsureDir(filepath.Dir(sourcePath), cfg); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}

	// If source is a symlink, remove it first
	if isLink {
		if err := os.Remove(sourcePath); err != nil {
			return fmt.Errorf("removing symlink: %w", err)
		}
	}

	// Copy file from repo to source location
	if fs.PathExists(repoPath) {
		if err := fs.CopyWithPermissions(repoPath, sourcePath, cfg); err != nil {
			return fmt.Errorf("copying file back: %w", err)
		}

		// Delete from repo
		if err := os.Remove(repoPath); err != nil {
			return fmt.Errorf("removing from repo: %w", err)
		}

		// Clean up empty parent directories in repo
		cleanEmptyDirs(filepath.Dir(repoPath))
	}

	// Remove from config
	if err := cfg.RemoveManagedFile(mf.SourcePath); err != nil {
		return fmt.Errorf("updating config: %w", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-remove", FilePath: mf.SourcePath}, cfg); err != nil {
		if !quiet {
			fmt.Printf("  %s[!]%s Post-remove hook warning: %v\n", colorYellow, colorReset, err)
		}
	}

	if !quiet {
		if keepRepo {
			fmt.Printf("  %s[OK]%s %s (removed from management, kept in repo)\n", colorGreen, colorReset, mf.SourcePath)
		} else {
			fmt.Printf("  %s[OK]%s %s (removed from management and repo)\n", colorGreen, colorReset, mf.SourcePath)
		}
	}
	return nil
}

// cleanEmptyDirs removes empty parent directories up to the repo root
func cleanEmptyDirs(dir string) {
	for {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}

		// Directory is empty, remove it
		if err := os.Remove(dir); err != nil {
			break
		}

		// Move up to parent
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
