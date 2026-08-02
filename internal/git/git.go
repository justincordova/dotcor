package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// gitCommandTimeout bounds local git commands (status, diff, log, etc.).
	// Local git should never take this long on a healthy repo.
	gitCommandTimeout = 30 * time.Second

	// gitNetworkTimeout bounds network-bound git commands (push, pull, clone,
	// fetch). Slow connections and large repos justify a longer ceiling, but a
	// bound is essential — without one, a dead remote or auth prompt wedges the TUI.
	gitNetworkTimeout = 5 * time.Minute

	// gitWaitDelay bounds how long Wait will block on a grandchild that
	// inherited the command's output pipes after the command itself has
	// finished or been killed.
	gitWaitDelay = 5 * time.Second
)

// ValidateRemoteURL rejects URLs that would let git execute arbitrary code
// or read local files via shell-style transports. It accepts the common
// safe transports: https, ssh, git, http (intranet only), and the
// scp-style user@host:path form widely used by GitHub/GitLab.
//
// Rejected explicitly:
//   - ext::          arbitrary subprocess transport (RCE)
//   - file://        local fs reads, not appropriate for a sync remote
//   - any value starting with `-`  (would be parsed as a git flag)
//
// Empty URL is allowed — callers use that to mean "no remote".
func ValidateRemoteURL(remoteURL string) error {
	if remoteURL == "" {
		return nil
	}
	if strings.HasPrefix(remoteURL, "-") {
		return fmt.Errorf("URL cannot start with '-' (looks like a flag): %q", remoteURL)
	}

	// Hard rejections — these transports can read local files or run code.
	dangerousPrefixes := []string{
		"ext::",
		"file://",
		"file:",
	}
	lower := strings.ToLower(remoteURL)
	for _, p := range dangerousPrefixes {
		if strings.HasPrefix(lower, p) {
			return fmt.Errorf("transport %q is not allowed", strings.TrimSuffix(p, "://"))
		}
	}

	// Allowlist: explicit scheme (https/http/ssh/git) or scp-style.
	allowedSchemes := []string{"https://", "http://", "ssh://", "git://"}
	for _, s := range allowedSchemes {
		if strings.HasPrefix(lower, s) {
			return nil
		}
	}

	// scp-style: user@host:path. Must contain '@' before ':' and ':' before
	// any '/'. Reject if the host part is empty or starts with '-'.
	if at := strings.Index(remoteURL, "@"); at > 0 {
		rest := remoteURL[at+1:]
		colon := strings.Index(rest, ":")
		slash := strings.Index(rest, "/")
		if colon > 0 && (slash == -1 || colon < slash) {
			host := rest[:colon]
			if host == "" || strings.HasPrefix(host, "-") {
				return fmt.Errorf("invalid host in scp-style URL: %q", remoteURL)
			}
			return nil
		}
	}

	return fmt.Errorf("URL must use https://, ssh://, git://, or user@host:path form: %q", remoteURL)
}

// runGitCommand executes a git command with the default local timeout.
// Caller must defer the returned cancel to release the context.
func runGitCommand(dir string, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	return runGitCommandWithTimeout(dir, gitCommandTimeout, name, args...)
}

// runGitNetworkCommand executes a git command with the longer network timeout.
// Use for push/pull/clone/fetch. Caller must defer the returned cancel.
func runGitNetworkCommand(dir string, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	cmd, cancel := runGitCommandWithTimeout(dir, gitNetworkTimeout, name, args...)

	// CommandContext kills only the direct `git` process on timeout. A
	// grandchild — ssh, git-remote-https, a credential helper — inherits the
	// stdout/stderr pipe write ends, so Wait would block past the deadline
	// waiting for EOF and the TUI's push would never resolve. WaitDelay
	// bounds that: the pipes are force-closed and the process killed.
	//
	// Only network commands get this. Wait returns ErrWaitDelay INSTEAD OF
	// nil when a successful command's pipes had to be force-closed, so
	// applying it to every local command risked reporting a landed commit as
	// a failure. Network commands are where a wedged grandchild is the real
	// hazard, and their call sites tolerate the delay via isBenignWaitDelay.
	cmd.WaitDelay = gitWaitDelay
	cmd.Env = append(cmd.Env, sshEnvFor(dir)...)
	return cmd, cancel
}

