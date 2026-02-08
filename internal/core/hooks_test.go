package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Logger: slog.Default(),
	}
}

func TestGetHooksDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	expected := filepath.Join(home, ".dotcor", "hooks")
	cfg := testConfig()

	got, err := GetHooksDir(cfg)
	if err != nil {
		t.Fatalf("GetHooksDir() error = %v", err)
	}

	if got != expected {
		t.Errorf("GetHooksDir() = %v, want %v", got, expected)
	}
}

func TestRunHook_MissingHook(t *testing.T) {
	t.Run("missing hook returns nil", func(t *testing.T) {
		ctx := HookContext{
			HookType: "nonexistent-hook",
			FilePath: "/tmp/test.txt",
		}

		cfg := testConfig()
		err := RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with missing hook should return nil, got %v", err)
		}
	})
}

func TestRunHook_ExecutableHook(t *testing.T) {
	t.Run("successful hook execution", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\nexit 0\n"
		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with successful hook should return nil, got %v", err)
		}
	})

	t.Run("hook that fails", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\nexit 1\n"
		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with failed hook should return nil (graceful degradation), got %v", err)
		}
	})

	t.Run("hook with custom exit code", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\nexit 42\n"
		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with failed hook should return nil (graceful degradation), got %v", err)
		}
	})
}

func TestRunHook_NonExecutableHook(t *testing.T) {
	t.Run("non-executable file is skipped", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\necho 'should not run'\nexit 0\n"
		if err := os.WriteFile(hookPath, []byte(hookContent), 0644); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with non-executable hook should return nil, got %v", err)
		}
	})
}

func TestRunHook_DirectoryInsteadOfFile(t *testing.T) {
	t.Run("directory with hook name is skipped", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		if err := os.Mkdir(hookPath, 0755); err != nil {
			t.Fatalf("failed to create hook directory: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "test-hook",
			FilePath: "/tmp/test.txt",
			RepoPath: "test/repo/path.txt",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with directory should return nil, got %v", err)
		}
	})
}

func TestRunHook_EmptyRepoPath(t *testing.T) {
	t.Run("hook with empty repo path", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dotcor-hooks-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		dotcorDir := filepath.Join(tempDir, ".dotcor")
		hooksDir := filepath.Join(dotcorDir, "hooks")
		if err := os.MkdirAll(hooksDir, 0755); err != nil {
			t.Fatalf("failed to create hooks directory: %v", err)
		}

		hookPath := filepath.Join(hooksDir, "test-hook")
		hookContent := "#!/bin/bash\nexit 0\n"
		if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
			t.Fatalf("failed to create hook: %v", err)
		}

		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tempDir)
		defer os.Setenv("HOME", oldHome)

		ctx := HookContext{
			HookType: "pre-sync",
			FilePath: "",
			RepoPath: "",
		}

		cfg := testConfig()
		err = RunHook(ctx, cfg)
		if err != nil {
			t.Errorf("RunHook() with empty repo path should return nil, got %v", err)
		}
	})
}
