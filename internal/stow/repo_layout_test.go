package stow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClassifyFiles_SameBasenameSelectionsDoNotCollide pins the fix for
// silent data loss.
//
// Repo-relative paths were measured from the selection directory, and
// derivePkgName collapses a deep path to its leaf. Selecting ~/.config/nvim
// and ~/.local/share/nvim therefore produced the same package AND the same
// relative path, so the second file overwrote the first in the repo and both
// $HOME symlinks pointed at it. One file's only copy was destroyed — it was
// not in $HOME, not in the repo, not in backups — and the operation reported
// success.
func TestClassifyFiles_SameBasenameSelectionsDoNotCollide(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	a := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	b := filepath.Join(homeDir, ".local", "share", "nvim", "init.lua")
	writeFile(t, a, "CONFIG-NVIM")
	writeFile(t, b, "LOCAL-SHARE-NVIM")

	plan, err := ClassifyFiles([]string{filepath.Dir(a), filepath.Dir(b)}, repoDir, homeDir, nil)
	require.NoError(t, err)

	dests := map[string]bool{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			assert.False(t, dests[cf.RepoDest],
				"two source files must never share a repo destination: %s", cf.RepoDest)
			dests[cf.RepoDest] = true
		}
	}

	result, err := ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)
	require.Empty(t, result.Failures)

	gotA, err := os.ReadFile(a)
	require.NoError(t, err)
	gotB, err := os.ReadFile(b)
	require.NoError(t, err)

	assert.Equal(t, "CONFIG-NVIM", string(gotA), "each file must keep its own content")
	assert.Equal(t, "LOCAL-SHARE-NVIM", string(gotB))
}

// TestClassifyFiles_RepoMirrorsHomeLayout pins the contract every other
// component assumes: Link, Unlink and DiscoverPackages all map
// repo/<pkg>/<rel> to $HOME/<rel>, and SPEC.md documents package nvim as
// holding .config/nvim/init.lua.
func TestClassifyFiles_RepoMirrorsHomeLayout(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	src := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	writeFile(t, src, "vim.opt.number = true")

	plan, err := ClassifyFiles([]string{filepath.Dir(src)}, repoDir, homeDir, nil)
	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)
	require.Len(t, plan.Packages[0].Files, 1)

	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"),
		plan.Packages[0].Files[0].RelPath,
		"the package must mirror the $HOME-relative path")
}

// TestClassifyFiles_AddedFileRoundTripsThroughLinkAndUnlink is the end-to-end
// consequence: after adding, discovery must report the package linked, and
// unlink must be able to remove the link it created.
func TestClassifyFiles_AddedFileRoundTripsThroughLinkAndUnlink(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	src := filepath.Join(homeDir, ".config", "nvim", "init.lua")
	writeFile(t, src, "vim.opt.number = true")

	plan, err := ClassifyFiles([]string{filepath.Dir(src)}, repoDir, homeDir, nil)
	require.NoError(t, err)
	_, err = ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)

	packages, err := DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	require.Len(t, packages, 1)
	assert.Equal(t, StatusLinked, packages[0].Status,
		"a file that was just added must show as linked, not unlinked")

	// Stowing again must be a no-op, not create a stray link at $HOME root.
	_, err = Link(repoDir, homeDir, packages[0].Name, nil)
	require.NoError(t, err)
	_, strayErr := os.Lstat(filepath.Join(homeDir, "init.lua"))
	assert.True(t, os.IsNotExist(strayErr), "stow must not create a stray ~/init.lua")

	// And unlink must be able to remove the real link.
	un, err := Unlink(repoDir, homeDir, packages[0].Name)
	require.NoError(t, err)
	assert.Equal(t, 1, un.Unlinked, "unlink must remove the link that was created")
}

