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
	Use:   "sync [files...]",
	Short: "Commit changes to repository",
	Long: `Sync dotfiles by committing changes and optionally pushing to remote.

This command:
1. Checks for uncommitted changes
2. Creates a timestamped commit
3. Pushes to remote (only with --push flag)
4. Shows warning if no remote is configured

Examples:
  dotcor sync                 # Commit all files
  dotcor sync ~/.zshrc        # Commit specific file(s)
  dotcor sync --push          # Commit and push to remote
  dotcor sync --no-push       # Commit only (same as default)
  dotcor sync --preview       # Show what would be synced
  dotcor sync -m "message"    # Custom commit message`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().Bool("push", false, "Commit and push to remote")
	syncCmd.Flags().Bool("no-push", false, "Commit but don't push to remote (default behavior)")
	syncCmd.Flags().Bool("preview", false, "Show what would be synced without making changes")
	syncCmd.Flags().Bool("dry-run", false, "Alias for --preview")
	syncCmd.Flags().BoolP("force", "f", false, "Sync without confirmation")
	syncCmd.Flags().StringP("message", "m", "", "Custom commit message")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	pushFlag, _ := cmd.Flags().GetBool("push")
	noPush, _ := cmd.Flags().GetBool("no-push")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	message, _ := cmd.Flags().GetString("message")

	// Treat --no-push as default (no push)
	if noPush {
		pushFlag = false
	}

	// Treat --dry-run as --preview
	if dryRun {
		preview = true
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Filter to specific files if provided
	filesToSync := []string{}
	filesToSyncRepoPaths := []string{}
	if len(args) > 0 {
		for _, arg := range args {
			mf, err := cfg.GetManagedFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[!] %s is not managed\n", arg)
			} else {
				filesToSync = append(filesToSync, arg)
				filesToSyncRepoPaths = append(filesToSyncRepoPaths, mf.RepoPath)
			}
		}
		if len(filesToSync) == 0 && len(args) > 0 {
			return fmt.Errorf("no valid files specified")
		}
		fmt.Printf("Syncing %d file(s)...\n", len(filesToSync))
	}

	// Check if git is available
	if !git.IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Get repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
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
		return showSyncPreview(repoPath, hasChanges, gitStatus, pushFlag)
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

	// Show push status if --push is used
	if gitStatus.AheadBy > 0 && pushFlag {
		fmt.Printf("%d commit(s) to push to remote.\n", gitStatus.AheadBy)
		fmt.Println("")
	}

	// Confirm unless --force
	if !force {
		if !confirmSync(hasChanges, pushFlag && gitStatus.AheadBy > 0) {
			fmt.Println("Sync cancelled.")
			return nil
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-sync", FilePath: ""}, cfg); err != nil {
		fmt.Printf("[!] Pre-sync hook warning: %v\n", err)
	}

	// Acquire lock
	if err := core.AcquireLock(cfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		if err := core.ReleaseLock(cfg); err != nil {
			cfg.Logger.Error("failed to release lock", "error", err)
		}
	}()

	// Commit changes
	if hasChanges {
		commitMsg := message
		if commitMsg == "" {
			if len(filesToSync) > 0 {
				commitMsg = fmt.Sprintf("Sync %d file(s) - %s", len(filesToSync), time.Now().Format("2006-01-02 15:04"))
			} else {
				commitMsg = fmt.Sprintf("Sync dotfiles - %s", time.Now().Format("2006-01-02 15:04"))
			}
		}

		// Use selective commit if files specified
		if len(filesToSyncRepoPaths) > 0 {
			if err := git.AutoCommitFiles(repoPath, filesToSyncRepoPaths, commitMsg); err != nil {
				return fmt.Errorf("committing changes: %w", err)
			}
		} else {
			if err := git.AutoCommit(repoPath, commitMsg); err != nil {
				return fmt.Errorf("committing changes: %w", err)
			}
		}
		fmt.Println("[OK] Changes committed")

		// Clear uncommitted flags for all files
		uncommittedFiles := cfg.GetUncommittedFiles()
		for _, mf := range uncommittedFiles {
			if err := cfg.ClearUncommitted(mf.SourcePath); err != nil {
				fmt.Printf("[!] Failed to clear uncommitted flag for %s: %v\n", mf.SourcePath, err)
			}
		}
	}

	// Push to remote (only if --push is used)
	if pushFlag {
		// Check if remote exists
		remoteURL, _ := git.GetRemoteURL(repoPath)
		if remoteURL != "" {
			if err := pushToRemote(repoPath); err != nil {
				return fmt.Errorf("pushing to remote: %w", err)
			}
			fmt.Println("[OK] Pushed to remote")
		} else {
			fmt.Println("[!] No remote configured. Use 'git remote add origin <url>' to set up.")
		}
	} else {
		// Show warning if no remote is configured
		remoteURL, _ := git.GetRemoteURL(repoPath)
		if remoteURL == "" {
			fmt.Println("")
			fmt.Println("[!] Tip: You haven't added a remote repository yet.")
			fmt.Println("  Use 'git remote add origin <url>' to set up remote.")
			fmt.Println("  Then run 'dotcor sync --push' to push your changes.")
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "post-sync", FilePath: ""}, cfg); err != nil {
		fmt.Printf("[!] Post-sync hook warning: %v\n", err)
	}

	fmt.Println("")
	fmt.Println("Sync complete!")
	return nil
}

// showSyncPreview shows what would be synced
func showSyncPreview(repoPath string, hasChanges bool, gitStatus git.StatusInfo, pushFlag bool) error {
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
	if pushFlag {
		if gitStatus.RemoteExists {
			if gitStatus.AheadBy > 0 {
				fmt.Printf("Would push %d commit(s) to remote.\n", gitStatus.AheadBy)
			} else if gitStatus.BehindBy > 0 {
				fmt.Printf("[!] Remote is %d commit(s) ahead. Consider 'git pull' first.\n", gitStatus.BehindBy)
			} else {
				fmt.Println("Already in sync with remote.")
			}
		} else {
			fmt.Println("No remote configured.")
		}
	} else {
		if gitStatus.RemoteExists {
			fmt.Println("Use --push flag to push to remote.")
		} else {
			fmt.Println("No remote configured.")
			fmt.Println("Use 'git remote add origin <url>' to set up.")
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
	// Push with progress
	return git.PushWithProgress(repoPath)
}
