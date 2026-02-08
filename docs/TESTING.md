# Testing Documentation

## Testing Philosophy

DotCor follows Test-Driven Development (TDD) to ensure safety, reliability, and confidence in every commit. Since DotCor manipulates user dotfiles and system state, a bug can corrupt configuration files, leave broken symlinks, create incomplete transactions, or lose user data. TDD catches these issues early, before they affect real user systems.

### TDD Approach

When adding new features to DotCor:

1. **Write the failing test first** - Define the expected behavior
2. **Run the test** - Confirm it fails
3. **Implement the minimum code** - Make the test pass
4. **Refactor** - Clean up while keeping tests passing

This approach forces you to think about the API and edge cases before writing implementation code.

---

## Testify Framework

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

Fails the test immediately (no more execution) on assertion failure. Use `require` when subsequent code depends on this check.

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

---

## AAA Pattern

DotCor uses the **Arrange-Act-Assert (AAA)** pattern for all tests with explicit section comments. This provides clear separation of setup, execution, and verification.

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
func TestNormalizePath(t *testing.T) {
    // Arrange
    home, err := os.UserHomeDir()
    require.NoError(t, err, "failed to get home dir")

    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {
            name:  "tilde expansion",
            input: "~/.zshrc",
            want:  filepath.Join(home, ".zshrc"),
        },
        {
            name:  "absolute path",
            input: "/usr/local/bin",
            want:  "/usr/local/bin",
        },
        {
            name:    "empty path",
            input:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Act
            got, err := NormalizePath(tt.input)

            // Assert
            if tt.wantErr {
                assert.Error(t, err, "NormalizePath() should return error")
            } else {
                assert.NoError(t, err, "NormalizePath() should not return error")
                assert.Equal(t, tt.want, got, "NormalizePath() result")
            }
        })
    }
}
```

### Error Handling Example

```go
func TestCreateFile(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    testFile := filepath.Join(tempDir, "test.txt")

    // Act
    err := CreateFile(testFile, []byte("content"))

    // Assert
    assert.NoError(t, err, "CreateFile() error")
    assert.FileExists(t, testFile, "file should exist")
}
```

---

## Test Naming Conventions

### Test Function Names

```go
// Format: Test<Function>_<Scenario>_<ExpectedResult>
func TestNormalizePath_Tilde_ExpandsToHome(t *testing.T) { }
func TestNormalizePath_Absolute_ReturnsUnchanged(t *testing.T) { }
func TestNormalizePath_Empty_ReturnsError(t *testing.T) { }

// For methods: Test<Struct>_<Method>_<Scenario>_<ExpectedResult>
func TestTransaction_Execute_Success_Commits(t *testing.T) { }
func TestTransaction_Execute_Failure_RollsBack(t *testing.T) { }
func TestTransaction_Rollback_Empty_NoOp(t *testing.T) { }
```

### Table-Driven Tests

Use table-driven tests for multiple scenarios. Subtest names should be descriptive:

```go
tests := []struct {
    name string
    input string
    want string
    wantErr bool
}{
    {
        name: "valid zshrc file",
        input: "~/.zshrc",
        want: "/home/user/.zshrc",
    },
    {
        name: "file with API key should fail",
        input: "~/.env",
        wantErr: true,
    },
    {
        name: "nested config path",
        input: "~/.config/myapp/config.yml",
        want: "/home/user/.config/myapp/config.yml",
    },
}
```

---

## Test Organization

### Unit Tests: Next to Code

Unit tests are placed in `*_test.go` files next to the implementation:

```
internal/
├── config/
│   ├── config.go
│   ├── config_test.go      # Unit tests for config package
│   ├── paths.go
│   └── paths_test.go       # Unit tests for paths functions
├── core/
│   ├── transaction.go
│   ├── transaction_test.go  # Unit tests for transaction logic
│   ├── backup.go
│   └── backup_test.go
└── fs/
    ├── fs.go
    └── fs_test.go
```

### Integration Tests: tests/integration/

End-to-end tests that exercise complete workflows:

```
tests/
└── integration/
    ├── init_workflow_test.go
    ├── add_workflow_test.go
    └── sync_workflow_test.go
```

### Command Tests: cmd/dotcor/

CLI command tests:

```
cmd/dotcor/
├── main.go
├── add.go
├── add_test.go            # Command integration tests
├── remove.go
├── remove_test.go
└── test_helpers.go        # Shared helpers for command tests
```

### Test Helpers: Local to Package

Each package has its own `test_helpers.go` for shared utilities:

```go
// cmd/dotcor/test_helpers.go
package main

