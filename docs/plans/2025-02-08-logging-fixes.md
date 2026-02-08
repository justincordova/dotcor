# Logging Refactor Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix critical logging issues identified in code review: logger lifecycle, output destination, error handling, field name consistency, and test coverage

**Architecture:** Fix logger initialization order, switch logs to stderr, standardize field names, fix quiet mode, add comprehensive tests

**Tech Stack:** Go 1.21+, log/slog, Cobra, testing package

---

## Overview

This plan fixes 8 categories of issues identified in production-grade code review:

1. **HIGH - Logger Lifecycle**: Configure logger before loading config
2. **HIGH - stderr Output**: Change default log output from stdout to stderr
3. **HIGH - JSON + Log File**: Preserve JSON format when writing to file
4. **MEDIUM - Quiet Mode**: Suppress only INFO (show WARN and ERROR)
5. **MEDIUM - Field Names**: Standardize to "file", "src", "dst", "repo"
6. **HIGH - Error Duplication**: Log and return simple errors
7. **HIGH - Test Coverage**: Add logging tests to core packages
8. **LOW - Duration Tracking**: Add duration_ms to key operations

---

### Task 1: Fix Logger Lifecycle in main.go

**Files:**
- Modify: `cmd/dotcor/main.go:72-100`
- Modify: `cmd/dotcor/init.go:362-391`
- Modify: `cmd/dotcor/add.go:46-77`

**Step 1: Update runRoot to configure logger first**

Open `cmd/dotcor/main.go` and replace the `runRoot` function (lines 72-100):

```go
func runRoot(cmd *cobra.Command, args []string) {
	printBanner()

	// Configure logger FIRST, before loading config
	defaultCfg, err := config.NewDefaultConfig()
	if err != nil {
		fmt.Printf("  %s[!] Failed to create default config%s\n", colorYellow, colorReset)
		return
	}
	configureLogger(cmd, defaultCfg)

	// Now load config with logger available
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = defaultCfg
		fmt.Printf("  %s[!] Not initialized%s\n", colorYellow, colorReset)
		fmt.Println()
		fmt.Printf("  %sGet started:%s\n", colorDim, colorReset)
		fmt.Println("    dotcor init          Initialize DotCor")
		fmt.Println("    dotcor --help        Show all commands")
		fmt.Println()
		return
	}

	// Transfer logger to loaded config
	cfg.Logger = defaultCfg.Logger

	// Show quick status
	showQuickStatus(cfg)
}
```

**Step 2: Update runAdd to configure logger early**

Open `cmd/dotcor/add.go` and update the logger configuration (lines 46-77):

```go
func runAdd(cmd *cobra.Command, args []string) error {
	category, _ := cmd.Flags().GetString("category")
	force, _ := cmd.Flags().GetBool("force")
	recursive, _ := cmd.Flags().GetBool("recursive")
	isTemplate, _ := cmd.Flags().GetBool("template")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// Configure logger early for operations
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// ... rest of function continues unchanged ...
```

**Step 3: Update runInit to configure logger early**

Open `cmd/dotcor/init.go` and update `runInit` function (lines 54-98):

```go
func runInit(cmd *cobra.Command, args []string) error {
	applyFlag, _ := cmd.Flags().GetBool("apply")
	interactiveFlag, _ := cmd.Flags().GetBool("interactive")

	// Configure logger early
	defaultCfg, err := config.NewDefaultConfig()
	if err != nil {
		return fmt.Errorf("creating default config: %w", err)
	}
	configureLogger(cmd, defaultCfg)

	// Check symlink support first
	supported, err := fs.SupportsSymlinks()
	if err != nil {
		return fmt.Errorf("checking symlink support: %w", err)
	}
	if !supported {
		// ... existing error handling ...
		return fmt.Errorf("symlinks not supported")
	}

	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	// Check if already initialized
	if fs.PathExists(configDir) && !applyFlag {
		fmt.Printf("DotCor is already initialized at %s\n", configDir)
		fmt.Println("Use 'dotcor status' to check current state.")
		fmt.Println("Use 'dotcor init --apply' to create symlinks from existing config.")
		return nil
	}

	// Acquire lock
	if err := core.AcquireLock(defaultCfg); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer core.ReleaseLock(defaultCfg)

	// ... rest of function continues unchanged, using defaultCfg instead of cfg for logger ...
```

