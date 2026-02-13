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

	fmt.Printf("\n  %sDotCor Doctor%s\n", colorLightPink, colorReset)
	fmt.Println("")

	issues := 0
	fixed := 0

	// Check 1: Configuration
	fmt.Println("Checking configuration...")
	configIssues, configFixed := checkConfiguration(fix, cmd)
	issues += configIssues
	fixed += configFixed

	// Check 2: Lock file
	fmt.Println("Checking lock file...")
	lockIssues, lockFixed := checkLockFile(fix, cmd)
	issues += lockIssues
	fixed += lockFixed

	// Check 3: Repository
	fmt.Println("Checking repository...")
	repoIssues, repoFixed := checkRepository(fix, cmd)
	issues += repoIssues
	fixed += repoFixed

	// Check 4: Symlinks
	fmt.Println("Checking symlinks...")
	symlinkIssues, symlinkFixed := checkSymlinks(fix, cmd)
	issues += symlinkIssues
	fixed += symlinkFixed

	// Check 5: Orphaned files
	fmt.Println("Checking for orphaned files...")
	orphanIssues, orphanFixed := checkOrphanedFiles(fix, cmd)
	issues += orphanIssues
	fixed += orphanFixed

	// Check 6: Permissions
	fmt.Println("Checking permissions...")
	permIssues, permFixed := checkPermissions(fix, cmd)
	issues += permIssues
	fixed += permFixed

	// Check 7: Git config
	fmt.Println("Checking git configuration...")
	gitConfigIssues, gitConfigFixed := checkGitConfig(fix, cmd)
	issues += gitConfigIssues
	fixed += gitConfigFixed

	// Check 8: Git remote
	fmt.Println("Checking git remote...")
	gitRemoteIssues, gitRemoteFixed := checkGitRemote(fix, cmd)
	issues += gitRemoteIssues
	fixed += gitRemoteFixed

	// Check 9: Hook permissions
	fmt.Println("Checking hook permissions...")
	hookPermIssues, hookPermFixed := checkHookPermissions(fix, cmd)
	issues += hookPermIssues
	fixed += hookPermFixed

	// Summary
	fmt.Println("")
	fmt.Println("Summary")
	fmt.Println("-------")

	if issues == 0 {
		fmt.Printf("%s[OK]%s No issues found. Your DotCor setup is healthy!\n", colorGreen, colorReset)
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
func checkConfiguration(fix bool, cmd *cobra.Command) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("  %s[X]%s Config error: %v\n", colorRed, colorReset, err)
		issues++

		if fix {
			// Try to create default config
			newCfg, err := config.NewDefaultConfig()
			if err == nil {
				if err := newCfg.SaveConfig(); err == nil {
					fmt.Printf("  %s[OK]%s Created new default config\n", colorGreen, colorReset)
					fixed++
				}
			}
		}
		return
	}
	configureLogger(cmd, cfg)

	// Check repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		fmt.Printf("  %s[X]%s Invalid repo path: %v\n", colorRed, colorReset, err)
		issues++
		return
	}

	if !fs.PathExists(repoPath) {
		fmt.Printf("  %s[X]%s Repository directory missing: %s\n", colorRed, colorReset, repoPath)
		issues++

		if fix {
			cfg, err := config.LoadConfig()
			if err == nil {
				configureLogger(cmd, cfg)
				if err := fs.EnsureDir(repoPath, cfg); err == nil {
					fmt.Printf("  %s[OK]%s Created repository directory: %s\n", colorGreen, colorReset, repoPath)
					fixed++
				}
			}
		}
	}

	fmt.Printf("  %s[OK]%s Configuration valid\n", colorGreen, colorReset)
	return
}

