package tests

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/justincordova/dotcor/internal/config"
	"github.com/justincordova/dotcor/internal/core"
	"github.com/justincordova/dotcor/internal/fs"
	"github.com/justincordova/dotcor/internal/git"
)

func testConfig() *config.Config {
	return &config.Config{
		Logger: slog.Default(),
	}
}

// TestIntegration_InitAddListRemove tests the core workflow:
// init -> add a file -> list -> remove -> verify cleanup
func TestIntegration_InitAddListRemove(t *testing.T) {
	// Create temp directory structure
	tempDir, err := os.MkdirTemp("", "dotcor-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	homeDir := filepath.Join(tempDir, "home")
	repoDir := filepath.Join(tempDir, "dotcor")
	filesDir := filepath.Join(repoDir, "files")

	// Create directories
	for _, dir := range []string{homeDir, repoDir, filesDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create %s: %v", dir, err)
		}
	}

	// Create a dotfile in home
	dotfile := filepath.Join(homeDir, ".zshrc")
	dotfileContent := []byte("# zshrc content\nexport PATH=/usr/bin\n")
	if err := os.WriteFile(dotfile, dotfileContent, 0644); err != nil {
		t.Fatalf("failed to create dotfile: %v", err)
	}

	// Initialize git repo (optional but realistic)
	if git.IsGitInstalled() {
		if err := git.InitRepo(repoDir); err != nil {
			t.Fatalf("git init failed: %v", err)
		}
	}

	// Create config
	cfg := &config.Config{
		Logger:         slog.Default(),
		Version:        config.CurrentConfigVersion,
		RepoPath:       filesDir,
		GitEnabled:     false,
		IgnorePatterns: []string{"*.swp", ".DS_Store"},
		ManagedFiles:   []config.ManagedFile{},
	}

	// === ADD OPERATION ===
	// 1. Create backup
	backupPath, err := core.CreateBackup(dotfile, cfg)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("CreateBackup() did not create backup file")
	}

	// 2. Generate repo path for the dotfile (use simple path since we're in temp)
	repoPath := "shell/zshrc"
	fullRepoPath := filepath.Join(filesDir, repoPath)

	// 3. Ensure parent directory exists
	if err := fs.EnsureDir(filepath.Dir(fullRepoPath), cfg); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	// 4. Move file to repo using transaction
	tx := core.NewTransaction(cfg)
	if err := tx.Execute(&core.MoveFileOp{Src: dotfile, Dst: fullRepoPath, Config: cfg}); err != nil {
		t.Fatalf("MoveFileOp.Do() error = %v", err)
	}

	// 5. Create symlink (linkPath, targetPath - where symlink is, what it points to)
	relPath, err := config.ComputeRelativeSymlink(dotfile, fullRepoPath)
	if err != nil {
		t.Fatalf("ComputeRelativeSymlink() error = %v", err)
	}
	if err := os.Symlink(relPath, dotfile); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	// 6. Update config
	cfg.ManagedFiles = append(cfg.ManagedFiles, config.ManagedFile{
		SourcePath: dotfile,
		RepoPath:   repoPath,
	})

	tx.Commit()

	// === VERIFY ADD ===
	// Symlink exists and points to repo
	isSymlink, err := fs.IsSymlink(dotfile)
	if err != nil {
		t.Fatalf("IsSymlink() error = %v", err)
	}
	if !isSymlink {
		t.Error("dotfile should be a symlink after add")
	}

	// Repo file exists with correct content
	content, err := os.ReadFile(fullRepoPath)
	if err != nil {
		t.Fatalf("failed to read repo file: %v", err)
	}
	if string(content) != string(dotfileContent) {
		t.Errorf("repo file content mismatch: got %q, want %q", content, dotfileContent)
	}

	// Symlink is valid
	isValid, err := fs.IsValidSymlink(dotfile)
	if err != nil {
		t.Fatalf("IsValidSymlink() error = %v", err)
	}
	if !isValid {
		t.Error("symlink should be valid")
	}

	// === LIST OPERATION ===
	// Verify managed files count
	if len(cfg.ManagedFiles) != 1 {
		t.Errorf("ManagedFiles count = %d, want 1", len(cfg.ManagedFiles))
	}

	// === REMOVE OPERATION ===
	// 1. Find the managed file
	var managedFile *config.ManagedFile
	for i := range cfg.ManagedFiles {
		if cfg.ManagedFiles[i].SourcePath == dotfile {
			managedFile = &cfg.ManagedFiles[i]
			break
		}
	}
	if managedFile == nil {
		t.Fatal("managed file not found in config")
	}

	// 2. Remove symlink and restore file
	removeTx := core.NewTransaction(cfg)

	// Remove the symlink
	if err := os.Remove(dotfile); err != nil {
		t.Fatalf("Remove symlink error = %v", err)
	}

	// Move file back from repo
	if err := removeTx.Execute(&core.MoveFileOp{Src: fullRepoPath, Dst: dotfile, Config: cfg}); err != nil {
		t.Fatalf("MoveFileOp.Do() (remove) error = %v", err)
	}

	// 3. Update config - remove the managed file
	newManagedFiles := make([]config.ManagedFile, 0)
	for _, mf := range cfg.ManagedFiles {
		if mf.SourcePath != dotfile {
			newManagedFiles = append(newManagedFiles, mf)
		}
	}
	cfg.ManagedFiles = newManagedFiles

	removeTx.Commit()

	// === VERIFY REMOVE ===
	// File exists and is not a symlink
	isSymlink, err = fs.IsSymlink(dotfile)
	if err != nil {
		t.Fatalf("IsSymlink() error = %v", err)
	}
	if isSymlink {
		t.Error("dotfile should not be a symlink after remove")
	}

	// File has correct content
	content, err = os.ReadFile(dotfile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(content) != string(dotfileContent) {
		t.Errorf("restored file content mismatch: got %q, want %q", content, dotfileContent)
	}

	// Config has no managed files
	if len(cfg.ManagedFiles) != 0 {
		t.Errorf("ManagedFiles count after remove = %d, want 0", len(cfg.ManagedFiles))
	}
}

