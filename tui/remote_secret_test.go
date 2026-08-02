package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justincordova/dotcor/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyGitRemote_NeverPersistsPassword pins the fix for a credential
// leak. .dotcorrc lives inside the repository and is staged by `git add -A`,
// so a remote entered as https://user:token@host was committed and pushed.
func TestApplyGitRemote_NeverPersistsPassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOTCOR_DIR", dir)

	require.NoError(t, git.InitRepo(dir))

	m := NewModel(testCfg(), "test")
	m.repoDir = dir

	m.loading = false

	msg := m.applyGitRemote("https://jc:ghp_SECRET_TOKEN@github.com/jc/dots.git")()
	result, ok := msg.(settingsMsg)
	require.True(t, ok, "expected a settingsMsg, got %T", msg)
	require.NoError(t, result.err)

	// The config write happens in the Update handler, against current
	// in-memory state, so drive the message through it.
	updated, _ := m.Update(result)
	require.NoError(t, updated.(Model).err)

	data, err := os.ReadFile(filepath.Join(dir, ".dotcorrc"))
	require.NoError(t, err)

	assert.NotContains(t, string(data), "ghp_SECRET_TOKEN",
		".dotcorrc is committed by `git add -A`; it must never contain a token")
	assert.Contains(t, string(data), "https://jc@github.com/jc/dots.git",
		"the host and username are still useful and are not secret")

	require.NotNil(t, result.gitRemote)
	assert.NotContains(t, *result.gitRemote, "ghp_SECRET_TOKEN")
	assert.Equal(t, *result.gitRemote, updated.(Model).cfg.GitRemote)
}

// TestApplyGitRemote_DoesNotRevertConcurrentPatternEdit pins the fix for a
// silently lost setting. The command used to save a whole-file snapshot
// captured before the git subprocesses ran, so an ignore-pattern edit made
// while SetRemote was forking was reverted on disk with no error shown.
func TestApplyGitRemote_DoesNotRevertConcurrentPatternEdit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOTCOR_DIR", dir)
	require.NoError(t, git.InitRepo(dir))

	m := NewModel(testCfg(), "test")
	m.loading = false
	m.repoDir = dir

	// The command is built and run while the pattern list is still empty.
	msg := m.applyGitRemote("https://github.com/jc/dots.git")()
	result := msg.(settingsMsg)
	require.NoError(t, result.err)

	// Meanwhile the user adds a pattern, which reaches the model.
	m.cfg.IgnorePatterns = append(m.cfg.IgnorePatterns, "*.env")

	updated, _ := m.Update(result)
	require.NoError(t, updated.(Model).err)

	data, err := os.ReadFile(filepath.Join(dir, ".dotcorrc"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "*.env",
		"a pattern added while the remote was being applied must not be reverted")
	assert.Contains(t, string(data), "https://github.com/jc/dots.git")
}

// TestSettingsRemote_NotAppliedOptimistically pins the fix for a display that
// diverged from what was actually stored: the model adopted the new URL
// before the write, so a rejected value showed as configured while
// .git/config and .dotcorrc still held the old one.
func TestSettingsRemote_NotAppliedOptimistically(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.repoDir = t.TempDir()
	m.cfg.GitRemote = "https://github.com/old/repo.git"
	m.activeView = SettingsView
	m.settingsStep = settingsStepEditRemote
	m.settingsInput.SetValue("https://github.com/new/repo.git")

	updated, cmd := updateSettingsEditRemote(m, tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)

	assert.Equal(t, "https://github.com/old/repo.git", updated.(Model).cfg.GitRemote,
		"the model must not adopt the new URL until the write succeeds")
}

// TestSettingsMsg_AdoptsRemoteOnSuccess covers the other half.
func TestSettingsMsg_AdoptsRemoteOnSuccess(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.loading = false
	m.cfg.GitRemote = "old"

	saved := "https://github.com/new/repo.git"
	updated, _ := m.Update(settingsMsg{msg: "Remote saved", gitRemote: &saved})

	assert.Equal(t, saved, updated.(Model).cfg.GitRemote)
}

// TestInitFlow_DoesNotRunGitInline pins the init wizard against the same
// blocking-UI problem the settings flow was fixed for: SetRemote forks up to
// two subprocesses, each with a 30s ceiling.
func TestInitFlow_DoesNotRunGitInline(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.repoDir = t.TempDir()
	m.initStep = 2
	m.settingsInput.SetValue("https://github.com/u/dots.git")

	updated, cmd := m.updateInit(tea.KeyMsg{Type: tea.KeyEnter})

	require.NotNil(t, cmd, "the git work must be handed to a command")
	assert.Equal(t, 0, updated.(Model).initStep)
	assert.NoError(t, updated.(Model).err)
}

// TestInitFlow_RejectsDangerousRemote keeps the RCE guard on the init path.
func TestInitFlow_RejectsDangerousRemote(t *testing.T) {
	m := NewModel(testCfg(), "test")
	m.repoDir = t.TempDir()
	m.initStep = 2
	m.settingsInput.SetValue("ext::sh -c 'touch /tmp/pwned'")

	updated, _ := m.updateInit(tea.KeyMsg{Type: tea.KeyEnter})

	require.Error(t, updated.(Model).err)
	assert.True(t, strings.Contains(updated.(Model).err.Error(), "invalid remote URL"))
}
