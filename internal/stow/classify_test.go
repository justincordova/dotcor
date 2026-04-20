package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test helpers ─────────────────────────────────────────────────────────────

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

// ─── ClassifyFiles unit tests ─────────────────────────────────────────────────

func TestClassifyFiles_Add_FileDirectlyInHome(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Loose file directly in $HOME → Add.
	filePath := filepath.Join(homeDir, ".bashrc")
	writeFile(t, filePath, "bash content")

	plan, err := ClassifyFiles([]string{filePath}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	assert.Equal(t, "home", plan.Packages[0].Name)
	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassAdd, plan.Packages[0].Files[0].Class)
	assert.Equal(t, filePath, plan.Packages[0].Files[0].AbsPath)
}

func TestClassifyFiles_Adopt_HomeSymlinkPointsAtFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	srcDir := filepath.Join(tmpDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// File exists in a non-$HOME folder; $HOME has a symlink pointing at it.
	filePath := filepath.Join(srcDir, ".zshrc")
	writeFile(t, filePath, "zsh content")
	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(filePath, homeLink))

	plan, err := ClassifyFiles([]string{filePath}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	cf := plan.Packages[0].Files[0]
	assert.Equal(t, ClassAdopt, cf.Class)
	assert.Equal(t, homeLink, cf.HomeSymlink)
}

func TestClassifyFiles_Track_RegularFileInFolder_NoHomeLink(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	srcDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Regular file not in $HOME and no $HOME symlink → Track.
	filePath := filepath.Join(srcDir, "README.md")
	writeFile(t, filePath, "readme")

	plan, err := ClassifyFiles([]string{filePath}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassTrack, plan.Packages[0].Files[0].Class)
}

func TestClassifyFiles_Managed_SymlinkIntoRepo(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// File in repo; $HOME has symlink pointing to repo file.
	repoFile := filepath.Join(repoDir, "zsh", ".zshrc")
	writeFile(t, repoFile, "zsh content")
	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(repoFile, homeLink))

	// The home symlink is the "file" we're classifying.
	plan, err := ClassifyFiles([]string{homeLink}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassManaged, plan.Packages[0].Files[0].Class)
}

func TestClassifyFiles_Foreign_SymlinkOutsideRepo(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	externalFile := filepath.Join(tmpDir, "external", "work.conf")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	writeFile(t, externalFile, "work config")

	// File in $HOME is a symlink pointing outside repo → Foreign.
	homeLink := filepath.Join(homeDir, "work.conf")
	require.NoError(t, os.Symlink(externalFile, homeLink))

	plan, err := ClassifyFiles([]string{homeLink}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)
	cf := plan.Packages[0].Files[0]
	assert.Equal(t, ClassForeign, cf.Class)
	// ForeignTarget is resolved via EvalSymlinks (handles macOS /var → /private/var).
	resolvedExternal, err := filepath.EvalSymlinks(externalFile)
	require.NoError(t, err)
	assert.Equal(t, resolvedExternal, cf.ForeignTarget)
}

func TestClassifyFiles_StowParentSplit(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	dotfiles := filepath.Join(tmpDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Stow-parent: all direct children are directories.
	writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")
	writeFile(t, filepath.Join(dotfiles, "git", ".gitconfig"), "git")
	writeFile(t, filepath.Join(dotfiles, "nvim", ".config", "nvim", "init.lua"), "nvim")

	plan, err := ClassifyFiles([]string{dotfiles}, repoDir, homeDir)

	require.NoError(t, err)
	assert.Len(t, plan.Packages, 3)

	pkgNames := map[string]bool{}
	for _, pkg := range plan.Packages {
		pkgNames[pkg.Name] = true
	}
	assert.True(t, pkgNames["zsh"])
	assert.True(t, pkgNames["git"])
	assert.True(t, pkgNames["nvim"])
}

func TestClassifyFiles_MixedSelection_FolderAndLooseHomeFile(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	srcDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// A folder with a regular file (Track) and a $HOME file (Add).
	writeFile(t, filepath.Join(srcDir, "README.md"), "readme")
	homeFile := filepath.Join(homeDir, ".bashrc")
	writeFile(t, homeFile, "bash")

	plan, err := ClassifyFiles([]string{srcDir, homeFile}, repoDir, homeDir)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(plan.Packages), 1)

	classMap := map[Class]bool{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			classMap[cf.Class] = true
		}
	}
	assert.True(t, classMap[ClassTrack], "expected Track for folder file")
	assert.True(t, classMap[ClassAdd], "expected Add for $HOME file")
}

func TestClassifyFiles_NestedReAdd_ManagedFilesAppear(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	parentDir := filepath.Join(tmpDir, "parent")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// A regular file to track.
	regularFile := filepath.Join(parentDir, "regular.txt")
	writeFile(t, regularFile, "regular")

	// A managed file: symlink into repo.
	repoFile := filepath.Join(repoDir, "pkg", "managed.txt")
	writeFile(t, repoFile, "managed")
	managedLink := filepath.Join(parentDir, "managed.txt")
	require.NoError(t, os.Symlink(repoFile, managedLink))

	plan, err := ClassifyFiles([]string{parentDir}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)

	classMap := map[Class]int{}
	for _, cf := range plan.Packages[0].Files {
		classMap[cf.Class]++
	}
	assert.Equal(t, 1, classMap[ClassTrack], "expected 1 Track")
	assert.Equal(t, 1, classMap[ClassManaged], "expected 1 Managed")
}

