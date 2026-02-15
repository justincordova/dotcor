package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/utils"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup-backups",
	Short: "Clean up old backup files",
	Long: `Clean up old backup files to free disk space.

By default, removes backups older than 30 days while keeping at least
the 5 most recent backups for each file.

Examples:
  dotcor cleanup-backups                    # Remove backups older than 30 days
  dotcor cleanup-backups --older-than 7d    # Remove backups older than 7 days
  dotcor cleanup-backups --keep 10          # Keep at least 10 recent backups
  dotcor cleanup-backups --all              # Remove all backups`,
	RunE: runCleanup,
}

func init() {
	cleanupCmd.Flags().String("older-than", "30d", "Remove backups older than duration (e.g., 7d, 1w, 1m)")
	cleanupCmd.Flags().Int("keep", 5, "Minimum number of backups to keep")
	cleanupCmd.Flags().Bool("all", false, "Remove all backups")
	cleanupCmd.Flags().Bool("dry-run", false, "Show what would be removed without making changes")
	cleanupCmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	cleanupCmd.Flags().Bool("auto", false, "Use smart defaults: keep last 10 backups, delete older than 30 days (dry-run by default)")
	cleanupCmd.Flags().Bool("execute", false, "Execute cleanup (required with --auto to actually delete)")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	olderThan, _ := cmd.Flags().GetString("older-than")
	keep, _ := cmd.Flags().GetInt("keep")
	all, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	auto, _ := cmd.Flags().GetBool("auto")
	execute, _ := cmd.Flags().GetBool("execute")

	// Handle --auto flag with smart defaults
	if auto {
		fmt.Printf("%sAuto cleanup with defaults:%s\n", colorLightPink, colorReset)
		fmt.Println("  - Keep: last 10 backups")
		fmt.Println("  - Delete: backups older than 30 days")
		fmt.Println("")

		// Apply smart defaults
		keep = 10
		olderThan = "30d"

		// Auto is dry-run by default unless --execute is provided
		if !execute {
			dryRun = true
		}

		// When --execute is used, skip confirmation
		if execute {
			force = true
		}
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Parse duration
	duration, err := utils.ParseDuration(olderThan)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	if all {
		duration = 0
		keep = 0
	}

	// Get current backup stats
	backupCount, err := core.GetBackupCount(cfg)
	if err != nil {
		return fmt.Errorf("getting backup count: %w", err)
	}

	totalSize, err := core.GetTotalBackupSize(cfg)
	if err != nil {
		return fmt.Errorf("getting backup size: %w", err)
	}

	if backupCount == 0 {
		fmt.Println("No backups to clean up.")
		return nil
	}

	fmt.Printf("%sCurrent backups:%s %d files, %s\n", colorLightPink, colorReset, backupCount, utils.FormatSize(totalSize))
	fmt.Println("")

	// Preview what would be deleted (doesn't actually delete)
	candidates, freedSpace, err := core.PreviewCleanup(duration, keep, cfg)
	if err != nil {
		return fmt.Errorf("previewing cleanup: %w", err)
	}

	if len(candidates) == 0 {
		fmt.Println("No backups match cleanup criteria.")
		return nil
	}

	// Dry run - just show what would be deleted
	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
		fmt.Printf("Would delete %d backup set(s), freeing %s\n", len(candidates), utils.FormatSize(freedSpace))
		if auto && !execute {
			fmt.Println("")
			fmt.Printf("Run 'dotcor cleanup-backups --auto --execute' to proceed.\n")
		}
		return nil
	}

	// Confirmation
	if !force {
		fmt.Printf("Delete %d backup set(s), freeing %s? [y/N]: ", len(candidates), utils.FormatSize(freedSpace))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "y" && input != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Actually delete
	deleted, failed, freedSpace, err := core.CleanOldBackups(duration, keep, cfg)
	if err != nil {
		// Report partial success if some deletions worked
		if deleted > 0 {
			fmt.Printf("%s[!]%s Removed %d backup set(s), freed %s\n", colorYellow, colorReset, deleted, utils.FormatSize(freedSpace))
			fmt.Printf("  Failed to remove %d backup set(s): %v\n", failed, err)
		} else {
			return fmt.Errorf("cleaning backups: %w", err)
		}
	} else {
		fmt.Printf("%s[OK]%s Removed %d backup set(s), freed %s\n", colorGreen, colorReset, deleted, utils.FormatSize(freedSpace))
	}

	// Show new stats
	newCount, _ := core.GetBackupCount(cfg)
	newSize, _ := core.GetTotalBackupSize(cfg)
	fmt.Printf("%sRemaining:%s %d files, %s\n", colorLightPink, colorReset, newCount, utils.FormatSize(newSize))

	return nil
}
