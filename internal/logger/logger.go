package logger

import (
	"fmt"
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

func New(level string, logFilePath string) *slog.Logger {
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
		}))
	}

	rotateLogIfNeeded(logFilePath)

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to open log file %s: %v\n", logFilePath, err)
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		}))
	}

	handler := charmlog.NewWithOptions(file, charmlog.Options{
		ReportTimestamp: true,
		Level:           lvl,
	})

	return slog.New(handler)
}

func rotateLogIfNeeded(logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxLogSize {
		return
	}

	for i := maxBackups - 1; i >= 1; i-- {
		older := fmt.Sprintf("%s.%d", logPath, i)
		newer := fmt.Sprintf("%s.%d", logPath, i+1)
		if i == maxBackups-1 {
			_ = os.Remove(older)
		}
		if _, err := os.Stat(older); err == nil {
			_ = os.Rename(older, newer)
		}
	}
	_ = os.Rename(logPath, logPath+".1")
}