// checkLockFile checks for stale locks
func checkLockFile(fix bool, cmd *cobra.Command) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("  %s[X]%s Config error: %v\n", colorRed, colorReset, err)
		issues++
		return
	}
	configureLogger(cmd, cfg)

	info, err := core.GetLockInfo()
	if err != nil {
		return
	}

	if info == nil {
		fmt.Printf("  %s[OK]%s No lock file\n", colorGreen, colorReset)
		return
	}

	// Check if lock is from our process
	if info.PID == os.Getpid() {
		fmt.Printf("  %s[OK]%s Lock held by current process\n", colorGreen, colorReset)
		return
	}

	// Check if lock is stale
	lockPath, _ := getLockPathForCheck()
	if lockPath == "" {
		return
	}

	stale, _ := core.IsStale(lockPath, cfg)
	if !stale {
		fmt.Printf("  %s[!]%s Lock held by PID %d on %s\n", colorYellow, colorReset, info.PID, info.Hostname)
		fmt.Println("    (Lock appears active - another dotcor process may be running)")
		return
	}

	fmt.Printf("  %s[X]%s Stale lock from PID %d (process dead)\n", colorRed, colorReset, info.PID)
	issues++

	if fix {
		if err := core.ForceReleaseLock(cfg); err == nil {
			fmt.Printf("  %s[OK]%s Removed stale lock\n", colorGreen, colorReset)
			fixed++
		} else {
			fmt.Printf("  %s[X]%s Could not remove lock: %v\n", colorRed, colorReset, err)
		}
	}

	return
}

// checkRepository checks the Git repository
func checkRepository(fix bool, cmd *cobra.Command) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return
	}

	// Check if git is installed
	if !git.IsGitInstalled() {
		fmt.Printf("  %s[!]%s Git is not installed (recommended)\n", colorYellow, colorReset)
		return
	}

	// Check if it's a git repo
	if !git.IsRepo(repoPath) {
		fmt.Printf("  %s[X]%s Not a Git repository: %s\n", colorRed, colorReset, repoPath)
		issues++

		if fix {
			if err := git.InitRepo(repoPath); err == nil {
				fmt.Printf("  %s[OK]%s Initialized Git repository\n", colorGreen, colorReset)
				fixed++
			} else {
				fmt.Printf("  %s[X]%s Could not initialize: %v\n", colorRed, colorReset, err)
			}
		}
		return
	}

	// Check for uncommitted changes
	hasChanges, _ := git.HasChanges(repoPath)
	if hasChanges {
		fmt.Printf("  %s[!]%s Uncommitted changes in repository\n", colorYellow, colorReset)
		fmt.Println("    Run 'dotcor sync' to commit changes")
	} else {
		fmt.Printf("  %s[OK]%s Git repository healthy\n", colorGreen, colorReset)
	}

	return
}

// checkSymlinks validates all managed symlinks
func checkSymlinks(fix bool, cmd *cobra.Command) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}
	configureLogger(cmd, cfg)

	files := cfg.ManagedFiles
	if len(files) == 0 {
		fmt.Println("  - No managed files")
		return
	}

	for _, mf := range files {
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			continue
		}

		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			continue
		}

		// Check if source exists
		if !fs.PathExists(sourcePath) {
			fmt.Printf("  %s[X]%s Missing symlink: %s\n", colorRed, colorReset, mf.SourcePath)
			issues++

			if fix && fs.PathExists(repoPath) {
				if err := fs.CreateSymlink(repoPath, sourcePath, cfg); err == nil {
					fmt.Printf("  %s[OK]%s Recreated symlink: %s\n", colorGreen, colorReset, mf.SourcePath)
					fixed++
				}
			}
			continue
		}

		// Check if it's a symlink
		isLink, _ := fs.IsSymlink(sourcePath)
		if !isLink {
			fmt.Printf("  %s[X]%s Not a symlink: %s (regular file)\n", colorRed, colorReset, mf.SourcePath)
			issues++
			continue
		}

		// Check if symlink is valid
		valid, _ := fs.IsValidSymlink(sourcePath)
		if !valid {
			fmt.Printf("  %s[X]%s Broken symlink: %s\n", colorRed, colorReset, mf.SourcePath)
			issues++

			if fix && fs.PathExists(repoPath) {
				// Remove broken symlink and recreate
				os.Remove(sourcePath)
				if err := fs.CreateSymlink(repoPath, sourcePath, cfg); err == nil {
					fmt.Printf("  %s[OK]%s Fixed symlink: %s\n", colorGreen, colorReset, mf.SourcePath)
					fixed++
				}
			}
		}
	}

	if issues == 0 {
		fmt.Printf("  %s[OK]%s All %d symlinks healthy\n", colorGreen, colorReset, len(files))
	}

	return
}

