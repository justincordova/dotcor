# Comprehensive Testing Plan for DotCor

**Date:** 2026-02-09
**Status:** Draft
**Target Release:** v0.6.0

## Overview

Create comprehensive unit test coverage for all 14 DotCor commands with edge case testing and selected integration tests for core commands.

**Commands to Test (14 total):**
- init, add, remove, list, status, sync, restore (core - need integration tests)
- diff, history, adopt, doctor, rebuild-config, rebuild-links, clone, cleanup-backups (supporting)

**Coverage Targets:**
- Each command: 75%+ coverage
- Test edge cases, error paths, flag variations
- Integration tests for 7 core commands (end-to-end workflows)

**Testing Approach:**
- **Unit Tests:** Test individual command logic, flags, error handling, edge cases
- **Integration Tests:** Test realistic multi-command workflows (init → add → list → status → sync → restore → remove)
- **Test Structure:** Use AAA pattern, table-driven tests for multiple scenarios, testify framework

## Test Strategy & Patterns

**Test Framework:** Testify v1.11.1 with AAA pattern (Arrange-Act-Assert) and section comments

**Test Patterns:**

1. **Table-Driven Tests** for multiple scenarios:
   - Happy paths, error paths, edge cases
   - Different flag combinations
   - Various input types

2. **Subtests** for related scenarios using `t.Run()`:
   - Each scenario isolated and independent
   - Clear failure identification

3. **Test Helpers** in `cmd/dotcor/test_helpers.go`:
    - `CreateTestConfig(t)` - Create temp config with logger
    - `CreateTestFile(t, path, content)` - Create test file with content
    - `CreateTestSymlink(t, target, link)` - Create symlink
    - `AssertFileExists(t, path)` - File existence assertions
    - `AssertFileNotExists(t, path)` - File non-existence assertions
    - `AssertFileContent(t, path, expected)` - Verify file content
    - `AssertSymlinkPointsTo(t, link, target)` - Symlink validation
    - `RunCommand(t, cmd, args, env)` - Execute dotcor command with environment
    - `CreateTestLogger(t)` - Create logger with buffer for log capture
    - `AssertLogContains(t, buffer, level, msg)` - Verify log emission
    - `SetupGitRepo(t, path)` - Create test git repository
    - `CreateTestConfigFile(t, path, content)` - Create YAML config file
    - All helpers marked with `t.Helper()` for accurate error reporting

4. **Command Execution Pattern:**
   - Build temporary dotcor binary
   - Set up test environment (temp HOME, config)
   - Execute command with arguments
   - Capture stdout/stderr/exit code
   - Verify output and side effects

5. **Error Testing:**
    - Test each error path explicitly
    - Verify error messages are helpful
    - Test graceful degradation (e.g., git failures)

6. **Structured Logging Testing:**
    - Verify logs emit at correct levels (DEBUG, INFO, WARN, ERROR)
    - Verify structured fields are included (op, file, error, etc.)
    - Test log level flags (--debug, --quiet)
    - Capture log output for verification
    - Test JSON log format when applicable
    - Example:
      ```go
      func TestAdd_EmitsCorrectLogs(t *testing.T) {
          var buf bytes.Buffer
          handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
          logger := slog.New(handler)
          cfg := CreateTestConfig(t)
          cfg.Logger = logger

          // Act
          err := AddFile(sourcePath, cfg)

          // Assert
          assert.NoError(t, err)
          output := buf.String()
          assert.Contains(t, output, "validating source file")
          assert.Contains(t, output, "backup created")
          assert.Contains(t, output, "symlink created")
      }
      ```

## Command-Specific Test Coverage

### Core Commands (with integration tests):

**1. init**
- Happy path: Creates directories, initializes config, sets up git repo
- Error paths: Permission denied, disk full, invalid path
- Flags: `--apply` creates symlinks, `--force` overwrites existing
- Integration: init → verify structure

**2. add**
- Single file, multiple files, directory (recursive), glob patterns
- Validation: File exists, not secret, not already managed
- Error paths: Permission denied, file not found, secret detection, backup failure
- Flags: `--category`, `--dry-run`, `--force`
- Integration: init → add → verify symlink and config