// isBenignWaitDelay reports whether err is only the WaitDelay bookkeeping
// error, which os/exec returns in place of nil when a command exited
// successfully but its pipes had to be force-closed.
func isBenignWaitDelay(err error) bool {
	return errors.Is(err, exec.ErrWaitDelay)
}

func runGitCommandWithTimeout(dir string, timeout time.Duration, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, "git", append([]string{name}, args...)...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	return cmd, cancel
}

// gitEnv returns the environment every git invocation runs with.
//
// Two problems it solves:
//
//   - Interactive prompts. The TUI owns the terminal under the alt-screen.
//     A git subprocess that prompts for credentials writes onto that screen
//     and then blocks for the whole timeout, wedging the UI with a corrupted
//     display. GIT_TERMINAL_PROMPT/GIT_ASKPASS stop git's own prompts;
//     BatchMode is the only thing that reliably stops OpenSSH, which opens
//     /dev/tty directly for passphrase and host-key prompts and ignores both
//     stdin and SSH_ASKPASS when a tty is available.
//
//   - Translated messages. Several code paths branch on git's human-readable
//     output ("nothing to commit"). Those strings are gettext-translated, so
//     on a localised system the branch silently stops matching. LC_ALL=C
//     pins them.
//
// The askpass helpers are `true`, not `echo`. Git runs $GIT_ASKPASS with the
// prompt as its argument and reads STDOUT as the answer, so `echo` submits the
// prompt text itself as the username or password — a nonsensical credential
// sent to the server, and repeated attempts count toward provider rate limits.
// `true` produces no output, which fails cleanly.
//
// GIT_SSH_COMMAND is deliberately NOT set here; see sshEnvFor.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=true",
		"SSH_ASKPASS=true",
		"SSH_ASKPASS_REQUIRE=never",
		"LC_ALL=C",
		"LANGUAGE=",
	)
}

// sshEnvFor returns the GIT_SSH_COMMAND entries a network command needs, or
// nil when the caller's own configuration should be left alone.
//
// Only network commands invoke ssh, so only they need this. Checking
// $GIT_SSH_COMMAND alone was not enough: git also reads core.sshCommand from
// its config, and the environment variable takes precedence over it — so a
// user who configured their key with
// `git config core.sshCommand "ssh -i ~/.ssh/dotfiles_key"` had it silently
// replaced by a bare ssh, and BatchMode then suppressed the very prompt that
// would have explained the resulting "Permission denied (publickey)".
//
// When either is configured we append BatchMode to the user's own command
// rather than replacing it, so their identity settings survive while the TUI
// is still protected from a /dev/tty passphrase prompt.
func sshEnvFor(dir string) []string {
	if existing := os.Getenv("GIT_SSH_COMMAND"); existing != "" {
		return []string{"GIT_SSH_COMMAND=" + existing + " -o BatchMode=yes"}
	}
	if configured := sshCommandFromConfig(dir); configured != "" {
		return []string{"GIT_SSH_COMMAND=" + configured + " -o BatchMode=yes"}
	}
	return []string{"GIT_SSH_COMMAND=ssh -o BatchMode=yes"}
}

