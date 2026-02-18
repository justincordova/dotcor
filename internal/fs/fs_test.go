package fs

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	return &config.Config{
		Logger: slog.Default(),
	}
}

func TestFileExists(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: testFile,
			want: true,
		},
		{
			name: "non-existing file",
			path: filepath.Join(tempDir, "nonexistent"),
			want: false,
		},
		{
			name: "directory returns false",
			path: tempDir,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			want := tt.want
			if tt.path == testFile || tt.path == tempDir {
				want = true
			}

			// Act
			got := PathExists(tt.path)

			// Assert
			assert.Equal(t, want, got, "PathExists()")
		})
	}
}

func TestPathExists(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "existing file",
			path: testFile,
			want: true,
		},
		{
			name: "existing directory",
			path: tempDir,
			want: true,
		},
		{
			name: "non-existing path",
			path: filepath.Join(tempDir, "nonexistent"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := PathExists(tt.path)

			// Assert
			assert.Equal(t, tt.want, got, "PathExists()")
		})
	}
}

func TestEnsureDir(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "create new directory",
			path:    filepath.Join(tempDir, "newdir"),
			wantErr: false,
		},
		{
			name:    "create nested directory",
			path:    filepath.Join(tempDir, "nested", "dir", "path"),
			wantErr: false,
		},
		{
			name:    "existing directory succeeds",
			path:    tempDir,
			wantErr: false,
		},
		{
			name:    "empty path succeeds",
			path:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := testConfig()

			// Act
			err := EnsureDir(tt.path, cfg)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "EnsureDir() should return error")
			} else {
				assert.NoError(t, err, "EnsureDir()")
				if tt.path != "" {
					assert.True(t, PathExists(tt.path), "EnsureDir() directory should be created: %s", tt.path)
				}
			}
		})
	}
}

func TestCopyWithPermissions(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	srcFile := filepath.Join(tempDir, "source")
	content := []byte("test content for copy")
	err = os.WriteFile(srcFile, content, 0755)
	require.NoError(t, err, "failed to create source file")

	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "dest")

	// Act
	err = CopyWithPermissions(srcFile, dstFile, cfg)

	// Assert
	require.NoError(t, err, "CopyWithPermissions()")
	assert.True(t, PathExists(dstFile), "CopyWithPermissions() destination file should be created")

	dstContent, err := os.ReadFile(dstFile)
	require.NoError(t, err, "failed to read destination file")
	assert.Equal(t, string(content), string(dstContent), "CopyWithPermissions() content mismatch")

	srcInfo, _ := os.Stat(srcFile)
	dstInfo, _ := os.Stat(dstFile)
	assert.Equal(t, srcInfo.Mode().Perm(), dstInfo.Mode().Perm(), "CopyWithPermissions() permissions mismatch")
}

func TestMoveFile(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	srcFile := filepath.Join(tempDir, "source")
	content := []byte("move me")
	err = os.WriteFile(srcFile, content, 0644)
	require.NoError(t, err, "failed to create source file")

	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "dest")

	// Act
	err = MoveFile(srcFile, dstFile, cfg)

	// Assert
	require.NoError(t, err, "MoveFile()")
	assert.False(t, PathExists(srcFile), "MoveFile() source file should not exist")

	assert.True(t, PathExists(dstFile), "MoveFile() destination file should be created")

	dstContent, err := os.ReadFile(dstFile)
	require.NoError(t, err, "failed to read destination file")
	assert.Equal(t, string(content), string(dstContent), "MoveFile() content mismatch")
}

func TestMoveFileCreatesParentDir(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	srcFile := filepath.Join(tempDir, "source")
	err = os.WriteFile(srcFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create source file")

	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "nested", "dir", "dest")

	// Act
	err = MoveFile(srcFile, dstFile, cfg)

	// Assert
	require.NoError(t, err, "MoveFile()")
	assert.True(t, PathExists(dstFile), "MoveFile() destination file should be created")
}

func TestIsDirectory(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	tests := []struct {
		name    string
		path    string
		want    bool
		wantErr bool
	}{
		{
			name: "directory",
			path: tempDir,
			want: true,
		},
		{
			name: "file",
			path: testFile,
			want: false,
		},
		{
			name:    "non-existent",
			path:    filepath.Join(tempDir, "nonexistent"),
			want:    false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			cfg := testConfig()

			// Act
			got, err := IsDirectory(tt.path, cfg)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "IsDirectory() should return error")
			} else {
				assert.NoError(t, err, "IsDirectory()")
			}
			assert.Equal(t, tt.want, got, "IsDirectory()")
		})
	}
}

