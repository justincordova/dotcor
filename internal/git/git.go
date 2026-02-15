package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// StatusInfo represents Git repository status
type StatusInfo struct {
	HasUncommitted bool
	ChangedFiles   []string
	AheadBy        int
	BehindBy       int
	Branch         string
	RemoteExists   bool
}

// CommitInfo represents a single Git commit
type CommitInfo struct {
	Hash    string
	Author  string
	Date    time.Time
	Message string
}

// IsGitInstalled checks if git command is available
func IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// InitRepo initializes git repository in directory
func InitRepo(repoPath string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init failed: %s: %w", string(output), err)
	}
	return nil
}

// IsRepo checks if directory is a git repository
// Checks for .git directory directly to avoid walking up to parent repos
func IsRepo(repoPath string) bool {
	gitDir := repoPath + "/.git"
	info, err := os.Stat(gitDir)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// AutoCommit stages all changes and commits with message
// Returns nil if no changes to commit
func AutoCommit(repoPath, message string) error {
	// Check if there are changes
	hasChanges, err := HasChanges(repoPath)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !hasChanges {
		return nil // Nothing to commit
	}

	// Stage all changes
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s: %w", string(output), err)
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = repoPath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		// Check if it's "nothing to commit" error
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("git commit failed: %s: %w", string(output), err)
	}

	return nil
}

// AutoCommitFiles commits specific files or all changes
func AutoCommitFiles(repoPath string, files []string, message string) error {
	if !IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Stage specific files or all changes
	var cmd *exec.Cmd
	if len(files) > 0 {
		args := append([]string{"add"}, files...)
		cmd = exec.Command("git", args...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("staging files: %w", err)
		}
	} else {
		cmd = exec.Command("git", "add", "-A")
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("staging all changes: %w", err)
		}
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = repoPath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		// Check if it's "nothing to commit" error
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("committing: %s: %w", string(output), err)
	}

	return nil
}

// Sync commits all changes and pushes to remote (if configured)
func Sync(repoPath string) error {
	// Generate commit message with timestamp
	message := fmt.Sprintf("Sync dotfiles - %s", time.Now().Format("2006-01-02 15:04"))

	// Commit changes
	if err := AutoCommit(repoPath, message); err != nil {
		return err
	}

	// Check if remote exists
	remoteURL, err := GetRemoteURL(repoPath)
	if err != nil || remoteURL == "" {
		return nil // No remote configured, skip push
	}

	// Get current branch name
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = repoPath
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Check if upstream is configured for this branch
	upstreamCmd := exec.Command("git", "config", fmt.Sprintf("branch.%s.remote", branch))
	upstreamCmd.Dir = repoPath
	hasUpstream := upstreamCmd.Run() == nil

	// Push to remote, set upstream if not configured
	var pushCmd *exec.Cmd
	if hasUpstream {
		pushCmd = exec.Command("git", "push")
	} else {
		pushCmd = exec.Command("git", "push", "-u", "origin", branch)
	}
	pushCmd.Dir = repoPath
	if output, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %s: %w", string(output), err)
	}

	return nil
}

// PushWithProgress pushes to remote and shows progress
func PushWithProgress(repoPath string) error {
	// Get current branch name
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = repoPath
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Check if upstream is configured for this branch
	upstreamCmd := exec.Command("git", "config", fmt.Sprintf("branch.%s.remote", branch))
	upstreamCmd.Dir = repoPath
	hasUpstream := upstreamCmd.Run() == nil

	// Push with progress
	var pushCmd *exec.Cmd
	if hasUpstream {
		pushCmd = exec.Command("git", "push", "--progress")
	} else {
		pushCmd = exec.Command("git", "push", "-u", "origin", branch, "--progress")
	}
	pushCmd.Dir = repoPath
	pushCmd.Stdout = nil
	pushCmd.Stderr = nil
	return pushCmd.Run()
}

// HasChanges checks if working tree has uncommitted changes
func HasChanges(repoPath string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// SetRemote configures git remote
func SetRemote(repoPath, remoteName, remoteURL string) error {
	// Check if remote already exists
	existingURL, _ := GetRemoteURL(repoPath)
	if existingURL != "" {
		// Update existing remote
		cmd := exec.Command("git", "remote", "set-url", remoteName, remoteURL)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git remote set-url failed: %s: %w", string(output), err)
		}
	} else {
		// Add new remote
		cmd := exec.Command("git", "remote", "add", remoteName, remoteURL)
		cmd.Dir = repoPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git remote add failed: %s: %w", string(output), err)
		}
	}
	return nil
}

// GetRemoteURL returns configured remote URL, or empty if none
func GetRemoteURL(repoPath string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", nil // No remote configured
	}
	return strings.TrimSpace(string(output)), nil
}

