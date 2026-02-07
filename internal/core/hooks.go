package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// GetHooksDir returns the hooks directory path (~/.dotcor/hooks)
func GetHooksDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".dotcor", "hooks"), nil
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
func RunHook(ctx HookContext) error {
	hooksDir, err := GetHooksDir()
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

	if info.Mode().Perm()&0111 == 0 {
		return nil
	}

	cmd := exec.Command(hookPath)
	cmd.Dir = hooksDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("DOTCOR_HOOK=%s", ctx.HookType),
		fmt.Sprintf("DOTCOR_FILE=%s", ctx.FilePath),
	)
	if ctx.RepoPath != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("DOTCOR_REPO_PATH=%s", ctx.RepoPath))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				fmt.Fprintf(os.Stderr, "[!] Hook %s failed (exit code %d): %s\n", ctx.HookType, status.ExitStatus(), string(output))
				return nil
			}
		}
		fmt.Fprintf(os.Stderr, "[!] Hook %s failed: %v\n", ctx.HookType, err)
		if len(output) > 0 {
			fmt.Fprintf(os.Stderr, "  Output: %s\n", string(output))
		}
		return nil
	}

	if len(output) > 0 {
		fmt.Printf("[Hook %s] %s", ctx.HookType, string(output))
	}

	return nil
}
