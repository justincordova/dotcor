package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/justincordova/dotcor/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func init() {
	// Load .env file if it exists
	_ = godotenv.Load()
}

var (
	version = "1.0.4"
)

// ANSI color codes
const (
	colorReset     = "\033[0m"
	colorDim       = "\033[2m"
	colorBold      = "\033[1m"
	colorRed       = "\033[31m"
	colorGreen     = "\033[32m"
	colorYellow    = "\033[33m"
	colorBlue      = "\033[34m"
	colorCyan      = "\033[36m"
	colorGray      = "\033[90m"
	colorWhite     = "\033[97m"
	colorOrange    = "\033[38;5;208m"
	colorPink      = "\033[38;5;205m"
	colorLightPink = "\033[38;5;218m"
	colorLime      = "\033[38;5;118m"
	colorCritical  = "\033[38;5;196m"
	colorWarnLabel = "\033[38;5;226m"
	colorInfoLabel = "\033[38;5;39m"
)

// Success prints success message in green
func Success(msg string) {
	fmt.Printf("%s✓%s %s\n", colorGreen, colorReset, msg)
}

// Warning prints warning message in yellow
func Warning(msg string) {
	fmt.Printf("%s⚠%s %s\n", colorYellow, colorReset, msg)
}

// Error prints error message in red
func Error(msg string) {
	fmt.Printf("%s✗%s %s\n", colorRed, colorReset, msg)
}

// Info prints info message in blue
func Info(msg string) {
	fmt.Printf("%sℹ%s %s\n", colorBlue, colorReset, msg)
}

// DryRun prints dry-run message in gray
func DryRun(msg string) {
	fmt.Printf("%s•%s %s\n", colorGray, colorReset, msg)
}

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

	versionStr := "v" + version
	fmt.Printf("  %s%s%s %s%s· symlink-based dotfile manager%s\n", colorBold, colorLightPink, versionStr, colorReset, colorDim, colorReset)
	fmt.Println()
}

var rootCmd = &cobra.Command{
	Use:   "dotcor",
	Short: "A simple, fast dotfile manager with symlinks and Git automation",
	Long: fmt.Sprintf(`DotCor combines %s%sthe simplicity of GNU Stow%s%s with automatic Git commits.

Manage your dotfiles with symlinks - edit files directly, changes instantly
appear in your repository. Built-in Git automation handles commits and sync.`,
		colorRed, colorBold, colorReset, colorRed),
	Version: "v" + version,
	Run:     runRoot,
}

func init() {
	viper.SetDefault("version", version)

	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().Bool("quiet", false, "Suppress INFO messages")
	rootCmd.PersistentFlags().String("log-file", "", "Write logs to file")
	rootCmd.PersistentFlags().Bool("json", false, "Output logs in JSON format")

	// Add commands in order of importance (most common first)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(remoteCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(adoptCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(rebuildCmd)
	rootCmd.AddCommand(rebuildLinksCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(listBackupsCmd)
	rootCmd.AddCommand(backupDiffCmd)

	// Replace help command to add ? and h as aliases with custom ordering
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:     "help [command]",
		Aliases: []string{"?", "h"},
		Short:   "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type dotcor help [path to command] for full details.`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				printCustomHelp(c.Root())
			} else {
				cmd, _, e := c.Root().Find(args)
				if cmd == nil || e != nil {
					c.Printf("Unknown help topic %#q\n", args)
					_ = c.Root().Usage()
				} else {
					cmd.InitDefaultHelpFlag()
					_ = cmd.Help()
				}
			}
		},
	})
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
		fmt.Printf("  %s[!] Not initialized%s\n", colorYellow, colorReset)
		fmt.Println()
		fmt.Printf("  %sGet started:%s\n", colorLightPink, colorReset)
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
	files := cfg.ManagedFiles
	totalFiles := len(files)

	// Get changed files from git
	var changedFiles []string
	var gitStatus git.StatusInfo
	configDir, err := config.GetConfigDir()
	if err == nil && git.IsGitInstalled() && git.IsRepo(configDir) {
		gitStatus, err = git.GetStatus(configDir)
		if err == nil {
			changedFiles = gitStatus.ChangedFiles
		}
	}

	// Count problems
	problemCount := 0
	for _, f := range files {
		fs := CheckFileStatus(cfg, f, changedFiles)
		if fs.Status != "ok" {
			problemCount++
		}
	}

	// Status section
	fmt.Printf("  %s%sStatus%s\n", colorBold, colorLightPink, colorReset)
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
	if len(changedFiles) > 0 || gitStatus.HasUncommitted {
		fmt.Printf("  %s %s uncommitted changes\n", colorYellow, colorReset)
	} else if git.IsGitInstalled() && git.IsRepo(configDir) {
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

	fmt.Println()
	fmt.Printf("  %sCommands:%s  status · add · sync · --help\n", colorLightPink, colorReset)
	fmt.Println()
}

func printCustomHelp(cmd *cobra.Command) {
	fmt.Printf("Usage:\n  %s\n\n", cmd.UseLine())

	fmt.Printf("%sMain Commands:%s\n", colorLightPink, colorReset)
	printCmd(initCmd)
	printCmd(addCmd)
	printCmd(removeCmd)
	printCmd(listCmd)
	printCmd(statusCmd)
	printCmd(syncCmd)

	fmt.Printf("\n%sAdditional Commands:%s\n", colorLightPink, colorReset)
	printCmd(remoteCmd)
	printCmd(restoreCmd)
	printCmd(historyCmd)
	printCmd(diffCmd)
	printCmd(adoptCmd)
	printCmd(doctorCmd)
	printCmd(rebuildCmd)
	printCmd(rebuildLinksCmd)
	printCmd(cloneCmd)
	printCmd(cleanupCmd)
	printCmd(listBackupsCmd)
	printCmd(backupDiffCmd)

	fmt.Printf("\n%sFlags:%s", colorLightPink, colorReset)
	cmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		fmt.Printf("  --%-20s %s\n", flag.Name, flag.Usage)
	})

	fmt.Println()
	fmt.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
}

func printCmd(cmd *cobra.Command) {
	if cmd.IsAvailableCommand() {
		fmt.Printf("  %s%-20s%s %s\n", colorGreen, cmd.Name(), colorReset, cmd.Short)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
