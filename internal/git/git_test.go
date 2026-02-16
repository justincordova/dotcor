package git

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

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

func TestAutoCommit(t *testing.T) {
	require.True(t, IsGitInstalled(), "git must be installed for this test")

	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

	configureGitUser(t, tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

	err = InitRepo(tempDir)
	require.NoError(t, err, "InitRepo() should not error")

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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
	defer os.RemoveAll(tempDir)

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

func TestParseGitStatusLine_RenamedFiles(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "renamed file (R)",
			line:     "R  oldname.txt -> newname.txt",
			expected: "newname.txt",
		},
		{
			name:     "renamed file (RR)",
			line:     "RR oldname.txt -> newname.txt",
			expected: "newname.txt",
		},
		{
			name:     "renamed file with spaces",
			line:     "R  old name.txt -> new name.txt",
			expected: "new name.txt",
		},
		{
			name:     "renamed file in subdirectory",
			line:     "R  old/oldname.txt -> new/newname.txt",
			expected: "new/newname.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should return new filename for renamed files")
		})
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
			name:     "renamed file without arrow",
			line:     "R  oldname.txt newname.txt",
			expected: "",
		},
		{
			name:     "trailing whitespace",
			line:     "M  file.txt   ",
			expected: "file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGitStatusLine(tt.line)
			assert.Equal(t, tt.expected, result, "parseGitStatusLine() should handle edge cases correctly")
		})
	}
}
