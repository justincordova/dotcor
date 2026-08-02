package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectV1Layout_FilesDirExists_ReturnsTrue(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("cfg"), 0644))

	// Act
	result := DetectV1Layout(repoDir)

	// Assert
	assert.True(t, result)
}

func TestDetectV1Layout_NoFilesDir_ReturnsFalse(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755))

	// Act
	result := DetectV1Layout(repoDir)

	// Assert
	assert.False(t, result)
}

func TestDetectV1Layout_FilesIsFile_ReturnsFalse(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files"), []byte("not a dir"), 0644))

	// Act
	result := DetectV1Layout(repoDir)

	// Assert
	assert.False(t, result)
}

func TestPlanMigration_CorrectSteps(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "git", "gitconfig"), []byte("g"), 0644))

	// Act
	steps, err := PlanMigration(repoDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, steps, 2)

	stepMap := make(map[string]string)
	for _, s := range steps {
		stepMap[filepath.Base(s.Dst)] = filepath.Base(s.Src)
	}
	assert.Equal(t, "zshrc", stepMap["zshrc"])
	assert.Equal(t, "gitconfig", stepMap["gitconfig"])
}

func TestPlanMigration_NoFilesDir_ReturnsError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Act
	_, err := PlanMigration(repoDir)

	// Assert
	assert.Error(t, err)
}

func TestPlanMigration_NestedFiles(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "nvim", ".config", "nvim"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "nvim", ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	// Act
	steps, err := PlanMigration(repoDir)

	// Assert
	require.NoError(t, err)
	require.Len(t, steps, 1)

	expectedDst := filepath.Join(repoDir, "nvim", ".config", "nvim", "init.lua")
	assert.Equal(t, expectedDst, steps[0].Dst)
}

func TestExecuteMigration_MovesFiles(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "git", "gitconfig"), []byte("g"), 0644))

	steps, err := PlanMigration(repoDir)
	require.NoError(t, err)

	// Act
	err = ExecuteMigration(repoDir, steps)

	// Assert
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(repoDir, "shell", "zshrc"))
	assert.FileExists(t, filepath.Join(repoDir, "git", "gitconfig"))

	_, err = os.Stat(filepath.Join(repoDir, "files", "shell", "zshrc"))
	assert.True(t, os.IsNotExist(err))
}

func TestExecuteMigration_CleansEmptyFilesDir(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))

	steps, err := PlanMigration(repoDir)
	require.NoError(t, err)

	// Act
	err = ExecuteMigration(repoDir, steps)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(repoDir, "shell", "zshrc"))

	_, err = os.Stat(filepath.Join(repoDir, "files"))
	assert.True(t, os.IsNotExist(err))
}

func TestExecuteMigration_NestedFiles(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "nvim", ".config", "nvim"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "nvim", ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	steps, err := PlanMigration(repoDir)
	require.NoError(t, err)

	// Act
	err = ExecuteMigration(repoDir, steps)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(repoDir, "nvim", ".config", "nvim", "init.lua"))

	_, err = os.Stat(filepath.Join(repoDir, "files"))
	assert.True(t, os.IsNotExist(err))
}

func TestExecuteMigration_RollsBackOnFailure(t *testing.T) {
	// Arrange: a v1 layout with two files. The first migrates cleanly; the
	// second is forced to fail by pre-creating a regular file where the
	// destination directory needs to be — MkdirAll cannot turn a file into
	// a directory, so step 2 fails. The rollback must restore step 1.
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "git"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "git", "gitconfig"), []byte("g"), 0644))

	// Plant a file at "git" so MkdirAll(filepath.Dir(repoDir/git/gitconfig))
	// (which expands to repoDir/git) collides with a file.
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "git"), []byte("collision"), 0644))

	steps, err := PlanMigration(repoDir)
	require.NoError(t, err)
	require.Len(t, steps, 2)

	// Order steps so shell migrates first, then git fails.
	if filepath.Base(filepath.Dir(steps[0].Dst)) == "git" {
		steps[0], steps[1] = steps[1], steps[0]
	}

	// Act
	err = ExecuteMigration(repoDir, steps)

	// Assert: error mentions rollback, and the first file is back where it
	// started — the user is left in their original v1 layout.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back")

	zshrcSrc := filepath.Join(repoDir, "files", "shell", "zshrc")
	zshrcDst := filepath.Join(repoDir, "shell", "zshrc")
	assert.FileExists(t, zshrcSrc, "v1 source must be restored on rollback")
	_, statErr := os.Stat(zshrcDst)
	assert.True(t, os.IsNotExist(statErr), "v2 destination must not exist after rollback")
}

