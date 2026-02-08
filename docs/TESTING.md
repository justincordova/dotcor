# DotCor Testing Guide

This guide explains testing practices, conventions, and best practices for DotCor.

## Overview

DotCor follows a comprehensive testing strategy focusing on:

1. **Testify framework** - Use `github.com/stretchr/testify` v1.11.1 for all assertions
2. **AAA Pattern** - All tests follow Arrange-Act-Assert structure with section comments
3. **Critical path coverage** - Prioritize core packages and CLI commands
4. **Pre-commit validation** - Build and tests must pass before any commit
5. **New feature testing** - All new features and changes require corresponding tests

### Coverage Goals

| Package | Target Coverage |
|---------|-----------------|
| config  | 85%            |
| core    | 90%            |
| fs      | 85%            |
| git     | 80%            |
| commands| 75%            |
| overall | 85%            |

## Testing Philosophy

### Test-Driven Development (TDD)

DotCor follows Test-Driven Development (TDD) to ensure safety, reliability, and confidence in every commit. Since DotCor manipulates user dotfiles and system state, a bug can corrupt configuration files, leave broken symlinks, create incomplete transactions, or lose user data. TDD catches these issues early, before they affect real user systems.

**TDD Approach:**

1. **Write failing test first** - Define expected behavior
2. **Run test** - Confirm it fails
3. **Implement minimum code** - Make test pass
4. **Refactor** - Clean up while keeping tests passing

This approach forces you to think about API and edge cases before writing implementation code.

### Test for New Features

**MANDATORY:** When adding new features or making significant changes:

1. Write tests before implementing (TDD)
2. Ensure tests cover:
   - Happy path (normal operation)
   - Error paths (edge cases, invalid inputs)
   - Integration with other components

3. Example workflow:
   ```
   - Implement new command
   - Write command_test.go with comprehensive coverage
   - Write integration tests in tests/integration/
   - Update docs/TESTING.md if new patterns emerge
   ```

## AAA Pattern

Every test must have three clearly marked sections with comments:

```go
func TestFunctionName_Scenario_ExpectedResult(t *testing.T) {
    // Arrange
    // - Set up test data
    // - Create temp directories
    // - Initialize dependencies
    input := "test input"
    expected := "expected output"

    // Act
    // - Call the function being tested
    // - Capture results
    result, err := FunctionUnderTest(input)

    // Assert
    // - Verify results using testify assertions
    // - Check error conditions
    // - Verify side effects
    assert.NoError(t, err, "FunctionUnderTest should not return error")
    assert.Equal(t, expected, result, "result should match expected")
}
```

### Why AAA Matters

- **Arrange** is clear about what's being tested
- **Act** shows exactly what's being called
- **Assert** validates expected behavior
- Comments make test intent explicit
- Easier to understand and maintain

### Basic Example

```go
func TestNormalizePath(t *testing.T) {
    // Arrange
    home, err := os.UserHomeDir()
    require.NoError(t, err, "failed to get home dir")

    testPath := filepath.Join(home, ".zshrc")
    want := "~/.zshrc"

    // Act
    got, err := NormalizePath(testPath)

    // Assert
    require.NoError(t, err, "NormalizePath() error")
    assert.Equal(t, want, got, "NormalizePath() result")
}
```

### Table-Driven Test Example

```go
func TestValidateRepoPath(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {
            name:    "valid relative path",
            path:    "shell/zshrc",
            wantErr: false,
        },
        {
            name:    "absolute path",
            path:    "/shell/zshrc",
            wantErr: true,
        },
        {
            name:    "path traversal",
            path:    "../shell/zshrc",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            testPath := tt.path

            // Act
            err := ValidateRepoPath(testPath)

            // Assert
            if tt.wantErr {
                assert.Error(t, err, "should reject invalid path")
            } else {
                assert.NoError(t, err, "should accept valid path")
            }
        })
    }
}
```

## Go Test Organization

### Idiomatic Structure

**CRITICAL:** Follow Go best practices for test organization:

1. **Unit tests live alongside code they test**

```
internal/core/validator.go        → internal/core/validator_test.go
cmd/dotcor/add.go             → cmd/dotcor/add_test.go
```

Why?
- Access to unexported identifiers (critical in Go)
- Accurate coverage attribution
- Refactors don't break imports
- Idiomatic and expected by Go developers

2. **Integration tests live separately**

```
tests/integration/
├── init_add_list_test.go
├── remove_restore_test.go
└── sync_git_test.go
```

Why?
- Real workflows, slower, fewer tests
- Exercise entire system
- Don't need access to unexported

3. **Helpers stay local**

```
cmd/dotcor/test_helpers.go     # For command tests
```

Why?
- Belong with code they help test
- Not globally shared across unrelated packages

4. **Fixtures go in testdata/**

```
testdata/fixtures/zshrc
testdata/fixtures/bashrc
```

Why?
- Go treats testdata/ specially
- Ignored by go list
- Safe for binaries

### What Goes Where

| Test Type | Location | Notes |
|-----------|-----------|-------|
| Unit tests | `internal/*/filename_test.go` | Next to code being tested |
| Command tests | `cmd/dotcor/*_test.go` | Next to command file |
| Integration tests | `tests/integration/*_test.go` | Only tests here |
| Test helpers | `cmd/dotcor/test_helpers.go` | Local to package |
| Fixtures | `testdata/fixtures/*` | Static test data |

### Project Structure

```
dotcor/
├── cmd/dotcor/          # CLI commands (Cobra)
│   ├── main.go
│   ├── init.go          # Initialize repository
│   ├── init_test.go     # Tests for init command
│   ├── add.go           # Add files to management
│   ├── add_test.go      # Tests for add command
│   ├── remove.go        # Remove from management
│   ├── remove_test.go   # Tests for remove command
│   ├── list.go          # List managed files
│   ├── list_test.go     # Tests for list command
│   ├── status.go        # Show status
│   ├── status_test.go   # Tests for status command
│   ├── sync.go          # Git sync
│   ├── sync_test.go     # Tests for sync command
│   ├── restore.go      # Restore files
│   ├── restore_test.go  # Tests for restore command
│   └── test_helpers.go  # Helper functions for command tests
│
├── internal/
│   ├── config/          # Configuration management
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── paths.go
│   │   ├── paths_test.go
│   │   └── migrate.go
│   │
│   ├── core/            # Business logic (validator, transaction, backup, lock, etc.)
│   │   ├── transaction.go
│   │   ├── transaction_test.go
│   │   ├── validator.go
│   │   ├── validator_test.go
│   │   ├── backup.go
│   │   ├── backup_test.go
│   │   ├── lock.go
│   │   ├── lock_test.go
│   │   ├── hooks.go
│   │   ├── hooks_test.go
│   │   ├── ignore.go
│   │   ├── ignore_test.go
│   │   └── templates.go
│   │
│   ├── fs/              # File system operations, symlink handling
│   │   ├── symlink.go
│   │   ├── symlink_test.go
│   │   ├── fs.go
│   │   └── fs_test.go
│   │
│   └── git/             # Git wrapper
│       ├── git.go
│       └── git_test.go
│
├── tests/
│   └── integration/     # Integration tests only
│       ├── init_add_list_test.go
│       ├── remove_restore_test.go
│       └── sync_git_test.go
│
├── testdata/
│   └── fixtures/       # Static test data
│       ├── zshrc
│       └── bashrc
│
└── docs/
    ├── TESTING.md        # Testing documentation
    └── PLANS/
        └── TESTING_PLAN.md
```

### Common Mistakes to Avoid

❌ **Do NOT:**
- Put unit tests in a separate `tests/` directory
- Name files `*_test_helpers.go` (use `test_helpers.go`)
- Share helpers across unrelated packages
- Test multiple commands in one file
- Use global mutable state in tests
- Depend on test execution order

✅ **Do:**
- Put `*_test.go` next to the code it tests
- Keep helpers local to their package
- Use `t.TempDir()` for test isolation
- Make each test independent
- Use `t.Helper()` in helper functions

## Testify Assertions Reference

DotCor uses **testify v1.11.1** for enhanced testing capabilities.

### testify/assert Package

Provides rich assertion functions with descriptive messages. Use `assert` when you want to check multiple things and continue execution even on failure.

```go
import "github.com/stretchr/testify/assert"

func TestNormalizePath(t *testing.T) {
    got := NormalizePath("~/.zshrc")
    want := filepath.Join(home, ".zshrc")

    assert.Equal(t, want, got, "NormalizePath() result")
    assert.NoError(t, err, "NormalizePath() error")
    assert.NotNil(t, result, "result should not be nil")
    assert.FileExists(t, path, "file should exist")
    assert.True(t, isSymlink, "should be a symlink")
    assert.Contains(t, output, "expected text", "output should contain text")
    assert.Error(t, err, "should return error")
}
```

### testify/require Package

Fails test immediately (no more execution) on assertion failure. Use `require` when subsequent code depends on this check.

```go
import "github.com/stretchr/testify/require"

func TestFileOperations(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testFile := filepath.Join(tempDir, "test.txt")

    // Critical: fail immediately if setup fails
    err := os.WriteFile(testFile, []byte("content"), 0644)
    require.NoError(t, err, "failed to create test file")

    // Safe to proceed - testFile is guaranteed to exist
    content, err := os.ReadFile(testFile)
    require.NoError(t, err, "failed to read test file")
}
```

**When to use each:**
- `require.NoError`: File setup, critical dependencies, nil checks before dereferencing
- `assert.NoError`: Optional operations, multiple checks, non-critical failures

### Common Assertions

| Assertion | Usage | Description |
|-----------|--------|-------------|
| `assert.Equal(t, exp, act)` | `assert.Equal(t, want, got)` | Exact match |
| `assert.NoError(t, err)` | `assert.NoError(t, err)` | No error expected |
| `assert.Error(t, err)` | `assert.Error(t, err)` | Error expected |
| `assert.ErrorIs(t, err, target)` | `assert.ErrorIs(t, err, SomeErr)` | Specific error type |
| `assert.True(t, cond, msg)` | `assert.True(t, isSymlink, "should be symlink")` | Boolean with message |
| `assert.False(t, cond, msg)` | `assert.False(t, cond, "should not be symlink")` | Boolean negation |
| `assert.Contains(t, slice, item)` | `assert.Contains(t, files, "test.txt")` | Slice membership |
| `assert.NotContains(t, slice, item)` | `assert.NotContains(t, files, "bad.txt")` | Slice not contains |
| `assert.Len(t, slice, n)` | `assert.Len(t, files, 2)` | Slice length |
| `assert.Nil(t, obj)` | `assert.Nil(t, obj)` | Object is nil |
| `assert.NotNil(t, obj)` | `assert.NotNil(t, obj)` | Object is not nil |
| `assert.FileExists(t, path)` | `assert.FileExists(t, "/tmp/file.txt")` | File exists |
| `assert.DirExists(t, path)` | `assert.DirExists(t, "/tmp/dir")` | Directory exists |
| `assert.Empty(t, collection)` | `assert.Empty(t, slice)` | Collection is empty |
| `assert.NotEmpty(t, collection)` | `assert.NotEmpty(t, slice)` | Collection is not empty |

## Test Naming Convention

Use this pattern for test names:

```
Test<Function>_<Scenario>_<ExpectedResult>
```

### Examples

Good names:
- `TestValidateRepoPath_ValidPath_ReturnsNil`
- `TestCreateBackup_NonexistentFile_ReturnsError`
- `TestTransactionExecute_Failure_RollsBack`

Bad names:
- `TestValidateRepoPath1` (unclear)
- `TestCreateBackupError` (doesn't describe scenario)
- `TestTransaction` (too broad)

### For Methods

For methods, use this format:
```
Test<Struct>_<Method>_<Scenario>_<ExpectedResult>
```

```go
func TestTransaction_Execute_Success_Commits(t *testing.T) { }
func TestTransaction_Execute_Failure_RollsBack(t *testing.T) { }
func TestTransaction_Rollback_Empty_NoOp(t *testing.T) { }
```

## Test Structure

### Test Isolation

Each test must be independent:

- Use `t.TempDir()` for test directories (auto-cleanup)
- Don't depend on other tests
- Clean up resources manually if needed
- Ensure tests are repeatable

```go
func TestFileOperation(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()  // Auto-cleanup after test
    testFile := filepath.Join(tempDir, "test.txt")

    // Act & Assert...
}
```

**Benefits:**
- Automatic cleanup: Even if test fails or panics
- Unique directory: No race conditions between parallel tests
- Safe path: No conflicts with other tests

### Table-Driven Tests

Use table-driven tests for multiple similar scenarios:

```go
func TestValidateRepoPath(t *testing.T) {
    tests := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {
            name:    "valid relative path",
            path:    "shell/zshrc",
            wantErr: false,
        },
        {
            name:    "absolute path",
            path:    "/shell/zshrc",
            wantErr: true,
        },
        {
            name:    "path traversal",
            path:    "../shell/zshrc",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            testPath := tt.path

            // Act
            err := ValidateRepoPath(testPath)

            // Assert
            if tt.wantErr {
                assert.Error(t, err, "should reject invalid path")
            } else {
                assert.NoError(t, err, "should accept valid path")
            }
        })
    }
}
```

### Subtests

Use `t.Run()` for related test scenarios:

```go
func TestSomeFunction(t *testing.T) {
    t.Run("with valid input", func(t *testing.T) {
        // test case
    })

    t.Run("with invalid input", func(t *testing.T) {
        // test case
    })

    t.Run("with edge case", func(t *testing.T) {
        // test case
    })
}
```

## Test Helpers

### Command Test Helpers

For command-level tests, use helpers in `cmd/dotcor/test_helpers.go`:

```go
// Create test config with temp directory
cfg := CreateTestConfig(t)

// Create test file with content
CreateTestFile(t, "/tmp/file.txt", "content")

// Assert file exists
AssertFileExists(t, "/tmp/file.txt")

// Assert file does not exist
AssertFileNotExists(t, "/tmp/nonexistent.txt")

// Assert file content matches
AssertFileContent(t, "/tmp/file.txt", "expected content")

// Assert directory exists
AssertDirExists(t, "/tmp/dir")
```

### Using t.Helper()

Always mark helper functions with `t.Helper()` for accurate error reporting:

```go
func CreateTestConfig(t *testing.T) *config.Config {
    t.Helper()  // This makes error show correct line in calling test

    tempDir := t.TempDir()
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    return &config.Config{
        Logger:         logger,
        RepoPath:       filepath.Join(tempDir, "files"),
        GitEnabled:     false,
        IgnorePatterns: []string{},
        ManagedFiles:   []config.ManagedFile{},
    }
}

func CreateTestFile(t *testing.T, path, content string) {
    t.Helper()

    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        t.Fatalf("failed to create directory: %v", err)
    }

    if err := os.WriteFile(path, []byte(content), 0644); err != nil {
        t.Fatalf("failed to create file: %v", err)
    }
}

func AssertFileExists(t *testing.T, path string) {
    t.Helper()

    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        t.Errorf("file does not exist: %s", path)
        return
    }
    if err != nil {
        t.Errorf("failed to stat file %s: %v", path, err)
        return
    }
    if info.IsDir() {
        t.Errorf("path is a directory, not a file: %s", path)
    }
}

func AssertFileNotExists(t *testing.T, path string) {
    t.Helper()

    _, err := os.Stat(path)
    if err == nil {
        t.Errorf("file exists but should not: %s", path)
        return
    }
    if !os.IsNotExist(err) {
        t.Errorf("unexpected error checking file %s: %v", path, err)
    }
}

func AssertDirExists(t *testing.T, path string) {
    t.Helper()

    info, err := os.Stat(path)
    if os.IsNotExist(err) {
        t.Errorf("directory does not exist: %s", path)
        return
    }
    if err != nil {
        t.Errorf("failed to stat directory %s: %v", path, err)
        return
    }
    if !info.IsDir() {
        t.Errorf("path is a file, not a directory: %s", path)
    }
}

func AssertFileContent(t *testing.T, path, expectedContent string) {
    t.Helper()

    content, err := os.ReadFile(path)
    if err != nil {
        t.Fatalf("failed to read file %s: %v", path, err)
    }

    if string(content) != expectedContent {
        t.Errorf("file content mismatch\n  got: %q\n  want: %q", string(content), expectedContent)
    }
}
```

### Custom Assertions

Create reusable assertions for common checks:

```go
// AssertFileIsSymlink checks that a file is a valid symlink
func AssertFileIsSymlink(t *testing.T, path string) {
    t.Helper()
    isSymlink, err := fs.IsSymlink(path)
    assert.NoError(t, err, "should check if file is symlink")
    assert.True(t, isSymlink, "file should be a symlink")
}

// AssertSymlinkTarget checks that symlink points to expected target
func AssertSymlinkTarget(t *testing.T, symlink, expectedTarget string) {
    t.Helper()
    target, err := os.Readlink(symlink)
    assert.NoError(t, err, "should read symlink")
    assert.Contains(t, target, expectedTarget, "symlink should point to target")
}
```

## Testing Commands

### Run All Tests

```bash
go test ./... -v
```

### Run Specific Package

```bash
go test ./internal/core/... -v
go test ./cmd/dotcor/... -v
```

### Run Specific Test

```bash
go test ./internal/core/... -v -run TestValidateRepoPath
go test ./internal/core/... -v -run TestTransactionExecute/Fails
```

### Run Tests with Coverage

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

### Generate Coverage Report

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Run with Verbose Output

```bash
go test ./... -v
```

### Run with Race Detection

```bash
go test -race ./...
```

### Run Integration Tests Only

```bash
go test ./tests/... -v
```

## Pre-Commit Workflow

**MANDATORY:** Every commit must follow this workflow:

```bash
# Step 1: Build
go build ./...
# If this fails, fix errors before continuing

# Step 2: Run tests
go test ./...
# If this fails, fix tests before committing

# Step 3: Commit (only if both above succeed)
git add .
git commit -m "feat: add feature"
```

One-liner:
```bash
go build ./... && go test ./... && git commit -m "message"
```

### Why Both Steps?

- `go build` catches compilation errors that tests might miss (unused imports, missing packages, type errors)
- `go test` catches logical errors, broken behavior, regressions, and edge cases

### Why This Matters

- Prevents broken commits
- Catches compilation errors early
- Ensures tests always pass
- Maintains code quality

### Git Hook (Optional)

Add a pre-commit hook to enforce this automatically:

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running pre-commit checks..."

if ! go build ./...; then
    echo "❌ Build failed - commit aborted"
    exit 1
fi

if ! go test ./...; then
    echo "❌ Tests failed - commit aborted"
    exit 1
fi

echo "✓ All checks passed"
```

## Test Coverage

### Coverage Goals

| Package | Target Coverage |
|---------|-----------------|
| config  | 85%            |
| core    | 90%            |
| fs      | 85%            |
| git     | 80%            |
| commands| 75%            |
| overall | 85%            |

### Check Coverage

```bash
# View coverage percentage
go test ./... -cover

# Detailed coverage by function
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# Find low-coverage files
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | sort -k3 -n | head -20
```

## Testing Checklist

Before submitting code, ensure:

### Code Quality
- [ ] Build succeeds: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Code follows project conventions
- [ ] No commented-out code

### Test Quality
- [ ] Test uses AAA pattern with section comments
- [ ] Test uses testify assertions (not manual checks)
- [ ] Test name follows naming convention
- [ ] Happy path tested
- [ ] Error paths tested
- [ ] Edge cases tested
- [ ] Test is independent (doesn't depend on other tests)
- [ ] Test uses `t.Helper()` in helper functions

### Coverage
- [ ] New code has tests
- [ ] Coverage meets targets for package
- [ ] No obvious gaps in coverage

## Best Practices

### DO ✅

- Use testify assertions for clarity
- Add AAA section comments
- Write descriptive test names
- Use `t.TempDir()` for isolation
- Test both happy and error paths
- Keep tests simple and focused
- Use table-driven tests for multiple cases
- Clean up resources (or use t.TempDir)
- Write tests for new features before implementation
- Mark helpers with `t.Helper()`

### DON'T ❌

- Skip tests without good reason
- Use `t.Fatal()` in helper functions (use `t.Helper()`)
- Write tests that depend on execution order
- Add unnecessary complexity
- Test multiple concerns in one test
- Use `time.Sleep()` for synchronization
- Hardcode paths (use temp directories)
- Put unit tests in separate `tests/` directory
- Share helpers across unrelated packages
- Ignore error returns

### require.NoError for Fatal Errors

Use `require.NoError` when subsequent code depends on success:

```go
func TestProcessFile(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testFile := filepath.Join(tempDir, "test.txt")

    // Fatal: stop test if setup fails
    err := os.WriteFile(testFile, []byte("content"), 0644)
    require.NoError(t, err, "failed to create test file")

    // Safe to proceed - testFile is guaranteed to exist
    content, err := os.ReadFile(testFile)
    require.NoError(t, err, "failed to read test file")

    assert.Equal(t, "content", string(content))
}
```

### assert.NoError for Non-Fatal Errors

Use `assert.NoError` when you want to continue checking even on failure:

```go
func TestMultipleOperations(t *testing.T) {
    // Check first operation
    err1 := Operation1()
    assert.NoError(t, err1, "Operation1() error")

    // Check second operation (even if first failed)
    err2 := Operation2()
    assert.NoError(t, err2, "Operation2() error")

    // Both errors reported if present
}
```

### Consistent Error Messages

Use format strings for consistent, informative error messages:

```go
// Good: Descriptive, includes function name
assert.NoError(t, err, "CreateFile(%s) error", path)
assert.Equal(t, want, got, "NormalizePath(%s) result", input)
assert.FileExists(t, path, "backup file should exist at %s", path)

// Bad: Generic
assert.NoError(t, err)
assert.Equal(t, want, got)
```

## Common Patterns

### Testing File Operations

```go
func TestFileOperation(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testFile := filepath.Join(tempDir, "test.txt")
    content := []byte("test content")

    // Act
    err := os.WriteFile(testFile, content, 0644)

    // Assert
    assert.NoError(t, err, "should create test file")
    assert.FileExists(t, testFile, "file should exist")

    readContent, err := os.ReadFile(testFile)
    assert.NoError(t, err, "should read test file")
    assert.Equal(t, content, readContent, "content should match")
}
```

### Testing Error Handling

```go
func TestErrorHandling(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "valid", false},
        {"empty input", "", true},
        {"invalid input", "invalid", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            input := tt.input

            // Act
            result, err := Process(input)

            // Assert
            if tt.wantErr {
                assert.Error(t, err, "should return error for %s", tt.name)
            } else {
                assert.NoError(t, err, "should not return error for %s", tt.name)
                assert.NotEmpty(t, result, "should return result")
            }
        })
    }
}
```

### Testing Transactions

```go
func TestTransactionExecute_Failure_RollsBack(t *testing.T) {
    // Arrange
    cfg := &config.Config{Logger: slog.Default()}
    tx := NewTransaction(cfg)
    op1 := &mockOperation{}
    op2 := &mockOperation{doErr: errors.New("operation failed")}

    // Act - First operation succeeds
    if err := tx.Execute(op1); err != nil {
        t.Fatalf("First Execute() error = %v", err)
    }
    err := tx.Execute(op2)

    // Assert
    assert.Error(t, err, "Execute should return error when operation fails")
    assert.Equal(t, 1, op1.undoCalls, "Rollback should have called Undo() on op1")
}
```

### Testing Symlink Operations

```go
func TestSymlinkCreation(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    targetFile := filepath.Join(tempDir, "target")
    symlinkFile := filepath.Join(tempDir, "symlink")

    if err := os.WriteFile(targetFile, []byte("content"), 0644); err != nil {
        t.Fatalf("failed to create target: %v", err)
    }

    // Act
    err := os.Symlink("target", symlinkFile)

    // Assert
    assert.NoError(t, err, "should create symlink")
    assert.FileExists(t, symlinkFile, "symlink should exist")

    isSymlink, err := fs.IsSymlink(symlinkFile)
    assert.NoError(t, err, "should check if symlink")
    assert.True(t, isSymlink, "should be symlink")
}
```

### Testing Config Operations

```go
func TestConfigLoad(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    configPath := filepath.Join(tempDir, "config.yaml")
    configContent := `
version: 1
repo_path: ~/.dotcor/files
git_enabled: false
managed_files: []
`
    if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
        t.Fatalf("failed to write config: %v", err)
    }

    // Act
    cfg, err := config.Load(configPath)

    // Assert
    assert.NoError(t, err, "should load config")
    assert.Equal(t, "1", cfg.Version, "version should match")
    assert.Equal(t, "~/.dotcor/files", cfg.RepoPath, "repo path should match")
}
```

### File Operations with Error Checking

```go
func TestCopyFile(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    src := filepath.Join(tempDir, "source.txt")
    dst := filepath.Join(tempDir, "dest.txt")
    content := []byte("test content")

    err := os.WriteFile(src, content, 0644)
    require.NoError(t, err, "failed to create source file")

    // Act
    err = CopyFile(src, dst)

    // Assert
    assert.NoError(t, err, "CopyFile() error")
    assert.FileExists(t, src, "source should still exist")
    assert.FileExists(t, dst, "destination should be created")

    // Verify content preserved
    got, err := os.ReadFile(dst)
    require.NoError(t, err, "failed to read destination")
    assert.Equal(t, content, got, "content should be preserved")
}
```

### Error Handling and Recovery

```go
func TestTransaction_Rollback(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := &config.Config{Logger: slog.Default()}
    tx := NewTransaction(cfg)

    file1 := filepath.Join(tempDir, "file1.txt")
    file2 := filepath.Join(tempDir, "file2.txt")
    err := os.WriteFile(file1, []byte("content1"), 0644)
    require.NoError(t, err, "failed to create file1")

    // Act - Execute first operation
    op1 := &CopyFileOp{Src: file1, Dst: filepath.Join(tempDir, "copy1.txt"), Config: cfg}
    err = tx.Execute(op1)
    require.NoError(t, err, "first operation should succeed")

    // Act - Execute failing operation
    failingOp := &mockOperation{doErr: errors.New("failed")}
    err = tx.Execute(failingOp)

    // Assert - Transaction should have rolled back
    assert.Error(t, err, "second operation should fail")
    assert.NoFileExists(t, filepath.Join(tempDir, "copy1.txt"), "rolled back file should not exist")
}
```

### Secret Detection

```go
func TestSecretDetection(t *testing.T) {
    tests := []struct {
        name    string
        content string
        wantErr bool
    }{
        {
            name:    "normal content",
            content: "just normal text",
            wantErr: false,
        },
        {
            name:    "API key",
            content: "export API_KEY=sk-1234567890abcdef",
            wantErr: true,
        },
        {
            name:    "AWS access key",
            content: "aws_access_key_id=AKIAIOSFODNN7EXAMPLE",
            wantErr: true,
        },
        {
            name:    "password",
            content: "password=secret123",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := DetectSecrets(tt.content)
            if tt.wantErr {
                assert.Error(t, err, "should detect secret")
            } else {
                assert.NoError(t, err, "should not detect secret")
            }
        })
    }
}
```

## Debugging Tests

### Verbose Output

```bash
go test ./... -v
```

### Run Specific Test with Debugging

```bash
go test ./internal/core/... -v -run TestValidateRepoPath/Valid
```

### Use Test Log

```go
t.Logf("Testing with input: %v", input)
t.Log("Got result:", result)
```

### Print Variables (for debugging only)

```go
// For debugging only - remove before committing
fmt.Printf("DEBUG: result = %+v\n", result)
```

## Integration Testing

Integration tests live in `tests/integration/` and test complete workflows:

```go
func TestIntegration_InitAddListRemove(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    homeDir := filepath.Join(tempDir, "home")
    repoDir := filepath.Join(tempDir, "dotcor")

    // Act
    // ... perform init, add, list, remove operations

    // Assert
    // ... verify end-to-end behavior
}
```

Integration tests should:
- Test realistic user workflows
- Exercise multiple components together
- Be slower but fewer in number than unit tests
- Use real filesystem operations (temp dirs)

## Mocking

For external dependencies, consider creating mock operations:

```go
// mockOperation is a simple operation for testing
type mockOperation struct {
    doErr     error
    undoErr   error
    doCalls   int
    undoCalls int
}

func (m *mockOperation) Do() error {
    m.doCalls++
    return m.doErr
}

func (m *mockOperation) Undo() error {
    m.undoCalls++
    return m.undoErr
}

func (m *mockOperation) Describe() string {
    return "mock operation"
}
```

## Common Pitfalls

### 1. Not Using t.TempDir()

❌ Bad:
```go
func TestSomething(t *testing.T) {
    tempDir := "/tmp/test-" + uuid.New().String()
    os.MkdirAll(tempDir, 0755)
    defer os.RemoveAll(tempDir)  // Manual cleanup
}
```

✅ Good:
```go
func TestSomething(t *testing.T) {
    tempDir := t.TempDir()  // Auto-cleanup
}
```

### 2. Not Marking Helpers with t.Helper()

❌ Bad:
```go
func CreateTestFile(t *testing.T, path, content string) {
    os.WriteFile(path, []byte(content), 0644)
}
```

✅ Good:
```go
func CreateTestFile(t *testing.T, path, content string) {
    t.Helper()  // Shows correct line in error messages
    os.WriteFile(path, []byte(content), 0644)
}
```

### 3. Testing Multiple Concerns

❌ Bad:
```go
func TestAddAndRemove(t *testing.T) {
    // Tests both add and remove in one test
}
```

✅ Good:
```go
func TestAdd_SingleFile_Success(t *testing.T) {
    // Tests only add
}

func TestRemove_SingleFile_Success(t *testing.T) {
    // Tests only remove
}
```

### 4. Ignoring Error Returns

❌ Bad:
```go
os.WriteFile(path, content, 0644)  // Ignoring error
```

✅ Good:
```go
if err := os.WriteFile(path, content, 0644); err != nil {
    t.Fatalf("failed to create file: %v", err)
}
```

## Summary Checklist

Before marking a feature as complete, ensure:

- [ ] Unit tests exist for all public functions
- [ ] Error cases are tested
- [ ] Edge cases are covered
- [ ] Integration tests validate workflow
- [ ] Tests use `t.TempDir()` for file operations
- [ ] Test helpers are marked with `t.Helper()`
- [ ] Tests follow AAA pattern with comments
- [ ] Pre-commit workflow passes: `go build && go test`
- [ ] Coverage meets target (85%+ for core packages)

## Resources

- [testify documentation](https://github.com/stretchr/testify)
- [Go testing package](https://pkg.go.dev/testing)
- [Table-driven tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Go blog: testing patterns](https://go.dev/doc/tutorial/add-a-test)
- [DotCor Development Guide](../CLAUDE.md)

## For Implementation Details

See `docs/plans/TESTING_PLAN.md` for:
- Detailed implementation steps
- Task-by-task testing plan
- Checklist for comprehensive testing coverage
