# macOS-First Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove all cross-platform code and make DotCor macOS-only, simplifying codebase and reducing maintenance burden.

**Architecture:** Strip platform detection, platform-specific file filtering, Windows/Linux support code, and update all documentation to reflect macOS-only status.

**Tech Stack:** Go 1.21+, Cobra, Viper, goreleaser

**IMPORTANT:** Before each commit, run:
```bash
go build ./...      # Ensure code compiles
go test ./...       # Ensure all tests pass
```

If tests fail due to changes in a task, update the task to fix the issue before committing.

---

## Task 1: Update GoReleaser configuration

**Files:**
- Modify: `.goreleaser.yaml`

**Step 1: Read current GoReleaser config**

```bash
cat .goreleaser.yaml
```

**Step 2: Edit goos to only include darwin**

Edit `.goreleaser.yaml:13-16` from:
```yaml
    goos:
      - darwin
      - linux
      - windows
```

To:
```yaml
    goos:
      - darwin
```

**Step 3: Verify build works**

```bash
goreleaser release --snapshot --clean --skip=publish
```

Expected: Build succeeds with only darwin targets

**Step 4: Commit**

```bash
git add .goreleaser.yaml
git commit -m "refactor: goreleaser build for macOS only"
```

---

## Task 2: Remove platform filtering from config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Remove Platforms field from ManagedFile struct**

Edit `internal/config/config.go:29-35` from:

```go
// ManagedFile represents a single managed dotfile
type ManagedFile struct {
	SourcePath     string    `yaml:"source_path"`     // ~/.zshrc (normalized, with ~)
	RepoPath       string    `yaml:"repo_path"`       // shell/zshrc (relative to files/)
	AddedAt        time.Time `yaml:"added_at"`        // When the file was added
	Platforms      []string  `yaml:"platforms"`       // ["darwin", "linux"] or empty for all
	HasUncommitted bool      `yaml:"has_uncommitted"` // Track if Git commit failed
}
```

To:

```go
// ManagedFile represents a single managed dotfile
type ManagedFile struct {
	SourcePath     string    `yaml:"source_path"`     // ~/.zshrc (normalized, with ~)
	RepoPath       string    `yaml:"repo_path"`       // shell/zshrc (relative to files/)
	AddedAt        time.Time `yaml:"added_at"`        // When the file was added
	HasUncommitted bool      `yaml:"has_uncommitted"` // Track if Git commit failed
}
```

**Step 2: Remove platform filtering functions**

Edit `internal/config/config.go:224-236` - Remove entire `GetManagedFilesForPlatform()` function.

Edit `internal/config/config.go:273-318` - Remove entire section containing:
- `GetCurrentPlatform()` function
- `ShouldApplyOnPlatform()` function
- `contains()` helper function
- `containsAt()` helper function

**Step 3: Update callers of GetManagedFilesForPlatform in commands**

Find all usages:
```bash
grep -rn "GetManagedFilesForPlatform" cmd/
```

Replace `GetManagedFilesForPlatform()` calls with direct `c.ManagedFiles` access in:
- `cmd/dotcor/main.go:104`
- `cmd/dotcor/list.go:55`
- `cmd/dotcor/status.go:130`
- `cmd/dotcor/doctor.go:281`
- `cmd/dotcor/doctor.go:468`
- `cmd/dotcor/remove.go:70`
- `cmd/dotcor/rebuild-links.go:67`

**Step 4: Update callers of GetManagedFilesForPlatform in tests**

Update `cmd/dotcor/list_test.go`:
- Line 70: Replace `GetManagedFilesForPlatform()` with `ManagedFiles`
- Line 108: Replace `GetManagedFilesForPlatform()` with `ManagedFiles`
- Line 166: Replace `GetManagedFilesForPlatform()` with `ManagedFiles`

**Step 5: Remove "No files configured for this platform" message**

Edit `cmd/dotcor/init.go:180-182` - Remove platform-specific message.

**Step 6: Run tests**

```bash
go test ./internal/config/...
go test ./cmd/dotcor/...
```

Expected: Tests pass after removing platform code

**Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go cmd/dotcor/main.go cmd/dotcor/list.go cmd/dotcor/status.go cmd/dotcor/doctor.go cmd/dotcor/remove.go cmd/dotcor/rebuild-links.go cmd/dotcor/list_test.go cmd/dotcor/init.go
git commit -m "refactor: remove platform filtering from config and commands"
```

---

## Task 3: Simplify symlink handling (remove Windows checks)

**Files:**
- Modify: `internal/fs/symlink.go`
- Modify: `internal/fs/symlink_test.go`

**Step 1: Remove SupportsSymlinks() function**

Edit `internal/fs/symlink.go:176-207` - Delete entire `SupportsSymlinks()` function and its comment.

**Step 2: Remove platform check from CreateSymlink**

Edit `internal/fs/symlink.go:30-38` from:

```go
	// Check if platform supports symlinks
	supported, err := SupportsSymlinks()
	if err != nil {
		return fmt.Errorf("checking symlink support: %w", err)
	}
	if !supported {
		return ErrSymlinkUnsupported
	}
```

To nothing (delete these lines).

**Step 3: Remove ErrSymlinkUnsupported**

Edit `internal/fs/symlink.go:14-15` - Delete error declaration:
```go
var ErrSymlinkUnsupported = errors.New("symlink support required - enable Developer Mode on Windows")
```

**Step 4: Update callers that check symlink support**

Find usages:
```bash
grep -rn "SupportsSymlinks\|ErrSymlinkUnsupported" cmd/
```

Remove symlink support checks from:
- `cmd/dotcor/clone.go:76-86`
- `cmd/dotcor/init.go:66-78`

**Step 5: Remove platform-specific test skips**

Edit `internal/fs/symlink_test.go` - Find all `t.Skip("symlinks not supported on this platform")` lines and remove the platform check guards.

Lines to check: 25, 34, 97, 124, 195, 252, 279, 310, 367, 404

**Step 6: Run tests**

```bash
go test ./internal/fs/...
```

Expected: All symlink tests pass without platform skips

**Step 7: Commit**

```bash
git add internal/fs/symlink.go internal/fs/symlink_test.go cmd/dotcor/clone.go cmd/dotcor/init.go
git commit -m "refactor: remove Windows symlink checks"
```

---

## Task 4: Simplify lock handling (remove Windows-specific code)

**Files:**
- Modify: `internal/core/lock.go`

**Step 1: Remove Windows-specific process alive check**

Edit `internal/core/lock.go:218-264` - Replace entire section:

From:
```go
// isProcessAlive checks if a process with given PID is still running
func isProcessAlive(pid int) (bool, error) {
	if runtime.GOOS == "windows" {
		return isProcessAliveWindows(pid)
	}
	return isProcessAliveUnix(pid)
}

// isProcessAliveUnix checks if process is alive on Unix systems
func isProcessAliveUnix(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil // Process doesn't exist
	}

	// On Unix, signal 0 checks if process exists without killing it
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we don't have permission
		return false, nil
	}

	return true, nil
}

