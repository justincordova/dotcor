package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [files...]",
	Short: "Show status of managed dotfiles and repository",
	Long: `Show comprehensive status of your DotCor setup.

Displays:
- Symlink health for each managed file
- Git repository status (uncommitted changes, remote sync)
- Overall statistics

Examples:
  dotcor status                # Show full status
  dotcor status ~/.zshrc       # Show status for specific file
  dotcor status --quick        # Show summary only
  dotcor status --problems     # Show only files with issues`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolP("quick", "q", false, "Show summary only")
	statusCmd.Flags().Bool("problems", false, "Show only files with problems")
	statusCmd.Flags().Bool("json", false, "Output as JSON")
	statusCmd.Flags().Bool("prompt", false, "Output minimal for shell prompts")
}

func runStatus(cmd *cobra.Command, args []string) error {
	quick, _ := cmd.Flags().GetBool("quick")
	problemsOnly, _ := cmd.Flags().GetBool("problems")
	jsonFormat, _ := cmd.Flags().GetBool("json")
	prompt, _ := cmd.Flags().GetBool("prompt")

	// Check if initialized
	configPath, err := config.GetConfigPath()
	if err != nil {
		return fmt.Errorf("getting config path: %w", err)
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found\nRun 'dotcor init' first")
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	configureLogger(cmd, cfg)

	// Collect status
	status := collectStatus(cfg, args)

	// Output
	if prompt {
		return outputStatusPrompt(status, args)
	}

	if jsonFormat {
		return outputStatusJSON(status, cfg)
	}

	if quick {
		return outputStatusQuick(status)
	}

	err = outputStatusFull(status, problemsOnly, cfg)
	if err != nil {
		return err
	}

	// Show files with uncommitted adds separately
	uncommittedFiles := cfg.GetUncommittedFiles()
	if len(uncommittedFiles) > 0 {
		fmt.Println("")
		fmt.Println("Uncommitted Files:")
		for _, mf := range uncommittedFiles {
			fmt.Printf("  %s[!]%s %s\n", colorYellow, colorReset, mf.SourcePath)
		}
		fmt.Println("")
		fmt.Println("Run 'dotcor sync' to commit these changes.")
	}

	return nil
}

// StatusReport contains all status information
type StatusReport struct {
	Files      []FileStatus
	GitStatus  GitStatusInfo
	Statistics StatusStats
}

// FileStatus represents the status of a single managed file
type FileStatus struct {
	SourcePath string
	RepoPath   string
	Status     string
	Problem    string
}

// GitStatusInfo contains git-related status
type GitStatusInfo struct {
	IsRepo         bool
	HasUncommitted bool
	ChangedFiles   []string
	Branch         string
	AheadBy        int
	BehindBy       int
	RemoteExists   bool
}

// StatusStats contains summary statistics
type StatusStats struct {
	TotalFiles       int
	HealthyFiles     int
	ProblematicFiles int
}

// collectStatus gathers all status information
func collectStatus(cfg *config.Config, fileArgs []string) StatusReport {
	report := StatusReport{}

	// Get managed files, optionally filtered
	files := cfg.ManagedFiles
	if len(fileArgs) > 0 {
		var filtered []config.ManagedFile
		for _, arg := range fileArgs {
			mf, err := cfg.GetManagedFile(arg)
			if err == nil {
				filtered = append(filtered, *mf)
			} else {
				fmt.Fprintf(os.Stderr, "%s[!]%s %s is not managed\n", colorYellow, colorReset, arg)
			}
		}
		files = filtered
	}
	report.Statistics.TotalFiles = len(files)

	// Get git status first to have changed files available
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	var changedFiles []string
	if err == nil && git.IsGitInstalled() && git.IsRepo(repoPath) {
		gitStatus, _ := git.GetStatus(repoPath)
		report.GitStatus = GitStatusInfo{
			IsRepo:         true,
			HasUncommitted: gitStatus.HasUncommitted,
			ChangedFiles:   gitStatus.ChangedFiles,
			Branch:         gitStatus.Branch,
			AheadBy:        gitStatus.AheadBy,
			BehindBy:       gitStatus.BehindBy,
			RemoteExists:   gitStatus.RemoteExists,
		}
		changedFiles = gitStatus.ChangedFiles
	}

	// Check each file
	for _, f := range files {
		fs := CheckFileStatus(cfg, f, changedFiles)
		report.Files = append(report.Files, fs)

		if fs.Status == "ok" {
			report.Statistics.HealthyFiles++
		} else {
			report.Statistics.ProblematicFiles++
		}
	}

	return report
}

// CheckFileStatus checks the status of a single managed file
func CheckFileStatus(cfg *config.Config, mf config.ManagedFile, changedFiles []string) FileStatus {
	status := FileStatus{
		SourcePath: mf.SourcePath,
		RepoPath:   mf.RepoPath,
	}

	// Check if file has uncommitted changes
	for _, cf := range changedFiles {
		if cf == mf.RepoPath {
			status.Status = "modified"
			status.Problem = "uncommitted changes"
			return status
		}
	}

	// Expand paths
	sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
	if err != nil {
		status.Status = "error"
		status.Problem = "invalid source path"
		return status
	}

	repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
	if err != nil {
		status.Status = "error"
		status.Problem = "invalid repo path"
		return status
	}

	// Check if repo file exists
	if !fs.PathExists(repoPath) {
		status.Status = "missing-repo"
		status.Problem = "file missing from repository"
		return status
	}

	// Check if source path exists
	if !fs.PathExists(sourcePath) {
		status.Status = "missing-source"
		status.Problem = "symlink missing"
		return status
	}

	// Check if it's a symlink
	isLink, err := fs.IsSymlink(sourcePath)
	if err != nil {
		status.Status = "error"
		status.Problem = fmt.Sprintf("error checking symlink: %v", err)
		return status
	}

	if !isLink {
		status.Status = "not-symlink"
		status.Problem = "source is a regular file, not a symlink"
		return status
	}

	// Check if symlink is valid
	valid, err := fs.IsValidSymlink(sourcePath)
	if err != nil {
		status.Status = "error"
		status.Problem = fmt.Sprintf("error validating symlink: %v", err)
		return status
	}

	if !valid {
		status.Status = "broken"
		status.Problem = "symlink target does not exist"
		return status
	}

	// Check if symlink points to correct target
	target, err := fs.ReadSymlink(sourcePath)
	if err != nil {
		status.Status = "error"
		status.Problem = fmt.Sprintf("error reading symlink: %v", err)
		return status
	}

	// Get expected target
	expectedRel, _ := config.ComputeRelativeSymlink(sourcePath, repoPath)

	// Compare (allowing both relative and absolute)
	if target != expectedRel && target != repoPath {
		// Try resolving relative path
		resolvedTarget := resolvePath(getDir(sourcePath), target)
		if resolvedTarget != repoPath {
			status.Status = "wrong-target"
			status.Problem = fmt.Sprintf("points to %s instead of repo file", target)
			return status
		}
	}

	status.Status = "ok"
	return status
}

// outputStatusFull outputs detailed status
func outputStatusFull(status StatusReport, problemsOnly bool, cfg *config.Config) error {
	// Header
	fmt.Printf("\n  %sDotCor Status%s\n", colorLightPink, colorReset)
	fmt.Println("")

	// Files section
	if len(status.Files) > 0 {
		fmt.Printf("  %sManaged Files:%s\n", colorLightPink, colorReset)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

		hasProblems := false
		for _, f := range status.Files {
			if problemsOnly && f.Status == "ok" {
				continue
			}

			icon := getStatusIcon(f.Status)
			if f.Status == "ok" {
				fmt.Fprintf(w, "  %s %s\tok\n", icon, f.SourcePath)
			} else if f.Status == "modified" {
				fmt.Fprintf(w, "  %s %s\t~\n", icon, f.SourcePath)
			} else {
				fmt.Fprintf(w, "  %s %s\t%s\n", icon, f.SourcePath, f.Problem)
				hasProblems = true
			}
		}

		w.Flush()

		if problemsOnly && !hasProblems {
			fmt.Println("  All files are healthy!")
		}

		fmt.Println("")
	}

	// Git section
	if status.GitStatus.IsRepo {
		fmt.Printf("  %sGit Repository:%s\n", colorLightPink, colorReset)

		if status.GitStatus.Branch != "" {
			fmt.Printf("  Branch: %s\n", status.GitStatus.Branch)
		}

		if status.GitStatus.HasUncommitted {
			fmt.Printf("  %s[!]%s Uncommitted changes\n", colorYellow, colorReset)
			if len(status.GitStatus.ChangedFiles) > 0 {
				for _, changedFile := range status.GitStatus.ChangedFiles {
					sourcePath := mapRepoToSourcePath(changedFile, cfg)
					if sourcePath != "" {
						fmt.Printf("    - %s\n", sourcePath)
					} else {
						fmt.Printf("    - %s\n", changedFile)
					}
				}
			} else {
				repoPath, _ := config.ExpandPath(cfg.RepoPath, cfg)
				if git.IsGitInstalled() && git.IsRepo(repoPath) {
					changedFiles, err := git.GetChangedFiles(repoPath)
					if err == nil && len(changedFiles) > 0 {
						for _, changedFile := range changedFiles {
							sourcePath := mapRepoToSourcePath(changedFile, cfg)
							if sourcePath != "" {
								fmt.Printf("    - %s\n", sourcePath)
							} else {
								fmt.Printf("    - %s\n", changedFile)
							}
						}
					}
				}
			}
		} else {
			fmt.Printf("  %s[OK]%s Working tree clean\n", colorGreen, colorReset)
		}

		if status.GitStatus.RemoteExists {
			if status.GitStatus.AheadBy > 0 {
				fmt.Printf("  ↑ %d commit(s) ahead of remote\n", status.GitStatus.AheadBy)
			}
			if status.GitStatus.BehindBy > 0 {
				fmt.Printf("  ↓ %d commit(s) behind remote\n", status.GitStatus.BehindBy)
			}
			if status.GitStatus.AheadBy == 0 && status.GitStatus.BehindBy == 0 && !status.GitStatus.HasUncommitted {
				fmt.Printf("  %s[OK]%s In sync with remote\n", colorGreen, colorReset)
			}
		} else {
			fmt.Println("  - No remote configured")
		}

		fmt.Println("")
	}

	// Summary
	fmt.Printf("  %sSummary:%s %d files managed", colorLightPink, colorReset, status.Statistics.TotalFiles)
	if status.Statistics.ProblematicFiles > 0 {
		fmt.Printf(", %d with issues", status.Statistics.ProblematicFiles)
	}
	fmt.Println("")

	// Suggestions
	if status.Statistics.ProblematicFiles > 0 {
		fmt.Println("")
		fmt.Println("Run 'dotcor doctor' for detailed diagnostics and repair suggestions.")
	}

	return nil
}

// outputStatusQuick outputs summary only
func outputStatusQuick(status StatusReport) error {
	// One-line summary
	if status.Statistics.ProblematicFiles == 0 {
		fmt.Printf("%s[OK]%s %d files managed, all healthy\n", colorGreen, colorReset, status.Statistics.TotalFiles)
	} else {
		fmt.Printf("%s[!]%s %d files managed, %d with issues\n",
			colorYellow, colorReset, status.Statistics.TotalFiles, status.Statistics.ProblematicFiles)
	}

	if status.GitStatus.IsRepo && status.GitStatus.HasUncommitted {
		fmt.Printf("%s[!]%s Uncommitted changes in repository\n", colorYellow, colorReset)
	}

	return nil
}

// outputStatusPrompt outputs minimal for shell prompts
func outputStatusPrompt(status StatusReport, args []string) error {
	files := status.Files
	if len(args) > 0 {
		var filtered []FileStatus
		for _, f := range files {
			for _, arg := range args {
				if f.SourcePath == arg {
					filtered = append(filtered, f)
					break
				}
			}
		}
		files = filtered
	}

	syncedCount := 0
	brokenSymlinks := 0
	missingFiles := 0
	modifiedFiles := 0
	otherIssues := 0

	for _, f := range files {
		switch f.Status {
		case "ok":
			syncedCount++
		case "broken":
			brokenSymlinks++
		case "missing-source", "missing-repo":
			missingFiles++
		case "modified":
			modifiedFiles++
		default:
			otherIssues++
		}
	}

	if issuesCount := brokenSymlinks + missingFiles + modifiedFiles + otherIssues; issuesCount == 0 {
		fmt.Printf("✓ %d synced\n", syncedCount)
	} else {
		var issueDesc string
		var suggestion string

		if brokenSymlinks > 0 {
			issueDesc = fmt.Sprintf("%d broken symlink", brokenSymlinks)
			if brokenSymlinks > 1 {
				issueDesc += "s"
			}
			suggestion = "run 'dotcor doctor --fix'"
		} else if missingFiles > 0 {
			issueDesc = fmt.Sprintf("%d missing file", missingFiles)
			if missingFiles > 1 {
				issueDesc += "s"
			}
			suggestion = "run 'dotcor doctor'"
		} else if modifiedFiles > 0 {
			issueDesc = fmt.Sprintf("%d uncommitted", modifiedFiles)
			suggestion = "run 'dotcor sync'"
		} else if status.GitStatus.BehindBy > 0 {
			issueDesc = fmt.Sprintf("%d commit(s) behind", status.GitStatus.BehindBy)
			suggestion = "run 'dotcor sync'"
		} else {
			issueDesc = fmt.Sprintf("%d issue", otherIssues)
			if otherIssues > 1 {
				issueDesc += "s"
			}
			suggestion = "run 'dotcor doctor'"
		}

		fmt.Printf("✓ %d synced, ⚠ %s (%s)\n", syncedCount, issueDesc, suggestion)
	}

	return nil
}

// statusJSONOutput represents the JSON structure for status output
type statusJSONOutput struct {
	TotalFiles       int              `json:"total_files"`
	HealthyFiles     int              `json:"healthy_files"`
	ProblematicFiles int              `json:"problematic_files"`
	BackupCount      int              `json:"backup_count"`
	LockStatus       string           `json:"lock_status"`
	ConfigIssues     []string         `json:"config_issues"`
	Git              *gitJSONOutput   `json:"git,omitempty"`
	Files            []fileJSONOutput `json:"files"`
}

type gitJSONOutput struct {
	Branch       string `json:"branch"`
	Uncommitted  bool   `json:"uncommitted"`
	Ahead        int    `json:"ahead"`
	Behind       int    `json:"behind"`
	RemoteExists bool   `json:"remote_exists"`
}

type fileJSONOutput struct {
	Source  string `json:"source"`
	Status  string `json:"status"`
	Problem string `json:"problem"`
}

// outputStatusJSON outputs status as JSON
func outputStatusJSON(status StatusReport, cfg *config.Config) error {
	backupCount, _ := core.GetBackupCount(cfg)
	lockStatus := getLockStatusString()
	configIssues := collectConfigIssues(cfg)

	output := statusJSONOutput{
		TotalFiles:       status.Statistics.TotalFiles,
		HealthyFiles:     status.Statistics.HealthyFiles,
		ProblematicFiles: status.Statistics.ProblematicFiles,
		BackupCount:      backupCount,
		LockStatus:       lockStatus,
		ConfigIssues:     configIssues,
		Files:            make([]fileJSONOutput, 0, len(status.Files)),
	}

	if status.GitStatus.IsRepo {
		output.Git = &gitJSONOutput{
			Branch:       status.GitStatus.Branch,
			Uncommitted:  status.GitStatus.HasUncommitted,
			Ahead:        status.GitStatus.AheadBy,
			Behind:       status.GitStatus.BehindBy,
			RemoteExists: status.GitStatus.RemoteExists,
		}
	}

	for _, f := range status.Files {
		problem := f.Problem
		if problem == "" {
			problem = "none"
		}
		output.Files = append(output.Files, fileJSONOutput{
			Source:  f.SourcePath,
			Status:  f.Status,
			Problem: problem,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// getStatusIcon returns an icon for the given status
func getStatusIcon(status string) string {
	switch status {
	case "ok":
		return colorGreen + "[OK]" + colorReset
	case "modified":
		return colorYellow + "[!]" + colorReset
	case "missing-repo", "missing-source", "broken", "not-symlink", "wrong-target":
		return colorRed + "[X]" + colorReset
	default:
		return "?"
	}
}

// CheckLockStatus checks if there's a stale lock (used by doctor)
func CheckLockStatus() (bool, *core.LockInfo, error) {
	info, err := core.GetLockInfo()
	if err != nil {
		return false, nil, err
	}

	if info == nil {
		return false, nil, nil // No lock
	}

	// Check if it's our own lock
	if info.PID == os.Getpid() {
		return false, info, nil // Our own lock
	}

	// Check if it's stale
	lockPath, _ := getLockPathForCheck()
	if lockPath != "" {
		cfg, err := config.LoadConfig()
		if err == nil {
			stale, _ := core.IsStale(lockPath, cfg)
			if stale {
				return true, info, nil // Stale lock
			}
		}
	}

	return false, info, nil // Active lock from another process
}

// getLockPathForCheck returns lock path for checking (internal use)
func getLockPathForCheck() (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return configDir + "/.lock", nil
}

// getLockStatusString returns the current lock status as a string
func getLockStatusString() string {
	info, err := core.GetLockInfo()
	if err != nil {
		return "error"
	}

	if info == nil {
		return "none"
	}

	// Check if it's our own lock
	if info.PID == os.Getpid() {
		return "none"
	}

	// Check if it's stale
	lockPath, _ := getLockPathForCheck()
	if lockPath != "" {
		cfg, err := config.LoadConfig()
		if err == nil {
			stale, _ := core.IsStale(lockPath, cfg)
			if stale {
				return "stale"
			}
		}
	}

	return "active"
}

// collectConfigIssues returns a list of configuration issues
func collectConfigIssues(cfg *config.Config) []string {
	issues := []string{}

	if len(cfg.ManagedFiles) == 0 {
		issues = append(issues, "no managed files")
	}

	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		issues = append(issues, "invalid repo path")
	} else if !fs.PathExists(repoPath) {
		issues = append(issues, "repo path does not exist")
	}

	if cfg.GitEnabled {
		if !git.IsGitInstalled() {
			issues = append(issues, "git enabled but git not installed")
		} else if !git.IsRepo(repoPath) {
			issues = append(issues, "git enabled but repo not initialized")
		}
	}

	for _, mf := range cfg.ManagedFiles {
		_, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			issues = append(issues, fmt.Sprintf("invalid source path: %s", mf.SourcePath))
			continue
		}
		repoFilePath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			issues = append(issues, fmt.Sprintf("invalid repo path: %s", mf.RepoPath))
		}
		if !fs.PathExists(repoFilePath) {
			issues = append(issues, fmt.Sprintf("repo file missing: %s", mf.SourcePath))
		}
	}

	return issues
}

// getDir returns the directory part of a path
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// mapRepoToSourcePath maps a repo file path to its corresponding source path
func mapRepoToSourcePath(repoFilePath string, cfg *config.Config) string {
	for _, mf := range cfg.ManagedFiles {
		if mf.RepoPath == repoFilePath {
			return mf.SourcePath
		}
	}
	return ""
}

// resolvePath resolves a potentially relative path against a base directory
func resolvePath(baseDir, path string) string {
	if len(path) > 0 && (path[0] == '/' || (len(path) > 1 && path[1] == ':')) {
		return path
	}
	// Simple path resolution - join and clean
	result := baseDir + "/" + path
	// Clean up .. references
	parts := strings.Split(result, "/")
	var clean []string
	for _, part := range parts {
		if part == ".." && len(clean) > 0 {
			clean = clean[:len(clean)-1]
		} else if part != "." && part != "" {
			clean = append(clean, part)
		}
	}
	return "/" + strings.Join(clean, "/")
}
