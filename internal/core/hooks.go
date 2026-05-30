package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/justincordova/dotcor/internal/config"
)

// hookTimeout bounds how long a single hook script may run. A hook is
// user-authored code that runs while dotcor holds its global lock, so an
// unbounded hook (waiting on stdin, a hung network call, an infinite
// loop) would otherwise wedge the entire operation — and freeze the TUI,
// which owns the terminal. The hook is killed when this deadline elapses.
//
// It is a var rather than a const only so tests can shrink it; it is not
// modified at runtime.
var hookTimeout = 60 * time.Second

// GetHooksDir returns the hooks directory path (<config dir>/hooks).
//
// It derives from config.GetConfigDir() so that the DOTCOR_DIR override
// is honored, matching GetBackupDir and the lock path. Previously this
// hardcoded ~/.dotcor/hooks, so with DOTCOR_DIR set hooks silently
// resolved to the wrong directory.
func GetHooksDir(cfg *config.Config) (string, error) {
	cfg.Logger.Debug("getting hooks directory")
	configDir, err := config.GetConfigDir()
	if err != nil {
		cfg.Logger.Error("failed to get config directory", "error", err)
		return "", fmt.Errorf("getting config directory: %w", err)
	}
	return filepath.Join(configDir, "hooks"), nil
}

// HookContext provides context for hook execution
type HookContext struct {
	HookType string // e.g., "pre-add", "post-add", "pre-remove", etc.
	FilePath string // Source file path being operated on
	RepoPath string // Repo path (only for post hooks)
}

// RunHook executes a hook script if it exists
// Gracefully skips if hook doesn't exist or isn't executable
// Logs errors but doesn't fail the main operation
func RunHook(ctx HookContext, cfg *config.Config) error {
	cfg.Logger.Debug("running hook", "type", ctx.HookType, "file", ctx.FilePath)
	hooksDir, err := GetHooksDir(cfg)
	if err != nil {
		return fmt.Errorf("getting hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, ctx.HookType)

	info, err := os.Stat(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("checking hook %s: %w", ctx.HookType, err)
	}

	if info.IsDir() {
		return nil
	}

	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return nil
	}

	cmdCtx, cancel := context.WithTimeout(context.Background(), hookTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, hookPath)
	cmd.Dir = hooksDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DOTCOR_HOOK=%s", ctx.HookType),
		fmt.Sprintf("DOTCOR_FILE=%s", ctx.FilePath),
	)
	if ctx.RepoPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DOTCOR_REPO_PATH=%s", ctx.RepoPath))
	}
	// Run the hook in its own process group and kill the whole group on
	// timeout, so a hook that spawns children doesn't leak orphaned
	// processes after we give up on it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			cfg.Logger.Warn("hook timed out and was killed", "hook", ctx.HookType, "timeout", hookTimeout, "output", string(output))
			return nil
		}
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				cfg.Logger.Warn("hook failed", "hook", ctx.HookType, "exit_code", status.ExitStatus(), "output", string(output))
				return nil
			}
		}
		cfg.Logger.Warn("hook failed", "hook", ctx.HookType, "error", err, "output", string(output))
		return nil
	}

	if len(output) > 0 {
		cfg.Logger.Debug("hook output", "hook", ctx.HookType, "output", string(output))
	}

	return nil
}
