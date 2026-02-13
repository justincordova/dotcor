package core

import (
	"fmt"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
)

// ValidationResult represents the result of pre-flight checks
type ValidationResult struct {
	Success bool
	Checks  []CheckResult
}

// CheckResult represents a single validation check
type CheckResult struct {
	Name    string
	Passed  bool
	Message string
}

// RunPreflightValidation runs all pre-flight checks for an operation
func RunPreflightValidation(cfg *config.Config, operation string, files []string) ValidationResult {
	var checks []CheckResult

	// Check 1: Symlinks exist and are valid
	for _, mf := range cfg.ManagedFiles {
		sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
		if err != nil {
			checks = append(checks, CheckResult{
				Name:    "symlink_valid",
				Passed:  false,
				Message: fmt.Sprintf("%s: cannot expand path: %v", mf.SourcePath, err),
			})
			continue
		}

		isLink, err := fs.IsSymlink(sourcePath)
		if err != nil {
			checks = append(checks, CheckResult{
				Name:    "symlink_valid",
				Passed:  false,
				Message: fmt.Sprintf("%s: cannot check symlink: %v", mf.SourcePath, err),
			})
			continue
		}

		if !isLink {
			checks = append(checks, CheckResult{
				Name:    "symlink_valid",
				Passed:  false,
				Message: fmt.Sprintf("%s: not a symlink", mf.SourcePath),
			})
			continue
		}

		valid, err := fs.IsValidSymlink(sourcePath)
		if err != nil || !valid {
			checks = append(checks, CheckResult{
				Name:    "symlink_valid",
				Passed:  false,
				Message: fmt.Sprintf("%s: broken symlink", mf.SourcePath),
			})
			continue
		}

		checks = append(checks, CheckResult{
			Name:    "symlink_valid",
			Passed:  true,
			Message: fmt.Sprintf("%s: symlink exists and is valid", mf.SourcePath),
		})
	}

	// Check 2: Repo files exist
	for _, mf := range cfg.ManagedFiles {
		repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
		if err != nil {
			checks = append(checks, CheckResult{
				Name:    "repo_file_exists",
				Passed:  false,
				Message: fmt.Sprintf("%s: cannot get repo path: %v", mf.RepoPath, err),
			})
			continue
		}

		if !fs.PathExists(repoPath) {
			checks = append(checks, CheckResult{
				Name:    "repo_file_exists",
				Passed:  false,
				Message: fmt.Sprintf("%s: repo file missing: %s", mf.SourcePath, repoPath),
			})
		} else {
			checks = append(checks, CheckResult{
				Name:    "repo_file_exists",
				Passed:  true,
				Message: fmt.Sprintf("%s: repo file exists", mf.SourcePath),
			})
		}
	}

	// Check 3: Backup directory available
	backupDir, err := GetBackupDir()
	if err != nil {
		checks = append(checks, CheckResult{
			Name:    "backup_dir_available",
			Passed:  false,
			Message: fmt.Sprintf("cannot get backup directory: %v", err),
		})
	} else {
		if fs.PathExists(backupDir) {
			checks = append(checks, CheckResult{
				Name:    "backup_dir_available",
				Passed:  true,
				Message: "backup directory exists",
			})
		} else {
			checks = append(checks, CheckResult{
				Name:    "backup_dir_available",
				Passed:  false,
				Message: "backup directory missing",
			})
		}
	}

	// Check 4: Git status (if git enabled)
	if cfg.GitEnabled {
		repoPath, err := config.ExpandPath(cfg.RepoPath, cfg)
		if err == nil {
			hasChanges, _ := git.HasChanges(repoPath)
			if hasChanges {
				checks = append(checks, CheckResult{
					Name:    "git_clean",
					Passed:  true,
					Message: "uncommitted changes in repository",
				})
			} else {
				checks = append(checks, CheckResult{
					Name:    "git_clean",
					Passed:  true,
					Message: "git repository clean",
				})
			}
		}
	}

	// Determine overall success
	success := true
	for _, check := range checks {
		if !check.Passed {
			success = false
		}
	}

	return ValidationResult{
		Success: success,
		Checks:  checks,
	}
}

// DisplayValidationResults shows validation results to user
func DisplayValidationResults(result ValidationResult) error {
	// ANSI color codes
	colorLightPink := "\033[38;5;218m"
	colorReset := "\033[0m"
	colorGreen := "\033[32m"
	colorRed := "\033[31m"

	fmt.Printf("\n  %sPre-flight checks:%s\n", colorLightPink, colorReset)
	fmt.Println("")

	// Group checks by type
	checkGroups := make(map[string][]CheckResult)
	for _, check := range result.Checks {
		checkGroups[check.Name] = append(checkGroups[check.Name], check)
	}

	allPassed := true

	// Display summary for each check type
	for checkType, checks := range checkGroups {
		passed := 0
		failed := 0
		var failures []string

		for _, check := range checks {
			if check.Passed {
				passed++
			} else {
				failed++
				failures = append(failures, check.Message)
				allPassed = false
			}
		}

		// Show summary line
		total := passed + failed
		if total == 1 {
			// Single check - show the message directly
			if failed > 0 {
				fmt.Printf("  %s✗%s %s\n", colorRed, colorReset, failures[0])
			} else {
				fmt.Printf("  %s✓%s %s\n", colorGreen, colorReset, checks[0].Message)
			}
		} else {
			// Multiple checks - show count summary
			if failed == 0 {
				fmt.Printf("  %s✓%s %d %s OK\n", colorGreen, colorReset, passed, checkType)
			} else if passed == 0 {
				fmt.Printf("  %s✗%s %d %s failed\n", colorRed, colorReset, failed, checkType)
				for _, msg := range failures {
					fmt.Printf("    %s✗%s %s\n", colorRed, colorReset, msg)
				}
			} else {
				fmt.Printf("  %s✗%s %d/%d %s OK (%d failed)\n", colorRed, colorReset, passed, total, checkType, failed)
				for _, msg := range failures {
					fmt.Printf("    %s✗%s %s\n", colorRed, colorReset, msg)
				}
			}
		}
	}

	fmt.Println("")

	if !allPassed {
		return fmt.Errorf("pre-flight checks failed")
	}

	return nil
}
