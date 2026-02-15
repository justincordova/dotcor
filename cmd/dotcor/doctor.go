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

const (
	severityCritical = "CRITICAL"
	severityWarning  = "WARNING"
	severityInfo     = "INFO"
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
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix CRITICAL issues (requires explicit consent)")
	doctorCmd.Flags().Bool("fix-all", false, "Attempt to fix WARNING and INFO issues automatically")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	fix, _ := cmd.Flags().GetBool("fix")
	fixAll, _ := cmd.Flags().GetBool("fix-all")

	fmt.Printf("\n  %sDotCor Doctor%s\n", colorLightPink, colorReset)
	fmt.Println("")

	issues := 0
	critical := 0
	fixed := 0

	// Check 1: Configuration
	fmt.Println("Checking configuration...")
	configIssues, configCritical, configFixed := checkConfiguration(fix, fixAll, cmd)
	issues += configIssues
	critical += configCritical
	fixed += configFixed

	// Check 2: Lock file
	fmt.Println("Checking lock file...")
	lockIssues, lockCritical, lockFixed := checkLockFile(fix, fixAll, cmd)
	issues += lockIssues
	critical += lockCritical
	fixed += lockFixed

	// Check 3: Repository
	fmt.Println("Checking repository...")
	repoIssues, repoCritical, repoFixed := checkRepository(fix, fixAll, cmd)
	issues += repoIssues
	critical += repoCritical
	fixed += repoFixed

	// Check 4: Symlinks
	fmt.Println("Checking symlinks...")
	symlinkIssues, symlinkCritical, symlinkFixed := checkSymlinks(fix, fixAll, cmd)
	issues += symlinkIssues
	critical += symlinkCritical
	fixed += symlinkFixed

	// Check 5: Orphaned files
	fmt.Println("Checking for orphaned files...")
	orphanIssues, orphanCritical, orphanFixed := checkOrphanedFiles(fix, fixAll, cmd)
	issues += orphanIssues
	critical += orphanCritical
	fixed += orphanFixed

	// Check 6: Permissions
	fmt.Println("Checking permissions...")
	permIssues, permCritical, permFixed := checkPermissions(fix, fixAll, cmd)
	issues += permIssues
	critical += permCritical
	fixed += permFixed

	// Check 7: Git config
	fmt.Println("Checking git configuration...")
	gitConfigIssues, gitConfigCritical, gitConfigFixed := checkGitConfig(fix, fixAll, cmd)
	issues += gitConfigIssues
	critical += gitConfigCritical
	fixed += gitConfigFixed

	// Check 8: Git remote
	fmt.Println("Checking git remote...")
	gitRemoteIssues, gitRemoteCritical, gitRemoteFixed := checkGitRemote(fix, fixAll, cmd)
	issues += gitRemoteIssues
	critical += gitRemoteCritical
	fixed += gitRemoteFixed

	// Check 9: Hook permissions
	fmt.Println("Checking hook permissions...")
	hookPermIssues, hookPermCritical, hookPermFixed := checkHookPermissions(fix, fixAll, cmd)
	issues += hookPermIssues
	critical += hookPermCritical
	fixed += hookPermFixed

	// Summary
	fmt.Println("")
	fmt.Println("Summary")
	fmt.Println("-------")

	if issues == 0 {
		fmt.Printf("%s[OK]%s No issues found. Your DotCor setup is healthy!\n", colorGreen, colorReset)
	} else {
		if critical > 0 {
			fmt.Printf("%s[CRITICAL]%s %d critical issue(s)", colorCritical, colorReset, critical)
			if fix && fixed > 0 {
				fmt.Printf(", fixed %d", fixed)
			}
			fmt.Println("")
		} else {
			fmt.Printf("Found %d issue(s)", issues)
			if (fix || fixAll) && fixed > 0 {
				fmt.Printf(", fixed %d", fixed)
			}
			fmt.Println("")
		}

		if !fix && !fixAll && issues > fixed {
			fmt.Println("\nRun 'dotcor doctor --fix' to attempt repairs.")
			fmt.Println("Run 'dotcor doctor --fix-all' to auto-fix non-critical issues.")
		}
	}

	return nil
}

// shouldFix determines if an issue should be fixed based on severity and flags
func shouldFix(severity string, fixCritical, fixNonCritical bool) bool {
	if severity == severityCritical {
		return fixCritical
	}
	if severity == severityWarning || severity == severityInfo {
		return fixNonCritical
	}
	return false
}

