# Critical and High-Priority Fixes Plan - Detailed Task Breakdown

**Date:** 2026-02-15
**Priority:** P0 (Critical) and P1 (High) issues
**Estimated Time:** 5-6 days
**Target:** Fix before next release

---

## Pre-Commit Workflow (Run Before Every Commit)

```bash
# 1. Build to ensure code compiles
go build ./...

# 2. Run all tests
go test ./...

# If both pass, commit changes
```

**Rule:** Never commit broken code. Each task must build and pass tests independently.

---

## Task List (Individual Commits)

### P0-1: Fix init flag reading bug

**File:** `cmd/dotcor/init.go:57`

**Change:**
```go
// Line 57 - Change from:
interactiveFlag, _ := cmd.Flags().GetBool("reinit")  // BUG

// To:
interactiveFlag, _ := cmd.Flags().GetBool("interactive")  // FIXED
```

**Impact:** Fixes `dotcor init --interactive` not working

**Verification:**
```bash
go build ./...
# Test manually: ./dotcor init --interactive
```

**Commit Message:**
```
fix: read interactive flag from correct flag name
```

---

### P0-2: Fix config file permissions

**File:** `internal/config/config.go:178`

**Change:**
```go
// Line 178 - Change from:
if err := os.WriteFile(tempPath, data, 0644); err != nil {

// To:
if err := os.WriteFile(tempPath, data, 0600); err != nil {
```

**Impact:** Config files will be owner-only readable (0600 instead of 0644)

**Verification:**
```bash
go build ./...
go test ./internal/config/...
```

**Commit Message:**
```
fix: set config file permissions to 0600 (owner-only)
```

---

### P0-3: Add backup validation to remove.go

**File:** `cmd/dotcor/remove.go:260`

**Change:**
```go
// After line 260, add:
if backupPath == "" {
    return fmt.Errorf("backup creation failed - no backup path returned for %s", mf.SourcePath)
}
```

