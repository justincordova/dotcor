package fs

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
)

func TestSupportsSymlinks(t *testing.T) {
	supported, err := SupportsSymlinks()
	if err != nil {
		t.Fatalf("SupportsSymlinks() error = %v", err)
	}

	// On Unix systems, should always be supported
	if runtime.GOOS != "windows" && !supported {
		t.Error("SupportsSymlinks() = false on Unix system")
	}
}

func TestIsSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular file
	regularFile := filepath.Join(tempDir, "regular")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	// Create a symlink
	symlinkFile := filepath.Join(tempDir, "symlink")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

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
			got, err := IsSymlink(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsSymlink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create symlink with relative path
	symlinkFile := filepath.Join(tempDir, "symlink")
	if err := os.Symlink("target", symlinkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Read symlink
	got, err := ReadSymlink(symlinkFile)
	if err != nil {
		t.Fatalf("ReadSymlink() error = %v", err)
	}
	if got != "target" {
		t.Errorf("ReadSymlink() = %v, want 'target'", got)
	}
}

func TestIsValidSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create valid symlink
	validSymlink := filepath.Join(tempDir, "valid_symlink")
	if err := os.Symlink(targetFile, validSymlink); err != nil {
		t.Fatalf("failed to create valid symlink: %v", err)
	}

	// Create broken symlink (points to non-existent file)
	brokenSymlink := filepath.Join(tempDir, "broken_symlink")
	if err := os.Symlink(filepath.Join(tempDir, "nonexistent"), brokenSymlink); err != nil {
		t.Fatalf("failed to create broken symlink: %v", err)
	}

	// Create regular file (not a symlink)
	regularFile := filepath.Join(tempDir, "regular")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

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
			got, err := IsValidSymlink(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsValidSymlink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRelativeSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create relative symlink
	relSymlink := filepath.Join(tempDir, "rel_symlink")
	if err := os.Symlink("target", relSymlink); err != nil {
		t.Fatalf("failed to create relative symlink: %v", err)
	}

	// Create absolute symlink
	absSymlink := filepath.Join(tempDir, "abs_symlink")
	if err := os.Symlink(targetFile, absSymlink); err != nil {
		t.Fatalf("failed to create absolute symlink: %v", err)
	}

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
			got, err := IsRelativeSymlink(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsRelativeSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("IsRelativeSymlink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create relative symlink
	symlinkFile := filepath.Join(tempDir, "symlink")
	if err := os.Symlink("target", symlinkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Resolve symlink
	got, err := ResolveSymlink(symlinkFile)
	if err != nil {
		t.Fatalf("ResolveSymlink() error = %v", err)
	}

	// Should resolve to absolute path of target
	if got != targetFile {
		t.Errorf("ResolveSymlink() = %v, want %v", got, targetFile)
	}
}

func TestGetSymlinkStatus(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create valid symlink with relative path
	validSymlink := filepath.Join(tempDir, "valid_symlink")
	if err := os.Symlink("target", validSymlink); err != nil {
		t.Fatalf("failed to create valid symlink: %v", err)
	}

	// Test GetSymlinkStatus
	status, err := GetSymlinkStatus(validSymlink, targetFile)
	if err != nil {
		t.Fatalf("GetSymlinkStatus() error = %v", err)
	}

	if !status.Exists {
		t.Error("GetSymlinkStatus() Exists = false, want true")
	}
	if !status.IsSymlink {
		t.Error("GetSymlinkStatus() IsSymlink = false, want true")
	}
	if !status.TargetExists {
		t.Error("GetSymlinkStatus() TargetExists = false, want true")
	}
	if !status.IsRelative {
		t.Error("GetSymlinkStatus() IsRelative = false, want true")
	}
	if status.ActualTarget != "target" {
		t.Errorf("GetSymlinkStatus() ActualTarget = %v, want 'target'", status.ActualTarget)
	}
}

func TestRemoveSymlink(t *testing.T) {
	// Skip on Windows if symlinks not supported
	supported, _ := SupportsSymlinks()
	if !supported {
		t.Skip("symlinks not supported on this platform")
	}

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create target file
	targetFile := filepath.Join(tempDir, "target")
	if err := os.WriteFile(targetFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create symlink
	symlinkFile := filepath.Join(tempDir, "symlink")
	if err := os.Symlink(targetFile, symlinkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Remove symlink
	cfg := &config.Config{Logger: slog.Default()}
	if err := RemoveSymlink(symlinkFile, cfg); err != nil {
		t.Fatalf("RemoveSymlink() error = %v", err)
	}

	// Verify symlink is gone
	if PathExists(symlinkFile) {
		t.Error("RemoveSymlink() symlink still exists")
	}

	// Verify target still exists
	if !PathExists(targetFile) {
		t.Error("RemoveSymlink() target was removed")
	}
}

func TestRemoveSymlinkErrorsOnRegularFile(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create regular file
	regularFile := filepath.Join(tempDir, "regular")
	if err := os.WriteFile(regularFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create regular file: %v", err)
	}

	// Try to remove regular file as symlink (should error)
	cfg := &config.Config{Logger: slog.Default()}
	err = RemoveSymlink(regularFile, cfg)
	if err == nil {
		t.Error("RemoveSymlink() should error on regular file")
	}

	// Verify regular file still exists
	if !PathExists(regularFile) {
		t.Error("RemoveSymlink() removed regular file")
	}

	// Try to remove as symlink - should fail
	err = RemoveSymlink(regularFile, cfg)
	if err == nil {
		t.Error("RemoveSymlink() should error on regular file")
	}

	// Verify file still exists
	if !PathExists(regularFile) {
		t.Error("RemoveSymlink() removed regular file")
	}
}
