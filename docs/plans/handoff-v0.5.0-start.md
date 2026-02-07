# Handoff: v0.3 and v0.4 Complete

## Current State
**Version:** v0.4.0 (Simple Template System)
**Status:** v0.3.0 and v0.4.0 complete, ready for v0.5.0 (Improved Doctor)

---

## Completed Tasks

### v0.3.0 - Recursive Add

**Implementation:**
- Added `--recursive` flag to add command (`cmd/dotcor/add.go`)
- GetFilesRecursive already implemented in `internal/fs/fs.go`
- Recursive directory support with progress indicator (>10 files shows "Adding N files...")
- Tests exist for GetFilesRecursive in `internal/fs/fs_test.go`

**Changes:**
- `cmd/dotcor/add.go`: Added --recursive flag, recursive directory expansion logic
- `cmd/dotcor/main.go`: Updated version to 0.3.0

**Tagged:** v0.3.0

---

### v0.4.0 - Simple Template System

**Implementation:**
- Created template substitution system (`internal/core/templates.go`)
- Simple string-based substitution (NOT Go templates)
- Variables: `{{ .Hostname }}`, `{{ .OS }}`, `{{ .User }}`, `{{ .Home }}`
- Added `--template` flag to add command
- Templates stored with `.template` extension
- Created `rebuild-links` command to render templates
- Comprehensive tests in `internal/core/templates_test.go`

**Files Created:**
- `internal/core/templates.go` - Template substitution logic
- `internal/core/templates_test.go` - Template system tests
- `cmd/dotcor/rebuild-links.go` - Rebuild symlinks from templates

**Files Modified:**
- `cmd/dotcor/add.go` - Added --template flag and .template extension handling
- `cmd/dotcor/main.go` - Updated version to 0.4.0

**Tagged:** v0.4.0

---

## Next Task: v0.5.0 - Improved Doctor

**PLAN.md Section:** See "### v0.5 - Improved Doctor"

**Requirements:**
- More diagnostic checks (permissions, git config, symlink health)
- Actionable fix suggestions
- `--fix` flag for automatic repairs
- Better output formatting
- Checks: symlink validity, git health, permissions, config validity, locks, git remote, hook permissions

**Current State of Doctor:**
Doctor command exists at `cmd/dotcor/doctor.go` with these checks:
1. ✓ Configuration validity
2. ✓ Lock file (stale detection)
3. ✓ Git repository (git installed, repo initialized, uncommitted changes)
4. ✓ Symlinks (broken, missing, regular file)
5. ✓ Orphaned files in repo

**Missing Checks to Add:**
1. **Permissions check** - Check if files/directories have correct permissions
2. **Git config check** - Verify git user.email and user.name are configured
3. **Git remote check** - Check if remote is configured and accessible
4. **Hook permissions check** - Verify hooks are executable (if they exist)

**Implementation Steps:**
1. Add `checkPermissions()` function to verify file/directory permissions
2. Add `checkGitConfig()` function to verify git user config
3. Add `checkGitRemote()` function to check remote configuration
4. Add `checkHookPermissions()` function to verify hooks are executable
5. Integrate new checks into `runDoctor()` function
6. Improve output formatting (use color-coded status)
7. Add actionable fix suggestions for each issue type

**Fixes to Implement:**
- Create missing directories with correct permissions
- Set git user.email and user.name with defaults if missing
- Add remote if missing (prompt user)
- Make hooks executable with chmod +x

---

## Version Status

| Version | Status | Tag |
|----------|--------|-----|
| v0.1.0   | ✓      | Yes |
| v0.1.1   | ✓      | Yes |
| v0.1.2   | ✓      | Yes |
| v0.2.0   | ✓      | Yes |
| v0.3.0   | ✓      | Yes |
| v0.4.0   | ✓      | Yes |
| v0.5.0   | In Progress | No |
| v0.6.0   | Pending | No |
| v0.7.0   | Pending | No |
| v1.0.0   | Pending | No |

---

## Remaining Tasks to v1.0

### v0.5.0 - Improved Doctor (Current)
- Add permissions check
- Add git config check
- Add git remote check
- Add hook permissions check
- Improve output formatting
- Add actionable fixes

