package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnlink_LinkedFile_RemovesSymlink(t *testing.T) {
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
	result, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Unlinked)
	assert.Equal(t, 0, result.Removed)

	_, err = os.Lstat(targetPath)
	assert.True(t, os.IsNotExist(err))

	assert.FileExists(t, sourceFile)
}

func TestUnlink_NestedFile_RemovesSymlinkAndCleansEmptyDirs(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	sourceFile := filepath.Join(pkgDir, ".config", "nvim", "init.lua")
	require.NoError(t, os.WriteFile(sourceFile, []byte("lua"), 0644))

	targetPath := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0755))
	relLink, err := filepath.Rel(filepath.Dir(targetPath), sourceFile)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, targetPath))

	// Act
	result, err := Unlink(repoDir, homeDir, "nvim")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Unlinked)
	assert.Equal(t, 2, result.Removed)

	_, err = os.Lstat(targetPath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(homeDir, ".config"))
	assert.True(t, os.IsNotExist(err))
}

func TestUnlink_NotLinked_Skips(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("cfg"), 0644))

	// Act
	result, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Unlinked)
	assert.Equal(t, 0, result.Removed)
}

func TestUnlink_RegularFile_Skips(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("repo"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("regular file"), 0644))

	// Act
	result, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Unlinked)

	content, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, "regular file", string(content))
}

func TestUnlink_SymlinkPointsElsewhere_Skips(t *testing.T) {
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
	result, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Unlinked)

	_, err = os.Lstat(targetPath)
	assert.NoError(t, err)
}

func TestUnlink_PackageNotFound_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Act
	_, err := Unlink(repoDir, homeDir, "nonexistent")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUnlink_MultipleFiles_UnlinksAll(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "tmux")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	source1 := filepath.Join(pkgDir, ".tmux.conf")
	require.NoError(t, os.WriteFile(source1, []byte("tmux"), 0644))
	source2 := filepath.Join(pkgDir, ".tmux.theme.conf")
	require.NoError(t, os.WriteFile(source2, []byte("theme"), 0644))

	target1 := filepath.Join(homeDir, ".tmux.conf")
	rel1, err := filepath.Rel(filepath.Dir(target1), source1)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel1, target1))

	target2 := filepath.Join(homeDir, ".tmux.theme.conf")
	rel2, err := filepath.Rel(filepath.Dir(target2), source2)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel2, target2))

	// Act
	result, err := Unlink(repoDir, homeDir, "tmux")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, result.Unlinked)

	_, err = os.Lstat(target1)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Lstat(target2)
	assert.True(t, os.IsNotExist(err))
}

func TestUnlink_DoesNotRemoveHomeDir(t *testing.T) {
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
	result, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, result.Unlinked)

	assert.DirExists(t, homeDir)
}

func TestUnlink_EmptyPackage_UnlinksNothing(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "empty")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Act
	result, err := Unlink(repoDir, homeDir, "empty")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, result.Unlinked)
	assert.Equal(t, 0, result.Removed)
}

func TestLinkThenUnlink_RoundTrip(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("cfg"), 0644))

	linkResult, err := Link(repoDir, homeDir, "zsh", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, linkResult.Linked)

	// Act
	unlinkResult, err := Unlink(repoDir, homeDir, "zsh")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, unlinkResult.Unlinked)

	_, err = os.Lstat(filepath.Join(homeDir, ".zshrc"))
	assert.True(t, os.IsNotExist(err))
	assert.FileExists(t, filepath.Join(pkgDir, ".zshrc"))
}
