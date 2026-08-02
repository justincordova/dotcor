package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdopt_ForeignSymlink_ReparentsToRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	oldDir := filepath.Join(tmpDir, "old-dotfiles")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(oldDir, "nvim"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("init"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "nvim", "options.lua"), []byte("opts"), 0644))

	require.NoError(t, os.Symlink(
		filepath.Join(oldDir, "nvim", "options.lua"),
		filepath.Join(homeDir, ".config", "nvim", "options.lua"),
	))

	_, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)

	result, err := Adopt(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Adopted)
	assert.Equal(t, 0, result.Skipped)
	assert.Empty(t, result.Failures)

	assert.FileExists(t, filepath.Join(pkgDir, ".config", "nvim", "options.lua"))

	linkTarget, err := os.Readlink(filepath.Join(homeDir, ".config", "nvim", "options.lua"))
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))

	resolved := filepath.Join(filepath.Dir(filepath.Join(homeDir, ".config", "nvim", "options.lua")), linkTarget)
	assert.Equal(t, filepath.Clean(filepath.Join(pkgDir, ".config", "nvim", "options.lua")), filepath.Clean(resolved))

	content, err := os.ReadFile(filepath.Join(pkgDir, ".config", "nvim", "options.lua"))
	require.NoError(t, err)
	assert.Equal(t, "opts", string(content))
}

func TestAdopt_NoForeignSymlinks_SkipsAll(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("cfg"), 0644))

	_, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)

	result, err := Adopt(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Adopted)
	assert.Equal(t, 0, result.Skipped)
}

func TestAdopt_BrokenSymlink_Skips(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("cfg"), 0644))
	require.NoError(t, os.Symlink("/nonexistent/file", filepath.Join(homeDir, ".config", "nvim", "broken.lua")))

	_, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)

	result, err := Adopt(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Adopted)
	assert.Equal(t, 1, result.Skipped)
	assert.Len(t, result.Failures, 1)
}

func TestAdopt_PackageNotFound_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	_, err := Adopt(repoDir, homeDir, "nonexistent")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestAdopt_MultipleForeignSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	oldDir := filepath.Join(tmpDir, "old-dotfiles")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim", "lua"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim", "lua"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(oldDir, "nvim", "lua"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("init"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "nvim", "lua", "opts.lua"), []byte("opts"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "nvim", "lua", "keys.lua"), []byte("keys"), 0644))

	require.NoError(t, os.Symlink(
		filepath.Join(oldDir, "nvim", "lua", "opts.lua"),
		filepath.Join(homeDir, ".config", "nvim", "lua", "opts.lua"),
	))
	require.NoError(t, os.Symlink(
		filepath.Join(oldDir, "nvim", "lua", "keys.lua"),
		filepath.Join(homeDir, ".config", "nvim", "lua", "keys.lua"),
	))

	_, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)

	result, err := Adopt(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 2, result.Adopted)
	assert.Empty(t, result.Failures)
	assert.FileExists(t, filepath.Join(pkgDir, ".config", "nvim", "lua", "opts.lua"))
	assert.FileExists(t, filepath.Join(pkgDir, ".config", "nvim", "lua", "keys.lua"))

	optsContent, err := os.ReadFile(filepath.Join(pkgDir, ".config", "nvim", "lua", "opts.lua"))
	require.NoError(t, err)
	assert.Equal(t, "opts", string(optsContent))
}

func TestAdopt_RegularFileNotSymlink_NotAdopted(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("init"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".config", "nvim", "regular.lua"), []byte("regular"), 0644))

	_, err := Link(repoDir, homeDir, "nvim", nil)
	require.NoError(t, err)

	result, err := Adopt(repoDir, homeDir, "nvim")

	require.NoError(t, err)
	assert.Equal(t, 0, result.Adopted)
}
