package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all managed dotfiles",
	Aliases: []string{"ls"},
	Long: `List all dotfiles currently managed by DotCor.

By default shows source paths. Use flags to show more details.

Examples:
  dotcor list                  # List all managed files
  dotcor list --long           # Show detailed info including repo paths
  dotcor list --category       # Group by category
  dotcor list --status         # Show symlink status
  dotcor list --json           # Output as JSON`,
	RunE: runList,
}

func init() {
	listCmd.Flags().BoolP("long", "l", false, "Show detailed information")
	listCmd.Flags().Bool("category", false, "Group files by category")
	listCmd.Flags().Bool("status", false, "Show symlink status")
	listCmd.Flags().Bool("json", false, "Output as JSON")
	listCmd.Flags().Bool("paths-only", false, "Output only paths (for scripting)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	longFormat, _ := cmd.Flags().GetBool("long")
	byCategory, _ := cmd.Flags().GetBool("category")
	showStatus, _ := cmd.Flags().GetBool("status")
	jsonFormat, _ := cmd.Flags().GetBool("json")
	pathsOnly, _ := cmd.Flags().GetBool("paths-only")

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
	}
	configureLogger(cmd, cfg)

	files := cfg.GetManagedFilesForPlatform()

	if len(files) == 0 {
		fmt.Println("No files managed by DotCor.")
		fmt.Println("Run 'dotcor add <file>' to start managing dotfiles.")
		return nil
	}

	// Handle JSON output
	if jsonFormat {
		return outputJSON(cfg, files, showStatus)
	}

	// Handle paths-only output
	if pathsOnly {
		for _, f := range files {
			fmt.Println(f.SourcePath)
		}
		return nil
	}

	// Handle category grouping
	if byCategory {
		return outputByCategory(cfg, files, showStatus)
	}

	// Standard or long format
	if longFormat || showStatus {
		return outputLong(cfg, files, showStatus)
	}

	// Simple format
	return outputSimple(files)
}

// outputSimple shows just the file paths
func outputSimple(files []config.ManagedFile) error {
	for _, f := range files {
		fmt.Printf("  %s\n", f.SourcePath)
	}
	fmt.Printf("\n%d file(s) managed\n", len(files))
	return nil
}

// outputLong shows detailed information in a table
func outputLong(cfg *config.Config, files []config.ManagedFile, showStatus bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	if showStatus {
		fmt.Fprintln(w, "SOURCE\tREPO PATH\tSTATUS\tADDED")
	} else {
		fmt.Fprintln(w, "SOURCE\tREPO PATH\tADDED")
	}

	for _, f := range files {
		addedAt := f.AddedAt.Format("2006-01-02")

		if showStatus {
			status := getSymlinkStatus(cfg, f)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.SourcePath, f.RepoPath, status, addedAt)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", f.SourcePath, f.RepoPath, addedAt)
		}
	}

	w.Flush()
	fmt.Printf("\n%d file(s) managed\n", len(files))
	return nil
}

// outputByCategory groups files by their category (directory in repo)
func outputByCategory(cfg *config.Config, files []config.ManagedFile, showStatus bool) error {
	// Group by category (first directory component of repo path)
	categories := make(map[string][]config.ManagedFile)

	for _, f := range files {
		category := getCategory(f.RepoPath)
		categories[category] = append(categories[category], f)
	}

	// Sort category names
	var categoryNames []string
	for name := range categories {
		categoryNames = append(categoryNames, name)
	}
	sort.Strings(categoryNames)

	// Output by category
	for _, category := range categoryNames {
		fmt.Printf("\n[%s]\n", category)
		categoryFiles := categories[category]

		for _, f := range categoryFiles {
			if showStatus {
				status := getSymlinkStatus(cfg, f)
				fmt.Printf("  %s (%s)\n", f.SourcePath, status)
			} else {
				fmt.Printf("  %s\n", f.SourcePath)
			}
		}
	}

	fmt.Printf("\n%d file(s) in %d categories\n", len(files), len(categories))
	return nil
}

// outputJSON outputs the file list as JSON
func outputJSON(cfg *config.Config, files []config.ManagedFile, showStatus bool) error {
	fmt.Println("[")

	for i, f := range files {
		status := ""
		if showStatus {
			status = getSymlinkStatus(cfg, f)
		}

		comma := ","
		if i == len(files)-1 {
			comma = ""
		}

		if showStatus {
			fmt.Printf("  {\"source\": \"%s\", \"repo\": \"%s\", \"status\": \"%s\", \"added\": \"%s\"}%s\n",
				f.SourcePath, f.RepoPath, status, f.AddedAt.Format("2006-01-02"), comma)
		} else {
			fmt.Printf("  {\"source\": \"%s\", \"repo\": \"%s\", \"added\": \"%s\"}%s\n",
				f.SourcePath, f.RepoPath, f.AddedAt.Format("2006-01-02"), comma)
		}
	}

	fmt.Println("]")
	return nil
}

// getCategory extracts the category from a repo path
func getCategory(repoPath string) string {
	parts := strings.SplitN(repoPath, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return "other"
}

// getSymlinkStatus checks the status of a symlink
func getSymlinkStatus(cfg *config.Config, f config.ManagedFile) string {
	sourcePath, err := config.ExpandPath(f.SourcePath, cfg)
	if err != nil {
		return "error"
	}

	// Check if source exists
	if !fs.PathExists(sourcePath) {
		return "missing"
	}

	// Check if it's a symlink
	isLink, err := fs.IsSymlink(sourcePath)
	if err != nil {
		return "error"
	}

	if !isLink {
		return "not-symlink"
	}

	// Check if symlink is valid (target exists)
	valid, err := fs.IsValidSymlink(sourcePath)
	if err != nil {
		return "error"
	}

	if !valid {
		return "broken"
	}

	// Check if symlink points to correct target
	target, err := fs.ReadSymlink(sourcePath)
	if err != nil {
		return "error"
	}

	expectedTarget, err := config.GetRepoFilePath(cfg, f.RepoPath)
	if err != nil {
		return "error"
	}

	// Compute expected relative target
	expectedRel, err := config.ComputeRelativeSymlink(sourcePath, expectedTarget)
	if err != nil {
		return "error"
	}

	// Compare targets (handle both relative and absolute)
	if target == expectedRel || target == expectedTarget {
		return "ok"
	}

	// Check if they resolve to the same file
	sourceDir := getDir(sourcePath)
	resolvedTarget := resolvePath(sourceDir, target)
	if resolvedTarget == expectedTarget {
		return "ok"
	}

	return "wrong-target"
}

// getDir returns the directory part of a path
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// resolvePath resolves a potentially relative path against a base directory
func resolvePath(baseDir, path string) string {
	if len(path) > 0 && (path[0] == '/' || (len(path) > 1 && path[1] == ':')) {
		return path
	}
	// Simple path resolution - join and clean
	result := baseDir + "/" + path
	// Clean up .. references
	parts := strings.Split(result, "/")
	var clean []string
	for _, part := range parts {
		if part == ".." && len(clean) > 0 {
			clean = clean[:len(clean)-1]
		} else if part != "." && part != "" {
			clean = append(clean, part)
		}
	}
	return "/" + strings.Join(clean, "/")
}
