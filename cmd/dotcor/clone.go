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

// isValidGitURL checks if URL looks like a valid git remote URL
func isValidGitURL(url string) bool {
	prefixes := []string{
		"http://",
		"https://",
		"git://",
		"git@",
		"ssh://",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}

var cloneCmd = &cobra.Command{
	Use:   "clone <url>",
	Short: "Clone dotfiles from a remote repository",
	Long: `Clone your dotfiles repository to a new machine.

This command:
1. Clones the repository to ~/.dotcor/files
2. Creates symlinks for all managed files
3. Sets up DotCor configuration

This is the recommended way to set up DotCor on a new machine.

Examples:
  dotcor clone git@github.com:user/dotfiles.git
  dotcor clone https://github.com/user/dotfiles.git
  dotcor clone git@github.com:user/dotfiles.git --apply`,
	Args: cobra.ExactArgs(1),
	RunE: runClone,
}

func init() {
	cloneCmd.Flags().Bool("apply", false, "Create symlinks after cloning")
	cloneCmd.Flags().BoolP("force", "f", false, "Overwrite existing dotcor directory")
	rootCmd.AddCommand(cloneCmd)
}

func runClone(cmd *cobra.Command, args []string) error {
	repoURL := args[0]
	apply, _ := cmd.Flags().GetBool("apply")
	force, _ := cmd.Flags().GetBool("force")

	// Load or create config
	cfg, err := config.LoadConfig()
	if err != nil {
		// Config doesn't exist, create default
		cfg, err = config.NewDefaultConfig()
		if err != nil {
			return fmt.Errorf("creating config: %w", err)
		}
	}
	configureLogger(cmd, cfg)

	// Check if git is installed
	if !git.IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	filesDir := configDir + "/files"

	// Check if already exists
	if fs.PathExists(configDir) {
		if !force {
			fmt.Printf("DotCor directory already exists: %s\n", configDir)
			fmt.Print("Overwrite? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input != "y" && input != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		// Remove existing
		fmt.Println("Removing existing DotCor directory...")
		if err := fs.RemoveAll(configDir, cfg); err != nil {
			return fmt.Errorf("removing existing directory: %w", err)
		}
	}

	// Acquire lock - may fail if directory is new, which is expected
	lockErr := core.AcquireLock(cfg)
	if lockErr == nil {
		defer func() {
			if err := core.ReleaseLock(cfg); err != nil {
				cfg.Logger.Error("failed to release lock", "error", err)
			}
		}()
	}
	// Note: Lock acquisition failure is expected when cloning to a new directory

	// Create config directory structure
	fmt.Println("Setting up DotCor...")

	if err := fs.EnsureDir(configDir, cfg); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	backupsDir := configDir + "/backups"
	if err := fs.EnsureDir(backupsDir, cfg); err != nil {
		return fmt.Errorf("creating backups directory: %w", err)
	}

	// Clone repository
	fmt.Printf("Cloning repository from %s...\n", repoURL)

	if err := git.CloneWithProgress(repoURL, filesDir); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	fmt.Println("[OK] Repository cloned")

	// Set up git remote if this is a remote URL
	if isValidGitURL(repoURL) {
		if err := git.SetRemote(filesDir, "origin", repoURL); err != nil {
			fmt.Fprintf(os.Stderr, "[!] Warning: failed to set git remote: %v\n", err)
		} else {
			fmt.Println("[OK] Git remote configured")
		}
	}

	// Check for config.yaml in repo
	configPath := filesDir + "/config.yaml"
	if fs.PathExists(configPath) {
		// Copy config to correct location
		destConfig := configDir + "/config.yaml"
		if err := fs.CopyFile(configPath, destConfig, cfg); err != nil {
			fmt.Printf("[!] Could not copy config: %v\n", err)
		} else {
			// Reload the config from the copied location
			cfg, err = config.LoadConfig()
			if err == nil {
				fmt.Println("[OK] Configuration loaded from repository")
			}
		}
	} else {
		// Save the default config we created earlier
		if err := cfg.SaveConfig(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("[OK] Created default configuration")
		fmt.Println("  Note: Run 'dotcor rebuild-config --scan' to detect files")
	}

	// Apply symlinks if requested
	if apply {
		fmt.Println("")
		fmt.Println("Creating symlinks...")

		return applySymlinks(cfg)
	}

	fmt.Println("")
	fmt.Println("Clone complete!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  dotcor init --apply    # Create symlinks for managed files")
	fmt.Println("  dotcor list            # View managed files")
	fmt.Println("  dotcor status          # Check current state")

	return nil
}
