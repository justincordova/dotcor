# Log/slog Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace ad-hoc `fmt.Printf` logging with Go 1.21+ `log/slog` for structured logging throughout DotCor codebase.

**Architecture:** Dependency injection via Config struct - logger created once at startup, passed to all functions via config parameter. Hybrid approach: keep `fmt.Printf` for user-facing UI (banners, tables, prompts) and use `slog` for system logging (debug, info, warn, error).

**Tech Stack:** Go 1.21+ `log/slog`, Cobra CLI framework, Viper for config

---

## Task 1: Create logger package infrastructure

**Files:**
- Create: `internal/logger/logger.go`
- Create: `internal/logger/logger_test.go`
- Modify: `go.mod`

**Step 1: Create logger.go with configuration helpers**

```go
package logger

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// ConfigureFromFlags creates a configured slog.Logger based on CLI flags
func ConfigureFromFlags(cmd *cobra.Command) *slog.Logger {
	debug, _ := cmd.Flags().GetBool("debug")
	quiet, _ := cmd.Flags().GetBool("quiet")
	logFile, _ := cmd.Flags().GetString("log-file")
	jsonFormat, _ := cmd.Flags().GetBool("json")

	// Determine log level
	level := levelFromFlags(debug, quiet)

	// Create handler
	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				// Shorten common field names for readability
				if a.Key == "source_path" {
					a.Key = "src"
				}
				if a.Key == "repo_path" {
					a.Key = "repo"
				}
				if a.Key == "backup_path" {
					a.Key = "backup"
				}
				return a
			},
		})
	}

	// If log file specified, create file handler
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// Fallback to stderr if file creation fails
			return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))
		}
		handler = slog.NewTextHandler(file, &slog.HandlerOptions{
			Level: level,
		})
	}

	return slog.New(handler)
}

// levelFromFlags determines log level from debug/quiet flags
func levelFromFlags(debug, quiet bool) slog.Level {
	switch {
	case quiet:
		return slog.LevelError
	case debug:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}
```

**Step 2: Create logger_test.go**

```go
package logger

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigureFromFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.PersistentFlags().Bool("debug", false, "")
	cmd.PersistentFlags().Bool("quiet", false, "")
	cmd.PersistentFlags().String("log-file", "", "")
	cmd.PersistentFlags().Bool("json", false, "")

	// Test default configuration
	logger := ConfigureFromFlags(cmd)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLevelFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		quiet    bool
		expected slog.Level
	}{
		{"default", false, false, slog.LevelInfo},
		{"debug", true, false, slog.LevelDebug},
		{"quiet", false, true, slog.LevelError},
		{"debug overrides quiet", true, true, slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levelFromFlags(tt.debug, tt.quiet)
			if got != tt.expected {
				t.Errorf("levelFromFlags(%v, %v) = %v, want %v", tt.debug, tt.quiet, got, tt.expected)
			}
		})
	}
}

func TestLogEmission(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected log output")
	}
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./internal/logger/... -v`
Expected: All tests PASS

**Step 4: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go
git commit -m "feat: create logger package with slog configuration"
```

---

## Task 2: Add Logger field to Config struct

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add Logger field to Config struct**

```go
type Config struct {
	Logger         *slog.Logger  // NEW: Structured logger for system logging
	Version        string        `yaml:"version"`
	RepoPath       string        `yaml:"repo_path"`
	GitEnabled     bool          `yaml:"git_enabled"`
	GitRemote      string        `yaml:"git_remote"`
	IgnorePatterns []string      `yaml:"ignore_patterns"`
	ManagedFiles   []ManagedFile `yaml:"managed_files"`
}
```

**Step 2: Update NewDefaultConfig to initialize logger**

```go
func NewDefaultConfig() *Config {
	return &Config{
		Logger:         nil,  // Will be set after config load
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(homeDir, ".dotcor", "files"),
		GitEnabled:     true,
		IgnorePatterns: GetDefaultIgnorePatterns(),
		ManagedFiles:   []ManagedFile{},
	}
}
```

**Step 3: Run tests to verify config still works**

Run: `go test ./internal/config/... -v`
Expected: All tests PASS

**Step 4: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add Logger field to Config struct"
```

---

## Task 3: Add logging CLI flags to main.go

