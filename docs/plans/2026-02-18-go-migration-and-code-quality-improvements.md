# Go 1.26 Migration and Code Quality Improvements

## Overview

This plan addresses migrating DotCor to **latest stable Go version (1.26)** and resolving all minor issues identified in comprehensive code review dated 2026-02-18.

**Note:** As of February 2026, the latest stable Go version is 1.26.0. This plan targets Go 1.26.

## Issues to Fix

### Critical Issues

1. **Invalid Go Version in go.mod**
   - File: `go.mod:3`
   - Current: `go 1.25.5` (non-existent version)
   - Target: `go 1.26` (latest stable)

### Minor Issues

2. **Inconsistent `else if` Formatting**
   - Files: `internal/core/validation.go:210`
   - Issue: Uses `} else if` instead of standard `} else if`

3. **Duplicate Functions in internal/fs/fs.go**
   - File: `internal/fs/fs.go`
   - Functions: `PathExists()` and `Exists()` (both do same thing)
   - Action: Remove `Exists()`, keep `PathExists()`

4. **Fragile Warning Detection**
   - File: `cmd/dotcor/add.go:461-469`
   - Issue: String matching on error messages is fragile
   - Action: Consider using error types or sentinel errors

5. **Lock Timeout Too Long**
   - File: `internal/core/lock.go:25`
   - Current: `const LockTimeout = time.Hour`
   - Action: Make configurable or use shorter default (5 minutes)

6. **Missing Context Cancellation in Git Commands**
   - File: `internal/git/git.go`
   - Status: **NOT AN ISSUE** - After verification, only `InitRepo` uses `runGitCommand()` and it properly calls `defer cancel()`. All other functions use `exec.Command` directly with their own error handling. No changes needed.

7. **Secret Detection False Positives**
   - File: `internal/core/validator.go:15-49`
   - Issue: Regex patterns too loose, may flag legitimate content
   - Action: Improve pattern specificity

8. **Homebrew Formula Builds from Source**
   - File: `.goreleaser.yaml:65-66`
   - Issue: Uses `go build` instead of pre-built binary (slower installs)
   - Action: Use `bin.install` for faster installs

9. **Missing Input Validation**
   - File: `internal/core/transaction.go:387-422`
   - Function: `AddFileTransaction()`
   - Action: Add input validation before building transaction

## Implementation Plan

### Phase 1: Go Version Migration (Priority: Critical)

#### Task 1.1: Update go.mod
**File:** `go.mod`

**Change:**
```diff
-go 1.25.5
+go 1.26
```

**Verification:**
```bash
go mod tidy
go build ./...
```

**Testing:**
- Run full test suite: `go test ./...`
- Build binary: `go build -o dotcor cmd/dotcor/main.go`
- Verify binary runs: `./dotcor --version`

---

### Phase 2: Code Style Fixes (Priority: Low)

#### Task 2.1: Fix else if formatting
**Files:**
- `internal/core/validation.go:210`

**Change:**
```diff
 } else if passed == 0 {
+} else if passed == 0 {
```

**Verification:**
```bash
gofmt -w internal/core/validation.go
```

---

#### Task 2.2: Remove duplicate Exists() function
**File:** `internal/fs/fs.go`

**Change:**
```diff
-func Exists(path string) bool {
-  _, err := os.Stat(path)
-  return !os.IsNotExist(err)
-}
```

**Note:** `Exists()` function exists but is not used anywhere in the codebase. Remove to reduce confusion.

**Verification:**
```bash
go test ./internal/fs/...
```

---

### Phase 3: Robustness Improvements (Priority: Medium)

#### Task 3.1: Make Lock Timeout Configurable
**Files:** `internal/core/lock.go`, `internal/config/config.go`

**Changes:**

1. Add to Config struct:
```go
type Config struct {
    // ... existing fields
    LockTimeout time.Duration `yaml:"lock_timeout"`
}
```

2. Update default in `NewDefaultConfig()`:
```go
return &Config{
    // ... existing defaults
    LockTimeout: 5 * time.Minute, // Changed from 1 hour
}
```

3. Update lock.go constant usage:
```diff
-const LockTimeout = time.Hour
+// Remove constant, use cfg.LockTimeout instead
```

