package logger

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func ConfigureFromFlags(cmd *cobra.Command) *slog.Logger {
	debug, _ := cmd.Flags().GetBool("debug")
	quiet, _ := cmd.Flags().GetBool("quiet")
	logFile, _ := cmd.Flags().GetString("log-file")
	jsonFormat, _ := cmd.Flags().GetBool("json")

	level := levelFromFlags(debug, quiet)

	var handler slog.Handler
	if jsonFormat {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
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

	if logFile != "" {
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
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

func levelFromFlags(debug, quiet bool) slog.Level {
	switch {
	case debug:
		return slog.LevelDebug
	case quiet:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
