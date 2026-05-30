package fs

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/justincordova/dotcor/internal/config"
)

func TestIsSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	regularFile := filepath.Join(tempDir, "regular")
	err = os.WriteFile(regularFile, []byte("test"), 0644)
	require.NoError(t, err)

	symlinkFile := filepath.Join(tempDir, "symlink")
	err = os.Symlink(regularFile, symlinkFile)
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "regular file",
			path: regularFile,
			want: false,
		},
		{
			name: "symlink",
			path: symlinkFile,
			want: true,
		},
		{
			name: "non-existent",
			path: filepath.Join(tempDir, "nonexistent"),
			want: false,
		},
		{
			name: "directory",
			path: tempDir,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := IsSymlink(tt.path)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	symlinkFile := filepath.Join(tempDir, "symlink")
	err = os.Symlink("target", symlinkFile)
	require.NoError(t, err)

	// Act
	got, err := ReadSymlink(symlinkFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "target", got)
}

func TestIsValidSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	validSymlink := filepath.Join(tempDir, "valid_symlink")
	err = os.Symlink(targetFile, validSymlink)
	require.NoError(t, err)

	brokenSymlink := filepath.Join(tempDir, "broken_symlink")
	err = os.Symlink(filepath.Join(tempDir, "nonexistent"), brokenSymlink)
	require.NoError(t, err)

	regularFile := filepath.Join(tempDir, "regular")
	err = os.WriteFile(regularFile, []byte("test"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "valid symlink",
			path: validSymlink,
			want: true,
		},
		{
			name: "broken symlink",
			path: brokenSymlink,
			want: false,
		},
		{
			name: "regular file",
			path: regularFile,
			want: false,
		},
		{
			name: "non-existent path",
			path: filepath.Join(tempDir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := IsValidSymlink(tt.path)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRelativeSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	relSymlink := filepath.Join(tempDir, "rel_symlink")
	err = os.Symlink("target", relSymlink)
	require.NoError(t, err)

	absSymlink := filepath.Join(tempDir, "abs_symlink")
	err = os.Symlink(targetFile, absSymlink)
	require.NoError(t, err)

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "relative symlink",
			path: relSymlink,
			want: true,
		},
		{
			name: "absolute symlink",
			path: absSymlink,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := IsRelativeSymlink(tt.path)

			// Assert
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	symlinkFile := filepath.Join(tempDir, "symlink")
	err = os.Symlink("target", symlinkFile)
	require.NoError(t, err)

	// Act
	got, err := ResolveSymlink(symlinkFile)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, targetFile, got)
}

func TestGetSymlinkStatus(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	validSymlink := filepath.Join(tempDir, "valid_symlink")
	err = os.Symlink("target", validSymlink)
	require.NoError(t, err)

	// Act
	status, err := GetSymlinkStatus(validSymlink, targetFile)

	// Assert
	require.NoError(t, err)
	assert.True(t, status.Exists)
	assert.True(t, status.IsSymlink)
	assert.True(t, status.TargetExists)
	assert.True(t, status.IsRelative)
	assert.Equal(t, "target", status.ActualTarget)
}

func TestRemoveSymlink(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	targetFile := filepath.Join(tempDir, "target")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err)

	symlinkFile := filepath.Join(tempDir, "symlink")
	err = os.Symlink(targetFile, symlinkFile)
	require.NoError(t, err)

	cfg := &config.Config{Logger: slog.Default()}

	// Act
	err = RemoveSymlink(symlinkFile, cfg)

	// Assert
	require.NoError(t, err)
	assert.False(t, PathExists(symlinkFile), "RemoveSymlink() symlink should not exist")
	assert.True(t, PathExists(targetFile), "RemoveSymlink() target should still exist")
}

func TestRemoveSymlinkErrorsOnRegularFile(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	regularFile := filepath.Join(tempDir, "regular")
	err = os.WriteFile(regularFile, []byte("content"), 0644)
	require.NoError(t, err)

	cfg := &config.Config{Logger: slog.Default()}

	// Act
	err = RemoveSymlink(regularFile, cfg)

	// Assert
	assert.Error(t, err, "RemoveSymlink() should error on regular file")
	assert.True(t, PathExists(regularFile), "RemoveSymlink() should not remove regular file")

	// Act - second attempt
	err = RemoveSymlink(regularFile, cfg)

	// Assert
	assert.Error(t, err, "RemoveSymlink() should error on regular file")
	assert.True(t, PathExists(regularFile), "RemoveSymlink() should not remove regular file")
}

