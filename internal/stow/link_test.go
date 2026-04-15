package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLink_SingleFile_CreatesSymlink(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("zsh config"), 0644))

	// Act
	result, err := Link(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)

	targetPath := filepath.Join(homeDir, ".zshrc")
	linkTarget, err := os.Readlink(targetPath)
	require.NoError(t, err)

	expectedRel, err := filepath.Rel(filepath.Dir(targetPath), filepath.Join(pkgDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, expectedRel, linkTarget)
}

func TestLink_NestedFile_CreatesParentDirs(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	// Act
	result, err := Link(repoDir, homeDir, "nvim")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)

	targetPath := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	assert.FileExists(t, targetPath)

	linkTarget, err := os.Readlink(targetPath)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))

	sourcePath := filepath.Join(pkgDir, ".config", "nvim", "init.lua")
	resolved := filepath.Join(filepath.Dir(targetPath), linkTarget)
	assert.Equal(t, filepath.Clean(sourcePath), filepath.Clean(resolved))
}

func TestLink_AlreadyLinked_Skips(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	sourceFile := filepath.Join(pkgDir, ".zshrc")
	require.NoError(t, os.WriteFile(sourceFile, []byte("cfg"), 0644))

	targetPath := filepath.Join(homeDir, ".zshrc")
	relLink, err := filepath.Rel(filepath.Dir(targetPath), sourceFile)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, targetPath))

	// Act
	result, err := Link(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Linked)
	assert.Equal(t, 1, result.Skipped)
	assert.Empty(t, result.Conflicts)
}

func TestLink_ConflictRegularFile_SkipsAndReports(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("existing"), 0644))

	// Act
	result, err := Link(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Linked)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.Conflicts, 1)
	assert.Equal(t, ".zshrc", result.Conflicts[0])

	content, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, "existing", string(content))
}

func TestLink_ConflictWrongSymlink_SkipsAndReports(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "other"), []byte("other"), 0644))

	targetPath := filepath.Join(homeDir, ".zshrc")
	relLink, err := filepath.Rel(filepath.Dir(targetPath), filepath.Join(tmpDir, "other"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, targetPath))

	// Act
	result, err := Link(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Linked)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.Conflicts, 1)
}

func TestLink_PackageNotFound_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Act
	_, err := Link(repoDir, homeDir, "nonexistent")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLink_MultipleFiles_LinksAll(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "tmux")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.conf"), []byte("tmux"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.theme.conf"), []byte("theme"), 0644))

	// Act
	result, err := Link(repoDir, homeDir, "tmux")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)

	assert.FileExists(t, filepath.Join(homeDir, ".tmux.conf"))
	assert.FileExists(t, filepath.Join(homeDir, ".tmux.theme.conf"))
}

func TestLink_MixedConflictAndClean(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "tmux")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.conf"), []byte("tmux"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.theme.conf"), []byte("theme"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".tmux.conf"), []byte("existing"), 0644))

	// Act
	result, err := Link(repoDir, homeDir, "tmux")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)
	assert.Equal(t, 1, result.Skipped)
	require.Len(t, result.Conflicts, 1)
	assert.Equal(t, ".tmux.conf", result.Conflicts[0])
}

func TestLink_EmptyPackage_LinksNothing(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "empty")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Act
	result, err := Link(repoDir, homeDir, "empty")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)
}

func TestLink_RelativeSymlinkPath(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	result, err := Link(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)

	targetPath := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	linkTarget, err := os.Readlink(targetPath)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))
	assert.Contains(t, linkTarget, "..")
}

func TestLinkWithBackup_ConflictRegularFile_BacksUpAndLinks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backups")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("existing"), 0600))

	result, err := LinkWithBackup(repoDir, homeDir, "zsh", backupDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)
	assert.Equal(t, 0, result.Skipped)

	content, readErr := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, readErr)
	assert.Equal(t, "repo", string(content))

	entries, readErr := os.ReadDir(backupDir)
	require.NoError(t, readErr)
	assert.GreaterOrEqual(t, len(entries), 1)
}

func TestLinkWithBackup_NoConflicts_ReturnsOriginalResult(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backups")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo"), 0644))

	result, err := LinkWithBackup(repoDir, homeDir, "zsh", backupDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)

	entries, readErr := os.ReadDir(backupDir)
	if readErr == nil {
		assert.Equal(t, 0, len(entries))
	}
}

func TestLinkWithBackup_MixedConflictAndClean(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	backupDir := filepath.Join(tmpDir, "backups")
	pkgDir := filepath.Join(repoDir, "tmux")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.conf"), []byte("tmux"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".tmux.theme.conf"), []byte("theme"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".tmux.conf"), []byte("existing"), 0644))

	result, err := LinkWithBackup(repoDir, homeDir, "tmux", backupDir)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)
}

func TestLink_AutoDetectedFiles_CopiesToRepoAndLinks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim", "lua", "plugins"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("init"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".config", "nvim", "lua", "plugins", "telescope.lua"), []byte("tel"), 0644))

	result, err := Link(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Linked)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Conflicts)

	repoCopy := filepath.Join(pkgDir, ".config", "nvim", "lua", "plugins", "telescope.lua")
	assert.FileExists(t, repoCopy)

	data, readErr := os.ReadFile(repoCopy)
	require.NoError(t, readErr)
	assert.Equal(t, "tel", string(data))

	homeLink := filepath.Join(homeDir, ".config", "nvim", "lua", "plugins", "telescope.lua")
	linkTarget, readErr := os.Readlink(homeLink)
	require.NoError(t, readErr)
	assert.False(t, filepath.IsAbs(linkTarget))
}
