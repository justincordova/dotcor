package main

import (
	"fmt"
	"os"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "0.5.5" // Test coverage improvements
)

// ANSI color codes
const (
	colorReset     = "\033[0m"
	colorDim       = "\033[2m"
	colorBold      = "\033[1m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
	colorCyan      = "\033[36m"
	colorWhite     = "\033[97m"
	colorOrange    = "\033[38;5;208m"
	colorPink      = "\033[38;5;205m"
	colorLightPink = "\033[38;5;218m"
	colorLime      = "\033[38;5;118m"
)

func printBanner() {
	fmt.Println()
	fmt.Print(colorLightPink)
	fmt.Println("  ██████╗  ██████╗ ████████╗ ██████╗ ██████╗ ██████╗ ")
	fmt.Println("  ██╔══██╗██╔═══██╗╚══██╔══╝██╔════╝██╔═══██╗██╔══██╗")
	fmt.Println("  ██║  ██║██║   ██║   ██║   ██║     ██║   ██║██████╔╝")
	fmt.Println("  ██║  ██║██║   ██║   ██║   ██║     ██║   ██║██╔══██╗")
	fmt.Println("  ██████╔╝╚██████╔╝   ██║   ╚██████╗╚██████╔╝██║  ██║")
	fmt.Println("  ╚═════╝  ╚═════╝    ╚═╝    ╚═════╝ ╚═════╝ ╚═╝  ╚═╝")
	fmt.Print(colorReset)
	fmt.Println()
	fmt.Printf("  %s%sv%s%s %s· symlink-based dotfile manager%s\n", colorBold, colorLightPink, version, colorReset, colorDim, colorReset)
	fmt.Println()
}

var rootCmd = &cobra.Command{
	Use:   "dotcor",
	Short: "A simple, fast dotfile manager with symlinks and Git automation",
	Long: `DotCor combines the simplicity of GNU Stow with automatic Git commits.

Manage your dotfiles with symlinks - edit files directly, changes instantly
appear in your repository. Built-in Git automation handles commits and sync.`,
	Version: version,
	Run:     runRoot,
}

func init() {
	viper.SetDefault("version", version)

	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress INFO messages")
	rootCmd.PersistentFlags().String("log-file", "", "Write logs to file")
	rootCmd.PersistentFlags().Bool("json", false, "Output logs in JSON format")
}

func configureLogger(cmd *cobra.Command, cfg *config.Config) {
	cfg.Logger = logger.ConfigureFromFlags(cmd)
}

func runRoot(cmd *cobra.Command, args []string) {
	printBanner()

	// Configure logger FIRST, before loading config
	defaultCfg, err := config.NewDefaultConfig()
	if err != nil {
		fmt.Printf("  %s[!] Failed to create default config%s\n", colorYellow, colorReset)
		return
	}
	configureLogger(cmd, defaultCfg)

	// Now load config with logger available
	cfg, err := config.LoadConfig()
	if err != nil {
		cfg = defaultCfg
		fmt.Printf("  %s[!] Not initialized%s\n", colorYellow, colorReset)
		fmt.Println()
		fmt.Printf("  %sGet started:%s\n", colorDim, colorReset)
		fmt.Println("    dotcor init          Initialize DotCor")
		fmt.Println("    dotcor --help        Show all commands")
		fmt.Println()
		return
	}

	// Transfer logger to loaded config
	cfg.Logger = defaultCfg.Logger

	// Show quick status
	showQuickStatus(cfg)
}

func showQuickStatus(cfg *config.Config) {
	files := cfg.GetManagedFilesForPlatform()
	totalFiles := len(files)

	// Count problems
	problemCount := 0
	for _, f := range files {
		fs := CheckFileStatus(cfg, f)
		if fs.Status != "ok" {
			problemCount++
		}
	}

	// Status section
	fmt.Printf("  %sStatus%s\n", colorBold, colorReset)
	fmt.Printf("  %s──────%s\n", colorDim, colorReset)

	// Files status
	if totalFiles == 0 {
		fmt.Printf("  %s %s No files managed\n", colorDim, colorReset)
	} else {
		if problemCount == 0 {
			fmt.Printf("  %s*%s %d file(s) %s[OK]%s\n", colorGreen, colorReset, totalFiles, colorGreen, colorReset)
		} else {
			fmt.Printf("  %s*%s %d file(s), %s%d with issues%s\n", colorYellow, colorReset, totalFiles, colorYellow, problemCount, colorReset)
		}
	}

	// Git status
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err == nil && git.IsGitInstalled() && git.IsRepo(repoPath) {
		gitStatus, err := git.GetStatus(repoPath)
		if err == nil {
			if gitStatus.HasUncommitted {
				fmt.Printf("  %s %s uncommitted changes\n", colorYellow, colorReset)
			} else {
				fmt.Printf("  %s*%s clean %s[OK]%s\n", colorGreen, colorReset, colorGreen, colorReset)
			}

			if gitStatus.RemoteExists {
				if gitStatus.AheadBy > 0 {
					fmt.Printf("  %s↑%s %d to push\n", colorCyan, colorReset, gitStatus.AheadBy)
				}
				if gitStatus.BehindBy > 0 {
					fmt.Printf("  %s↓%s %d to pull\n", colorCyan, colorReset, gitStatus.BehindBy)
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("  %sCommands:%s  status · add · sync · --help\n", colorDim, colorReset)
	fmt.Println()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
