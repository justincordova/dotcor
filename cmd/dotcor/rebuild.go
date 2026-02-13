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

var rebuildCmd = &cobra.Command{
	Use:   "rebuild-config",
	Short: "Rebuild config from repository contents",
	Long: `Rebuild the DotCor configuration file from the repository contents.

This is useful when:
- The config file was lost or corrupted
- Migrating from another dotfile manager
- Syncing config with actual repository state

Options:
  --scan     Scan repository for files and add to config
  --verify   Verify config matches repository (no changes)

Examples:
  dotcor rebuild-config --scan      # Add repo files to config
  dotcor rebuild-config --verify    # Check config vs repo`,
	RunE: runRebuild,
}

func init() {
	rebuildCmd.Flags().Bool("scan", false, "Scan repository for files and add to config")
	rebuildCmd.Flags().Bool("verify", false, "Verify config matches repository")
	rebuildCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompts")
	rootCmd.AddCommand(rebuildCmd)
}

func runRebuild(cmd *cobra.Command, args []string) error {
	scan, _ := cmd.Flags().GetBool("scan")
	verify, _ := cmd.Flags().GetBool("verify")
	force, _ := cmd.Flags().GetBool("force")

	if !scan && !verify {
		return fmt.Errorf("specify --scan or --verify")
	}

	// Load config (or create if doesn't exist)
	cfg, err := config.LoadConfig()
	if err != nil {
		// Try to create default config
		cfg, err = config.NewDefaultConfig()
		if err != nil {
			return fmt.Errorf("creating config: %w", err)
		}
	}
	configureLogger(cmd, cfg)

	// Get repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return fmt.Errorf("expanding repo path: %w", err)
	}

	if !fs.PathExists(repoPath) {
		return fmt.Errorf("repository does not exist: %s", repoPath)
	}

	if verify {
		return verifyConfig(cfg, repoPath)
	}

	return scanAndRebuild(cfg, repoPath, force)
}

// verifyConfig checks if config matches repository contents
func verifyConfig(cfg *config.Config, repoPath string) error {
	fmt.Println("Verifying configuration...")
	fmt.Println("")

	// Build set of tracked repo paths
	tracked := make(map[string]bool)
	for _, mf := range cfg.ManagedFiles {
		tracked[mf.RepoPath] = true
	}

	// Find files in repo
	repoFiles, err := scanRepoFiles(repoPath)
	if err != nil {
		return fmt.Errorf("scanning repository: %w", err)
	}

	// Check for discrepancies
	var missing []string  // In config but not in repo
	var orphaned []string // In repo but not in config

	// Check each tracked file exists in repo
	for _, mf := range cfg.ManagedFiles {
		fullPath := filepath.Join(repoPath, mf.RepoPath)
		if !fs.PathExists(fullPath) {
			missing = append(missing, mf.RepoPath)
		}
	}

	// Check each repo file is tracked
	for _, repoFile := range repoFiles {
		if !tracked[repoFile] {
			orphaned = append(orphaned, repoFile)
		}
	}

	// Report
	if len(missing) == 0 && len(orphaned) == 0 {
		fmt.Printf("%s[OK]%s Configuration matches repository\n", colorGreen, colorReset)
		fmt.Printf("  %d file(s) tracked\n", len(cfg.ManagedFiles))
		return nil
	}

	if len(missing) > 0 {
		fmt.Printf("Missing from repository (%d):\n", len(missing))
		for _, m := range missing {
			fmt.Printf("  %s[X]%s %s\n", colorRed, colorReset, m)
		}
		fmt.Println("")
	}

	if len(orphaned) > 0 {
		fmt.Printf("Not in configuration (%d):\n", len(orphaned))
		for _, o := range orphaned {
			fmt.Printf("  ? %s\n", o)
		}
		fmt.Println("")
	}

	fmt.Println("Run 'dotcor rebuild-config --scan' to synchronize.")
	return nil
}

// scanAndRebuild scans repository and updates config
func scanAndRebuild(cfg *config.Config, repoPath string, force bool) error {
	fmt.Println("Scanning repository...")
	fmt.Println("")

	// Build set of already tracked paths
	tracked := make(map[string]bool)
	for _, mf := range cfg.ManagedFiles {
		tracked[mf.RepoPath] = true
	}

	// Find files in repo
	repoFiles, err := scanRepoFiles(repoPath)
	if err != nil {
		return fmt.Errorf("scanning repository: %w", err)
	}

	// Find untracked files
	var untracked []string
	for _, repoFile := range repoFiles {
		if !tracked[repoFile] {
			untracked = append(untracked, repoFile)
		}
	}

	if len(untracked) == 0 {
		fmt.Println("No untracked files found in repository.")
		return nil
	}

	fmt.Printf("Found %d untracked file(s):\n", len(untracked))
	for _, u := range untracked {
		fmt.Printf("  + %s\n", u)
	}
	fmt.Println("")

	// Confirmation
	if !force {
		fmt.Printf("Add %d file(s) to configuration? [y/N]: ", len(untracked))

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input != "y" && input != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
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

	// Add files to config
	added := 0
	for _, repoFile := range untracked {
		// Generate source path from repo path
		sourcePath := generateSourcePath(repoFile)

		mf := config.ManagedFile{
			SourcePath: sourcePath,
			RepoPath:   repoFile,
			AddedAt:    time.Now(),
		}

		cfg.ManagedFiles = append(cfg.ManagedFiles, mf)
		added++
		fmt.Printf("  %s[OK]%s Added %s → %s\n", colorGreen, colorReset, repoFile, sourcePath)
	}

	// Save config
	if err := cfg.SaveConfig(); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("")
	fmt.Printf("Added %d file(s) to configuration.\n", added)

	// Git commit
	if git.IsGitInstalled() && added > 0 {
		message := fmt.Sprintf("Rebuild config: add %d file(s)", added)
		if err := git.AutoCommit(repoPath, message); err != nil {
			fmt.Printf("%s[!]%s Git commit failed: %v\n", colorYellow, colorReset, err)
		} else {
			fmt.Printf("%s[OK]%s Committed to Git\n", colorGreen, colorReset)
		}
	}

	return nil
}

// scanRepoFiles scans the repository for managed files
func scanRepoFiles(repoPath string) ([]string, error) {
	var files []string

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip .git directory
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip config.yaml
		if info.Name() == "config.yaml" {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			return nil
		}

		files = append(files, relPath)
		return nil
	})

	return files, err
}

// generateSourcePath generates a source path from a repo path
func generateSourcePath(repoPath string) string {
	// Common mappings
	// shell/zshrc → ~/.zshrc
	// shell/bashrc → ~/.bashrc
	// git/gitconfig → ~/.gitconfig
	// vim/vimrc → ~/.vimrc
	// config/nvim/init.lua → ~/.config/nvim/init.lua

	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) < 2 {
		return "~/" + addDot(repoPath)
	}

	category := parts[0]
	filename := parts[1]

	switch category {
	case "shell":
		return "~/." + filename
	case "git":
		return "~/." + filename
	case "vim":
		return "~/." + filename
	case "tmux":
		return "~/." + filename
	case "config":
		return "~/.config/" + filename
	default:
		// If category looks like a config dir (contains a file), put in .config
		if strings.Contains(filename, "/") {
			return "~/.config/" + category + "/" + filename
		}
		return "~/." + addDot(filename)
	}
}

// addDot adds a dot prefix if not already present
func addDot(name string) string {
	if strings.HasPrefix(name, ".") {
		return name
	}
	return "." + name
}