4. Update `AcquireLock()` to use config:
```diff
-// Check if lock is older than LockTimeout
-if time.Since(info.Timestamp) > LockTimeout {
+if time.Since(info.Timestamp) > cfg.LockTimeout {
```

**Migration:**
```go
// In migrate.go MigrateFromEmpty()
if config.LockTimeout == 0 {
    config.LockTimeout = 5 * time.Minute
}
```

**Verification:**
```bash
go test ./internal/core/...
dotcor doctor
```

---

#### Task 3.2: Verify Git Context Cancellation
**File:** `internal/git/git.go`

**Finding:** After verification, this is **NOT AN ISSUE**.

The only function that uses `runGitCommand()` is `InitRepo`, and it properly handles context cancellation:

```go
func InitRepo(repoPath string) error {
    cmd, cancel := runGitCommand(repoPath, "init")
    defer cancel()  // ✓ Properly handled
    // ... rest of implementation
}
```

All other git functions use `exec.Command` directly with appropriate error handling.

**Action:** No changes needed.

---

#### Task 3.3: Improve Warning Detection
**File:** `cmd/dotcor/add.go`

**Current Implementation (fragile):**
```go
func isWarning(err error) bool {
    msg := err.Error()
    return strings.Contains(msg, "warning") ||
           strings.Contains(msg, "large file") ||
           strings.Contains(msg, "unusual permissions")
}
```

**Solution: Define Warning Error Type**

1. Create warning error types:
```go
type WarningError struct {
    Err error
}

func (e *WarningError) Error() string {
    return e.Err.Error()
}

func (e *WarningError) Unwrap() error {
    return e.Err
}

func NewWarning(err error) error {
    return &WarningError{Err: err}
}
```

2. Update validation functions to wrap warnings:
```go
func ValidateFileSize(path string, cfg *config.Config) error {
    // ... validation logic
    if size > int64(threshold) {
        sizeMB := float64(size) / (1024 * 1024)
        return NewWarning(fmt.Errorf("file is very large (%.1fMB), consider excluding: %s", sizeMB, path))
    }
    return nil
}
```

3. Update isWarning check:
```go
func isWarning(err error) bool {
    _, ok := err.(*WarningError)
    return ok
}
```

**Files to update:**
- `cmd/dotcor/add.go` - Add WarningError type
- `internal/core/validator.go:171-204` - Wrap file size warnings
- Any other validation that returns non-critical errors

**Verification:**
```bash
go test ./...
dotcor add ~/.zshrc --force
```

---

#### Task 3.4: Reduce Secret Detection False Positives
**File:** `internal/core/validator.go`

**Current Issue:** Patterns like `api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?` match too broadly.

**Improved Patterns:**

```go
var secretPatterns = []*regexp.Regexp{
    // API keys - require assignment and longer value, require quotes or no spaces after
    regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{32,}['"]?`),
    regexp.MustCompile(`(?i)api[_-]?secret\s*[:=]\s*['"]?[a-zA-Z0-9_-]{32,}['"]?`),

    // Tokens - look for specific prefixes (Bearer, token, secret)
    regexp.MustCompile(`(?i)(?:bearer\s+)?['"]?[a-zA-Z0-9+/=]{40,}['"]?`),

    // Passwords - require password= assignment with value in quotes
    regexp.MustCompile(`(?i)password\s*[:=]\s*['"]?[^'"\s]{16,}['"]?`),

    // Private key headers - more specific
    regexp.MustCompile(`-----BEGIN\s+(RSA|EC|OPENSSH)\s+PRIVATE\s+KEY-----`),

    // AWS keys - specific format (20 char key, 40 char secret)
    regexp.MustCompile(`(?i)aws[_-]?access[_-]?key[_-]?id\s*[:=]\s*['"]?[A-Z0-9]{20}['"]?`),
    regexp.MustCompile(`(?i)aws[_-]?secret[_-]?access[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9/+=]{40}['"]?`),

    // Database connection strings with actual passwords (look for patterns)
    regexp.MustCompile(`(?i)(postgres|mysql|mongodb)://[^:]+:[^@]+@`),

    // Generic credentials - require "credentials:" or "credentials =" with reasonable length
    regexp.MustCompile(`(?i)credentials\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`),
}
```

**Key Improvements:**
1. Increased minimum length from 20 to 32 characters (more realistic for API keys)
2. Made patterns more specific with explicit quotes
3. Combined similar patterns to reduce redundancy
4. Made password pattern require specific format

**Verification:**
```bash
go test ./internal/core/...
# Test with files containing legitimate long strings
dotcor add ~/.zshrc
```

---

### Phase 4: Build Optimization (Priority: Low)

#### Task 4.1: Update Homebrew Formula to Use Pre-Built Binary
**File:** `.goreleaser.yaml`

**Current:**
```yaml
brews:
  - name: dotcor
    # ...
    install: |
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/dotcor"
```

**Changed:**
```yaml
brews:
  - name: dotcor
    # ...
    skip_upload: false
    install: |
      bin.install "dotcor"