func TestSymlinkPointsToRepo_ChecksRepositoryMembership(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	repoDir := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err, "failed to create repo dir")

	targetFile := filepath.Join(repoDir, "file.txt")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create target file")

	symlinkFile := filepath.Join(tempDir, "symlink")
	relPath, _ := filepath.Rel(filepath.Dir(symlinkFile), targetFile)
	err = os.Symlink(relPath, symlinkFile)
	require.NoError(t, err, "failed to create symlink")

	// Act
	pointsToRepo, err := SymlinkPointsToRepo(symlinkFile, repoDir)

	// Assert
	require.NoError(t, err, "SymlinkPointsToRepo() should not error")
	assert.True(t, pointsToRepo, "SymlinkPointsToRepo() should return true when symlink points to repo")

	nonRepoDir := filepath.Join(tempDir, "other")
	pointsToRepo, err = SymlinkPointsToRepo(symlinkFile, nonRepoDir)
	require.NoError(t, err, "SymlinkPointsToRepo() should not error")
	assert.False(t, pointsToRepo, "SymlinkPointsToRepo() should return false when symlink doesn't point to repo")
}

func TestCreateSymlink_RelativePath_ComputedCorrectly(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	repoDir := filepath.Join(tempDir, "repo")
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err, "failed to create repo dir")

	targetFile := filepath.Join(repoDir, "file.txt")
	err = os.WriteFile(targetFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create target file")

	homeDir := filepath.Join(tempDir, "home")
	err = os.MkdirAll(homeDir, 0755)
	require.NoError(t, err, "failed to create home dir")

	linkFile := filepath.Join(homeDir, "link.txt")

	cfg := &config.Config{Logger: slog.Default()}

	// Act
	err = CreateSymlink(targetFile, linkFile, cfg)

	// Assert
	require.NoError(t, err, "CreateSymlink() should not error")

	isLink, err := IsSymlink(linkFile)
	require.NoError(t, err, "IsSymlink() should not error")
	assert.True(t, isLink, "CreateSymlink() should create a symlink")

	target, err := ReadSymlink(linkFile)
	require.NoError(t, err, "ReadSymlink() should not error")
	assert.False(t, filepath.IsAbs(target), "CreateSymlink() should create relative symlink")
}

// TestCreateSymlink_RecoversFromStaleTempLink verifies that a leftover
// "<link>.tmp" symlink (from a prior crash between symlink and rename)
// does not permanently break CreateSymlink. The function must clear the
// stale temp link and succeed.
func TestCreateSymlink_RecoversFromStaleTempLink(t *testing.T) {
	tempDir := t.TempDir()

	repoDir := filepath.Join(tempDir, "repo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	targetFile := filepath.Join(repoDir, "file.txt")
	require.NoError(t, os.WriteFile(targetFile, []byte("test"), 0644))

	homeDir := filepath.Join(tempDir, "home")
	require.NoError(t, os.MkdirAll(homeDir, 0755))
	linkFile := filepath.Join(homeDir, "link.txt")

	// Simulate a crash that left a stale temp symlink behind.
	require.NoError(t, os.Symlink("garbage-target", linkFile+".tmp"))

	cfg := &config.Config{Logger: slog.Default()}

	err := CreateSymlink(targetFile, linkFile, cfg)
	require.NoError(t, err, "CreateSymlink should recover from a stale temp link")

	isLink, err := IsSymlink(linkFile)
	require.NoError(t, err)
	assert.True(t, isLink, "the real symlink should be created")

	// The temp link should have been consumed by the rename, not linger.
	_, statErr := os.Lstat(linkFile + ".tmp")
	assert.True(t, os.IsNotExist(statErr), "stale temp link should no longer exist")
}

func TestSymlinkWithFlatPaths(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	dotcorDir := filepath.Join(tempDir, "dotcor")
	filesDir := filepath.Join(dotcorDir, "files")

	if err := os.MkdirAll(filepath.Join(filesDir, ".config/nvim"), 0755); err != nil {
		t.Fatalf("failed to create files dir: %v", err)
	}

	homeDirAbs, err := filepath.Abs(homeDir)
	if err != nil {
		t.Fatalf("failed to get abs path: %v", err)
	}

	// Source file in files directory
	repoFile := filepath.Join(filesDir, ".config/nvim/init.vim")
	if err := os.WriteFile(repoFile, []byte("nvim config"), 0644); err != nil {
		t.Fatalf("failed to create repo file: %v", err)
	}

	// Where symlink should be
	symlinkPath := filepath.Join(homeDirAbs, ".config/nvim/init.vim")

	// Act - Create symlink
	cfg := &config.Config{Logger: slog.Default()}
	err = CreateSymlink(repoFile, symlinkPath, cfg)

	// Assert
	assert.NoError(t, err, "CreateSymlink should succeed")

	// Verify symlink is created
	isLink, err := IsSymlink(symlinkPath)
	require.NoError(t, err, "IsSymlink should not error")
	assert.True(t, isLink, "CreateSymlink should create a symlink")

	// Verify symlink points to correct target using exact path
	linkTarget, err := ReadSymlink(symlinkPath)
	assert.NoError(t, err, "should be able to read symlink target")
	assert.False(t, filepath.IsAbs(linkTarget), "CreateSymlink should create relative symlink")

	// Verify the relative path is correct
	expectedRelPath, err := filepath.Rel(filepath.Dir(symlinkPath), repoFile)
	require.NoError(t, err, "should be able to compute relative path")
	assert.Equal(t, expectedRelPath, linkTarget, "symlink target should match expected relative path")
}
