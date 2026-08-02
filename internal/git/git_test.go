package git

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsGitInstalled(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed, skipping tests")
	}
}

func TestInitRepo(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	gitDir := filepath.Join(tempDir, ".git")
	_, err = os.Stat(gitDir)
	assert.NoError(t, err, "InitRepo() should create .git directory")
}

func TestIsRepo(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	isRepo := IsRepo(tempDir)
	assert.False(t, isRepo, "IsRepo() should return false for non-repo directory")

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	isRepo = IsRepo(tempDir)
	assert.True(t, isRepo, "IsRepo() should return true after InitRepo()")
}

func TestHasChanges(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	// InitRepo writes a starter .gitignore (covering *.dotcor-tmp,
	// logs/, backups/, etc.) so a fresh repo has one untracked file.
	// Stage and commit it so the rest of the test sees a clean repo.
	stageInitialCommit(t, tempDir)

	hasChanges, err := HasChanges(tempDir)
	assert.NoError(t, err, "HasChanges() should not error")
	assert.False(t, hasChanges, "HasChanges() should return false for clean repo")

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	hasChanges, err = HasChanges(tempDir)
	assert.NoError(t, err, "HasChanges() should not error")
	assert.True(t, hasChanges, "HasChanges() should return true after adding file")
}

// stageInitialCommit adds and commits any files written by InitRepo
// (currently the starter .gitignore) so tests that assert "clean repo
// after init" continue to hold.
func stageInitialCommit(t *testing.T, repoPath string) {
	t.Helper()
	configCmds := [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"add", "."},
		{"commit", "-m", "init"},
	}
	for _, args := range configCmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Logf("git %v: %s: %v", args, string(out), err)
		}
	}
}

func TestAutoCommit(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	err = AutoCommit(tempDir, "test commit", slog.Default())
	assert.NoError(t, err, "AutoCommit() with no changes should not error")

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "add test file", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	hasChanges, err := HasChanges(tempDir)
	assert.NoError(t, err, "HasChanges() should not error")
	assert.False(t, hasChanges, "AutoCommit() should have committed all changes")
}

func TestGetStatus(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)
	// Commit the starter .gitignore InitRepo writes so the rest of the
	// test can assert "clean repo" semantics.
	stageInitialCommit(t, tempDir)

	status, err := GetStatus(tempDir)
	require.NoError(t, err, "GetStatus() should not error")

	assert.False(t, status.RemoteExists, "GetStatus().RemoteExists should be false without remote")
	assert.False(t, status.HasUncommitted, "GetStatus().HasUncommitted should be false for clean repo")

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	status, err = GetStatus(tempDir)
	require.NoError(t, err, "GetStatus() should not error")

	assert.True(t, status.HasUncommitted, "GetStatus().HasUncommitted should be true with changes")
}

func TestGetRemoteURL(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	url, err := GetRemoteURL(tempDir)
	assert.NoError(t, err, "GetRemoteURL() should not error")
	assert.Empty(t, url, "GetRemoteURL() should return empty string for repo without remote")
}

func TestSetRemote(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	testURL := "https://github.com/test/repo.git"
	err = SetRemote(tempDir, "origin", testURL)
	require.NoError(t, err, "SetRemote() should not error")

	url, err := GetRemoteURL(tempDir)
	require.NoError(t, err, "GetRemoteURL() should not error")
	assert.Equal(t, testURL, url, "GetRemoteURL() should return set remote URL")

	newURL := "https://github.com/test/new-repo.git"
	err = SetRemote(tempDir, "origin", newURL)
	require.NoError(t, err, "SetRemote() update should not error")

	url, err = GetRemoteURL(tempDir)
	require.NoError(t, err, "GetRemoteURL() should not error")
	assert.Equal(t, newURL, url, "GetRemoteURL() should return updated remote URL")
}

func TestSetRemote_CreatesNew(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	testURL := "https://github.com/test/repo.git"

	// Act
	err = SetRemote(tempDir, "origin", testURL)

	// Assert
	require.NoError(t, err, "SetRemote() should not error when creating new remote")

	url, err := GetRemoteURL(tempDir)
	require.NoError(t, err, "GetRemoteURL() should not error")
	assert.Equal(t, testURL, url, "SetRemote() should create new remote with correct URL")
}

func TestSetRemote_UpdatesExisting(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	originalURL := "https://github.com/test/original.git"
	err = SetRemote(tempDir, "origin", originalURL)
	require.NoError(t, err, "SetRemote() should not error")

	updatedURL := "https://github.com/test/updated.git"

	// Act
	err = SetRemote(tempDir, "origin", updatedURL)

	// Assert
	require.NoError(t, err, "SetRemote() should not error when updating existing remote")

	url, err := GetRemoteURL(tempDir)
	require.NoError(t, err, "GetRemoteURL() should not error")
	assert.Equal(t, updatedURL, url, "SetRemote() should update remote URL")
	assert.NotEqual(t, originalURL, url, "SetRemote() should have changed the URL")
}

