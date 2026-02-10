# Comprehensive Testing Plan for DotCor (macOS-Only)

**Date:** 2026-02-09
**Status:** Updated for macOS-first refactor
**Target Release:** v0.6.0

## Overview

Create comprehensive unit test coverage for all 14 DotCor commands with edge case testing and selected integration tests for core commands. This plan has been updated to reflect the macOS-first refactor, which removed all cross-platform code.

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

## macOS-First Refactor Changes

**Removed Platform-Related Code:**
- ❌ No `Platforms` field in `ManagedFile` struct
- ❌ No `GetManagedFilesForPlatform()` function
- ❌ No `GetCurrentPlatform()` or `ShouldApplyOnPlatform()` functions
- ❌ No Windows symlink checks (`SupportsSymlinks()`)
- ❌ No Windows-specific lock handling (`isProcessAliveWindows()`)
- ❌ No `OS` field in `TemplateContext`
- ❌ No `{{ .OS }}` template variable
- ❌ No `--platforms` flag in add command
- ❌ No platform-specific integration tests

**Updated Testing Focus:**
- ✅ Simplified tests without platform branching
- ✅ Direct `ManagedFiles` access (no filtering)
- ✅ Native macOS symlink handling (no support checks)
- ✅ Unix-only lock handling (no Windows paths)
- ✅ Simplified template context (Hostname, User, Home only)

## Command-Specific Test Coverage

### Core Commands (with integration tests):

**1. init**
- Happy path: Creates directories, initializes config, sets up git repo
- Error paths: Permission denied, disk full, invalid path
- Flags: `--apply` creates symlinks, `--interactive` mode
- Integration: init → verify structure
- **Note:** No symlink support checks (macOS native)

**2. add**
- Single file, multiple files, directory (recursive), glob patterns
- Validation: File exists, not secret, not already managed
- Error paths: Permission denied, file not found, secret detection, backup failure
- Flags: `--category`, `--dry-run`, `--force` (no --platforms)
- Integration: init → add → verify symlink and config
- **Note:** No platforms field in ManagedFile creation

**3. remove**
- Single file, multiple files, directory
- Error paths: File not managed, permission denied, symlink removal fails
- Flags: `--backup`, `--force`
- Integration: init → add → remove → verify removal

**4. list**
- No files, single file, multiple files
- Flags: `--json`, `--verbose`, `--category filter`
- Integration: init → add → list → verify output
- **Note:** Uses direct `ManagedFiles` access

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
- **Note:** No platforms field in ManagedFile creation

**11. doctor**
- Healthy system, broken symlink, stale lock, missing git
- Flags: `--fix`, `--verbose`
- Error paths: None (doctor runs in degraded state)
- **Note:** No platform-specific checks

**12. rebuild-config**
- Valid repo, empty repo, scan flag
- Flags: `--scan`, `--dry-run`
- Error paths: Invalid config, not initialized
- **Note:** No platforms field in ManagedFile creation

**13. rebuild-links**
- Template files, no templates, specific files
- Flags: `--dry-run`, `--verbose`
- Error paths: Invalid template, permission denied
- **Note:** Only {{ .Hostname }}, {{ .User }}, {{ .Home }} variables (no {{ .OS }})

**14. clone**
- Valid URL, invalid URL, existing directory
- Error paths: Network failure, invalid repo
- Flags: `--branch`, `--depth`
- **Note:** No symlink support checks (macOS native)

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
├── rebuild-links_test.go    # Expand to 400+ lines (no platforms field)
├── clone_test.go            # Expand to 400+ lines
├── cleanup_test.go          # Expand to 400+ lines
└── init_test.go             # Expand to 500+ lines

