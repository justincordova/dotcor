package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [file...]",
	Short: "Restore dotfiles from Git history or backup",
	Long: `Restore dotfiles from Git history or from backups.

By default, restores from the most recent Git commit. Use --to to specify
a different commit or reference. Use --from-backup to restore from a backup.

Examples:
  dotcor restore ~/.zshrc                # Restore single file from latest commit
  dotcor restore ~/.zshrc ~/.bashrc      # Restore multiple files
  dotcor restore ~/.zshrc --to HEAD~1    # Restore from previous commit
  dotcor restore ~/.zshrc --to abc123    # Restore from specific commit
  dotcor restore ~/.zshrc --from-backup  # Restore from backup
  dotcor restore --list-backups          # List available backups`,
	RunE: runRestore,
}

func init() {
	restoreCmd.Flags().String("to", "HEAD", "Git reference to restore from (e.g., HEAD~1, abc123)")
	restoreCmd.Flags().Bool("from-backup", false, "Restore from backup instead of Git history")
	restoreCmd.Flags().Bool("list-backups", false, "List available backups")
	restoreCmd.Flags().Bool("preview", false, "Show what would be restored without making changes")
	restoreCmd.Flags().Bool("dry-run", false, "Alias for --preview")
	restoreCmd.Flags().Bool("diff", false, "Show diff between current and target version")
	restoreCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
}