// TestClassifyFiles_StowRepoKeepsPackageRelativePaths guards the Stow import
// convention: inside a real Stow repository, package contents are already
// $HOME-relative. The evidence that it IS one is that its entries correspond
// to real $HOME paths — here ~/.zshrc and ~/.config exist, as they do in any
// Stow setup that has been linked.
func TestClassifyFiles_StowRepoKeepsPackageRelativePaths(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	dotfiles := filepath.Join(homeDir, "dotfiles")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(dotfiles, "nvim", ".config", "nvim", "init.lua"), "nvim")
	writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")

	// The $HOME side of a linked Stow setup. Evidence is required per
	// package, so each package needs its own link — which is exactly what
	// stow creates (it folds ~/.config into the only package providing it).
	require.NoError(t, os.Symlink(filepath.Join(dotfiles, "zsh", ".zshrc"), filepath.Join(homeDir, ".zshrc")))
	require.NoError(t, os.Symlink(filepath.Join(dotfiles, "nvim", ".config"), filepath.Join(homeDir, ".config")))

	plan, err := ClassifyFiles([]string{dotfiles}, repoDir, homeDir, nil)
	require.NoError(t, err)

	rels := map[string]string{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			rels[pkg.Name] = cf.RelPath
		}
	}
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), rels["nvim"])
	assert.Equal(t, ".zshrc", rels["zsh"])
}

// TestClassifyFiles_XdgCompatLinkDoesNotFlipSiblings pins the fix for one
// package's evidence deciding for all of them.
//
// A legacy-compat link such as ~/.gitconfig → ~/.config/git/.gitconfig is
// genuine evidence for the git package, but applying it to every sibling
// measured ~/.config/nvim/init.lua as a bare "init.lua" — breaking that
// package's $HOME mapping and scattering a stray ~/init.lua on the next stow.
func TestClassifyFiles_XdgCompatLinkDoesNotFlipSiblings(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	gitconf := filepath.Join(homeDir, ".config", "git", ".gitconfig")
	writeFile(t, gitconf, "git")
	writeFile(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"), "nvim")
	require.NoError(t, os.Symlink(gitconf, filepath.Join(homeDir, ".gitconfig")))

	plan, err := ClassifyFiles([]string{filepath.Join(homeDir, ".config")}, repoDir, homeDir, nil)
	require.NoError(t, err)

	rels := map[string]string{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			rels[pkg.Name] = cf.RelPath
		}
	}

	// git has its own evidence, so it keeps the stow-style path that maps
	// back to the ~/.gitconfig the user actually uses.
	assert.Equal(t, ".gitconfig", rels["git"])
	// nvim has none, so it must stay $HOME-relative.
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), rels["nvim"],
		"a sibling package must not inherit another package's evidence")
}

// TestClassifyFiles_LooseHomeFileProducesNoWarning pins the fix for a false
// alarm on the most common selection there is. A depth-1 file was added to
// the directory scan list, and the resulting ENOTDIR was reported as
// "adopt detection degraded" — untruthfully, since $HOME was already scanned.
func TestClassifyFiles_LooseHomeFileProducesNoWarning(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	zshrc := filepath.Join(homeDir, ".zshrc")
	writeFile(t, zshrc, "export ZSH=1")

	plan, err := ClassifyFiles([]string{zshrc}, repoDir, homeDir, nil)

	require.NoError(t, err)
	assert.Empty(t, plan.Warnings, "selecting a loose $HOME file must not warn")
	require.Len(t, plan.Packages, 1)
	assert.Equal(t, ".zshrc", plan.Packages[0].Files[0].RelPath)
}

// TestClassifyFiles_DotConfigIsNotAStowRepo pins the discriminator. ~/.config
// and ~/.local are dirs-only on virtually every machine, so a purely
// structural "every child is a directory" test classified them as Stow repos
// — measuring ~/.config/nvim/init.lua as a bare "init.lua", which broke the
// repo↔$HOME mapping and let two selections claim one destination.
func TestClassifyFiles_DotConfigIsNotAStowRepo(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(homeDir, ".config", "nvim", "init.lua"), "nvim")
	writeFile(t, filepath.Join(homeDir, ".config", "starship", "config.toml"), "starship")

	plan, err := ClassifyFiles([]string{filepath.Join(homeDir, ".config")}, repoDir, homeDir, nil)
	require.NoError(t, err)

	rels := map[string]string{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			rels[pkg.Name] = cf.RelPath
		}
	}
	assert.Equal(t, filepath.Join(".config", "nvim", "init.lua"), rels["nvim"])
	assert.Equal(t, filepath.Join(".config", "starship", "config.toml"), rels["starship"])
}

