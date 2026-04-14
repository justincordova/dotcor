# DotCor Development Guide

## Project Overview

DotCor is a symlink-based dotfile manager with a Bubble Tea TUI. It uses GNU Stow-style package directories and automatic Git commits, enabling users to manage dotfiles across machines with minimal friction.

**Core Philosophy:**
- Edit dotfiles directly; changes instantly appear in the repository via symlinks
- Git automation handles versioning without manual intervention
- Safety-first with backups, transactions, and locking
- macOS native - Built for macOS with full symlink support

## Architecture

### Technology Stack

- **Language:** Go 1.26
- **TUI Framework:** Bubble Tea (Charm)
- **Styling:** Lip Gloss (Rosé Pine theme)
- **Components:** Bubbles, BubbleZone, Harmonica
- **Configuration:** YAML (`gopkg.in/yaml.v3`)
- **Version Control:** Git (via os/exec)

### Project Structure

```
dotcor/
├── cmd/dotcor/main.go          # Thin entry point: flags, init prompt, launch TUI
│
├── internal/
│   ├── config/
│   │   ├── config.go           # Simplified .dotcorrc load/save
│   │   └── paths.go            # Path normalization, glob expansion
│   │
│   ├── core/
│   │   ├── backup.go           # Backup/restore operations
│   │   ├── lock.go             # File-based locking
│   │   ├── transaction.go      # Transaction/rollback semantics
│   │   ├── ignore.go           # Ignore pattern matching
│   │   ├── hooks.go            # Hook system (pre/post operations)
│   │   └── templates.go        # Simple template substitution
│   │
│   ├── fs/
│   │   ├── fs.go               # File operations (move, copy)
│   │   └── symlink.go          # macOS symlink handling
│   │
│   ├── git/
│   │   └── git.go              # Git command wrapper
│   │
│   ├── logger/
│   │   └── logger.go           # File logging + rotation
│   │
│   └── stow/
│       ├── package.go          # Package discovery, validation
│       ├── link.go             # Symlink creation (individual files)
│       ├── unlink.go           # Symlink removal + empty dir cleanup
│       └── migrate.go          # v1 → v2 migration
│
├── tui/
│   ├── app.go                  # Root Bubble Tea model
│   ├── dashboard.go            # Main dashboard view
│   ├── add_view.go             # Add file flow
│   ├── diff_view.go            # Diff viewer
│   ├── history_view.go         # History browser
│   ├── help_view.go            # Keybinding help
│   ├── logs_view.go            # Log viewer
│   ├── settings_view.go        # Settings editor
│   ├── styles.go               # Rosé Pine Lip Gloss definitions
│   └── keys.go                 # Keybinding definitions
│
├── tests/
│   └── integration_test.go     # Integration tests
│
├── docs/
│   ├── SPEC.md                 # Full specification
│   ├── TESTING.md              # Testing conventions
│   ├── LOGGING.md              # Logging guide
│   └── RELEASING.md            # Release process
│
├── README.md                   # User documentation
└── CLAUDE.md                   # This file
```

### Key Design Decisions

1. **Stow-style packages:** Top-level directories in `~/.dotcor/` mirror `$HOME` path structure
2. **Filesystem-only state:** No managed_files list — packages and symlinks discovered from disk
3. **Individual file symlinks:** Never symlink directories, always individual files with relative paths
4. **Transaction/Rollback:** Wrap multi-step operations to prevent partial failures
5. **File-Based Locking:** Prevent concurrent operations with stale lock detection
6. **Structured Logging:** File-based (`~/.dotcor/logs/dotcor.log`) with TUI log viewer
7. **No CLI framework:** No Cobra/Viper — thin `main.go` parses `os.Args`, launches TUI

### Repository Layout (`~/.dotcor/`)

```
~/.dotcor/
├── .git/
├── .dotcorrc                  # Minimal config (git settings, ignore patterns)
├── logs/dotcor.log            # Rotated, 5MB max
├── backups/                   # Timestamped backups
├── zsh/                       # Package: mirrors $HOME
│   └── .zshrc
├── nvim/
│   └── .config/nvim/init.lua
└── git/
    └── .gitconfig
```

Excluded from packages: `.git`, `logs`, `backups`, `.stow-local-ignore`, `.dotcorrc`.

### Data Flow

```
User presses 's' on "zsh" package in TUI
        │
        ▼ stow.Link()
        │
        ├── Walk zsh/ for files
        ├── For each file:
        │   ├── Backup original if exists
        │   ├── Create parent dirs in $HOME
        │   └── Create relative symlink
        ├── git.AutoCommit("stow zsh")
        │
        ▼
~/.zshrc (symlink) → ../.dotcor/zsh/.zshrc (actual file)
                            │
                            └── Git repository
```

