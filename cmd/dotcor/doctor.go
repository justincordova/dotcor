package main

import (
	"fmt"
	"os"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and repair DotCor issues",
	Long: `Run diagnostics on your DotCor setup and optionally repair issues.

Checks for:
- Configuration validity
- Symlink health
- Git repository status
- Stale lock files
- Orphaned files

Examples:
  dotcor doctor          # Run diagnostics
  dotcor doctor --fix    # Attempt to fix found issues`,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix found issues")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fix, _ := cmd.Flags().GetBool("fix")

	fmt.Println("DotCor Doctor")
	fmt.Println("=============")
	fmt.Println("")

	issues := 0
	fixed := 0

	// Check 1: Configuration
	fmt.Println("Checking configuration...")
	configIssues, configFixed := checkConfiguration(fix)
	issues += configIssues
	fixed += configFixed

	// Check 2: Lock file
	fmt.Println("Checking lock file...")
	lockIssues, lockFixed := checkLockFile(fix)
	issues += lockIssues
	fixed += lockFixed

	// Check 3: Repository
	fmt.Println("Checking repository...")
	repoIssues, repoFixed := checkRepository(fix)
	issues += repoIssues
	fixed += repoFixed

	// Check 4: Symlinks
	fmt.Println("Checking symlinks...")
	symlinkIssues, symlinkFixed := checkSymlinks(fix)
	issues += symlinkIssues
	fixed += symlinkFixed

	// Check 5: Orphaned files
	fmt.Println("Checking for orphaned files...")
	orphanIssues, orphanFixed := checkOrphanedFiles(fix)
	issues += orphanIssues
	fixed += orphanFixed

	// Summary
	fmt.Println("")
	fmt.Println("Summary")
	fmt.Println("-------")

	if issues == 0 {
		fmt.Println("[OK] No issues found. Your DotCor setup is healthy!")
	} else {
		fmt.Printf("Found %d issue(s)", issues)
		if fix && fixed > 0 {
			fmt.Printf(", fixed %d", fixed)
		}
		fmt.Println("")

		if !fix && issues > fixed {
			fmt.Println("\nRun 'dotcor doctor --fix' to attempt repairs.")
		}
	}

	return nil
}

// checkConfiguration validates the config file
func checkConfiguration(fix bool) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("  [X] Config error: %v\n", err)
		issues++

		if fix {
			// Try to create default config
			newCfg, err := config.NewDefaultConfig()
			if err == nil {
				if err := newCfg.SaveConfig(); err == nil {
					fmt.Println("  [OK] Created new default config")
					fixed++
				}
			}
		}
		return
	}

	// Check repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath)
	if err != nil {
		fmt.Printf("  [X] Invalid repo path: %v\n", err)
		issues++
		return
	}

	if !fs.PathExists(repoPath) {
		fmt.Printf("  [X] Repository directory missing: %s\n", repoPath)
		issues++

		if fix {
			if err := fs.EnsureDir(repoPath); err == nil {
				fmt.Printf("  [OK] Created repository directory: %s\n", repoPath)
				fixed++
			}
		}
	}

	fmt.Println("  [OK] Configuration valid")
	return
}

// checkLockFile checks for stale locks
func checkLockFile(fix bool) (issues, fixed int) {
	info, err := core.GetLockInfo()
	if err != nil {
		return
	}

	if info == nil {
		fmt.Println("  [OK] No lock file")
		return
	}

	// Check if lock is from our process
	if info.PID == os.Getpid() {
		fmt.Println("  [OK] Lock held by current process")
		return
	}

	// Check if lock is stale
	lockPath, _ := getLockPathForCheck()
	if lockPath == "" {
		return
	}

	stale, _ := core.IsStale(lockPath)
	if !stale {
		fmt.Printf("  [!] Lock held by PID %d on %s\n", info.PID, info.Hostname)
		fmt.Println("    (Lock appears active - another dotcor process may be running)")
		return
	}

	fmt.Printf("  [X] Stale lock from PID %d (process dead)\n", info.PID)
	issues++

	if fix {
		if err := core.ForceReleaseLock(); err == nil {
			fmt.Println("  [OK] Removed stale lock")
			fixed++
		} else {
			fmt.Printf("  [X] Could not remove lock: %v\n", err)
		}
	}

	return
}