func TestGetFileHistory(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("v1"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "initial commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	err = os.WriteFile(testFile, []byte("v2"), 0644)
	require.NoError(t, err, "failed to update test file")

	err = AutoCommit(tempDir, "second commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	history, err := GetFileHistory(tempDir, "test.txt", 10)
	require.NoError(t, err, "GetFileHistory() should not error")

	assert.Len(t, history, 2, "GetFileHistory() should return 2 commits")
	assert.Equal(t, "second commit", history[0].Message, "Most recent commit should be first")
}

func TestGetCurrentCommit(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "test commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	commit, err := GetCurrentCommit(tempDir)
	require.NoError(t, err, "GetCurrentCommit() should not error")

	assert.Len(t, commit, 40, "GetCurrentCommit() should return 40-character hash")
}

func TestGetChangedFiles(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")
	// Commit the starter .gitignore so the empty-changed-files assertion holds.
	stageInitialCommit(t, tempDir)

	files, err := GetChangedFiles(tempDir)
	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.Empty(t, files, "GetChangedFiles() should return empty slice for clean repo")

	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		testFile := filepath.Join(tempDir, name)
		err = os.WriteFile(testFile, []byte("content"), 0644)
		require.NoError(t, err, "failed to create test file")
	}

	files, err = GetChangedFiles(tempDir)
	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.Len(t, files, 3, "GetChangedFiles() should return 3 changed files")
}

func TestGetDiff(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("original"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "initial commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	diff, err := GetDiff(tempDir)
	assert.NoError(t, err, "GetDiff() should not error")
	assert.Empty(t, diff, "GetDiff() should return empty string for clean repo")

	err = os.WriteFile(testFile, []byte("modified"), 0644)
	require.NoError(t, err, "failed to modify test file")

	diff, err = GetDiff(tempDir)
	assert.NoError(t, err, "GetDiff() should not error")
	assert.NotEmpty(t, diff, "GetDiff() should return diff for modified file")
}

func TestStageAndUnstageFile(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	initialFile := filepath.Join(tempDir, "initial.txt")
	err = os.WriteFile(initialFile, []byte("initial"), 0644)
	require.NoError(t, err, "failed to create initial file")

	err = AutoCommit(tempDir, "initial commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = StageFile(tempDir, "test.txt")
	assert.NoError(t, err, "StageFile() should not error")

	err = UnstageFile(tempDir, "test.txt")
	assert.NoError(t, err, "UnstageFile() should not error")
}

func TestStatusInfo(t *testing.T) {
	info := StatusInfo{
		HasUncommitted: true,
		AheadBy:        2,
		BehindBy:       1,
		Branch:         "main",
		RemoteExists:   true,
	}

	assert.True(t, info.HasUncommitted, "StatusInfo.HasUncommitted should be true")
	assert.Equal(t, 2, info.AheadBy, "StatusInfo.AheadBy should be 2")
	assert.Equal(t, 1, info.BehindBy, "StatusInfo.BehindBy should be 1")
	assert.Equal(t, "main", info.Branch, "StatusInfo.Branch should be 'main'")
	assert.True(t, info.RemoteExists, "StatusInfo.RemoteExists should be true")
}

func TestCommitInfo(t *testing.T) {
	now := time.Now()
	info := CommitInfo{
		Hash:    "abc123",
		Author:  "Test User",
		Date:    now,
		Message: "test commit",
	}

	assert.Equal(t, "abc123", info.Hash, "CommitInfo.Hash should be 'abc123'")
	assert.Equal(t, "Test User", info.Author, "CommitInfo.Author should be 'Test User'")
	assert.Equal(t, "test commit", info.Message, "CommitInfo.Message should be 'test commit'")
}

func configureGitUser(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	err := cmd.Run()
	require.NoError(t, err, "failed to configure git user.email")

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	err = cmd.Run()
	require.NoError(t, err, "failed to configure git user.name")
}

func TestGetConfig_ReturnsValue(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	// Act
	value, err := GetConfig(tempDir, "user.name")

	// Assert
	require.NoError(t, err, "GetConfig() should not error")
	assert.Equal(t, "Test User", value, "GetConfig() should return configured value")
}

func TestSetConfig_SetsValue(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	// Act
	err = SetConfig(tempDir, "user.name", "Custom User")

	// Assert
	require.NoError(t, err, "SetConfig() should not error")

	value, err := GetConfig(tempDir, "user.name")
	require.NoError(t, err, "GetConfig() should not error")
	assert.Equal(t, "Custom User", value, "SetConfig() should set the value")
}

func TestStageFile_StageCorrectFile(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	// Act
	err = StageFile(tempDir, "test.txt")

	// Assert
	require.NoError(t, err, "StageFile() should not error")

	changed, err := HasChanges(tempDir)
	require.NoError(t, err, "HasChanges() should not error")
	assert.True(t, changed, "StageFile() should stage file (changes still pending commit)")
}

func TestUnstageFile_UnstagesFile(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	initialFile := filepath.Join(tempDir, "initial.txt")
	err = os.WriteFile(initialFile, []byte("initial"), 0644)
	require.NoError(t, err, "failed to create initial file")

	err = AutoCommit(tempDir, "initial commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	modifiedFile := filepath.Join(tempDir, "modified.txt")
	err = os.WriteFile(modifiedFile, []byte("modified"), 0644)
	require.NoError(t, err, "failed to create modified file")

	err = StageFile(tempDir, "modified.txt")
	require.NoError(t, err, "StageFile() should not error")

	// Act
	err = UnstageFile(tempDir, "modified.txt")

	// Assert
	require.NoError(t, err, "UnstageFile() should not error")

	changed, err := HasChanges(tempDir)
	require.NoError(t, err, "HasChanges() should not error")
	assert.True(t, changed, "UnstageFile() should unstage the file (changes should be uncommitted)")
}

func TestPull_FetchesAndMerges(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "initial commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	// Act
	err = Pull(tempDir)

	// Assert
	assert.Error(t, err, "Pull() should error when no remote is configured")
	assert.Contains(t, err.Error(), "no tracking information", "Pull() error should indicate no tracking information")
}

func TestGetCurrentCommit_ReturnsHash(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err, "failed to create test file")

	err = AutoCommit(tempDir, "test commit", slog.Default())
	require.NoError(t, err, "AutoCommit() should not error")

	// Act
	commit, err := GetCurrentCommit(tempDir)

	// Assert
	require.NoError(t, err, "GetCurrentCommit() should not error")
	assert.NotEmpty(t, commit, "GetCurrentCommit() should return non-empty hash")
	assert.Len(t, commit, 40, "GetCurrentCommit() should return 40-character hash")
}

func TestParseGitStatusLine_UntrackedFiles(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "basic untracked file",
			line:     "?? newfile.txt",
			expected: "newfile.txt",
		},
		{
			name:     "untracked file in subdirectory",
			line:     "?? subdir/newfile.txt",
			expected: "subdir/newfile.txt",
		},
		{
			name:     "untracked file with leading dot",
			line:     "?? .hiddenfile",
			expected: ".hiddenfile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should return correct filename")
		})
	}
}

