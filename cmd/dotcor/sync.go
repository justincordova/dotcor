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
	Short: "Commit and optionally push to remote",
	Long: `Sync dotfiles by committing changes and pushing to remote (if configured).

This command:
1. Checks for uncommitted changes
2. Creates a timestamped commit
3. Auto-pushes to remote if configured
4. Commits only if no remote is configured

Examples:
  dotcor sync                 # Commit and auto-push if remote exists
  dotcor sync ~/.zshrc        # Sync specific file(s)
  dotcor sync --no-push       # Commit only (skip push)
  dotcor sync --preview       # Show what would be synced
  dotcor sync -m "message"    # Custom commit message`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().Bool("no-push", false, "Commit only, skip auto-push")
	syncCmd.Flags().Bool("preview", false, "Show what would be synced without making changes")
	syncCmd.Flags().Bool("dry-run", false, "Alias for --preview")
	syncCmd.Flags().BoolP("force", "f", false, "Sync without confirmation")
	syncCmd.Flags().StringP("message", "m", "", "Custom commit message")
}

func runSync(cmd *cobra.Command, args []string) error {
	noPush, _ := cmd.Flags().GetBool("no-push")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	message, _ := cmd.Flags().GetString("message")

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
	cfg.Logger.Info("sync flag check", "no_push", noPush)

	// Filter to specific files if provided
	filesToSync := []string{}
	filesToSyncRepoPaths := []string{}
	if len(args) > 0 {
		for _, arg := range args {
			mf, err := cfg.GetManagedFile(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s[!]%s %s is not managed\n", colorYellow, colorReset, arg)
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
		return showSyncPreview(repoPath, hasChanges, gitStatus, !noPush && gitStatus.RemoteExists)
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

	// Show push status if ahead and remote exists
	if gitStatus.AheadBy > 0 && gitStatus.RemoteExists {
		fmt.Printf("%d commit(s) to push to remote.\n", gitStatus.AheadBy)
		fmt.Println("")
	}

	// Confirm unless --force
	willPush := gitStatus.RemoteExists && !noPush
	cfg.Logger.Debug("sync decisions", "remote_exists", gitStatus.RemoteExists, "no_push_flag", noPush, "will_push", willPush)
	if !force {
		if !confirmSync(hasChanges, willPush && gitStatus.AheadBy > 0) {
			fmt.Println("Sync cancelled.")
			return nil
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-sync", FilePath: ""}, cfg); err != nil {
		fmt.Printf("%s[!]%s Pre-sync hook warning: %v\n", colorYellow, colorReset, err)
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
				commitMsg = fmt.Sprintf("Update %d file(s) - %s", len(filesToSync), time.Now().Format("2006-01-02 15:04"))
			} else {
				commitMsg = fmt.Sprintf("Update dotfiles - %s", time.Now().Format("2006-01-02 15:04"))
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
		fmt.Printf("%s[OK]%s Changes committed\n", colorGreen, colorReset)

		// Clear uncommitted flags for all files
		uncommittedFiles := cfg.GetUncommittedFiles()
		for _, mf := range uncommittedFiles {
			if err := cfg.ClearUncommitted(mf.SourcePath); err != nil {
				fmt.Printf("%s[!]%s Failed to clear uncommitted flag for %s: %v\n", colorYellow, colorReset, mf.SourcePath, err)
			}
		}
	}

	// Push to remote (auto-detect if remote exists and not --no-push)
	cfg.Logger.Debug("push decision", "will_push", willPush, "remote_exists", gitStatus.RemoteExists)
	if willPush {
		// Auto-push to remote
		cfg.Logger.Debug("attempting push to remote", "path", repoPath)
		if err := pushToRemote(repoPath); err != nil {
			return fmt.Errorf("pushing to remote: %w", err)
		}
		fmt.Printf("%s[OK]%s Pushed to remote\n", colorGreen, colorReset)
	} else if !gitStatus.RemoteExists {
		// No remote configured - show tip
		fmt.Println("")
		fmt.Printf("%s[!]%s Tip: You haven't added a remote repository yet.\n", colorYellow, colorReset)
		fmt.Printf("  Use 'git remote add origin <url>' to set up remote.\n")
		fmt.Printf("  Then changes will be auto-pushed on sync.\n")
	}

	if err := core.RunHook(core.HookContext{HookType: "post-sync", FilePath: ""}, cfg); err != nil {
		fmt.Printf("%s[!]%s Post-sync hook warning: %v\n", colorYellow, colorReset, err)
	}

	fmt.Println("")
	fmt.Println("Sync complete!")
	return nil
}

// showSyncPreview shows what would be synced
func showSyncPreview(repoPath string, hasChanges bool, gitStatus git.StatusInfo, willPush bool) error {
	fmt.Printf("\n  %sSync Preview%s\n", colorLightPink, colorReset)
	fmt.Println("")

	if hasChanges {
		fmt.Printf("  %sUncommitted changes:%s\n", colorLightPink, colorReset)
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
	if willPush && gitStatus.RemoteExists {
		if gitStatus.AheadBy > 0 {
			fmt.Printf("Would push %d commit(s) to remote.\n", gitStatus.AheadBy)
		} else if gitStatus.BehindBy > 0 {
			fmt.Printf("%s[!]%s Remote is %d commit(s) ahead. Consider 'git pull' first.\n", colorYellow, colorReset, gitStatus.BehindBy)
		} else {
			fmt.Println("Already in sync with remote.")
		}
	} else if !gitStatus.RemoteExists {
		fmt.Println("No remote configured.")
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
