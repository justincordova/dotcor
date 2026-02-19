package main

import (
	"fmt"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List managed dotfiles",
	Long: `List all managed dotfiles with optional filtering.

Shows managed files with their symlink status and git status.

Examples:
  dotcor list                           # List all files
  dotcor list --pattern "config/*"     # Filter by pattern
  dotcor list --modified               # Show only modified files
  dotcor list --broken                  # Show only files with issues
  dotcor list --healthy                 # Show only healthy files`,
	RunE: runList,
}

func init() {
	listCmd.Flags().String("pattern", "", "Filter by glob pattern (e.g., \"config/nvim/*\")")
	listCmd.Flags().Bool("modified", false, "Show only files with uncommitted changes")
	listCmd.Flags().Bool("broken", false, "Show only files with issues")
	listCmd.Flags().Bool("healthy", false, "Show only healthy files")
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	configureLogger(cmd, cfg)

	files := cfg.ManagedFiles
	if len(files) == 0 {
		fmt.Println("No files managed.")
		return nil
	}

	pattern, _ := cmd.Flags().GetString("pattern")
	modifiedOnly, _ := cmd.Flags().GetBool("modified")
	brokenOnly, _ := cmd.Flags().GetBool("broken")
	healthyOnly, _ := cmd.Flags().GetBool("healthy")

	configDir, err := config.GetConfigDir()
	var changedFiles []string
	if err == nil && git.IsGitInstalled() && git.IsRepo(configDir) {
		gitStatus, _ := git.GetStatus(configDir)
		changedFiles = gitStatus.ChangedFiles
	}

	var filtered []struct {
		path    string
		status  string
		problem string
	}

	filterActive := pattern != "" || modifiedOnly || brokenOnly || healthyOnly

	for _, f := range files {
		fs := CheckFileStatus(cfg, f, changedFiles)

		if pattern != "" {
			matched := core.MatchesPattern(fs.SourcePath, pattern)
			if !matched {
				continue
			}
		}

		if modifiedOnly && fs.Status != "modified" {
			continue
		}

		if brokenOnly && fs.Status == "ok" {
			continue
		}

		if healthyOnly && fs.Status != "ok" {
			continue
		}

		filtered = append(filtered, struct {
			path    string
			status  string
			problem string
		}{
			path:    fs.SourcePath,
			status:  fs.Status,
			problem: fs.Problem,
		})
	}

	if len(filtered) == 0 {
		fmt.Println("No files found matching criteria.")
		return nil
	}

	if filterActive {
		fmt.Printf("%sFilter applied: %d result(s)%s\n\n", colorLightPink, len(filtered), colorReset)
	}

	for _, f := range filtered {
		icon := getStatusIcon(f.status)
		switch f.status {
		case "ok":
			fmt.Printf("  %s %s\n", icon, f.path)
		case "modified":
			fmt.Printf("  %s %s [uncommitted]\n", icon, f.path)
		default:
			fmt.Printf("  %s %s [%s]\n", icon, f.path, f.problem)
		}
	}

	return nil
}