**Files:**
- Modify: `cmd/dotcor/main.go`

**Step 1: Add CLI flags for logging control**

```go
func init() {
	viper.SetDefault("version", version)

	// Add logging flags to root command
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress INFO messages")
	rootCmd.PersistentFlags().String("log-file", "", "Write logs to file")
	rootCmd.PersistentFlags().Bool("json", false, "Output logs in JSON format")
}
```

**Step 2: Create configureLogger function**

```go
func configureLogger(cmd *cobra.Command, cfg *config.Config) {
	// Configure logger from flags
	cfg.Logger = logger.ConfigureFromFlags(cmd)
}
```

**Step 3: Update runRoot to configure logger**

```go
func runRoot(cmd *cobra.Command, args []string) {
	printBanner()

	// Configure logger first
	configureLogger(cmd, &config.Config{})

	// Try to load config and show status
	cfg, err := config.LoadConfig()
	if err != nil {
		// Not initialized - use default config with logger
		cfg = config.NewDefaultConfig()
		configureLogger(cmd, cfg)

		fmt.Printf("  %s[!] Not initialized%s\n", colorYellow, colorReset)
		fmt.Println()
		fmt.Printf("  %sGet started:%s\n", colorDim, colorReset)
		fmt.Println("    dotcor init          Initialize DotCor")
		fmt.Println("    dotcor --help        Show all commands")
		fmt.Println()
		return
	}

	// Configure logger with loaded config
	configureLogger(cmd, cfg)

	// Show quick status
	showQuickStatus(cfg)
}
```

**Step 4: Add import for logger package**

```go
import (
	"fmt"
	"os"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/logger"  // NEW
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

**Step 5: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Test basic logging**

Run: `./dotcor --debug version`
Expected: Debug logs emitted to stderr (if any)

**Step 7: Commit**

```bash
git add cmd/dotcor/main.go
git commit -m "feat: add logging CLI flags and configuration"
```

---

## Task 4: Refactor internal/core/backup.go

**Files:**
- Modify: `internal/core/backup.go`
- Modify: `internal/core/backup_test.go`

**Step 1: Add cfg parameter to all exported functions**

```go
func CreateBackup(sourcePath string, cfg *config.Config) (string, error) {
	cfg.Logger.Debug("creating backup", "file", sourcePath)

	// Expand source path
	expanded, err := config.ExpandPath(sourcePath)
	if err != nil {
		cfg.Logger.Error("failed to expand path", "file", sourcePath, "error", err)
		return "", err
	}

	// Check if source exists
	if !fs.FileExists(expanded) {
		cfg.Logger.Error("source file does not exist", "file", sourcePath)
		return "", fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Get backup directory
	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return "", err
	}

	// ... rest of function implementation ...

	cfg.Logger.Info("backup created",
		"file", sourcePath,
		"path", backupPath,
	)

	return backupPath, nil
}