// TestParseGitStatusLine_RenamedFiles covers the -z record layout: a
// rename/copy record holds only the DESTINATION path, and the source path
// follows as a separate record. There is no " -> " separator in -z output.
func TestParseGitStatusLine_RenamedFiles(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "renamed file (R)",
			line:     "R  newname.txt",
			expected: "newname.txt",
		},
		{
			name:     "renamed file (RR)",
			line:     "RR newname.txt",
			expected: "newname.txt",
		},
		{
			name:     "renamed file with spaces",
			line:     "R  new name.txt",
			expected: "new name.txt",
		},
		{
			name:     "renamed file in subdirectory",
			line:     "R  new/newname.txt",
			expected: "new/newname.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should return the destination path")
		})
	}
}

// TestIsRenameRecord pins the fix for RM/RD/RA/RU and the copy codes.
// Matching on a "R " / "RR " prefix missed every rename that was also
// modified in the worktree, and those records were then parsed as ordinary
// entries yielding a bogus filename.
func TestIsRenameRecord(t *testing.T) {
	renames := []string{"R  a", "RR a", "RM a", "RD a", "RA a", "RU a", "C  a", "CM a"}
	for _, line := range renames {
		assert.True(t, isRenameRecord(line), "%q should be treated as a rename/copy record", line)
	}

	others := []string{"M  a", "?? a", "A  a", "D  a", "MM a", "", "M"}
	for _, line := range others {
		assert.False(t, isRenameRecord(line), "%q should not be treated as a rename/copy record", line)
	}
}

func TestParseGitStatusLine_StandardCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "modified file (M)",
			line:     "M  modified.txt",
			expected: "modified.txt",
		},
		{
			name:     "staged modification (MM)",
			line:     "MM modified.txt",
			expected: "modified.txt",
		},
		{
			name:     "deleted file (D)",
			line:     "D  deleted.txt",
			expected: "deleted.txt",
		},
		{
			name:     "added file (A)",
			line:     "A  added.txt",
			expected: "added.txt",
		},
		{
			name:     "file with leading dot",
			line:     "M  .hiddenfile",
			expected: ".hiddenfile",
		},
		{
			name:     "file with spaces in name",
			line:     "M  file with spaces.txt",
			expected: "file with spaces.txt",
		},
		{
			name:     "file in subdirectory",
			line:     "M  subdir/file.txt",
			expected: "subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should return correct filename")
		})
	}
}

func TestParseGitStatusLine_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "empty string",
			line:     "",
			expected: "",
		},
		{
			name:     "single character",
			line:     "M",
			expected: "",
		},
		{
			name:     "two characters only",
			line:     "M ",
			expected: "",
		},
		{
			name:     "three characters only",
			line:     "M  ",
			expected: "",
		},
		{
			// Trailing spaces are part of the filename. -z emits paths raw,
			// so trimming them would produce a name git cannot resolve.
			name:     "trailing whitespace is preserved",
			line:     "M  file.txt   ",
			expected: "file.txt   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should handle edge cases correctly")
		})
	}
}