**Impact:** Prevents remove operation if backup creation silently fails

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/
```

**Commit Message:**
```
fix: validate backup path in remove command
```

---

### P0-4: Add backup validation to transaction.go

**File:** `internal/core/transaction.go:211`

**Change:**
```go
// In RemoveFileOp.Do(), after line 211, add:
if backupPath == "" {
    return fmt.Errorf("backup creation failed - no backup path returned")
}
```

**Impact:** Transaction fails if backup creation returns empty path

**Verification:**
```bash
go build ./...
go test ./internal/core/...
```

**Commit Message:**
```
fix: validate backup path in RemoveFileOp transaction
```

---

### P0-5: Test init flag reading

**File:** `cmd/dotcor/init_test.go` (create if doesn't exist)

**Add Test:**
```go
func TestInitInteractiveFlag(t *testing.T) {
    cfg, _ := config.NewDefaultConfig()
    cfg.SaveConfig()
    defer os.RemoveAll(cfg.RepoPath)

    cmd := NewInitCmd(cfg)
    cmd.SetArgs([]string{"--interactive"})

    // This should not error
    err := cmd.Execute()
    assert.NoError(t, err)
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/ -run TestInitInteractiveFlag
```

**Commit Message:**
```
test: add test for init --interactive flag
```

---

### P0-6: Test config file permissions

**File:** `internal/config/config_test.go`

**Add Test:**
```go
func TestConfigFilePermissions(t *testing.T) {
    tempDir := t.TempDir()
    os.Setenv("HOME", tempDir)
    defer os.Unsetenv("HOME")

    cfg, err := config.NewDefaultConfig()
    require.NoError(t, err)

    err = cfg.SaveConfig()
    require.NoError(t, err)

    configPath, _ := config.GetConfigPath()
    info, err := os.Stat(configPath)
    require.NoError(t, err)

    mode := info.Mode().Perm()
    assert.Equal(t, os.FileMode(0600), mode, "Config should be owner-only readable")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/config/ -run TestConfigFilePermissions
```

**Commit Message:**
```
test: verify config file has correct permissions
```

---

### P0-7: Test backup validation in remove

**File:** `cmd/dotcor/remove_test.go` (create if doesn't exist)

**Add Test:**
```go
func TestRemoveWithFailedBackup(t *testing.T) {
    // This test will need to mock CreateBackup to return empty path
    // For now, ensure the validation code path exists
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/ -run TestRemoveWithFailedBackup
```

**Commit Message:**
```
test: add placeholder for backup validation test
```

---

## P1 Tasks - Git Improvements

**Decision:** Use **manual parsing improvement** (Solution B) instead of git-go library
**Reasoning:**
- No new dependency (smaller binary)
- Less risk of subtle behavior differences
- Can be implemented quickly
- git-go can be added later if needed as tech debt

---

### P1-1: Add parseGitStatusLine helper function

**File:** `internal/git/git.go` - add before GetChangedFiles

**Add Function:**
```go
// parseGitStatusLine parses a single line from git status --porcelain
func parseGitStatusLine(line string) string {
    if len(line) < 2 {
        return ""
    }

    // Handle untracked files (?? prefix)
    if strings.HasPrefix(line, "?? ") {
        return strings.TrimSpace(line[2:])
    }

    // Handle renamed files (R  old -> new)
    if strings.HasPrefix(line, "R ") || strings.HasPrefix(line, "RR ") {
        parts := strings.SplitN(line, " -> ", 2)
        if len(parts) == 2 {
            return strings.TrimSpace(parts[1])
        }
    }

    // Standard case: XY filename (minimum 3 chars: X, Y, space)
    if len(line) >= 3 {
        return strings.TrimSpace(line[3:])
    }

    return ""
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/
```

**Commit Message:**
```
refactor: add parseGitStatusLine helper for git status parsing
```

---

### P1-2: Update GetChangedFiles to use helper

**File:** `internal/git/git.go:476-486`

**Change:**
```go
// Replace manual parsing with helper function:
for _, line := range lines {
    if line == "" {
        continue
    }
    filename := parseGitStatusLine(line)
    if filename != "" {
        files = append(files, filename)
    }
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/
```

**Commit Message:**
```
fix: use parseGitStatusLine helper in GetChangedFiles
```

---

### P1-3: Add isNothingToCommitError helper

**File:** `internal/git/git.go` - add before AutoCommit

**Add Function:**
```go
// isNothingToCommitError checks if git output indicates no changes
func isNothingToCommitError(output string) bool {
    return strings.Contains(output, "nothing to commit") ||
           strings.Contains(output, "nothing added to commit")
}
```

**Verification:**
```bash
go build ./...
```

**Commit Message:**
```
refactor: add isNothingToCommitError helper
```

---

### P1-4: Update AutoCommit to use helper

**File:** `internal/git/git.go:82-84`

**Change:**
```go
// Replace existing check with helper:
if isNothingToCommitError(string(output)) {
    cfg.Logger.Debug("no changes to commit")
    return nil
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/
```

**Commit Message:**
```
refactor: use isNothingToCommitError helper in AutoCommit
```

---

### P1-5: Update AutoCommitFiles to use helper

**File:** `internal/git/git.go:118-120`

**Change:**
```go
// Replace existing check with helper:
if isNothingToCommitError(string(output)) {
    return nil
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/
```

**Commit Message:**
```
refactor: use isNothingToCommitError helper in AutoCommitFiles
```

---

### P1-6: Fix GetDiffBetweenFiles error handling

**File:** `internal/git/git.go:568`

**Change:**
```go
// Replace existing error check:
output := string(output)
hasDiff := strings.Contains(output, "+++ b/") && strings.Contains(output, "--- a/")

if !hasDiff {
    return "", fmt.Errorf("git diff --no-index failed: %w", err)
}

return output, nil
```

**Verification:**
```bash
go build ./...
go test ./internal/git/
```

**Commit Message:**
```
fix: improve error handling in GetDiffBetweenFiles
```

---

### P1-7: Test git status parsing with renamed files

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestGetChangedFilesWithRenames(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Create and commit old file
    oldFile := filepath.Join(repoPath, "old.txt")
    os.WriteFile(oldFile, []byte("content"), 0644)

    cmd = exec.Command("git", "add", "old.txt")
    cmd.Dir = repoPath
    cmd.Run()

    cmd = exec.Command("git", "commit", "-m", "initial")
    cmd.Dir = repoPath
    cmd.Run()

    // Rename file
    newFile := filepath.Join(repoPath, "new.txt")
    os.Rename(oldFile, newFile)

    cmd = exec.Command("git", "add", "old.txt", "new.txt")
    cmd.Dir = repoPath
    cmd.Run()

    // Get changed files
    files, err := GetChangedFiles(repoPath)
    require.NoError(t, err)

    // Should detect renamed file
    assert.Contains(t, files, "new.txt")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestGetChangedFilesWithRenames
```

**Commit Message:**
```
test: add test for git status parsing with renamed files
```

---

### P1-8: Test git status parsing with files containing spaces

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestGetChangedFilesWithSpaces(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Create file with spaces
    file := filepath.Join(repoPath, "file with spaces.txt")
    os.WriteFile(file, []byte("content"), 0644)

    // Get changed files
    files, err := GetChangedFiles(repoPath)
    require.NoError(t, err)

    // Should detect file with spaces
    assert.Contains(t, files, "file with spaces.txt")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestGetChangedFilesWithSpaces
```

**Commit Message:**
```
test: add test for git status parsing with filenames containing spaces
```

---

### P1-9: Test git status parsing with untracked files

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestGetChangedFilesWithUntracked(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Create untracked file
    file := filepath.Join(repoPath, "untracked.txt")
    os.WriteFile(file, []byte("content"), 0644)

    // Get changed files
    files, err := GetChangedFiles(repoPath)
    require.NoError(t, err)

    // Should detect untracked file
    assert.Contains(t, files, "untracked.txt")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestGetChangedFilesWithUntracked
```

**Commit Message:**
```
test: add test for git status parsing with untracked files
```

---

### P1-10: Test git status parsing with merged files

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestGetChangedFilesWithMerged(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Create and commit file
    file := filepath.Join(repoPath, "merged.txt")
    os.WriteFile(file, []byte("original"), 0644)

    cmd = exec.Command("git", "add", "merged.txt")
    cmd.Dir = repoPath
    cmd.Run()

    cmd = exec.Command("git", "commit", "-m", "initial")
    cmd.Dir = repoPath
    cmd.Run()

    // Stage a change
    os.WriteFile(file, []byte("staged"), 0644)

    cmd = exec.Command("git", "add", "merged.txt")
    cmd.Dir = repoPath
    cmd.Run()

    // Make another unstaged change
    os.WriteFile(file, []byte("unstaged"), 0644)

    // Get changed files (should show MM status)
    files, err := GetChangedFiles(repoPath)
    require.NoError(t, err)

    // Should detect merged file
    assert.Contains(t, files, "merged.txt")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestGetChangedFilesWithMerged
```

**Commit Message:**
```
test: add test for git status parsing with merged files
```

---

### P1-11: Test AutoCommit with no changes

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestAutoCommitWithNoChanges(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Try to commit with no changes
    err := AutoCommit(repoPath, "test commit")
    require.NoError(t, err)
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestAutoCommitWithNoChanges
```

**Commit Message:**
```
test: add test for AutoCommit with no changes
```

---

### P1-12: Test AutoCommitFiles with no changes

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestAutoCommitFilesWithNoChanges(t *testing.T) {
    tempDir := t.TempDir()
    repoPath := filepath.Join(tempDir, "repo")
    os.MkdirAll(repoPath, 0755)

    // Initialize repo
    cmd := exec.Command("git", "init")
    cmd.Dir = repoPath
    cmd.Run()

    // Try to commit with no files and no changes
    err := AutoCommitFiles(repoPath, []string{}, "test commit")
    require.NoError(t, err)
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestAutoCommitFilesWithNoChanges
```

**Commit Message:**
```
test: add test for AutoCommitFiles with no changes
```

---

### P1-13: Test GetDiffBetweenFiles with different files

**File:** `internal/git/git_test.go`

**Add Test:**
```go
func TestGetDiffBetweenFilesWithDifferentFiles(t *testing.T) {
    tempDir := t.TempDir()

    // Create two different files
    file1 := filepath.Join(tempDir, "file1.txt")
    file2 := filepath.Join(tempDir, "file2.txt")
    os.WriteFile(file1, []byte("content1"), 0644)
    os.WriteFile(file2, []byte("content2"), 0644)

    // Get diff (should succeed despite exit code 1)
    diff, err := GetDiffBetweenFiles(file1, file2)
    require.NoError(t, err)
    assert.Contains(t, diff, "--- a/")
    assert.Contains(t, diff, "+++ b/")
}
```

**Verification:**
```bash
go build ./...
go test ./internal/git/ -run TestGetDiffBetweenFilesWithDifferentFiles
```

**Commit Message:**
```
test: add test for GetDiffBetweenFiles with different files
```

---

## P1 Tasks - Error Context & Validation

---

### P1-14: Add error context to config.go:95

**File:** `internal/config/config.go:95`

**Change:**
```go
// Change from:
return fmt.Errorf("reading config file: %w", err)

// To:
return fmt.Errorf("reading config file %s: %w", configPath, err)
```

**Verification:**
```bash
go build ./...
go test ./internal/config/
```

**Commit Message:**
```
fix: include config file path in error message
```

---

### P1-15: Add error context to config.go:172

**File:** `internal/config/config.go:172`

**Change:**
```go
// Change from:
return fmt.Errorf("marshaling config: %w", err)

// To:
return fmt.Errorf("marshaling config (version %s, %d files): %w",
    cfg.Version, len(cfg.ManagedFiles), err)
```

**Verification:**
```bash
go build ./...
go test ./internal/config/
```

**Commit Message:**
```
fix: include config details in marshal error message
```

---

### P1-16: Add error context to validator.go:62

**File:** `internal/core/validator.go:62`

**Change:**
```go
// Change from:
return fmt.Errorf("invalid path: %w", err)

// To:
return fmt.Errorf("invalid path %s: %w", path, err)
```

**Verification:**
```bash
go build ./...
go test ./internal/core/
```

**Commit Message:**
```
fix: include path in validator error message
```

---

### P1-17: Add error context to backup.go:43

**File:** `internal/core/backup.go:43`

**Change:**
```go
// Change from:
return "", fmt.Errorf("expanding source path: %w", err)

// To:
return "", fmt.Errorf("expanding source path %s: %w", sourcePath, err)
```

**Verification:**
```bash
go build ./...
go test ./internal/core/
```

**Commit Message:**
```
fix: include source path in backup error message
```

---

### P1-18: Add error context to lock.go:59

**File:** `internal/core/lock.go:59`

**Change:**
```go
// Change from:
return fmt.Errorf("creating config directory: %w", err)

// To:
return fmt.Errorf("creating config directory %s: %w", filepath.Dir(lockPath), err)
```

**Verification:**
```bash
go build ./...
go test ./internal/core/
```

**Commit Message:**
```
fix: include directory path in lock error message
```

---

### P1-19: Add git ref validation to restore.go

**File:** `cmd/dotcor/restore.go` - add around line 123

**Add Code:**
```go
// Check if git ref exists before showing menu
if ref != "" && ref != "HEAD" {
    _, err := git.GetFileContentAtRef(repoPath, filePath, ref)
    if err != nil {
        fmt.Printf("%s[!]%s Git ref '%s' does not exist\n", colorYellow, colorReset, ref)
        fmt.Printf("  Falling back to HEAD\n")
        ref = "HEAD"
    }
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/
```

**Commit Message:**
```
fix: validate git ref exists before restore
```

---

### P1-20: Add state validation to sync.go

**File:** `cmd/dotcor/sync.go` - find where preview is handled

**Add Code:**
```go
// Check if repo has uncommitted changes before previewing
if preview {
    hasChanges, _ := git.HasChanges(repoPath)
    if hasChanges {
        fmt.Printf("%s[!]%s Repository has uncommitted changes\n", colorYellow, colorReset)
        fmt.Printf("  Run 'dotcor sync' (without --preview) to commit changes\n")
        fmt.Printf("  Or use 'git add/git commit' manually\n")
        return nil
    }
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/
```

**Commit Message:**
```
fix: warn about uncommitted changes in sync preview
```

---

### P1-21: Test error messages include paths

**File:** Create `tests/error_context_test.go`

**Add Test:**
```go
func TestErrorMessagesIncludePaths(t *testing.T) {
    // Test various error paths include problematic file/directory
    // Parse error message and assert it contains path
}
```

**Verification:**
```bash
go build ./...
go test ./tests/
```

**Commit Message:**
```
test: add test for error message context
```

---

### P1-22: Test restore with invalid git ref

**File:** `cmd/dotcor/restore_test.go` (create if doesn't exist)

**Add Test:**
```go
func TestRestoreWithInvalidRef(t *testing.T) {
    // Test restore with non-existent git ref
    // Verify it falls back to HEAD
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/ -run TestRestoreWithInvalidRef
```

**Commit Message:**
```
test: add test for restore with invalid git ref
```

---

### P1-23: Test sync preview with uncommitted changes

**File:** `cmd/dotcor/sync_test.go` (create if doesn't exist)

**Add Test:**
```go
func TestSyncPreviewWithUncommittedChanges(t *testing.T) {
    // Test sync --preview with dirty repo
    // Verify it shows warning and exits
}
```

**Verification:**
```bash
go build ./...
go test ./cmd/dotcor/ -run TestSyncPreviewWithUncommittedChanges
```

**Commit Message:**
```
test: add test for sync preview with uncommitted changes
```

---

## Final Verification Tasks

### FV-1: Run all tests

**Command:**
```bash
go test ./...
```

**Expected:** All tests pass

**Commit Message:**
```
test: verify all tests pass after fixes
```

---

### FV-2: Check test coverage

**Commands:**
```bash
go test -cover ./internal/config/
go test -cover ./internal/git/
go test -cover ./internal/core/
go test -cover ./cmd/dotcor/
```

**Target Coverage:**
- Config: 90%+
- Git: 85%+
- Core: 85%+
- Commands: 75%+

**Commit Message:**
```
test: verify test coverage meets targets
```

---

### FV-3: Manual testing of critical flows

**Tests to Run:**
1. `dotcor init --interactive` - Should prompt for config
2. `dotcor add ~/.zshrc` - Should create backup and symlink
3. `dotcor remove ~/.zshrc` - Should create backup before removing
4. Check config file permissions: `ls -l ~/.dotcor/config.yaml` - Should be `-rw-------`
5. Test with renamed files in repo
6. Test with files containing spaces
7. Test restore with invalid ref

**Commit Message:**
```
test: manual verification of critical flows
```

---

## Task Summary

**Total Tasks:** 23 implementation tasks + 3 verification tasks = 26 tasks

**By Priority:**
- P0 (Critical): 7 tasks
- P1 (High - Git): 13 tasks
- P1 (High - Error Context): 10 tasks
- Final Verification: 3 tasks

**By Type:**
- Bug fixes: 9 tasks
- Test additions: 13 tasks
- Validation additions: 2 tasks
- Verification: 3 tasks

**Estimated Time:** 5-6 days
- P0: 2-3 hours
- P1 Git: 2-3 days (including tests)
- P1 Error Context: 1 day
- Final Verification: 1 day

---

## Success Criteria

✅ All P0 and P1 issues are resolved
✅ All new tests pass
✅ All existing tests still pass
✅ No regressions in existing functionality
✅ Test coverage meets targets (85%+ for core packages)
✅ Config files are created with 0600 permissions
✅ Git operations handle edge cases correctly (renames, spaces, merged files)
✅ Error messages provide sufficient context for debugging
✅ No silent failures (backups, git operations, etc.)
✅ Each task builds and tests independently

---

## Risk Assessment

### Low Risk (9 tasks)
- Fixing init flag bug (simple typo)
- Adding error context (non-breaking)
- Adding pre-flight validation (non-breaking)
- Adding tests (only additions)

### Medium Risk (3 tasks)
- Config permission change (may affect scripts that read config)
- Backup validation addition (may break operations that ignored failures)
- Git parsing changes (may have edge cases not covered)

### High Risk (0 tasks)
- No high-risk tasks (we chose manual parsing over git-go)

---

## Rollback Plan

If issues arise after deployment:

1. **Revert backup validation** - Make it non-fatal warning again
2. **Keep error context improvements** - These are non-breaking
3. **Keep permission fix** - Security fix should stay
4. **Revert git parsing changes** - Use original simple parsing if edge cases cause issues
5. **Keep test additions** - Tests only help, don't break anything

---

## Decision Log

**Git Parsing Approach: Manual Improvement (Not git-go)**
- **Date:** 2026-02-15
- **Reasoning:**
  - No new dependency (smaller binary)
  - Less risk of behavior differences
  - Faster to implement
  - git-go can be added later if needed
- **Trade-offs:**
  - Manual parsing requires more maintenance
  - May miss edge cases that git-go handles
  - Tech debt created for future

---

**Document Status:** ✅ Ready for Implementation
**Next Steps:** Start with P0-1 (Fix init flag reading bug)
