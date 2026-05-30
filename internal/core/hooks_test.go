package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		Logger: slog.Default(),
	}
}

func TestGetHooksDir(t *testing.T) {
	// Arrange
	home, err := os.UserHomeDir()
	require.NoError(t, err, "failed to get home directory")

	expected := filepath.Join(home, ".dotcor", "hooks")
	cfg := testConfig()

	// Act
	got, err := GetHooksDir(cfg)

	// Assert
	require.NoError(t, err, "GetHooksDir() error")
	assert.Equal(t, expected, got)
}

func TestRunHook_MissingHook(t *testing.T) {
	t.Run("missing hook returns nil", func(t *testing.T) {
		// Arrange
		ctx := HookContext{
			HookType: "nonexistent-hook",
			FilePath: "/tmp/test.txt",
		}
		cfg := testConfig()

		// Act
		err := RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with missing hook should return nil")
	})
}

// setupHookEnv creates a hooks directory with a single executable hook
// and points HOME at the parent temp dir, so RunHook resolves
// $HOME/.dotcor/hooks/<hookType> to the script we just wrote. Returns
// the temp dir for use by the caller.
func setupHookEnv(t *testing.T, hookType, hookScript string) string {
	t.Helper()
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, ".dotcor", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755), "failed to create hooks directory")
	hookPath := filepath.Join(hooksDir, hookType)
	require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0755), "failed to create hook")
	t.Setenv("HOME", tempDir)
	return tempDir
}

func TestRunHook_ExecutableHook(t *testing.T) {
	t.Run("successful hook returns nil", func(t *testing.T) {
		setupHookEnv(t, "test-hook", "#!/bin/bash\nexit 0\n")
		ctx := HookContext{HookType: "test-hook", FilePath: "/tmp/test.txt", RepoPath: "test/repo/path.txt"}
		err := RunHook(ctx, testConfig())
		assert.NoError(t, err, "RunHook() with successful hook should return nil")
	})

	t.Run("non-zero exit is swallowed (graceful degradation)", func(t *testing.T) {
		// The hook actually fails this time. RunHook deliberately swallows
		// non-zero exits so a misbehaving user hook never blocks dotcor's
		// main flow. Previously this subtest used `exit 0` and tested
		// nothing.
		setupHookEnv(t, "test-hook", "#!/bin/bash\nexit 1\n")
		ctx := HookContext{HookType: "test-hook", FilePath: "/tmp/test.txt", RepoPath: "test/repo/path.txt"}
		err := RunHook(ctx, testConfig())
		assert.NoError(t, err, "RunHook() should swallow non-zero exit and return nil")
	})

	t.Run("custom non-zero exit code also swallowed", func(t *testing.T) {
		setupHookEnv(t, "test-hook", "#!/bin/bash\nexit 42\n")
		ctx := HookContext{HookType: "test-hook", FilePath: "/tmp/test.txt", RepoPath: "test/repo/path.txt"}
		err := RunHook(ctx, testConfig())
		assert.NoError(t, err, "RunHook() should swallow exit 42 and return nil")
	})
}

// TestRunHook_Timeout verifies that a hook which would otherwise hang
// forever is killed and RunHook returns (without blocking) once the
// timeout elapses. The hook sleeps far longer than hookTimeout, but we
// temporarily shrink hookTimeout so the test stays fast.
func TestRunHook_Timeout(t *testing.T) {
	orig := hookTimeout
	hookTimeout = 500 * time.Millisecond
	defer func() { hookTimeout = orig }()

	setupHookEnv(t, "test-hook", "#!/bin/bash\nsleep 30\n")
	ctx := HookContext{HookType: "test-hook", FilePath: "/tmp/test.txt"}

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- RunHook(ctx, testConfig()) }()

	select {
	case err := <-done:
		assert.NoError(t, err, "RunHook() should swallow timeout and return nil")
		assert.Less(t, time.Since(start), 10*time.Second, "RunHook should return shortly after the timeout, not wait for the hook to finish")
	case <-time.After(10 * time.Second):
		t.Fatal("RunHook did not return after hook timeout — the hook was not killed")
	}
}