func TestGetDiffBetweenFiles_DifferentFiles(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")

	err = os.WriteFile(file1, []byte("content 1"), 0644)
	require.NoError(t, err, "failed to create file1")

	err = os.WriteFile(file2, []byte("content 2"), 0644)
	require.NoError(t, err, "failed to create file2")

	diff, err := GetDiffBetweenFiles(file1, file2)

	require.NoError(t, err, "GetDiffBetweenFiles() should not error for different files")
	assert.NotEmpty(t, diff, "GetDiffBetweenFiles() should return diff for different files")
	assert.Contains(t, diff, "+++ b/", "diff should contain +++ b/ marker")
	assert.Contains(t, diff, "--- a/", "diff should contain --- a/ marker")
}

func TestGetDiffBetweenFiles_BinaryFiles(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	file1 := filepath.Join(tempDir, "a.bin")
	file2 := filepath.Join(tempDir, "b.bin")

	// NUL bytes make git treat these as binary, so the diff has no
	// +++ b/ / --- a/ hunk markers — only a "Binary files ... differ"
	// line. The old string-matching impl reported this as an error.
	require.NoError(t, os.WriteFile(file1, []byte{0x00, 0x01, 0x02}, 0644), "failed to create a.bin")
	require.NoError(t, os.WriteFile(file2, []byte{0x00, 0x01, 0x03}, 0644), "failed to create b.bin")

	diff, err := GetDiffBetweenFiles(file1, file2)

	require.NoError(t, err, "GetDiffBetweenFiles() should not error for differing binary files")
	assert.NotEmpty(t, diff, "GetDiffBetweenFiles() should return a diff for differing binary files")
	assert.Contains(t, diff, "differ", "binary diff should mention the files differ")
}

func TestGetDiffBetweenFiles_IdenticalFiles(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")

	content := []byte("same content")
	err = os.WriteFile(file1, content, 0644)
	require.NoError(t, err, "failed to create file1")

	err = os.WriteFile(file2, content, 0644)
	require.NoError(t, err, "failed to create file2")

	diff, err := GetDiffBetweenFiles(file1, file2)

	require.NoError(t, err, "GetDiffBetweenFiles() should not error for identical files")
	assert.Empty(t, diff, "GetDiffBetweenFiles() should return empty diff for identical files")
}

func TestGetDiffBetweenFiles_MissingFile(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "nonexistent.txt")

	err = os.WriteFile(file1, []byte("content"), 0644)
	require.NoError(t, err, "failed to create file1")

	diff, err := GetDiffBetweenFiles(file1, file2)

	assert.Error(t, err, "GetDiffBetweenFiles() should error when file doesn't exist")
	assert.Empty(t, diff, "GetDiffBetweenFiles() should return empty string on error")
}

func TestGetChangedFilesWithRenames(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	configureGitUser(t, repo)

	testFile := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("write test.txt failed: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add test.txt failed: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	cmd = exec.Command("git", "mv", "test.txt", "renamed.txt")
	cmd.Dir = repo
	if err := cmd.Run(); err != nil {
		t.Fatalf("git mv failed: %v", err)
	}

	files, err := GetChangedFiles(repo)

	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.Contains(t, files, "renamed.txt", "GetChangedFiles() should include renamed file")
}

func TestGetChangedFilesWithSpaces(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	err := InitRepo(repo)
	require.NoError(t, err, "InitRepo() should not error")

	file1 := filepath.Join(repo, "file with spaces.txt")
	err = os.WriteFile(file1, []byte("content"), 0644)
	require.NoError(t, err, "failed to create file with spaces")

	file2 := filepath.Join(repo, "another spaced file.txt")
	err = os.WriteFile(file2, []byte("content"), 0644)
	require.NoError(t, err, "failed to create another file with spaces")

	files, err := GetChangedFiles(repo)

	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.Contains(t, files, "file with spaces.txt", "GetChangedFiles() should include file with spaces")
	assert.Contains(t, files, "another spaced file.txt", "GetChangedFiles() should include another file with spaces")
}

func TestGetChangedFilesWithUntracked(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	err := InitRepo(repo)
	require.NoError(t, err, "InitRepo() should not error")

	untracked1 := filepath.Join(repo, "untracked1.txt")
	err = os.WriteFile(untracked1, []byte("content"), 0644)
	require.NoError(t, err, "failed to create untracked file")

	untracked2 := filepath.Join(repo, "untracked2.txt")
	err = os.WriteFile(untracked2, []byte("content"), 0644)
	require.NoError(t, err, "failed to create another untracked file")

	files, err := GetChangedFiles(repo)

	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.Contains(t, files, "untracked1.txt", "GetChangedFiles() should include untracked file")
	assert.Contains(t, files, "untracked2.txt", "GetChangedFiles() should include another untracked file")
}

func TestGetChangedFilesWithMerged(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = repo
	err := cmd.Run()
	require.NoError(t, err, "git init failed")

	configureGitUser(t, repo)

	initialFile := filepath.Join(repo, "initial.txt")
	err = os.WriteFile(initialFile, []byte("initial"), 0644)
	require.NoError(t, err, "failed to create initial file")

	cmd = exec.Command("git", "add", "initial.txt")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git add initial.txt failed")

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git commit failed")

	cmd = exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git checkout -b failed")

	featureFile := filepath.Join(repo, "feature.txt")
	err = os.WriteFile(featureFile, []byte("feature content"), 0644)
	require.NoError(t, err, "failed to create feature file")

	cmd = exec.Command("git", "add", "feature.txt")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git add feature.txt failed")

	cmd = exec.Command("git", "commit", "-m", "add feature file")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git commit failed")

	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git checkout main failed")

	mainFile := filepath.Join(repo, "main.txt")
	err = os.WriteFile(mainFile, []byte("main content"), 0644)
	require.NoError(t, err, "failed to create main file")

	cmd = exec.Command("git", "add", "main.txt")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git add main.txt failed")

	cmd = exec.Command("git", "commit", "-m", "add main file")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git commit failed")

	cmd = exec.Command("git", "merge", "feature", "--no-commit")
	cmd.Dir = repo
	err = cmd.Run()
	require.NoError(t, err, "git merge failed")

	files, err := GetChangedFiles(repo)

	assert.NoError(t, err, "GetChangedFiles() should not error")
	assert.NotEmpty(t, files, "GetChangedFiles() should return files from merge")
	assert.Contains(t, files, "feature.txt", "GetChangedFiles() should include merged feature file")
}

func TestAutoCommitWithNoChanges(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	err := InitRepo(repo)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, repo)

	err = AutoCommit(repo, "test commit", slog.Default())

	assert.NoError(t, err, "AutoCommit() should not error when there are no changes")

	hasChanges, err := HasChanges(repo)
	assert.NoError(t, err, "HasChanges() should not error")
	assert.False(t, hasChanges, "AutoCommit() should not create changes when there are none")
}