// GetStatus returns git status information
func GetStatus(repoPath string) (StatusInfo, error) {
	status := StatusInfo{}

	// Get current branch
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = repoPath
	branchOutput, err := branchCmd.Output()
	if err == nil {
		status.Branch = strings.TrimSpace(string(branchOutput))
	}

	// Check for uncommitted changes
	hasChanges, err := HasChanges(repoPath)
	if err == nil {
		status.HasUncommitted = hasChanges
		if hasChanges {
			changedFiles, err := GetChangedFiles(repoPath)
			if err == nil {
				status.ChangedFiles = changedFiles
			}
		}
	}

	// Check if remote exists
	remoteURL, _ := GetRemoteURL(repoPath)
	status.RemoteExists = remoteURL != ""

	// Get ahead/behind counts if remote exists
	if status.RemoteExists && status.Branch != "" {
		aheadBehindCmd := exec.Command("git", "rev-list", "--left-right", "--count", fmt.Sprintf("origin/%s...HEAD", status.Branch))
		aheadBehindCmd.Dir = repoPath
		output, err := aheadBehindCmd.Output()
		if err == nil {
			parts := strings.Fields(string(output))
			if len(parts) >= 2 {
				var err error
				status.BehindBy, err = strconv.Atoi(parts[0])
				if err != nil {
					return status, fmt.Errorf("failed to parse behind count: %w", err)
				}
				status.AheadBy, err = strconv.Atoi(parts[1])
				if err != nil {
					return status, fmt.Errorf("failed to parse ahead count: %w", err)
				}
			}
		}
	}

	return status, nil
}

// GetFileHistory returns git log for specific file
func GetFileHistory(repoPath, filePath string, limit int) ([]CommitInfo, error) {
	if limit <= 0 {
		limit = 10
	}

	// Use format: hash|author|date|message
	format := "%H|%an|%aI|%s"
	cmd := exec.Command("git", "log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", format), "--", filePath)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []CommitInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		date, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			// Try other common formats
			date, err = time.Parse(time.RFC1123, parts[2])
			if err != nil {
				date, err = time.Parse("2006-01-02 15:04:05", parts[2])
				if err != nil {
					// If all fail, use zero time and log warning
					date = time.Time{}
				}
			}
		}
		commits = append(commits, CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    date,
			Message: parts[3],
		})
	}

	return commits, nil
}

// RestoreFile restores file from git history
func RestoreFile(repoPath, filePath, ref string) error {
	if ref == "" {
		ref = "HEAD"
	}

	cmd := exec.Command("git", "checkout", ref, "--", filePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %s: %w", string(output), err)
	}
	return nil
}

// GetDiff returns unified diff for uncommitted changes
func GetDiff(repoPath string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's just "no diff" situation
		if len(output) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(output), nil
}

// GetFileDiff returns diff for specific file
func GetFileDiff(repoPath, filePath string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD", "--", filePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(output), nil
}

// GetDiffStat returns diffstat (summary of changes)
func GetDiffStat(repoPath string) (string, error) {
	cmd := exec.Command("git", "diff", "HEAD", "--stat")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("git diff --stat failed: %w", err)
	}
	return string(output), nil
}

// Clone clones a repository to the specified path
func Clone(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s: %w", string(output), err)
	}
	return nil
}

// CloneWithProgress clones repository and shows progress
func CloneWithProgress(url, destPath string) error {
	if !IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Clone with progress
	cmd := exec.Command("git", "clone", "--progress", url, destPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// Pull pulls changes from remote
func Pull(repoPath string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %s: %w", string(output), err)
	}
	return nil
}

// GetCurrentCommit returns the current commit hash
func GetCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// GetChangedFiles returns list of changed files
func GetChangedFiles(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "DEBUG GIT: raw git status output: %s\n", string(output))

	var files []string
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Git status --porcelain format: XY filename
		// X and Y are single characters for staged and unstaged status
		// They can be spaces if no change
		if len(line) < 4 {
			continue
		}

		// Extract filename (everything after first 3 characters: X, Y, and space)
		// Examples: "M .zshrc" or "M  filename" or "MM filename"
		filename := strings.TrimSpace(line[3:])
		if filename != "" {
			files = append(files, filename)
		}
	}

	return files, nil
}

// StageFile stages a specific file
func StageFile(repoPath, filePath string) error {
	cmd := exec.Command("git", "add", filePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s: %w", string(output), err)
	}
	return nil
}

// UnstageFile unstages a specific file
func UnstageFile(repoPath, filePath string) error {
	cmd := exec.Command("git", "reset", "HEAD", "--", filePath)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %s: %w", string(output), err)
	}
	return nil
}

// GetConfig retrieves a git config value
func GetConfig(repoPath, key string) (string, error) {
	cmd := exec.Command("git", "config", key)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SetConfig sets a git config value
func SetConfig(repoPath, key, value string) error {
	cmd := exec.Command("git", "config", key, value)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