**3. remove**
- Single file, multiple files, directory
- Error paths: File not managed, permission denied, symlink removal fails
- Flags: `--backup`, `--force`
- Integration: init → add → remove → verify removal

**4. list**
- No files, single file, multiple files
- Flags: `--json`, `--verbose`, `--category filter`
- Integration: init → add → list → verify output

**5. status**
- Not initialized, healthy system, pending changes
- Flags: `--verbose`, `--json`
- Integration: init → add → status → verify state

**6. sync**
- No changes, local changes, remote changes
- Error paths: Git not installed, no remote, network failure
- Flags: `--no-push`, `--no-commit`, `--pull-only`
- Integration: init → add → sync → verify commit

**7. restore**
- Head (latest), specific commit, by file
- Error paths: Invalid commit, file not managed, permission denied
- Flags: `--commit`, `--file`, `--dry-run`
- Integration: init → add → modify → restore → verify restored

### Supporting Commands:

**8. diff**
- No changes, local changes, specific file
- Flags: `--file`, `--verbose`
- Error paths: Not initialized, invalid file

**9. history**
- All files, specific file, limit output
- Flags: `--file`, `--limit`, `--oneline`
- Error paths: Not initialized, invalid file

**10. adopt**
- Valid symlink, not a symlink, already managed
- Error paths: Invalid symlink, permission denied
- Flags: `--force`

**11. doctor**
- Healthy system, broken symlink, stale lock, missing git
- Flags: `--fix`, `--verbose`
- Error paths: None (doctor runs in degraded state)

**12. rebuild-config**
- Valid repo, empty repo, scan flag
- Flags: `--scan`, `--dry-run`
- Error paths: Invalid config, not initialized

**13. rebuild-links**
- Template files, no templates, specific files
- Flags: `--dry-run`, `--verbose`
- Error paths: Invalid template, permission denied

**14. clone**
- Valid URL, invalid URL, existing directory
- Error paths: Network failure, invalid repo
- Flags: `--branch`, `--depth`

## Implementation Structure

**File Organization:**

```
cmd/dotcor/
├── test_helpers.go          # Enhanced with new helpers
├── add_test.go              # Expand to 600+ lines
├── remove_test.go           # Expand to 400+ lines
├── list_test.go             # Expand to 350+ lines
├── status_test.go           # Expand to 400+ lines
├── sync_test.go             # Expand to 500+ lines
├── restore_test.go          # Expand to 450+ lines
├── diff_test.go             # Expand to 400+ lines
├── history_test.go          # Expand to 400+ lines
├── adopt_test.go            # Expand to 400+ lines
├── doctor_test.go           # Expand to 500+ lines
├── rebuild_test.go          # Expand to 400+ lines
├── rebuild-links_test.go    # Expand to 400+ lines
├── clone_test.go            # Expand to 400+ lines
├── cleanup_test.go          # Expand to 400+ lines
└── init_test.go             # Expand to 500+ lines

tests/integration/
├── core_workflow_test.go    # New: init → add → list → status
├── sync_workflow_test.go    # New: add → sync → verify
├── restore_workflow_test.go # New: add → modify → restore → verify
└── remove_workflow_test.go   # New: add → remove → verify

**Integration Test Details:**

1. **Core Workflow (core_workflow_test.go):**
   - Test init command creates proper structure
   - Add multiple files (shell, nvim, git configs)
   - List command shows all managed files
   - Status command shows healthy state
   - Verify symlinks created correctly
   - Verify config updated with all files
   - Verify git repo initialized with commits

2. **Sync Workflow (sync_workflow_test.go):**
   - Initialize repo with files
   - Make local changes to managed file
   - Run sync with commit verification
   - Test sync with remote (mock git push)
   - Test `--no-push` flag
   - Test `--no-commit` flag
   - Verify git history contains commits

3. **Restore Workflow (restore_workflow_test.go):**
   - Add file and create initial commit
   - Modify file content
   - Restore to head (latest commit)
   - Verify file content restored
   - Test restore to specific commit
   - Test `--dry-run` flag
   - Verify restore doesn't break symlinks

4. **Remove Workflow (remove_workflow_test.go):**
   - Add multiple files
   - Remove single file
   - Verify backup created (with `--backup` flag)
   - Verify symlink removed
   - Verify config updated
   - Test remove directory
   - Test remove with force flag

**Integration Test Assertions:**
   - Each step verifies state before proceeding
   - End state matches expected configuration
   - All side effects verified (files, symlinks, config, git)
   - Cleanup performed after each test

docs/plans/
└── 2026-02-09-comprehensive-testing-plan.md  # This plan
```

