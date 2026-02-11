package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/utils"
	"github.com/spf13/cobra"
)

var listBackupsCmd = &cobra.Command{
	Use:   "list-backups [file]",
	Short: "List all backups",
	Long: `List all backup files for managed dotfiles.

Shows timestamped backups with their age and size.
Can list all backups or filter by specific file.

Examples:
  dotcor list-backups                 # List all backups
  dotcor list-backups ~/.zshrc          # List backups for specific file`,
	RunE: runListBackups,
}

func init() {
	rootCmd.AddCommand(listBackupsCmd)
}

func runListBackups(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	configureLogger(cmd, cfg)

	backupDir, err := core.GetBackupDir()
	if err != nil {
		return fmt.Errorf("getting backup directory: %w", err)
	}

	if !fs.PathExists(backupDir) {
		fmt.Println("No backups found.")
		return nil
	}

	var backups []BackupEntry
	err = filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == backupDir {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(backupDir, path)
		backupEntry := parseBackupPath(relPath, info, cfg)

		if backupEntry != nil {
			backups = append(backups, *backupEntry)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("reading backup directory: %w", err)
	}

	if len(args) > 0 {
		var filtered []BackupEntry
		for _, backup := range backups {
			for _, arg := range args {
				if backup.SourcePath == arg {
					filtered = append(filtered, backup)
					break
				}
			}
		}
		backups = filtered
	}

	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	fmt.Printf("Found %d backup(s):\n\n", len(backups))

	for _, backup := range backups {
		age := time.Since(backup.Timestamp)
		fmt.Printf("  %s\n", backup.SourcePath)
		fmt.Printf("    Date: %s (%s)\n", backup.Timestamp.Format("2006-01-02 15:04:05"), utils.FormatAge(age))
		fmt.Printf("    Size: %s\n", utils.FormatSize(backup.Size))
		fmt.Printf("    Path: %s\n", backup.BackupPath)
		fmt.Println("")
	}

	return nil
}

type BackupEntry struct {
	SourcePath string
	BackupPath string
	Timestamp  time.Time
	Size       int64
}

func parseBackupPath(relPath string, info os.FileInfo, cfg *config.Config) *BackupEntry {
	parts := splitPathComponents(relPath)
	if len(parts) < 2 {
		return nil
	}

	timestamp, err := time.Parse("2006-01-02_15-04-05", parts[0])
	if err != nil {
		return nil
	}

	sourcePath := filepath.Join(parts[1:]...)

	return &BackupEntry{
		SourcePath: "~/" + sourcePath,
		BackupPath: relPath,
		Timestamp:  timestamp,
		Size:       info.Size(),
	}
}

func splitPathComponents(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if dir == path {
			parts = append(parts, file)
			break
		}
		parts = append(parts, file)
		path = dir
	}
	reverse(parts)
	return parts
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
