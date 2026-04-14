package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/stow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullStowWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	content := []byte("# zshrc content\nexport PATH=/usr/bin\n")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), content, 0644))

	result, err := stow.Link(repoDir, homeDir, "zsh")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)
	assert.Empty(t, result.Conflicts)

	symlinkPath := filepath.Join(homeDir, ".zshrc")
	linkTarget, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))

	resolved := filepath.Clean(filepath.Join(filepath.Dir(symlinkPath), linkTarget))
	expected := filepath.Clean(filepath.Join(repoDir, "zsh", ".zshrc"))
	assert.Equal(t, expected, resolved)

	readContent, err := os.ReadFile(symlinkPath)
	require.NoError(t, err)
	assert.Equal(t, content, readContent)

	packages, err := stow.DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, "zsh", packages[0].Name)
	assert.Equal(t, stow.StatusLinked, packages[0].Status)

	unlinkResult, err := stow.Unlink(repoDir, homeDir, "zsh")
	require.NoError(t, err)
	assert.Equal(t, 1, unlinkResult.Unlinked)

	_, err = os.Lstat(symlinkPath)
	assert.True(t, os.IsNotExist(err))

	packages, err = stow.DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, stow.StatusUnlinked, packages[0].Status)
}

func TestNestedFileStow(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "nvim", ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	content := []byte("local o = vim.o")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "nvim", ".config", "nvim", "init.lua"), content, 0644))

	result, err := stow.Link(repoDir, homeDir, "nvim")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)

	symlinkPath := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	assert.FileExists(t, symlinkPath)

	linkTarget, err := os.Readlink(symlinkPath)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))

	unlinkResult, err := stow.Unlink(repoDir, homeDir, "nvim")
	require.NoError(t, err)
	assert.Equal(t, 1, unlinkResult.Unlinked)
	assert.GreaterOrEqual(t, unlinkResult.Removed, 2)

	_, err = os.Lstat(symlinkPath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(homeDir, ".config"))
	assert.True(t, os.IsNotExist(err))
}

func TestConflictDetection(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), []byte("new"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte("existing"), 0644))

	result, err := stow.Link(repoDir, homeDir, "zsh")
	require.NoError(t, err)
	assert.Equal(t, 0, result.Linked)
	assert.Equal(t, 1, result.Skipped)
	assert.Contains(t, result.Conflicts, ".zshrc")

	existingContent, err := os.ReadFile(filepath.Join(homeDir, ".zshrc"))
	require.NoError(t, err)
	assert.Equal(t, []byte("existing"), existingContent)
}

func TestMultiplePackages(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "nvim", ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "git"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "nvim", ".config", "nvim", "init.lua"), []byte("n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "git", ".gitconfig"), []byte("g"), 0644))

	for _, pkg := range []string{"zsh", "nvim", "git"} {
		result, err := stow.Link(repoDir, homeDir, pkg)
		require.NoError(t, err)
		assert.Equal(t, 1, result.Linked)
	}

	packages, err := stow.DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 3)
	for _, p := range packages {
		assert.Equal(t, stow.StatusLinked, p.Status)
	}

	unlinkResult, err := stow.Unlink(repoDir, homeDir, "nvim")
	require.NoError(t, err)
	assert.Equal(t, 1, unlinkResult.Unlinked)

	assert.FileExists(t, filepath.Join(homeDir, ".zshrc"))
	assert.NoFileExists(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"))
	assert.FileExists(t, filepath.Join(homeDir, ".gitconfig"))

	packages, err = stow.DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	pkgMap := make(map[string]stow.Package)
	for _, p := range packages {
		pkgMap[p.Name] = p
	}
	assert.Equal(t, stow.StatusLinked, pkgMap["zsh"].Status)
	assert.Equal(t, stow.StatusUnlinked, pkgMap["nvim"].Status)
	assert.Equal(t, stow.StatusLinked, pkgMap["git"].Status)
}

func TestV1Migration(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "git"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "git", "gitconfig"), []byte("g"), 0644))

	assert.True(t, stow.DetectV1Layout(repoDir))

	steps, err := stow.PlanMigration(repoDir)
	require.NoError(t, err)
	require.Len(t, steps, 2)

	stepMap := make(map[string]string)
	for _, s := range steps {
		stepMap[filepath.Base(s.Dst)] = filepath.Base(s.Src)
	}
	assert.Equal(t, "zshrc", stepMap["zshrc"])
	assert.Equal(t, "gitconfig", stepMap["gitconfig"])

	err = stow.ExecuteMigration(repoDir, steps)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(repoDir, "shell", "zshrc"))
	assert.FileExists(t, filepath.Join(repoDir, "git", "gitconfig"))
	assert.False(t, stow.DetectV1Layout(repoDir))
}

func TestEmptyDirCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "app", ".config", "app"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "app", ".config", "app", "settings.json"), []byte("{}"), 0644))

	result, err := stow.Link(repoDir, homeDir, "app")
	require.NoError(t, err)
	assert.Equal(t, 1, result.Linked)

	symlinkPath := filepath.Join(homeDir, ".config", "app", "settings.json")
	assert.FileExists(t, symlinkPath)

	unlinkResult, err := stow.Unlink(repoDir, homeDir, "app")
	require.NoError(t, err)
	assert.Equal(t, 1, unlinkResult.Unlinked)
	assert.GreaterOrEqual(t, unlinkResult.Removed, 2)

	_, err = os.Stat(symlinkPath)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(homeDir, ".config"))
	assert.True(t, os.IsNotExist(err))
}
