package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Commit all changes and push to remote",
	Long: `Sync dotfiles by committing changes and optionally pushing to remote.

This command:
1. Checks for uncommitted changes
2. Creates a timestamped commit
3. Pushes to remote (if configured and not --no-push)

Examples:
  dotcor sync                 # Commit and push
  dotcor sync --no-push       # Commit only
  dotcor sync --preview       # Show what would be synced
  dotcor sync -m "message"    # Custom commit message`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().Bool("no-push", false, "Commit but don't push to remote")
	syncCmd.Flags().Bool("preview", false, "Show what would be synced without making changes")
	syncCmd.Flags().BoolP("force", "f", false, "Sync without confirmation")
	syncCmd.Flags().StringP("message", "m", "", "Custom commit message")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	noPush, _ := cmd.Flags().GetBool("no-push")
	preview, _ := cmd.Flags().GetBool("preview")
	force, _ := cmd.Flags().GetBool("force")
	message, _ := cmd.Flags().GetString("message")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}

	// Check if git is available
	if !git.IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Get repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("expanding repo path: %w", err)
	}

	// Check if it's a git repo
	if !git.IsRepo(repoPath) {
		return fmt.Errorf("dotcor repository is not a git repository")
	}

	// Check for changes
	hasChanges, err := git.HasChanges(repoPath)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}

	// Get git status
	gitStatus, err := git.GetStatus(repoPath)
	if err != nil {
		return fmt.Errorf("getting git status: %w", err)
	}

	// Preview mode
	if preview {
		return showSyncPreview(repoPath, hasChanges, gitStatus, noPush)
	}

	// Nothing to sync
	if !hasChanges && gitStatus.AheadBy == 0 {
		fmt.Println("Nothing to sync. Working tree is clean and up to date.")
		return nil
	}

	// Show what will be synced
	if hasChanges {
		fmt.Println("Changes to be committed:")
		changedFiles, _ := git.GetChangedFiles(repoPath)
		for _, f := range changedFiles {
			fmt.Printf("  %s\n", f)
		}
		fmt.Println("")
	}

	if gitStatus.AheadBy > 0 && !noPush {
		fmt.Printf("%d commit(s) to push to remote.\n", gitStatus.AheadBy)
		fmt.Println("")
	}

	// Confirm unless --force
	if !force {
		if !confirmSync(hasChanges, gitStatus.AheadBy > 0 && !noPush) {
			fmt.Println("Sync cancelled.")
			return nil
		}
	}

	// Acquire lock
	if err := core.AcquireLock(); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer core.ReleaseLock()

	// Commit changes
	if hasChanges {
		commitMsg := message
		if commitMsg == "" {
			commitMsg = fmt.Sprintf("Sync dotfiles - %s", time.Now().Format("2006-01-02 15:04"))
		}

		if err := git.AutoCommit(repoPath, commitMsg); err != nil {
			return fmt.Errorf("committing changes: %w", err)
		}
		fmt.Println("✓ Changes committed")

		// Clear uncommitted flags for all files
		uncommittedFiles := cfg.GetUncommittedFiles()
		for _, mf := range uncommittedFiles {
			if err := cfg.ClearUncommitted(mf.SourcePath); err != nil {
				fmt.Printf("⚠ Failed to clear uncommitted flag for %s: %v\n", mf.SourcePath, err)
			}
		}
	}

	// Push to remote
	if !noPush {
		// Check if remote exists
		remoteURL, _ := git.GetRemoteURL(repoPath)
		if remoteURL != "" {
			if err := pushToRemote(repoPath); err != nil {
				return fmt.Errorf("pushing to remote: %w", err)
			}
			fmt.Println("✓ Pushed to remote")
		} else {
			fmt.Println("⚠ No remote configured. Use 'git remote add origin <url>' to set up.")
		}
	}

	fmt.Println("")
	fmt.Println("Sync complete!")
	return nil
}

// showSyncPreview shows what would be synced
func showSyncPreview(repoPath string, hasChanges bool, gitStatus git.StatusInfo, noPush bool) error {
	fmt.Println("Sync Preview")
	fmt.Println("============")
	fmt.Println("")

	if hasChanges {
		fmt.Println("Uncommitted changes:")
		changedFiles, _ := git.GetChangedFiles(repoPath)
		for _, f := range changedFiles {
			fmt.Printf("  M %s\n", f)
		}
		fmt.Println("")

		// Show diff stat
		diffStat, _ := git.GetDiffStat(repoPath)
		if diffStat != "" {
			fmt.Println("Summary:")
			fmt.Print(diffStat)
			fmt.Println("")
		}
	} else {
		fmt.Println("No uncommitted changes.")
		fmt.Println("")
	}

	// Show push status
	if !noPush {
		if gitStatus.RemoteExists {
			if gitStatus.AheadBy > 0 {
				fmt.Printf("Would push %d commit(s) to remote.\n", gitStatus.AheadBy)
			} else if gitStatus.BehindBy > 0 {
				fmt.Printf("⚠ Remote is %d commit(s) ahead. Consider 'git pull' first.\n", gitStatus.BehindBy)
			} else {
				fmt.Println("Already in sync with remote.")
			}
		} else {
			fmt.Println("No remote configured.")
		}
	}

	return nil
}

// confirmSync prompts for confirmation
func confirmSync(hasChanges bool, willPush bool) bool {
	var action string
	if hasChanges && willPush {
		action = "commit and push"
	} else if hasChanges {
		action = "commit"
	} else if willPush {
		action = "push"
	} else {
		return true
	}

	fmt.Printf("Proceed to %s? [Y/n]: ", action)

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "" || input == "y" || input == "yes"
}

// pushToRemote pushes changes to remote
func pushToRemote(repoPath string) error {
	// Use git push
	return git.Sync(repoPath)
}
