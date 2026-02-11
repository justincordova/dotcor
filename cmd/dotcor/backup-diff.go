package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/spf13/cobra"
)

var backupDiffCmd = &cobra.Command{
	Use:   "backup-diff [file]",
	Short: "Show changes since last backup",
	Long: `Compare current files to latest backup.

Shows differences between current files and most recent backup.
Useful for reviewing what would be restored from backup.

Examples:
  dotcor backup-diff ~/.zshrc           # Show diff for specific file
  dotcor backup-diff                       # Show diffs for all files`,
	RunE: runBackupDiff,
}

func init() {
	backupDiffCmd.Flags().BoolP("stat", "s", false, "Show summary of changes")
	rootCmd.AddCommand(backupDiffCmd)
}

func runBackupDiff(cmd *cobra.Command, args []string) error {
	showStat, _ := cmd.Flags().GetBool("stat")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	configureLogger(cmd, cfg)

	files := cfg.ManagedFiles
	if len(args) > 0 {
		var filtered []config.ManagedFile
		for _, arg := range args {
			mf, err := cfg.GetManagedFile(arg)
			if err == nil {
				filtered = append(filtered, *mf)
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		return fmt.Errorf("no files to compare")
	}

	for _, mf := range files {
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			continue
		}

		latestBackup, err := core.GetLatestBackup(mf.SourcePath, cfg)
		if err != nil {
			fmt.Printf("  - %s: no backup\n", mf.SourcePath)
			continue
		}

		fmt.Printf("Changes since backup for %s:\n", mf.SourcePath)
		fmt.Printf("  Backup: %s (%s)\n", latestBackup.BackupPath, latestBackup.Timestamp.Format("2006-01-02 15:04:05"))

		if showStat {
			showBackupStat(sourcePath, latestBackup.BackupPath)
		} else {
			showBackupDiff(sourcePath, latestBackup.BackupPath)
		}
		fmt.Println("")
	}

	return nil
}

func showBackupDiff(sourcePath, backupPath string) {
	cmd := exec.Command("diff", "-u", backupPath, sourcePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func showBackupStat(sourcePath, backupPath string) {
	cmd := exec.Command("diff", "--stat", backupPath, sourcePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  Error getting diff stat: %v\n", err)
		return
	}
	fmt.Printf("  %s\n", strings.TrimSpace(string(output)))
}