**Test File Structure Pattern:**

```go
package main

// Imports (testing, testify, os, filepath, etc.)

// Helper functions (if command-specific)

// Happy Path Tests
func Test<Command>_<Scenario>_<ExpectedResult>(t *testing.T) { }
func Test<Command>_<Scenario2>_<ExpectedResult2>(t *testing.T) { }

// Error Path Tests
func Test<Command>_<ErrorScenario>_ReturnsError(t *testing.T) { }
func Test<Command>_<ErrorScenario2>_ReturnsError(t *testing.T) { }

// Flag Variation Tests
func Test<Command>_Flag_<FlagName>_Behavior(t *testing.T) { }

// Structured Logging Tests
func Test<Command>_EmitsCorrectLogs(t *testing.T) { }
func Test<Command>_LogLevels_Verified(t *testing.T) { }

// Table-Driven Tests (for multiple scenarios)
func Test<Command>_<Feature>_MultipleScenarios(t *testing.T) {
    tests := []struct { ... }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) { ... })
    }
}
```

**Assertion Strategies:**

1. **File System Assertions:**
   - File exists/not exists: `AssertFileExists`, `AssertFileNotExists`
   - File content: `AssertFileContent(t, path, expected)`
   - Directory structure: Verify directory layout after operations
   - Symlink validation: `AssertSymlinkPointsTo(t, link, expectedTarget)`

2. **Config State Assertions:**
   - Read config file and verify managed files list
   - Check config version and fields
   - Verify config saves correctly after operations

3. **Git State Assertions:**
   - Verify git repo initialized: Check `.git` directory exists
   - Verify commits created: Use `git log` and check commit messages
   - Verify files committed: Check `git status` shows no uncommitted changes
   - Verify remote configuration: Check `git remote -v`

4. **Log Assertions:**
   - Capture log output to buffer
   - Verify log messages contain expected text
   - Verify logs at correct level (DEBUG/INFO/WARN/ERROR)
   - Verify structured fields present (op, file, error, etc.)

5. **Output Assertions:**
   - Capture stdout/stderr from command execution
   - Verify output contains expected messages
   - Verify output format (JSON, table, etc.)
   - Verify error messages are helpful and actionable

**Test Data Organization:**

```
testdata/
├── fixtures/
│   ├── configs/
│   │   ├── valid-config.yaml
│   │   ├── invalid-config.yaml
│   │   └── empty-config.yaml
│   ├── files/
│   │   ├── zshrc
│   │   ├── bashrc
│   │   └── nvim-init.vim
│   └── templates/
│       ├── zshrc.template
│       └── gitconfig.template
└── expected/
    ├── add-output.txt
    ├── list-json-output.json
    └── status-verbose-output.txt
```

## Test Coverage Metrics & Goals

**Coverage Targets:**

| Command | Current Lines | Target Lines | Target Coverage |
|---------|---------------|--------------|-----------------|
| init | 292 | 500+ | 75%+ |
| add | 387 | 600+ | 75%+ |
| remove | 219 | 400+ | 75%+ |
| list | 185 | 350+ | 75%+ |
| status | 362 | 400+ | 75%+ |
| sync | 297 | 500+ | 75%+ |
| restore | 205 | 450+ | 75%+ |
| diff | 286 | 400+ | 75%+ |
| history | 281 | 400+ | 75%+ |
| adopt | 248 | 400+ | 75%+ |
| doctor | 347 | 500+ | 75%+ |
| rebuild-config | 210 | 400+ | 75%+ |
| rebuild-links | 261 | 400+ | 75%+ |
| clone | 178 | 400+ | 75%+ |
| cleanup | 259 | 400+ | 75%+ |
| **Total** | **4,017** | **7,400+** | **75%+** |