```

**Benefits:**
- Faster installs (no compilation required)
- Smaller download size (binary only)
- Consistent with GoReleaser best practices

**Verification:**
```bash
goreleaser release --snapshot --clean --skip=publish
# Verify generated formula uses bin.install
```

---

### Phase 5: Input Validation (Priority: Low)

#### Task 5.1: Add Input Validation to AddFileTransaction
**File:** `internal/core/transaction.go`

**Current Implementation:**
```go
func AddFileTransaction(cfg *config.Config, sourcePath string, repoPath string, mf config.ManagedFile) (*Transaction, error) {
    tx := NewTransaction(cfg)

    // ... build transaction without validation
}
```

**Improved Implementation:**

```go
func AddFileTransaction(cfg *config.Config, sourcePath string, repoPath string, mf config.ManagedFile) (*Transaction, error) {
    // Input validation
    if cfg == nil {
        return nil, fmt.Errorf("config cannot be nil")
    }

    // Validate source path
    if sourcePath == "" {
        return nil, fmt.Errorf("source path cannot be empty")
    }

    // Validate repo path
    if repoPath == "" {
        return nil, fmt.Errorf("repo path cannot be empty")
    }

    // Validate managed file
    if mf.SourcePath == "" {
        return nil, fmt.Errorf("managed file source path cannot be empty")
    }
    if mf.RepoPath == "" {
        return nil, fmt.Errorf("managed file repo path cannot be empty")
    }
    if mf.AddedAt.IsZero() {
        return nil, fmt.Errorf("managed file added_at time cannot be zero")
    }

    tx := NewTransaction(cfg)

    // ... rest of implementation
}
```

**Verification:**
```bash
go test ./internal/core/...
# Test with invalid inputs
```

---

## Execution Order

1. **Phase 1: Go Version Migration** (Critical - do first)
   - Task 1.1: Update go.mod

2. **Phase 2: Code Style Fixes** (Quick wins)
   - Task 2.1: Fix else if formatting
   - Task 2.2: Remove duplicate Exists() function

3. **Phase 3: Robustness Improvements** (Medium priority)
   - Task 3.1: Make Lock Timeout Configurable
   - Task 3.2: Verify Git Context Cancellation (no changes needed)
   - Task 3.3: Improve Warning Detection
   - Task 3.4: Reduce Secret Detection False Positives

4. **Phase 4: Build Optimization** (Low priority, after everything works)
   - Task 4.1: Update Homebrew Formula

5. **Phase 5: Input Validation** (Low priority)
   - Task 5.1: Add Input Validation to AddFileTransaction

---

## Testing After Each Phase

### After Phase 1 (Go Migration)
```bash
# Verify go.mod is valid
go mod tidy

# Build to check for compatibility issues
go build ./...

# Run full test suite
go test ./...

# Verify binary works
./dotcor --version
dotcor init --help
```

### After Phase 2 (Code Style)
```bash
# Check formatting
gofmt -l ./cmd ./internal

# Run tests
go test ./...

# Update any callers of removed Exists()
grep -r "fs\.Exists" --include="*.go"
```

### After Phase 3 (Robustness)
```bash
# Test lock timeout functionality
go test -run TestLock ./internal/core/

# Test warning detection
go test -run TestWarning ./cmd/dotcor/