**Step 4: Verify compilation**

Run: `go build ./cmd/dotcor/`
Expected: Success with no errors

**Step 5: Commit**

```bash
git add cmd/dotcor/main.go cmd/dotcor/add.go cmd/dotcor/init.go
git commit -m "fix: configure logger before loading config to prevent nil pointer panics"
```

---

### Task 2: Change Default Log Output to stderr

**Files:**
- Modify: `internal/logger/logger.go:18-40`

**Step 1: Update ConfigureFromFlags to use stderr**

Open `internal/logger/logger.go` and replace lines 18-40:

```go
func ConfigureFromFlags(cmd *cobra.Command) *slog.Logger {
	debug, _ := cmd.Flags().GetBool("debug")
	quiet, _ := cmd.Flags().GetBool("quiet")
	logFile, _ := cmd.Flags().GetString("log-file")
	jsonFormat, _ := cmd.Flags().GetBool("json")

	level := levelFromFlags(debug, quiet)

	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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

	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
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
```

**Step 2: Verify logs go to stderr**

Run: `go build ./... && ./dotcor add ~/.nonexistent 2>test.log 1>/dev/null; cat test.log`
Expected: log output in test.log, stdout empty

**Step 3: Commit**

```bash
git add internal/logger/logger.go
git commit -m "fix: send logs to stderr instead of stdout to avoid polluting CLI output"
```

---

### Task 3: Fix JSON + Log File Combination

**Files:**
- Modify: `internal/logger/logger.go:41-51`

**Step 1: Preserve JSON format when writing to file**

Open `internal/logger/logger.go` and replace lines 41-51:

```go
	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: level,
			}))
		}
		
		// Preserve JSON format if requested
		if jsonFormat {
			handler = slog.NewJSONHandler(file, &slog.HandlerOptions{
				Level: level,
			})
		} else {
			handler = slog.NewTextHandler(file, &slog.HandlerOptions{
				Level: level,
			})
		}
	}
```

**Step 2: Test JSON + log file**

Run: `go build ./... && ./dotcor --json --log-file=/tmp/dotcor.log add ~/.zshrc && cat /tmp/dotcor.log`
Expected: JSON format in log file, not text

**Step 3: Commit**

```bash
git add internal/logger/logger.go
git commit -m "fix: preserve JSON format when writing to log file"
```

---

### Task 4: Fix Quiet Mode to Suppress Only INFO

**Files:**
- Modify: `internal/logger/logger.go:56-65`

**Step 1: Update levelFromFlags to suppress only INFO**

Open `internal/logger/logger.go` and replace lines 56-65:

```go
func levelFromFlags(debug, quiet bool) slog.Level {
	switch {
	case debug:
		return slog.LevelDebug
	case quiet:
		return slog.LevelWarn  // Show WARN and ERROR, suppress INFO and DEBUG
	default:
		return slog.LevelInfo
	}
}
```

**Step 2: Test quiet mode shows warnings**

Run: `go build ./... && ./dotcor --quiet add ~/.largefile 2>&1 | grep -i warn`
Expected: WARN messages still appear in quiet mode

**Step 3: Update documentation**

Open `docs/LOGGING.md` and update line 44-48:

```markdown
### --quiet
Suppress INFO and DEBUG messages (show only WARN and ERROR).

```bash
dotcor add ~/.zshrc --quiet
```

Useful for scripting or when you only want to see warnings and errors.
```

**Step 4: Verify documentation**

Run: `go build ./... && ./dotcor --help | grep quiet`
Expected: Help text matches new behavior