// checkOrphanedFiles finds files in repo not tracked in config
func checkOrphanedFiles(fix bool, cmd *cobra.Command) (issues, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return
	}

	// Build set of tracked repo paths
	tracked := make(map[string]bool)
	for _, mf := range cfg.ManagedFiles {
		tracked[mf.RepoPath] = true
	}

	// Walk repo directory and find orphans
	orphans := findOrphanedFilesTopLevel(repoPath, tracked)

	if len(orphans) == 0 {
		fmt.Printf("  %s[OK]%s No orphaned files\n", colorGreen, colorReset)
		return
	}

	fmt.Printf("  %s[!]%s Found %d orphaned file(s) in repository:\n", colorYellow, colorReset, len(orphans))
	for _, orphan := range orphans {
		fmt.Printf("    - %s\n", orphan)
	}
	issues += len(orphans)

	// Note: We don't auto-fix orphaned files as they might be intentional
	fmt.Println("    Run 'dotcor rebuild-config --scan' to add them to config")

	return
}

// findOrphanedFilesTopLevel finds files in repo root not tracked in config (non-recursive)
func findOrphanedFilesTopLevel(repoPath string, tracked map[string]bool) []string {
	var orphans []string

	entries, err := os.ReadDir(repoPath)
	if err != nil {
		return orphans
	}

	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == "config.yaml" {
			continue
		}
		if !entry.IsDir() && !tracked[entry.Name()] {
			orphans = append(orphans, entry.Name())
		}
	}

	return orphans
}

// checkPermissions verifies file and directory permissions
func checkPermissions(fix bool, cmd *cobra.Command) (int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0
	}
	configureLogger(cmd, cfg)

	issues := 0
	fixed := 0

	files := cfg.ManagedFiles
	if len(files) == 0 {
		fmt.Println("  - No managed files to check")
		return 0, 0
	}

	for _, mf := range files {
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			continue
		}

		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			continue
		}

		// Check source path permissions
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			continue
		}

		// Check repo file permissions
		repoInfo, err := os.Stat(repoPath)
		if err != nil {
			continue
		}

		// Warn about overly permissive files (world-writable)
		mode := sourceInfo.Mode()
		if mode.Perm()&0002 != 0 {
			fmt.Printf("  %s[X]%s World-writable: %s\n", colorRed, colorReset, mf.SourcePath)
			issues++

			if fix {
				// Remove world-writable permission
				newMode := mode.Perm() &^ 0002
				if err := os.Chmod(sourcePath, newMode); err == nil {
					fmt.Printf("  %s[OK]%s Fixed permissions: %s\n", colorGreen, colorReset, mf.SourcePath)
					fixed++
				}
			}
		}

		// Check if repo file is readable
		if repoInfo.Mode().Perm()&0400 == 0 {
			fmt.Printf("  %s[X]%s Not readable: %s\n", colorRed, colorReset, mf.RepoPath)
			issues++

			if fix {
				if err := os.Chmod(repoPath, repoInfo.Mode()|0400); err == nil {
					fmt.Printf("  %s[OK]%s Made readable: %s\n", colorGreen, colorReset, mf.RepoPath)
					fixed++
				}
			}
		}
	}

	if issues == 0 {
		fmt.Printf("  %s[OK]%s All %d files have correct permissions\n", colorGreen, colorReset, len(files))
	}

	return issues, fixed
}