// TestAutoCommitFilesWithNoChanges runs against a freshly-initialised
// repo where InitRepo has already written the starter .gitignore.
// AutoCommitFiles is called with nil files (so it does `git add -A`)
// and an empty changes baseline shouldn't be empty - the .gitignore
// IS an untracked change. We commit it via stageInitialCommit first so
// the subsequent AutoCommitFiles call genuinely sees a clean repo and
// hits the "nothing to commit" branch.
//
// The previous version of this test was non-deterministic: it didn't
// commit the starter .gitignore, so the AutoCommitFiles call DID find
// changes (the .gitignore) and tried to commit them. Without git user
// config, the commit failed with a "Please tell me who you are" error
// instead of the expected "nothing to commit". The test happened to
// pass when the leftover git user from another test's process state
// was still configured, but failed in isolation.
func TestAutoCommitFilesWithNoChanges(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	repo := t.TempDir()

	err := InitRepo(repo)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, repo)
	// Commit InitRepo's starter .gitignore so the next call really
	// sees a clean tree.
	stageInitialCommit(t, repo)

	err = AutoCommitFiles(repo, nil, "test commit")

	assert.NoError(t, err, "AutoCommitFiles() should not error when there are no changes")

	hasChanges, err := HasChanges(repo)
	assert.NoError(t, err, "HasChanges() should not error")
	assert.False(t, hasChanges, "AutoCommitFiles() should not create changes when there are none")
}

func TestGetDiffBetweenFilesWithDifferentFiles(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	err := os.WriteFile(file1, []byte("content 1"), 0644)
	require.NoError(t, err, "failed to create file1")

	file2 := filepath.Join(tempDir, "file2.txt")
	err = os.WriteFile(file2, []byte("content 2"), 0644)
	require.NoError(t, err, "failed to create file2")

	diff, err := GetDiffBetweenFiles(file1, file2)

	require.NoError(t, err, "GetDiffBetweenFiles() should not error for different files")
	assert.NotEmpty(t, diff, "GetDiffBetweenFiles() should return diff for different files")
	assert.Contains(t, diff, "+++ b/", "diff should contain +++ b/ marker")
	assert.Contains(t, diff, "--- a/", "diff should contain --- a/ marker")
}

func TestRefExistsValidation(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tmpDir := t.TempDir()
	err := InitRepo(tmpDir)
	require.NoError(t, err, "InitRepo() should not error")

	// Test that malicious refs are rejected
	maliciousRefs := []string{
		"../../../etc/passwd",
		"..\\..\\..\\windows\\system32",
		"/absolute/path",
		"",
	}

	for _, ref := range maliciousRefs {
		exists, err := RefExists(tmpDir, ref)
		if err == nil {
			t.Errorf("Should reject malicious ref %s", ref)
		}
		if exists {
			t.Errorf("Should not find malicious ref %s", ref)
		}
	}
}

func TestGitCommandTimeout(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tmpDir := t.TempDir()
	err := InitRepo(tmpDir)
	require.NoError(t, err, "InitRepo() should not error")

	// Test that git commands don't hang indefinitely
	start := time.Now()

	// Try to get status (should complete quickly)
	status, err := GetStatus(tmpDir)

	elapsed := time.Since(start)
	if err != nil && elapsed > 10*time.Second {
		t.Error("git command took too long, should have timeout")
	}

	_ = status
}