// TestIntegration_TransactionRollback tests that failed operations roll back correctly
func TestIntegration_TransactionRollback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create source files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	dest1 := filepath.Join(tempDir, "dest1.txt")
	dest2 := filepath.Join(tempDir, "dest2.txt")

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	cfg := testConfig()
	tx := core.NewTransaction(cfg)

	// First operation succeeds
	if err := tx.Execute(&core.CopyFileOp{Src: file1, Dst: dest1, Config: cfg}); err != nil {
		t.Fatalf("first CopyFileOp error = %v", err)
	}

	// Verify first copy worked
	if !fs.PathExists(dest1) {
		t.Error("dest1 should exist after first operation")
	}

	// Second operation will fail (try to copy to a path that's a directory)
	if err := os.MkdirAll(dest2, 0755); err != nil {
		t.Fatalf("failed to create dest2 dir: %v", err)
	}

	// This should fail and trigger rollback
	err = tx.Execute(&core.CopyFileOp{Src: file2, Dst: dest2, Config: cfg})
	if err == nil {
		t.Error("copying to directory path should fail")
	}

	// After rollback, dest1 should be removed
	if fs.PathExists(dest1) {
		t.Error("dest1 should be removed after rollback")
	}

	// Original files should still exist
	if !fs.PathExists(file1) {
		t.Error("file1 should still exist after rollback")
	}
	if !fs.PathExists(file2) {
		t.Error("file2 should still exist after rollback")
	}
}

// TestIntegration_SecretDetection tests that secret detection works in the workflow
func TestIntegration_SecretDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-integration-*")
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
			name:        "clean config",
			content:     "# Normal config\nexport PATH=/usr/bin\n",
			wantSecrets: false,
		},
		{
			name:        "contains api key",
			content:     "API_KEY=sk_live_abcdefghijklmnopqrstuvwxyz123\n",
			wantSecrets: true,
		},
		{
			name:        "contains password",
			content:     "db_password=supersecret123\n",
			wantSecrets: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, tt.name+".txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			secrets, err := core.DetectSecrets(testFile)
			if err != nil {
				t.Fatalf("DetectSecrets() error = %v", err)
			}

			hasSecrets := len(secrets) > 0
			if hasSecrets != tt.wantSecrets {
				t.Errorf("DetectSecrets() found secrets = %v, want %v", hasSecrets, tt.wantSecrets)
			}
		})
	}
}

// TestIntegration_IgnorePatterns tests that ignore patterns work correctly
func TestIntegration_IgnorePatterns(t *testing.T) {
	patterns := []string{"*.swp", ".DS_Store", ".env", "*.key"}

	tests := []struct {
		filename    string
		wantIgnored bool
	}{
		{".zshrc.swp", true},
		{".DS_Store", true},
		{".env", true},
		{"secret.key", true},
		{".zshrc", false},
		{".gitconfig", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			ignored, _ := core.ShouldIgnore("/home/user/"+tt.filename, patterns)
			if ignored != tt.wantIgnored {
				t.Errorf("ShouldIgnore(%s) = %v, want %v", tt.filename, ignored, tt.wantIgnored)
			}
		})
	}
}

// TestIntegration_BackupRestore tests backup and restore functionality
func TestIntegration_BackupRestore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dotcor-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create original file
	originalFile := filepath.Join(tempDir, "original.txt")
	originalContent := []byte("original content")
	if err := os.WriteFile(originalFile, originalContent, 0644); err != nil {
		t.Fatalf("failed to create original file: %v", err)
	}

	// Create backup
	cfg := testConfig()
	backupPath, err := core.CreateBackup(originalFile, cfg)
	if err != nil {
		t.Fatalf("CreateBackup() error = %v", err)
	}

	// Verify backup exists with correct content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup: %v", err)
	}
	if string(backupContent) != string(originalContent) {
		t.Errorf("backup content mismatch: got %q, want %q", backupContent, originalContent)
	}

	// Modify original
	modifiedContent := []byte("modified content")
	if err := os.WriteFile(originalFile, modifiedContent, 0644); err != nil {
		t.Fatalf("failed to modify original file: %v", err)
	}

	// Restore from backup
	restoredFile := filepath.Join(tempDir, "restored.txt")
	if err := core.RestoreBackup(backupPath, restoredFile, cfg); err != nil {
		t.Fatalf("RestoreBackup() error = %v", err)
	}

	// Verify restored content matches original
	restoredContent, err := os.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restoredContent) != string(originalContent) {
		t.Errorf("restored content mismatch: got %q, want %q", restoredContent, originalContent)
	}
}

