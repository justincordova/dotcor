# DotCor Issues List

Comprehensive code review of v0.1.0 through v0.5.0 identifying issues that need to be addressed before v1.0 production release.

## Summary

After reviewing all source files in the DotCor project, found:

- **8 Critical issues** (data loss risks, broken functionality)
- **15 Important issues** (architecture problems, missing features, poor error handling, test gaps, logic errors)
- **19 Minor issues** (code style, documentation, refactoring opportunities)

**Status:** Not ready for v1.0 production - Critical bugs must be fixed first

**Note:** This file was created during code review and issue tracking. Detailed fix tracking table below.

---

## Critical Issues (Must Fix)

### Issue #1: Permission check inverted
**File:** `cmd/doctor.go:479`
**Severity:** Critical

**What's wrong:**
```go
if repoInfo.Mode().Perm()&0400 != 0 {
    fmt.Printf("  [X] Not readable: %s\n", mf.RepoPath)
    issues++
    // Fix logic...
}
```
The condition checks if the READ bit is **SET** (`!= 0`), but the error message says "Not readable". This is inverted logic.

**Why it matters:**
Files that are NOT readable won't be detected, causing the doctor to report healthy status when files are actually inaccessible.

**How to fix:**
```go
// Change line 479 to:
if repoInfo.Mode().Perm()&0400 == 0 {
    fmt.Printf("  [X] Not readable: %s\n", mf.RepoPath)
    issues++
    // Fix logic...
}
```

**Target Version:** v0.5.1

---

### Issue #2: World-writable check logic error
**File:** `cmd/doctor.go:470-475`
**Severity:** Critical

**What's wrong:**
```go
if mode.Perm()&0002 != 0 {
    // ... fix removes world-writable
    newMode := mode.Perm() & 0755
```
If a file is world-writable (0002 bit set), `mode.Perm() & 0755` clears the world-writable bit but also potentially removes execute permissions for group/other.

**Why it matters:**
Files will lose execute permissions after "fixing" them.

**How to fix:**
```go
// Change line 470 to:
newMode := mode.Perm() &^ 0002  // Only clear world-writable bit
```

**Target Version:** v0.5.1

---

### Issue #3: Unsafe symlink recreation
**File:** `internal/core/transaction.go:80`
**Severity:** Critical

**What's wrong:**
```go
func (op *RemoveSymlinkOp) Undo() error {
    // Recreate symlink
    return os.Symlink(op.savedTarget, op.Link)
}
```
Uses `os.Symlink` instead of the safe `fs.CreateSymlink()` which validates paths and ensures proper relative link creation. No validation that target still exists.

**Why it matters:**
Rollback can create broken symlinks or symlinks to non-existent targets, leaving system in inconsistent state.

**How to fix:**
```go
func (op *RemoveSymlinkOp) Undo() error {
    // Use safe symlink creation with validation
    return fs.CreateSymlink(op.savedTarget, op.Link)
}
```

**Target Version:** v0.5.1

---

### Issue #4: Unsafe backup restoration
**File:** `cmd/add.go:308-311`
**Severity:** Critical

**What's wrong:**
```go
if backupPath != "" {
    if restoreErr := core.RestoreBackup(backupPath, expanded); restoreErr != nil {
        fmt.Fprintf(os.Stderr, "  [!] Failed to restore backup: %v\n", restoreErr)
    }
}
```
If backup creation failed (empty `backupPath`), transaction proceeds anyway. If transaction fails, no backup to restore. No check if `backupPath` is valid before restoration.

**Why it matters:**
Data loss risk - if transaction fails after failed backup, file cannot be restored.

**How to fix:**
```go
// After backup creation in runAdd function:
if backupPath == "" {
    return fmt.Errorf("backup creation failed, aborting operation")
}

// Only proceed if backup was created successfully
```

**Target Version:** v0.5.1

---

### Issue #5: Config mutation without tracking
**File:** `internal/core/transaction.go:47-52`
**Severity:** Critical

**What's wrong:**
```go
func (op *AddToConfigOp) Do() error {
    op.Config.ManagedFiles = append(op.Config.ManagedFiles, op.File)
    return op.Config.SaveConfig()
}
```
Directly mutates slice without tracking original state. `Undo` uses `RemoveManagedFile` which searches by source path - if multiple files have same source path, removes wrong one.

**Why it matters:**
Transaction rollback can remove wrong file or fail silently. Undo operation not idempotent.

**How to fix:**
```go
type AddToConfigOp struct {
    Config    *config.Config
    File      config.ManagedFile
    fileIndex int  // Track index instead of direct mutation
}

func (op *AddToConfigOp) Do() error {
    op.Config.ManagedFiles = append(op.Config.ManagedFiles, op.File)
    op.fileIndex = len(op.Config.ManagedFiles) - 1
    return op.Config.SaveConfig()
}

func (op *AddToConfigOp) Undo() error {
    // Remove by index to ensure we remove the correct file
    if op.fileIndex >= 0 && op.fileIndex < len(op.Config.ManagedFiles) {
        op.Config.ManagedFiles = append(
            op.Config.ManagedFiles[:op.fileIndex],
            op.Config.ManagedFiles[op.fileIndex+1:]...,
        )
        return op.Config.SaveConfig()
    }
    return fmt.Errorf("invalid file index: %d", op.fileIndex)
}
```

