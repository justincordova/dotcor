package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var adoptCmd = &cobra.Command{
	Use:   "adopt [symlink]...",
	Short: "Adopt existing symlinks into DotCor management",
	Long: `Adopt existing symlinks that point to files in a dotfiles repository.

This command is useful when migrating from another dotfile manager or when you
already have symlinks set up manually. It adds the symlink relationship to
DotCor's config without moving any files.

Requirements:
- The symlink must exist and be valid
- The target must be inside the DotCor repository (~/.dotcor/files)

Examples:
  dotcor adopt ~/.zshrc                 # Adopt single symlink
  dotcor adopt ~/.zshrc ~/.bashrc       # Adopt multiple symlinks
  dotcor adopt --scan                   # Scan home directory for adoptable symlinks`,
	RunE: runAdopt,
}

func init() {
	adoptCmd.Flags().Bool("scan", false, "Scan home directory for symlinks pointing to dotcor repo")
	adoptCmd.Flags().Bool("dry-run", false, "Show what would be adopted without making changes")
}

func runAdopt(cmd *cobra.Command, args []string) error {
	scanFlag, _ := cmd.Flags().GetBool("scan")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Acquire lock (skip for dry-run)
	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer func() {
			if err := core.ReleaseLock(cfg); err != nil {
				cfg.Logger.Error("failed to release lock", "error", err)
			}
		}()
	}

	var symlinks []string

	if scanFlag {
		// Scan for adoptable symlinks
		found, err := scanForAdoptableSymlinks(cfg)
		if err != nil {
			return fmt.Errorf("scanning for symlinks: %w", err)
		}
		symlinks = found
	} else {
		if len(args) == 0 {
			return fmt.Errorf("specify symlinks to adopt or use --scan to find them")
		}
		symlinks = args
	}

	if len(symlinks) == 0 {
		fmt.Println("No adoptable symlinks found.")
		return nil
	}

	if dryRun {
		fmt.Println("Dry run - no changes will be made:")
		fmt.Println("")
	}

	// Process each symlink
	adopted := 0
	skipped := 0

	for _, symlink := range symlinks {
		result, err := processAdoptSymlink(cfg, symlink, dryRun)
		switch result {
		case adoptResultSuccess:
			adopted++
		case adoptResultSkipped:
			skipped++
		case adoptResultError:
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s[X]%s %s: %v\n", colorRed, colorReset, symlink, err)
			}
			skipped++
		}
	}

	// Summary
	fmt.Println("")
	if dryRun {
		fmt.Printf("Would adopt %d symlink(s)\n", adopted)
		return nil
	}

	fmt.Printf("Adopted %d symlink(s)", adopted)
	if skipped > 0 {
		fmt.Printf(", skipped %d", skipped)
	}
	fmt.Println("")

	// Git commit (config changed, but no new files)
	if git.IsGitInstalled() && adopted > 0 && !dryRun {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err != nil {
			fmt.Printf("%s[!]%s Git commit skipped: invalid repo path: %v\n", colorYellow, colorReset, err)
		} else {
			message := fmt.Sprintf("Adopt %d existing symlink(s)", adopted)
			if err := git.AutoCommit(repoPath, message, cfg.Logger); err != nil {
				fmt.Printf("%s[!]%s Git commit failed: %v\n", colorYellow, colorReset, err)
			}
		}
	}

	return nil
}

type adoptResult int

const (
	adoptResultSuccess adoptResult = iota
	adoptResultSkipped
	adoptResultError
)