**Step 5: Commit**

```bash
git add internal/logger/logger.go docs/LOGGING.md
git commit -m "fix: quiet mode suppresses only INFO, keeps WARN and ERROR visible"
```

---

### Task 5: Standardize Field Names

**Files:**
- Modify: `internal/core/validator.go:56, 69, 84, 90, 107, 111, 173, 177, 183, 188, 192`
- Modify: `internal/core/backup.go:39, 43, 48, 91, 95, 112, 123, 137`

**Step 1: Update validator.go to use consistent "file" field**

Open `internal/core/validator.go` and make these replacements:

Line 56: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("validating source file", "file", path)
```

Line 61: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to expand path", "file", path, "error", err)
```

Line 69: Change `"path", path` to `"file", path`
```go
cfg.Logger.Error("file does not exist", "file", path)
```

Line 72: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to check file", "file", path, "error", err)
```

Line 78: Change `"path", path` to `"file", path`
```go
cfg.Logger.Warn("path is a directory", "file", path)
```

Line 84: Change `"path", path` to `"file", path`
```go
cfg.Logger.Error("file is not readable", "file", path)
```

Line 90: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("file is in dotcor directory", "file", path, "error", err)
```

Line 97: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to check symlink", "file", path, "error", err)
```

Line 103: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to check symlink target", "file", path, "error", err)
```

Line 107: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("file already managed by dotcor", "file", path)
```

Line 111: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("file is symlink pointing elsewhere", "file", path)
```

Line 115: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("validation complete", "file", path, "valid", true)
```

Line 173: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("validating file size", "file", path)
```

Line 177: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to expand path for validation", "file", path, "error", err)
```

Line 183: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Error("failed to get file info", "file", path, "error", err)
```

Line 188: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("file size check", "file", path, "size", size, "threshold", LargeFileThreshold)
```

Line 192: Change `"path", path` to `"file", path`
```go
cfg.Logger.Warn("file is very large", "file", path, "size_mb", sizeMB)
```

Line 239: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("running all validations", "file", path)
```

Line 259: Change `"path", path, "error", err` to `"file", path, "error", err`
```go
cfg.Logger.Warn("secret detection failed, skipping", "file", path, "error", err)
```

Line 262: Change `"path", path` to `"file", path`
```go
cfg.Logger.Warn("potential secrets detected", "file", path, "count", len(secretWarnings))
```

Line 267: Change `"path", path` to `"file", path`
```go
cfg.Logger.Debug("all validations complete", "file", path, "warnings", len(warnings))
```

**Step 2: Update backup.go to use consistent "file" field**

Open `internal/core/backup.go` and make these replacements:

Line 39: Already uses `"file"` - no change needed

Line 43: Already uses `"file"` - no change needed

Line 48: Already uses `"file"` - no change needed

Line 91: Already uses `"file"` - no change needed

Line 95: Already uses `"backup"` and `"target"` - no change needed

Line 105: Already uses `"backup"` and `"target"` - no change needed

Line 112: Already uses `"backup"` - no change needed

Line 123: Already uses `"backup"` - no change needed

Line 137: Already uses `"backup"` and `"target"` - no change needed

**Step 3: Update fs.go to use consistent "src" and "dst" fields**

Open `internal/fs/fs.go` and verify:

Line 13: `"src", src, "dst", dst` - ✅ Correct
Line 25: `"src", src, "dst", dst, "error", err` - ✅ Correct
Line 44: `"src", src, "dst", dst` - ✅ Correct

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success with no errors

**Step 5: Commit**

```bash
git add internal/core/validator.go
git commit -m "refactor: standardize log field names to use 'file' consistently"
```

---

### Task 6: Fix Error Logging Duplication Pattern

**Files:**
- Modify: `internal/core/validator.go:61-62, 69-70, 72-73, 84-85, 90-91, 97-98, 103-104`
- Modify: `internal/core/backup.go:42-44, 48-49, 55-56, 69-70, 86-87, 91-92, 112-113, 118-119, 123-124, 128-129, 133-134`