// TestRunGitCommand_AppliesTimeout verifies that runGitCommand actually
// wires a context-bound timeout into the command. The previous version
// of this test asserted cmd.Cancel != nil, which always passes for
// CommandContext regardless of whether a timeout was set - it tested
// nothing useful.
//
// The helper accepts an arbitrary timeout via runGitCommandWithTimeout;
// we use a 1ms timeout, sleep 50ms to give the context time to fire,
// then assert Run() returns the expected DeadlineExceeded-shaped error.
// This proves the context-with-deadline path is actually engaged.
func TestRunGitCommand_AppliesTimeout(t *testing.T) {
	require.True(t, IsGitInstalled())

	// Use a deliberately tiny timeout. `git status` on a non-repo
	// would normally fail fast for other reasons, so use `version`
	// which is always available; we still expect the timeout to fire
	// before completion if we sleep long enough.
	cmd, cancel := runGitCommandWithTimeout("/tmp", time.Nanosecond, "version")
	defer cancel()
	require.NotNil(t, cmd)

	// Run should fail because the context has already expired.
	err := cmd.Run()
	assert.Error(t, err, "Run() must fail when context deadline has already passed")
}

// TestRunGitNetworkCommand_UsesLongerTimeout asserts that the network
// helper actually uses a longer timeout than the local helper. The
// previous version just asserted both had non-nil Cancel funcs - which
// any context-bound command always does. Now we compare the package-
// level timeout constants directly, since the helpers are thin wrappers
// over them.
func TestRunGitNetworkCommand_UsesLongerTimeout(t *testing.T) {
	assert.Greater(t, gitNetworkTimeout, gitCommandTimeout,
		"gitNetworkTimeout (%s) must exceed gitCommandTimeout (%s); push/pull/clone need headroom for slow connections",
		gitNetworkTimeout, gitCommandTimeout)
}

func TestValidateRemoteURL_AcceptsSafeTransports(t *testing.T) {
	cases := []string{
		"",
		"https://github.com/user/repo.git",
		"http://gitlab.local/repo",
		"ssh://git@github.com/user/repo.git",
		"git://example.com/repo.git",
		"git@github.com:user/repo.git",
		"deploy@gitlab.com:org/sub/repo.git",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			assert.NoError(t, ValidateRemoteURL(url))
		})
	}
}

func TestValidateRemoteURL_RejectsDangerousTransports(t *testing.T) {
	// These would let an attacker who can poison .dotcorrc execute arbitrary
	// commands or read local files on the next sync.
	cases := []string{
		"ext::sh -c 'rm -rf /'",
		"file:///etc/passwd",
		"file:secret",
		"-upload-pack=evil https://github.com/user/repo",
		"--exec=rm",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			assert.Error(t, ValidateRemoteURL(url))
		})
	}
}

func TestValidateRemoteURL_RejectsMalformedURLs(t *testing.T) {
	cases := []string{
		"not-a-url",
		"@host:path",     // empty user before @
		"user@:path",     // empty host
		"user@-bad:path", // host starts with -
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			assert.Error(t, ValidateRemoteURL(url))
		})
	}
}

// TestGitEnv_SuppressesInteractivePrompts pins the fix for a TUI hang:
// a git subprocess that prompts for credentials writes onto the alt-screen
// and blocks for the whole timeout. Every git invocation must carry the
// prompt-suppression and locale-pinning variables, not just the two
// progress-reporting call sites that used to set them by hand.
func TestGitEnv_SuppressesInteractivePrompts(t *testing.T) {
	env := gitEnv()

	seen := map[string]string{}
	for _, kv := range env {
		if i := strings.Index(kv, "="); i > 0 {
			seen[kv[:i]] = kv[i+1:]
		}
	}

	assert.Equal(t, "0", seen["GIT_TERMINAL_PROMPT"])
	assert.Equal(t, "echo", seen["GIT_ASKPASS"])
	assert.Equal(t, "never", seen["SSH_ASKPASS_REQUIRE"])
	assert.Contains(t, seen["GIT_SSH_COMMAND"], "BatchMode=yes")
	// Locale must be pinned: isNothingToCommitError and friends branch on
	// git's translated human-readable output.
	assert.Equal(t, "C", seen["LC_ALL"])
	assert.Equal(t, "", seen["LANGUAGE"])
}

// TestGitEnv_PreservesUserSSHCommand ensures we never clobber a deliberate
// ssh configuration; the prompt guard is not worth breaking a user's setup.
func TestGitEnv_PreservesUserSSHCommand(t *testing.T) {
	t.Setenv("GIT_SSH_COMMAND", "ssh -i /custom/key")

	var got string
	for _, kv := range gitEnv() {
		if strings.HasPrefix(kv, "GIT_SSH_COMMAND=") {
			got = strings.TrimPrefix(kv, "GIT_SSH_COMMAND=")
		}
	}
	assert.Equal(t, "ssh -i /custom/key", got)
}