**Quality Metrics:**
- All tests use AAA pattern with section comments
- All test helpers marked with `t.Helper()`
- No flaky tests (deterministic, isolated)
- Clear, descriptive test names
- Table-driven tests for multiple scenarios
- Integration tests for core workflows

**Verification:**
```bash
# Check coverage per package
go test ./cmd/dotcor/... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# Find untested lines (below threshold)
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | awk '$3 < 75.0 {print $0}'

# Coverage by command
go test ./cmd/dotcor/... -coverprofile=cmd_coverage.out
go tool cover -func=cmd_coverage.out | grep -E "(add_test.go|init_test.go|...)"

# Check for untested functions
go test ./... -covermode=count
```

## Implementation Steps

**Phase 1: Preparation & Infrastructure**
1. Soft revert tag v0.5.5 and commit 9d44eef
2. Enhance `cmd/dotcor/test_helpers.go` with new helper functions
3. Create integration test structure in `tests/integration/`
4. Set up coverage tracking and baseline measurements

**Phase 2: Core Command Tests (Priority)**
5. Expand `init_test.go` to 500+ lines (all scenarios)
6. Expand `add_test.go` to 600+ lines (all scenarios)
7. Expand `remove_test.go` to 400+ lines (all scenarios)
8. Expand `list_test.go` to 350+ lines (all scenarios)
9. Expand `status_test.go` to 400+ lines (all scenarios)
10. Expand `sync_test.go` to 500+ lines (all scenarios)
11. Expand `restore_test.go` to 450+ lines (all scenarios)

**Phase 3: Supporting Command Tests**
12. Expand `diff_test.go` to 400+ lines
13. Expand `history_test.go` to 400+ lines
14. Expand `adopt_test.go` to 400+ lines
15. Expand `doctor_test.go` to 500+ lines
16. Expand `rebuild_test.go` to 400+ lines
17. Expand `rebuild-links_test.go` to 400+ lines
18. Expand `clone_test.go` to 400+ lines
19. Expand `cleanup_test.go` to 400+ lines

**Phase 4: Integration Tests**
20. Create `tests/integration/core_workflow_test.go`
21. Create `tests/integration/sync_workflow_test.go`
22. Create `tests/integration/restore_workflow_test.go`
23. Create `tests/integration/remove_workflow_test.go`

**Phase 5: Quality Assurance**
24. Run `golangci-lint run ./...` and fix all issues
25. Verify all tests pass: `go test ./... -v`
26. Run race detector: `go test ./... -race`
27. Generate coverage report and verify 75%+ target
28. Generate HTML coverage report: `go tool cover -html=coverage.out`
29. Review untested code: Identify gaps and add tests
30. Conduct in-depth senior engineer review:
    - Review test quality, clarity, and best practices
    - Verify edge cases and error paths are covered
    - Check test isolation and determinism
    - Ensure AAA pattern consistency
    - Validate helper functions are properly marked
    - Review code organization and naming
    - Verify structured logging tests are comprehensive
    - Check cross-platform considerations
    - Verify performance (test execution time, memory)

**Phase 6: Finalization**
28. Commit changes (atomic commits per command)
29. Create new tag v0.6.0 with comprehensive testing release

## Cross-Platform Testing

**Platform-Specific Considerations:**

1. **Symlink Handling:**
   - macOS/Linux: Native symlink support via `os.Symlink()`
   - Windows: Requires Developer Mode for symlink support
   - Test symlink creation and resolution on all platforms
   - Test relative vs absolute symlinks