func TestFullMigration_V1ToV2(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "shell"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "git"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "files", "nvim", ".config", "nvim"), 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "shell", "zshrc"), []byte("z"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "git", "gitconfig"), []byte("g"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "files", "nvim", ".config", "nvim", "init.lua"), []byte("lua"), 0644))

	require.True(t, DetectV1Layout(repoDir))

	steps, err := PlanMigration(repoDir)
	require.NoError(t, err)
	require.Len(t, steps, 3)

	// Act - execute migration
	err = ExecuteMigration(repoDir, steps)
	require.NoError(t, err)

	assert.False(t, DetectV1Layout(repoDir))

	// Act - discover packages after migration
	packages, err := DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)

	pkgMap := make(map[string]Package)
	for _, p := range packages {
		pkgMap[p.Name] = p
	}

	assert.Contains(t, pkgMap, "shell")
	assert.Contains(t, pkgMap, "git")
	assert.Contains(t, pkgMap, "nvim")

	require.Len(t, pkgMap["nvim"].Files, 1)
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), pkgMap["nvim"].Files[0].RelPath)

	// Act - link a package after migration
	linkResult, err := Link(repoDir, homeDir, "shell")
	require.NoError(t, err)
	assert.Equal(t, 1, linkResult.Linked)

	assert.FileExists(t, filepath.Join(homeDir, "zshrc"))

	targetPath := filepath.Join(homeDir, "zshrc")
	linkTarget, err := os.Readlink(targetPath)
	require.NoError(t, err)
	assert.False(t, filepath.IsAbs(linkTarget))
}

// TestCleanEmptyParents_StopsAtRepoRootWithTrailingSeparator pins the fix for
// a boundary check that compared raw strings.
//
// stopAt comes from config.GetConfigDir, which returns $DOTCOR_DIR verbatim.
// A trailing separator never string-equals the canonical paths produced by
// filepath.Walk, so the loop walked past the repository root, removed it, and
// continued up into $HOME.
func TestCleanEmptyParents_StopsAtRepoRootWithTrailingSeparator(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "dotcor")
	deep := filepath.Join(repoDir, "files", "nested")
	require.NoError(t, os.MkdirAll(deep, 0755))

	// stopAt as a user would supply it via DOTCOR_DIR with a trailing slash.
	cleanEmptyParents(deep, repoDir+string(filepath.Separator))

	_, err := os.Stat(repoDir)
	assert.NoError(t, err, "the repository root must never be removed")

	_, err = os.Stat(tmp)
	assert.NoError(t, err, "directories above the repository must never be touched")

	_, err = os.Stat(deep)
	assert.True(t, os.IsNotExist(err), "the empty source tree should still be cleaned")
}

// TestCleanEmptyParents_LeavesNonEmptyDirectories guards the normal path.
func TestCleanEmptyParents_LeavesNonEmptyDirectories(t *testing.T) {
	tmp := t.TempDir()
	repoDir := filepath.Join(tmp, "dotcor")
	keep := filepath.Join(repoDir, "files")
	deep := filepath.Join(keep, "nested")
	require.NoError(t, os.MkdirAll(deep, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(keep, "keep.txt"), []byte("x"), 0644))

	cleanEmptyParents(deep, repoDir)

	_, err := os.Stat(keep)
	assert.NoError(t, err, "a directory with contents must survive")
	_, err = os.Stat(deep)
	assert.True(t, os.IsNotExist(err), "the empty leaf should be removed")
}