**Target Version:** v0.5.1

---

### Issue #6: Undo may corrupt config on duplicate source paths
**File:** `internal/core/transaction.go:67-73`
**Severity:** Critical

**What's wrong:**
```go
func (op *RemoveFromConfigOp) Do() error {
    file, err := op.Config.GetManagedFile(op.sourcePath)
    if err != nil {
        return err
    }
    op.savedFile = file
    return op.Config.RemoveManagedFile(op.sourcePath)
}
```
`RemoveManagedFile` removes **first matching** file by source path. If there are duplicates, removes wrong one. Then `Undo` adds back `savedFile` but another duplicate may remain.

**Why it matters:**
Config can have duplicate entries or missing entries after rollback, leading to inconsistent state.

**How to fix:**
```go
type RemoveFromConfigOp struct {
    Config      *config.Config
    sourcePath  string
    fileIndex   int  // Track index of removed file
    savedFile   config.ManagedFile
}

func (op *RemoveFromConfigOp) Do() error {
    // Find by index instead of source path
    for i, mf := range op.Config.ManagedFiles {
        if mf.SourcePath == op.sourcePath {
            op.savedFile = mf
            op.fileIndex = i
            op.Config.ManagedFiles = append(
                op.Config.ManagedFiles[:i],
                op.Config.ManagedFiles[i+1:]...,
            )
            return op.Config.SaveConfig()
        }
    }
    return fmt.Errorf("managed file not found: %s", op.sourcePath)
}

func (op *RemoveFromConfigOp) Undo() error {
    // Insert back at correct position
    op.Config.ManagedFiles = append(
        op.Config.ManagedFiles[:op.fileIndex],
        append([]config.ManagedFile{op.savedFile}, op.Config.ManagedFiles[op.fileIndex:]...)...,
    )
    return op.Config.SaveConfig()
}
```

**Target Version:** v0.5.1

---

### Issue #7: Race condition in symlink creation
**File:** `internal/fs/symlink.go:62-67`
**Severity:** Critical

**What's wrong:**
```go
if _, err := os.Lstat(expandedLink); err == nil {
    if err := os.Remove(expandedLink); err != nil {
        return fmt.Errorf("removing existing file: %w", err)
    }
}
// Create symlink
if err := os.Symlink(relPath, expandedLink); err != nil {
    return fmt.Errorf("creating symlink: %w", err)
}
```
Between `os.Remove` and `os.Symlink`, another process could create a file, causing symlink creation to fail or link to wrong file.

**Why it matters:**
Not atomic - can create race condition where wrong file is linked or operation fails unexpectedly.

**How to fix:**
```go
// Use atomic rename pattern for cross-platform safety
func CreateSymlink(target, link string) error {
    // ... validation code ...

    expandedLink, _ := paths.ExpandPath(link)

    // Create symlink in temp location first
    tempLink := expandedLink + ".tmp"
    if err := os.Symlink(relPath, tempLink); err != nil {
        return fmt.Errorf("creating temp symlink: %w", err)
    }

    // Atomically rename to target (works on Unix, Windows supports it too)
    if err := os.Rename(tempLink, expandedLink); err != nil {
        os.Remove(tempLink)
        return fmt.Errorf("moving symlink into place: %w", err)
    }

    return nil
}
```

**Target Version:** v0.5.1

---

### Issue #8: Incomplete stale lock handling
**File:** `internal/core/lock.go:70-74`
**Severity:** Critical

**What's wrong:**
```go
if stale {
    // Try to remove stale lock and retry
    if removeErr := os.Remove(lockPath); removeErr != nil {
        info, _ := ReadLockInfo(lockPath)
        return fmt.Errorf("%w: PID %d (process appears dead). Run 'dotcor doctor --fix'", ErrStaleLock, info.PID)
    }
    // Retry lock acquisition after removing stale lock
    return AcquireLock()
}
```
Recursive call to `AcquireLock()` after removing stale lock - if lock file is recreated by another process, infinite recursion possible.

**Why it matters:**
Can cause stack overflow or hang if multiple processes compete for lock.

**How to fix:**
```go
const maxRetries = 3

func AcquireLockWithRetry() error {
    for i := 0; i < maxRetries; i++ {
        err := tryAcquireLock()
        if err == nil {
            return nil
        }

        if errors.Is(err, ErrStaleLock) {
            // Clear stale lock and retry
            clearLock()
            continue
        }

        return err
    }
    return fmt.Errorf("failed to acquire lock after %d attempts", maxRetries)
}
```

**Target Version:** v0.5.1

---

## Important Issues (Should Fix)

### Issue #9: Unsafe file write loses permissions
**File:** `cmd/rebuild-links.go:119`
**Severity:** Important

**What's wrong:**
```go
if err := os.WriteFile(baseRepoPath, []byte(renderedContent), 0644); err != nil {
```
Hardcoded permissions `0644` - overwrites template file permissions. If original had execute permissions, they're lost.