// processAdoptSymlink handles adopting a single symlink
func processAdoptSymlink(cfg *config.Config, symlinkPath string, dryRun bool) (adoptResult, error) {
	// Expand and normalize path
	expanded, err := config.ExpandPath(symlinkPath, cfg)
	if err != nil {
		return adoptResultError, fmt.Errorf("expanding path: %w", err)
	}

	normalized, err := config.NormalizePath(symlinkPath)
	if err != nil {
		normalized = symlinkPath
	}

	// Check if it's actually a symlink
	isLink, err := fs.IsSymlink(expanded)
	if err != nil {
		return adoptResultError, fmt.Errorf("checking symlink: %w", err)
	}
	if !isLink {
		return adoptResultError, fmt.Errorf("not a symlink")
	}

	// Get symlink target
	target, err := fs.ReadSymlink(expanded)
	if err != nil {
		return adoptResultError, fmt.Errorf("reading symlink: %w", err)
	}

	// Resolve target to absolute path
	var absoluteTarget string
	if filepath.IsAbs(target) {
		absoluteTarget = target
	} else {
		// Relative symlink - resolve from symlink directory
		symlinkDir := filepath.Dir(expanded)
		absoluteTarget = filepath.Clean(filepath.Join(symlinkDir, target))
	}

	// Check if target exists
	if !fs.PathExists(absoluteTarget) {
		return adoptResultError, fmt.Errorf("symlink target does not exist: %s", target)
	}

	// Check if target is inside dotcor repo
	repoFilesPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return adoptResultError, fmt.Errorf("expanding repo path: %w", err)
	}

	relPath, err := filepath.Rel(repoFilesPath, absoluteTarget)
	targetInRepo := err == nil && relPath != ".." && !(len(relPath) > 2 && relPath[:3] == "../")

	// Check if already managed
	if cfg.IsManaged(symlinkPath) {
		fmt.Printf("  - %s (already managed)\n", normalized)
		return adoptResultSkipped, nil
	}

	if dryRun {
		if targetInRepo {
			fmt.Printf("  + %s → %s (already in repo)\n", normalized, relPath)
		} else {
			fmt.Printf("  + %s → %s (will copy target)\n", normalized, relPath)
		}
		return adoptResultSuccess, nil
	}

	// If target is not in repo, copy it there
	var actualRepoPath string
	if !targetInRepo {
		// Generate repo path for the source
		generatedRepoPath, err := config.GenerateRepoPath(symlinkPath, cfg)
		if err != nil {
			return adoptResultError, fmt.Errorf("generating repo path: %w", err)
		}
		actualRepoPath = generatedRepoPath

		// Copy target file into repo
		repoFilePath, err := config.GetRepoFilePath(cfg, actualRepoPath)
		if err != nil {
			return adoptResultError, fmt.Errorf("getting repo file path: %w", err)
		}

		if err := fs.CopyWithPermissions(absoluteTarget, repoFilePath, cfg); err != nil {
			return adoptResultError, fmt.Errorf("copying target to repo: %w", err)
		}

		fmt.Printf("  %s[OK]%s Copied %s to %s\n", colorGreen, colorReset, absoluteTarget, repoFilePath)
	} else {
		// Target already in repo, use its existing relative path
		actualRepoPath = relPath
	}

	// Recreate symlink to point to repo file
	repoFilePath, err := config.GetRepoFilePath(cfg, actualRepoPath)
	if err != nil {
		return adoptResultError, fmt.Errorf("getting repo file path: %w", err)
	}

	// Remove old symlink
	if err := os.Remove(expanded); err != nil && !os.IsNotExist(err) {
		return adoptResultError, fmt.Errorf("removing old symlink: %w", err)
	}

	// Create new relative symlink
	if err := fs.CreateSymlink(repoFilePath, expanded, cfg); err != nil {
		return adoptResultError, fmt.Errorf("creating new symlink: %w", err)
	}

	// Add to config
	mf := config.ManagedFile{
		SourcePath: normalized,
		RepoPath:   actualRepoPath,
		AddedAt:    time.Now(),
	}

	if err := cfg.AddManagedFile(mf); err != nil {
		return adoptResultError, fmt.Errorf("adding to config: %w", err)
	}

	fmt.Printf("  %s[OK]%s Updated %s → %s\n", colorGreen, colorReset, normalized, actualRepoPath)
	return adoptResultSuccess, nil
}

// scanForAdoptableSymlinks scans the home directory for symlinks pointing to dotcor repo
func scanForAdoptableSymlinks(cfg *config.Config) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	repoFilesPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("expanding repo path: %w", err)
	}

	var adoptable []string

	// Scan common dotfile locations
	locations := []string{
		home,
		filepath.Join(home, ".config"),
	}

	for _, location := range locations {
		if !fs.PathExists(location) {
			continue
		}

		entries, err := os.ReadDir(location)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			fullPath := filepath.Join(location, entry.Name())

			// Check if it's a symlink
			info, err := os.Lstat(fullPath)
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}

			// Read symlink target
			target, err := os.Readlink(fullPath)
			if err != nil {
				continue
			}

			// Resolve target
			var absoluteTarget string
			if filepath.IsAbs(target) {
				absoluteTarget = target
			} else {
				absoluteTarget = filepath.Clean(filepath.Join(filepath.Dir(fullPath), target))
			}

			// Check if target is inside dotcor repo
			relPath, err := filepath.Rel(repoFilesPath, absoluteTarget)
			if err != nil {
				continue
			}
			if relPath == ".." || (len(relPath) > 2 && relPath[:3] == "../") {
				continue
			}

			// Check if already managed
			normalized, _ := config.NormalizePath(fullPath)
			if cfg.IsManaged(normalized) {
				continue
			}

			adoptable = append(adoptable, normalized)
		}
	}

	return adoptable, nil
}
