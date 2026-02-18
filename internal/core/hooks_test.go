package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

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

func TestRunHook_ExecutableHook(t *testing.T) {
	t.Run("successful hook execution", func(t *testing.T) {
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
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with successful hook should return nil")
	})

	t.Run("hook that fails", func(t *testing.T) {
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
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with failed hook should return nil (graceful degradation)")
	})

	t.Run("hook with custom exit code", func(t *testing.T) {
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
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}
		cfg := testConfig()

		// Act
		err = RunHook(ctx, cfg)

		// Assert
		assert.NoError(t, err, "RunHook() with failed hook should return nil (graceful degradation)")
	})
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

func TestRunHook_EnvironmentVariables_PassedToHook(t *testing.T) {
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
	hookContent := "#!/bin/bash\n[ \"$DOTCOR_HOOK\" = \"test-hook\" ] && [ \"$DOTCOR_FILE\" = \"/test/file\" ] && [ \"$DOTCOR_REPO_PATH\" = \"/repo/path\" ]\n"
	require.NoError(t, os.WriteFile(hookPath, []byte(hookContent), 0755), "failed to create hook")

	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", tempDir), "failed to set HOME")
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome), "failed to restore HOME")
	}()

	ctx := HookContext{
		HookType: "test-hook",
		FilePath: "/test/file",
		RepoPath: "/repo/path",
	}
	cfg := testConfig()

	// Act
	err = RunHook(ctx, cfg)

	// Assert
	assert.NoError(t, err, "RunHook() should pass environment variables to hook")
}