// checkGitConfig verifies git user configuration
func checkGitConfig(fix bool, cmd *cobra.Command) (int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return 0, 0
	}

	if !git.IsGitInstalled() || !git.IsRepo(repoPath) {
		return 0, 0
	}

	issues := 0
	fixed := 0

	// Check git user.name
	userName, err := git.GetConfig(repoPath, "user.name")
	if err != nil || userName == "" {
		fmt.Printf("  %s[X]%s Git user.name not configured\n", colorRed, colorReset)
		issues++

		if fix {
			// Get current username as default
			currentUser := os.Getenv("USER")
			if currentUser == "" {
				currentUser = os.Getenv("USERNAME")
			}
			if currentUser != "" {
				if err := git.SetConfig(repoPath, "user.name", currentUser); err == nil {
					fmt.Printf("  %s[OK]%s Set user.name to %s\n", colorGreen, colorReset, currentUser)
					fixed++
				}
			}
		}
	}

	// Check git user.email
	userEmail, err := git.GetConfig(repoPath, "user.email")
	if err != nil || userEmail == "" {
		fmt.Printf("  %s[X]%s Git user.email not configured\n", colorRed, colorReset)
		issues++

		if fix {
			// Suggest setting email
			fmt.Printf("  %s[!]%s Run: git config --global user.email 'your-email@example.com'\n", colorYellow, colorReset)
		}
	}

	if issues == 0 {
		fmt.Printf("  %s[OK]%s Git user configuration valid\n", colorGreen, colorReset)
	}

	return issues, fixed
}

// checkGitRemote checks if git remote is configured
func checkGitRemote(fix bool, cmd *cobra.Command) (int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return 0, 0
	}

	if !git.IsGitInstalled() || !git.IsRepo(repoPath) {
		return 0, 0
	}

	issues := 0
	fixed := 0

	remoteURL, err := git.GetRemoteURL(repoPath)
	if err != nil {
		return 0, 0
	}

	if remoteURL == "" {
		fmt.Printf("  %s[X]%s No git remote configured\n", colorRed, colorReset)
		issues++

		if fix {
			fmt.Printf("  %s[!]%s Run: git remote add origin <url>\n", colorYellow, colorReset)
			fmt.Printf("  %s[!]%s Or create a new repository on GitHub/GitLab/Bitbucket\n", colorYellow, colorReset)
		}
	} else {
		fmt.Printf("  %s[OK]%s Git remote configured: %s\n", colorGreen, colorReset, remoteURL)
	}

	return issues, fixed
}

// checkHookPermissions verifies hooks are executable
func checkHookPermissions(fix bool, cmd *cobra.Command) (int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0
	}
	configureLogger(cmd, cfg)

	hooksDir, err := core.GetHooksDir(cfg)
	if err != nil {
		return 0, 0
	}

	issues := 0
	fixed := 0

	// Check if hooks directory exists
	if !fs.PathExists(hooksDir) {
		fmt.Println("  - No hooks directory")
		return 0, 0
	}

	// Common hook names
	hookNames := []string{
		"pre-add", "post-add",
		"pre-remove", "post-remove",
		"pre-sync", "post-sync",
		"pre-restore", "post-restore",
	}

	foundHooks := 0
	for _, hookName := range hookNames {
		hookPath := hooksDir + "/" + hookName

		info, err := os.Stat(hookPath)
		if err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("  %s[X]%s Cannot access hook: %s (%v)\n", colorRed, colorReset, hookName, err)
				issues++
			}
			continue
		}

		if info.IsDir() {
			continue
		}

		foundHooks++

		// Check if hook is executable
		if info.Mode().Perm()&0111 == 0 {
			fmt.Printf("  %s[X]%s Hook not executable: %s\n", colorRed, colorReset, hookName)
			issues++

			if fix {
				if err := os.Chmod(hookPath, 0755); err == nil {
					fmt.Printf("  %s[OK]%s Made executable: %s\n", colorGreen, colorReset, hookName)
					fixed++
				}
			}
		}
	}

	if issues == 0 {
		if foundHooks == 0 {
			fmt.Println("  - No hooks found")
		} else {
			fmt.Printf("  %s[OK]%s All %d hook(s) are executable\n", colorGreen, colorReset, foundHooks)
		}
	}

	return issues, fixed
}
