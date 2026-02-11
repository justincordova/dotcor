package core

import (
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPreflightValidation_AllChecksPass(t *testing.T) {
	t.Run("all checks pass with valid setup", func(t *testing.T) {
		// Arrange
		cfg, err := config.NewDefaultConfig()
		require.NoError(t, err)
		cfg.RepoPath = "~/.dotcor/files"

		// Act
		result := RunPreflightValidation(cfg, "test", []string{})

		// Assert
		assert.True(t, result.Success, "validation should succeed")
		for _, check := range result.Checks {
			if check.Name != "git_clean" {
				assert.True(t, check.Passed, "check should pass: %s", check.Name)
			}
		}
	})
}

func TestDisplayValidationResults_AllPassed(t *testing.T) {
	t.Run("displays passed checks", func(t *testing.T) {
		// Arrange
		result := ValidationResult{
			Success: true,
			Checks: []CheckResult{
				{Name: "test1", Passed: true, Message: "Check 1 passed"},
				{Name: "test2", Passed: true, Message: "Check 2 passed"},
			},
		}

		// Act
		err := DisplayValidationResults(result)

		// Assert
		assert.NoError(t, err, "should return no error when all checks pass")
	})
}

func TestDisplayValidationResults_FailedChecks(t *testing.T) {
	t.Run("returns error when checks fail", func(t *testing.T) {
		// Arrange
		result := ValidationResult{
			Success: false,
			Checks: []CheckResult{
				{Name: "test1", Passed: true, Message: "Check 1 passed"},
				{Name: "test2", Passed: false, Message: "Check 2 failed"},
			},
		}

		// Act
		err := DisplayValidationResults(result)

		// Assert
		assert.Error(t, err, "should return error when checks fail")
	})
}
