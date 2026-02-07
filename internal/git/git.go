package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StatusInfo represents Git repository status
type StatusInfo struct {
	HasUncommitted bool
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
func IsRepo(repoPath string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = repoPath
	err := cmd.Run()
	return err == nil
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
				status.BehindBy, _ = strconv.Atoi(parts[0])
				status.AheadBy, _ = strconv.Atoi(parts[1])
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

		date, _ := time.Parse(time.RFC3339, parts[2])
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

	var files []string
	lines := strings.Split(string(output), "\n")
	// Match pattern: XY filename (where XY is status codes)
	re := regexp.MustCompile(`^.{2}\s+(.+)$`)

	for _, line := range lines {
		if line == "" {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) >= 2 {
			files = append(files, matches[1])
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
