package core

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/stretchr/testify/assert"
)

// createTestConfig creates a test config with logger
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	// Create logger that writes to nowhere for tests
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &config.Config{
		Logger:   logger,
		RepoPath: t.TempDir(),
	}
}

func TestValidateRepoPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "valid relative path",
			path:    "shell/zshrc",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "config/nvim/init.lua",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "absolute path",
			path:    "/shell/zshrc",
			wantErr: true,
		},
		{
			name:    "path traversal",
			path:    "../shell/zshrc",
			wantErr: true,
		},
		{
			name:    "path with internal traversal",
			path:    "shell/../git/gitconfig",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			testPath := tt.path

			// Act
			err := ValidateRepoPath(testPath)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "should return error for invalid path: %s", testPath)
			} else {
				assert.NoError(t, err, "should not return error for valid path: %s", testPath)
			}
		})
	}
}

func TestDetectSecrets(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		content     string
		wantSecrets bool
	}{
		{
			name:        "no secrets",
			content:     "# This is a normal config\nexport PATH=/usr/bin\n",
			wantSecrets: false,
		},
		{
			name:        "api key",
			content:     "API_KEY=mock_api_key_for_testing_purposes_only\n",
			wantSecrets: true,
		},
		{
			name:        "password",
			content:     "password=mysecretpassword123\n",
			wantSecrets: true,
		},
		{
			name:        "private key header",
			content:     "-----BEGIN RSA PRIVATE KEY-----\nMIIEpA...\n-----END RSA PRIVATE KEY-----",
			wantSecrets: true,
		},
		{
			name:        "aws credentials",
			content:     "aws_access_key_id=MOCKAWSACCESSKEYID20\n",
			wantSecrets: true,
		},
		{
			name:        "database url with password",
			content:     "DATABASE_URL=postgres://user:secretpass@localhost/db\n",
			wantSecrets: true,
		},
		{
			name:        "access token",
			content:     "access_token = 'mock_access_token_for_testing_1234567890'\n",
			wantSecrets: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			testFile := filepath.Join(tempDir, tt.name+".txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			// Act
			secrets, err := DetectSecrets(testFile)
			assert.NoError(t, err, "DetectSecrets should not error")

			// Assert
			gotSecrets := len(secrets) > 0
			assert.Equal(t, tt.wantSecrets, gotSecrets,
				"DetectSecrets(%q) found secrets = %v, want %v (secrets: %v)",
				tt.name, gotSecrets, tt.wantSecrets, secrets)
		})
	}
}

func TestValidateNotAlreadyManaged(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Version:  config.CurrentConfigVersion,
		RepoPath: "~/.dotcor/files",
		ManagedFiles: []config.ManagedFile{
			{
				SourcePath: "~/.zshrc",
				RepoPath:   "shell/zshrc",
			},
		},
	}

	tests := []struct {
		name       string
		sourcePath string
		wantErr    bool
	}{
		{
			name:       "unmanaged file",
			sourcePath: "~/.bashrc",
			wantErr:    false,
		},
		{
			name:       "already managed file",
			sourcePath: "~/.zshrc",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			sourcePath := tt.sourcePath

			// Act
			err := ValidateNotAlreadyManaged(cfg, sourcePath)

			// Assert
			if tt.wantErr {
				assert.Error(t, err, "should return error for already managed file: %s", sourcePath)
			} else {
				assert.NoError(t, err, "should not return error for unmanaged file: %s", sourcePath)
			}
		})
	}
}

func TestValidateFileSize(t *testing.T) {
	// Arrange
	tempDir := t.TempDir()
	cfg := createTestConfig(t)
	smallFile := filepath.Join(tempDir, "small")
	if err := os.WriteFile(smallFile, []byte("small content"), 0644); err != nil {
		t.Fatalf("failed to create small file: %v", err)
	}

	// Act
	err := ValidateFileSize(smallFile, cfg)

	// Assert
	assert.NoError(t, err, "ValidateFileSize should accept small file")

	// Note: We don't test large files as creating 100MB+ files is slow
	// The function logic is straightforward
}

func TestShouldWarnAboutSecrets(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		warnings []string
		want     bool
	}{
		{
			name:     "no warnings",
			path:     "~/.zshrc",
			warnings: []string{},
			want:     false,
		},
		{
			name:     "with warnings",
			path:     "~/.env",
			warnings: []string{"Line 1: API_KEY=..."},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			path := tt.path
			warnings := tt.warnings

			// Act
			got := ShouldWarnAboutSecrets(path, warnings)

			// Assert
			assert.Equal(t, tt.want, got, "ShouldWarnAboutSecrets(%q, %v) = %v, want %v",
				path, warnings, got, tt.want)
		})
	}
}

func TestValidateFileSizeEdgeCases(t *testing.T) {
	cfg := createTestConfig(t)

	// Test negative threshold
	cfg.LargeFileThreshold = -100

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// Should treat negative same as zero (disabled)
	err = ValidateFileSize(testFile, cfg)
	if err != nil {
		t.Error("negative threshold should disable validation")
	}
}