**Step 1: Update validator.go to log without wrapping errors**

Open `internal/core/validator.go` and update these sections:

Lines 60-62: Remove error wrapping in log message
```go
if err != nil {
	cfg.Logger.Error("failed to expand path", "file", path, "error", err)
	return fmt.Errorf("invalid path: %w", err)
}
```

Lines 68-70: Log at DEBUG level instead of ERROR for validation failure
```go
if os.IsNotExist(err) {
	cfg.Logger.Debug("validation failed: file does not exist", "file", path)
	return fmt.Errorf("file does not exist: %s", path)
}
```

Lines 72-73: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: cannot check file", "file", path, "error", err)
return fmt.Errorf("checking file: %w", err)
```

Lines 84-85: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: file not readable", "file", path)
return fmt.Errorf("file is not readable: %s", path)
```

Lines 90-91: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: file in dotcor directory", "file", path, "error", err)
return err
```

Lines 97-98: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: cannot check symlink", "file", path, "error", err)
return fmt.Errorf("checking symlink: %w", err)
```

Lines 103-104: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: cannot check symlink target", "file", path, "error", err)
return fmt.Errorf("checking symlink target: %w", err)
```

Lines 177-178: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: cannot expand path", "file", path, "error", err)
return fmt.Errorf("expanding path: %w", err)
```

Lines 183-184: Log at DEBUG level
```go
cfg.Logger.Debug("validation failed: cannot get file info", "file", path, "error", err)
return fmt.Errorf("getting file info: %w", err)
```

**Step 2: Update backup.go to log errors without duplication**

Open `internal/core/backup.go` and update these sections:

Lines 42-44: Keep as-is (actual error, not validation)

Lines 47-49: Keep as-is (actual error)

Lines 55-56: Keep as-is (actual error)

Lines 68-70: Keep as-is (actual error)

Lines 86-87: Keep as-is (actual error)

Lines 91-92: Keep as-is (actual error)

Lines 112-113: Keep as-is (actual error)

Lines 118-119: Keep as-is (actual error)

Lines 123-124: Keep as-is (actual error)

Lines 128-129: Keep as-is (actual error)

Lines 133-134: Keep as-is (actual error)

**Step 3: Update transaction.go to log rollback without wrapping**

Open `internal/core/transaction.go` and update:

Line 45: Remove error wrapping
```go
cfg.Logger.Error("transaction execute failed", "error", fmt.Errorf("already committed"))
```
Change to:
```go
cfg.Logger.Error("transaction execute failed", "error", "already committed")
```

**Step 4: Verify compilation**

Run: `go build ./...`
Expected: Success with no errors

**Step 5: Test error logging**

Run: `go build ./... && ./dotcor add /nonexistent 2>&1 | grep "validation failed"`
Expected: DEBUG logs for validation failures, not ERROR

**Step 6: Commit**

```bash
git add internal/core/validator.go internal/core/transaction.go
git commit -m "refactor: log validation failures at DEBUG level, avoid duplicate error logging"
```

---

### Task 7: Add Comprehensive Logging Tests

**Files:**
- Create: `internal/core/backup_logging_test.go`
- Create: `internal/core/transaction_logging_test.go`
- Create: `internal/fs/fs_logging_test.go`
- Modify: `internal/logger/logger_test.go`

**Step 1: Write backup logging test**

Create `internal/core/backup_logging_test.go`:

```go
package core

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justincordova/dotcor/internal/config"
)

func TestCreateBackupLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	sourcePath := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(sourcePath, []byte("test content"), 0644)

	backupPath, err := CreateBackup(sourcePath, cfg)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if backupPath == "" {
		t.Fatal("expected non-empty backup path")
	}

	output := buf.String()

	// Verify DEBUG logs
	if !containsLog(output, "DEBUG creating backup") {
		t.Error("expected DEBUG log for 'creating backup'")
	}

	// Verify INFO logs
	if !containsLog(output, "INFO backup created") {
		t.Error("expected INFO log for 'backup created'")
	}

	// Verify structured fields
	if !containsLog(output, "file="+sourcePath) {
		t.Error("expected 'file' field in logs")
	}
}

func TestCreateBackupWithDuration(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	sourcePath := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(sourcePath, []byte("test content"), 0644)

	start := time.Now()
	backupPath, err := CreateBackup(sourcePath, cfg)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	if backupPath == "" {
		t.Fatal("expected non-empty backup path")
	}

	t.Logf("Backup created in %v", duration)
}

func TestRestoreBackupLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	// Create source file
	sourcePath := filepath.Join(t.TempDir(), "source.txt")
	os.WriteFile(sourcePath, []byte("source content"), 0644)

	// Create backup
	backupPath, err := CreateBackup(sourcePath, cfg)
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	// Clear buffer
	buf.Reset()

	// Restore backup
	targetPath := filepath.Join(t.TempDir(), "target.txt")
	err = RestoreBackup(backupPath, targetPath, cfg)
	if err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	output := buf.String()

	// Verify DEBUG logs
	if !containsLog(output, "DEBUG restoring from backup") {
		t.Error("expected DEBUG log for 'restoring from backup'")
	}

	// Verify INFO logs
	if !containsLog(output, "INFO backup restored") {
		t.Error("expected INFO log for 'backup restored'")
	}
}

func containsLog(output, substring string) bool {
	return bytes.Contains([]byte(output), []byte(substring))
}
```

**Step 2: Write transaction logging test**

Create `internal/core/transaction_logging_test.go`:

```go
package core

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

func TestTransactionExecuteLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	tx := NewTransaction(cfg)

	// Create a simple test operation
	testOp := &testOperation{
		shouldFail: false,
		description: "test operation",
	}

	err := tx.Execute(testOp)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()

	// Verify DEBUG log for operation execution
	if !containsLog(output, "DEBUG executing operation") {
		t.Error("expected DEBUG log for 'executing operation'")
	}

	// Verify DEBUG log for operation success
	if !containsLog(output, "DEBUG operation executed successfully") {
		t.Error("expected DEBUG log for 'operation executed successfully'")
	}

	// Verify operation description in log
	if !containsLog(output, "test operation") {
		t.Error("expected operation description in logs")
	}
}

func TestTransactionRollbackLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	tx := NewTransaction(cfg)

	// Create a failing operation
	failingOp := &testOperation{
		shouldFail: true,
		description: "failing operation",
	}

	// Execute a successful operation first
	testOp := &testOperation{
		shouldFail: false,
		description: "successful operation",
	}
	tx.Execute(testOp)

	// Clear buffer
	buf.Reset()

	// Execute failing operation to trigger rollback
	err := tx.Execute(failingOp)
	if err == nil {
		t.Fatal("expected error from failing operation")
	}

	output := buf.String()

	// Verify WARN log for rollback
	if !containsLog(output, "WARN rolling back transaction") {
		t.Error("expected WARN log for 'rolling back transaction'")
	}

	// Verify DEBUG log for rolling back operation
	if !containsLog(output, "DEBUG rolling back operation") {
		t.Error("expected DEBUG log for 'rolling back operation'")
	}

	// Verify ERROR log for operation failure
	if !containsLog(output, "ERROR operation failed") {
		t.Error("expected ERROR log for 'operation failed'")
	}
}

// testOperation is a simple test operation
type testOperation struct {
	shouldFail bool
	description string
	executed   bool
	undone     bool
}

func (op *testOperation) Do() error {
	op.executed = true
	if op.shouldFail {
		return &testError{msg: "operation failed"}
	}
	return nil
}

func (op *testOperation) Undo() error {
	op.undone = true
	return nil
}

func (op *testOperation) Describe() string {
	return op.description
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
```