// TestIntegration_GitWorkflow tests git operations in workflow
func TestIntegration_GitWorkflow(t *testing.T) {
	if !git.IsGitInstalled() {
		t.Skip("git not installed")
	}

	tempDir, err := os.MkdirTemp("", "dotcor-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize repo
	if err := git.InitRepo(tempDir); err != nil {
		t.Fatalf("InitRepo() error = %v", err)
	}

	// Configure git user
	configureGitUser(t, tempDir)

	// Initial state: clean, no commits
	hasChanges, err := git.HasChanges(tempDir)
	if err != nil {
		t.Fatalf("HasChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("new repo should have no changes")
	}

	// Create and commit file
	file1 := filepath.Join(tempDir, "dotfile1")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := git.AutoCommit(tempDir, "add dotfile1"); err != nil {
		t.Fatalf("AutoCommit() error = %v", err)
	}

	// Should be clean after commit
	hasChanges, err = git.HasChanges(tempDir)
	if err != nil {
		t.Fatalf("HasChanges() error = %v", err)
	}
	if hasChanges {
		t.Error("repo should be clean after commit")
	}

	// Add another file
	file2 := filepath.Join(tempDir, "dotfile2")
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if err := git.AutoCommit(tempDir, "add dotfile2"); err != nil {
		t.Fatalf("AutoCommit() error = %v", err)
	}

	// Get history
	history, err := git.GetFileHistory(tempDir, "dotfile1", 10)
	if err != nil {
		t.Fatalf("GetFileHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Errorf("GetFileHistory() returned %d commits, want 1", len(history))
	}

	// Modify file
	if err := os.WriteFile(file1, []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Check diff
	diff, err := git.GetDiff(tempDir)
	if err != nil {
		t.Fatalf("GetDiff() error = %v", err)
	}
	if diff == "" {
		t.Error("GetDiff() should return diff for modified file")
	}

	// Get status
	status, err := git.GetStatus(tempDir)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if !status.HasUncommitted {
		t.Error("GetStatus().HasUncommitted should be true")
	}
}

// TestIntegration_PathNormalization tests path handling throughout workflow
func TestIntegration_PathNormalization(t *testing.T) {
	// Test normalization preserves home tilde
	home, _ := os.UserHomeDir()
	testPath := filepath.Join(home, ".zshrc")

	normalized, err := config.NormalizePath(testPath)
	if err != nil {
		t.Fatalf("NormalizePath() error = %v", err)
	}

	if normalized != "~/.zshrc" {
		t.Errorf("NormalizePath() = %q, want %q", normalized, "~/.zshrc")
	}

	// Test expansion
	expanded, err := config.ExpandPath(normalized, nil)
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}

	if expanded != testPath {
		t.Errorf("ExpandPath() = %q, want %q", expanded, testPath)
	}
}

// TestIntegration_ValidatorRepoPath tests repo path validation
func TestIntegration_ValidatorRepoPath(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"shell/zshrc", false},
		{"config/nvim/init.lua", false},
		{"", true},
		{"/absolute/path", true},
		{"../traversal", true},
		{"path/../traversal", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := core.ValidateRepoPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepoPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// TestIntegration_GenerateRepoPath tests automatic repo path generation
func TestIntegration_GenerateRepoPath(t *testing.T) {
	tests := []struct {
		sourcePath string
		customPath string
		wantPrefix string
	}{
		{"~/.zshrc", "", "shell/"},
		{"~/.bashrc", "", "shell/"},
		{"~/.gitconfig", "", "git/"},
		{"~/.vimrc", "", "vim/"},
		{"~/.custom", "custom/myfile", "custom/"},
		{"~/.random", "", "misc/"},
	}

	for _, tt := range tests {
		t.Run(tt.sourcePath, func(t *testing.T) {
			result, err := config.GenerateRepoPath(tt.sourcePath, tt.customPath, nil)
			if err != nil {
				t.Fatalf("GenerateRepoPath() error = %v", err)
			}

			if !filepath.HasPrefix(result, tt.wantPrefix) {
				t.Errorf("GenerateRepoPath(%s) = %q, want prefix %q", tt.sourcePath, result, tt.wantPrefix)
			}
		})
	}
}

// Helper function to configure git user in test repos
func configureGitUser(t *testing.T, repoPath string) {
	t.Helper()

	cmd := exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure git user.email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to configure git user.name: %v", err)
	}
}