func TestClassifyFiles_PackageMerge_SameNameCollides(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Two separate selections that derive the same package name should merge.
	file1 := filepath.Join(homeDir, ".bashrc")
	file2 := filepath.Join(homeDir, ".bash_profile")
	writeFile(t, file1, "bashrc")
	writeFile(t, file2, "bash_profile")

	plan, err := ClassifyFiles([]string{file1, file2}, repoDir, homeDir)

	require.NoError(t, err)
	// Both files should end up in the same "home" package.
	assert.Len(t, plan.Packages, 1)
	assert.Equal(t, "home", plan.Packages[0].Name)
	assert.Len(t, plan.Packages[0].Files, 2)
}

func TestClassifyFiles_ForeignDefault_IsOff(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	externalFile := filepath.Join(tmpDir, "ext", "work.conf")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	writeFile(t, externalFile, "content")

	homeLink := filepath.Join(homeDir, "work.conf")
	require.NoError(t, os.Symlink(externalFile, homeLink))

	plan, err := ClassifyFiles([]string{homeLink}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages[0].Files, 1)
	cf := plan.Packages[0].Files[0]
	assert.Equal(t, ClassForeign, cf.Class)

	// Verify default toggle is OFF.
	toggles := BuildDefaultToggles(plan)
	assert.False(t, toggles[FileID(plan.Packages[0].Name, cf.RelPath)])
}

func TestClassifyFiles_SymlinkedDir_NotFollowed(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	realDir := filepath.Join(tmpDir, "realdir")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// A regular file in a real dir.
	writeFile(t, filepath.Join(realDir, "file.txt"), "content")

	// Create a symlinked directory inside the walk root.
	walkRoot := filepath.Join(tmpDir, "walkroot")
	require.NoError(t, os.MkdirAll(walkRoot, 0755))
	writeFile(t, filepath.Join(walkRoot, "real.txt"), "real")
	require.NoError(t, os.Symlink(realDir, filepath.Join(walkRoot, "linked_dir")))

	plan, err := ClassifyFiles([]string{walkRoot}, repoDir, homeDir)

	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	// Only real.txt should appear; linked_dir is skipped.
	assert.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, "real.txt", plan.Packages[0].Files[0].RelPath)
}

// ─── ExecuteClassification integration tests ──────────────────────────────────

func TestExecuteClassification_Add_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Loose $HOME file.
	srcPath := filepath.Join(homeDir, ".bashrc")
	writeFile(t, srcPath, "bash content")

	plan, err := ClassifyFiles([]string{srcPath}, repoDir, homeDir)
	require.NoError(t, err)

	toggles := BuildDefaultToggles(plan)
	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Added)
	assert.Empty(t, result.Failures)

	// Original $HOME path should now be a symlink.
	lfi, err := os.Lstat(srcPath)
	require.NoError(t, err)
	assert.NotZero(t, lfi.Mode()&os.ModeSymlink)

	// Repo file should exist with original content.
	repoFile := filepath.Join(repoDir, "home", ".bashrc")
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)
	assert.Equal(t, "bash content", string(content))
}

func TestExecuteClassification_Adopt_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	srcDir := filepath.Join(tmpDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// File in external dir; $HOME symlink points at it.
	filePath := filepath.Join(srcDir, ".zshrc")
	writeFile(t, filePath, "zsh content")
	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(filePath, homeLink))

	plan, err := ClassifyFiles([]string{filePath}, repoDir, homeDir)
	require.NoError(t, err)

	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassAdopt, plan.Packages[0].Files[0].Class)

	toggles := BuildDefaultToggles(plan)
	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Adopted)
	assert.Empty(t, result.Failures)

	// $HOME symlink should now point at repo.
	resolved, err := filepath.EvalSymlinks(homeLink)
	require.NoError(t, err)
	pkgName := plan.Packages[0].Name
	relPath := plan.Packages[0].Files[0].RelPath
	expectedRepo := filepath.Join(repoDir, pkgName, relPath)
	expectedResolved, err := filepath.EvalSymlinks(expectedRepo)
	require.NoError(t, err)
	assert.Equal(t, expectedResolved, resolved)
}

func TestExecuteClassification_Track_HappyPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	srcDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	filePath := filepath.Join(srcDir, "README.md")
	writeFile(t, filePath, "readme content")

	plan, err := ClassifyFiles([]string{filePath}, repoDir, homeDir)
	require.NoError(t, err)

	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassTrack, plan.Packages[0].Files[0].Class)

	toggles := BuildDefaultToggles(plan)
	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Tracked)
	assert.Empty(t, result.Failures)

	// Source should now be a symlink.
	lfi, err := os.Lstat(filePath)
	require.NoError(t, err)
	assert.NotZero(t, lfi.Mode()&os.ModeSymlink)

	// Content preserved.
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "readme content", string(content))
}

