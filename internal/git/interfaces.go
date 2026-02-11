package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GitCommander defines the interface for executing git commands
type GitCommander interface {
	Run(args ...string) error
	CombinedOutput(args ...string) ([]byte, error)
	Output(args ...string) ([]byte, error)
}

// ExecGitCommander wraps exec.Command for git operations
type ExecGitCommander struct{}

// Run executes a git command and discards output
func (c *ExecGitCommander) Run(args ...string) error {
	cmd := exec.Command("git", args...)
	return cmd.Run()
}

// CombinedOutput executes a git command and returns combined output
func (c *ExecGitCommander) CombinedOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.CombinedOutput()
}

// Output executes a git command and returns stdout output
func (c *ExecGitCommander) Output(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	return cmd.Output()
}

// GitService defines the interface for git operations
type GitService interface {
	InitRepo(repoPath string) error
	IsRepo(repoPath string) bool
	AutoCommit(repoPath, message string) error
	Commit(repoPath, message string) error
	Push(repoPath string) error
	Pull(repoPath string) error
	HasChanges(repoPath string) (bool, error)
	GetStatus(repoPath string) (StatusInfo, error)
	GetDiff(repoPath string) (string, error)
	GetFileHistory(repoPath, filePath string, n int) ([]CommitInfo, error)
	GetCurrentCommit(repoPath string) (string, error)
	GetRemoteURL(repoPath string) (string, error)
	IsGitInstalled() bool
}

// GitServiceImpl is the default implementation using ExecGitCommander
type GitServiceImpl struct {
	commander GitCommander
}

// NewGitService creates a new GitService with the default commander
func NewGitService() GitService {
	return &GitServiceImpl{
		commander: &ExecGitCommander{},
	}
}

// NewGitServiceWithCommander creates a new GitService with a custom commander
func NewGitServiceWithCommander(commander GitCommander) GitService {
	return &GitServiceImpl{
		commander: commander,
	}
}

// InitRepo initializes git repository in directory
func (s *GitServiceImpl) InitRepo(repoPath string) error {
	err := s.commander.Run("init")
	if err != nil {
		return &GitError{op: "init", err: err}
	}
	return nil
}

// IsRepo checks if directory is a git repository
func (s *GitServiceImpl) IsRepo(repoPath string) bool {
	err := s.commander.Run("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// AutoCommit stages all changes and commits with message
func (s *GitServiceImpl) AutoCommit(repoPath, message string) error {
	hasChanges, err := s.HasChanges(repoPath)
	if err != nil {
		return &GitError{op: "has-changes", err: err}
	}
	if !hasChanges {
		return nil
	}

	if _, err := s.commander.CombinedOutput("add", "-A"); err != nil {
		return &GitError{op: "add", err: err}
	}

	return s.Commit(repoPath, message)
}

// Commit creates a commit with the given message
func (s *GitServiceImpl) Commit(repoPath, message string) error {
	err := s.commander.Run("commit", "-m", message)
	if err != nil {
		return &GitError{op: "commit", err: err}
	}
	return nil
}

// Push pushes to the remote
func (s *GitServiceImpl) Push(repoPath string) error {
	err := s.commander.Run("push")
	if err != nil {
		return &GitError{op: "push", err: err}
	}
	return nil
}

// Pull pulls from the remote
func (s *GitServiceImpl) Pull(repoPath string) error {
	err := s.commander.Run("pull")
	if err != nil {
		return &GitError{op: "pull", err: err}
	}
	return nil
}

// HasChanges checks if there are uncommitted changes
func (s *GitServiceImpl) HasChanges(repoPath string) (bool, error) {
	output, err := s.commander.Output("status", "--porcelain")
	if err != nil {
		return false, &GitError{op: "status", err: err}
	}
	return len(output) > 0, nil
}

// GetStatus returns the git status
func (s *GitServiceImpl) GetStatus(repoPath string) (StatusInfo, error) {
	status := StatusInfo{}

	output, err := s.commander.Output("status", "--porcelain")
	if err != nil {
		return status, &GitError{op: "status", err: err}
	}
	status.HasUncommitted = len(output) > 0

	branch, err := s.commander.Output("rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		status.Branch = string(branch)
	}

	ahead, _ := s.commander.Output("rev-list", "--count", "--not", "origin/"+status.Branch)
	if err == nil {
		if n, _ := strconv.Atoi(string(ahead)); n > 0 {
			status.AheadBy = n
		}
	}

	return status, nil
}

// GetDiff returns the diff of uncommitted changes
func (s *GitServiceImpl) GetDiff(repoPath string) (string, error) {
	output, err := s.commander.Output("diff")
	if err != nil {
		return "", &GitError{op: "diff", err: err}
	}
	return string(output), nil
}

// GetFileHistory returns the commit history for a file
func (s *GitServiceImpl) GetFileHistory(repoPath, filePath string, n int) ([]CommitInfo, error) {
	output, err := s.commander.Output("log", "--oneline", "-n", string(rune('0'+n)), "--", filePath)
	if err != nil {
		return nil, &GitError{op: "log", err: err}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}

	var commits []CommitInfo
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 2 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0],
				Message: parts[1],
			})
		}
	}

	return commits, nil
}

