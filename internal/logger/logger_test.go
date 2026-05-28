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

	var _ *slog.Logger = l //nolint:staticcheck
}

// TestRotateLogIfNeeded_PreservesAllBackups exercises the bug where the
// rotation loop deleted the wrong slot (maxBackups-1 instead of
// maxBackups), silently losing a backup on every rotation. With three
// existing backups, every numbered slot must be present after one
// rotation and the contents must have shifted by exactly one position.
func TestRotateLogIfNeeded_PreservesAllBackups(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "rotate.log")

	// Seed: current log over the rotation threshold + .1/.2/.3 backups.
	big := make([]byte, maxLogSize+1)
	require.NoError(t, os.WriteFile(logPath, big, 0644))
	require.NoError(t, os.WriteFile(logPath+".1", []byte("one"), 0644))
	require.NoError(t, os.WriteFile(logPath+".2", []byte("two"), 0644))
	require.NoError(t, os.WriteFile(logPath+".3", []byte("three"), 0644))

	rotateLogIfNeeded(logPath)

	// .1 should now hold the rotated current log (size ≈ maxLogSize+1).
	info1, err := os.Stat(logPath + ".1")
	require.NoError(t, err, ".1 must exist after rotation")
	assert.Greater(t, info1.Size(), int64(maxLogSize), ".1 must be the rotated current log")

	// .2 should hold what was in .1 ("one").
	got2, err := os.ReadFile(logPath + ".2")
	require.NoError(t, err, ".2 must exist after rotation")
	assert.Equal(t, "one", string(got2), ".2 must hold the previous .1 contents — regression: was being deleted")

	// .3 should hold what was in .2 ("two").
	got3, err := os.ReadFile(logPath + ".3")
	require.NoError(t, err, ".3 must exist after rotation")
	assert.Equal(t, "two", string(got3), ".3 must hold the previous .2 contents")

	// .4 must never exist — rotation keeps at most maxBackups files.
	_, err = os.Stat(logPath + ".4")
	assert.True(t, os.IsNotExist(err), ".4 must not exist after rotation")
}