// isProcessAliveWindows checks if process is alive on Windows
// Note: On Windows, os.FindProcess always succeeds, so we check
// if we can signal the process. This is imperfect but works for
// most cases where the PID has been reused or the process is gone.
func isProcessAliveWindows(pid int) (bool, error) {
	// On Windows, we try to find and signal the process
	// FindProcess on Windows doesn't actually verify the process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	// On Windows, Signal(0) returns an error if process doesn't exist
	// or we don't have permission. We treat both as "not alive" for
	// lock staleness purposes since either way we can't communicate with it.
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false, nil
	}

	return true, nil
}
```

To:
```go
// isProcessAlive checks if a process with given PID is still running
func isProcessAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, nil // Process doesn't exist
	}

	// On Unix, signal 0 checks if process exists without killing it
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we don't have permission
		return false, nil
	}

	return true, nil
}
```

**Step 2: Remove unused import**

Edit `internal/core/lock.go:8` - Remove `"runtime"` import if no longer used after cleanup.

**Step 3: Run tests**

```bash
go test ./internal/core/...
```

Expected: Lock tests pass

**Step 4: Commit**

```bash
git add internal/core/lock.go
git commit -m "refactor: remove Windows-specific lock handling"
```

---

## Task 5: Simplify template context (hardcode macOS)

**Files:**
- Modify: `internal/core/templates.go`
- Modify: `internal/core/templates_test.go`

**Step 1: Remove OS field from TemplateContext**

Edit `internal/core/templates.go:10-16` from:

```go
// TemplateContext holds variables for template substitution
type TemplateContext struct {
	Hostname string
	OS       string
	User     string
	Home     string
}
```

To:

```go
// TemplateContext holds variables for template substitution
type TemplateContext struct {
	Hostname string
	User     string
	Home     string
}
```

**Step 2: Remove OS from GetTemplateContext**

Edit `internal/core/templates.go:19-41` from:

```go
// GetTemplateContext returns the current template context
func GetTemplateContext() (*TemplateContext, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	currentUser, err := user.Current()
	if err != nil {
		currentUser = &user.User{HomeDir: "~"}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	return &TemplateContext{
		Hostname: hostname,
		OS:       runtime.GOOS,
		User:     currentUser.Username,
		Home:     home,
	}, nil
}
```

To:

```go
// GetTemplateContext returns the current template context
func GetTemplateContext() (*TemplateContext, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	currentUser, err := user.Current()
	if err != nil {
		currentUser = &user.User{HomeDir: "~"}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}

	return &TemplateContext{
		Hostname: hostname,
		User:     currentUser.Username,
		Home:     home,
	}, nil
}
```

**Step 3: Remove OS substitution from SubstituteTemplate**

Edit `internal/core/templates.go:44-52` from:

```go
// SubstituteTemplates performs simple {{ variable }} substitution
// Supports: {{ .Hostname }}, {{ .OS }}, {{ .User }}, {{ .Home }}
func SubstituteTemplate(content string, ctx *TemplateContext) string {
	result := content
	result = strings.ReplaceAll(result, "{{ .Hostname }}", ctx.Hostname)
	result = strings.ReplaceAll(result, "{{ .OS }}", ctx.OS)
	result = strings.ReplaceAll(result, "{{ .User }}", ctx.User)
	result = strings.ReplaceAll(result, "{{ .Home }}", ctx.Home)
	return result
}
```

To:

```go
// SubstituteTemplates performs simple {{ variable }} substitution
// Supports: {{ .Hostname }}, {{ .User }}, {{ .Home }}
func SubstituteTemplate(content string, ctx *TemplateContext) string {
	result := content
	result = strings.ReplaceAll(result, "{{ .Hostname }}", ctx.Hostname)
	result = strings.ReplaceAll(result, "{{ .User }}", ctx.User)
	result = strings.ReplaceAll(result, "{{ .Home }}", ctx.Home)
	return result
}
```

**Step 4: Update tests**

Edit `internal/core/templates_test.go` - Remove test cases that check for `{{ .OS }}` substitution.

**Step 5: Run tests**

```bash
go test ./internal/core/...
```

Expected: Template tests pass without OS field

**Step 6: Update templates test to remove OS field tests**

Edit `internal/core/templates_test.go`:
- Line 28: Remove OS field from test struct
- Line 46: Remove expected "os=linux" assertion
- Line 151: Remove OS field from test struct
- Line 166: Remove assert.Contains for "darwin"
- Line 172: Remove OS field from test string

**Step 7: Run tests**

```bash
go test ./internal/core/...
```

Expected: Template tests pass without OS field

**Step 8: Commit**

```bash
git add internal/core/templates.go internal/core/templates_test.go
git commit -m "refactor: remove OS template variable (macOS-only)"
```

---

## Task 6: Remove empty Platforms initialization in commands

**Files:**
- Modify: `cmd/dotcor/add.go`
- Modify: `cmd/dotcor/adopt.go`
- Modify: `cmd/dotcor/rebuild.go`

**Step 1: Remove Platforms field from ManagedFile creation in add.go**

Edit `cmd/dotcor/add.go:286-297` - Remove `Platforms: []string{}` line.

**Step 2: Remove Platforms field from ManagedFile creation in adopt.go**

Edit `cmd/dotcor/adopt.go:250-261` - Remove `Platforms: []string{}` line.

**Step 3: Remove Platforms field from ManagedFile creation in rebuild.go**

Edit `cmd/dotcor/rebuild.go:204-215` - Remove `Platforms: []string{}` line.

**Step 4: Remove platforms flag from add command**

Edit `cmd/dotcor/add.go` - Remove platforms flag definition and its usage.

**Step 5: Remove platforms from rebuild-links_test.go**

Edit `cmd/dotcor/rebuild-links_test.go` - Remove `platforms: []` initialization from test cases at lines 43, 110, 168, 208, 248.

**Step 6: Run tests**

```bash
go test ./cmd/dotcor/...
```

Expected: All command tests pass

**Step 7: Commit**

```bash
git add cmd/dotcor/add.go cmd/dotcor/adopt.go cmd/dotcor/rebuild.go cmd/dotcor/rebuild-links_test.go
git commit -m "refactor: remove Platforms field from command implementations"
```

---

## Task 7: Remove platform-specific integration tests

**Files:**
- Modify: `tests/integration_test.go`

**Step 1: Remove TestIntegration_ConfigPlatformFiltering test**

Edit `tests/integration_test.go:484-514` - Remove entire test function and its comment.

**Step 2: Run integration tests**

```bash
go test ./tests/...
```

Expected: Tests pass without platform filtering test

**Step 3: Commit**

```bash
git add tests/integration_test.go
git commit -m "refactor: remove platform filtering integration tests"
```

---

## Task 8: Update README.md to reflect macOS-only status

**Files:**
- Modify: `README.md`

**Step 1: Update Features section**

Edit `README.md:10-18` from:

```
## Features

- **Symlink-based** - Edit files directly, changes instantly appear in your repository
- **Zero-config** - Automatic path organization with sensible defaults
- **Git automation** - Auto-commits after every operation, manual Git usage optional
- **Cross-platform** - Works on macOS, Linux, and Windows
- **Simple CLI** - Easy-to-use commands for everyday dotfile management
- **Git history** - Built-in restore and history commands leveraging Git
```

To:

```
## Features

- **Symlink-based** - Edit files directly, changes instantly appear in your repository
- **Zero-config** - Automatic path organization with sensible defaults
- **Git automation** - Auto-commits after every operation, manual Git usage optional
- **macOS native** - Built for macOS with full symlink support out of box
- **Simple CLI** - Easy-to-use commands for everyday dotfile management
- **Git history** - Built-in restore and history commands leveraging Git
```

**Step 2: Update Cross-Platform Support section**

Edit `README.md:441-468` - Replace entire "Cross-Platform Support" section with:

```
## macOS Support

DotCor is built exclusively for macOS, taking advantage of native symlink support.

### Requirements

- macOS 10.14 (Mojave) or later
- Git (for version control)
```

**Step 3: Update Installation section**

Edit `README.md:24-29` - Remove Linux from Homebrew instructions.

From:

```
**Homebrew (macOS/Linux):**
```bash
brew tap justincordova/dotcor
brew install dotcor
```
```

To:

```
**Homebrew:**
```bash
brew tap justincordova/dotcor
brew install dotcor
```
```

**Step 4: Update comparison tables**

Edit `README.md:470-496` - Remove cross-platform references from comparison tables.

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: update README for macOS-only focus"
```

---

## Task 9: Update CLAUDE.md to remove cross-platform references

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update Project Overview section**

Edit `CLAUDE.md:20-28` - Change cross-platform bullet.

From:

```
- **Cross-platform support** (macOS, Linux, Windows with Developer Mode)
```

To:

```
- **macOS native** - Built for macOS with native symlink support
```

**Step 2: Update Key Design Decisions**

Edit `CLAUDE.md:58-60` - Remove Windows symlink decision.

**Step 3: Update Common Patterns section**

Edit CLAUDE.md - Remove platform-specific examples or patterns.

**Step 4: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for macOS-only focus"
```

---

## Task 10: Update docs/PLAN.md to reflect macOS-only architecture

**Files:**
- Modify: `docs/PLAN.md`

**Step 1: Update Core Design Decisions**

Edit `docs/PLAN.md:11-22` - Remove cross-platform bullet.

From:

```
**Core Design Decisions:**
- **Storage:** Symlink-based (files live in repo, symlinks point to them)
- **Symlink type:** Relative paths (portable across machines and mount points)
- **File granularity:** Individual files with recursive directory support
- **Safety:** Automatic backups before destructive operations + transaction rollback
- **Concurrency:** File-based locking to prevent concurrent operations
- **Git:** Automatic commits after every operation with robust error handling
- **Conflicts:** Let Git handle merges (we provide helpful messages)
- **Config:** YAML format with Viper, versioned for migrations
- **Paths:** Normalize to `~` for portability
- **Cross-platform:** macOS/Linux native, Windows requires Developer Mode (NO FALLBACK)
- **Security:** Secret detection and ignore patterns
```

To:

```
**Core Design Decisions:**
- **Storage:** Symlink-based (files live in repo, symlinks point to them)
- **Symlink type:** Relative paths (portable across machines and mount points)
- **File granularity:** Individual files with recursive directory support
- **Safety:** Automatic backups before destructive operations + transaction rollback
- **Concurrency:** File-based locking to prevent concurrent operations
- **Git:** Automatic commits after every operation with robust error handling
- **Conflicts:** Let Git handle merges (we provide helpful messages)
- **Config:** YAML format with Viper, versioned for migrations
- **Paths:** Normalize to `~` for portability
- **Platform:** macOS native with full symlink support
- **Security:** Secret detection and ignore patterns
```

**Step 2: Remove Windows-specific decision**

Edit docs/PLAN.md - Remove entire "Windows: No Copy Fallback" decision section (around line 276-288).

**Step 3: Remove Windows symlink detection from plan**

Edit docs/PLAN.md:597-673 - Remove Windows symlink detection code and error message from plan.

**Step 4: Remove Windows-specific lock handling from plan**

Edit docs/PLAN.md:958 - Remove Windows lock file pattern note.

**Step 5: Remove Windows-specific path handling notes**

Edit docs/PLAN.md:1921-1926 - Remove Windows vs macOS/Linux path handling comparison.

**Step 6: Remove ADR-006 from plan**

Edit docs/PLAN.md:2149-2151 - Remove "NO Windows copy mode fallback" ADR section.

**Step 7: Remove Windows error messages from plan**

Edit docs/PLAN.md:1267 - Remove Windows-specific error message example.

**Step 3: Update config data model**

Edit docs/PLAN.md:161-166 - Remove platforms field from example.

From:

```yaml
  - source_path: ~/.zshrc
    repo_path: shell/zshrc
    added_at: 2025-01-04T10:30:00Z
    platforms: []                # Empty = all platforms, or ["darwin", "linux", "windows", "wsl"]
    has_uncommitted: false       # Track if add succeeded but commit failed
```

To:

```yaml
  - source_path: ~/.zshrc
    repo_path: shell/zshrc
    added_at: 2025-01-04T10:30:00Z
    has_uncommitted: false       # Track if add succeeded but commit failed
```

**Step 4: Update ManagedFile struct in plan**

Edit docs/PLAN.md:344-350 - Remove Platforms field from struct definition.

**Step 5: Update implementation notes**

Edit docs/PLAN.md:419 - Remove platform detection note.

**Step 6: Update path utilities section**

Edit docs/PLAN.md:472-489 - Remove platform-related functions from plan.

**Step 7: Update symlink handling notes**

Edit docs/PLAN.md - Remove Windows symlink detection code (around line 646-673).

**Step 8: Commit**

```bash
git add docs/PLAN.md
git commit -m "docs: update PLAN.md for macOS-only architecture"
```

---

## Task 11: Update additional documentation files

**Files:**
- Modify: `docs/RELEASING.md`
- Modify: `docs/GORELEASER_PLAN.md`
- Modify: `docs/TESTING.md`
- Modify: `docs/LOGGING.md`
- Modify: `docs/ISSUES.md`

**Step 1: Update docs/RELEASING.md**

Search and update Linux/Windows references:
```bash
grep -n "Linux\|Windows" docs/RELEASING.md
```

Update build targets section to remove Linux and Windows entries.

**Step 2: Update docs/GORELEASER_PLAN.md**

Search and update Linux/Windows references:
```bash
grep -n "Linux\|Windows" docs/GORELEASER_PLAN.md
```

Update to reflect macOS-only build targets.

**Step 3: Update docs/TESTING.md**

Search and update cross-platform testing references:
```bash
grep -n "cross.?platform\|Linux\|Windows" docs/TESTING.md
```

Remove platform-specific testing requirements.

**Step 4: Check docs/LOGGING.md for platform references**

```bash
grep -n "Linux\|Windows" docs/LOGGING.md
```

Update if any platform references found.

**Step 5: Check docs/ISSUES.md for platform references**

```bash
grep -n "Linux\|Windows" docs/ISSUES.md
```

Update if any platform references found.

**Step 6: Commit**

```bash
git add docs/RELEASING.md docs/GORELEASER_PLAN.md docs/TESTING.md docs/LOGGING.md docs/ISSUES.md
git commit -m "docs: update additional docs for macOS-only focus"
```

---

## Task 12: Final validation and cleanup

**Files:**
- Multiple

**Step 1: Search for remaining runtime.GOOS usage**

```bash
grep -rn "runtime\.GOOS" --include="*.go" internal/ cmd/
```

Review any remaining usage - should be none.

**Step 2: Search for remaining platform references in Go code**

```bash
grep -rn "platform\|Platform" --include="*.go" internal/ cmd/ | grep -v "//"
```

Review and remove any remaining platform-specific code.

**Step 3: Search for remaining Linux/Windows strings in Go code**

```bash
grep -rn '"linux"\|"windows"\|"darwin"' --include="*.go" internal/ cmd/
```

Review and update any hardcoded OS strings.

**Step 4: Search for remaining cross-platform mentions in docs**

```bash
grep -rn "cross.?platform\|Linux\|Windows" --include="*.md" . | grep -v "plans/"
```

Update any remaining references in documentation.

**Step 5: Run full test suite**

```bash
go test ./...
```

Review and remove any remaining platform-specific code.

**Step 2: Search for remaining cross-platform mentions in docs**

```bash
grep -rn "cross.?platform\|Linux\|Windows" --include="*.md" .
```

Update any remaining references.

**Step 3: Run full test suite**

```bash
go test ./...
```

Expected: All tests pass

**Step 4: Build project**

```bash
go build ./...
```

Expected: Build succeeds without errors

**Step 5: Build release binaries**

```bash
goreleaser release --snapshot --clean --skip=publish
```

Expected: Only darwin binaries built

**Step 6: Verify functionality**

```bash
./dist/dotcor_darwin_arm64_v*/dotcor --version
./dist/dotcor_darwin_amd64/dotcor --version
```

Expected: Version displayed correctly

**Step 7: Commit any cleanup changes**

```bash
git add -A
git commit -m "refactor: final cleanup for macOS-only refactor"
```

---

## Task 13: Create summary document

**Files:**
- Create: `docs/plans/2026-02-09-macos-first-refactor-summary.md`

**Step 1: Create summary document**

Write comprehensive summary documenting all changes, metrics, and impact.

**Step 2: Commit**

```bash
git add docs/plans/2026-02-09-macos-first-refactor-summary.md
git commit -m "docs: add macOS-first refactor summary"
```

---

## Post-Implementation Notes

After completing all tasks:

1. **Verify build**: Ensure `go build ./...` succeeds
2. **Run tests**: Ensure `go test ./...` passes
3. **Test release**: Verify goreleaser builds only macOS binaries
4. **Update CHANGELOG** (if exists): Document breaking change
5. **Tag version**: Consider tagging as breaking release (v0.3.0 or v1.0.0)

**Breaking Changes:**
- Config files with `platforms` field will ignore it (backward compatible)
- Existing installations on Linux/Windows will not receive updates
- Template files using `{{ .OS }}` will need to be updated

**Migration Path:**
- Existing users: No migration needed (config is backward compatible)
- New users: macOS-only tooling is clear from documentation
- Future Linux support: Can re-add platform filtering incrementally