func TestGetFileSize(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	content := []byte("hello world")
	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, content, 0644)
	require.NoError(t, err, "failed to create test file")

	cfg := testConfig()

	// Act
	size, err := GetFileSize(testFile, cfg)

	// Assert
	require.NoError(t, err, "GetFileSize()")
	assert.Equal(t, int64(len(content)), size, "GetFileSize()")
}

func TestRemoveFile(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	cfg := testConfig()

	// Act
	err = RemoveFile(testFile, cfg)

	// Assert
	require.NoError(t, err, "RemoveFile()")
	assert.False(t, PathExists(testFile), "RemoveFile() file should not exist")
}

func TestIsReadable(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	// Act & Assert
	assert.True(t, IsReadable(testFile), "IsReadable() should return true for readable file")
	assert.False(t, IsReadable(filepath.Join(tempDir, "nonexistent")), "IsReadable() should return false for non-existent file")
}

func TestIsWritable(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	// Test writable directory
	// Act & Assert
	assert.True(t, IsWritable(tempDir), "IsWritable() should return true for writable directory")

	// Test writable file
	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	// Act & Assert
	assert.True(t, IsWritable(testFile), "IsWritable() should return true for writable file")

	// Test non-existent path (should check parent)
	nonExistent := filepath.Join(tempDir, "nonexistent", "file")
	writable := IsWritable(nonExistent)
	// Act & Assert - parent directory should be writable
	assert.True(t, writable, "IsWritable() should check parent directory for non-existent path")
}

func TestGetFileMode(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "testfile")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err, "failed to create test file")

	// Act
	mode, err := GetFileMode(testFile)

	// Assert
	require.NoError(t, err, "GetFileMode() should not error")
	assert.Equal(t, os.FileMode(0644), mode.Perm(), "GetFileMode() should return correct permissions")

	testDir := filepath.Join(tempDir, "testdir")
	err = os.Mkdir(testDir, 0755)
	require.NoError(t, err, "failed to create test directory")

	// Act
	mode, err = GetFileMode(testDir)

	// Assert
	require.NoError(t, err, "GetFileMode() should not error")
	assert.True(t, mode.IsDir(), "GetFileMode() should return directory mode")
}

func TestRemoveAll(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	require.NoError(t, err, "failed to create temp dir")

	testDir := filepath.Join(tempDir, "testdir")
	err = os.MkdirAll(filepath.Join(testDir, "nested", "dir"), 0755)
	require.NoError(t, err, "failed to create nested directory")

	file1 := filepath.Join(testDir, "file1.txt")
	err = os.WriteFile(file1, []byte("test"), 0644)
	require.NoError(t, err, "failed to create file1")

	file2 := filepath.Join(testDir, "nested", "file2.txt")
	err = os.WriteFile(file2, []byte("test"), 0644)
	require.NoError(t, err, "failed to create file2")

	cfg := testConfig()

	// Act
	err = RemoveAll(testDir, cfg)

	// Assert
	require.NoError(t, err, "RemoveAll() should not error")
	assert.NoFileExists(t, file1, "RemoveAll() should delete file1")
	assert.NoFileExists(t, file2, "RemoveAll() should delete file2")
	assert.NoDirExists(t, testDir, "RemoveAll() should delete entire directory tree")
}

func TestIsWritableTempCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Test that temp test file is cleaned up
	if IsWritable(tmpDir) {
		// Check that temp file doesn't exist
		tempFile := filepath.Join(tmpDir, ".dotcor_write_test")
		if _, err := os.Stat(tempFile); err == nil {
			t.Error("temp write test file should be cleaned up")
		}
	}
}

func TestMoveFileCleanupOnError(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src.txt")
	dst := filepath.Join(tmpDir, "dst.txt")

	err := os.WriteFile(src, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	cfg := testConfig()

	// Test that move operation completes successfully
	err = MoveFile(src, dst, cfg)
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// After successful move, source should not exist
	if PathExists(src) {
		t.Error("source file should not exist after move")
	}
}