// sshCommandFromConfig reads core.sshCommand, returning "" when unset. It is
// a local config read, so it stays well inside the command timeout.
func sshCommandFromConfig(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--get", "core.sshCommand")
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// StatusInfo represents Git repository status
type StatusInfo struct {
	HasUncommitted bool
	ChangedFiles   []string
	AheadBy        int
	BehindBy       int
	Branch         string
	RemoteExists   bool
	// Detached is true when HEAD is detached (rebase, bisect, or explicit
	// checkout of a commit). Branch holds the short SHA in that case.
	Detached bool
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

// InitRepo initializes git repository in directory.
//
// A minimal .gitignore is written so transient artifacts from interrupted
// operations (`*.dotcor-tmp`, `*.dotcor-restore`) and local-only state
// (`logs/`, `backups/`) never reach the remote. The ignore is appended
// idempotently — if the user already has a .gitignore, missing patterns
// are added and existing ones are left alone.
func InitRepo(repoPath string) error {
	cmd, cancel := runGitCommand(repoPath, "init")
	defer cancel()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git init failed: %s: %w", RedactURLCredentials(string(output)), err)
	}

	if err := EnsureIgnorePatterns(repoPath); err != nil {
		// Non-fatal — the init succeeded, and returning an error here would
		// roll back an already-created repo. But it must not be silent: the
		// patterns keep backups/ out of the commit, and that directory holds
		// verbatim copies of files such as ~/.ssh and ~/.gnupg material.
		slog.Default().Warn("could not write dotcor .gitignore entries",
			"repo", repoPath, "err", err)
	}
	return nil
}

// EnsureIgnorePatterns adds dotcor's required .gitignore entries to an
// existing repository.
//
// Exported so it can be applied on startup, not only at init: a repository
// created before these patterns existed would otherwise keep sweeping
// backups/ into every commit forever.
func EnsureIgnorePatterns(repoPath string) error {
	return ensureDotcorGitignore(repoPath)
}

// ensureDotcorGitignore appends dotcor-specific patterns to .gitignore.
// Patterns that already appear (line-wise match) are not duplicated.
func ensureDotcorGitignore(repoPath string) error {
	required := []string{
		"# dotcor",
		"*.dotcor-tmp",
		"*.dotcor-restore",
		"logs/",
		"backups/",
	}

	path := filepath.Join(repoPath, ".gitignore")
	existing, _ := os.ReadFile(path) // missing file is fine

	have := make(map[string]bool)
	for _, line := range strings.Split(string(existing), "\n") {
		have[strings.TrimSpace(line)] = true
	}

	var toAdd []string
	for _, pat := range required {
		if !have[pat] {
			toAdd = append(toAdd, pat)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	var buf strings.Builder
	buf.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		buf.WriteString("\n")
	}
	if len(existing) > 0 {
		buf.WriteString("\n")
	}
	for _, pat := range toAdd {
		buf.WriteString(pat)
		buf.WriteString("\n")
	}
	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// IsRepo checks if directory is a git repository.
// Checks for a .git entry directly under repoPath so we don't walk up
// into a parent repository. The entry is normally a directory but is a
// regular file in linked worktrees (`gitdir: …`) — both forms count.
func IsRepo(repoPath string) bool {
	gitPath := filepath.Join(repoPath, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// isNothingToCommitError checks if git output indicates no changes
func isNothingToCommitError(output string) bool {
	return strings.Contains(output, "nothing to commit") ||
		strings.Contains(output, "nothing added to commit")
}

// AutoCommit stages all changes and commits with message
// Returns nil if no changes to commit
func AutoCommit(repoPath, message string, logger *slog.Logger) error {
	// Check if there are changes
	hasChanges, err := HasChanges(repoPath)
	if err != nil {
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !hasChanges {
		return nil // Nothing to commit
	}

	// Stage all changes
	addCmd, cancelAdd := runGitCommand(repoPath, "add", "-A")
	defer cancelAdd()
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s: %w", RedactURLCredentials(string(output)), err)
	}

	// Commit
	commitCmd, cancelCommit := runGitCommand(repoPath, "commit", "-m", message)
	defer cancelCommit()
	if output, err := commitCmd.CombinedOutput(); err != nil {
		if isNothingToCommitError(string(output)) {
			if logger != nil {
				logger.Debug("no changes to commit")
			}
			return nil
		}
		return fmt.Errorf("git commit failed: %s: %w", RedactURLCredentials(string(output)), err)
	}

	return nil
}

// AutoCommitFiles commits specific files or all changes
func AutoCommitFiles(repoPath string, files []string, message string) error {
	if !IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Stage specific files or all changes
	if len(files) > 0 {
		addCmd, cancel := runGitCommand(repoPath, "add", files...)
		defer cancel()
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("staging files: %w", err)
		}
	} else {
		addCmd, cancel := runGitCommand(repoPath, "add", "-A")
		defer cancel()
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("staging all changes: %w", err)
		}
	}

	// Commit
	commitCmd, cancel := runGitCommand(repoPath, "commit", "-m", message)
	defer cancel()
	if output, err := commitCmd.CombinedOutput(); err != nil {
		if isNothingToCommitError(string(output)) {
			return nil
		}
		return fmt.Errorf("committing: %s: %w", RedactURLCredentials(string(output)), err)
	}

	return nil
}

type SyncResult struct {
	Committed    bool
	FilesChanged int
	Pushed       bool
	Branch       string
}

func SyncDetailed(repoPath string, logger *slog.Logger) (SyncResult, error) {
	result := SyncResult{}

	status, err := GetStatus(repoPath)
	if err != nil {
		return result, err
	}
	result.Branch = status.Branch

	if status.HasUncommitted {
		message := fmt.Sprintf("Sync dotfiles - %s", time.Now().Format("2006-01-02 15:04"))
		if err := AutoCommit(repoPath, message, logger); err != nil {
			return result, err
		}
		result.Committed = true
		result.FilesChanged = len(status.ChangedFiles)
	}

	// Propagate the error rather than discarding it. GetRemoteURL now
	// distinguishes "no remote configured" (exit 2) from a real failure, but
	// swallowing the error here reinstates the silent no-op it was fixed for:
	// a timed-out lookup would skip the push and still report success.
	remoteURL, err := GetRemoteURL(repoPath)
	if err != nil {
		return result, fmt.Errorf("checking remote: %w", err)
	}
	if remoteURL == "" {
		return result, nil
	}

	branch := status.Branch
	if branch == "" {
		return result, nil
	}

	upstreamCheck, cancelUpstream := runGitCommand(repoPath, "config", fmt.Sprintf("branch.%s.remote", branch))
	hasUpstream := upstreamCheck.Run() == nil
	cancelUpstream()

	var pushCmd *exec.Cmd
	var cancelPush context.CancelFunc
	if hasUpstream {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push")
	} else {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push", "-u", "origin", branch)
	}
	defer cancelPush()
	if output, err := pushCmd.CombinedOutput(); err != nil && !isBenignWaitDelay(err) {
		return result, fmt.Errorf("git push failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	result.Pushed = true
	return result, nil
}

// Sync commits all changes and pushes to remote (if configured)
func Sync(repoPath string, logger *slog.Logger) error {
	// Generate commit message with timestamp
	message := fmt.Sprintf("Sync dotfiles - %s", time.Now().Format("2006-01-02 15:04"))

	// Commit changes
	if err := AutoCommit(repoPath, message, logger); err != nil {
		return err
	}

	// Only an absent remote is a reason to skip the push. Treating every
	// failure as "no remote" made Sync a silent no-op that still reported
	// success while nothing had been pushed.
	remoteURL, err := GetRemoteURL(repoPath)
	if err != nil {
		return fmt.Errorf("checking remote: %w", err)
	}
	if remoteURL == "" {
		return nil // No remote configured, skip push
	}

	// Get current branch name
	branchCmd, cancelBranch := runGitCommand(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := branchCmd.Output()
	cancelBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Check if upstream is configured for this branch
	upstreamCmd, cancelUpstream := runGitCommand(repoPath, "config", fmt.Sprintf("branch.%s.remote", branch))
	hasUpstream := upstreamCmd.Run() == nil
	cancelUpstream()

	// Push to remote, set upstream if not configured
	var pushCmd *exec.Cmd
	var cancelPush context.CancelFunc
	if hasUpstream {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push")
	} else {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push", "-u", "origin", branch)
	}
	defer cancelPush()
	if output, err := pushCmd.CombinedOutput(); err != nil && !isBenignWaitDelay(err) {
		return fmt.Errorf("git push failed: %s: %w", RedactURLCredentials(string(output)), err)
	}

	return nil
}

// PushWithProgress pushes to remote and shows progress
func PushWithProgress(repoPath string) error {
	// Get current branch name
	branchCmd, cancelBranch := runGitCommand(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := branchCmd.Output()
	cancelBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}
	branch := strings.TrimSpace(string(branchOutput))

	// Check if upstream is configured for this branch
	upstreamCmd, cancelUpstream := runGitCommand(repoPath, "config", fmt.Sprintf("branch.%s.remote", branch))
	hasUpstream := upstreamCmd.Run() == nil
	cancelUpstream()

	// Push with progress
	var pushCmd *exec.Cmd
	var cancelPush context.CancelFunc
	if hasUpstream {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push", "--progress")
	} else {
		pushCmd, cancelPush = runGitNetworkCommand(repoPath, "push", "-u", "origin", branch, "--progress")
	}
	defer cancelPush()
	// Prompt suppression is applied to every git invocation by gitEnv().
	// Capture stderr rather than discarding it: without it the caller —
	// and therefore the TUI footer — only ever sees "exit status 1", so a
	// rejected non-fast-forward push looks identical to a missing remote.
	var stderr bytes.Buffer
	pushCmd.Stdout = nil
	pushCmd.Stderr = &stderr
	if err := pushCmd.Run(); err != nil && !isBenignWaitDelay(err) {
		return wrapGitError("git push", stderr.String(), err)
	}
	return nil
}

// wrapGitError builds an error carrying git's own diagnostics.
//
// Any credentials embedded in a remote URL are redacted first:
// ValidateRemoteURL deliberately permits https://user:token@host, so git's
// output can legitimately contain a secret that must not reach the UI or
// the log file.
func wrapGitError(what, stderr string, err error) error {
	msg := strings.TrimSpace(RedactURLCredentials(stderr))
	if msg == "" {
		return fmt.Errorf("%s failed: %w", what, err)
	}
	return fmt.Errorf("%s failed: %s: %w", what, msg, err)
}

// credentialInURL matches the userinfo section of a URL: scheme://user:secret@
// Group 1 is the scheme, group 2 is the username.
var credentialInURL = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^:/@\s]*):[^/@\s]*@`)

// RedactURLCredentials strips passwords and tokens from any URL in s so git
// output can be shown to the user or written to the log safely.
func RedactURLCredentials(s string) string {
	return credentialInURL.ReplaceAllString(s, "${1}***:***@")
}

// StripURLPassword removes the password/token from a URL's userinfo, keeping
// the username. It is a no-op for URLs that carry no password.
//
// This is what gets persisted to .dotcorrc. That file lives inside the
// repository and is picked up by `git add -A`, so a remote entered as
// https://user:ghp_token@github.com/... would otherwise be committed and
// pushed — publishing the token in the repository's history. The operative
// copy of the URL lives in .git/config, which is never staged, so stripping
// the secret here costs nothing operationally.
func StripURLPassword(remoteURL string) string {
	return credentialInURL.ReplaceAllString(remoteURL, "${1}${2}@")
}

// HasChanges checks if working tree has uncommitted changes
func HasChanges(repoPath string) (bool, error) {
	cmd, cancel := runGitCommand(repoPath, "status", "--porcelain")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status failed: %w", err)
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// SetRemote configures git remote.
//
// remoteURL is validated against an allowlist of safe transports
// (https, ssh, git, scp-style user@host:path). Git accepts the
// `ext::sh -c '...'` transport which executes arbitrary commands on
// fetch/push — refusing it here prevents a poisoned `.dotcorrc`
// (which is itself often synced) from becoming RCE on next sync.
func SetRemote(repoPath, remoteName, remoteURL string) error {
	if err := ValidateRemoteURL(remoteURL); err != nil {
		return fmt.Errorf("invalid remote URL: %w", err)
	}
	// Check if remote already exists
	existingURL, _ := GetRemoteURL(repoPath)
	if existingURL != "" {
		// Update existing remote
		cmd, cancel := runGitCommand(repoPath, "remote", "set-url", remoteName, remoteURL)
		defer cancel()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git remote set-url failed: %s: %w", RedactURLCredentials(string(output)), err)
		}
	} else {
		// Add new remote
		cmd, cancel := runGitCommand(repoPath, "remote", "add", remoteName, remoteURL)
		defer cancel()
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git remote add failed: %s: %w", RedactURLCredentials(string(output)), err)
		}
	}
	return nil
}

// GetRemoteURL returns the configured origin URL, or empty if none is set.
//
// Only exit code 2 means "no such remote". Collapsing every other failure
// (context timeout, git missing, not a repository) into "" made Sync a silent
// no-op: it skips the push and reports success while nothing was pushed.
func GetRemoteURL(repoPath string) (string, error) {
	cmd, cancel := runGitCommand(repoPath, "remote", "get-url", "origin")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return "", nil // No remote configured
		}
		return "", fmt.Errorf("git remote get-url failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// RemoveRemote removes the named remote from the repo. A missing remote is
// not an error — the post-condition (no remote configured) holds either way.
func RemoveRemote(repoPath, remoteName string) error {
	cmd, cancel := runGitCommand(repoPath, "remote", "remove", remoteName)
	defer cancel()
	if output, err := cmd.CombinedOutput(); err != nil {
		// `git remote remove <name>` returns 2 when the remote doesn't
		// exist. That's the desired end state — treat as success. Match on
		// the exit code, not the message: "No such remote" is translated,
		// so on a localised system the string compare silently stops
		// matching and clearing an absent remote reports a failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return nil
		}
		return fmt.Errorf("git remote remove failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// GetStatus returns git status information
func GetStatus(repoPath string) (StatusInfo, error) {
	status := StatusInfo{}

	// Get current branch. `git branch --show-current` returns empty on
	// detached HEAD, mid-rebase, mid-bisect, or on a fresh repo with no
	// commits yet. Fall back to `git rev-parse --short HEAD` and mark the
	// status detached so the dashboard can render an appropriate pill
	// instead of falling through to the "no git repository" branch.
	branchCmd, cancelBranch := runGitCommand(repoPath, "branch", "--show-current")
	branchOutput, err := branchCmd.Output()
	cancelBranch()
	if err == nil {
		status.Branch = strings.TrimSpace(string(branchOutput))
	}
	if status.Branch == "" {
		shaCmd, cancelSHA := runGitCommand(repoPath, "rev-parse", "--short", "HEAD")
		shaOutput, shaErr := shaCmd.Output()
		cancelSHA()
		if shaErr == nil {
			sha := strings.TrimSpace(string(shaOutput))
			if sha != "" {
				status.Branch = sha
				status.Detached = true
			}
		}
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
		aheadBehindCmd, cancelAB := runGitCommand(repoPath, "rev-list", "--left-right", "--count", fmt.Sprintf("origin/%s...HEAD", status.Branch))
		output, err := aheadBehindCmd.Output()
		cancelAB()
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
	cmd, cancel := runGitCommand(repoPath, "log", fmt.Sprintf("-n%d", limit), fmt.Sprintf("--format=%s", format), "--", filePath)
	defer cancel()
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

	cmd, cancel := runGitCommand(repoPath, "checkout", ref, "--", filePath)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// GetDiff returns unified diff for uncommitted changes
func GetDiff(repoPath string) (string, error) {
	cmd, cancel := runGitCommand(repoPath, "diff", "HEAD")
	defer cancel()
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
	cmd, cancel := runGitCommand(repoPath, "diff", "HEAD", "--", filePath)
	defer cancel()
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
	cmd, cancel := runGitCommand(repoPath, "diff", "HEAD", "--stat")
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("git diff --stat failed: %w", err)
	}
	return string(output), nil
}

// Clone clones a repository to the specified path.
//
// The URL goes through the same allowlist as SetRemote/CloneWithProgress —
// git's `ext::` transport executes arbitrary commands, so an unvalidated URL
// here is remote code execution. `--` stops a URL or path beginning with `-`
// from being read as a flag.
func Clone(url, destPath string) error {
	if err := ValidateRemoteURL(url); err != nil {
		return fmt.Errorf("invalid clone URL: %w", err)
	}
	cmd, cancel := runGitNetworkCommand("", "clone", "--", url, destPath)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil && !isBenignWaitDelay(err) {
		return fmt.Errorf("git clone failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// CloneWithProgress clones repository and shows progress
func CloneWithProgress(url, destPath string) error {
	if !IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}
	if err := ValidateRemoteURL(url); err != nil {
		return fmt.Errorf("invalid clone URL: %w", err)
	}

	// Clone with progress
	cmd, cancel := runGitNetworkCommand("", "clone", "--progress", url, destPath)
	defer cancel()
	// Prompt suppression is applied to every git invocation by gitEnv().
	var stderr bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil && !isBenignWaitDelay(err) {
		return wrapGitError("git clone", stderr.String(), err)
	}
	return nil
}

// Pull pulls changes from remote
func Pull(repoPath string) error {
	cmd, cancel := runGitNetworkCommand(repoPath, "pull")
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil && !isBenignWaitDelay(err) {
		return fmt.Errorf("git pull failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// GetCurrentCommit returns the current commit hash
func GetCurrentCommit(repoPath string) (string, error) {
	cmd, cancel := runGitCommand(repoPath, "rev-parse", "HEAD")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// parseGitStatusLine parses a single record from `git status --porcelain -z`.
//
// The -z form is what makes this tractable: without it git C-quotes any path
// containing a non-ASCII byte, a quote, or a control character
// ("caf\303\251.txt"), and stripping the surrounding quotes without
// unescaping the body yields a corrupted name that no later git command can
// match. With -z, paths are emitted raw and NUL-delimited.
//
// A record is "XY <path>", where XY is the two-character status code. Rename
// and copy records place the destination in this record and the source in the
// following one; the caller is responsible for consuming that extra record
// (see splitPorcelainZ).
func parseGitStatusLine(line string) string {
	// Minimum record is two status characters, a space, and one path byte.
	if len(line) < 4 {
		return ""
	}
	return line[3:]
}

// isRenameRecord reports whether a porcelain record carries a second,
// following record holding the rename/copy source path.
//
// Matching on the full two-character code rather than a "R " / "RR " prefix
// matters: git emits RM when a path is renamed in the index and then modified
// in the worktree, plus RD/RA/RU during a conflicted merge. Those were
// previously parsed as ordinary records, producing a bogus filename.
func isRenameRecord(line string) bool {
	if len(line) < 2 {
		return false
	}
	return line[0] == 'R' || line[0] == 'C'
}

// GetChangedFiles returns the list of changed files, one entry per path.
func GetChangedFiles(repoPath string) ([]string, error) {
	// core.quotepath=false is belt-and-braces alongside -z: it stops git
	// quoting non-ASCII paths in any output this command produces.
	cmd, cancel := runGitCommand(repoPath, "-c", "core.quotepath=false", "status", "--porcelain", "-z")
	defer cancel()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	records := strings.Split(string(output), "\x00")

	var files []string
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == "" {
			continue
		}
		// A rename/copy record is followed by a bare source path with no
		// status prefix. Skip it so it isn't parsed as its own record.
		if isRenameRecord(record) {
			i++
		}
		if filename := parseGitStatusLine(record); filename != "" {
			files = append(files, filename)
		}
	}

	return files, nil
}

// StageFile stages a specific file
func StageFile(repoPath, filePath string) error {
	cmd, cancel := runGitCommand(repoPath, "add", filePath)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// UnstageFile unstages a specific file
func UnstageFile(repoPath, filePath string) error {
	cmd, cancel := runGitCommand(repoPath, "reset", "HEAD", "--", filePath)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset failed: %s: %w", RedactURLCredentials(string(output)), err)
	}
	return nil
}

// GetConfig retrieves a git config value
func GetConfig(repoPath, key string) (string, error) {
	cmd, cancel := runGitCommand(repoPath, "config", key)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SetConfig sets a git config value
func SetConfig(repoPath, key, value string) error {
	cmd, cancel := runGitCommand(repoPath, "config", key, value)
	defer cancel()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// GetFileDiffFromRef returns diff between working tree file and specific ref
func GetFileDiffFromRef(repoPath, filePath, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	cmd, cancel := runGitCommand(repoPath, "diff", ref, "--", filePath)
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("git diff failed: %w", err)
	}
	return string(output), nil
}

// GetFileContentAtRef returns file content at specific ref
func GetFileContentAtRef(repoPath, filePath, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	cmd, cancel := runGitCommand(repoPath, "show", fmt.Sprintf("%s:%s", ref, filePath))
	defer cancel()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git show failed: %w", err)
	}
	return string(output), nil
}

// GetDiffBetweenFiles returns diff between two arbitrary files.
//
// `git diff --no-index` exits 0 when the files are identical and 1 when
// they differ — exit 1 is the normal "they differ" signal, not a
// failure. It ALSO exits 1 when a file can't be accessed (e.g. it
// doesn't exist), so the exit code alone can't tell a real diff from an
// error.
//
// We distinguish them by stream: a genuine diff writes only to stdout,
// while access/usage failures write an `error:`/`fatal:` line to stderr.
// The previous implementation grepped combined output for `+++ b/` /
// `--- a/` markers, which mis-classified binary diffs (`Binary files a
// and b differ`, no such markers) as errors.
func GetDiffBetweenFiles(file1, file2 string) (string, error) {
	cmd, cancel := runGitCommand("", "diff", "--no-index", "--", file1, file2)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		// Exit 0: files are identical (empty diff).
		return stdout.String(), nil
	}

	exitErr, ok := err.(*exec.ExitError)
	if ok && exitErr.ExitCode() == 1 && stderr.Len() == 0 {
		// Exit 1 with nothing on stderr: the files differ. This covers
		// both textual and binary diffs.
		return stdout.String(), nil
	}

	return "", fmt.Errorf("git diff --no-index failed: %s: %w", strings.TrimSpace(stderr.String()), err)
}

// RefExists checks if a git reference exists in the repository
func RefExists(repoPath, ref string) (bool, error) {
	if ref == "" {
		return false, fmt.Errorf("ref is empty")
	}

	// Validate ref format to prevent path traversal
	if strings.Contains(ref, "..") {
		return false, fmt.Errorf("ref contains path traversal: %s", ref)
	}

	if strings.Contains(ref, "\\") {
		return false, fmt.Errorf("ref contains backslash: %s", ref)
	}

	if filepath.IsAbs(ref) && !strings.HasPrefix(ref, "refs/") && !strings.HasPrefix(ref, "HEAD") {
		return false, fmt.Errorf("ref is absolute but not a valid ref: %s", ref)
	}

	// `git cat-file -e <ref>` exits 1 only for a well-formed object name that
	// isn't present; a name that doesn't resolve at all (an ordinary branch
	// name that doesn't exist) is a fatal error with exit 128. That made
	// RefExists return an error for the most common "does not exist" case.
	// `rev-parse --verify --quiet` exits 1 for every non-resolution.
	cmd, cancel := runGitCommand(repoPath, "rev-parse", "--verify", "--quiet", ref+"^{object}")
	defer cancel()
	cmd.Stdout = nil
	err := cmd.Run()

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git rev-parse failed for ref %s: %w", ref, err)
	}

	return true, nil
}