func runRestore(cmd *cobra.Command, args []string) error {
	toRef, _ := cmd.Flags().GetString("to")
	fromBackup, _ := cmd.Flags().GetBool("from-backup")
	listBackups, _ := cmd.Flags().GetBool("list-backups")
	preview, _ := cmd.Flags().GetBool("preview")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	showDiff, _ := cmd.Flags().GetBool("diff")
	force, _ := cmd.Flags().GetBool("force")

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

	// Add pre-flight validation
	result := core.RunPreflightValidation(cfg, "restore", []string{})
	if err := core.DisplayValidationResults(result); err != nil {
		return err
	}

	// Handle --list-backups
	if listBackups {
		return listAllBackups(cmd)
	}

	// Require file argument
	if len(args) == 0 {
		return fmt.Errorf("specify files to restore")
	}

	// Restore each file
	successCount := 0
	for _, sourcePath := range args {
		err := restoreSingleFile(sourcePath, toRef, fromBackup, preview, showDiff, force, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s[!]%s %s: %v\n", colorYellow, colorReset, sourcePath, err)
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("no files restored successfully")
	}

	return nil
}

// restoreSingleFile restores a single file
func restoreSingleFile(sourcePath, toRef string, fromBackup, preview, showDiff, force bool, cfg *config.Config) error {
	// Get managed file info
	mf, err := cfg.GetManagedFile(sourcePath)
	if err != nil {
		return fmt.Errorf("file not managed: %s", sourcePath)
	}

	// Get repo path
	repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
	if err != nil {
		return fmt.Errorf("getting repo path: %w", err)
	}

	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	// Handle backup restore
	if fromBackup {
		return restoreFromBackup(mf.SourcePath, repoPath, preview, showDiff, force, cfg)
	}

	// Git restore
	return restoreFromGit(configDir, mf.RepoPath, repoPath, toRef, preview, showDiff, force, cfg)
}

// restoreFromGit restores a file from Git history
func restoreFromGit(configDir, repoPath, fullRepoPath, ref string, preview, showDiff, force bool, cfg *config.Config) error {
	// Get git path (with files/ prefix)
	gitPath := config.GetGitFilePath(repoPath)

	// Check if it's a git repo
	if !git.IsRepo(configDir) {
		return fmt.Errorf("repository is not a git repository")
	}

	// Validate that the ref exists
	refExists, err := git.RefExists(configDir, ref)
	if err != nil {
		return fmt.Errorf("validating git ref %s: %w", ref, err)
	}
	if !refExists {
		return fmt.Errorf("git ref %s does not exist", ref)
	}

	// Show preview of what will be restored
	if preview {
		fmt.Printf("Would restore %s from %s\n", repoPath, ref)

		// Show commit info
		commits, err := git.GetFileHistory(configDir, gitPath, 1)
		if err == nil && len(commits) > 0 {
			fmt.Printf("\n%sCurrent version:%s\n", colorLightPink, colorReset)
			fmt.Printf("  %s %s - %s\n", commits[0].Hash[:7], commits[0].Date.Format("2006-01-02"), commits[0].Message)
		}

		// Show diff if requested
		if showDiff {
			fmt.Printf("\n%sDiff:%s\n", colorLightPink, colorReset)
			diffOutput, err := git.GetFileDiffFromRef(configDir, gitPath, ref)
			if err != nil {
				fmt.Printf("%s[!]%s Could not generate diff: %v\n", colorYellow, colorReset, err)
			} else if diffOutput == "" {
				fmt.Println("  No differences found.")
			} else {
				fmt.Println(diffOutput)
			}
		}

		return nil
	}

	// Confirmation
	if !force {
		fmt.Printf("Restore %s from %s?\n", repoPath, ref)
		fmt.Println("This will overwrite the current version.")
		fmt.Println("")

		if !confirmRestore() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-restore", FilePath: repoPath}, cfg); err != nil {
		fmt.Printf("%s[!]%s Pre-restore hook warning: %v\n", colorYellow, colorReset, err)
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

	// Create backup of current version
	backupPath, err := core.CreateBackup(fullRepoPath, cfg)
	if err != nil {
		fmt.Printf("%s[!]%s Could not create backup: %v\n", colorYellow, colorReset, err)
	} else {
		fmt.Printf("%s[OK]%s Backed up current version to %s\n", colorGreen, colorReset, backupPath)
	}

	// Restore from Git
	if err := git.RestoreFile(configDir, gitPath, ref); err != nil {
		return fmt.Errorf("restoring from git: %w", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: repoPath}, cfg); err != nil {
		fmt.Printf("%s[!]%s Post-restore hook warning: %v\n", colorYellow, colorReset, err)
	}

	fmt.Printf("%s[OK]%s Restored %s from %s\n", colorGreen, colorReset, repoPath, ref)
	return nil
}

// restoreFromBackup restores a file from backup
func restoreFromBackup(sourcePath, repoPath string, preview, showDiff, force bool, cfg *config.Config) error {
	// Normalize source path for backup lookup
	normalized, err := config.NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	// Find backups
	backups, err := core.GetBackupsForFile(normalized, cfg)
	if err != nil {
		return fmt.Errorf("finding backups: %w", err)
	}

	if len(backups) == 0 {
		return fmt.Errorf("no backups found for %s", sourcePath)
	}

	// Use most recent backup
	backup := backups[0]

	if preview {
		fmt.Printf("Would restore %s from backup:\n", sourcePath)
		fmt.Printf("  %s (%s)\n", backup.BackupPath, backup.Timestamp.Format("2006-01-02 15:04:05"))

		// Show diff if requested
		if showDiff {
			fmt.Printf("\n%sDiff:%s\n", colorLightPink, colorReset)

			// Expand paths for diff
			expandedBackup, err := config.ExpandPath(backup.BackupPath, cfg)
			if err != nil {
				fmt.Printf("%s[!]%s Could not expand backup path: %v\n", colorYellow, colorReset, err)
				return nil
			}

			expandedRepo, err := config.ExpandPath(repoPath, cfg)
			if err != nil {
				fmt.Printf("%s[!]%s Could not expand repo path: %v\n", colorYellow, colorReset, err)
				return nil
			}

			// Check if files exist
			if !fs.PathExists(expandedBackup) {
				fmt.Println("  Backup file does not exist.")
				return nil
			}

			if !fs.PathExists(expandedRepo) {
				fmt.Printf("  Current file does not exist. Would create from backup.\n")
				return nil
			}

			// Generate diff
			diffOutput, err := git.GetDiffBetweenFiles(expandedRepo, expandedBackup)
			if err != nil {
				fmt.Printf("%s[!]%s Could not generate diff: %v\n", colorYellow, colorReset, err)
			} else if diffOutput == "" {
				fmt.Println("  No differences found.")
			} else {
				fmt.Println(diffOutput)
			}
		}

		return nil
	}

	// Confirmation
	if !force {
		fmt.Printf("Restore %s from backup?\n", sourcePath)
		fmt.Printf("%sBackup:%s %s (%s)\n", colorLightPink, colorReset, backup.BackupPath, backup.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Println("")

		if !confirmRestore() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("%s[!]%s Pre-restore hook warning: %v\n", colorYellow, colorReset, err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("%s[!]%s Post-restore hook warning: %v\n", colorYellow, colorReset, err)
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

	// Restore from backup
	if err := core.RestoreBackup(backup.BackupPath, repoPath, cfg); err != nil {
		return fmt.Errorf("restoring from backup: %w", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("%s[!]%s Post-restore hook warning: %v\n", colorYellow, colorReset, err)
	}

	fmt.Printf("%s[OK]%s Restored %s from backup\n", colorGreen, colorReset, sourcePath)
	return nil
}

// listAllBackups shows all available backups
func listAllBackups(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	configureLogger(cmd, cfg)
	backups, err := core.ListBackups(cfg)
	if err != nil {
		return fmt.Errorf("listing backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}

	fmt.Println("Available backups:")
	fmt.Println("")

	currentDate := ""
	for _, b := range backups {
		date := b.Timestamp.Format("2006-01-02")
		if date != currentDate {
			if currentDate != "" {
				fmt.Println("")
			}
			fmt.Printf("[%s]\n", date)
			currentDate = date
		}

		fmt.Printf("  %s  %s  (%d bytes)\n",
			b.Timestamp.Format("15:04:05"),
			b.SourcePath,
			b.Size,
		)
	}

	fmt.Printf("\n%d backup(s) total\n", len(backups))
	return nil
}

// confirmRestore prompts for confirmation
func confirmRestore() bool {
	fmt.Print("Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}