### v0.6.0 - Polish & Bug Fixes
- Address bugs/issues from v0.2-v0.5
- Performance improvements
- UX refinements

### v0.7.0 - Machine Profiles
- Machine profiles with separate managed file lists
- Profile switching (`dotcor set-profile <name>`)
- List profiles (`dotcor list-profiles`)

### v1.0.0 - Production Release
- All v0.2-v0.7 complete
- Comprehensive testing
- Stable, production-ready
- Documentation complete

---

## Development Workflow Reminder

**From PLAN.md - "Development Workflow & Session Handoff" section:**

### Per-Task Process:
1. Write failing test
2. Run test to verify it fails
3. Write minimal implementation
4. Run test to verify it passes
5. Build project (`go build ./...`)
6. Run all tests (`go test ./...`)
7. **Commit changes** (one commit per logical task)

### Pre-Commit Gate:
1. `go build ./...` - Must succeed
2. `go test ./...` - Must pass
3. Run lint if configured
4. **Do NOT commit** if any step fails

### Commit Discipline:
- **Never batch commits** - Each task gets its own commit
- **Commit after EACH task** - Don't accumulate uncommitted work
- **Atomic changes** - One concern per commit
- **Clear commit messages** - Conventional format, descriptive

---

## Git Status

**Branch:** main
**Ahead of origin/main:** 7 commits
**Working tree:** Clean

**Recent Commits:**
```
8fac154 feat: implement simple template system
2f1d006 chore: update version to 0.3.0
42655f2 feat: add --recursive flag to add command for directory support
28a9a43 feat: implement hooks system for add command
817cb2a feat: add hooks to remove, sync, and restore commands
26e41b7 test: add hook system tests and update version to 0.2.0
```

**Local Tags:**
```
v0.1.0
v0.1.1
v0.2.0
v0.3.0
v0.4.0
```

---

## Build & Test Status

**Last Run:**
```
go build ./...     ✓ Success
go test ./...      ✓ All pass
```

**Note:** No tests have been written for the new doctor checks yet.

---

## Key Code References

### Template System
- `internal/core/templates.go:31` - `GetTemplateContext()` - Returns template variables
- `internal/core/templates.go:45` - `SubstituteTemplate()` - Simple string substitution
- `internal/core/templates.go:57` - `IsTemplateFile()` - Check for .template extension
- `cmd/dotcor/rebuild-links.go:47` - `runRebuildLinks()` - Main rebuild logic

### Recursive Add
- `internal/fs/fs.go:166` - `GetFilesRecursive()` - Recursive file walk
- `cmd/dotcor/add.go:76` - Recursive directory expansion logic

---

## Notes

### Template System Design Decisions
- **Simple string substitution** - Not Go templates to keep complexity low
- **Extension-based** - `.template` extension identifies templates
- **No Go template complexity** - Only 4 simple variables supported
- **Rebuild on demand** - `rebuild-links` command renders templates when needed

### Doctor Enhancement Plan
- Check if hooks exist and are executable
- Verify git user config is set
- Check if git remote is configured
- Verify file/directory permissions are correct
- Provide actionable fixes for each issue found

---

## Session Context

**User Requirements:**
- Continue until v1.0.0 is complete
- At v1.0.0 do final code review and integration test
- Keep working until done
- Use executing-plans workflow

**Current Progress:**
- v0.1.0 → v0.4.0: Complete
- v0.5.0 (Doctor): In Progress - ready to implement
- Remaining: v0.6.0, v0.7.0, v1.0.0

---

## Quick Start for Next Session

1. Read this handoff document
2. Read PLAN.md section for v0.5.0 - Improved Doctor
3. Implement missing doctor checks:
   - checkPermissions()
   - checkGitConfig()
   - checkGitRemote()
   - checkHookPermissions()
4. Integrate into `runDoctor()` function
5. Follow per-task workflow (test → implement → build → test → commit)
6. After v0.5.0 complete: update version to 0.5.0 and tag
7. Continue to v0.6.0, v0.7.0, v1.0.0
8. At v1.0.0: Final code review + integration test