// TestRunGitNetworkCommand_SetsWaitDelay pins the guard against a grandchild
// process (ssh, credential helper) holding the output pipe open past the
// context deadline and blocking Wait forever.
func TestRunGitNetworkCommand_SetsWaitDelay(t *testing.T) {
	cmd, cancel := runGitNetworkCommand(t.TempDir(), "push")
	defer cancel()

	assert.Positive(t, cmd.WaitDelay, "network commands must bound child cleanup")
}

// TestRunGitCommand_NoWaitDelayOnLocalCommands pins the scope of that guard.
//
// os/exec returns ErrWaitDelay INSTEAD OF nil when a successful command's
// pipes had to be force-closed. Applying WaitDelay to local commands meant a
// commit that actually landed could be reported as "git commit failed", so it
// is restricted to the network commands where a wedged grandchild is the real
// hazard and whose call sites tolerate the delay explicitly.
func TestRunGitCommand_NoWaitDelayOnLocalCommands(t *testing.T) {
	cmd, cancel := runGitCommand(t.TempDir(), "status")
	defer cancel()

	assert.Zero(t, cmd.WaitDelay,
		"a local command must not risk reporting success as ErrWaitDelay")
}

// TestIsBenignWaitDelay distinguishes the bookkeeping error from real ones.
func TestIsBenignWaitDelay(t *testing.T) {
	assert.True(t, isBenignWaitDelay(exec.ErrWaitDelay))
	assert.True(t, isBenignWaitDelay(fmt.Errorf("wrapped: %w", exec.ErrWaitDelay)))
	assert.False(t, isBenignWaitDelay(nil))
	assert.False(t, isBenignWaitDelay(errors.New("exit status 1")))
}

// TestRedactURLCredentials strips secrets from git output before it reaches
// the TUI footer or the log file. ValidateRemoteURL permits
// https://user:token@host, so git's own messages can echo a live token.
func TestRedactURLCredentials(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "https token",
			in:   "fatal: could not read from 'https://jc:ghp_secret123@github.com/o/r.git'",
			want: "fatal: could not read from 'https://***:***@github.com/o/r.git'",
		},
		{
			name: "no credentials is untouched",
			in:   "fatal: could not read from 'https://github.com/o/r.git'",
			want: "fatal: could not read from 'https://github.com/o/r.git'",
		},
		{
			name: "ssh scheme",
			in:   "error: ssh://git:hunter2@example.com/repo",
			want: "error: ssh://***:***@example.com/repo",
		},
		{
			name: "plain message untouched",
			in:   "Updates were rejected because the remote contains work",
			want: "Updates were rejected because the remote contains work",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RedactURLCredentials(tc.in))
		})
	}
}

// TestPushWithProgress_ReportsGitStderr pins the fix for an unhelpful error:
// stderr used to be discarded, so every failure surfaced as a bare
// "exit status N" with no indication of what went wrong.
func TestPushWithProgress_ReportsGitStderr(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	stageInitialCommit(t, repo)
	require.NoError(t, SetRemote(repo, "origin", "https://127.0.0.1:1/nope.git"))

	err := PushWithProgress(repo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "git push failed")
	assert.NotEqual(t, "exit status 1", err.Error(), "bare exit status tells the user nothing")
}

// TestGetChangedFiles_NonASCIIPath pins the fix for corrupted filenames.
// `git status --porcelain` C-quotes any path with a non-ASCII byte
// ("caf\303\251.txt"); stripping the outer quotes without unescaping the
// body produced a literal backslash-octal string that no later git command
// could match. The -z form emits paths raw.
func TestGetChangedFiles_NonASCIIPath(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	stageInitialCommit(t, repo)

	name := "café.txt"
	require.NoError(t, os.WriteFile(filepath.Join(repo, name), []byte("x"), 0644))

	files, err := GetChangedFiles(repo)

	require.NoError(t, err)
	assert.Contains(t, files, name, "non-ASCII filename must survive parsing intact")
}

// TestGetChangedFiles_QuoteInPath covers the other trigger for C-quoting.
func TestGetChangedFiles_QuoteInPath(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	stageInitialCommit(t, repo)

	name := `quo"te.txt`
	require.NoError(t, os.WriteFile(filepath.Join(repo, name), []byte("x"), 0644))

	files, err := GetChangedFiles(repo)

	require.NoError(t, err)
	assert.Contains(t, files, name)
}

// TestGetChangedFiles_RenamedAndModified pins the RM case. git emits RM when
// a path is renamed in the index and then modified in the worktree. That code
// matched neither the old "R " nor "RR " prefix, so the whole record was
// parsed as a plain entry and yielded "old.txt -> new.txt" as one filename.
func TestGetChangedFiles_RenamedAndModified(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "old.txt"), []byte("x"), 0644))
	stageInitialCommit(t, repo)

	runGit(t, repo, "mv", "old.txt", "new.txt")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("changed"), 0644))

	files, err := GetChangedFiles(repo)

	require.NoError(t, err)
	assert.Contains(t, files, "new.txt", "rename destination must be reported")
	for _, f := range files {
		assert.NotContains(t, f, " -> ", "a filename must never contain the rename separator")
		assert.NotEqual(t, "old.txt", f, "the rename source must not be reported as a changed file")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}

