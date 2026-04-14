package core

import (
	"fmt"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
)

type ValidationResult struct {
	Success bool
	Checks  []CheckResult
}

type CheckResult struct {
	Name    string
	Passed  bool
	Message string
}

func RunPreflightValidation(cfg *config.Config, operation string, files []string) ValidationResult {
	var checks []CheckResult

	checks = append(checks, CheckResult{
		Name:    "files_provided",
		Passed:  len(files) > 0,
		Message: fmt.Sprintf("%d files to process", len(files)),
	})

	for _, file := range files {
		sourcePath, err := config.ExpandPath(file, cfg)
		if err != nil {
			checks = append(checks, CheckResult{
				Name:    "file_exists",
				Passed:  false,
				Message: fmt.Sprintf("%s: cannot expand path: %v", file, err),
			})
			continue
		}

		if !fs.PathExists(sourcePath) {
			checks = append(checks, CheckResult{
				Name:    "file_exists",
				Passed:  false,
				Message: fmt.Sprintf("%s: does not exist", file),
			})
		} else {
			checks = append(checks, CheckResult{
				Name:    "file_exists",
				Passed:  true,
				Message: fmt.Sprintf("%s: exists", file),
			})
		}
	}

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

	if cfg.GitEnabled {
		configDir, err := config.GetConfigDir()
		if err == nil {
			hasChanges, _ := git.HasChanges(configDir)
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

func DisplayValidationResults(result ValidationResult) error {
	colorLightPink := "\033[38;5;218m"
	colorReset := "\033[0m"
	colorGreen := "\033[32m"
	colorRed := "\033[31m"

	fmt.Printf("\n  %sPre-flight checks:%s\n", colorLightPink, colorReset)
	fmt.Println("")

	checkGroups := make(map[string][]CheckResult)
	for _, check := range result.Checks {
		checkGroups[check.Name] = append(checkGroups[check.Name], check)
	}

	allPassed := true

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

		total := passed + failed
		if total == 1 {
			if failed > 0 {
				fmt.Printf("  %s✗%s %s\n", colorRed, colorReset, failures[0])
			} else {
				fmt.Printf("  %s✓%s %s\n", colorGreen, colorReset, checks[0].Message)
			}
		} else {
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