## Coding Standards

### Go Conventions

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names; avoid single-letter names except in loops
- Keep functions focused and small
- Error handling: always check and handle errors explicitly
- Use `internal/` for packages not meant for external consumption

### Error Handling

```go
// Good: explicit error handling with context
if err := someOperation(); err != nil {
    return fmt.Errorf("failed to do X: %w", err)
}

// Bad: ignoring errors
someOperation()
```

### Naming Conventions

- Files: lowercase with underscores (`config_paths.go`)
- Packages: lowercase, single word preferred
- Public functions: PascalCase with clear verb prefixes (`CreateSymlink`, `ValidateFile`)
- Private functions: camelCase

### Testing

- Write tests alongside implementation
- Use table-driven tests for multiple cases
- Test edge cases: empty inputs, missing files, permission errors
- Integration tests for multi-step operations
- TUI tests: call `Update()` with messages, assert state changes

## Design Principles

### Safety First

- Always backup before destructive operations
- Use transactions for multi-step operations
- Never leave system in broken state on failure
- Git commit failures should not fail the main operation

### Fail Gracefully

- Errors appear as status messages in TUI footer
- Critical errors show centered modals with actionable steps
- Conflicts (file exists, not a symlink) show resolution options

### Minimal Surprise

- Follow conventions from similar tools (Stow, Chezmoi)
- Keybindings follow vim/lazygit conventions (j/k, ?, q)
- Default behaviors should be conservative

## Git Workflow

### Commit Guidelines

- **Commit per feature:** Each commit covers one complete feature across all its files
- **Never combine** unrelated features in one commit
- **Clear messages:** Descriptive, present tense, imperative mood

```
# Good commit messages
feat(stow): add package discovery, link, unlink, and v1 migration
refactor(internal): simplify config, logger, and path references for stow layout
feat(tui): add bubble tea foundation with dashboard and views
docs: update documentation, goreleaser, and ci for v2.0

# Bad commit messages
updates
fix stuff
WIP
feat: add styles.go
feat: add keys.go
```

### Branch Strategy

- `main` is the primary branch
- Feature branches for significant changes
- Keep commits clean before merging

### Prohibited Phrases

Never include these in commit messages or code comments:
- "generated by Claude"
- "authored by Claude"
- "written by AI"
- Any similar attribution to AI assistance

## Development Workflow

### Before Making Changes

1. Read existing code in the affected area
2. Understand the current patterns and conventions
3. For architectural changes, discuss before implementing

### During Development

- Make incremental changes over large rewrites
- Preserve existing style and patterns
- Test changes locally before committing
- Keep changes focused and minimal
- **Commit after completing each feature** - don't batch multiple features into one commit

### Commit After Every Feature

When implementing from a plan:
- Complete one full feature (may span multiple files and packages)
- Verify it compiles with `go build ./...`
- Make a commit with a descriptive message
- Use conventional commit format: `feat:`, `fix:`, `refactor:`, etc.
- One commit per feature — not per file, not per session, not per minor task

### Code Review Mindset

- Prefer simple solutions over clever ones
- Question additions that increase complexity
- Verify error handling is complete
- Check for edge cases

## Building and Running

```bash
# Build
go build -o dotcor cmd/dotcor/main.go

# Run (launches TUI)
./dotcor

# Check version
./dotcor --version

# Run tests
go test ./...

# Run specific package tests
go test ./internal/stow/...
```

### CLI Flags (minimal, before TUI launch)

| Flag | Action |
|------|--------|
| `--version` | Print version, exit |
| `--debug` | Set log level to debug |
| `--log-level` | Set log level (debug/info/warn/error) |

## TUI Keybindings

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `s` | Stow (link) selected package |
| `u` | Unstow (unlink) selected package |
| `S` | Sync (git commit + push) |
| `a` | Add file |
| `d` | Remove file |
| `D` | View diff |
| `H` | View history |
| `L` | Toggle log viewer |
| `/?` | Search / Help |
| `q` | Quit |

## Release Workflow

DotCor uses **GoReleaser** for automated multi-platform releases.

### Automated Release Process

Releases are triggered by pushing a version tag:

```bash
# Create and push tag
git tag -a v2.0.0 -m "Release v2.0.0: Description"
git push origin v2.0.0
```

**GitHub Actions automatically:**
1. Builds binaries for macOS (amd64/arm64)
2. Creates GitHub Release with archives and checksums
3. Updates Homebrew tap (justincordova/homebrew-dotcor)

### Testing Releases Locally

Before pushing a real tag:

```bash
# Install GoReleaser
brew install goreleaser

# Test build locally (doesn't publish)
goreleaser release --snapshot --clean --skip=publish

# Verify artifacts
ls -lh dist/
./dist/dotcor_darwin_arm64_v8.0/dotcor --version
```