// TestRefExists_MissingBranch pins the fix for a wrong error. `git cat-file
// -e` exits 128 (not 1) for a name that doesn't resolve at all, so a plain
// missing branch used to be reported as a hard failure instead of "absent".
func TestRefExists_MissingBranch(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	stageInitialCommit(t, repo)

	exists, err := RefExists(repo, "no-such-branch")

	require.NoError(t, err, "a missing branch is not an error")
	assert.False(t, exists)
}

func TestRefExists_PresentRef(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))
	stageInitialCommit(t, repo)

	exists, err := RefExists(repo, "HEAD")

	require.NoError(t, err)
	assert.True(t, exists)
}

// TestGetRemoteURL_NoRemote confirms the "no remote" case is still reported
// as (empty, nil) now that other failures propagate.
func TestGetRemoteURL_NoRemote(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))

	url, err := GetRemoteURL(repo)

	require.NoError(t, err)
	assert.Empty(t, url)
}

// TestGetRemoteURL_NotARepoErrors pins the fix for a silent no-op: any
// failure other than "no such remote" used to be swallowed as "no remote",
// so Sync skipped the push and still reported success.
func TestGetRemoteURL_NotARepoErrors(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}

	_, err := GetRemoteURL(t.TempDir())

	assert.Error(t, err, "a non-repository must not look like 'no remote configured'")
}

// TestClone_RejectsDangerousTransport pins the RCE guard. git's ext::
// transport runs an arbitrary command; Clone previously passed the URL
// straight through while CloneWithProgress validated it.
func TestClone_RejectsDangerousTransport(t *testing.T) {
	err := Clone("ext::sh -c 'touch /tmp/dotcor-pwned'", t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid clone URL")
}

func TestClone_RejectsFlagLikeURL(t *testing.T) {
	err := Clone("--upload-pack=touch /tmp/dotcor-pwned", t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid clone URL")
}

// TestRemoveRemote_MissingRemoteIsSuccess pins the exit-code check that
// replaced a match on git's translated "No such remote" message.
func TestRemoveRemote_MissingRemoteIsSuccess(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))

	assert.NoError(t, RemoveRemote(repo, "origin"), "removing an absent remote reaches the desired end state")
}

// TestStripURLPassword pins the fix for a credential leak.
//
// .dotcorrc lives inside the repository and is picked up by `git add -A`, so
// a remote entered as https://user:ghp_token@github.com/... was committed and
// pushed — publishing the token in the repository's history.
func TestStripURLPassword(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"https token", "https://jc:ghp_secret@github.com/jc/dots.git", "https://jc@github.com/jc/dots.git"},
		{"no password", "https://github.com/jc/dots.git", "https://github.com/jc/dots.git"},
		{"ssh scp form untouched", "git@github.com:jc/dots.git", "git@github.com:jc/dots.git"},
		{"ssh scheme user only", "ssh://git@github.com/jc/dots.git", "ssh://git@github.com/jc/dots.git"},
		{"ssh scheme with password", "ssh://git:pw@github.com/jc/dots.git", "ssh://git@github.com/jc/dots.git"},
		{"empty", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StripURLPassword(tc.in)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "ghp_secret")
			assert.NotContains(t, got, ":pw@")
		})
	}
}

// TestEnsureIgnorePatterns_AppliesToExistingRepo pins the fix for a
// repository created before these patterns existed. backups/ holds verbatim
// copies of managed files — including ~/.ssh and ~/.gnupg material — so a
// repo without the entry sweeps them into every commit and push.
func TestEnsureIgnorePatterns_AppliesToExistingRepo(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", repo)
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("# mine\n*.tmp\n"), 0644))

	require.NoError(t, EnsureIgnorePatterns(repo))

	data, err := os.ReadFile(filepath.Join(repo, ".gitignore"))
	require.NoError(t, err)
	content := string(data)

	assert.Contains(t, content, "backups/")
	assert.Contains(t, content, "logs/")
	assert.Contains(t, content, "*.tmp", "the user's own entries must be preserved")
	assert.Contains(t, content, "# mine")
}

// TestInitRepo_IgnoresBackupsDirectory is the end-to-end guard: after init,
// staging everything must not include the backup tree.
func TestInitRepo_IgnoresBackupsDirectory(t *testing.T) {
	if !IsGitInstalled() {
		t.Skip("git not installed")
	}
	repo := t.TempDir()
	require.NoError(t, InitRepo(repo))

	require.NoError(t, os.MkdirAll(filepath.Join(repo, "backups", "ts", ".ssh"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "backups", "ts", ".ssh", "id_rsa"), []byte("KEY"), 0600))

	runGit(t, repo, "config", "user.email", "t@t.t")
	runGit(t, repo, "config", "user.name", "t")
	runGit(t, repo, "add", "-A")

	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = repo
	out, err := cmd.Output()
	require.NoError(t, err)

	assert.NotContains(t, string(out), "id_rsa", "backed-up private material must never be staged")
	assert.NotContains(t, string(out), "backups/")
}
