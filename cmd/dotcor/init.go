package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

// Common dotfiles to scan for in interactive mode
var commonDotfiles = []string{
	"~/.zshrc",
	"~/.bashrc",
	"~/.bash_profile",
	"~/.profile",
	"~/.gitconfig",
	"~/.gitignore_global",
	"~/.vimrc",
	"~/.tmux.conf",
	"~/.config/nvim/init.vim",
	"~/.config/nvim/init.lua",
	"~/.config/alacritty/alacritty.yml",
	"~/.config/alacritty/alacritty.toml",
	"~/.config/kitty/kitty.conf",
	"~/.config/starship.toml",
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize DotCor repository",
	Long: `Creates ~/.dotcor directory structure and initializes Git repository.

Examples:
  dotcor init                    # Basic initialization
  dotcor init --interactive      # Scan for dotfiles and select which to add
  dotcor init --apply            # Create symlinks from existing config (new machine)`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().Bool("apply", false, "Create symlinks from existing config (for new machine setup)")
	initCmd.Flags().Bool("interactive", false, "Interactively select existing dotfiles to add")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	applyFlag, _ := cmd.Flags().GetBool("apply")
	interactiveFlag, _ := cmd.Flags().GetBool("interactive")

	// Check symlink support first
	supported, err := fs.SupportsSymlinks()
	if err != nil {
		return fmt.Errorf("checking symlink support: %w", err)
	}
	if !supported {
		fmt.Fprintln(os.Stderr, "[X] Symlinks not supported on this platform.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Windows users: Enable Developer Mode")
		fmt.Fprintln(os.Stderr, "  Settings → Update & Security → For developers → Developer Mode")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Then restart your terminal and try again.")
		return fmt.Errorf("symlinks not supported")
	}

	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	// Check if already initialized
	if fs.PathExists(configDir) && !applyFlag {
		fmt.Printf("DotCor is already initialized at %s\n", configDir)
		fmt.Println("Use 'dotcor status' to check current state.")
		fmt.Println("Use 'dotcor init --apply' to create symlinks from existing config.")
		return nil
	}

	// Create default config for lock operations
	defaultCfg, err := config.NewDefaultConfig()
	if err != nil {
		return fmt.Errorf("creating default config: %w", err)
	}

	// Acquire lock
	if err := core.AcquireLock(defaultCfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer core.ReleaseLock(defaultCfg)

	// Create directory structure
	filesDir := filepath.Join(configDir, "files")
	backupsDir := filepath.Join(configDir, "backups")
	hooksDir := filepath.Join(configDir, "hooks")

	fmt.Println("Initializing DotCor...")

	if err := fs.EnsureDir(configDir, defaultCfg); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	fmt.Printf("[OK] Created %s\n", configDir)

	if err := fs.EnsureDir(filesDir, defaultCfg); err != nil {
		return fmt.Errorf("creating files directory: %w", err)
	}

	if err := fs.EnsureDir(backupsDir, defaultCfg); err != nil {
		return fmt.Errorf("creating backups directory: %w", err)
	}

	if err := fs.EnsureDir(hooksDir, defaultCfg); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	// Initialize Git repository
	if git.IsGitInstalled() {
		if !git.IsRepo(filesDir) {
			if err := git.InitRepo(filesDir); err != nil {
				fmt.Printf("[!] Git init failed: %v\n", err)
			} else {
				fmt.Println("[OK] Initialized Git repository")
			}
		}
	} else {
		fmt.Println("[!] Git not found. Installing Git is recommended for version control.")
	}

	// Create or load config
	var cfg *config.Config
	if applyFlag {
		// Load existing config
		cfg, err = config.LoadConfig()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
	} else {
		// Create new default config
		cfg, err = config.NewDefaultConfig()
		if err != nil {
			return fmt.Errorf("creating default config: %w", err)
		}
		if err := cfg.SaveConfig(); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		fmt.Println("[OK] Created config.yaml")
	}

	// Handle --apply flag (create symlinks from existing config)
	if applyFlag {
		return applySymlinks(cfg)
	}

	// Handle --interactive flag
	if interactiveFlag {
		return interactiveInit(cfg)
	}

	fmt.Println("")
	fmt.Println("DotCor initialized successfully!")
	fmt.Println("")
	fmt.Println("Next steps:")
	fmt.Println("  dotcor add ~/.zshrc     # Add a dotfile")
	fmt.Println("  dotcor list             # List managed files")
	fmt.Println("  dotcor status           # Check status")

	return nil
}

// applySymlinks creates symlinks for all managed files in config
func applySymlinks(cfg *config.Config) error {
	files := cfg.GetManagedFilesForPlatform()
	if len(files) == 0 {
		fmt.Println("No files configured for this platform.")
		return nil
	}

	fmt.Printf("\nCreating symlinks for %d files...\n", len(files))

	created := 0
	skipped := 0

	for _, mf := range files {
		// Get full paths
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			fmt.Printf("  [X] %s (invalid path)\n", mf.SourcePath)
			continue
		}

		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			fmt.Printf("  [X] %s (invalid repo path)\n", mf.SourcePath)
			continue
		}

		// Check if repo file exists
		if !fs.PathExists(repoPath) {
			fmt.Printf("  [X] %s (not in repository)\n", mf.SourcePath)
			continue
		}

		// Check if symlink already exists and is correct
		if isLink, _ := fs.IsSymlink(sourcePath); isLink {
			if valid, _ := fs.IsValidSymlink(sourcePath); valid {
				fmt.Printf("  - %s (already linked)\n", mf.SourcePath)
				skipped++
				continue
			}
		}

		// Backup existing file if it exists
		if fs.PathExists(sourcePath) {
			backupPath, err := core.CreateBackup(sourcePath, cfg)
			if err != nil {
				fmt.Printf("  [X] %s (backup failed: %v)\n", mf.SourcePath, err)
				continue
			}
			fmt.Printf("  → Backed up to %s\n", backupPath)
			os.Remove(sourcePath)
		}

		// Create symlink
		if err := fs.CreateSymlink(repoPath, sourcePath, cfg); err != nil {
			fmt.Printf("  [X] %s (%v)\n", mf.SourcePath, err)
			continue
		}

		fmt.Printf("  [OK] %s\n", mf.SourcePath)
		created++
	}

	fmt.Printf("\nCreated %d symlinks, skipped %d\n", created, skipped)
	return nil
}

