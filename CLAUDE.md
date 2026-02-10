# DotCor Development Guide

## Project Overview

DotCor is a symlink-based dotfile manager built in Go. It combines the simplicity of GNU Stow with automatic Git commits, enabling users to manage dotfiles across machines with minimal friction.

**Core Philosophy:**
- Edit dotfiles directly; changes instantly appear in the repository via symlinks
- Git automation handles versioning without manual intervention
- Safety-first with backups, transactions, and locking
- macOS native - Built for macOS with full symlink support

## Architecture

### Technology Stack

- **Language:** Go 1.21+
- **CLI Framework:** Cobra
- **Configuration:** Viper (YAML)
- **Version Control:** Git (via os/exec)

### Project Structure

```
dotcor/
├── cmd/dotcor/               # CLI commands (Cobra)
│   ├── main.go               # Entry point, root command
│   ├── init.go               # dotcor init
│   ├── add.go                # dotcor add
│   ├── remove.go             # dotcor remove
│   ├── list.go               # dotcor list
│   ├── status.go             # dotcor status
│   ├── sync.go               # dotcor sync
│   ├── restore.go            # dotcor restore
│   ├── history.go            # dotcor history
│   ├── diff.go               # dotcor diff
│   ├── adopt.go              # dotcor adopt
│   ├── doctor.go             # dotcor doctor
│   ├── rebuild.go            # dotcor rebuild-config
│   ├── rebuild-links.go      # dotcor rebuild-links (render templates)
│   ├── clone.go              # dotcor clone
│   ├── cleanup.go            # dotcor cleanup-backups
│   ├── test_helpers.go       # Test utilities
│   └── *_test.go            # Command tests
│
├── internal/
│   ├── config/               # Configuration management
│   │   ├── config.go         # Config struct, Load/Save
│   │   ├── paths.go          # Path normalization, glob expansion
│   │   └── migrate.go        # Config version migrations
│   │
│   ├── core/                 # Business logic
│   │   ├── validator.go      # Validation, secret detection
│   │   ├── backup.go         # Backup/restore operations
│   │   ├── lock.go           # File-based locking
│   │   ├── transaction.go    # Transaction/rollback semantics
│   │   ├── ignore.go         # Ignore pattern matching
│   │   ├── hooks.go          # Hook system (pre/post operations)
│   │   └── templates.go      # Simple template substitution
│   │
 │   ├── fs/                   # File system operations
 │   │   ├── fs.go             # File operations (move, copy)
 │   │   └── symlink.go        # macOS symlink handling
 │   │
│   ├── git/                  # Git integration
│   │   └── git.go            # Git command wrapper
│   │
│   └── logger/               # Structured logging
│       └── logger.go         # slog configuration
│
├── tests/
│   └── integration_test.go    # Integration tests
│
├── docs/
│   ├── LOGGING.md            # Logging guide
│   ├── TESTING.md            # Testing conventions and patterns
│   ├── RELEASING.md          # Release process
│   └── PLAN.md               # Implementation plan
│
├── README.md                 # User documentation
└── CLAUDE.md                 # This file
```

### Key Design Decisions

1. **Relative Symlinks:** Use `filepath.Rel()` for portability across machines
2. **Transaction/Rollback:** Wrap multi-step operations to prevent partial failures
3. **File-Based Locking:** Prevent concurrent operations with stale lock detection
4. **Secret Detection:** Scan for API keys, passwords, tokens before adding files
5. **Versioned Config:** Include version field for future schema migrations
6. **Structured Logging:** Use `log/slog` with dependency injection, separate from user-facing output
7. **Hook System:** Pre/post operation hooks in `~/.dotcor/hooks/` for extensibility
8. **Simple Templates:** Basic `{{ .Hostname }}` substitution via `rebuild-links` command

### Data Flow

```
User's dotfile (~/.zshrc)
        │
        ▼ dotcor add
        │
        ├── Backup original
        ├── Move to ~/.dotcor/files/shell/zshrc
        ├── Create relative symlink
        ├── Update config.yaml
        └── Git commit
        │
        ▼
~/.zshrc (symlink) → .dotcor/files/shell/zshrc (actual file)
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

## Design Principles

### Safety First

- Always backup before destructive operations
- Use transactions for multi-step operations
- Never leave system in broken state on failure
- Git commit failures should not fail the main operation

### Fail Gracefully

- Provide clear, actionable error messages
- Suggest fixes when possible
- Offer `--force` flags for advanced users, but default to safe behavior

### Minimal Surprise

- Follow conventions from similar tools (Stow, Chezmoi)
- Keep command structure intuitive
- Default behaviors should be conservative

## Git Workflow

### Commit Guidelines

- **Atomic commits:** Each commit addresses a single concern
- **Never combine** unrelated changes in one commit
- **Clear messages:** Descriptive, present tense, imperative mood

```
# Good commit messages
fix: handle missing parent directory in symlink creation
feat: add glob pattern support to add command
refactor: extract path normalization to separate function

# Bad commit messages
updates
fix stuff
WIP
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
- **Commit after completing each task** - don't batch multiple tasks into one commit

### Commit After Every Task

When implementing from a plan:
- Complete one logical unit of work (e.g., one module, one command)
- Verify it compiles with `go build ./...`
- Make a commit with a descriptive message
- Use conventional commit format: `feat:`, `fix:`, `refactor:`, etc.
- Keep commits atomic - one concern per commit

### Code Review Mindset