**Why it matters:**
Users lose file permissions after template rendering (e.g., scripts become non-executable).

**How to fix:**
```go
// Get original file permissions
originalMode := os.FileMode(0644)
if info, err := os.Stat(baseRepoPath); err == nil {
    originalMode = info.Mode() & 0777
}

// Write file with preserved permissions
if err := os.WriteFile(baseRepoPath, []byte(renderedContent), originalMode); err != nil {
```

**Target Version:** v0.5.2

---

### Issue #10: Incorrect date format parsing
**File:** `internal/git/git.go:43`
**Severity:** Important

**What's wrong:**
```go
date, _ := time.Parse(time.RFC3339, parts[2])
```
`time.RFC3339` expects format with timezone offset (`+00:00`), but git log without `--date=iso` may not provide this format. Error is silently ignored.

**Why it matters:**
History dates may be incorrect or zero-valued if parsing fails silently.

**How to fix:**
```go
date, err := time.Parse(time.RFC3339, parts[2])
if err != nil {
    // Try other common formats
    date, err = time.Parse(time.RFC1123, parts[2])
    if err != nil {
        date, err = time.Parse("2006-01-02 15:04:05", parts[2])
        if err != nil {
            return CommitInfo{}, fmt.Errorf("unable to parse git date %s: %w", parts[2], err)
        }
    }
}
```

**Target Version:** v0.5.2

---

### Issue #11: Unsafe ahead/behind parsing
**File:** `internal/git/git.go:200`
**Severity:** Important

**What's wrong:**
```go
if len(parts) >= 2 {
    status.BehindBy, _ = strconv.Atoi(parts[0])
    status.AheadBy, _ = strconv.Atoi(parts[1])
}
```
`strconv.Atoi` errors are ignored. If git output format changes, values are silently zero.

**Why it matters:**
Incorrect sync status reporting - won't show pending changes or conflicts.

**How to fix:**
```go
if len(parts) >= 2 {
    var err error
    status.BehindBy, err = strconv.Atoi(parts[0])
    if err != nil {
        return status, fmt.Errorf("failed to parse behind count: %w", err)
    }
    status.AheadBy, err = strconv.Atoi(parts[1])
    if err != nil {
        return status, fmt.Errorf("failed to parse ahead count: %w", err)
    }
}
```

**Target Version:** v0.5.2

---

### Issue #12: Git remote not set before push
**File:** `cmd/clone.go:91`
**Severity:** Important

**What's wrong:**
After cloning, code doesn't check or set git remote. Later operations that try to push will fail.

**Why it matters:**
Users can't sync changes without manually setting git remote, breaking core workflow.

**How to fix:**
```go
// After successful clone, check if remote should be set
if cfg.GitRemote != "" {
    if err := git.SetRemote(repoPath, "origin", cfg.GitRemote); err != nil {
        fmt.Fprintf(os.Stderr, "Warning: failed to set git remote: %v\n", err)
    } else {
        fmt.Println("Git remote configured:", cfg.GitRemote)
    }
}
```

**Target Version:** v0.5.2

---

### Issue #13: Silent error in Execute rollback
**File:** `internal/core/transaction.go:43-50`
**Severity:** Important

**What's wrong:**
```go
func (t *Transaction) Execute(op Operation) error {
    if t.committed {
        return fmt.Errorf("transaction already committed")
    }
    if err := op.Do(); err != nil {
        t.Rollback()
        return fmt.Errorf("executing %s: %w", op.Describe(), err)
    }
    t.executed = append(t.executed, op)
    return nil
}
```
If `Rollback()` fails, the error is lost. Caller only gets original operation error, not that rollback also failed.

**Why it matters:**
System may be in inconsistent state but error doesn't indicate rollback failure.

**How to fix:**
```go
if err := op.Do(); err != nil {
    rollbackErr := t.Rollback()
    if rollbackErr != nil {
        return fmt.Errorf("executing %s: %w (rollback also failed: %v)", op.Describe(), err, rollbackErr)
    }
    return fmt.Errorf("executing %s: %w", op.Describe(), err)
}
```

**Target Version:** v0.5.2

---

### Issue #14: Rollback continues on errors
**File:** `internal/core/transaction.go:59-80`
**Severity:** Important

**What's wrong:**
```go
for i := len(t.executed) - 1; i >= 0; i-- {
    op := t.executed[i]
    if err := op.Undo(); err != nil {
        errs = append(errs, fmt.Errorf("rolling back %s: %w", op.Describe(), err))
    }
}
```
Continues rollback even if operations fail. If undo of file move fails, subsequent undo of config update may leave system corrupted.

**Why it matters:**
Partial rollback can corrupt system state - some operations undone, others not.

**How to fix:**
```go
// Stop rollback on first error to preserve consistent state
for i := len(t.executed) - 1; i >= 0; i-- {
    op := t.executed[i]
    if err := op.Undo(); err != nil {
        // Return immediately - don't continue rolling back
        return fmt.Errorf("rolling back %s: %w (rollback stopped at %d/%d operations)",
            op.Describe(), err, len(t.executed)-i-1, len(t.executed))
    }
}
```