// checkConfiguration validates the config file
func checkConfiguration(fix, fixAll bool, cmd *cobra.Command) (issues, critical, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("  %s[%s]%s Config error: %v\n", colorCritical, severityCritical, colorReset, err)
		issues++
		critical++

		if shouldFix(severityCritical, fix, fixAll) {
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
		fmt.Printf("  %s[%s]%s Invalid repo path: %v\n", colorCritical, severityCritical, colorReset, err)
		issues++
		critical++
		return
	}

	if !fs.PathExists(repoPath) {
		fmt.Printf("  %s[%s]%s Repository directory missing: %s\n", colorCritical, severityCritical, colorReset, repoPath)
		issues++
		critical++

		if shouldFix(severityCritical, fix, fixAll) {
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
func checkLockFile(fix, fixAll bool, cmd *cobra.Command) (issues, critical, fixed int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("  %s[%s]%s Config error: %v\n", colorCritical, severityCritical, colorReset, err)
		issues++
		critical++
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
		fmt.Printf("  %s[%s]%s Lock held by PID %d on %s\n", colorWarnLabel, severityWarning, colorReset, info.PID, info.Hostname)
		fmt.Println("    (Lock appears active - another dotcor process may be running)")
		return
	}

	fmt.Printf("  %s[%s]%s Stale lock from PID %d (process dead)\n", colorWarnLabel, severityWarning, colorReset, info.PID)
	issues++

	if shouldFix(severityWarning, fix, fixAll) {
		if err := core.ForceReleaseLock(cfg); err == nil {
			fmt.Printf("  %s[OK]%s Removed stale lock\n", colorGreen, colorReset)
			fixed++
		} else {
			fmt.Printf("  %s[X]%s Could not remove lock: %v\n", colorRed, colorReset, err)
		}
	} else {
		fmt.Printf("  %s[Suggested]%s Remove stale lock file: %s\n", colorInfoLabel, colorReset, lockPath)
	}

	return
}

// checkRepository checks the Git repository
func checkRepository(fix, fixAll bool, cmd *cobra.Command) (issues, critical, fixed int) {
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
		fmt.Printf("  %s[%s]%s Git is not installed (recommended)\n", colorInfoLabel, severityInfo, colorReset)
		return
	}

	// Check if it's a git repo
	if !git.IsRepo(repoPath) {
		fmt.Printf("  %s[%s]%s Not a Git repository: %s\n", colorCritical, severityCritical, colorReset, repoPath)
		issues++
		critical++

		if shouldFix(severityCritical, fix, fixAll) {
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
		fmt.Printf("  %s[%s]%s Uncommitted changes in repository\n", colorWarnLabel, severityWarning, colorReset)
		fmt.Printf("    %s[Suggested]%s Run: dotcor sync\n", colorInfoLabel, colorReset)
	} else {
		fmt.Printf("  %s[OK]%s Git repository healthy\n", colorGreen, colorReset)
	}

	return
}

// checkSymlinks validates all managed symlinks
func checkSymlinks(fix, fixAll bool, cmd *cobra.Command) (issues, critical, fixed int) {
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
			fmt.Printf("  %s[%s]%s Missing symlink: %s\n", colorWarnLabel, severityWarning, colorReset, mf.SourcePath)
			issues++

			if shouldFix(severityWarning, fix, fixAll) && fs.PathExists(repoPath) {
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
			fmt.Printf("  %s[%s]%s Not a symlink: %s (regular file)\n", colorWarnLabel, severityWarning, colorReset, mf.SourcePath)
			issues++
			continue
		}

		// Check if symlink is valid
		valid, _ := fs.IsValidSymlink(sourcePath)
		if !valid {
			fmt.Printf("  %s[%s]%s Broken symlink: %s\n", colorWarnLabel, severityWarning, colorReset, mf.SourcePath)
			issues++

			if shouldFix(severityWarning, fix, fixAll) && fs.PathExists(repoPath) {
				// Remove broken symlink and recreate
				os.Remove(sourcePath)
				if err := fs.CreateSymlink(repoPath, sourcePath, cfg); err == nil {
					fmt.Printf("  %s[OK]%s Fixed symlink: %s\n", colorGreen, colorReset, mf.SourcePath)
					fixed++
				}
			}
		}
	}

	// Determine severity based on percentage of broken symlinks
	if issues > 0 {
		ratio := float64(issues) / float64(len(files))
		if ratio > 0.5 {
			critical = issues
		}
	}

	if issues == 0 {
		fmt.Printf("  %s[OK]%s All %d symlinks healthy\n", colorGreen, colorReset, len(files))
	}

	return
}

// checkOrphanedFiles finds files in repo not tracked in config
func checkOrphanedFiles(fix, fixAll bool, cmd *cobra.Command) (issues, critical, fixed int) {
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

	fmt.Printf("  %s[%s]%s Found %d orphaned file(s) in repository:\n", colorInfoLabel, severityInfo, colorReset, len(orphans))
	for _, orphan := range orphans {
		fmt.Printf("    - %s\n", orphan)
	}
	issues += len(orphans)

	// Note: We don't auto-fix orphaned files as they might be intentional
	fmt.Printf("    %s[Suggested]%s Run: dotcor rebuild-config --scan\n", colorInfoLabel, colorReset)

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
func checkPermissions(fix, fixAll bool, cmd *cobra.Command) (int, int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0, 0
	}
	configureLogger(cmd, cfg)

	issues := 0
	critical := 0
	fixed := 0

	files := cfg.ManagedFiles
	if len(files) == 0 {
		fmt.Println("  - No managed files to check")
		return 0, 0, 0
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
			fmt.Printf("  %s[%s]%s World-writable: %s\n", colorCritical, severityCritical, colorReset, mf.SourcePath)
			issues++
			critical++

			if shouldFix(severityCritical, fix, fixAll) {
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
			fmt.Printf("  %s[%s]%s Not readable: %s\n", colorWarnLabel, severityWarning, colorReset, mf.RepoPath)
			issues++

			if shouldFix(severityWarning, fix, fixAll) {
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

	return issues, critical, fixed
}

// checkGitConfig verifies git user configuration
func checkGitConfig(fix, fixAll bool, cmd *cobra.Command) (int, int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0, 0
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return 0, 0, 0
	}

	if !git.IsGitInstalled() || !git.IsRepo(repoPath) {
		return 0, 0, 0
	}

	issues := 0
	critical := 0
	fixed := 0

	// Check git user.name
	userName, err := git.GetConfig(repoPath, "user.name")
	if err != nil || userName == "" {
		fmt.Printf("  %s[%s]%s Git user.name not configured\n", colorWarnLabel, severityWarning, colorReset)
		issues++

		if shouldFix(severityWarning, fix, fixAll) {
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
		} else {
			fmt.Printf("    %s[Suggested]%s Run: git config --global user.name \"Your Name\"\n", colorInfoLabel, colorReset)
		}
	}

	// Check git user.email
	userEmail, err := git.GetConfig(repoPath, "user.email")
	if err != nil || userEmail == "" {
		fmt.Printf("  %s[%s]%s Git user.email not configured\n", colorWarnLabel, severityWarning, colorReset)
		issues++

		if shouldFix(severityWarning, fix, fixAll) {
			// We cannot auto-generate a sensible email, so just show suggestion
			fmt.Printf("  %s[INFO]%s Cannot auto-fix - please set manually\n", colorInfoLabel, colorReset)
		}
		fmt.Printf("    %s[Suggested]%s Run: git config --global user.email \"you@example.com\"\n", colorInfoLabel, colorReset)
	}

	if issues == 0 {
		fmt.Printf("  %s[OK]%s Git user configuration valid\n", colorGreen, colorReset)
	}

	return issues, critical, fixed
}

// checkGitRemote checks if git remote is configured
func checkGitRemote(fix, fixAll bool, cmd *cobra.Command) (int, int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0, 0
	}
	configureLogger(cmd, cfg)

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return 0, 0, 0
	}

	if !git.IsGitInstalled() || !git.IsRepo(repoPath) {
		return 0, 0, 0
	}

	issues := 0
	critical := 0
	fixed := 0

	remoteURL, err := git.GetRemoteURL(repoPath)
	if err != nil {
		return 0, 0, 0
	}

	if remoteURL == "" {
		fmt.Printf("  %s[%s]%s No git remote configured\n", colorWarnLabel, severityWarning, colorReset)
		issues++

		if shouldFix(severityWarning, fix, fixAll) {
			// We cannot auto-generate a remote URL
			fmt.Printf("  %s[INFO]%s Cannot auto-fix - please set manually\n", colorInfoLabel, colorReset)
		}
		fmt.Printf("    %s[Suggested]%s Run: git remote add origin <url>\n", colorInfoLabel, colorReset)
		fmt.Printf("    %s[Suggested]%s Create a repository on GitHub/GitLab/Bitbucket first\n", colorInfoLabel, colorReset)
	} else {
		fmt.Printf("  %s[OK]%s Git remote configured: %s\n", colorGreen, colorReset, remoteURL)
	}

	return issues, critical, fixed
}

// checkHookPermissions verifies hooks are executable
func checkHookPermissions(fix, fixAll bool, cmd *cobra.Command) (int, int, int) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return 0, 0, 0
	}
	configureLogger(cmd, cfg)

	hooksDir, err := core.GetHooksDir(cfg)
	if err != nil {
		return 0, 0, 0
	}

	issues := 0
	critical := 0
	fixed := 0

	// Check if hooks directory exists
	if !fs.PathExists(hooksDir) {
		fmt.Println("  - No hooks directory")
		return 0, 0, 0
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
				fmt.Printf("  %s[%s]%s Cannot access hook: %s (%v)\n", colorWarnLabel, severityWarning, colorReset, hookName, err)
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
			fmt.Printf("  %s[%s]%s Hook not executable: %s\n", colorWarnLabel, severityWarning, colorReset, hookName)
			issues++

			if shouldFix(severityWarning, fix, fixAll) {
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

	return issues, critical, fixed
}
