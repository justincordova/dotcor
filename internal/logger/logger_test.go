package logger

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"
)

func TestConfigureFromFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.PersistentFlags().Bool("debug", false, "")
	cmd.PersistentFlags().Bool("quiet", false, "")
	cmd.PersistentFlags().String("log-file", "", "")
	cmd.PersistentFlags().Bool("json", false, "")

	logger := ConfigureFromFlags(cmd)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLevelFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		debug    bool
		quiet    bool
		expected slog.Level
	}{
		{"default", false, false, slog.LevelInfo},
		{"debug", true, false, slog.LevelDebug},
		{"quiet", false, true, slog.LevelWarn},
		{"debug overrides quiet", true, true, slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := levelFromFlags(tt.debug, tt.quiet)
			if got != tt.expected {
				t.Errorf("levelFromFlags(%v, %v) = %v, want %v", tt.debug, tt.quiet, got, tt.expected)
			}
		})
	}
}

func TestLogEmission(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	logger.Info("test message", "key", "value")

	output := buf.String()
	if len(output) == 0 {
		t.Fatal("expected log output")
	}
}
