# Comprehensive Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Establish best-practice testing across DotCor with testify, AAA pattern, and comprehensive coverage of critical paths.

**Architecture:**
- Convert existing tests to testify/assert for consistency
- Apply AAA pattern (Arrange-Act-Assert) with section comments
- Add missing command-level tests (currently 0% coverage)
- Improve integration tests for realistic workflows
- Achieve 85% overall coverage with focus on core packages and CLI

**Tech Stack:**
- testify v1.11.1 for assertions
- Go 1.25.5 testing package
- t.TempDir() for test isolation
- testify/mock for Git operations (optional)

**Pre-commit workflow (MANDATORY):**
1. `go build ./...` - Must succeed
2. `go test ./...` - Must succeed
3. `git commit ...` - Only if 1 & 2 pass

---

## Context Notes for Engineers

**About DotCor:** A symlink-based dotfile manager in Go. Users add dotfiles (e.g., ~/.zshrc) which get moved to ~/.dotcor/files/ and replaced with relative symlinks. Git integration automatically commits changes.

### Idiomatic Go Test Organization

**CRITICAL:** Follow Go best practices for test organization:

1. **Unit tests live alongside code they test**
   - `internal/core/validator.go` → `internal/core/validator_test.go`
   - `cmd/dotcor/add.go` → `cmd/dotcor/add_test.go`
   - NOT in a separate top-level `tests/` directory

2. **Integration tests live separately**
   - `tests/integration/` for end-to-end tests
   - Real workflows, slower, fewer tests

3. **Helpers stay local**
   - `cmd/dotcor/test_helpers.go` for command tests
   - NOT globally shared across unrelated packages