func TestExecuteClassification_Foreign_ToggleOnPath(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	externalFile := filepath.Join(tmpDir, "ext", "work.conf")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	writeFile(t, externalFile, "work content")

	homeLink := filepath.Join(homeDir, "work.conf")
	require.NoError(t, os.Symlink(externalFile, homeLink))

	plan, err := ClassifyFiles([]string{homeLink}, repoDir, homeDir)
	require.NoError(t, err)

	require.Len(t, plan.Packages[0].Files, 1)
	cf := plan.Packages[0].Files[0]
	assert.Equal(t, ClassForeign, cf.Class)

	// Manually toggle ON.
	toggles := BuildDefaultToggles(plan)
	toggles[FileID(plan.Packages[0].Name, cf.RelPath)] = true

	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Foreign)
	assert.Empty(t, result.Failures)

	// Repo should have the file.
	pkgName := plan.Packages[0].Name
	repoFile := filepath.Join(repoDir, pkgName, cf.RelPath)
	content, err := os.ReadFile(repoFile)
	require.NoError(t, err)
	assert.Equal(t, "work content", string(content))
}

func TestExecuteClassification_Managed_NeverTouched(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	repoFile := filepath.Join(repoDir, "zsh", ".zshrc")
	writeFile(t, repoFile, "zsh content")
	homeLink := filepath.Join(homeDir, ".zshrc")
	require.NoError(t, os.Symlink(repoFile, homeLink))

	plan, err := ClassifyFiles([]string{homeLink}, repoDir, homeDir)
	require.NoError(t, err)

	require.Len(t, plan.Packages[0].Files, 1)
	assert.Equal(t, ClassManaged, plan.Packages[0].Files[0].Class)

	// Even if we try to force-toggle managed, it should be ignored.
	toggles := map[string]bool{
		FileID(plan.Packages[0].Name, plan.Packages[0].Files[0].RelPath): true,
	}

	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Managed)
	assert.Equal(t, 0, result.Added+result.Adopted+result.Tracked+result.Foreign)
	assert.Empty(t, result.Failures)
}

func TestExecuteClassification_Rollback_PartialFailureContinues(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("cannot test permission failures as root")
	}

	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// file1: loose $HOME file — should succeed (ClassAdd).
	file1 := filepath.Join(homeDir, ".bashrc")
	writeFile(t, file1, "bashrc")

	// file2: Track file whose repo destination parent will be made read-only.
	srcDir := filepath.Join(tmpDir, "configs")
	file2 := filepath.Join(srcDir, "locked.txt")
	writeFile(t, file2, "locked")

	plan, err := ClassifyFiles([]string{file1, file2}, repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, plan.Packages, 2)

	// Find which package corresponds to file2 (the Track/configs one).
	var trackPkgName string
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			if cf.AbsPath == file2 {
				trackPkgName = pkg.Name
			}
		}
	}
	require.NotEmpty(t, trackPkgName, "expected to find file2's package")

	// Create the repo destination parent and make it read-only so the write fails.
	trackPkgDir := filepath.Join(repoDir, trackPkgName)
	require.NoError(t, os.MkdirAll(trackPkgDir, 0755))
	require.NoError(t, os.Chmod(trackPkgDir, 0555)) // r-xr-xr-x: no write
	t.Cleanup(func() { _ = os.Chmod(trackPkgDir, 0755) })

	toggles := BuildDefaultToggles(plan)
	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err) // ExecuteClassification never returns an error itself
	assert.Equal(t, 1, result.Added, "file1 (Add) should succeed")
	assert.Equal(t, 1, len(result.Failures), "file2 (Track into read-only dir) should fail")
	assert.Equal(t, trackPkgName, result.Failures[0].PackageName)
}

func TestExecuteClassification_MultiPackage_AllExecute(t *testing.T) {
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	dotfiles := filepath.Join(tmpDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	// Stow-parent → multiple packages.
	writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")
	writeFile(t, filepath.Join(dotfiles, "git", ".gitconfig"), "git")

	// Create $HOME symlinks so these classify as Adopt.
	require.NoError(t, os.Symlink(
		filepath.Join(dotfiles, "zsh", ".zshrc"),
		filepath.Join(homeDir, ".zshrc"),
	))
	require.NoError(t, os.Symlink(
		filepath.Join(dotfiles, "git", ".gitconfig"),
		filepath.Join(homeDir, ".gitconfig"),
	))

	plan, err := ClassifyFiles([]string{dotfiles}, repoDir, homeDir)
	require.NoError(t, err)
	assert.Len(t, plan.Packages, 2)

	toggles := BuildDefaultToggles(plan)
	result, err := ExecuteClassification(plan, toggles, repoDir, homeDir)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Adopted)
	assert.Empty(t, result.Failures)
}
