# DotCor Logging Guide

## Overview

DotCor uses `charmbracelet/log` for structured file-based logging. Since the TUI owns the terminal, all log output goes to a file — never stderr.

## Default Log File

```
~/.dotcor/logs/dotcor.log
```

Auto-created on first run. Auto-rotated, 5MB max.

## Log Levels

| Level | Usage |
|-------|-------|
| **debug** | Function entry/exit, intermediate states |
| **info** | User-relevant events (file linked, backup created, git commit) |
| **warn** | Non-critical issues (hook failures, large files) |
| **error** | Stopping failures (lock held, permission errors) |

## CLI Flags

| Flag | Action |
|------|--------|
| `--debug` | Set log level to debug |
| `--log-level <level>` | Set level: debug, info, warn, error |

Example:

```bash
dotcor --debug
dotcor --log-level info
```

## TUI Log Viewer

Press `L` in the TUI to open the built-in log viewer with level filtering. This reads from the same log file.

## Constructor

```go
logger := logger.New(level, logFilePath)
```

- `level`: "debug", "info", "warn", "error"
- `logFilePath`: optional; defaults to `~/.dotcor/logs/dotcor.log`

## Structured Fields

All log calls include key-value pairs:

```go
cfg.Logger.Debug("creating symlink", "target", target, "link", link)
cfg.Logger.Info("package stowed", "package", name, "files", count)
cfg.Logger.Warn("file is large", "path", path, "size_mb", size)
cfg.Logger.Error("failed to create symlink", "error", err)
```

## Log Output

Example `~/.dotcor/logs/dotcor.log`:

```
2026/04/14 10:30:00 DEBU creating symlink target=/home/user/.dotcor/zsh/.zshrc link=/home/user/.zshrc
2026/04/14 10:30:00 INFO package stowed files=1 package=zsh
2026/04/14 10:30:00 WARN file is large path=/home/user/.bigconfig size_mb=12
2026/04/14 10:30:00 ERRO failed to create symlink error=permission denied
```

## No Stderr When TUI Running

The TUI owns the terminal. All logging goes to the file. There is no stderr output while the TUI is active.

## Common Fields

| Field | Description |
|-------|-------------|
| `file` | File being operated on |
| `package` | Package name |
| `src` | Source path |
| `dst` | Destination path |
| `error` | Error object |
| `duration` | Operation duration |

## Usage in Code

```go
func SomeOperation(path string, cfg *config.Config) error {
    cfg.Logger.Debug("operation starting", "file", path)

    if err != nil {
        cfg.Logger.Error("operation failed", "file", path, "error", err)
        return err
    }

    cfg.Logger.Info("operation complete", "file", path)
    return nil
}
```