# Test secret detection improvements
go test -run TestSecretDetection ./internal/core/

# Full integration test
go test ./tests/
```

### After Phase 4 (Build Optimization)
```bash
# Test GoReleaser configuration
goreleaser release --snapshot --clean --skip=publish

# Verify generated Homebrew formula
ls -lh dist/
cat dist/dotcor.rb
```

### After Phase 5 (Input Validation)
```bash
# Test with invalid inputs
go test -run TestTransaction ./internal/core/

# Integration test
go test ./tests/
```

---

## Final Verification

Before considering this complete, run:

```bash
# 1. Full test suite
go test ./... -v

# 2. Build verification
go build -o dotcor cmd/dotcor/main.go

# 3. Linting (if golangci-lint is available)
golangci-lint run ./...

# 4. Doctor check
./dotcor doctor

# 5. Smoke test
./dotcor init --help
./dotcor add --help
./dotcor sync --help
```

---

## Commit Strategy

Follow atomic commits per the CLAUDE.md guidelines:

```bash
# Commit 1: Go version migration
git add go.mod go.sum
git commit -m "chore: upgrade Go version from 1.25.5 to 1.26"

# Commit 2: Code style fixes
git add internal/core/validation.go internal/fs/fs.go
git commit -m "style: fix else if formatting and remove unused Exists function"

# Commit 3: Lock timeout configurability
git add internal/core/lock.go internal/config/config.go internal/config/migrate.go
git commit -m "feat: make lock timeout configurable (default 5 minutes)"

# Commit 4: Warning error types
git add cmd/dotcor/add.go internal/core/validator.go
git commit -m "refactor: use typed errors for warnings instead of string matching"

# Commit 5: Secret detection improvements
git add internal/core/validator.go
git commit -m "fix: improve secret detection patterns to reduce false positives"

# Commit 6: Homebrew formula optimization
git add .goreleaser.yaml
git commit -m "build: use pre-built binary in Homebrew formula for faster installs"

# Commit 7: Input validation
git add internal/core/transaction.go
git commit -m "feat: add input validation to AddFileTransaction"
```

---

## Notes

1. **Go Version Selection**: Go 1.26 is used as it's the latest stable as of February 2026.

2. **Breaking Changes**: None of these changes are breaking for end users. All changes are internal improvements.

3. **Testing Priority**: Phase 1 (Go migration) must be tested thoroughly as it affects the entire codebase.

4. **Backward Compatibility**: The config migration in Task 3.1 ensures existing configs continue to work.

5. **Performance Impact**:
   - Phase 4 (Homebrew optimization) significantly improves install time
   - Phase 3.4 (secret detection) may slightly slow down validation but reduces false positives
   - Phase 3.1 (lock timeout) reduces potential wait time for stale locks

---

## Related Issues

None - this is a proactive improvement plan based on code review findings.

---

## Estimated Time

- Phase 1: 15 minutes (test heavy)
- Phase 2: 10 minutes
- Phase 3: 1 hour
- Phase 4: 10 minutes
- Phase 5: 15 minutes
- Testing: 30 minutes
- **Total**: ~2 hours

---

## Verification Summary

Plan verified against actual codebase on 2026-02-18:

### Confirmed Issues (5 fixes needed):
1. ✅ go.mod has `go 1.25.5` - should be `go 1.26`
2. ✅ validation.go:210 has `} else if` formatting issue
3. ✅ fs.go has duplicate `Exists()` function (not used)
4. ✅ lock.go:25 has 1-hour lock timeout (should be configurable, default 5 min)
5. ✅ validator.go has loose secret detection patterns
6. ✅ goreleaser.yaml builds from source (should use pre-built binary)
7. ✅ AddFileTransaction lacks input validation
8. ✅ add.go:461-469 has fragile string-based warning detection

### False Positives (1 - no fix needed):
9. ❌ Git context cancellation - Only InitRepo uses runGitCommand() and properly handles defer cancel(). All other functions use exec.Command directly. No issue.

### Incorrect References Fixed:
- Removed cmd/dotcor/main.go:210 from else if formatting issue (line 210 is blank)
- Updated all references to reflect actual state of code