2. **Path Handling:**
   - macOS/Linux: Forward slashes `/`
   - Windows: Backslashes `\`
   - Test `filepath.Join()` and `filepath.Rel()` behavior
   - Test path normalization across platforms

3. **Permissions:**
   - Test permission denied errors
   - Test file ownership checks
   - Note: Windows has different permission model

4. **Line Endings:**
   - macOS/Linux: LF (`\n`)
   - Windows: CRLF (`\r\n`)
   - Test file content preservation regardless of line endings

5. **Test Tags for Platform-Specific Tests:**
   ```go
   // +build !windows
   func TestLinuxOnlyFeature(t *testing.T) { }

   // +build darwin
   func TestMacOSOnlyFeature(t *testing.T) { }
   ```

**Platform Testing Strategy:**
   - Primary testing on macOS (development environment)
   - CI tests on Linux (GitHub Actions)
   - Manual testing on Windows (when possible)
   - Use build tags for platform-specific tests

## Performance Testing

**Test Execution Time:**

- Target: Each test runs in < 1 second
- Integration tests: Target < 10 seconds
- Total test suite: Target < 5 minutes

**Large File Handling:**

- Test operations with files > 10MB
- Test directory with 100+ files
- Verify no performance degradation

**Concurrent Operations:**

- Test multiple dotcor operations in parallel
- Verify lock file handling prevents race conditions
- Use `go test -race ./...` to detect race conditions

**Performance Benchmarks:**

```go
func BenchmarkAddFile(b *testing.B) {
    tempDir := b.TempDir()
    sourceFile := filepath.Join(tempDir, "test.txt")
    os.WriteFile(sourceFile, []byte("test content"), 0644)

    cfg := CreateTestConfig(&testing.T{})
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        AddFile(sourceFile, cfg)
    }
}
```

**Memory Usage:**

- Monitor memory usage during tests
- Test with large file sets (1000+ files)
- Verify no memory leaks

## CI/CD Integration

**GitHub Actions Workflow:**

```yaml
name: Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run tests
        run: go test ./... -v -race -coverprofile=coverage.out
      - name: Check coverage
        run: go tool cover -func=coverage.out | grep total
      - name: Run linter
        run: golangci-lint run ./...
```

**Pre-Commit Hook:**

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running pre-commit checks..."

if ! go build ./...; then
    echo "❌ Build failed - commit aborted"
    exit 1
fi

if ! go test ./... -race; then
    echo "❌ Tests failed - commit aborted"
    exit 1
fi

if ! golangci-lint run ./...; then
    echo "❌ Linter failed - commit aborted"
    exit 1
fi

echo "✓ All checks passed"
```

**Test Result Reporting:**

- Parse test results for CI dashboard
- Track coverage trends over time
- Alert on test failures
- Track test execution time

## Test Quality Standards

**Senior Engineer Review Criteria:**

1. **Test Clarity & Readability**
   - Clear, descriptive test names following `Test<Command>_<Scenario>_<ExpectedResult>` pattern
   - AAA pattern with section comments
   - Minimal test logic, clear assertions
   - Helpful failure messages

2. **Test Isolation & Determinism**
   - Each test independent (no dependency on other tests)
   - Use `t.TempDir()` for all file operations
   - No external state dependencies
   - No race conditions

3. **Comprehensive Coverage**
   - Happy paths tested
   - All error paths tested
   - Edge cases covered
   - Flag variations tested
   - Integration workflows validated

4. **Best Practices**
   - Table-driven tests for multiple scenarios
   - `t.Helper()` on all helper functions
   - `require.NoError` for setup, `assert.NoError` for validation
   - No hardcoded paths (use temp directories)
   - No commented-out code

5. **Code Organization**
   - Consistent structure across all test files
   - Helpers organized logically
   - No duplication (DRY principle)
   - Clear separation between unit and integration tests

6. **Error Handling**
    - All error returns checked
    - Error messages are descriptive
    - Graceful degradation tested
    - User-facing errors validated

7. **Structured Logging**
    - Log emissions verified at correct levels
    - Structured fields present and correct
    - Log level flags tested (--debug, --quiet, --json)
    - Logger properly injected in all functions
    - Logs not mixed with user output (fmt.Printf)
    - Test log capture and verification pattern used

8. **Cross-Platform Considerations**
    - Symlink handling tested for all platforms
    - Path handling works on macOS/Linux/Windows
    - Platform-specific tests properly tagged
    - No hardcoded platform assumptions

9. **Performance**
    - Tests run efficiently (< 1 second each)
    - No memory leaks detected
    - Race condition tests pass with `-race` flag
    - Large file handling tested

