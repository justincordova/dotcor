package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultLevel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	l := New("", logPath)
	require.NotNil(t, l)

	l.Warn("test message", "key", "value")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data, "log file should have content")
}

func TestNew_DebugLevel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "debug.log")

	l := New("debug", logPath)
	require.NotNil(t, l)

	l.Debug("debug message")
	l.Info("info message")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "debug message")
	assert.Contains(t, content, "info message")
}

func TestNew_WarnLevel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "warn.log")

	l := New("warn", logPath)
	require.NotNil(t, l)

	l.Debug("should not appear")
	l.Warn("should appear")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "should not appear")
	assert.Contains(t, content, "should appear")
}

func TestNew_ErrorLevel(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "error.log")

	l := New("error", logPath)
	require.NotNil(t, l)

	l.Warn("should not appear")
	l.Error("should appear")

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, "should not appear")
	assert.Contains(t, content, "should appear")
}

func TestNew_CreatesLogDirectory(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "nested", "dir", "test.log")

	l := New("info", logPath)
	require.NotNil(t, l)

	l.Info("test")

	_, err := os.Stat(filepath.Dir(logPath))
	assert.NoError(t, err, "should create log directory")
}

func TestNew_DefaultLogPath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	l := New("info", "")
	require.NotNil(t, l)

	l.Info("test message")

	expectedPath := filepath.Join(tempDir, ".dotcor", "logs", "dotcor.log")
	_, err := os.Stat(expectedPath)
	assert.NoError(t, err, "should create default log file at ~/.dotcor/logs/dotcor.log")
}

func TestNew_InvalidPath_Fallback(t *testing.T) {
	l := New("info", "")
	assert.NotNil(t, l)
}

func TestNew_ReturnsSlogLogger(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test.log")

	l := New("info", logPath)

	var _ *slog.Logger = l
}
