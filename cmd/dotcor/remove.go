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

By default, the file is copied back to its original location and removed
from the repository. Use --keep-repo to leave the file in the repository.

Examples:
  dotcor remove ~/.zshrc              # Remove file, copy back to original location
  dotcor remove ~/.zshrc --keep-repo  # Remove from management but keep in repo
  dotcor remove --all                 # Remove all files from management`,
	RunE: runRemove,
}

func init() {
	removeCmd.Flags().Bool("keep-repo", false, "Keep file in repository after removing")
	removeCmd.Flags().Bool("all", false, "Remove all files from management")
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	removeCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	keepRepo, _ := cmd.Flags().GetBool("keep-repo")
	removeAll, _ := cmd.Flags().GetBool("all")
	force, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if !removeAll && len(args) == 0 {
		return fmt.Errorf("specify files to remove or use --all")
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Acquire lock (skip for dry-run)
	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer core.ReleaseLock(cfg)
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
				fmt.Fprintf(os.Stderr, "  [X] %s: not managed\n", arg)
				continue
			}
			filesToRemove = append(filesToRemove, *mf)
		}
	}

	if len(filesToRemove) == 0 {
		return fmt.Errorf("no valid files to remove")
	}

	// Confirmation
	if !force && !dryRun {
		fmt.Printf("Remove %d file(s) from management?\n", len(filesToRemove))
		for _, f := range filesToRemove {
			fmt.Printf("  - %s\n", f.SourcePath)
		}
		fmt.Println("")

		if !confirmRemove() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	// Process each file
	removed := 0

	for _, mf := range filesToRemove {
		err := processRemoveFile(cfg, mf, keepRepo, dryRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [X] %s: %v\n", mf.SourcePath, err)
			continue
		}
		removed++
	}

	// Summary
	fmt.Println("")
	if dryRun {
		fmt.Printf("Would remove %d file(s) from management\n", removed)
		return nil
	}

	fmt.Printf("Removed %d file(s) from management\n", removed)

	// Git commit
	if git.IsGitInstalled() && removed > 0 && !keepRepo {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err != nil {
			fmt.Printf("[!] Git commit skipped: invalid repo path: %v\n", err)
		} else {
			message := fmt.Sprintf("Remove %d file(s) from management", removed)
			if err := git.AutoCommit(repoPath, message); err != nil {
				fmt.Printf("[!] Git commit failed: %v\n", err)
			} else {
				fmt.Println("[OK] Committed to Git")
			}
		}
	}

	return nil
}

// processRemoveFile handles removing a single file
func processRemoveFile(cfg *config.Config, mf config.ManagedFile, keepRepo bool, dryRun bool) error {
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
			fmt.Printf("    → Copy to %s\n", sourcePath)
			fmt.Printf("    → Remove from repo: %s\n", mf.RepoPath)
		}
		return nil
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-remove", FilePath: mf.SourcePath}, cfg); err != nil {
		fmt.Printf("  [!] Pre-remove hook warning: %v\n", err)
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
			fmt.Printf("  [!] Post-remove hook warning: %v\n", err)
		}

		fmt.Printf("  [OK] %s (removed from management, kept in repo)\n", mf.SourcePath)
		return nil
	}

	// Full removal: copy back and delete from repo

	// First, create backup of repo file
	if fs.PathExists(repoPath) {
		if _, err := core.CreateBackup(repoPath, cfg); err != nil {
			fmt.Printf("  [!] Backup failed for %s: %v\n", mf.RepoPath, err)
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
		fmt.Printf("  [!] Post-remove hook warning: %v\n", err)
	}

	fmt.Printf("  [OK] %s\n", mf.SourcePath)
	return nil
}

// confirmRemove prompts for confirmation
func confirmRemove() bool {
	fmt.Print("Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
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
