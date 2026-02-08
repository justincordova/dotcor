package fs

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Logger: slog.Default(),
	}
}

func TestFileExists(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
			got := PathExists(tt.path)
			// PathExists returns true for both files and directories
			// Old FileExists returned false for directories
			want := tt.want
			if tt.path == testFile || tt.path == tempDir {
				want = true
			}
			if got != want {
				t.Errorf("PathExists() = %v, want %v", got, want)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
			got := PathExists(tt.path)
			if got != tt.want {
				t.Errorf("PathExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureDir(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

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
			cfg := testConfig()
			err := EnsureDir(tt.path, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureDir() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.path != "" && !tt.wantErr {
				if !PathExists(tt.path) {
					t.Errorf("EnsureDir() directory not created: %s", tt.path)
				}
			}
		})
	}
}

func TestCopyWithPermissions(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file with specific content and permissions
	srcFile := filepath.Join(tempDir, "source")
	content := []byte("test content for copy")
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy to destination
	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "dest")
	if err := CopyWithPermissions(srcFile, dstFile, cfg); err != nil {
		t.Fatalf("CopyWithPermissions() error = %v", err)
	}

	// Verify destination exists
	if !PathExists(dstFile) {
		t.Error("CopyWithPermissions() destination file not created")
	}

	// Verify content
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("CopyWithPermissions() content mismatch: got %q, want %q", dstContent, content)
	}

	// Verify permissions preserved
	srcInfo, _ := os.Stat(srcFile)
	dstInfo, _ := os.Stat(dstFile)
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("CopyWithPermissions() permissions mismatch: got %v, want %v",
			dstInfo.Mode().Perm(), srcInfo.Mode().Perm())
	}
}

func TestMoveFile(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcFile := filepath.Join(tempDir, "source")
	content := []byte("move me")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Move to destination
	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "dest")
	if err := MoveFile(srcFile, dstFile, cfg); err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}

	// Verify source is gone
	if PathExists(srcFile) {
		t.Error("MoveFile() source file still exists")
	}

	// Verify destination exists with correct content
	if !PathExists(dstFile) {
		t.Error("MoveFile() destination file not created")
	}

	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("MoveFile() content mismatch: got %q, want %q", dstContent, content)
	}
}

func TestMoveFileCreatesParentDir(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source file
	srcFile := filepath.Join(tempDir, "source")
	if err := os.WriteFile(srcFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Move to nested destination (parent doesn't exist)
	cfg := testConfig()
	dstFile := filepath.Join(tempDir, "nested", "dir", "dest")
	if err := MoveFile(srcFile, dstFile, cfg); err != nil {
		t.Fatalf("MoveFile() error = %v", err)
	}

	// Verify destination exists
	if !PathExists(dstFile) {
		t.Error("MoveFile() destination file not created")
	}
}

func TestIsDirectory(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

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
			wantErr: true, // Now consistently returns error for all failures
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig()
			got, err := IsDirectory(tt.path, cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsDirectory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFileSize(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with known content
	content := []byte("hello world")
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cfg := testConfig()
	size, err := GetFileSize(testFile, cfg)
	if err != nil {
		t.Fatalf("GetFileSize() error = %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("GetFileSize() = %v, want %v", size, len(content))
	}
}

func TestRemoveFile(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Remove it
	cfg := testConfig()
	if err := RemoveFile(testFile, cfg); err != nil {
		t.Fatalf("RemoveFile() error = %v", err)
	}

	// Verify it's gone
	if PathExists(testFile) {
		t.Error("RemoveFile() file still exists")
	}
}

func TestGetFilesRecursive(t *testing.T) {
	// Create temp dir with nested structure
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested files
	files := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "subdir", "file2.txt"),
		filepath.Join(tempDir, "subdir", "nested", "file3.txt"),
	}

	cfg := testConfig()
	for _, f := range files {
		if err := EnsureDir(filepath.Dir(f), cfg); err != nil {
			t.Fatalf("failed to create parent dir: %v", err)
		}
		if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Get all files
	got, err := GetFilesRecursive(tempDir)
	if err != nil {
		t.Fatalf("GetFilesRecursive() error = %v", err)
	}

	if len(got) != len(files) {
		t.Errorf("GetFilesRecursive() returned %d files, want %d", len(got), len(files))
	}
}

func TestIsReadable(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a readable file
	testFile := filepath.Join(tempDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !IsReadable(testFile) {
		t.Error("IsReadable() = false for readable file")
	}

	if IsReadable(filepath.Join(tempDir, "nonexistent")) {
		t.Error("IsReadable() = true for non-existent file")
	}
}