import "testing"

func CreateTestConfig(t *testing.T) *config.Config {
    t.Helper()
    // Implementation
}
```

---

## Pre-Commit Workflow

Before committing any code, **you must verify**:

```bash
# Step 1: Build the project
go build ./...

# Step 2: Run all tests
go test ./...

# Step 3: Only commit if both pass
git commit -m "feat: add new feature"
```

### Why Both Steps?

- `go build` catches compilation errors that tests might miss (unused imports, missing packages, type errors)
- `go test` catches logical errors, broken behavior, regressions, and edge cases

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

---

## Best Practices

### Table-Driven Tests

Use table-driven tests for multiple scenarios:

```go
func TestValidateFile(t *testing.T) {
    tests := []struct {
        name    string
        content string
        wantErr bool
    }{
        {
            name:    "valid file",
            content: "normal content",
            wantErr: false,
        },
        {
            name:    "file with API key",
            content: "api_key=sk-1234567890",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateFile(tt.content)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### t.TempDir() for Isolation

Always use `t.TempDir()` for test files:

```go
func TestFileOperations(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()  // Auto-cleaned after test
    testFile := filepath.Join(tempDir, "test.txt")

    // Act
    err := os.WriteFile(testFile, []byte("content"), 0644)

    // Assert
    assert.NoError(t, err)
    // tempDir is automatically deleted when test completes
}
```

**Benefits:**
- Automatic cleanup: Even if test fails or panics
- Unique directory: No race conditions between parallel tests
- Safe path: No conflicts with other tests

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

---

## Common Patterns

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

### Symlink Validation

```go
func TestCreateSymlink(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    target := filepath.Join(tempDir, "target.txt")
    link := filepath.Join(tempDir, "link.txt")

    err := os.WriteFile(target, []byte("content"), 0644)
    require.NoError(t, err, "failed to create target file")

    // Act
    err = os.Symlink(target, link)

    // Assert
    assert.NoError(t, err, "Symlink() error")

    // Verify it's a symlink
    info, err := os.Lstat(link)
    require.NoError(t, err, "failed to stat link")
    assert.True(t, info.Mode()&os.ModeSymlink != 0, "path should be a symlink")

    // Verify it resolves correctly
    resolved, err := os.Readlink(link)
    require.NoError(t, err, "failed to read link")
    assert.Equal(t, target, resolved, "link should point to target")

    // Verify content accessible
    content, err := os.ReadFile(link)
    require.NoError(t, err, "failed to read via link")
    assert.Equal(t, []byte("content"), content, "content should match")
}
```

### Platform Filtering

```go
func TestPlatformSpecific(t *testing.T) {
    runtime.GOOS // "darwin", "linux", "windows", etc.

    if runtime.GOOS == "windows" {
        t.Skip("skipping Unix-specific test on Windows")
    }

    // Unix-specific test code
    // ...
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

### Migration Workflows

```go
func TestConfigMigration(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    oldConfigPath := filepath.Join(tempDir, "config.yaml")
    newConfigPath := filepath.Join(tempDir, "config.yaml")

    oldContent := `
repo_path: ~/.dotcor
git_enabled: true
`
    err := os.WriteFile(oldConfigPath, []byte(oldContent), 0644)
    require.NoError(t, err, "failed to create old config")

    // Act
    cfg, err := LoadConfig(oldConfigPath)
    require.NoError(t, err, "LoadConfig() error")

    // Assert - Migration should have occurred
    assert.Equal(t, "1.0.0", cfg.Version, "version should be migrated")
    assert.True(t, cfg.GitEnabled, "git_enabled should be preserved")
}
```

---

## Running Tests

### Run All Tests

```bash
go test ./...
```

### Run Specific Package

```bash
go test ./internal/config/...
```

### Run Specific Test

```bash
go test -run TestNormalizePath ./internal/config/...
```

### Run with Verbose Output

```bash
go test -v ./...
```

### Run with Coverage

```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Run with Race Detection

```bash
go test -race ./...
```

---

## Summary Checklist

Before marking a feature as complete, ensure:

- [ ] Unit tests exist for all public functions
- [ ] Error cases are tested
- [ ] Edge cases are covered
- [ ] Integration tests validate the workflow
- [ ] Tests use `t.TempDir()` for file operations
- [ ] Test helpers are marked with `t.Helper()`
- [ ] Tests follow AAA pattern with comments
- [ ] Pre-commit workflow passes: `go build && go test`
- [ ] Coverage meets target (85%+ for core packages)

---

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [DotCor Development Guide](../CLAUDE.md)