10. **Documentation**
    - Complex test logic has comments
    - Test names are self-documenting
    - Helper functions have clear purpose
    - Test data organized in testdata/

## Common Test Templates

**Template 1: Basic Command Test with Logging**

```go
func TestAdd_SingleFile_Success(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    sourceFile := filepath.Join(tempDir, ".zshrc")
    os.WriteFile(sourceFile, []byte("# Test config"), 0644)

    // Create logger with buffer for log capture
    var logBuf bytes.Buffer
    handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })
    logger := slog.New(handler)

    cfg := CreateTestConfig(t)
    cfg.Logger = logger
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    // Act
    err := AddFile(sourcePath, cfg)

    // Assert
    assert.NoError(t, err, "AddFile should succeed")
    AssertFileExists(t, filepath.Join(cfg.RepoPath, "shell", "zshrc"))
    AssertSymlinkPointsTo(t, sourceFile, filepath.Join(cfg.RepoPath, "shell", "zshrc"))

    // Verify logs
    logs := logBuf.String()
    assert.Contains(t, logs, "validating source file")
    assert.Contains(t, logs, "backup created")
    assert.Contains(t, logs, "symlink created")
}
```

**Template 2: Error Path Test**

```go
func TestAdd_FileNotFound_ReturnsError(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    sourceFile := filepath.Join(tempDir, ".zshrc")
    // Don't create file - it doesn't exist

    cfg := CreateTestConfig(t)
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    // Act
    err := AddFile(sourceFile, cfg)

    // Assert
    assert.Error(t, err, "should return error for non-existent file")
    assert.Contains(t, err.Error(), "not found", "error message should mention file not found")

    // Verify no side effects
    AssertFileNotExists(t, filepath.Join(cfg.RepoPath, "shell", "zshrc"))
}
```

**Template 3: Flag Variation Test**

```go
func TestAdd_DryRunFlag_NoChangesMade(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    sourceFile := filepath.Join(tempDir, ".zshrc")
    os.WriteFile(sourceFile, []byte("# Test"), 0644)

    cfg := CreateTestConfig(t)
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    // Act - Test with --dry-run flag
    err := AddFile(sourceFile, cfg, "--dry-run")

    // Assert
    assert.NoError(t, err, "should succeed")
    AssertFileNotExists(t, filepath.Join(cfg.RepoPath, "shell", "zshrc"))
    // File should still exist at original location
    AssertFileExists(t, sourceFile)
}
```

**Template 4: Table-Driven Test**

```go
func TestValidate_FileScenarios(t *testing.T) {
    tests := []struct {
        name      string
        filename  string
        content   string
        wantErr   bool
        errType   string
    }{
        {
            name:     "valid zshrc file",
            filename: ".zshrc",
            content:  "# Test config",
            wantErr:  false,
        },
        {
            name:     "env file with API key",
            filename: ".env",
            content:  "API_KEY=secret123",
            wantErr:  true,
            errType:  "secret detected",
        },
        {
            name:     "private key file",
            filename: ".ssh/id_rsa",
            content:  "-----BEGIN RSA PRIVATE KEY-----",
            wantErr:  true,
            errType:  "secret file",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Arrange
            tempDir := t.TempDir()
            testFile := filepath.Join(tempDir, tt.filename)
            os.WriteFile(testFile, []byte(tt.content), 0644)

            cfg := CreateTestConfig(t)

            // Act
            err := ValidateFile(testFile, cfg)

            // Assert
            if tt.wantErr {
                assert.Error(t, err, "should reject %s", tt.name)
                assert.Contains(t, err.Error(), tt.errType, "error should mention %s", tt.errType)
            } else {
                assert.NoError(t, err, "should accept %s", tt.name)
            }
        })
    }
}
```

**Template 5: Command Execution Test**