### Version Injection

Binaries get version from git tag via ldflags:
- `.goreleaser.yaml` configures: `-X main.version={{.Version}}`
- `cmd/dotcor/main.go` declares: `var version = "2.0.0"`
- Built binary shows: `dotcor v2.0.0`

### For Detailed Instructions

See [docs/RELEASING.md](docs/RELEASING.md) for:
- Pre-release checklist
- Monitoring releases
- Rollback procedures
- Troubleshooting

## Key Files Reference

| File | Purpose |
|------|---------|
| `docs/SPEC.md` | Full specification for v2.0 |
| `README.md` | User-facing documentation |
| `docs/TESTING.md` | Testing conventions, patterns, and best practices |
| `docs/LOGGING.md` | Structured logging guide |
| `docs/RELEASING.md` | Release process and GoReleaser workflow |
| `internal/config/config.go` | Config struct and Load/Save operations |
| `internal/stow/package.go` | Package discovery and file scanning |
| `internal/stow/link.go` | Stow (symlink creation) |
| `internal/stow/unlink.go` | Unstow (symlink removal) |
| `internal/core/transaction.go` | Transaction/rollback semantics |
| `internal/core/hooks.go` | Hook system for pre/post operations |
| `internal/fs/symlink.go` | macOS symlink handling |
| `internal/git/git.go` | Git command wrapper |
| `internal/logger/logger.go` | File logging configuration |
| `tui/app.go` | Root Bubble Tea model |
| `tui/dashboard.go` | Main dashboard rendering |
| `tui/styles.go` | Rosé Pine color definitions |

## Common Patterns

### Lock Acquisition

```go
func runCommand(cfg *config.Config) error {
    if err := lock.AcquireLock(cfg); err != nil {
        return err
    }
    defer lock.ReleaseLock(cfg)

    // command logic
}
```

### Transaction Usage

```go
tx := core.NewTransaction(cfg)
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
        panic(r)
    }
}()

if err := tx.Execute(&core.MoveFileOp{Src: src, Dst: dst, Config: cfg}); err != nil {
    return err
}

tx.Commit()
```

### Stow/Unstow

```go
packages, _ := stow.DiscoverPackages(repoDir, homeDir)
pkg := &packages[0]

result, err := stow.Link(pkg, homeDir)
// result.Linked, result.Skipped, result.Errors

result, err = stow.Unlink(pkg, homeDir)
// result.Unlinked, result.Skipped, result.Errors
```

### Path Normalization

```go
normalized, err := config.NormalizePath(absolutePath)
expanded, err := config.ExpandPath(normalizedPath, cfg)
```

### Structured Logging

File-based logging via `log/slog`:

```go
func SomeOperation(path string, cfg *config.Config) error {
    cfg.Logger.Debug("starting operation", "file", path)
    // ... implementation
    cfg.Logger.Info("operation complete", "file", path)
    return nil
}
```

Log output goes to `~/.dotcor/logs/dotcor.log` (not stderr — TUI owns the terminal).

#### Log Level Guidelines

- **DEBUG**: Function entry/exit, internal decisions, intermediate states
- **INFO**: User-relevant events (package stowed, backup created, git commits)
- **WARN**: Non-critical issues (hook failures, large files, retries)
- **ERROR**: Stopping failures (lock held, file not found, permission errors)

For detailed logging documentation, see [docs/LOGGING.md](docs/LOGGING.md).

## Notes for AI Assistants

- Read relevant files before suggesting changes
- Prefer editing existing code over creating new files
- Keep changes minimal and focused
- Ask before making architectural decisions
- Follow existing patterns in the codebase
- Be conservative with external dependencies
- Test suggestions mentally before proposing

## Testing Requirements

**CRITICAL:** When working with tests, always:
1. Reference [docs/TESTING.md](docs/TESTING.md) for testing conventions, patterns, and best practices
2. Write tests for new features, especially big features or significant code changes
3. Ensure tests cover happy paths, error paths, and edge cases
4. Follow AAA pattern (Arrange-Act-Assert) with testify framework
5. Organize tests logically: unit tests alongside code, integration tests in tests/
6. Pre-commit workflow: `go build ./... && go test ./...` before any commit

**When to write tests:**
- All new features must have tests
- Significant changes to existing features need test updates
- Bug fixes should include regression tests
- Core packages (config, core, fs, git, stow, logger) require comprehensive coverage (target 85%+)
- TUI model tests: call `Update()` with messages, assert state changes

Testing documentation at docs/TESTING.md includes:
- testify framework usage (assert/require packages)
- AAA pattern examples
- Test naming conventions
- Coverage goals and pre-commit workflow