**Target Version:** v0.5.2

---

### Issue #15: IsDirectory logic error
**File:** `internal/fs/fs.go:89-96`
**Severity:** Important

**What's wrong:**
```go
func IsDirectory(path string) (bool, error) {
    info, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            return false, nil
        }
        return false, fmt.Errorf("checking path: %w", err)
    }
    return info.IsDir(), nil
}
```
Returns `(false, nil)` for non-existent paths instead of `(false, error)`. But returns error for other errors like permission denied - inconsistent.

**Why it matters:**
Callers may not distinguish between "not a directory" and "can't check".

**How to fix:**
```go
func IsDirectory(path string) (bool, error) {
    info, err := os.Stat(path)
    if err != nil {
        return false, fmt.Errorf("checking path: %w", err)
    }
    return info.IsDir(), nil
}

// Separate helper for existence check if needed
func Exists(path string) bool {
    _, err := os.Stat(path)
    return !os.IsNotExist(err)
}
```

**Target Version:** v0.5.2

---

### Issue #16: Category map missing common configs
**File:** `internal/config/paths.go:92-100`
**Severity:** Important

**What's wrong:**
Category map is incomplete. Missing many common dotfiles:
- `.zshenv`, `.bashenv`
- `.ssh/config`
- `.profile`, `.bash_profile`, `.zprofile` (partial coverage)

**Why it matters:**
Files get categorized as "misc" instead of logical categories, making repo organization messy.

**How to fix:**
```go
var categoryMap = map[string]string{
    // Shell configurations
    ".zshrc": "shell",
    ".zshenv": "shell",
    ".zprofile": "shell",
    ".zlogin": "shell",
    ".zlogout": "shell",
    ".bashrc": "shell",
    ".bash_profile": "shell",
    ".bash_logout": "shell",
    ".bashenv": "shell",
    ".profile": "shell",
    ".sh_history": "shell",

    // Git
    ".gitconfig": "git",
    ".gitignore": "git",
    ".gitignore_global": "git",

    // SSH
    "ssh/config": "ssh",

    // Editors
    ".vimrc": "vim",
    ".vim": "vim",
    ".nvimrc": "nvim",
    ".editorconfig": "editor",

    // Terminal multiplexers
    ".tmux.conf": "tmux",
    ".screenrc": "screen",
}
```

**Target Version:** v0.5.2

---

### Issue #17: Template processing without validation
**File:** `cmd/rebuild-links.go:74-79`
**Severity:** Important

**What's wrong:**
```go
for _, mf := range managedFiles {
    if !core.IsTemplateFile(mf.RepoPath) {
        skipped++
        continue
    }
    // ... process all template files
}
```
Processes ALL files with `.template` extension even if they're not actually templates. No validation that file contains template variables.

**Why it matters:**
Files accidentally named `.template` get re-rendered even if they shouldn't be templates.

**How to fix:**
```go
func containsTemplateVariables(content string) bool {
    variables := []string{
        "{{ .Hostname }}",
        "{{ .OS }}",
        "{{ .User }}",
        "{{ .Home }}",
    }
    for _, v := range variables {
        if strings.Contains(content, v) {
            return true
        }
    }
    return false
}

// In rebuild-links.go:
for _, mf := range managedFiles {
    if !core.IsTemplateFile(mf.RepoPath) {
        continue
    }

    // Check if file actually contains template variables
    content, err := os.ReadFile(filepath.Join(repoPath, mf.RepoPath))
    if err != nil {
        fmt.Printf("  [!] Error reading %s: %v\n", mf.RepoPath, err)
        continue
    }

    if !containsTemplateVariables(string(content)) {
        fmt.Printf("  [!] Skipping %s: no template variables found\n", mf.RepoPath)
        continue
    }

    // Process template...
}
```

**Target Version:** v0.5.2

---

### Issue #18: Backup directory creation not validated
**File:** `internal/core/backup.go:34-53`
**Severity:** Important

**What's wrong:**
```go
timestampDir := filepath.Join(backupDir, timestamp)
if err := fs.EnsureDir(timestampDir); err != nil {
    return "", fmt.Errorf("creating backup directory: %w", err)
}
```
If `EnsureDir` fails because path exists as a file, backup creation fails silently (or with confusing error).

**Why it matters:**
Collisions with existing files prevent backup, but error doesn't clearly indicate of conflict.

**How to fix:**
```go
timestampDir := filepath.Join(backupDir, timestamp)

// Check if path exists and is a file (not directory)
if info, err := os.Stat(timestampDir); err == nil && !info.IsDir() {
    return "", fmt.Errorf("backup path exists as file, not directory: %s", timestampDir)
}

if err := fs.EnsureDir(timestampDir); err != nil {
    return "", fmt.Errorf("creating backup directory: %w", err)
}
```

**Target Version:** v0.5.2

---

### Issue #19: addFile doesn't use transactions
**File:** `cmd/add.go:353-367`
**Severity:** Important