// TestClassifyFiles_DirsOnlyHomeParentsDoNotCollide is the data-loss case that
// reached this code path even after the first layout fix: ~/.config and
// ~/.local are both dirs-only, both split into packages, and a shared child
// name produced one repo destination for two different files.
func TestClassifyFiles_DirsOnlyHomeParentsDoNotCollide(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	a := filepath.Join(homeDir, ".config", "app", "settings.json")
	b := filepath.Join(homeDir, ".local", "app", "settings.json")
	writeFile(t, a, "CONFIG-VERSION")
	writeFile(t, b, "LOCAL-VERSION")

	plan, err := ClassifyFiles(
		[]string{filepath.Join(homeDir, ".config"), filepath.Join(homeDir, ".local")},
		repoDir, homeDir, nil)
	require.NoError(t, err)

	dests := map[string]bool{}
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			assert.False(t, dests[cf.RepoDest], "duplicate repo destination: %s", cf.RepoDest)
			dests[cf.RepoDest] = true
		}
	}

	_, err = ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)

	gotA, err := os.ReadFile(a)
	require.NoError(t, err)
	gotB, err := os.ReadFile(b)
	require.NoError(t, err)
	assert.Equal(t, "CONFIG-VERSION", string(gotA))
	assert.Equal(t, "LOCAL-VERSION", string(gotB))
}

// TestMergeFile_RefusesCollidingDestination pins the structural guarantee:
// whatever the layout logic decides, one repo path can only ever be claimed by
// one source file, and the user is told about the refusal.
func TestMergeFile_RefusesCollidingDestination(t *testing.T) {
	plan := &ClassificationPlan{}
	pkgIndex := map[string]int{}
	destIndex := map[string]string{}

	first := ClassifiedFile{RelPath: "x", AbsPath: "/home/u/a/x", PackageName: "p", RepoDest: "/repo/p/x"}
	second := ClassifiedFile{RelPath: "x", AbsPath: "/home/u/b/x", PackageName: "p", RepoDest: "/repo/p/x"}

	mergeFile(plan, pkgIndex, destIndex, first)
	mergeFile(plan, pkgIndex, destIndex, second)

	require.Len(t, plan.Packages, 1)
	assert.Len(t, plan.Packages[0].Files, 1, "the colliding file must not be added")
	assert.Equal(t, "/home/u/a/x", plan.Packages[0].Files[0].AbsPath, "the first claim wins")
	require.Len(t, plan.Warnings, 1, "the refusal must be surfaced")
	assert.Contains(t, plan.Warnings[0], "/home/u/b/x")
}

