package main

import (
	"fmt"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote [url]",
	Short: "Show or set the git remote URL",
	Long: `Show the current git remote URL or set a new one.

If no URL is provided, shows the current remote.
If a URL is provided, sets the remote to that URL.

Examples:
  dotcor remote                                # Show current remote
  dotcor remote git@github.com:user/dotfiles.git  # Set remote
  dotcor remote https://github.com/user/dotfiles.git # Set remote`,
	RunE: runRemote,
}

func init() {
	remoteCmd.Flags().String("name", "origin", "Remote name (default: origin)")
}

func runRemote(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return fmt.Errorf("expanding repo path: %w", err)
	}

	if !git.IsRepo(repoPath) {
		return fmt.Errorf("dotcor repository is not a git repository")
	}

	if len(args) == 0 {
		remoteURL, err := git.GetRemoteURL(repoPath)
		if err != nil {
			return fmt.Errorf("getting remote: %w", err)
		}

		if remoteURL == "" {
			fmt.Println("No remote configured.")
			fmt.Println("")
			fmt.Println("Set a remote with:")
			fmt.Printf("  dotcor remote git@github.com:user/dotfiles.git\n")
			return nil
		}

		fmt.Printf("Remote: %s\n", remoteURL)
		return nil
	}

	remoteURL := args[0]

	if err := core.AcquireLock(cfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer func() {
		if err := core.ReleaseLock(cfg); err != nil {
			cfg.Logger.Error("failed to release lock", "error", err)
		}
	}()

	if err := git.SetRemote(repoPath, name, remoteURL); err != nil {
		return fmt.Errorf("setting remote: %w", err)
	}

	fmt.Printf("%s[OK]%s Remote set to %s\n", colorGreen, colorReset, remoteURL)
	fmt.Println("")
	fmt.Println("Push changes with:")
	fmt.Println("  dotcor sync")

	return nil
}