- Prefer simple solutions over clever ones
- Question additions that increase complexity
- Verify error handling is complete
- Check for edge cases

## Building and Running

```bash
# Build
go build -o dotcor cmd/dotcor/main.go

# Run directly
go run cmd/dotcor/main.go [command]

# Run tests
go test ./...

# Run specific package tests
go test ./internal/config/...
```

## Release Workflow

DotCor uses **GoReleaser** for automated multi-platform releases.

### Automated Release Process

Releases are triggered by pushing a version tag:

```bash
# Create and push tag
git tag -a v0.2.0 -m "Release v0.2.0: Description"
git push origin v0.2.0
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
- `cmd/dotcor/main.go` declares: `var version = "0.1.1"`
- Built binary shows: `dotcor version v0.2.0`

### For Detailed Instructions

See [docs/RELEASING.md](docs/RELEASING.md) for:
- Pre-release checklist
- Monitoring releases
- Rollback procedures
- Troubleshooting

## Key Files Reference

| File | Purpose |
|------|---------|
| `PLAN.md` | Detailed implementation plan with code examples |
| `README.md` | User-facing documentation |
| `docs/TESTING.md` | Testing conventions, patterns, and best practices |
| `docs/LOGGING.md` | Structured logging guide with level guidelines and examples |
| `docs/RELEASING.md` | Release process and GoReleaser workflow |
| `internal/config/config.go` | Config struct and Load/Save operations |
| `internal/core/transaction.go` | Transaction/rollback semantics |
 | `internal/core/hooks.go` | Hook system for pre/post operations |
 | `internal/core/templates.go` | Template variable substitution |
 | `internal/fs/symlink.go` | macOS symlink handling |
 | `internal/git/git.go` | Git command wrapper |
 | `internal/logger/logger.go` | Structured logging configuration |

## Common Patterns

### Lock Acquisition

```go
func runCommand() error {
    if err := lock.AcquireLock(); err != nil {
        return err
    }
    defer lock.ReleaseLock()

    // command logic
}
```

### Transaction Usage

```go
tx := NewTransaction()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
        panic(r)
    }
}()

if err := tx.Execute(&MoveFileOp{src, dst}); err != nil {
    return err
}

tx.Commit()
```

### Path Normalization

```go
// Always normalize paths for storage
normalized, err := paths.NormalizePath(absolutePath)

// Always expand paths before file operations
expanded, err := paths.ExpandPath(normalizedPath)
```

### Hook Usage

```go
// Before an operation (e.g., add)
ctx := core.HookContext{
    HookType: "pre-add",
    FilePath: sourcePath,
}
core.RunHook(ctx, cfg)  // Gracefully skips if hook doesn't exist

// After an operation (e.g., add)
ctx = core.HookContext{
    HookType: "post-add",
    FilePath: sourcePath,
    RepoPath: repoPath,
}
core.RunHook(ctx, cfg)  // Logs errors but doesn't fail operation
```

### Template Usage

```go
// Get template context
ctx, err := core.GetTemplateContext()
if err != nil {
    return err
}

// Substitute variables in content
rendered := core.SubstituteTemplate(originalContent, ctx)

// Check if file is a template
if core.IsTemplateFile(filename) {
    // Strip .template extension and render
    actualFile := core.StripTemplateExtension(filename)
}
```

### Structured Logging

DotCor uses Go 1.21+ `log/slog` for structured logging following these principles:

1. **Inject logger via Config struct** - Logger field in Config, passed to all functions
2. **Separate user output from logs** - fmt.Printf for UI, slog for system logging
3. **Use appropriate log levels** - DEBUG (internal), INFO (events), WARN (issues), ERROR (failures)
4. **Include structured fields** - Key-value pairs for context (op, file, error)
5. **Test log emission** - Verify logs at correct levels with proper fields

Example:
```go
func BackupFile(path string, cfg *config.Config) error {
    cfg.Logger.Debug("starting backup", "file", path)
    // ... implementation
    cfg.Logger.Info("backup created", "file", path, "backup", backupPath)
    return nil
}
```

#### When to Use cfg.Logger vs fmt.Printf

| Use Case | Method | Example |
|----------|--------|---------|
| System logging (operations, decisions) | `cfg.Logger.Debug/Info/Warn/Error` | `cfg.Logger.Debug("validating file", "path", path)` |
| User-facing UI (banners, tables, prompts) | `fmt.Printf` | `fmt.Printf("✓ Added %s\n", file)` |

#### Log Level Guidelines

- **DEBUG**: Function entry/exit, internal decisions, intermediate states
- **INFO**: User-relevant events (file added, backup created, git commits)
- **WARN**: Non-critical issues (hook failures, large files, retries)
- **ERROR**: Stopping failures (lock held, file not found, permission errors)

#### Common Structured Fields

- `op`: Operation type (backup, add, remove, git)
- `file`: File being operated on
- `src`: Source path (shortened from source_path)
- `repo`: Repository path (shortened from repo_path)
- `error`: Error object (for ERROR level logs)
- `duration`: Operation duration (useful for performance tracking)

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
- Core packages (config, core, fs, git, logger) require comprehensive coverage (target 85%+)
- Command tests should cover major CLI commands (init, add, remove, list, status, sync, restore, history, diff, adopt, doctor, rebuild-config, clone, cleanup)

Testing documentation at docs/TESTING.md includes:
- testify framework usage (assert/require packages)
- AAA pattern examples
- Test naming conventions
- Helper functions (cmd/dotcor/test_helpers.go)
- Coverage goals and pre-commit workflow
