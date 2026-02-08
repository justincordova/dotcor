package core

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
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
			err := ValidateRepoPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepoPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectSecrets(t *testing.T) {
	// Create temp dir
	tempDir, err := os.MkdirTemp("", "dotcor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

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
			// Create test file
			testFile := filepath.Join(tempDir, "testfile")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			secrets, err := DetectSecrets(testFile)
			if err != nil {
				t.Fatalf("DetectSecrets() error = %v", err)
			}

			gotSecrets := len(secrets) > 0
			if gotSecrets != tt.wantSecrets {
				t.Errorf("DetectSecrets() found secrets = %v, want %v (secrets: %v)", gotSecrets, tt.wantSecrets, secrets)
			}
		})
	}
}

func TestValidateNotAlreadyManaged(t *testing.T) {
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
			err := ValidateNotAlreadyManaged(cfg, tt.sourcePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotAlreadyManaged() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileSize(t *testing.T) {
	tempDir := t.TempDir()
	cfg := createTestConfig(t)

	// Create a small file (should pass)
	smallFile := filepath.Join(tempDir, "small")
	if err := os.WriteFile(smallFile, []byte("small content"), 0644); err != nil {
		t.Fatalf("failed to create small file: %v", err)
	}

	// Test small file
	if err := ValidateFileSize(smallFile, cfg); err != nil {
		t.Errorf("ValidateFileSize() error = %v for small file", err)
	}
	defer os.RemoveAll(tempDir)

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
			got := ShouldWarnAboutSecrets(tt.path, tt.warnings)
			if got != tt.want {
				t.Errorf("ShouldWarnAboutSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}
