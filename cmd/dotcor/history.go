package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history [file]",
	Short: "Show Git history for a dotfile",
	Long: `Show the Git commit history for a managed dotfile.

Without a file argument, shows the history for all dotfiles.
With a file argument, shows history for that specific file.

Examples:
  dotcor history                   # Show all commit history
  dotcor history ~/.zshrc          # Show history for specific file
  dotcor history -n 20             # Show last 20 commits
  dotcor history --oneline         # Compact format`,
	RunE: runHistory,
}

func init() {
	historyCmd.Flags().IntP("number", "n", 10, "Number of commits to show")
	historyCmd.Flags().Bool("oneline", false, "Show compact one-line format")
	historyCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("number")
	oneline, _ := cmd.Flags().GetBool("oneline")
	jsonFormat, _ := cmd.Flags().GetBool("json")

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

	// Get repo path
	repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
	if err != nil {
		return fmt.Errorf("expanding repo path: %w", err)
	}

	// Check if it's a git repo
	if !git.IsRepo(repoPath) {
		return fmt.Errorf("dotcor repository is not a git repository")
	}

	// Determine which file to show history for
	var filePath string
	var displayPath string

	if len(args) > 0 {
		// Specific file
		sourcePath := args[0]
		mf, err := cfg.GetManagedFile(sourcePath)
		if err != nil {
			return fmt.Errorf("file not managed: %s", sourcePath)
		}
		filePath = mf.RepoPath
		displayPath = mf.SourcePath
	}

	// Get history
	commits, err := git.GetFileHistory(repoPath, filePath, limit)
	if err != nil {
		return fmt.Errorf("getting history: %w", err)
	}

	if len(commits) == 0 {
		if filePath != "" {
			fmt.Printf("No commits found for %s\n", displayPath)
		} else {
			fmt.Println("No commits found.")
		}
		return nil
	}

	// Output
	if jsonFormat {
		return outputHistoryJSON(commits)
	}

	if oneline {
		return outputHistoryOneline(commits)
	}

	return outputHistoryFull(commits, displayPath)
}

// outputHistoryFull shows detailed commit history
func outputHistoryFull(commits []git.CommitInfo, filePath string) error {
	if filePath != "" {
		fmt.Printf("%sHistory for %s:%s\n", colorLightPink, filePath, colorReset)
		fmt.Println("")
	}

	for i, c := range commits {
		fmt.Printf("commit %s\n", c.Hash)
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %s\n", c.Date.Format("Mon Jan 2 15:04:05 2006 -0700"))
		fmt.Println("")
		fmt.Printf("    %s\n", c.Message)

		if i < len(commits)-1 {
			fmt.Println("")
		}
	}

	return nil
}

// outputHistoryOneline shows compact one-line format
func outputHistoryOneline(commits []git.CommitInfo) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	for _, c := range commits {
		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n",
			shortHash,
			c.Date.Format("2006-01-02"),
			truncateMessage(c.Message, 60),
		)
	}

	w.Flush()
	return nil
}

// commitJSONOutput represents a commit in JSON format
type commitJSONOutput struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Message string `json:"message"`
}

// outputHistoryJSON outputs history as JSON
func outputHistoryJSON(commits []git.CommitInfo) error {
	output := make([]commitJSONOutput, 0, len(commits))

	for _, c := range commits {
		output = append(output, commitJSONOutput{
			Hash:    c.Hash,
			Author:  c.Author,
			Date:    c.Date.Format("2006-01-02T15:04:05Z07:00"),
			Message: c.Message,
		})
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

// truncateMessage truncates a message to a maximum length
func truncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen-3] + "..."
}