```go
func TestList_FlagJson_PrintsJsonOutput(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    sourceFile := filepath.Join(tempDir, ".zshrc")
    os.WriteFile(sourceFile, []byte("# Test"), 0644)

    // Initialize repo and add file
    cmd := exec.Command("go", "run", "cmd/dotcor/main.go", "init")
    cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
    require.NoError(t, cmd.Run(), "init should succeed")

    cmd = exec.Command("go", "run", "cmd/dotcor/main.go", "add", sourceFile)
    cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))
    require.NoError(t, cmd.Run(), "add should succeed")

    // Act - Run list with --json flag
    var stdout, stderr bytes.Buffer
    cmd = exec.Command("go", "run", "cmd/dotcor/main.go", "list", "--json")
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    cmd.Env = append(os.Environ(), fmt.Sprintf("HOME=%s", tempDir))

    err := cmd.Run()

    // Assert
    assert.NoError(t, err, "list --json should succeed")
    output := stdout.String()
    assert.Contains(t, output, "\"managed_files\"", "should be valid JSON")
    assert.Contains(t, output, "\".zshrc\"", "should contain file")
}
```

## Success Criteria

- [ ] All 14 command test files expanded to target line counts
- [ ] All tests pass: `go test ./... -v`
- [ ] Coverage达到 75%+ for all commands
- [ ] `golangci-lint` passes with zero errors
- [ ] Integration tests for 7 core commands pass
- [ ] Senior engineer review passes all criteria
- [ ] All tests are deterministic and isolated
- [ ] Code is clean, organized, and follows best practices

## Notes

### Testing Philosophy
- Focus on unit tests for comprehensive coverage
- Integration tests only for core command workflows
- Each command gets atomic commit during implementation
- Pre-commit gate: `go build ./... && go test ./...`
- Coverage report: `go tool cover -html=coverage.html`

### Logging Testing Requirements
- All commands must verify log emissions at correct levels
- Use `slog` handler with buffer to capture logs for verification
- Test DEBUG, INFO, WARN, ERROR level emissions
- Verify structured fields (op, file, error, duration, etc.)
- Test log level flags: --debug, --quiet, --json
- See `docs/LOGGING.md` for detailed logging guidelines

### Key Reference Documents
- `docs/LOGGING.md` - Structured logging guide with slog
- `docs/TESTING.md` - Testing conventions and best practices
- `cmd/dotcor/test_helpers.go` - Common test helper functions
- `CLAUDE.md` - Project development guide

### Testing Tools
- **Testify**: Assertion framework (`github.com/stretchr/testify`)
- **Go Testing**: Standard library testing package
- **Go Cover**: Coverage analysis (`go tool cover`)
- **Golangci-lint**: Linting and static analysis
- **Race Detector**: Concurrent operation safety (`go test -race`)

### Test Organization Principles
1. **AAA Pattern**: All tests must have Arrange, Act, Assert sections with comments
2. **Test Isolation**: Each test independent using `t.TempDir()`
3. **Deterministic**: No flaky tests, no time-based assertions
4. **Clear Names**: `Test<Command>_<Scenario>_<ExpectedResult>` pattern
5. **Helper Functions**: Marked with `t.Helper()` for accurate error reporting
6. **No Hardcoding**: Use temp directories, not fixed paths
7. **Error Checking**: All error returns checked with require/assert

### When to Write Tests
- **Before implementation**: TDD approach for complex logic
- **After implementation**: For simple features and refactoring
- **Bug fixes**: Add regression test for every bug fixed
- **New features**: Mandatory tests for all new functionality

### Test Review Checklist
- [ ] Test uses AAA pattern with section comments
- [ ] Test name follows convention: `Test<Command>_<Scenario>_<ExpectedResult>`
- [ ] Test helpers marked with `t.Helper()`
- [ ] All error returns checked
- [ ] No hardcoded paths (use `t.TempDir()`)
- [ ] Test is deterministic (no sleep, no random)
- [ ] Test is isolated (no dependency on other tests)
- [ ] Clear failure messages in assertions
- [ ] Logs verified (if applicable)
- [ ] Edge cases and error paths tested
- [ ] Table-driven tests for multiple scenarios (if applicable)

### Continuous Integration
- All tests must pass before merging
- Coverage tracked in CI dashboard
- Linter must pass (`golangci-lint run ./...`)
- Race detector must pass (`go test -race ./...`)
- Test execution time monitored