**What's wrong:**
Function manually moves files and creates symlinks without transaction wrapper. No rollback on failure.

**Why it matters:**
Inconsistent with rest of codebase - no proper transactional safety for init interactive mode.

**How to fix:**
```go
func addFile(sourcePath, repoPath string, template bool) error {
    // ... validation ...

    // Use transaction like main add command
    tx := core.NewTransaction()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()

    // Backup
    backupPath, err := core.CreateBackup(expanded)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Warning: backup failed: %v\n", err)
    }

    // Move file
    if err := tx.Execute(&core.MoveFileOp{expanded, fullRepoPath}); err != nil {
        return err
    }

    // Create symlink
    if err := tx.Execute(&core.CreateSymlinkOp{fullRepoPath, expanded}); err != nil {
        return err
    }

    // Add to config
    if err := tx.Execute(&core.AddToConfigOp{cfg, managedFile}); err != nil {
        return err
    }

    tx.Commit()
    return nil
}
```

**Target Version:** v0.5.2

---

### Issue #20: MoveFile partial failure
**File:** `internal/fs/fs.go:19-36`
**Severity:** Important

**What's wrong:**
```go
if err := os.Remove(src); err != nil {
    os.Remove(dst)  // Cleanup if remove fails
    return fmt.Errorf("removing original file: %w", err)
}
```
If `os.Rename` fails (cross-device), copy succeeds, but `os.Remove` fails, we remove `dst`. Then error is returned but original file still exists.

**Why it matters:**
Can lose to destination file if remove fails after successful copy.

**How to fix:**
```go
func MoveFile(src, dst string) error {
    // ... create dest dir ...

    // Check if dst exists before copying
    dstExisted := fs.FileExists(dst)

    err := os.Rename(src, dst)
    if err == nil {
        return nil
    }

    // Cross-device move, use copy
    if err := CopyWithPermissions(src, dst); err != nil {
        return fmt.Errorf("copying file: %w", err)
    }

    if err := os.Remove(src); err != nil {
        // Only remove dst if we created it (not if it existed before)
        if !dstExisted {
            os.Remove(dst)
        }
        return fmt.Errorf("removing original file: %w", err)
    }
    return nil
}
```

**Target Version:** v0.5.2

---

### Issue #21: Empty config version handling
**File:** `internal/config/config.go:90-91`
**Severity:** Important

**What's wrong:**
Creates default config if file doesn't exist, but doesn't initialize `managed_files` slice explicitly (relies on struct default).

**Why it matters:**
If struct initialization changes, managed files might not be empty list.

**How to fix:**
```go
func NewDefaultConfig() *Config {
    return &Config{
        Version:        CurrentConfigVersion,
        RepoPath:       filepath.Join(homeDir, ".dotcor", "files"),
        GitEnabled:     true,
        IgnorePatterns: GetDefaultIgnorePatterns(),
        ManagedFiles:   []ManagedFile{},  // Explicit initialization
    }
}
```

**Target Version:** v0.5.2

---

### Issue #22: Clone doesn't validate URL
**File:** `internal/git/git.go:314`
**Severity:** Important

**What's wrong:**
```go
func Clone(url, destPath string) error {
    cmd := exec.Command("git", "clone", url, destPath)
```
No validation that `url` looks like a valid git URL (starts with `http://`, `https://`, `git@`, etc.).

**Why it matters:**
User typos cause confusing errors instead of immediate validation.

**How to fix:**
```go
func isValidGitURL(url string) bool {
    prefixes := []string{
        "http://",
        "https://",
        "git://",
        "git@",
        "ssh://",
    }
    for _, prefix := range prefixes {
        if strings.HasPrefix(url, prefix) {
            return true
        }
    }
    return false
}

func Clone(url, destPath string) error {
    if !isValidGitURL(url) {
        return fmt.Errorf("invalid git URL format: %s", url)
    }
    // ... rest of clone logic
}
```

**Target Version:** v0.5.2

---

### Issue #23: Colorize function never called
**File:** `cmd/diff.go:175-201`
**Severity:** Important

**What's wrong:**
Function `colorize()` is defined but never called. Diff output is always plain text even when terminal supports colors.

**Why it matters:**
Missing feature - users don't get colored diff output that was intended.

**How to fix:**
```go
// In runDiff function:
if !nameOnly && !statFlag {
    output, err = getDiff(repoPath, filePath, staged)
    if err == nil {
        output = colorize(output)  // Add this line
    }
}
```

**Target Version:** v0.5.2

---

## Minor Issues (Nice to Have)

### Issue #24: Typo in regex (space in character class)
**File:** `internal/core/validator.go:17`
**Severity:** Minor

