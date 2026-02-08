# DotCor Logging Guide

## Overview

DotCor uses Go 1.21+ `log/slog` for structured system logging. User-facing output (banners, tables, progress) continues to use `fmt.Printf` for interactive UI.

## Architecture

The logging system follows a dependency injection pattern:

1. **Logger Creation**: Created at startup in `main.go` using `logger.ConfigureFromFlags()`
2. **Dependency Injection**: Logger stored in `config.Config.Logger` field
3. **Pass Through**: All functions that need logging receive `*config.Config` parameter
4. **Structured Fields**: All log calls include key-value pairs for context

### Separation of Concerns

- **System Logging** (`cfg.Logger`): Internal operations, decisions, flow control
- **User Output** (`fmt.Printf`): Interactive UI, banners, tables, prompts

## Log Levels

| Level | Usage | When to Use | Example |
|--------|--------|--------------|---------|
| **DEBUG** | Detailed internal operations | Function entry/exit, decisions, intermediate states | `cfg.Logger.Debug("creating symlink", "target", target, "link", link)` |
| **INFO** | Important events | User-relevant events (file added, backup created) | `cfg.Logger.Info("backup created", "file", sourcePath, "path", backupPath)` |
| **WARN** | Non-critical issues | Hook failures, retries, large files | `cfg.Logger.Warn("file is very large", "path", path, "size_mb", sizeMB)` |
| **ERROR** | Stopping failures | Lock held, file not found, permission errors | `cfg.Logger.Error("failed to expand path", "file", path, "error", err)` |

## CLI Flags

### --debug
Enable DEBUG level logging.

```bash
dotcor add ~/.zshrc --debug
```

Shows detailed internal operations like path expansion, validation steps, file operations.

### --quiet
Suppress INFO messages (show only ERROR).

```bash
dotcor add ~/.zshrc --quiet
```

Useful for scripting or when you only want to see errors.

### --log-file
Write logs to specified file instead of stderr.

```bash
dotcor add ~/.zshrc --log-file=/tmp/dotcor.log
```

Useful for debugging batch operations or logging to syslog.

### --json
Output logs in JSON format (machine-parseable).

```bash
dotcor add ~/.zshrc --json
```

Useful for log aggregation systems like ELK, Splunk, Datadog.

## Examples

### Normal Operation
```bash
$ dotcor add ~/.zshrc
✓ Added ~/.zshrc → shell/zshrc
[OK] Committed to Git
```

No logs shown (INFO level and below hidden).

### Debug Mode
```bash
$ dotcor add ~/.zshrc --debug
✓ Added ~/.zshrc → shell/zshrc

# Logs emitted to stderr:
2026/02/08 12:30:00 DEBUG validating source file path=~/.zshrc
2026/02/08 12:30:00 DEBUG validating file size path=~/.zshrc size=1024 threshold=104857600
2026/02/08 12:30:00 DEBUG generating repo path source=~/.zshrc custom=
2026/02/08 12:30:00 DEBUG repo path generated source=~/.zshrc repo=shell/zshrc
2026/02/08 12:30:00 DEBUG creating backup file=~/.zshrc
2026/02/08 12:30:00 INFO backup created file=~/.zshrc path=/Users/user/.dotcor/backups/2026-02-08_12-30-00/home/.zshrc
2026/02/08 12:30:00 DEBUG moving file src=~/.zshrc dst=~/.dotcor/files/shell/zshrc
2026/02/08 12:30:00 INFO file moved src=~/.zshrc dst=~/.dotcor/files/shell/zshrc
2026/02/08 12:30:00 DEBUG creating symlink target=~/.dotcor/files/shell/zshrc link=~/.zshrc
2026/02/08 12:30:00 INFO symlink created target=~/.dotcor/files/shell/zshrc link=~/.zshrc
2026/02/08 12:30:00 INFO added file src=~/.zshrc repo=shell/zshrc
```

Detailed insight into every operation.

### Quiet Mode
```bash
$ dotcor add ~/.zshrc --quiet
✓ Added ~/.zshrc → shell/zshrc
[OK] Committed to Git
```

Only ERROR logs shown.

### Log File
```bash
$ dotcor add ~/.zshrc --log-file=/tmp/dotcor.log
✓ Added ~/.zshrc → shell/zshrc
[OK] Committed to Git

$ cat /tmp/dotcor.log
2026/02/08 12:30:00 INFO validating source file path=~/.zshrc
2026/02/08 12:30:00 INFO added file src=~/.zshrc repo=shell/zshrc
...
```

Logs written to file instead of stderr.

### JSON Format
```bash
$ dotcor add ~/.zshrc --json
✓ Added ~/.zshrc → shell/zshrc

# Logs in JSON:
{"time":"2026-02-08T12:30:00Z","level":"INFO","msg":"validating source file","path":"~/.zshrc"}
{"time":"2026-02-08T12:30:00Z","level":"INFO","msg":"added file","src":"~/.zshrc","repo":"shell/zshrc"}
```

Machine-parseable format for log aggregation.

## Adding Logging to Code

### Functions with Config Parameter

All functions that need logging should receive `*config.Config` parameter:

```go
func SomeOperation(input string, cfg *config.Config) error {
    cfg.Logger.Debug("operation starting", "input", input)

    // ... do work ...

    if err != nil {
        cfg.Logger.Error("operation failed", "input", input, "error", err)
        return err
    }

    cfg.Logger.Info("operation completed", "input", input)
    return nil
}
```

### Pure Functions (No Config)

Functions without `*config.Config` parameter (path utilities, pure functions) should use `cfg` parameter if available, or pass `nil` if not:

```go
func NormalizePath(path string) (string, error) {
    // Pure function, no logging needed
    // ... implementation ...
}
```

```go
func ExpandPath(path string, cfg *config.Config) (string, error) {
    if cfg != nil {
        cfg.Logger.Debug("expanding path", "path", path)
    }
    // ... implementation ...
    return expanded, nil
}
```

## Structured Fields

Common fields to include in log calls:

| Field | Short | Description | Example |
|-------|-------|-------------|---------|
| `op` | - | Operation type | `"backup"`, `"add"`, `"remove"` |
| `file` | - | File being operated on | `"~/.zshrc"` |
| `src` | `source_path` | Source path (shortened) | `"~/.zshrc"` |
| `repo` | `repo_path` | Repository path (shortened) | `"shell/zshrc"` |
| `backup` | `backup_path` | Backup path (shortened) | `"/Users/.../backups/..."` |
| `error` | - | Error object (for ERROR level) | `err` |
| `duration` | - | Operation duration (ms) | `42` |

Note: Field names are automatically shortened by the logger's `ReplaceAttr` function (e.g., `source_path` → `src`).

## Best Practices

### 1. Log at Appropriate Level

- **DEBUG**: Function entry/exit, internal decisions, intermediate states
- **INFO**: User-relevant events (file added, backup created, git commits)
- **WARN**: Non-critical issues (hook failures, large files, retries)
- **ERROR**: Stopping failures (lock held, file not found, permission errors)

### 2. Include Structured Fields

Use key-value pairs for context:

```go
// Good
cfg.Logger.Info("file added", "src", sourcePath, "repo", repoPath)

// Bad
cfg.Logger.Info("file added " + sourcePath + " to " + repoPath)
```

### 3. Be Consistent with Field Names

Use common field names across the codebase:

```go
// Good - consistent
cfg.Logger.Debug("creating backup", "file", sourcePath)
cfg.Logger.Debug("creating symlink", "target", target, "link", link)

// Bad - inconsistent
cfg.Logger.Debug("creating backup", "source", sourcePath)
cfg.Logger.Debug("creating symlink", "to", target, "at", link)
```

### 4. Separate Concerns

- Use `cfg.Logger` for system logging
- Use `fmt.Printf` for user-facing UI
- Don't mix them

```go
// Good
cfg.Logger.Info("backup created", "file", sourcePath, "path", backupPath)
fmt.Printf("✓ Backed up %s\n", sourcePath)

// Bad
cfg.Logger.Info("✓ Backed up %s\n", sourcePath) // Don't put UI in logs
```

### 5. Test Log Emission

Verify logs emit at correct levels with proper fields:

```go
func TestCreateBackup(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    })
    logger := slog.New(handler)

    cfg := &config.Config{
        Logger:   logger,
        RepoPath: t.TempDir(),
    }

    // Test backup creation
    sourcePath := filepath.Join(t.TempDir(), "test.txt")
    os.WriteFile(sourcePath, []byte("test content"), 0644)

    backupPath, err := CreateBackup(sourcePath, cfg)
    if err != nil {
        t.Fatalf("CreateBackup failed: %v", err)
    }

    // Verify logs
    output := buf.String()
    if !strings.Contains(output, "creating backup") {
        t.Error("expected 'creating backup' in logs")
    }
    if !strings.Contains(output, "backup created") {
        t.Error("expected 'backup created' in logs")
    }
}
```

## Logger Configuration

The logger is configured in `cmd/dotcor/main.go`:

```go
func configureLogger(cmd *cobra.Command, cfg *config.Config) {
    // Configure logger from flags
    cfg.Logger = logger.ConfigureFromFlags(cmd)
}
```

Flags are read in `internal/logger/logger.go`:

```go
func ConfigureFromFlags(cmd *cobra.Command) *slog.Logger {
    debug, _ := cmd.Flags().GetBool("debug")
    quiet, _ := cmd.Flags().GetBool("quiet")
    logFile, _ := cmd.Flags().GetString("log-file")
    jsonFormat, _ := cmd.Flags().GetBool("json")

    // Determine log level
    level := levelFromFlags(debug, quiet)

    // Create handler
    var handler slog.Handler
    if jsonFormat {
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: level,
        })
    } else {
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: level,
            ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
                // Shorten common field names for readability
                if a.Key == "source_path" {
                    a.Key = "src"
                }
                if a.Key == "repo_path" {
                    a.Key = "repo"
                }
                if a.Key == "backup_path" {
                    a.Key = "backup"
                }
                return a
            },
        })
    }

    // If log file specified, create file handler
    if logFile != "" {
        file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
        if err != nil {
            // Fallback to stderr if file creation fails
            return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
                Level: level,
            }))
        }
        handler = slog.NewTextHandler(file, &slog.HandlerOptions{
            Level: level,
        })
    }

    return slog.New(handler)
}
```

## Troubleshooting

### Logs Not Appearing

1. Check if `--quiet` flag is set (suppresses INFO)
2. Check if `--debug` flag is missing (suppresses DEBUG)
3. Verify logger is configured before use
4. Check if `--log-file` is writing to a different location

### Too Much Logging

1. Remove `--debug` flag (only INFO and above)
2. Use `--quiet` flag (only ERROR)
3. Redirect stderr to `/dev/null`: `dotcor add ~/.zshrc 2>/dev/null`

### Structured Field Not Showing

1. Ensure field names are strings (not variables)
2. Check for typos in field names
3. Verify field name isn't being shortened by `ReplaceAttr`

## References

- [Go slog package documentation](https://pkg.go.dev/log/slog)
- [DotCor architecture guide](PLAN.md)
- [DotCor development guide](CLAUDE.md)