// checkRepository checks the Git repository
func checkRepository(fix bool) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	repoPath, err := config.ExpandPath(cfg.RepoPath)
	if err != nil {
		return
	}

	// Check if git is installed
	if !git.IsGitInstalled() {
		fmt.Println("  [!] Git is not installed (recommended)")
		return
	}

	// Check if it's a git repo
	if !git.IsRepo(repoPath) {
		fmt.Printf("  [X] Not a Git repository: %s\n", repoPath)
		issues++

		if fix {
			if err := git.InitRepo(repoPath); err == nil {
				fmt.Println("  [OK] Initialized Git repository")
				fixed++
			} else {
				fmt.Printf("  [X] Could not initialize: %v\n", err)
			}
		}
		return
	}

	// Check for uncommitted changes
	hasChanges, _ := git.HasChanges(repoPath)
	if hasChanges {
		fmt.Println("  [!] Uncommitted changes in repository")
		fmt.Println("    Run 'dotcor sync' to commit changes")
	} else {
		fmt.Println("  [OK] Git repository healthy")
	}

	return
}

// checkSymlinks validates all managed symlinks
func checkSymlinks(fix bool) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	files := cfg.GetManagedFilesForPlatform()
	if len(files) == 0 {
		fmt.Println("  - No managed files")
		return
	}

	for _, mf := range files {
		sourcePath, err := config.ExpandPath(mf.SourcePath)
		if err != nil {
			continue
		}

		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			continue
		}

		// Check if source exists
		if !fs.PathExists(sourcePath) {
			fmt.Printf("  [X] Missing symlink: %s\n", mf.SourcePath)
			issues++

			if fix && fs.FileExists(repoPath) {
				if err := fs.CreateSymlink(repoPath, sourcePath); err == nil {
					fmt.Printf("  [OK] Recreated symlink: %s\n", mf.SourcePath)
					fixed++
				}
			}
			continue
		}

		// Check if it's a symlink
		isLink, _ := fs.IsSymlink(sourcePath)
		if !isLink {
			fmt.Printf("  [X] Not a symlink: %s (regular file)\n", mf.SourcePath)
			issues++
			continue
		}

		// Check if symlink is valid
		valid, _ := fs.IsValidSymlink(sourcePath)
		if !valid {
			fmt.Printf("  [X] Broken symlink: %s\n", mf.SourcePath)
			issues++

			if fix && fs.FileExists(repoPath) {
				// Remove broken symlink and recreate
				os.Remove(sourcePath)
				if err := fs.CreateSymlink(repoPath, sourcePath); err == nil {
					fmt.Printf("  [OK] Fixed symlink: %s\n", mf.SourcePath)
					fixed++
				}
			}
		}
	}

	if issues == 0 {
		fmt.Printf("  [OK] All %d symlinks healthy\n", len(files))
	}

	return
}

// checkOrphanedFiles finds files in repo not tracked in config
func checkOrphanedFiles(fix bool) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}

	repoPath, err := config.ExpandPath(cfg.RepoPath)
	if err != nil {
		return
	}

	// Build set of tracked repo paths
	tracked := make(map[string]bool)
	for _, mf := range cfg.ManagedFiles {
		tracked[mf.RepoPath] = true
	}

	// Walk repo directory and find orphans
	orphans := findOrphanedFiles(repoPath, tracked)

	if len(orphans) == 0 {
		fmt.Println("  [OK] No orphaned files")
		return
	}

	fmt.Printf("  [!] Found %d orphaned file(s) in repository:\n", len(orphans))
	for _, orphan := range orphans {
		fmt.Printf("    - %s\n", orphan)
	}
	issues += len(orphans)

	// Note: We don't auto-fix orphaned files as they might be intentional
	fmt.Println("    Run 'dotcor rebuild-config --scan' to add them to config")

	return
}

// findOrphanedFiles finds files in repo not tracked in config
func findOrphanedFiles(repoPath string, tracked map[string]bool) []string {
	var orphans []string

	// Simple walk - look for files not in tracked set
	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return orphans
	}

	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == "config.yaml" {
			continue
		}

		if entry.IsDir() {
			// Recursively check subdirectory
			subOrphans := findOrphanedFilesRecursive(repoPath, entry.Name(), tracked)
			orphans = append(orphans, subOrphans...)
		} else {
			relPath := entry.Name()
			if !tracked[relPath] {
				orphans = append(orphans, relPath)
			}
		}
	}

	return orphans
}

// findOrphanedFilesRecursive recursively finds orphaned files
func findOrphanedFilesRecursive(basePath, relDir string, tracked map[string]bool) []string {
	var orphans []string

	fullDir := basePath + "/" + relDir
	entries, err := os.ReadDir(fullDir)
	if err != nil {
		return orphans
	}

	for _, entry := range entries {
		relPath := relDir + "/" + entry.Name()

		if entry.IsDir() {
			subOrphans := findOrphanedFilesRecursive(basePath, relPath, tracked)
			orphans = append(orphans, subOrphans...)
		} else {
			if !tracked[relPath] {
				orphans = append(orphans, relPath)
			}
		}
	}

	return orphans
}
