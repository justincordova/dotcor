package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [file]",
	Short: "Show uncommitted changes in managed dotfiles",
	Long: `Show Git diff of uncommitted changes in your dotfiles repository.

Without arguments, shows all uncommitted changes. With a file argument,
shows changes only for that specific file.

Examples:
  dotcor diff                  # Show all uncommitted changes
  dotcor diff ~/.zshrc         # Show changes for specific file
  dotcor diff --stat           # Show summary of changes
  dotcor diff --name-only      # List changed files only`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().Bool("stat", false, "Show diffstat (summary of changes)")
	diffCmd.Flags().Bool("name-only", false, "Show only names of changed files")
	diffCmd.Flags().Bool("staged", false, "Show staged changes only")
}

func runDiff(cmd *cobra.Command, args []string) error {
	statFlag, _ := cmd.Flags().GetBool("stat")
	nameOnly, _ := cmd.Flags().GetBool("name-only")
	staged, _ := cmd.Flags().GetBool("staged")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	// Check if git is available
	if !git.IsGitInstalled() {
		return fmt.Errorf("git is not installed")
	}

	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	// Check if it's a git repo
	if !git.IsRepo(configDir) {
		return fmt.Errorf("dotcor repository is not a git repository")
	}

	// Handle specific file argument
	var gitPath string
	if len(args) > 0 {
		// Convert source path to repo path
		sourcePath := args[0]
		mf, err := cfg.GetManagedFile(sourcePath)
		if err != nil {
			return fmt.Errorf("file not managed: %s", sourcePath)
		}
		gitPath = config.GetGitFilePath(mf.RepoPath)
	}

	// Get appropriate diff
	var output string

	if nameOnly {
		output, err = getChangedFileNames(configDir, staged)
	} else if statFlag {
		output, err = getDiffStat(configDir, gitPath, staged)
	} else {
		output, err = getDiff(configDir, gitPath, staged)
	}

	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	if output == "" {
		if gitPath != "" {
			fmt.Println("No changes for specified file.")
		} else {
			fmt.Println("No uncommitted changes.")
		}
		return nil
	}

	fmt.Print(colorize(output))
	return nil
}

// getDiff returns the full diff output
func getDiff(repoPath, filePath string, staged bool) (string, error) {
	if filePath != "" {
		// Get diff for specific file (staged vs unstaged)
		if staged {
			cmd := exec.Command("git", "diff", "--cached", "--", filePath)
			cmd.Dir = repoPath
			output, err := cmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("git diff --cached failed: %s: %w", string(output), err)
			}
			return string(output), nil
		}
		return git.GetFileDiff(repoPath, filePath)
	}

	// Get full diff (staged vs unstaged)
	if staged {
		cmd := exec.Command("git", "diff", "--cached")
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("git diff --cached failed: %s: %w", string(output), err)
		}
		return string(output), nil
	}
	return git.GetDiff(repoPath)
}

// getDiffStat returns the diffstat output
func getDiffStat(repoPath, filePath string, staged bool) (string, error) {
	if filePath != "" {
		// Git doesn't have a per-file stat, so we get full stat and filter
		stat, err := git.GetDiffStat(repoPath)
		if err != nil {
			return "", err
		}
		// Filter lines containing our file
		var filtered []string
		for _, line := range strings.Split(stat, "\n") {
			if strings.Contains(line, filePath) || strings.HasPrefix(line, " ") {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n"), nil
	}
	return git.GetDiffStat(repoPath)
}

// getChangedFileNames returns just the names of changed files
func getChangedFileNames(repoPath string, staged bool) (string, error) {
	files, err := git.GetChangedFiles(repoPath)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", nil
	}

	// Build output
	var output strings.Builder
	for _, file := range files {
		output.WriteString(file)
		output.WriteString("\n")
	}

	return output.String(), nil
}

// colorize adds ANSI colors to diff output if terminal supports it
func colorize(diff string) string {
	// Check if stdout is a terminal
	if !isTerminal() {
		return diff
	}

	var colored strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			colored.WriteString("\033[32m") // Green
			colored.WriteString(line)
			colored.WriteString("\033[0m")
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			colored.WriteString("\033[31m") // Red
			colored.WriteString(line)
			colored.WriteString("\033[0m")
		} else if strings.HasPrefix(line, "@@") {
			colored.WriteString("\033[36m") // Cyan
			colored.WriteString(line)
			colored.WriteString("\033[0m")
		} else {
			colored.WriteString(line)
		}
		colored.WriteString("\n")
	}

	return colored.String()
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