// interactiveInit scans for common dotfiles and offers to add them
func interactiveInit(cfg *config.Config) error {
	fmt.Println("\nChecking for existing dotfiles in your home directory...")
	fmt.Println("")

	// Find existing dotfiles
	var found []string
	var ignored []string

	for _, dotfile := range commonDotfiles {
		expanded, err := config.ExpandPath(dotfile, cfg)
		if err != nil {
			continue
		}

		if fs.PathExists(expanded) {
			// Check if it matches ignore patterns
			shouldIgnore, pattern := core.ShouldIgnore(expanded, cfg.IgnorePatterns)
			if shouldIgnore {
				ignored = append(ignored, fmt.Sprintf("%s (ignored - matches %s)", dotfile, pattern))
			} else {
				found = append(found, dotfile)
			}
		}
	}

	if len(found) == 0 {
		fmt.Println("No common dotfiles found.")
		return nil
	}

	fmt.Printf("Found %d dotfiles:\n", len(found))
	for i, f := range found {
		fmt.Printf("  [%d] %s\n", i+1, f)
	}

	if len(ignored) > 0 {
		fmt.Println("\nIgnored:")
		for _, f := range ignored {
			fmt.Printf("  - %s\n", f)
		}
	}

	fmt.Println("")
	fmt.Printf("Add all %d files? [Y/n]: ", len(found))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "" && input != "y" && input != "yes" {
		fmt.Println("Cancelled.")
		return nil
	}

	// Add selected files
	fmt.Println("\nAdding files...")
	added := 0

	for _, dotfile := range found {
		if err := addFile(cfg, dotfile, "", false); err != nil {
			fmt.Printf("  [X] %s: %v\n", dotfile, err)
		} else {
			fmt.Printf("  [OK] %s\n", dotfile)
			added++
		}
	}

	// Git commit
	if git.IsGitInstalled() && added > 0 {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err != nil {
			fmt.Printf("[!] Git commit skipped: invalid repo path: %v\n", err)
		} else {
			if err := git.AutoCommit(repoPath, fmt.Sprintf("Add %d dotfiles via interactive init", added)); err != nil {
				fmt.Printf("[!] Git commit failed: %v\n", err)
			} else {
				fmt.Println("[OK] Committed to Git")
			}
		}
	}

	fmt.Printf("\nDotCor setup complete! %d dotfiles managed.\n", added)
	return nil
}

// addFile adds a single file to dotcor management (used by interactive init)
func addFile(cfg *config.Config, sourcePath string, customRepoPath string, force bool) error {
	// Expand source path
	expanded, err := config.ExpandPath(sourcePath, cfg)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Validate
	if err := core.ValidateSourceFile(expanded, cfg); err != nil {
		return err
	}

	if cfg.IsManaged(sourcePath) {
		return fmt.Errorf("already managed")
	}

	// Generate repo path
	repoPath, err := config.GenerateRepoPath(sourcePath, customRepoPath, cfg)
	if err != nil {
		return fmt.Errorf("generating repo path: %w", err)
	}

	// Get full repo file path
	fullRepoPath, err := config.GetRepoFilePath(cfg, repoPath)
	if err != nil {
		return err
	}

	// Create backup
	if _, err := core.CreateBackup(expanded, cfg); err != nil {
		// Non-fatal, continue
	}

	// Move file to repo
	if err := fs.MoveFile(expanded, fullRepoPath, cfg); err != nil {
		return fmt.Errorf("moving file: %w", err)
	}

	// Create symlink
	if err := fs.CreateSymlink(fullRepoPath, expanded, cfg); err != nil {
		// Rollback: move file back
		fs.MoveFile(fullRepoPath, expanded, cfg)
		return fmt.Errorf("creating symlink: %w", err)
	}

	// Add to config
	normalized, _ := config.NormalizePath(sourcePath)
	mf := config.ManagedFile{
		SourcePath: normalized,
		RepoPath:   repoPath,
		AddedAt:    time.Now(),
		Platforms:  []string{},
	}

	cfg.ManagedFiles = append(cfg.ManagedFiles, mf)
	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}
