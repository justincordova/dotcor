package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	charmlog "github.com/charmbracelet/log"

	"github.com/justincordova/dotcor/internal/config"
)

const (
	maxLogSize = 10 * 1024 * 1024
	maxBackups = 3
)

// New is the package's historical entry point — kept for callers that
// don't need a close handle. Internally it delegates to NewWithCloser
// and discards the closer, which matches the previous behaviour (the OS
// reclaims the handle on exit).
func New(level string, logFilePath string) *slog.Logger {
	l, _ := NewWithCloser(level, logFilePath)
	return l
}

// NewWithCloser builds the logger and returns the underlying file handle
// (wrapped as an io.Closer) so callers can close it at shutdown. The
// closer is always non-nil: when the log file couldn't be opened the
// returned closer is a no-op so callers can always `defer closer.Close()`
// without a nil check.
func NewWithCloser(level string, logFilePath string) (*slog.Logger, io.Closer) {
	var lvl charmlog.Level
	switch level {
	case "debug":
		lvl = charmlog.DebugLevel
	case "info":
		lvl = charmlog.InfoLevel
	case "warn":
		lvl = charmlog.WarnLevel
	case "error":
		lvl = charmlog.ErrorLevel
	default:
		lvl = charmlog.WarnLevel
	}

	if logFilePath == "" {
		configDir, err := config.GetConfigDir()
		if err != nil {
			logFilePath = filepath.Join(os.TempDir(), "dotcor.log")
		} else {
			logFilePath = filepath.Join(configDir, "logs", "dotcor.log")
		}
	}

	logDir := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to create log directory %s: %v\n", logDir, err)
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})), noopCloser{}
	}

	rotateLogIfNeeded(logFilePath)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open log file %s: %v\n", logFilePath, err)
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})), noopCloser{}
	}

	handler := charmlog.NewWithOptions(file, charmlog.Options{
		ReportTimestamp: true,
		Level:           lvl,
	})

	return slog.New(handler), file
}

// noopCloser is returned when there's no real file to close — keeps
// callers free of nil checks.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// rotateLogIfNeeded rotates logs in the classic "x → x+1" pattern,
// keeping at most maxBackups historical files (.1 newest, .maxBackups
// oldest). The previous implementation deleted the slot we were about
// to write into (maxBackups-1) instead of the slot we were going to
// drop (maxBackups), so a backup was silently lost on every rotation.
func rotateLogIfNeeded(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxLogSize {
		return
	}

	// Drop the oldest backup so the .maxBackups slot is free.
	_ = os.Remove(fmt.Sprintf("%s.%d", logPath, maxBackups))

	// Shift remaining backups one slot older: .(maxBackups-1) → .maxBackups,
	// .(maxBackups-2) → .(maxBackups-1), …, .1 → .2.
	for i := maxBackups - 1; i >= 1; i-- {
		older := fmt.Sprintf("%s.%d", logPath, i)
		newer := fmt.Sprintf("%s.%d", logPath, i+1)
		if _, err := os.Stat(older); err == nil {
			_ = os.Rename(older, newer)
		}
	}
	_ = os.Rename(logPath, logPath+".1")
}