**What's wrong:**
`regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`)`
Regex has `[a-zA-Z0-9_-]` which includes space before `_` and after `9`.

**Why it matters:**
Secret detection may not work correctly if patterns have spaces.

**How to fix:**
```go
regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9_-]{20,}['"]?`)
```
Remove space: `[a-zA-Z0-9_-]`

**Target Version:** v0.5.3

---

### Issue #25: Typo in variable name
**File:** `internal/core/validator.go:23`
**Severity:** Minor

**What's wrong:**
Same issue as #24 - space in character class in regex pattern.

**How to fix:**
Remove space from character class.

**Target Version:** v0.5.3

---

### Issue #26: Inconsistent error handling
**Files:** Multiple locations
**Severity:** Minor

**What's wrong:**
Some functions log errors and continue, others return errors immediately. Inconsistent approach.

**Why it matters:**
Makes code harder to understand and debug.

**How to fix:**
Establish consistent error handling policy:
- Log warnings for non-critical failures
- Return errors for critical failures
- Document behavior in function docs

**Target Version:** v0.5.3

---

### Issue #27: Windows path handling incomplete
**File:** `internal/config/paths.go:56-67`
**Severity:** Minor

**What's wrong:**
```go
if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
    path = filepath.Join(home, path[2:])
}
```
On Windows, home paths use backslashes, but this only checks forward slash. `~\foo` not handled.

**Why it matters:**
Windows users with certain home directory paths may have issues.

**How to fix:**
```go
if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") ||
   strings.HasPrefix(path, "~"+string(filepath.Separator)) {
    path = filepath.Join(home, path[2:])
}
```

**Target Version:** v0.5.3

---

### Issue #28: Executable check readability
**File:** `internal/core/hooks.go:46-51`
**Severity:** Minor

**What's wrong:**
```go
if info.Mode().Perm()&0111 == 0 {
    return nil
}
```
Uses bitwise check that's less readable.

**How it matters:**
Slightly less readable code.

**How to fix:**
```go
if !info.Mode().Perm().IsRegular() || info.Mode().Perm()&0111 == 0 {
    return nil
}
```

**Target Version:** v0.5.3

---

### Issue #29: No input validation in commands
**Files:** `cmd/add.go`, `cmd/remove.go`, `cmd/restore.go`
**Severity:** Minor

**What's wrong:**
Many commands don't validate input parameters before using them.

**Why it matters:**
Users get cryptic errors instead of immediate feedback on invalid input.

**How to fix:**
Add input validation at command entry points with clear error messages.

**Target Version:** v0.5.3

---

### Issue #30: Duplicate status checking logic
**File:** `cmd/main.go:82-134`
**Severity:** Minor

**What's wrong:**
`showQuickStatus` duplicates logic from `status.go`.

**Why it matters:**
Code duplication, maintenance burden.

**How to fix:**
Extract common status checking to shared function.

**Target Version:** v0.5.3

---

### Issue #31: Missing error context
**Files:** Multiple
**Severity:** Minor

**What's wrong:**
Error messages don't include file path or context.

**Why it matters:**
Makes debugging harder.

**How to fix:**
Include more context in error messages.

**Target Version:** v0.5.3

---

### Issue #32: Typo in variable name
**File:** `internal/core/ignore.go:28`
**Severity:** Minor

**What's wrong:**
Variable has inconsistent spelling.

**How to fix:**
Fix variable name to be consistent.

**Target Version:** v0.5.3

---

### Issue #33: No timeout on git commands
**File:** `internal/git/git.go`
**Severity:** Minor

**What's wrong:**
Git commands have no timeout.

**Why it matters:**
CLI can hang indefinitely waiting for git.

**How to fix:**
Add context with timeout to git commands.

**Target Version:** v0.5.3

---

### Issue #34: parseDuration doesn't validate input
**File:** `cmd/cleanup.go:31-73`
**Severity:** Minor

**What's wrong:**
Doesn't validate parsed value is positive.

**Why it matters:**
Users can input invalid values like "-5d".

**How to fix:**
```go
if value <= 0 {
    return 0, fmt.Errorf("duration must be positive")
}
```

**Target Version:** v0.5.3

---

### Issue #35: Windows process check incomplete
**File:** `internal/core/lock.go:250-256`
**Severity:** Minor

**What's wrong:**
Can't distinguish between "process doesn't exist" and "no permission to check".

**Why it matters:**
Could improve error handling.

**How to fix:**
Accept limitation with clear documentation, or use platform-specific APIs to distinguish.

**Target Version:** v0.5.3

---

### Issue #36: Function naming confusing
**File:** `cmd/doctor.go:372-420`
**Severity:** Minor

**What's wrong:**
Function is `findOrphanedFiles` but calls `findOrphanedFilesRecursive` with same signature.

**How it matters:**
Code readability issue.

**How to fix:**
Rename to clarify recursive vs non-recursive, or merge into single function.

**Target Version:** v0.5.3

---

### Issue #37: IsWritable may create temp file in wrong location
**File:** `internal/fs/fs.go:197-219`
**Severity:** Minor

**What's wrong:**
For directories, creates `.dotcor_write_test` inside the directory being tested.

**Why it matters:**
May pollute user directories.

**How to fix:**
Use system temp directory instead.

**Target Version:** v0.5.3

---

### Issue #38: ListBackups slow for many backups
**File:** `internal/core/backup.go:286-309`
**Severity:** Minor

**What's wrong:**
Walks entire backup directory tree to find all backups.

**Why it matters:**
Performance issue with many backups.

**How to fix:**
Use directory listing by timestamp instead of full walk.

**Target Version:** v0.5.3

---

### Issue #39: No version checking in clone command
**File:** `cmd/clone.go:42-69`
**Severity:** Minor

**What's wrong:**
Clone doesn't check if existing config version is compatible.

**Why it matters:**
Could clone incompatible config.

**How to fix:**
After clone, load config and run version check/migration.

**Target Version:** v0.5.3

---

### Issue #40: GetFilesRecursive incomplete error handling
**File:** `internal/config/paths.go:234-242`
**Severity:** Minor

**What's wrong:**
Doesn't handle errors from walk properly.

**Why it matters:**
Callers may process incomplete file list.

**How to fix:**
Track walk errors and return first non-nil error.

**Target Version:** v0.5.3

---

### Issue #41: ValidateAll error handling inconsistent
**File:** `internal/core/validator.go:58-76`
**Severity:** Minor

**What's wrong:**
Large file validation is advisory only, not actually enforced as error.

**Why it matters:**
Unclear behavior.

**How to fix:**
Either make it error or document as advisory only.

**Target Version:** v0.5.3

---

### Issue #42: Missing documentation for complex functions
**Files:** `transaction.go`, `lock.go`, `backup.go`
**Severity:** Minor

**What's wrong:**
Complex functions lack doc comments explaining edge cases.

**Why it matters:**
Makes code harder to understand and maintain.

**How to fix:**
Add godoc comments explaining behavior and edge cases.

**Target Version:** v0.5.3

---

## Versioning Plan

### v0.5.1 - Critical Fixes (8 issues)
Fix data loss risks and broken functionality:
- Fix inverted permission checks (ISSUE #1)
- Fix world-writable check logic (ISSUE #2)
- Fix unsafe symlink recreation (ISSUE #3)
- Fix unsafe backup restoration (ISSUE #4)
- Fix config mutation tracking (ISSUE #5)
- Fix config undo corruption (ISSUE #6)
- Fix race condition in symlink creation (ISSUE #7)
- Fix incomplete stale lock handling (ISSUE #8)

### v0.5.2 - Important Fixes (15 issues)
Improve error handling, test coverage, and logic:
- Fix unsafe file write loses permissions (ISSUE #9)
- Fix incorrect date format parsing (ISSUE #10)
- Fix unsafe ahead/behind parsing (ISSUE #11)
- Fix git remote not set before push (ISSUE #12)
- Fix silent error in Execute rollback (ISSUE #13)
- Fix rollback continues on errors (ISSUE #14)
- Fix IsDirectory logic error (ISSUE #15)
- Fix category map missing common configs (ISSUE #16)
- Fix template processing without validation (ISSUE #17)
- Fix backup directory creation not validated (ISSUE #18)
- Fix addFile doesn't use transactions (ISSUE #19)
- Fix MoveFile partial failure (ISSUE #20)
- Fix empty config version handling (ISSUE #21)
- Fix clone doesn't validate URL (ISSUE #22)
- Fix colorize function never called (ISSUE #23)

### v0.5.3 - Minor Improvements (17 issues)
Code style, documentation, and refactoring:
- Fix typo in regex (ISSUE #24)
- Fix typo in variable name (ISSUE #25)
- Fix inconsistent error handling (ISSUE #26)
- Fix Windows path handling incomplete (ISSUE #27)
- Fix executable check readability (ISSUE #28)
- Add input validation in commands (ISSUE #29)
- Fix duplicate status checking logic (ISSUE #30)
- Add missing error context (ISSUE #31)
- Fix typo in variable name (ISSUE #32)
- Add timeout on git commands (ISSUE #33)
- Fix parseDuration validation (ISSUE #34)
- Improve Windows process check (ISSUE #35)
- Fix function naming confusing (ISSUE #36)
- Fix IsWritable temp file location (ISSUE #37)
- Improve ListBackups performance (ISSUE #38)
- Add version checking in clone command (ISSUE #39)
- Fix GetFilesRecursive error handling (ISSUE #40)
- Fix ValidateAll error handling (ISSUE #41)
- Add missing documentation (ISSUE #42)

### v0.6.0 - Complete v0.5.0 Doctor Implementation
Finish any remaining v0.5.0 requirements:
- Complete testing of all doctor checks
- Verify --fix flag works for all issues
- Update documentation

---

## Fix Tracking Table

| Issue | File | Severity | Target Version | Commit |
|-------|------|----------|----------------|--------|
| #1 | cmd/doctor.go:479 | Critical | v0.5.1 | 0aabf44 |
| #2 | cmd/doctor.go:470 | Critical | v0.5.1 | 94c22af |
| #3 | internal/core/transaction.go:80 | Critical | v0.5.1 | a8c40f8 |
| #4 | cmd/add.go:308 | Critical | v0.5.1 | 73df2c9 |
| #5 | internal/core/transaction.go:47 | Critical | v0.5.1 | a18e0ab |
| #6 | internal/core/transaction.go:67 | Critical | v0.5.1 | a671930 |
| #7 | internal/fs/symlink.go:62 | Critical | v0.5.1 | 559357c |
| #8 | internal/core/lock.go:70 | Critical | v0.5.1 | 2a0ae2c |
| #9 | cmd/rebuild-links.go:119 | Important | v0.5.2 | 249624f |
| #10 | internal/git/git.go:43 | Important | v0.5.2 | 2159bb5 |
| #11 | internal/git/git.go:200 | Important | v0.5.2 | 9451e86 |
| #12 | cmd/clone.go:91 | Important | v0.5.2 | 01afe97 |
| #13 | internal/core/transaction.go:43 | Important | v0.5.2 | b5b15d0 |
| #14 | internal/core/transaction.go:59 | Important | v0.5.2 | 231f93c |
| #15 | internal/fs/fs.go:89 | Important | v0.5.2 | 7aa9655 |
| #15-test | internal/fs/fs_test.go:302 | Important | v0.5.2 | a3b303e |
| #16 | internal/config/paths.go:92 | Important | v0.5.2 | 2ecb152 |
| #17 | cmd/rebuild-links.go:74 | Important | v0.5.2 | ea3f583 |
| #18 | internal/core/backup.go:34 | Important | v0.5.2 | f754f56 |
| #19 | cmd/add.go:353 | Important | v0.5.2 | (not fixed, deferred) | |
| #20 | internal/fs/fs.go:19 | Important | v0.5.2 | eda9cc0 |
| #21 | internal/config/config.go:90 | Important | v0.5.2 | 6df231c |
| #22 | internal/git/git.go:314 | Important | v0.5.2 | (not fixed, deferred) | |
| #23 | cmd/diff.go:175 | Important | v0.5.2 | 2666941 |
| #24-42 | Various | Minor | v0.5.3 | |

---

## Missing Features vs PLAN.md

### v0.2.0 - Hooks System
**Status:** ✅ Implemented
- All hooks implemented with graceful degradation
- Environment variables passed correctly

### v0.3.0 - Recursive Add
**Status:** ✅ Implemented
- `--recursive` flag works correctly
- Directory structure preserved
- Ignore patterns respected

### v0.4.0 - Simple Template System
**Status:** ⚠️ Partially Implemented
- Template substitution works
- ⚠️ Missing: Template rendering validation (ISSUE #17)

### v0.5.0 - Improved Doctor
**Status:** ⚠️ Partially Implemented
- Diagnostic checks implemented
- ⚠️ BUG: Permission checks have logic errors (ISSUE #1, #2)
- ⚠️ Missing: Some checks don't provide actionable suggestions

### v0.1.0 - Core Infrastructure
**Status:** ✅ Implemented
- All core modules implemented correctly

### v0.1.1 - All CLI Commands
**Status:** ✅ Implemented
- All commands and features present

---

## Strengths

1. **Clean architecture** - Well-organized package structure
2. **Comprehensive error handling** - Most functions return errors with context
3. **Transaction/rollback system** - Excellent implementation with Operation interface
4. **Safety-first design** - Backups before destructive operations, lock files
5. **Cross-platform support** - Proper handling of symlinks, paths
6. **Hooks system** - Graceful degradation, proper environment variable passing
7. **Testing** - Test files exist for all core modules
8. **Template system** - Simple and effective variable substitution
9. **Glob pattern support** - Well-implemented for batch operations
10. **Stale lock detection** - Proper process checking on Unix and Windows
11. **Atomic operations** - Uses atomic writes for config, proper lock acquisition
12. **Secret detection** - Comprehensive pattern matching

---

## Recommendations

1. **Fix critical permission check bugs** - These are data integrity issues
2. **Improve transaction rollback** - Stop on first error to preserve state
3. **Add comprehensive input validation** - Validate parameters early
4. **Standardize error handling** - Establish consistent policy
5. **Increase test coverage** - Add tests for edge cases
6. **Improve Windows support** - Thoroughly test on Windows
7. **Add integration tests** - Test full command workflows
8. **Add timeouts to external commands** - Prevent hangs
9. **Refactor duplicate code** - Extract common logic
10. **Improve documentation** - Add godoc comments

---

## Overall Assessment

**Ready for v1.0 production?** **No - With fixes**

**Reasoning:**
The project has solid architecture and most features implemented correctly, but **8 critical bugs** that must be fixed before production:

1. Inverted permission checks in doctor (can miss critical issues)
2. Unsafe transaction rollback (can corrupt state)
3. Race conditions in symlink creation
4. Unsafe file operations without proper validation
5. Config corruption risks in transaction undo operations
6. Incomplete error handling in several critical paths

These issues could lead to **data loss, corrupted configuration, or incorrect status reporting**.

After fixing critical bugs (v0.5.1), codebase would be production-ready. Important (v0.5.2) and minor (v0.5.3) issues are quality improvements that can be addressed iteratively.

**Estimated effort to reach production:** **Medium**
- Critical fixes: 1-2 days
- Important fixes: 2-3 days
- Minor improvements: 1-2 days
- Testing and validation: 1-2 days

Total: ~1 week of focused work to reach production readiness.