// TestClassifyFiles_OutsideHomeKeepsSelectionRelativePaths guards sources
// that have no $HOME mapping at all.
func TestClassifyFiles_OutsideHomeKeepsSelectionRelativePaths(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	external := filepath.Join(tmp, "external", "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(external, "sub", "app.conf"), "cfg")
	writeFile(t, filepath.Join(external, "top.conf"), "top")

	plan, err := ClassifyFiles([]string{external}, repoDir, homeDir, nil)
	require.NoError(t, err)
	require.Len(t, plan.Packages, 1)

	rels := map[string]bool{}
	for _, cf := range plan.Packages[0].Files {
		rels[cf.RelPath] = true
	}
	assert.True(t, rels[filepath.Join("sub", "app.conf")], "got %v", rels)
	assert.True(t, rels["top.conf"], "got %v", rels)
}

// TestUsesStowConvention_RequiresSymlinkEvidence pins the fix for a
// discriminator that fired on ordinary $HOME layouts.
//
// Matching on a shared NAME meant "~/.local/pipx/.cache matches ~/.cache" or
// "~/.config/emacs/bin matches ~/bin" flipped the base for every package in
// the selection, so Add scattered symlinks at $HOME root and into unrelated
// existing directories. Only a $HOME symlink resolving INTO the package is
// evidence that stowing actually happened.
func TestPackageUsesStowConvention_RequiresSymlinkEvidence(t *testing.T) {
	t.Run("name collision is not evidence", func(t *testing.T) {
		tmp := t.TempDir()
		homeDir := filepath.Join(tmp, "home")
		local := filepath.Join(homeDir, ".local")

		// A real ~/.local layout, and a real ~/.cache that merely shares a
		// name with ~/.local/pipx/.cache.
		writeFile(t, filepath.Join(local, "pipx", ".cache", "x"), "x")
		writeFile(t, filepath.Join(local, "bin", "uv"), "uv")
		require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".cache"), 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(homeDir, "bin"), 0755))

		assert.False(t, packageUsesStowConvention(filepath.Join(local, "pipx"), homeDir),
			"a shared directory name is not evidence of a Stow package")
		assert.False(t, packageUsesStowConvention(filepath.Join(local, "bin"), homeDir))
	})

	t.Run("symlink into the package is evidence", func(t *testing.T) {
		tmp := t.TempDir()
		homeDir := filepath.Join(tmp, "home")
		dotfiles := filepath.Join(homeDir, "dotfiles")
		require.NoError(t, os.MkdirAll(homeDir, 0755))
		writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")
		require.NoError(t, os.Symlink(
			filepath.Join(dotfiles, "zsh", ".zshrc"), filepath.Join(homeDir, ".zshrc")))

		assert.True(t, packageUsesStowConvention(filepath.Join(dotfiles, "zsh"), homeDir))
	})

	t.Run("symlink pointing elsewhere is not evidence", func(t *testing.T) {
		tmp := t.TempDir()
		homeDir := filepath.Join(tmp, "home")
		dotfiles := filepath.Join(homeDir, "dotfiles")
		other := filepath.Join(tmp, "other")
		require.NoError(t, os.MkdirAll(homeDir, 0755))
		writeFile(t, filepath.Join(dotfiles, "zsh", ".zshrc"), "zsh")
		writeFile(t, filepath.Join(other, ".zshrc"), "other")
		require.NoError(t, os.Symlink(filepath.Join(other, ".zshrc"), filepath.Join(homeDir, ".zshrc")))

		assert.False(t, packageUsesStowConvention(filepath.Join(dotfiles, "zsh"), homeDir),
			"a $HOME symlink pointing somewhere else is not evidence")
	})
}

// TestClassifyFiles_DotLocalWithNameCollisionsMapsToHome is the end-to-end
// consequence: Add must produce packages that DiscoverPackages agrees are
// linked, and must not scatter symlinks at $HOME root.
func TestClassifyFiles_DotLocalWithNameCollisionsMapsToHome(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	local := filepath.Join(homeDir, ".local")
	writeFile(t, filepath.Join(local, "pipx", ".cache", "x"), "x")
	writeFile(t, filepath.Join(local, "bin", "uv"), "uv")
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".cache"), 0755))

	plan, err := ClassifyFiles([]string{local}, repoDir, homeDir, nil)
	require.NoError(t, err)

	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			assert.True(t, strings.HasPrefix(cf.RelPath, ".local"+string(filepath.Separator)),
				"paths must stay $HOME-relative, got %q", cf.RelPath)
		}
	}

	_, err = ExecuteClassification(plan, BuildDefaultToggles(plan), repoDir, homeDir)
	require.NoError(t, err)

	packages, err := DiscoverPackages(repoDir, homeDir)
	require.NoError(t, err)
	for _, pkg := range packages {
		assert.Equal(t, StatusLinked, pkg.Status,
			"package %s must be reported linked right after Add", pkg.Name)
	}

	_, strayErr := os.Lstat(filepath.Join(homeDir, "uv"))
	assert.True(t, os.IsNotExist(strayErr), "stow must not scatter a symlink at $HOME root")
}
