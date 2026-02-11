package main

import (
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestBackupDiff_NoBackup_ReturnsError(t *testing.T) {
	t.Run("no backup returns error", func(t *testing.T) {
		// Arrange
		cfg := CreateTestConfig(t)
		cfg.ManagedFiles = []config.ManagedFile{
			{SourcePath: "~/.test-file", RepoPath: "test/test-file"},
		}

		// Act
		backupDiffCmd.SetArgs([]string{"~/.test-file"})
		err := runBackupDiff(backupDiffCmd, []string{"~/.test-file"})

		// Assert
		assert.Error(t, err, "should return error when no backup exists")
	})
}