func RestoreBackup(backupPath, targetPath string, cfg *config.Config) error {
	cfg.Logger.Debug("restoring from backup",
		"backup", backupPath,
		"target", targetPath,
	)

	// ... implementation ...

	cfg.Logger.Info("backup restored",
		"backup", backupPath,
		"target", targetPath,
	)

	return nil
}
```

**Step 2: Update backup_test.go to use test logger**

```go
func TestCreateBackup(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	// Test backup creation
	sourcePath := filepath.Join(t.TempDir(), "test.txt")
	err := os.WriteFile(sourcePath, []byte("test content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	backupPath, err := CreateBackup(sourcePath, cfg)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("backup file not created: %s", backupPath)
	}

	// Verify log was written
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected log output")
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/core/... -run TestCreateBackup -v`
Expected: Test PASS

**Step 4: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/core/backup.go internal/core/backup_test.go
git commit -m "refactor: add structured logging to backup operations"
```

---

## Task 5: Refactor internal/core/transaction.go

**Files:**
- Modify: `internal/core/transaction.go`
- Modify: `internal/core/transaction_test.go`

**Step 1: Add cfg to Transaction struct**

```go
type Transaction struct {
	config     *config.Config  // NEW
	operations []Operation
	rollbacks  []func() error
}

func NewTransaction(cfg *config.Config) *Transaction {
	return &Transaction{
		config:     cfg,
		operations: []Operation{},
		rollbacks:  []func() error{},
	}
}
```

**Step 2: Add logging to Execute, Rollback, Commit**

```go
func (t *Transaction) Execute(op Operation) error {
	t.config.Logger.Debug("executing operation", "op", op.Describe())

	if t.committed {
		err := fmt.Errorf("transaction already committed")
		t.config.Logger.Error("transaction execute failed", "error", err)
		return err
	}
	if err := op.Do(); err != nil {
		t.config.Logger.Error("operation failed, rolling back",
			"op", op.Describe(),
			"error", err,
		)
		t.Rollback()
		return fmt.Errorf("executing %s: %w", op.Describe(), err)
	}
	t.executed = append(t.executed, op)
	t.config.Logger.Debug("operation executed successfully", "op", op.Describe())
	return nil
}

func (t *Transaction) Rollback() error {
	t.config.Logger.Warn("rolling back transaction", "operations", len(t.executed))

	for i := len(t.executed) - 1; i >= 0; i-- {
		op := t.executed[i]
		t.config.Logger.Debug("rolling back operation", "op", op.Describe(), "index", i)
		if err := op.Undo(); err != nil {
			t.config.Logger.Error("rollback failed",
				"op", op.Describe(),
				"error", err,
				"index", i,
			)
			return fmt.Errorf("rolling back %s: %w", op.Describe(), err)
		}
	}
	t.config.Logger.Info("transaction rolled back")
	return nil
}

func (t *Transaction) Commit() {
	t.config.Logger.Debug("committing transaction", "operations", len(t.executed))
	t.committed = true
}
```

**Step 3: Update tests to use test logger**

```go
func TestTransactionRollback(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	tx := NewTransaction(cfg)
	// ... test logic ...

	// Verify logs were written
	output := buf.String()
	assert.Contains(t, output, "executing operation")
	assert.Contains(t, output, "rolling back transaction")
}
```

**Step 4: Run tests**

Run: `go test ./internal/core/... -run TestTransaction -v`
Expected: Tests PASS

**Step 5: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/core/transaction.go internal/core/transaction_test.go
git commit -m "refactor: add structured logging to transactions"
```

---

## Task 6: Refactor internal/core/lock.go

**Files:**
- Modify: `internal/core/lock.go`
- Modify: `internal/core/lock_test.go`

**Step 1: Add cfg parameter and logging**

```go
func AcquireLock(cfg *config.Config) error {
	cfg.Logger.Debug("acquiring lock")

	lockPath, err := GetLockPath()
	if err != nil {
		cfg.Logger.Error("failed to get lock path", "error", err)
		return err
	}

	// ... implementation ...

	cfg.Logger.Debug("lock acquired")
	return nil
}

func ReleaseLock(cfg *config.Config) error {
	cfg.Logger.Debug("releasing lock")

	lockPath, err := GetLockPath()
	if err != nil {
		cfg.Logger.Error("failed to get lock path", "error", err)
		return err
	}

	// ... implementation ...

	cfg.Logger.Debug("lock released")
	return nil
}

func IsStale(lockPath string, cfg *config.Config) (bool, error) {
	cfg.Logger.Debug("checking if lock is stale", "path", lockPath)

	// ... implementation ...

	cfg.Logger.Info("stale lock check result", "stale", stale)
	return stale, nil
}
```

**Step 2: Update tests**

```go
func TestAcquireLock(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	// Test lock acquisition
	err := AcquireLock(cfg)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	defer ReleaseLock(cfg)

	// Verify logs
	output := buf.String()
	assert.Contains(t, output, "acquiring lock")
	assert.Contains(t, output, "lock acquired")
}
```

**Step 3: Run tests**

Run: `go test ./internal/core/... -run TestLock -v`
Expected: Tests PASS

**Step 4: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/core/lock.go internal/core/lock_test.go
git commit -m "refactor: add structured logging to lock operations"
```

---

## Task 7: Refactor internal/core/hooks.go

**Files:**
- Modify: `internal/core/hooks.go`
- Modify: `internal/core/hooks_test.go`

**Step 1: Add cfg parameter and logging**

```go
func RunHook(ctx HookContext, cfg *config.Config) error {
	cfg.Logger.Debug("running hook", "type", ctx.HookType, "file", ctx.FilePath)

	hooksDir, err := GetHooksDir()
	if err != nil {
		cfg.Logger.Error("failed to get hooks directory", "error", err)
		return fmt.Errorf("getting hooks directory: %w", err)
	}

	// ... implementation ...

	cmd := exec.Command(hookPath)
	// ... setup ...

	output, err := cmd.CombinedOutput()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				cfg.Logger.Warn("hook failed",
					"type", ctx.HookType,
					"exit_code", status.ExitStatus(),
					"output", string(output),
				)
				fmt.Fprintf(os.Stderr, "[!] Hook %s failed (exit code %d): %s\n", ctx.HookType, status.ExitStatus(), string(output))
				return nil
			}
		}
		cfg.Logger.Warn("hook execution failed",
			"type", ctx.HookType,
			"error", err,
		)
		fmt.Fprintf(os.Stderr, "[!] Hook %s failed: %v\n", ctx.HookType, err)
		if len(output) > 0 {
			fmt.Fprintf(os.Stderr, "  Output: %s\n", string(output))
		}
		return nil
	}

	if len(output) > 0 {
		cfg.Logger.Info("hook output",
			"type", ctx.HookType,
			"output", string(output),
		)
		fmt.Printf("[Hook %s] %s", ctx.HookType, string(output))
	}

	cfg.Logger.Info("hook completed successfully", "type", ctx.HookType)
	return nil
}
```

**Step 2: Update tests**

```go
func TestRunHook(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	ctx := HookContext{
		HookType: "test-hook",
		FilePath: "/tmp/test",
	}

	// Test hook execution
	err := RunHook(ctx, cfg)
	if err != nil {
		t.Logf("Hook execution error (expected): %v", err)
	}

	// Verify logs
	output := buf.String()
	assert.Contains(t, output, "running hook")
}
```

**Step 3: Run tests**

Run: `go test ./internal/core/... -run TestHook -v`
Expected: Tests PASS

**Step 4: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/core/hooks.go internal/core/hooks_test.go
git commit -m "refactor: add structured logging to hook execution"
```

---

## Task 8: Refactor internal/git/git.go

**Files:**
- Modify: `internal/git/git.go`
- Modify: `internal/git/git_test.go`

**Step 1: Add cfg parameter and logging**

```go
func AutoCommit(repoPath, message string, cfg *config.Config) error {
	cfg.Logger.Debug("auto committing", "repo", repoPath, "message", message)

	// Check if there are changes
	hasChanges, err := HasChanges(repoPath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to check for changes", "repo", repoPath, "error", err)
		return fmt.Errorf("checking for changes: %w", err)
	}
	if !hasChanges {
		cfg.Logger.Debug("no changes to commit", "repo", repoPath)
		return nil
	}

	// Stage all changes
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = repoPath
	if output, err := addCmd.CombinedOutput(); err != nil {
		cfg.Logger.Error("git add failed",
			"repo", repoPath,
			"output", string(output),
			"error", err,
		)
		return fmt.Errorf("git add failed: %s: %w", string(output), err)
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = repoPath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		cfg.Logger.Error("git commit failed",
			"repo", repoPath,
			"output", string(output),
			"error", err,
		)
		return fmt.Errorf("git commit failed: %s: %w", string(output), err)
	}

	cfg.Logger.Info("git commit successful",
		"repo", repoPath,
		"message", message,
	)

	return nil
}

func HasChanges(repoPath string, cfg *config.Config) (bool, error) {
	cfg.Logger.Debug("checking for uncommitted changes", "repo", repoPath)

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		cfg.Logger.Error("git status failed", "repo", repoPath, "error", err)
		return false, err
	}

	hasChanges := len(strings.TrimSpace(string(output))) > 0
	cfg.Logger.Debug("changes check result", "repo", repoPath, "has_changes", hasChanges)

	return hasChanges, nil
}
```

**Step 2: Update all git functions with cfg parameter**

Similar pattern for: InitRepo, Sync, GetStatus, GetFileHistory, etc.

**Step 3: Update tests**

```go
func TestAutoCommit(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	// Setup test repo
	repoPath := t.TempDir()
	if err := InitRepo(repoPath, cfg); err != nil {
		t.Fatal(err)
	}

	// Test commit
	err := AutoCommit(repoPath, "test commit", cfg)
	if err != nil {
		t.Fatalf("AutoCommit failed: %v", err)
	}

	// Verify logs
	output := buf.String()
	assert.Contains(t, output, "auto committing")
	assert.Contains(t, output, "git commit successful")
}
```

**Step 4: Run tests**

Run: `go test ./internal/git/... -v`
Expected: Tests PASS

**Step 5: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "refactor: add structured logging to git operations"
```

---

## Task 9: Refactor internal/fs/fs.go

**Files:**
- Modify: `internal/fs/fs.go`
- Modify: `internal/fs/fs_test.go`

**Step 1: Add cfg parameter and logging**

```go
func MoveFile(src, dst string, cfg *config.Config) error {
	cfg.Logger.Debug("moving file", "src", src, "dst", dst)

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := EnsureDir(dstDir, cfg); err != nil {
		cfg.Logger.Error("failed to create destination directory",
			"dir", dstDir,
			"error", err,
		)
		return err
	}

	// Try atomic rename
	err := os.Rename(src, dst)
	if err == nil {
		cfg.Logger.Debug("file moved successfully", "src", src, "dst", dst)
		return nil
	}

	// Cross-device move, use copy
	cfg.Logger.Debug("cross-device move, using copy", "src", src, "dst", dst)

	dstExisted := FileExists(dst)
	if err := CopyWithPermissions(src, dst, cfg); err != nil {
		cfg.Logger.Error("file copy failed",
			"src", src,
			"dst", dst,
			"error", err,
		)
		return err
	}

	if err := os.Remove(src); err != nil {
		cfg.Logger.Error("failed to remove source file",
			"src", src,
			"error", err,
		)
		if !dstExisted {
			os.Remove(dst)
		}
		return fmt.Errorf("removing original file: %w", err)
	}

	cfg.Logger.Info("file moved", "src", src, "dst", dst)
	return nil
}

func CopyWithPermissions(src, dst string, cfg *config.Config) error {
	cfg.Logger.Debug("copying file", "src", src, "dst", dst)

	// ... implementation ...

	cfg.Logger.Debug("file copied", "src", src, "dst", dst)
	return nil
}
```

**Step 2: Update all fs functions with cfg parameter**

EnsureDir, FileExists, RemoveFile, etc. all get cfg parameter.

**Step 3: Update tests**

```go
func TestMoveFile(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	// Test file move
	src := filepath.Join(t.TempDir(), "source.txt")
	dst := filepath.Join(t.TempDir(), "dest.txt")
	os.WriteFile(src, []byte("test"), 0644)

	if err := MoveFile(src, dst, cfg); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// Verify file moved
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("destination file not created")
	}

	// Verify logs
	output := buf.String()
	assert.Contains(t, output, "moving file")
}
```

**Step 4: Run tests**

Run: `go test ./internal/fs/... -v`
Expected: Tests PASS

**Step 5: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 6: Commit**

```bash
git add internal/fs/fs.go internal/fs/fs_test.go
git commit -m "refactor: add structured logging to file operations"
```

---

## Task 10: Refactor internal/fs/symlink.go

**Files:**
- Modify: `internal/fs/symlink.go`
- Modify: `internal/fs/symlink_test.go`

**Step 1: Add cfg parameter and logging**

```go
func CreateSymlink(target, link string, cfg *config.Config) error {
	cfg.Logger.Debug("creating symlink", "target", target, "link", link)

	// ... implementation ...

	cfg.Logger.Info("symlink created",
		"target", target,
		"link", link,
	)

	return nil
}

func RemoveSymlink(link string, cfg *config.Config) error {
	cfg.Logger.Debug("removing symlink", "link", link)

	// ... implementation ...

	cfg.Logger.Debug("symlink removed", "link", link)
	return nil
}
```

**Step 2: Update tests**

```go
func TestCreateSymlink(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger: logger,
		RepoPath: t.TempDir(),
	}

	// Test symlink creation
	target := filepath.Join(t.TempDir(), "target.txt")
	link := filepath.Join(t.TempDir(), "link.txt")
	os.WriteFile(target, []byte("test"), 0644)

	if err := CreateSymlink(target, link, cfg); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	// Verify logs
	output := buf.String()
	assert.Contains(t, output, "creating symlink")
}
```

**Step 3: Run tests**

Run: `go test ./internal/fs/symlink_test.go -v`
Expected: Tests PASS

**Step 4: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/fs/symlink.go internal/fs/symlink_test.go
git commit -m "refactor: add structured logging to symlink operations"
```

---

## Task 11: Refactor cmd/dotcor/add.go

**Files:**
- Modify: `cmd/dotcor/add.go`

**Step 1: Add cfg parameter to runAdd**

```go
func runAdd(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}

	// ... get flags ...

	// Acquire lock (skip for dry-run)
	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return fmt.Errorf("acquiring lock: %w", err)
		}
		defer core.ReleaseLock(cfg)
	}

	// ... implementation ...

	for _, file := range files {
		cfg.Logger.Info("processing file", "file", file)

		// Validation
		normalized, err := config.NormalizePath(file)
		if err != nil {
			cfg.Logger.Error("failed to normalize path", "file", file, "error", err)
			if !force {
				return err
			}
			continue
		}

		cfg.Logger.Debug("file normalized", "file", file, "normalized", normalized)

		// ... rest of implementation ...
	}

	cfg.Logger.Info("add operation completed", "files_added", added, "skipped", skipped)
	return nil
}
```

**Step 2: Update all function calls to pass cfg**

Update calls to core functions to pass cfg parameter.

**Step 3: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 4: Manual test**

Run: `./dotcor add ~/.zshrc --dry-run`
Expected: Dry-run output with logs (if --debug used)

**Step 5: Commit**

```bash
git add cmd/dotcor/add.go
git commit -m "refactor: add structured logging to add command"
```

---

## Task 12: Refactor remaining command files

**Files:**
- Modify: `cmd/dotcor/remove.go`
- Modify: `cmd/dotcor/restore.go`
- Modify: `cmd/dotcor/sync.go`
- Modify: `cmd/dotcor/doctor.go`
- Modify: `cmd/dotcor/clone.go`
- Modify: `cmd/dotcor/init.go`
- Modify: `cmd/dotcor/cleanup.go`
- Modify: `cmd/dotcor/list.go`
- Modify: `cmd/dotcor/history.go`
- Modify: `cmd/dotcor/diff.go`
- Modify: `cmd/dotcor/status.go`
- Modify: `cmd/dotcor/rebuild.go`
- Modify: `cmd/dotcor/adopt.go`

**Step 1: Refactor cmd/dotcor/remove.go**

```go
func runRemove(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// ... flags ...

	if !dryRun {
		if err := core.AcquireLock(cfg); err != nil {
			return err
		}
		defer core.ReleaseLock(cfg)
	}

	for _, arg := range args {
		cfg.Logger.Info("removing managed file", "file", arg)

		// ... implementation ...
	}

	cfg.Logger.Info("remove operation completed")
	return nil
}
```

**Step 2: Refactor cmd/dotcor/restore.go**

Similar pattern: load cfg, add logging for each step.

**Step 3: Refactor cmd/dotcor/sync.go**

Similar pattern with git operation logging.

**Step 4: Refactor cmd/dotcor/doctor.go**

Log each health check.

**Step 5: Refactor all other commands**

Clone, init, cleanup, list, history, diff, status, rebuild, adopt - all add logging.

**Step 6: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 7: Commit**

```bash
git add cmd/dotcor/*.go
git commit -m "refactor: add structured logging to all commands"
```

---

## Task 13: Update config paths.go with logging

**Files:**
- Modify: `internal/config/paths.go`

**Step 1: Add cfg parameter and logging**

```go
func NormalizePath(path string) string {
	// Pure function, no logging needed
	// ... implementation
}

func ExpandPath(path string, cfg *config.Config) (string, error) {
	cfg.Logger.Debug("expanding path", "path", path)

	// ... implementation ...

	cfg.Logger.Debug("path expanded", "path", path, "expanded", expanded)
	return expanded, nil
}

func GetRepoFilePath(cfg *config.Config, repoPath string) (string, error) {
	cfg.Logger.Debug("getting repo file path", "repo", repoPath)

	// ... implementation

	return fullPath, nil
}
```

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Run tests**

Run: `go test ./internal/config/... -v`
Expected: Tests PASS

**Step 4: Commit**

```bash
git add internal/config/paths.go
git commit -m "refactor: add logging to path operations"
```

---

## Task 14: Update validator.go with logging

**Files:**
- Modify: `internal/core/validator.go`

**Step 1: Add cfg parameter and logging**

```go
func ValidateSourceFile(path string, cfg *config.Config) error {
	cfg.Logger.Debug("validating source file", "path", path)

	// ... validation checks ...

	// Log secret detection results
	if len(secrets) > 0 {
		cfg.Logger.Warn("potential secrets detected",
			"path", path,
			"secrets_count", len(secrets),
		)
	}

	cfg.Logger.Debug("validation complete", "path", path, "valid", true)
	return nil
}

func ValidateRepoPath(repoPath string, cfg *config.Config) error {
	cfg.Logger.Debug("validating repo path", "path", repoPath)

	// ... implementation ...

	cfg.Logger.Debug("repo path valid", "path", repoPath)
	return nil
}
```

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Run tests**

Run: `go test ./internal/core/validator_test.go -v`
Expected: Tests PASS

**Step 4: Commit**

```bash
git add internal/core/validator.go internal/core/validator_test.go
git commit -m "refactor: add logging to validation operations"
```

---

## Task 15: Run full test suite

**Files:** None

**Step 1: Run all tests**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Manual smoke test**

Run: `./dotcor --help`
Expected: Help text displayed, logs emitted to stderr (if --debug)

**Step 4: Commit any test fixes**

If tests failed, fix issues and commit.

---

## Task 16: Create LOGGING.md documentation

**Files:**
- Create: `LOGGING.md`

**Step 1: Create logging documentation**

```markdown
# DotCor Logging Guide

## Overview

DotCor uses Go 1.21+ `log/slog` for structured system logging. User-facing output (banners, tables, progress) continues to use `fmt.Printf` for interactive UI.

## Log Levels

- **DEBUG**: Detailed internal operations, decisions, flow (requires `--debug` flag)
- **INFO**: Important events (file added, backup created, git commits)
- **WARN**: Issues that don't stop operations (hook failures, retries)
- **ERROR**: Failures that stop operations (lock held, file not found)

## CLI Flags

- `--debug`: Enable DEBUG level logging
- `--quiet`: Suppress INFO messages (show only ERROR)
- `--log-file`: Write logs to specified file instead of stderr
- `--json`: Output logs in JSON format (machine-parseable)

## Examples

### Normal operation
```bash
$ dotcor add ~/.zshrc
✓ Added ~/.zshrc → shell/zshrc
```

### Debug mode
```bash
$ dotcor add ~/.zshrc --debug
✓ Added ~/.zshrc → shell/zshrc

# Logs emitted to stderr:
2025/01/07 12:30:00 DEBUG expanding path=~/.zshrc
2025/01/07 12:30:00 INFO added file src=~/.zshrc repo=shell/zshrc
```

### Quiet mode
```bash
$ dotcor add ~/.zshrc --quiet
✓ Added ~/.zshrc → shell/zshrc
# Only ERROR logs to stderr
```

### Log file
```bash
$ dotcor add ~/.zshrc --log-file=/tmp/dotcor.log
✓ Added ~/.zshrc → shell/zshrc
# Logs written to /tmp/dotcor.log
```

### JSON format
```bash
$ dotcor add ~/.zshrc --json
✓ Added ~/.zshrc → shell/zshrc

# Logs in JSON:
{"time":"2025-01-07T12:30:00Z","level":"INFO","msg":"added file","src":"~/.zshrc","repo":"shell/zshrc"}
```

## Adding Logging to Code

### Functions that need logging
```go
func SomeOperation(input string, cfg *config.Config) error {
	cfg.Logger.Debug("operation starting", "input", input)

	// ... do work ...

	if err != nil {
		cfg.Logger.Error("operation failed", "input", input, "error", err)
		return err
	}

	cfg.Logger.Info("operation completed", "input", input)
	return nil
}
```

### Structured fields
Common fields to include:
- `op`: Operation type (backup, add, remove, git)
- `file`: File being operated on
- `src`: Source path (shortened from source_path)
- `repo`: Repository path (shortened from repo_path)
- `error`: Error object (for ERROR level logs)
- `duration`: Operation duration (useful for performance tracking)

### User-facing output
Continue using `fmt.Printf` for interactive UI:
```go
fmt.Printf("✓ Added %s\n", file)
```

## Best Practices

1. **Log at appropriate level**
   - DEBUG: Internal operations, decisions
   - INFO: User-relevant events
   - WARN: Non-critical issues
   - ERROR: Stopping failures

2. **Include structured fields**
   - Use key-value pairs for context
   - Be consistent with field names
   - Shorten common names (src vs source_path)

3. **Separate concerns**
   - slog for system logging
   - fmt.Printf for user-facing UI
   - Don't mix them

4. **Test logging behavior**
   - Verify logs emit at correct levels
   - Check structured fields are present
   - Use test logger with buffer in tests
```

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add LOGGING.md
git commit -m "docs: add logging guide for contributors"
```

---

## Task 17: Update CLAUDE.md with logging guidelines

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add logging section to CLAUDE.md**

```markdown
## Logging

DotCor uses `log/slog` for structured logging following these principles:

1. **Inject logger via Config struct** - Logger field in Config, passed to all functions
2. **Separate user output from logs** - fmt.Printf for UI, slog for system logging
3. **Use appropriate log levels** - DEBUG (internal), INFO (events), WARN (issues), ERROR (failures)
4. **Include structured fields** - Key-value pairs for context (op, file, error)
5. **Test log emission** - Verify logs at correct levels with proper fields

Example:
```go
func BackupFile(path string, cfg *config.Config) error {
	cfg.Logger.Debug("starting backup", "file", path)
	// ... implementation
	cfg.Logger.Info("backup created", "file", path, "backup", backupPath)
	return nil
}
```
```

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with logging guidelines"
```

---

## Task 18: Final verification and integration testing

**Files:** None

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All tests PASS

**Step 2: Build project**

Run: `go build ./...`
Expected: Build succeeds with no warnings

**Step 3: Manual integration test**

```bash
# Test init
./dotcor init --debug

# Test add
./dotcor add ~/.zshrc --debug

# Test status
./dotcor status --debug

# Test sync
./dotcor sync --debug

# Test quiet mode
./dotcor status --quiet

# Test log file
./dotcor list --log-file=/tmp/test.log
cat /tmp/test.log

# Test JSON format
./dotcor list --json
```

Expected: All operations complete, logs emitted appropriately

**Step 4: Commit any final adjustments**

Fix any issues found during testing and commit.

---

## Task 19: Clean up LOGGING_REFACTOR_PLAN.md

**Files:**
- Remove: `LOGGING_REFACTOR.md` (if created during brainstorming)

**Step 1: Remove the design document if it exists**

Run: `rm -f LOGGING_REFACTOR.md`
Expected: File removed (or doesn't exist)

**Step 2: Final commit**

```bash
git add -A
git commit -m "chore: remove refactor design document after implementation"
```

---

## Success Criteria

### Completion Checklist

- [ ] All internal packages use structured logging via Config.Logger
- [ ] All commands inject and use logger via config
- [ ] User-facing UI unchanged (colors, tables, banners)
- [ ] --debug flag enables DEBUG logs
- [ ] --quiet flag suppresses INFO logs
- [ ] --log-file flag writes logs to file
- [ ] --json flag outputs JSON format
- [ ] All tests pass with mock loggers
- [ ] LOGGING.md created with usage guide
- [ ] CLAUDE.md updated with logging guidelines
- [ ] No fmt.Printf calls for internal operations (only UI)
- [ ] Build succeeds: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Manual integration testing successful

### Before Refactor

- Zero structured logging
- No debug capabilities
- No log aggregation support
- Difficult troubleshooting

### After Refactor

- Structured logs everywhere
- Debug mode for troubleshooting
- JSON format for log aggregation
- Clear separation of UI vs logs
- Easy to add new log contexts
- Testable logging behavior
- Complete documentation for contributors

---

Plan complete and saved to `docs/plans/LOGGING_REFACTOR_PLAN.md`.

**Two execution options:**

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**
