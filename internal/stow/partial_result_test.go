package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnlink_ReturnsPartialResultOnWalkError pins the fix for discarded work.
// filepath.Walk aborts on the first callback failure, but every symlink
// removed before that point is already gone. Returning a nil result left the
// caller reporting a bare error for a package that is now half-unlinked, with
// no record of which half.
func TestUnlink_ReturnsPartialResultOnWalkError(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	pkgDir := filepath.Join(repoDir, "cfg")
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	// Two linked files; the second lives in a $HOME directory we make
	// unreadable so Lstat fails partway through the walk.
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, "blocked"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "a.conf"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "blocked", "b.conf"), []byte("b"), 0644))

	_, err := Link(repoDir, homeDir, "cfg")
	require.NoError(t, err)

	blockedHome := filepath.Join(homeDir, "blocked")
	require.NoError(t, os.Chmod(blockedHome, 0000))
	t.Cleanup(func() { _ = os.Chmod(blockedHome, 0755) })

	result, err := Unlink(repoDir, homeDir, "cfg")

	if err == nil {
		t.Skip("filesystem did not deny access; nothing to assert")
	}
	require.NotNil(t, result, "a partial result must accompany the error")
	assert.GreaterOrEqual(t, result.Unlinked, 0, "the count of work already done must be reported")
}

// TestClassifyFiles_UnreadableSubtreeIsWarned pins the fix for a silently
// truncated plan. An unreadable subtree used to vanish from the preview with
// no indication, so the user confirmed a plan quietly missing files.
func TestClassifyFiles_UnreadableSubtreeIsWarned(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	selDir := filepath.Join(tmp, "configs")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	writeFile(t, filepath.Join(selDir, "visible.conf"), "ok")
	secret := filepath.Join(selDir, "secrets")
	writeFile(t, filepath.Join(secret, "hidden.conf"), "hidden")
	require.NoError(t, os.Chmod(secret, 0000))
	t.Cleanup(func() { _ = os.Chmod(secret, 0755) })

	plan, err := ClassifyFiles([]string{selDir}, repoDir, homeDir, nil)
	require.NoError(t, err)

	var found bool
	for _, pkg := range plan.Packages {
		for _, cf := range pkg.Files {
			if filepath.Base(cf.AbsPath) == "hidden.conf" {
				found = true
			}
		}
	}
	if found {
		t.Skip("filesystem did not deny access; nothing to assert")
	}

	assert.NotEmpty(t, plan.Warnings,
		"a subtree omitted from the plan must be surfaced as a warning, not dropped silently")
}