**Step 3: Write fs logging test**

Create `internal/fs/fs_logging_test.go`:

```go
package fs

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

func TestMoveFileLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	src := filepath.Join(t.TempDir(), "source.txt")
	dst := filepath.Join(t.TempDir(), "dest.txt")
	os.WriteFile(src, []byte("test content"), 0644)

	err := MoveFile(src, dst, cfg)
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	output := buf.String()

	// Verify DEBUG logs
	if !containsLogFS(output, "DEBUG moving file") {
		t.Error("expected DEBUG log for 'moving file'")
	}

	// Verify INFO logs
	if !containsLogFS(output, "INFO file moved") {
		t.Error("expected INFO log for 'file moved'")
	}

	// Verify structured fields
	if !containsLogFS(output, "src="+src) {
		t.Error("expected 'src' field in logs")
	}
	if !containsLogFS(output, "dst="+dst) {
		t.Error("expected 'dst' field in logs")
	}
}

func TestEnsureDirLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	dir := filepath.Join(t.TempDir(), "newdir")

	err := EnsureDir(dir, cfg)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	output := buf.String()

	// Verify DEBUG logs
	if !containsLogFS(output, "DEBUG ensuring directory exists") {
		t.Error("expected DEBUG log for 'ensuring directory exists'")
	}

	// Verify directory created log
	if !containsLogFS(output, "DEBUG directory created") {
		t.Error("expected DEBUG log for 'directory created'")
	}

	// Verify structured field
	if !containsLogFS(output, "path="+dir) {
		t.Error("expected 'path' field in logs")
	}
}

func TestRemoveFileLogging(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	cfg := &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}

	filePath := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(filePath, []byte("test"), 0644)

	err := RemoveFile(filePath, cfg)
	if err != nil {
		t.Fatalf("RemoveFile failed: %v", err)
	}

	output := buf.String()

	// Verify DEBUG logs
	if !containsLogFS(output, "DEBUG removing file") {
		t.Error("expected DEBUG log for 'removing file'")
	}

	if !containsLogFS(output, "DEBUG file removed") {
		t.Error("expected DEBUG log for 'file removed'")
	}

	// Verify structured field
	if !containsLogFS(output, "path="+filePath) {
		t.Error("expected 'path' field in logs")
	}
}

func containsLogFS(output, substring string) bool {
	return bytes.Contains([]byte(output), []byte(substring))
}
```

**Step 4: Update existing logger tests**

Open `internal/logger/logger_test.go` and add new test:

```go
func TestReplaceAttr(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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
	logger := slog.New(handler)

	logger.Info("test", "source_path", "~/.zshrc", "repo_path", "shell/zshrc")

	output := buf.String()

	if !strings.Contains(output, "src=~/.zshrc") {
		t.Errorf("expected field shortening, got: %s", output)
	}

	if !strings.Contains(output, "repo=shell/zshrc") {
		t.Errorf("expected field shortening, got: %s", output)
	}

	// Verify original field names are not present
	if strings.Contains(output, "source_path") {
		t.Errorf("field not shortened: got: %s", output)
	}
}
```

**Step 5: Run all logging tests**

