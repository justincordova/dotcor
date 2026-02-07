# Issues Fix History

This file tracks issues identified in the DotCor codebase across versions v0.1.0 through v0.5.3 and their fix commits.

## Original Issues Summary

After reviewing all source files in DotCor project, found:

- **8 Critical issues** (data loss risks, broken functionality)
- **15 Important issues** (architecture problems, missing features, poor error handling, test gaps, logic errors)
- **17 Minor issues** (code style, documentation, refactoring opportunities)

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
| #16 | internal/config/paths.go:92 | Important | v0.5.2 | 2ecb152 |
| #17 | cmd/rebuild-links.go:74 | Important | v0.5.2 | ea3f583 |
| #18 | internal/core/backup.go:34 | Important | v0.5.2 | f754f56 |
| #19 | cmd/add.go:353 | Important | v0.5.2 | (not fixed, deferred) |
| #20 | internal/fs/fs.go:19 | Important | v0.5.2 | eda9cc0 |
| #21 | internal/config/config.go:90 | Important | v0.5.2 | 6df231c |
| #22 | internal/git/git.go:314 | Important | v0.5.2 | (not fixed, deferred) |
| #23 | cmd/diff.go:175 | Important | v0.5.2 | 2666941 |
| #24 | internal/core/validator.go:17 | Minor | v0.5.3 | (not fixed, no actual issue) |
| #25 | internal/core/validator.go:23 | Minor | v0.5.3 | (not fixed, no actual issue) |
| #26 | Multiple locations | Minor | v0.5.3 | (already handled) |
| #27 | internal/config/paths.go:56-67 | Minor | v0.5.3 | ecc2bd4 |
| #28 | internal/core/hooks.go:46-51 | Minor | v0.5.3 | 1710cdd |
| #29 | cmd/add.go, cmd/remove.go, cmd/restore.go | Minor | v0.5.3 | (not fixed, deferred) |
| #30 | cmd/main.go:82-134 | Minor | v0.5.3 | (not fixed, deferred) |
| #31 | Multiple files | Minor | v0.5.3 | (not fixed, deferred) |
| #32 | internal/core/ignore.go:28 | Minor | v0.5.3 | (not fixed, no actual issue) |
| #33 | internal/git/git.go | Minor | v0.5.3 | (not fixed, deferred) |
| #34 | cmd/cleanup.go:31-73 | Minor | v0.5.3 | 1e7f065 |
| #35 | internal/core/lock.go:250-256 | Minor | v0.5.3 | (not fixed, deferred) |
| #36 | cmd/doctor.go:372-420 | Minor | v0.5.3 | (not fixed, LSP error) |
| #37 | internal/fs/fs.go:197-219 | Minor | v0.5.3 | (not fixed, LSP error) |
| #38 | internal/core/backup.go:286-309 | Minor | v0.5.3 | (not fixed, LSP error) |
| #39 | cmd/clone.go:42-69 | Minor | v0.5.3 | (not fixed, deferred) |
| #40 | internal/config/paths.go:234-242 | Minor | v0.5.3 | (not fixed, deferred) |
| #41 | internal/core/validator.go:58-76 | Minor | v0.5.3 | (not fixed, deferred) |
| #42 | transaction.go, lock.go, backup.go | Minor | v0.5.3 | 67c0346 |