// GetCurrentCommit returns the current commit hash
func (s *GitServiceImpl) GetCurrentCommit(repoPath string) (string, error) {
	output, err := s.commander.Output("rev-parse", "HEAD")
	if err != nil {
		return "", &GitError{op: "rev-parse", err: err}
	}
	return string(output), nil
}

// GetRemoteURL returns the remote URL for origin
func (s *GitServiceImpl) GetRemoteURL(repoPath string) (string, error) {
	output, err := s.commander.Output("remote", "get-url", "origin")
	if err != nil {
		return "", &GitError{op: "remote", err: err}
	}
	return string(output), nil
}

// IsGitInstalled checks if git command is available
func (s *GitServiceImpl) IsGitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// GitError represents an error from a git operation
type GitError struct {
	op  string
	err error
}

func (e *GitError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("git %s failed: %v", e.op, e.err)
	}
	return fmt.Sprintf("git %s failed", e.op)
}

func (e *GitError) Unwrap() error {
	return e.err
}

// MockGitService for testing without actual git
type MockGitService struct {
	Commits    []CommitInfo
	Errors     []error
	Status     StatusInfo
	Changes    bool
	CommitHash string
	RemoteURL  string
	RepoPath   string
}

// NewMockGitService creates a new mock git service
func NewMockGitService() *MockGitService {
	return &MockGitService{
		Commits:    []CommitInfo{},
		Errors:     []error{},
		Status:     StatusInfo{},
		Changes:    false,
		CommitHash: "mock123",
		RemoteURL:  "",
	}
}

// InitRepo initializes git repository (mock)
func (m *MockGitService) InitRepo(repoPath string) error {
	m.RepoPath = repoPath
	return m.popError()
}

// IsRepo checks if directory is a git repository (mock)
func (m *MockGitService) IsRepo(repoPath string) bool {
	return m.RepoPath == repoPath
}

// AutoCommit stages all changes and commits with message (mock)
func (m *MockGitService) AutoCommit(repoPath, message string) error {
	if m.Changes {
		m.Commits = append(m.Commits, CommitInfo{
			Hash:    m.CommitHash,
			Message: message,
		})
	}
	return m.popError()
}

// Commit creates a commit with the given message (mock)
func (m *MockGitService) Commit(repoPath, message string) error {
	m.Commits = append(m.Commits, CommitInfo{
		Hash:    m.CommitHash,
		Message: message,
	})
	return m.popError()
}

// Push pushes to the remote (mock)
func (m *MockGitService) Push(repoPath string) error {
	return m.popError()
}

// Pull pulls from the remote (mock)
func (m *MockGitService) Pull(repoPath string) error {
	return m.popError()
}

// HasChanges checks if there are uncommitted changes (mock)
func (m *MockGitService) HasChanges(repoPath string) (bool, error) {
	return m.Changes, m.popError()
}

// GetStatus returns the git status (mock)
func (m *MockGitService) GetStatus(repoPath string) (StatusInfo, error) {
	return m.Status, m.popError()
}

// GetDiff returns the diff of uncommitted changes (mock)
func (m *MockGitService) GetDiff(repoPath string) (string, error) {
	return "", m.popError()
}

// GetFileHistory returns the commit history for a file (mock)
func (m *MockGitService) GetFileHistory(repoPath, filePath string, n int) ([]CommitInfo, error) {
	return m.Commits, m.popError()
}

// GetCurrentCommit returns the current commit hash (mock)
func (m *MockGitService) GetCurrentCommit(repoPath string) (string, error) {
	return m.CommitHash, m.popError()
}

// GetRemoteURL returns the remote URL for origin (mock)
func (m *MockGitService) GetRemoteURL(repoPath string) (string, error) {
	return m.RemoteURL, m.popError()
}

// IsGitInstalled checks if git command is available (mock)
func (m *MockGitService) IsGitInstalled() bool {
	return true
}

// popError removes and returns the next error, or nil if no errors
func (m *MockGitService) popError() error {
	if len(m.Errors) == 0 {
		return nil
	}
	err := m.Errors[0]
	m.Errors = m.Errors[1:]
	return err
}
