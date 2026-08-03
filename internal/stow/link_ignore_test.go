package stow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLink_AutoDetectHonoursIgnorePatterns pins the fix for secrets being
// swept into the repository by a plain stow.
//
// Link's auto-detect pass walks the package's managed $HOME subtree and
// copies every untracked file into the repo, replacing the $HOME original
// with a symlink. It applied no ignore filtering at all — only the Add/Adopt
// path did — so pressing "s" on a package whose managed root contained
// ~/.ssh material copied private keys into a repository that is then
// committed and pushed to the configured remote.
func TestLink_AutoDetectHonoursIgnorePatterns(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	pkgDir := filepath.Join(repoDir, "cfg", ".config", "app")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "settings.toml"), []byte("tracked"), 0644))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	patterns := config.GetDefaultIgnorePatterns()

	// First link establishes the managed root under $HOME.
	_, err := Link(repoDir, homeDir, "cfg", patterns)
	require.NoError(t, err)

	// Secrets land untracked next to the managed file.
	appDir := filepath.Join(homeDir, ".config", "app")
	secrets := []string{".env", "server.pem", "id_rsa"}
	for _, name := range secrets {
		require.NoError(t, os.WriteFile(filepath.Join(appDir, name), []byte("TOPSECRET"), 0600))
	}
	// A non-secret untracked file must still be picked up.
	require.NoError(t, os.WriteFile(filepath.Join(appDir, "extra.toml"), []byte("ok"), 0644))

	result, err := Link(repoDir, homeDir, "cfg", patterns)
	require.NoError(t, err)

	for _, name := range secrets {
		_, statErr := os.Stat(filepath.Join(pkgDir, name))
		assert.True(t, os.IsNotExist(statErr),
			"%s matches an ignore pattern and must never be copied into the repo", name)

		// The $HOME original must be left alone, not replaced by a symlink.
		info, lerr := os.Lstat(filepath.Join(appDir, name))
		require.NoError(t, lerr)
		assert.Zero(t, info.Mode()&os.ModeSymlink,
			"%s must be left untouched in $HOME", name)
	}

	assert.Subset(t, result.Ignored, []string{
		filepath.Join(".config", "app", ".env"),
		filepath.Join(".config", "app", "server.pem"),
		filepath.Join(".config", "app", "id_rsa"),
	}, "skipped files must be reported so the user can see why")

	_, statErr := os.Stat(filepath.Join(pkgDir, "extra.toml"))
	assert.NoError(t, statErr, "a non-ignored untracked file must still be adopted")
}

// TestLink_NilPatternsAdoptsEverything keeps the unfiltered behaviour
// available for callers with no configuration.
func TestLink_NilPatternsAdoptsEverything(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	pkgDir := filepath.Join(repoDir, "cfg", ".config", "app")
	require.NoError(t, os.MkdirAll(pkgDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "settings.toml"), []byte("tracked"), 0644))
	require.NoError(t, os.MkdirAll(homeDir, 0755))

	_, err := Link(repoDir, homeDir, "cfg", nil)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(
		filepath.Join(homeDir, ".config", "app", ".env"), []byte("v"), 0600))

	result, err := Link(repoDir, homeDir, "cfg", nil)
	require.NoError(t, err)

	assert.Empty(t, result.Ignored)
	_, statErr := os.Stat(filepath.Join(pkgDir, ".env"))
	assert.NoError(t, statErr, "with no patterns configured nothing is filtered")
}

// TestAdopt_HonoursIgnorePatterns pins the fix for the path Link explicitly
// defers secrets to.
//
// Link refuses foreign symlinks and tells the user to press 'o' to adopt.
// Adopt then read the resolved target and wrote it into the package with no
// ignore filtering at all — and package directories, unlike logs/ and
// backups/, are not gitignored, so the plaintext was committed and pushed.
func TestAdopt_HonoursIgnorePatterns(t *testing.T) {
	tmp := t.TempDir()
	homeDir := filepath.Join(tmp, "home")
	repoDir := filepath.Join(tmp, "repo")
	secrets := filepath.Join(tmp, "secrets")
	pkgSub := filepath.Join(repoDir, "nvim", ".config", "nvim")

	require.NoError(t, os.MkdirAll(pkgSub, 0755))
	require.NoError(t, os.MkdirAll(secrets, 0755))
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgSub, "init.lua"), []byte("tracked"), 0644))

	patterns := config.GetDefaultIgnorePatterns()

	_, err := Link(repoDir, homeDir, "nvim", patterns)
	require.NoError(t, err)

	// Foreign symlinks in the managed root, pointing at secrets.
	appDir := filepath.Join(homeDir, ".config", "nvim")
	require.NoError(t, os.WriteFile(filepath.Join(secrets, "prod.key"), []byte("PRIVATE KEY"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(secrets, "dotenv"), []byte("TOKEN=abc"), 0600))
	require.NoError(t, os.Symlink(filepath.Join(secrets, "prod.key"), filepath.Join(appDir, "prod.key")))
	require.NoError(t, os.Symlink(filepath.Join(secrets, "dotenv"), filepath.Join(appDir, ".env")))
	// A non-secret foreign symlink must still be adopted.
	require.NoError(t, os.WriteFile(filepath.Join(secrets, "extra.lua"), []byte("ok"), 0644))
	require.NoError(t, os.Symlink(filepath.Join(secrets, "extra.lua"), filepath.Join(appDir, "extra.lua")))

	result, err := Adopt(repoDir, homeDir, "nvim", patterns)
	require.NoError(t, err)

	for _, name := range []string{"prod.key", ".env"} {
		_, statErr := os.Stat(filepath.Join(pkgSub, name))
		assert.True(t, os.IsNotExist(statErr),
			"%s matches an ignore pattern and must never be copied into the repo", name)
	}
	assert.Len(t, result.Ignored, 2, "skipped files must be reported")

	_, statErr := os.Stat(filepath.Join(pkgSub, "extra.lua"))
	assert.NoError(t, statErr, "a non-ignored foreign symlink must still be adopted")
	assert.Equal(t, 1, result.Adopted)
}