func TestRunHook_NonExecutableHook(t *testing.T) {
	t.Run("non-executable file is skipped", func(t *testing.T) {
		// Arrange
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		require.NoError(t, err, "failed to create temp dir")
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("failed to clean up temp dir: %v", err)
			}
		}()

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755), "failed to create hooks directory")

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\necho 'should not run'\nexit 0\n"
		require.NoError(t, os.WriteFile(hookPath, []byte(hookContent), 0644), "failed to create hook")

		oldHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", oldHome), "failed to restore HOME")
		}()

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with non-executable hook should return nil")
	})
}

func TestRunHook_DirectoryInsteadOfFile(t *testing.T) {
	t.Run("directory with hook name is skipped", func(t *testing.T) {
		// Arrange
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		require.NoError(t, err, "failed to create temp dir")
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("failed to clean up temp dir: %v", err)
			}
		}()

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755), "failed to create hooks directory")

		hookPath := filepath.Join(hooksDir, "test-hook")
		require.NoError(t, os.Mkdir(hookPath, 0755), "failed to create hook directory")

		oldHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", oldHome), "failed to restore HOME")
		}()

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with directory should return nil")
	})
}

func TestRunHook_EmptyRepoPath(t *testing.T) {
	t.Run("hook with empty repo path", func(t *testing.T) {
		// Arrange
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		require.NoError(t, err, "failed to create temp dir")
		defer func() {
			if err := os.RemoveAll(tempDir); err != nil {
				t.Logf("failed to clean up temp dir: %v", err)
			}
		}()

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		require.NoError(t, os.MkdirAll(hooksDir, 0755), "failed to create hooks directory")

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\nexit 0\n"
		require.NoError(t, os.WriteFile(hookPath, []byte(hookContent), 0755), "failed to create hook")

		oldHome := os.Getenv("HOME")
		require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
		defer func() {
			require.NoError(t, os.Setenv("HOME", oldHome), "failed to restore HOME")
		}()

		ctx := HookContext{
			HookType: "pre-sync",
			FilePath: "",
			RepoPath: "",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with empty repo path should return nil")
	})
}

// TestRunHook_EnvironmentVariables_PassedToHook verifies that the env
// vars actually reach the hook process. The previous version used a
// hook script that exited 0 if env vars matched and non-zero otherwise,
// then asserted RunHook returned nil — but RunHook swallows non-zero
// exit, so the test passed regardless of whether the env vars arrived.
//
// Now the hook writes the env var values to a sentinel file we can
// inspect after RunHook returns.
func TestRunHook_EnvironmentVariables_PassedToHook(t *testing.T) {
	tempDir := t.TempDir()
	hooksDir := filepath.Join(tempDir, ".dotcor", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	sentinel := filepath.Join(tempDir, "sentinel.txt")
	hookScript := "#!/bin/bash\n" +
		"printf '%s\\n%s\\n%s\\n' \"$DOTCOR_HOOK\" \"$DOTCOR_FILE\" \"$DOTCOR_REPO_PATH\" > " + sentinel + "\n"

	hookPath := filepath.Join(hooksDir, "test-hook")
	require.NoError(t, os.WriteFile(hookPath, []byte(hookScript), 0755))
	t.Setenv("HOME", tempDir)

	ctx := HookContext{
		HookType: "test-hook",
		FilePath: "/test/file",
		RepoPath: "/repo/path",
	}
	require.NoError(t, RunHook(ctx, testConfig()))

	got, err := os.ReadFile(sentinel)
	require.NoError(t, err, "hook should have written sentinel file")
	want := "test-hook\n/test/file\n/repo/path\n"
	assert.Equal(t, want, string(got), "hook should receive DOTCOR_HOOK, DOTCOR_FILE, DOTCOR_REPO_PATH env vars")
}