Run: `go test -v ./internal/core/... -run "Logging" ./internal/fs/... -run "Logging" ./internal/logger/...`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/core/backup_logging_test.go internal/core/transaction_logging_test.go internal/fs/fs_logging_test.go internal/logger/logger_test.go
git commit -m "test: add comprehensive logging tests for core packages"
```

---

### Task 8: Add Duration Tracking to Key Operations

**Files:**
- Modify: `internal/core/backup.go:36-100`
- Modify: `internal/fs/fs.go:12-40`

**Step 1: Add duration to CreateBackup**

Open `internal/core/backup.go` and update `CreateBackup` function (lines 36-100):

```go
func CreateBackup(sourcePath string, cfg *config.Config) (string, error) {
	start := time.Now()
	cfg.Logger.Debug("creating backup", "file", sourcePath)

	expanded, err := config.ExpandPath(sourcePath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand path", "file", sourcePath, "error", err)
		return "", fmt.Errorf("expanding source path: %w", err)
	}

	if !fs.PathExists(expanded) {
		cfg.Logger.Error("source file does not exist", "file", sourcePath)
		return "", fmt.Errorf("source file does not exist: %s", sourcePath)
	}

	// Get backup directory
	backupDir, err := GetBackupDir()
	if err != nil {
		cfg.Logger.Error("failed to get backup directory", "error", err)
		return "", err
	}

	// Create timestamped subdirectory
	timestamp := time.Now().Format(TimestampFormat)
	timestampDir := filepath.Join(backupDir, timestamp)

	// Check if path exists and is a file (not directory)
	if info, err := os.Stat(timestampDir); err == nil && !info.IsDir() {
		return "", fmt.Errorf("backup path exists as file, not directory: %s", timestampDir)
	}

	if err := fs.EnsureDir(timestampDir, cfg); err != nil {
		cfg.Logger.Error("failed to create backup directory", "error", err)
		return "", fmt.Errorf("creating backup directory: %w", err)
	}

	// Normalize source path and use as relative path in backup to preserve uniqueness
	normalized, err := config.NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	// Strip leading ~ and convert to relative path for storage
	// e.g., ~/.zshrc -> zshrc, ~/.config/nvim/init.lua -> config/nvim/init.lua
	backupRelativePath := strings.TrimPrefix(normalized, "~/")
	backupPath := filepath.Join(timestampDir, backupRelativePath)

	// Ensure parent directory exists
	if err := fs.EnsureDir(filepath.Dir(backupPath), cfg); err != nil {
		cfg.Logger.Error("failed to create backup subdirectory", "error", err)
		return "", fmt.Errorf("creating backup subdirectory: %w", err)
	}

	if err := fs.CopyWithPermissions(expanded, backupPath, cfg); err != nil {
		cfg.Logger.Error("failed to copy to backup", "src", expanded, "dst", backupPath, "error", err)
		return "", fmt.Errorf("copying to backup: %w", err)
	}

	cfg.Logger.Info("backup created",
		"file", sourcePath,
		"backup", backupPath,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return backupPath, nil
}
```

**Step 2: Add duration to RestoreBackup**

Open `internal/core/backup.go` and update `RestoreBackup` function (lines 103-143):

```go
func RestoreBackup(backupPath, targetPath string, cfg *config.Config) error {
	start := time.Now()
	cfg.Logger.Debug("restoring from backup",
		"backup", backupPath,
		"target", targetPath,
	)

	expandedBackup, err := config.ExpandPath(backupPath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand backup path", "error", err)
		return fmt.Errorf("expanding backup path: %w", err)
	}

	expandedTarget, err := config.ExpandPath(targetPath, cfg)
	if err != nil {
		cfg.Logger.Error("failed to expand target path", "error", err)
		return fmt.Errorf("expanding target path: %w", err)
	}

	if !fs.PathExists(expandedBackup) {
		cfg.Logger.Error("backup file does not exist", "path", backupPath)
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	if err := fs.EnsureDir(filepath.Dir(expandedTarget), cfg); err != nil {
		cfg.Logger.Error("failed to create target directory", "error", err)
		return fmt.Errorf("creating target directory: %w", err)
	}

	if err := fs.CopyWithPermissions(expandedBackup, expandedTarget, cfg); err != nil {
		cfg.Logger.Error("failed to restore from backup", "src", expandedBackup, "dst", expandedTarget, "error", err)
		return fmt.Errorf("restoring from backup: %w", err)
	}

	cfg.Logger.Info("backup restored",
		"backup", backupPath,
		"target", targetPath,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}
```

**Step 3: Add duration to MoveFile**

Open `internal/fs/fs.go` and update `MoveFile` function (lines 12-41):

```go
func MoveFile(src, dst string, cfg *config.Config) error {
	start := time.Now()
	cfg.Logger.Debug("moving file", "src", src, "dst", dst)

	if err := EnsureDir(filepath.Dir(dst), cfg); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	err := os.Rename(src, dst)
	if err == nil {
		cfg.Logger.Info("file moved",
			"src", src,
			"dst", dst,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil
	}

	cfg.Logger.Debug("rename failed, trying copy", "src", src, "dst", dst, "error", err)
	if err := CopyWithPermissions(src, dst, cfg); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	dstExisted := PathExists(dst)
	if err := os.Remove(src); err != nil {
		if !dstExisted {
			os.Remove(dst)
		}
		cfg.Logger.Error("failed to remove original file", "error", err)
		return fmt.Errorf("removing original file: %w", err)
	}

	cfg.Logger.Info("file moved",
		"src", src,
		"dst", dst,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}
```

**Step 4: Test duration tracking**

Run: `go build ./... && ./dotcor --debug add ~/.zshrc 2>&1 | grep duration_ms`
Expected: duration_ms field appears in logs

**Step 5: Update documentation**

Open `docs/LOGGING.md` and add to line 285-295:

```markdown
### Duration Tracking

Key operations now log execution time in milliseconds:

```bash
$ dotcor add ~/.zshrc --debug
2026/02/08 12:30:00 INFO backup created file=~/.zshrc backup=/Users/user/.dotcor/backups/... duration_ms=42
2026/02/08 12:30:00 INFO file moved src=~/.zshrc dst=~/.dotcor/files/shell/zshrc duration_ms=15
```

Duration helps identify performance bottlenecks and slow operations.
```

**Step 6: Verify compilation**

Run: `go build ./...`
Expected: Success with no errors

**Step 7: Commit**

```bash
git add internal/core/backup.go internal/fs/fs.go docs/LOGGING.md
git commit -m "feat: add duration_ms field to backup and file operations"
```

---

## Final Verification Steps

### Task 9: Full Integration Test

**Files:**
- No files created/modified

**Step 1: Build and run smoke test**

Run: `go build ./cmd/dotcor/ && ./dotcor --version`
Expected: Version output without errors

**Step 2: Test all log levels**

Run: `./dotcor --debug status 2>&1 | grep DEBUG | head -5`
Expected: DEBUG log output visible

Run: `./dotcor status 2>&1 | grep DEBUG`
Expected: No DEBUG output (default level is INFO)

Run: `./dotcor --quiet status 2>&1 | grep INFO`
Expected: No INFO output in quiet mode

**Step 3: Test stderr output**

Run: `./dotcor --debug status 1>/dev/null | wc -l`
Expected: Lines output to stderr (not captured by /dev/null)

**Step 4: Test JSON + log file**

Run: `./dotcor --json --log-file=/tmp/test.log status && head -1 /tmp/test.log | jq .`
Expected: Valid JSON in log file

**Step 5: Run full test suite**

Run: `go test ./... -v`
Expected: All tests pass, including new logging tests

**Step 6: Run linters**

Run: `go vet ./...`
Expected: No warnings

**Step 7: Commit verification**

```bash
git add .
git commit -m "test: verify all logging fixes work correctly"
```

---

## Summary

This plan addresses all 8 categories of logging issues identified in the code review:

1. ✅ Logger lifecycle fixed - configure before loading config
2. ✅ stderr output - logs no longer pollute stdout
3. ✅ JSON + log file - preserves format correctly
4. ✅ Quiet mode - suppresses only INFO, keeps WARN/ERROR
5. ✅ Field names - standardized to "file", "src", "dst"
6. ✅ Error duplication - validation failures at DEBUG level
7. ✅ Test coverage - comprehensive tests for core packages
8. ✅ Duration tracking - added to key operations

Total: 9 tasks, ~25 commits, ~4 hours of work
