package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [file]",
	Short: "Restore a dotfile from Git history or backup",
	Long: `Restore a dotfile from Git history or from a backup.

By default, restores from the most recent Git commit. Use --to to specify
a different commit or reference. Use --from-backup to restore from a backup.

Examples:
  dotcor restore ~/.zshrc                # Restore from latest commit
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
	restoreCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	toRef, _ := cmd.Flags().GetString("to")
	fromBackup, _ := cmd.Flags().GetBool("from-backup")
	listBackups, _ := cmd.Flags().GetBool("list-backups")
	preview, _ := cmd.Flags().GetBool("preview")
	force, _ := cmd.Flags().GetBool("force")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}

	// Handle --list-backups
	if listBackups {
		return listAllBackups()
	}

	// Require file argument
	if len(args) == 0 {
		return fmt.Errorf("specify a file to restore")
	}

	sourcePath := args[0]

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

	repoRoot, err := config.ExpandPath(cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("expanding repo root: %w", err)
	}

	// Handle backup restore
	if fromBackup {
		return restoreFromBackup(mf.SourcePath, repoPath, preview, force, cfg)
	}

	// Git restore
	return restoreFromGit(repoRoot, mf.RepoPath, repoPath, toRef, preview, force, cfg)
}

// restoreFromGit restores a file from Git history
func restoreFromGit(repoRoot, repoPath, fullRepoPath, ref string, preview, force bool, cfg *config.Config) error {
	// Check if it's a git repo
	if !git.IsRepo(repoRoot) {
		return fmt.Errorf("repository is not a git repository")
	}

	// Show preview of what will be restored
	if preview {
		fmt.Printf("Would restore %s from %s\n", repoPath, ref)

		// Show the commit info
		commits, err := git.GetFileHistory(repoRoot, repoPath, 1)
		if err == nil && len(commits) > 0 {
			fmt.Printf("\nCurrent version:\n")
			fmt.Printf("  %s %s - %s\n", commits[0].Hash[:7], commits[0].Date.Format("2006-01-02"), commits[0].Message)
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
		fmt.Printf("[!] Pre-restore hook warning: %v\n", err)
	}

	// Acquire lock
	if err := core.AcquireLock(cfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer core.ReleaseLock(cfg)

	// Create backup of current version
	backupPath, err := core.CreateBackup(fullRepoPath, cfg)
	if err != nil {
		fmt.Printf("[!] Could not create backup: %v\n", err)
	} else {
		fmt.Printf("[OK] Backed up current version to %s\n", backupPath)
	}

	// Restore from Git
	if err := git.RestoreFile(repoRoot, repoPath, ref); err != nil {
		return fmt.Errorf("restoring from git: %w", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: repoPath}, cfg); err != nil {
		fmt.Printf("[!] Post-restore hook warning: %v\n", err)
	}

	fmt.Printf("[OK] Restored %s from %s\n", repoPath, ref)
	return nil
}

// restoreFromBackup restores a file from backup
func restoreFromBackup(sourcePath, repoPath string, preview, force bool, cfg *config.Config) error {
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
		return nil
	}

	// Confirmation
	if !force {
		fmt.Printf("Restore %s from backup?\n", sourcePath)
		fmt.Printf("Backup: %s (%s)\n", backup.BackupPath, backup.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Println("")

		if !confirmRestore() {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	if err := core.RunHook(core.HookContext{HookType: "pre-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("[!] Pre-restore hook warning: %v\n", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("[!] Post-restore hook warning: %v\n", err)
	}

	// Acquire lock
	if err := core.AcquireLock(cfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer core.ReleaseLock(cfg)

	// Restore from backup
	if err := core.RestoreBackup(backup.BackupPath, repoPath, cfg); err != nil {
		return fmt.Errorf("restoring from backup: %w", err)
	}

	if err := core.RunHook(core.HookContext{HookType: "post-restore", FilePath: sourcePath}, cfg); err != nil {
		fmt.Printf("[!] Post-restore hook warning: %v\n", err)
	}

	fmt.Printf("[OK] Restored %s from backup\n", sourcePath)
	return nil
}

// listAllBackups shows all available backups
func listAllBackups() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
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
