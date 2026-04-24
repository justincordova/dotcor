package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/logger"
	"github.com/justincordova/dotcor/internal/stow"
	"github.com/justincordova/dotcor/tui"
)

var version = "dev"

func main() {
	debug := false
	logLevel := "warn"

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--version":
			fmt.Printf("dotcor %s\n", version)
			return
		case "--debug":
			debug = true
			logLevel = "debug"
		}
	}

	for i, arg := range os.Args[1:] {
		if arg == "--log-level" && i+1 < len(os.Args[1:]) {
			logLevel = os.Args[i+2]
		}
	}

	if debug {
		logLevel = "debug"
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		cfg, err = config.NewDefaultConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
	// NewWithCloser returns the underlying log file so we can flush and
	// close it at exit. Without this the handle leaks until the OS
	// reclaims it — harmless for short-lived runs but it also blocks any
	// future in-process log rotation.
	logr, logCloser := logger.NewWithCloser(logLevel, "")
	cfg.Logger = logr
	defer func() { _ = logCloser.Close() }()

	configDir, err := config.GetConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		fmt.Printf("Not initialized. Create %s? (y/N): ", configDir)
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			return
		}
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
			os.Exit(1)
		}
		if err := git.InitRepo(configDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git init failed: %v\n", err)
		}
		if err := cfg.SaveConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: config save failed: %v\n", err)
		}
		fmt.Printf("Created %s\n", configDir)
	}

	if stow.DetectV1Layout(configDir) {
		fmt.Printf("Found v1.x layout in %s/files/. Migrate to v2.0? (y/N): ", configDir)
		var response string
		_, _ = fmt.Scanln(&response)
		if strings.ToLower(response) == "y" {
			steps, err := stow.PlanMigration(configDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error planning migration: %v\n", err)
				os.Exit(1)
			}
			if err := stow.ExecuteMigration(configDir, steps); err != nil {
				fmt.Fprintf(os.Stderr, "error executing migration: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Migration complete")
		}
	}

	// Acquire a session-wide lock so two dotcor instances against the same
	// ~/.dotcor can't race on backup-path timestamps and on .dotcor-tmp
	// renames. Stale locks (process dead or > 5min old) are auto-cleaned
	// inside AcquireLock; a live conflict surfaces the holding PID/host.
	if err := core.AcquireLock(cfg); err != nil {
		// Surface the conflict explicitly so the user can act on it
		// (kill the other process, or run `dotcor doctor --fix` if the
		// lock is stuck). Not retried — interactive sessions shouldn't
		// silently block.
		if errors.Is(err, core.ErrLockHeld) {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "warning: lock acquisition failed: %v\n", err)
	}
	defer func() {
		if err := core.ReleaseLock(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: lock release failed: %v\n", err)
		}
	}()

	model := tui.NewModel(cfg, version)
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