4. **Fixtures go in testdata/**
   - `testdata/fixtures/` for static test data
   - Go treats `testdata/` specially

**Why this matters:**
- Access to unexported identifiers (critical in Go)
- Accurate coverage attribution
- Refactors don't break imports
- Idiomatic and expected by Go developers
- `go test ./...` works correctly

### Project Structure

```
dotcor/
├── cmd/dotcor/          # CLI commands (Cobra)
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
│   └── test_helpers.go # Helper functions for command tests
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

### AAA Pattern

Every test should have three clearly marked sections:

```go
// Arrange - Set up test data, create temp files, initialize dependencies
// Act - Call the function being tested
// Assert - Verify results using testify assertions
```

### Testify Common Assertions

- `assert.Equal(t, expected, actual)` - Exact match
- `assert.NoError(t, err)` - No error expected
- `assert.Error(t, err)` - Error expected
- `assert.True(t, condition, "message")` - Boolean with message
- `assert.Contains(t, slice, item)` - Slice membership
- `assert.FileExists(t, path)` - File exists

---

## Phase 1: Foundation - Convert Existing Tests to Testify

### Task 1: Add testify dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add testify to go.mod**

Add this line in the `require` section:

```go
github.com/stretchr/testify v1.11.1
```

**Step 2: Run go mod tidy**

```bash
go mod tidy
```

Expected: No errors, testify dependency added

**Step 3: Run tests to ensure nothing broke**

```bash
go test ./... -v
```

Expected: All existing tests still pass

**Step 4: Build to ensure everything compiles**

```bash
go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add testify testing framework v1.11.1"
```

---

### Task 2: Create test helpers for command tests

**Files:**
- Create: `cmd/dotcor/test_helpers.go`

**Step 1: Write test helpers file**

Create `cmd/dotcor/test_helpers.go`:

```go
package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

// CreateTestConfig creates a test config with temp directory for testing commands
func CreateTestConfig(t *testing.T) *config.Config {
	t.Helper()

	tempDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return &config.Config{
		Logger:         logger,
		RepoPath:       filepath.Join(tempDir, "files"),
		GitEnabled:     false, // Disable git for most command tests
		IgnorePatterns: []string{},
		ManagedFiles:   []config.ManagedFile{},
	}
}

// CreateTestFile creates a test file with specified content in temp directory
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

// AssertFileExists asserts that a file exists at the given path
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

// AssertFileNotExists asserts that a file does not exist at the given path
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

// AssertDirExists asserts that a directory exists at the given path
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

// AssertFileContent asserts that a file contains the expected content
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

**Step 2: Run tests to verify helpers compile**

```bash
go test ./cmd/dotcor/... -v
```

Expected: No compilation errors

**Step 3: Build to ensure everything compiles**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add cmd/dotcor/test_helpers.go
git commit -m "test: add helper functions for command-level testing"
```

---

### Task 3: Convert internal/core/validator_test.go to testify

**Files:**
- Modify: `internal/core/validator_test.go`

**Step 1: Read existing test file**

```bash
cat internal/core/validator_test.go
```

This is to understand current test structure before conversion.

**Step 2: Add testify import**

Add this import at the top:

```go
"github.com/stretchr/testify/assert"
```

**Step 3: Convert TestValidateRepoPath to AAA pattern**

Replace the existing test with:

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
			name:    "valid nested path",
			path:    "config/nvim/init.lua",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
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
		{
			name:    "path with internal traversal",
			path:    "shell/../git/gitconfig",
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
				assert.Error(t, err, "should return error for invalid path: %s", testPath)
			} else {
				assert.NoError(t, err, "should not return error for valid path: %s", testPath)
			}
		})
	}
}
```

**Step 4: Convert TestDetectSecrets to AAA pattern**

Replace with:

```go
func TestDetectSecrets(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantSecrets bool
	}{
		{
			name:        "no secrets",
			content:     "# This is a normal config\nexport PATH=/usr/bin\n",
			wantSecrets: false,
		},
		{
			name:        "api key",
			content:     "API_KEY=mock_api_key_for_testing_purposes_only\n",
			wantSecrets: true,
		},
		{
			name:        "password",
			content:     "password=mysecretpassword123\n",
			wantSecrets: true,
		},
		{
			name:        "private key header",
			content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEpA...\n-----END RSA PRIVATE KEY-----",
			wantSecrets: true,
		},
		{
			name:        "aws credentials",
			content:     "aws_access_key_id=MOCKAWSACCESSKEYID20\n",
			wantSecrets: true,
		},
		{
			name:        "database url with password",
			content:     "DATABASE_URL=postgres://user:secretpass@localhost/db\n",
			wantSecrets: true,
		},
		{
			name:        "access token",
			content:     "access_token = 'mock_access_token_for_testing_1234567890'\n",
			wantSecrets: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			testFile := filepath.Join(tempDir, tt.name+".txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Act
			secrets, err := DetectSecrets(testFile)
			assert.NoError(t, err, "DetectSecrets should not error")

			// Assert
			gotSecrets := len(secrets) > 0
			assert.Equal(t, tt.wantSecrets, gotSecrets,
				"DetectSecrets(%q) found secrets = %v, want %v (secrets: %v)",
				tt.name, gotSecrets, tt.wantSecrets, secrets)
		})
	}
}
```

**Step 5: Convert remaining validator tests**

Convert all remaining tests following same AAA pattern:
- TestValidateNotAlreadyManaged
- TestValidateFileSize
- TestShouldWarnAboutSecrets

For each:
- Add `// Arrange`, `// Act`, `// Assert` section comments
- Replace manual error checks with testify assertions

**Step 6: Run tests**

```bash
go test ./internal/core/... -v -run TestValidateRepoPath
go test ./internal/core/... -v -run TestDetectSecrets
go test ./internal/core/... -v -run TestValidateNotAlreadyManaged
go test ./internal/core/... -v -run TestValidateFileSize
go test ./internal/core/... -v -run TestShouldWarnAboutSecrets
```

Expected: All tests pass with testify assertions

**Step 7: Run all core tests**

```bash
go test ./internal/core/... -v
```

Expected: All tests pass

**Step 8: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 9: Commit**

```bash
git add internal/core/validator_test.go
git commit -m "test: convert validator tests to testify with AAA pattern"
```

---

### Task 4: Convert internal/core/transaction_test.go to testify

**Files:**
- Modify: `internal/core/transaction_test.go`

**Step 1: Add testify import**

```go
"github.com/stretchr/testify/assert"
```

**Step 2: Convert TestNewTransaction**

```go
func TestNewTransaction(t *testing.T) {
	// Arrange
	cfg := &config.Config{Logger: slog.Default()}

	// Act
	tx := NewTransaction(cfg)

	// Assert
	assert.NotNil(t, tx, "NewTransaction should return non-nil transaction")
	assert.False(t, tx.IsCommitted(), "NewTransaction should not be committed")
	assert.Equal(t, 0, tx.ExecutedCount(), "NewTransaction should have 0 executed operations")
}
```

**Step 3: Convert remaining transaction tests**

Convert all remaining tests:
- TestTransactionExecute
- TestTransactionExecuteFails
- TestTransactionRollback
- TestTransactionCommit
- TestTransactionExecuteAfterCommit
- TestMoveFileOp
- TestCreateDirOp
- TestCreateDirOpUndoNonEmpty
- TestOperationDescribe
- TestTransactionExecuteAll

Each follows same AAA pattern with testify assertions.

**Step 4: Run tests**

```bash
go test ./internal/core/... -v -run Transaction
```

Expected: All tests pass

**Step 5: Run all core tests**

```bash
go test ./internal/core/... -v
```

Expected: All tests pass

**Step 6: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 7: Commit**

```bash
git add internal/core/transaction_test.go
git commit -m "test: convert transaction tests to testify with AAA pattern"
```

---

### Task 5: Convert internal/config/config_test.go to testify

**Files:**
- Modify: `internal/config/config_test.go`

**Step 1: Add testify import**

```go
"github.com/stretchr/testify/assert"
```

**Step 2: Convert all tests to AAA pattern**

Convert all tests:
- TestGetDefaultIgnorePatterns
- TestNewDefaultConfig
- TestShouldApplyOnPlatform
- TestConfigManagedFiles
- TestGetManagedFilesForPlatform
- TestGetUncommittedFiles
- TestContains

Each follows AAA pattern with `// Arrange`, `// Act`, `// Assert` comments.

**Step 3: Run tests**

```bash
go test ./internal/config/... -v
```

Expected: All tests pass

**Step 4: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add internal/config/config_test.go
git commit -m "test: convert config tests to testify with AAA pattern"
```

---

### Task 6: Convert internal/config/paths_test.go to testify

**Files:**
- Modify: `internal/config/paths_test.go`

**Step 1: Add testify import and convert tests**

Add import and convert all tests to AAA pattern with testify.

**Step 2: Run tests**

```bash
go test ./internal/config/... -v
```

**Step 3: Build**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/config/paths_test.go
git commit -m "test: convert paths tests to testify with AAA pattern"
```

---

### Task 7: Convert internal/fs/symlink_test.go to testify

**Files:**
- Modify: `internal/fs/symlink_test.go`

**Step 1: Add testify import**

```go
"github.com/stretchr/testify/assert"
```

**Step 2: Convert all tests to AAA pattern**

Convert all symlink tests:
- TestSupportsSymlinks
- TestIsSymlink
- TestReadSymlink
- TestIsValidSymlink
- TestIsRelativeSymlink
- TestResolveSymlink
- TestGetSymlinkStatus
- TestRemoveSymlink
- TestRemoveSymlinkErrorsOnRegularFile

**Step 3: Run tests**

```bash
go test ./internal/fs/... -v
```

**Step 4: Build**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/fs/symlink_test.go
git commit -m "test: convert symlink tests to testify with AAA pattern"
```

---

### Task 8: Convert internal/fs/fs_test.go to testify

**Files:**
- Modify: `internal/fs/fs_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/fs/... -v
```

**Step 3: Build**

```bash
go build ./...
```

**Step 4: Commit**

```bash
git add internal/fs/fs_test.go
git commit -m "test: convert fs tests to testify with AAA pattern"
```

---

### Task 9: Convert internal/git/git_test.go to testify

**Files:**
- Modify: `internal/git/git_test.go`

**Step 1: Add testify import**

```go
"github.com/stretchr/testify/assert"
```

**Step 2: Convert all tests to AAA pattern**

Convert all git tests:
- TestIsGitInstalled
- TestInitRepo
- TestIsRepo
- TestHasChanges
- TestAutoCommit
- TestGetStatus
- TestGetRemoteURL
- TestSetRemote
- TestGetFileHistory
- TestGetCurrentCommit
- TestGetChangedFiles
- TestGetDiff
- TestStageAndUnstageFile
- TestStatusInfo
- TestCommitInfo

**Step 3: Run tests**

```bash
go test ./internal/git/... -v
```

**Step 4: Build**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/git/git_test.go
git commit -m "test: convert git tests to testify with AAA pattern"
```

---

### Task 10: Convert internal/core/backup_test.go to testify

**Files:**
- Modify: `internal/core/backup_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/core/... -v -run Backup
```

**Step 3: Build and commit**

```bash
go build ./... && git add internal/core/backup_test.go && git commit -m "test: convert backup tests to testify with AAA pattern"
```

---

### Task 11: Convert internal/core/lock_test.go to testify

**Files:**
- Modify: `internal/core/lock_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/core/... -v -run Lock
```

**Step 3: Build and commit**

```bash
go build ./... && git add internal/core/lock_test.go && git commit -m "test: convert lock tests to testify with AAA pattern"
```

---

### Task 12: Convert internal/core/hooks_test.go to testify

**Files:**
- Modify: `internal/core/hooks_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/core/... -v -run Hooks
```

**Step 3: Build and commit**

```bash
go build ./... && git add internal/core/hooks_test.go && git commit -m "test: convert hooks tests to testify with AAA pattern"
```

---

### Task 13: Convert internal/core/ignore_test.go to testify

**Files:**
- Modify: `internal/core/ignore_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/core/... -v -run Ignore
```

**Step 3: Build and commit**

```bash
go build ./... && git add internal/core/ignore_test.go && git commit -m "test: convert ignore tests to testify with AAA pattern"
```

---

### Task 14: Convert internal/core/templates_test.go to testify

**Files:**
- Modify: `internal/core/templates_test.go`

**Step 1: Add testify import and convert tests**

**Step 2: Run tests**

```bash
go test ./internal/core/... -v -run Templates
```

**Step 3: Build and commit**

```bash
go build ./... && git add internal/core/templates_test.go && git commit -m "test: convert templates tests to testify with AAA pattern"
```

---

## Phase 2: Add Missing Unit Tests

### Task 15: Create internal/config/migrate_test.go

**Files:**
- Create: `internal/config/migrate_test.go`

**Step 1: Write migration tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMigrateFromEmpty(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:  "", // Empty version indicates pre-v1.0 config
		RepoPath: "~/.dotcor/files",
		ManagedFiles: []ManagedFile{
			{SourcePath: "~/.zshrc", RepoPath: "shell/zshrc"},
		},
	}

	// Act
	err := MigrateFromEmpty(cfg)

	// Assert
	assert.NoError(t, err, "MigrateFromEmpty should not error")
	assert.Equal(t, CurrentConfigVersion, cfg.Version, "Version should be updated to current")
	assert.NotEmpty(t, cfg.IgnorePatterns, "IgnorePatterns should be populated")
}

func TestMigrateFromEmpty_NoManagedFiles(t *testing.T) {
	// Arrange
	cfg := &Config{
		Version:      "",
		RepoPath:     "~/.dotcor/files",
		ManagedFiles: []ManagedFile{},
	}

	// Act
	err := MigrateFromEmpty(cfg)

	// Assert
	assert.NoError(t, err, "MigrateFromEmpty should handle empty ManagedFiles")
	assert.Equal(t, CurrentConfigVersion, cfg.Version, "Version should be updated")
}

func TestGetMigrationPath(t *testing.T) {
	tests := []struct {
		name       string
		from       string
		to         string
		wantSteps  int
	}{
		{
			name:      "same version",
			from:      "1.0",
			to:        "1.0",
			wantSteps: 0,
		},
		{
			name:      "single step",
			from:      "1.0",
			to:        "1.1",
			wantSteps: 1,
		},
		{
			name:      "no path",
			from:      "1.0",
			to:        "2.0",
			wantSteps: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			path := GetMigrationPath(tt.from, tt.to)

			// Assert
			assert.Equal(t, tt.wantSteps, len(path),
				"GetMigrationPath(%s, %s) should return %d steps, got %d",
				tt.from, tt.to, tt.wantSteps, len(path))
		})
	}
}
```

**Step 2: Run tests**

```bash
go test ./internal/config/... -v -run Migrate
```

Expected: Tests pass

**Step 3: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add internal/config/migrate_test.go
git commit -m "test: add config migration tests"
```

---

## Phase 3: Add Command-Level Tests (NEW)

### Task 16: Create cmd/dotcor/init_test.go

**Files:**
- Create: `cmd/dotcor/init_test.go`

**Step 1: Write init command tests**

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitCommand_DefaultSettings(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	dotcorDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(dotcorDir, "files")

	// Mock home directory to temp dir
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", oldHome)

	// Remove existing config if present
	configPath := filepath.Join(dotcorDir, "config.yaml")
	os.RemoveAll(configPath)

	// Act
	cmd := rootCmd
	cmd.SetArgs([]string{"init"})
	err := cmd.Execute()

	// Assert
	assert.NoError(t, err, "init command should succeed")
	AssertDirExists(t, dotcorDir, "init should create .dotcor directory")
	AssertDirExists(t, filesDir, "init should create files subdirectory")

	// Check config was created
	AssertFileExists(t, configPath, "init should create config.yaml")
}

func TestInitCommand_AlreadyInitialized(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	dotcorDir := filepath.Join(tempDir, ".dotcor")

	// Mock home directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", oldHome)

	// Create existing .dotcor directory
	os.MkdirAll(dotcorDir, 0755)

	// Act
	cmd := rootCmd
	cmd.SetArgs([]string{"init"})
	err := cmd.Execute()

	// Assert
	// Should show appropriate message (may be error or warning)
	// Command should not fail hard
}

func TestInitCommand_ApplyMode(t *testing.T) {
	// Test --apply flag
}
```

**Step 2: Run tests**

```bash
go test ./cmd/dotcor/... -v -run TestInit
```

Expected: Tests pass

**Step 3: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add cmd/dotcor/init_test.go
git commit -m "test: add init command tests"
```

---

### Task 17: Create cmd/dotcor/add_test.go

**Files:**
- Create: `cmd/dotcor/add_test.go`

**Step 1: Write add command tests**

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddCommand_SingleFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dotcorDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(dotcorDir, "files")

	// Create directories
	os.MkdirAll(homeDir, 0755)
	os.MkdirAll(filesDir, 0755)

	// Mock home directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Create test config
	configDir := filepath.Join(dotcorDir, "config.yaml")
	os.WriteFile(configDir, []byte("version: 1\nrepo_path: ~/.dotcor/files\ngit_enabled: false\nmanaged_files: []\n"), 0644)

	// Create test dotfile
	dotfile := filepath.Join(homeDir, ".zshrc")
	CreateTestFile(t, dotfile, "# zsh config\nexport PATH=/usr/bin\n")

	// Act
	cmd := rootCmd
	cmd.SetArgs([]string{"add", dotfile, "--force"})
	err := cmd.Execute()

	// Assert
	assert.NoError(t, err, "add command should succeed")

	// Verify file was moved to repo
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	AssertFileExists(t, repoFile, "add should move file to repository")

	// Verify symlink was created
	AssertFileExists(t, dotfile, "add should create symlink at original location")

	// Verify symlink points to repo
	target, err := os.Readlink(dotfile)
	assert.NoError(t, err, "should be able to read symlink")
	assert.Contains(t, target, "shell/zshrc", "symlink should point to repo file")
}

func TestAddCommand_AlreadyManaged(t *testing.T) {
	// Test error for already managed file
}

func TestAddCommand_NonExistentFile(t *testing.T) {
	// Test error for non-existent file
}

func TestAddCommand_WithSecrets(t *testing.T) {
	// Test secret detection and warning
}

func TestAddCommand_ForceSkipsWarnings(t *testing.T) {
	// Test --force flag bypasses warnings
}
```

**Step 2: Run tests**

```bash
go test ./cmd/dotcor/... -v -run TestAdd
```

Expected: Tests pass

**Step 3: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add cmd/dotcor/add_test.go
git commit -m "test: add command tests"
```

---

### Task 18: Create cmd/dotcor/remove_test.go

**Files:**
- Create: `cmd/dotcor/remove_test.go`

**Step 1: Write remove command tests**

```go
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemoveCommand_SingleFile(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dotcorDir := filepath.Join(tempDir, ".dotcor")
	filesDir := filepath.Join(dotcorDir, "files")

	// Create directories
	os.MkdirAll(homeDir, 0755)
	os.MkdirAll(filesDir, 0755)

	// Mock home directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	// Setup: add a file first
	dotfile := filepath.Join(homeDir, ".zshrc")
	repoFile := filepath.Join(filesDir, "shell", "zshrc")
	os.WriteFile(repoFile, []byte("# zsh config\n"), 0644)
	os.Symlink("../../.dotcor/files/shell/zshrc", dotfile)

	// Create config with managed file
	configContent := `version: 1
repo_path: ~/.dotcor/files
git_enabled: false
managed_files:
  - source_path: ~/.zshrc
    repo_path: shell/zshrc
`
	configPath := filepath.Join(dotcorDir, "config.yaml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Act
	cmd := rootCmd
	cmd.SetArgs([]string{"remove", dotfile, "--force"})
	err := cmd.Execute()

	// Assert
	assert.NoError(t, err, "remove command should succeed")

	// Verify symlink was removed
	AssertFileNotExists(t, dotfile, "remove should remove symlink")

	// Verify file was restored from repo
	AssertFileExists(t, repoFile, "remove should restore file from repo")
}

func TestRemoveCommand_NonManagedFile(t *testing.T) {
	// Test error for non-managed file
}
```

**Step 2: Run tests**

```bash
go test ./cmd/dotcor/... -v -run TestRemove
```

Expected: Tests pass

**Step 3: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add cmd/dotcor/remove_test.go
git commit -m "test: add remove command tests"
```

---

## Phase 4: Improve Integration Tests

### Task 19: Reorganize tests/integration_test.go

**Files:**
- Modify: `tests/integration_test.go` (or restructure)

**Step 1: Split integration tests by scenario**

Create multiple test files in `tests/integration/`:
- `tests/integration/init_add_list_test.go`
- `tests/integration/remove_restore_test.go`
- `tests/integration/sync_git_test.go`

Move existing tests from `tests/integration_test.go` to appropriate files.

**Step 2: Convert to testify and AAA pattern**

Add testify imports and AAA section comments to all integration tests.

**Step 3: Run tests**

```bash
go test ./tests/... -v
```

Expected: All tests pass

**Step 4: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add tests/integration/
git commit -m "test: reorganize and improve integration tests"
```

---

## Phase 5: Final Documentation

### Task 20: Create docs/TESTING.md

**Files:**
- Create: `docs/TESTING.md`

**Step 1: Write comprehensive testing documentation**

Create `docs/TESTING.md` with:

```markdown
# DotCor Testing Guide

This document explains testing practices, conventions, and best practices for DotCor.

## Overview

DotCor follows a comprehensive testing strategy focusing on:

1. **Testify framework** - Use `github.com/stretchr/testify` v1.11.1 for all assertions
2. **AAA Pattern** - All tests follow Arrange-Act-Assert structure
3. **Critical path coverage** - Prioritize core packages and CLI commands
4. **Pre-commit validation** - Build and tests must pass before any commit
5. **New feature testing** - All new features and changes require corresponding tests

## Testing Philosophy

### Test-Driven Development (TDD)

1. Write failing test
2. Implement minimal code to make test pass
3. Refactor if needed
4. Repeat

This ensures code is testable, requirements are clear, and refactoring is safe.

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

### AAA Pattern

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
tests/integration/init_add_list_test.go
tests/integration/remove_restore_test.go
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

## Test Structure

### Test Isolation

Each test must be independent:

- Use `t.TempDir()` for test directories (auto-cleanup)
- Don't depend on other tests
- Clean up resources manually if needed
- Ensure tests are repeatable

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
            // Act
            err := ValidateRepoPath(tt.path)

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

### Why This Matters

- Prevents broken commits
- Catches compilation errors early
- Ensures tests always pass
- Maintains code quality

## Test Coverage

### Coverage Goals

| Package | Target Coverage |
|---------|----------------|
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

### Use t.Helper()

```go
func assertCustom(t *testing.T, condition bool, msg string) {
    t.Helper()  // This makes error show correct line in calling test
    if !condition {
        t.Errorf(msg)
    }
}
```

## Resources

- [testify documentation](https://github.com/stretchr/testify)
- [Go testing package](https://pkg.go.dev/testing)
- [Table-driven tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Go blog: testing patterns](https://go.dev/doc/tutorial/add-a-test)

## Questions?

Refer to `docs/plans/TESTING_PLAN.md` for implementation details.
```

**Step 2: Run all tests**

```bash
go test ./... -v
```

Expected: All tests pass

**Step 3: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 4: Commit**

```bash
git add docs/TESTING.md
git commit -m "docs: add comprehensive testing guide with AAA pattern and testify"
```

---

### Task 21: Update CLAUDE.md to reference TESTING.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Read CLAUDE.md**

```bash
cat CLAUDE.md
```

**Step 2: Add testing reference section**

Add this section to CLAUDE.md (insert after "Coding Standards" section):

```markdown
## Testing

DotCor follows comprehensive testing practices documented in `docs/TESTING.md`.

Key principles:
- Use testify v1.11.1 for all assertions
- Follow AAA pattern (Arrange-Act-Assert) with section comments
- Pre-commit workflow: `go build ./...` && go test ./...` before any commit
- Target 85%+ overall coverage with focus on critical paths
- Command-level tests for all CLI commands
- Unit tests live alongside code they test (idiomatic Go)
- Integration tests live separately in `tests/integration/`
- All new features and changes require corresponding tests

For detailed testing guidelines, patterns, and examples, see `docs/TESTING.md`.
```

**Step 3: Run all tests**

```bash
go test ./... -v
```

Expected: All tests pass

**Step 4: Build**

```bash
go build ./...
```

Expected: Build succeeds

**Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: reference TESTING.md in CLAUDE.md"
```

---

## Implementation Complete Checklist

- [ ] testify v1.11.1 dependency added
- [ ] Test helpers created (cmd/dotcor/test_helpers.go)
- [ ] All core tests converted to testify (validator, transaction, backup, lock, hooks, ignore, templates)
- [ ] All config tests converted to testify (config, paths)
- [ ] All fs tests converted to testify (symlink, fs)
- [ ] All git tests converted to testify
- [ ] Migration tests added (migrate_test.go)
- [ ] Command tests added (init, add, remove)
- [ ] Integration tests reorganized and improved
- [ ] Testing documentation created (docs/TESTING.md)
- [ ] CLAUDE.md updated to reference TESTING.md
- [ ] Overall coverage >= 85%
- [ ] All tests pass
- [ ] Build succeeds

---

**Total estimated commits: ~21**

**Total estimated time: 6-8 hours**

**Success criteria:**
- All existing tests converted to testify v1.11.1 with AAA pattern
- Command tests cover major CLI commands (init, add, remove)
- Integration tests reorganized by scenario and improved
- Overall test coverage reaches 85%
- All tests pass: `go test ./...`
- All tests build: `go build ./...`
- Testing documentation (docs/TESTING.md) created and comprehensive
- CLAUDE.md references TESTING.md for testing guidance
- Idiomatic Go test organization (tests next to code)
- New feature testing requirements documented
## Release Tagging

When all tasks in this plan are complete and verified:

```bash
# Step 1: Verify all tests pass
go test ./...

# Step 2: Verify build succeeds
go build ./...

# Step 3: Tag the release
git tag -a v0.5.4 -m "Release: Comprehensive testing implementation with testify and AAA pattern"

# Step 4: Push to remote
git push origin v0.5.4

# Step 5: Push branch
git push origin <feature-branch-name>
```

**Note:** This should be the last step after all implementation and verification is complete.

---