tests/integration/
├── core_workflow_test.go    # New: init → add → list → status
├── sync_workflow_test.go    # New: add → sync → verify
├── restore_workflow_test.go # New: add → modify → restore → verify
└── remove_workflow_test.go   # New: add → remove → verify
```

**Integration Test Details:**

1. **Core Workflow (core_workflow_test.go):**
   - Test init command creates proper structure
   - Add multiple files (shell, nvim, git configs)
   - List command shows all managed files
   - Status command shows healthy state
   - Verify symlinks created correctly (macOS native)
   - Verify config updated with all files (no platforms field)
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

## Test File Structure Pattern

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

## Assertion Strategies

1. **File System Assertions:**
   - File exists/not exists: `AssertFileExists`, `AssertFileNotExists`
   - File content: `AssertFileContent(t, path, expected)`
   - Directory structure: Verify directory layout after operations
   - Symlink validation: `AssertSymlinkPointsTo(t, link, expectedTarget)`
   - **Note:** No platform-specific symlink checks

2. **Config State Assertions:**
   - Read config file and verify managed files list
   - Check config version and fields
   - Verify config saves correctly after operations
   - **Note:** No platforms field validation

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

## Test Coverage Metrics & Goals

**Coverage Targets (Post-Refactor):**

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

## Implementation Steps

**Phase 1: Preparation & Infrastructure**
1. Enhance `cmd/dotcor/test_helpers.go` with new helper functions
2. Create integration test structure in `tests/integration/`
3. Set up coverage tracking and baseline measurements

**Phase 2: Core Command Tests (Priority)**
4. Expand `init_test.go` to 500+ lines (no symlink support checks)
5. Expand `add_test.go` to 600+ lines (no platforms field)
6. Expand `remove_test.go` to 400+ lines
7. Expand `list_test.go` to 350+ lines (direct ManagedFiles access)
8. Expand `status_test.go` to 400+ lines
9. Expand `sync_test.go` to 500+ lines
10. Expand `restore_test.go` to 450+ lines

**Phase 3: Supporting Command Tests**
11. Expand `diff_test.go` to 400+ lines
12. Expand `history_test.go` to 400+ lines
13. Expand `adopt_test.go` to 400+ lines (no platforms field)
14. Expand `doctor_test.go` to 500+ lines (no platform checks)
15. Expand `rebuild_test.go` to 400+ lines (no platforms field)
16. Expand `rebuild-links_test.go` to 400+ lines (no OS template var)
17. Expand `clone_test.go` to 400+ lines (no symlink checks)
18. Expand `cleanup_test.go` to 400+ lines

**Phase 4: Integration Tests**
19. Create `tests/integration/core_workflow_test.go`
20. Create `tests/integration/sync_workflow_test.go`
21. Create `tests/integration/restore_workflow_test.go`
22. Create `tests/integration/remove_workflow_test.go`

**Phase 5: Quality Assurance**
23. Run `golangci-lint run ./...` and fix all issues
24. Verify all tests pass: `go test ./... -v`
25. Run race detector: `go test ./... -race`
26. Generate coverage report and verify 75%+ target
27. Generate HTML coverage report: `go tool cover -html=coverage.out`
28. Review untested code: Identify gaps and add tests
29. Conduct in-depth senior engineer review:
    - Review test quality, clarity, and best practices
    - Verify edge cases and error paths are covered
    - Check test isolation and determinism
    - Ensure AAA pattern consistency
    - Validate helper functions are properly marked
    - Review code organization and naming
    - Verify structured logging tests are comprehensive
    - Check performance (test execution time, memory)

**Phase 6: Finalization**
30. Commit changes (atomic commits per command)
31. Create new tag v0.6.0 with comprehensive testing release

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

**Template 2: Template System Test (macOS-only)**

```go
func TestRebuildLinks_TemplateSubstitution(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    cfg := CreateTestConfig(t)
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    // Create template file (no {{ .OS }} variable)
    templatePath := filepath.Join(cfg.RepoPath, "zshrc.template")
    templateContent := "# Config for {{ .Hostname }}\nexport HOME={{ .Home }}\n"
    os.WriteFile(templatePath, []byte(templateContent), 0644)

    cfg.ManagedFiles = []config.ManagedFile{
        {
            SourcePath: "~/.zshrc",
            RepoPath:   "zshrc",
            AddedAt:    time.Now(),
        },
    }

    // Act
    err := RunRebuildLinks(cfg)

    // Assert
    assert.NoError(t, err, "RebuildLinks should succeed")

    // Verify template substitution (Hostname, User, Home only)
    renderedPath := filepath.Join(cfg.RepoPath, "zshrc")
    content, err := os.ReadFile(renderedPath)
    require.NoError(t, err)
    assert.Contains(t, string(content), "# Config for ")  // Hostname substituted
    assert.Contains(t, string(content), "export HOME=")   // Home substituted
    assert.NotContains(t, string(content), "{{ .OS }}")  // No OS variable
}
```

**Template 3: Symlink Test (No Platform Checks)**

```go
func TestAdd_CreatesRelativeSymlink(t *testing.T) {
    // Arrange
    tempDir := t.TempDir()
    sourceFile := filepath.Join(tempDir, ".zshrc")
    os.WriteFile(sourceFile, []byte("# Test config"), 0644)

    cfg := CreateTestConfig(t)
    cfg.RepoPath = filepath.Join(tempDir, "repo")

    // Act
    err := AddFile(sourceFile, cfg)

    // Assert
    assert.NoError(t, err, "AddFile should succeed")

    // Verify symlink created (no platform checks - macOS native)
    isSymlink, err := fs.IsSymlink(sourceFile)
    assert.NoError(t, err, "should check symlink status")
    assert.True(t, isSymlink, "source should be a symlink")

    // Verify relative symlink (portable across machines)
    target, err := os.Readlink(sourceFile)
    assert.NoError(t, err, "should read symlink")
    assert.NotContains(t, target, "/")  // Relative path, not absolute
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
- [ ] No platform-specific test code remains
- [ ] Template tests only use {{ .Hostname }}, {{ .User }}, {{ .Home }}
- [ ] Config tests don't validate platforms field

## Notes

### Testing Philosophy
- Focus on unit tests for comprehensive coverage
- Integration tests only for core command workflows
- Each command gets atomic commit during implementation
- Pre-commit gate: `go build ./... && go test ./...`
- Coverage report: `go tool cover -html=coverage.html`

### macOS-Only Testing
- Symlink tests assume native macOS support (no `SupportsSymlinks()` checks)
- Lock handling tests use Unix-only code (no Windows paths)
- Template tests don't verify `{{ .OS }}` substitution
- Config tests don't validate `Platforms` field
- Integration tests don't test platform filtering

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
- `docs/plans/2026-02-09-macos-first-refactor.md` - Refactor details

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
- [ ] No platform-specific test code remains
- [ ] No reference to platforms field or filtering

### Continuous Integration
- All tests must pass before merging
- Coverage tracked in CI dashboard
- Linter must pass (`golangci-lint run ./...`)
- Race detector must pass (`go test -race ./...`)
- Test execution time monitored
