package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverPackages_EmptyRepo_ReturnsEmpty(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, packages)
}

func TestDiscoverPackages_SinglePackage_Unlinked(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".zshrc"), []byte("zsh config"), 0644))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, "zsh", packages[0].Name)
	assert.Equal(t, StatusUnlinked, packages[0].Status)
	require.Len(t, packages[0].Files, 1)
	assert.Equal(t, ".zshrc", packages[0].Files[0].RelPath)
	assert.Equal(t, filepath.Join(homeDir, ".zshrc"), packages[0].Files[0].TargetPath)
	assert.False(t, packages[0].Files[0].IsLinked)
	assert.False(t, packages[0].Files[0].Exists)
}

func TestDiscoverPackages_LinkedPackage_ReturnsLinked(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "zsh")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	sourceFile := filepath.Join(pkgDir, ".zshrc")
	require.NoError(t, os.WriteFile(sourceFile, []byte("zsh config"), 0644))

	targetPath := filepath.Join(homeDir, ".zshrc")
	relLink, err := filepath.Rel(filepath.Dir(targetPath), sourceFile)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, targetPath))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusLinked, packages[0].Status)
	assert.True(t, packages[0].Files[0].IsLinked)
	assert.True(t, packages[0].Files[0].Exists)
	assert.True(t, packages[0].Files[0].IsSymlink)
}

func TestDiscoverPackages_PartialLink_ReturnsPartial(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "tmux")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	file1 := filepath.Join(pkgDir, ".tmux.conf")
	require.NoError(t, os.WriteFile(file1, []byte("tmux config"), 0644))
	file2 := filepath.Join(pkgDir, ".tmux.theme.conf")
	require.NoError(t, os.WriteFile(file2, []byte("tmux theme"), 0644))

	target1 := filepath.Join(homeDir, ".tmux.conf")
	relLink, err := filepath.Rel(filepath.Dir(target1), file1)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, target1))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusPartial, packages[0].Status)
	require.Len(t, packages[0].Files, 2)
	assert.True(t, packages[0].Files[0].IsLinked)
	assert.False(t, packages[0].Files[1].IsLinked)
}

func TestDiscoverPackages_ExcludesSpecialDirs(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "logs"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "backups"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".stow-local-ignore"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), []byte("cfg"), 0644))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, "zsh", packages[0].Name)
}

func TestDiscoverPackages_ExcludesDotfileDirs(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".dotcorrc_dir"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), []byte("cfg"), 0644))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, "zsh", packages[0].Name)
}

func TestDiscoverPackages_NestedFiles(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	require.Len(t, packages[0].Files, 1)
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), packages[0].Files[0].RelPath)
	assert.Equal(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"), packages[0].Files[0].TargetPath)
}

func TestDiscoverPackages_Conflict_DetectsNonSymlinkFile(t *testing.T) {
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
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.True(t, packages[0].Files[0].Exists)
	assert.False(t, packages[0].Files[0].IsSymlink)
	assert.False(t, packages[0].Files[0].IsLinked)
}

func TestDiscoverPackages_InvalidRepoDir_ReturnsError(t *testing.T) {
	// Act
	_, err := DiscoverPackages("/nonexistent/path", "/home")

	// Assert
	assert.Error(t, err)
}

func TestDiscoverPackages_MultiplePackages(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "git"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "zsh", ".zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "git", ".gitconfig"), []byte("g"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "nvim", "init.lua"), []byte("n"), 0644))

	// Act
	packages, err := DiscoverPackages(repoDir, homeDir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, packages, 3)

	names := make(map[string]bool)
	for _, p := range packages {
		names[p.Name] = true
	}
	assert.True(t, names["zsh"])
	assert.True(t, names["git"])
	assert.True(t, names["nvim"])
}

func TestComputeStatus_EmptyFiles_ReturnsUnlinked(t *testing.T) {
	assert.Equal(t, StatusUnlinked, computeStatus(nil))
	assert.Equal(t, StatusUnlinked, computeStatus([]FileEntry{}))
}

func TestDiscoverPackages_AutoDetectsNewFilesInManagedTree(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim", "lua", "plugins"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".config", "nvim", "init.lua"), []byte("init"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".config", "nvim", "lua", "plugins", "telescope.lua"), []byte("tel"), 0644))

	packages, err := DiscoverPackages(repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, packages, 1)
	require.Len(t, packages[0].Files, 2)

	repoFiles := 0
	autoFiles := 0
	for _, f := range packages[0].Files {
		if f.InRepo {
			repoFiles++
			assert.Equal(t, ".config/nvim/init.lua", f.RelPath)
		} else {
			autoFiles++
			assert.Equal(t, filepath.Join(".config", "nvim", "lua", "plugins", "telescope.lua"), f.RelPath)
			assert.True(t, f.Exists)
			assert.False(t, f.IsLinked)
		}
	}
	assert.Equal(t, 1, repoFiles)
	assert.Equal(t, 1, autoFiles)
	assert.Equal(t, StatusUnlinked, packages[0].Status)
}

func TestDiscoverPackages_NoAutoDetectForTopLevelFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "git")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, ".gitconfig"), []byte("git"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".gitignore_global"), []byte("ignore"), 0644))

	packages, err := DiscoverPackages(repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, packages, 1)
	require.Len(t, packages[0].Files, 1)
	assert.True(t, packages[0].Files[0].InRepo)
}

func TestDiscoverPackages_AutoDetectSetsStatusToPartial(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	pkgDir := filepath.Join(repoDir, "nvim")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "nvim"), 0755))

	repoFile := filepath.Join(pkgDir, ".config", "nvim", "init.lua")
	require.NoError(t, os.WriteFile(repoFile, []byte("init"), 0644))

	targetPath := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	relLink, err := filepath.Rel(filepath.Dir(targetPath), repoFile)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relLink, targetPath))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".config", "nvim", "new.lua"), []byte("new"), 0644))

	packages, err := DiscoverPackages(repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusPartial, packages[0].Status)
	require.Len(t, packages[0].Files, 2)
}
