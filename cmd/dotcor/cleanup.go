package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
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
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	olderThan, _ := cmd.Flags().GetString("older-than")
	keep, _ := cmd.Flags().GetInt("keep")
	all, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Parse duration
	duration, err := parseDuration(olderThan)
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

	fmt.Printf("Current backups: %d files, %s\n", backupCount, formatSize(totalSize))
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
		fmt.Printf("Would delete %d backup set(s), freeing %s\n", len(candidates), formatSize(freedSpace))
		return nil
	}

	// Confirmation
	if !force {
		fmt.Printf("Delete %d backup set(s), freeing %s? [y/N]: ", len(candidates), formatSize(freedSpace))

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
			fmt.Printf("[!] Removed %d backup set(s), freed %s\n", deleted, formatSize(freedSpace))
			fmt.Printf("  Failed to remove %d backup set(s): %v\n", failed, err)
		} else {
			return fmt.Errorf("cleaning backups: %w", err)
		}
	} else {
		fmt.Printf("[OK] Removed %d backup set(s), freed %s\n", deleted, formatSize(freedSpace))
	}

	// Show new stats
	newCount, _ := core.GetBackupCount(cfg)
	newSize, _ := core.GetTotalBackupSize(cfg)
	fmt.Printf("Remaining: %d files, %s\n", newCount, formatSize(newSize))

	return nil
}

// parseDuration parses a human-friendly duration string
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))

	if s == "" {
		return 30 * 24 * time.Hour, nil // Default: 30 days
	}

	// Handle common formats
	var multiplier time.Duration
	var value int

	if strings.HasSuffix(s, "d") {
		multiplier = 24 * time.Hour
		_, err := fmt.Sscanf(s, "%dd", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "w") {
		multiplier = 7 * 24 * time.Hour
		_, err := fmt.Sscanf(s, "%dw", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "m") {
		multiplier = 30 * 24 * time.Hour // Approximate month
		_, err := fmt.Sscanf(s, "%dm", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else if strings.HasSuffix(s, "h") {
		multiplier = time.Hour
		_, err := fmt.Sscanf(s, "%dh", &value)
		if err != nil {
			return 0, fmt.Errorf("invalid format: %s", s)
		}
		if value <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
	} else {
		// Try standard Go duration
		return time.ParseDuration(s)
	}

	return time.Duration(value) * multiplier, nil
}

// formatSize formats bytes as human-readable size
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
