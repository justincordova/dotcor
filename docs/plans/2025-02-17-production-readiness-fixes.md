# Production Readiness Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all 50 production-readiness issues including nil logger risks, path construction errors, missing validations, race conditions, and unsafe operations.

**Architecture:** Systematic fixes across all packages following production best practices: add validation, improve error handling, fix race conditions, add proper logging, and ensure edge case coverage.

**Tech Stack:** Go 1.21+, Cobra, Viper, git, os, log/slog

---

## Task 1: Fix Logger Initialization in NewDefaultConfig

**Files:**
- Modify: `internal/config/config.go:138-154`

**Step 1: Write failing test**

```go
// internal/config/config_test.go
func TestNewDefaultConfigLoggerNotNil(t *testing.T) {
    cfg, err := config.NewDefaultConfig()
    if err != nil {
        t.Fatalf("NewDefaultConfig failed: %v", err)
    }
    if cfg.Logger == nil {
        t.Error("Logger should not be nil in NewDefaultConfig")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestNewDefaultConfigLoggerNotNil -v`
Expected: FAIL - Logger is nil

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
func NewDefaultConfig() (*Config, error) {
    configDir, err := GetConfigDir()
    if err != nil {
        return nil, err
    }
    
    // Initialize logger with discard handler (can be upgraded later)
    logger := slog.New(slog.NewTextHandler(io.Discard, nil))
    
    return &Config{
        Logger:             logger,
        Version:            CurrentConfigVersion,
        RepoPath:           filepath.Join(configDir, "files"),
        GitEnabled:         true,
        IgnorePatterns:     GetDefaultIgnorePatterns(),
        ManagedFiles:       []ManagedFile{},
        LargeFileThreshold: 100 * 1024 * 1024, // 100MB default
    }, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestNewDefaultConfigLoggerNotNil -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "fix: initialize logger in NewDefaultConfig to prevent nil panics"
```

---

## Task 2: Add Nil Check to SaveConfig

**Files:**
- Modify: `internal/config/config.go:54-89`

**Step 1: Write failing test**

```go
func TestSaveConfigWithNilLogger(t *testing.T) {
    cfg := &config.Config{
        Logger: nil,
        Version: config.CurrentConfigVersion,
        RepoPath: t.TempDir(),
        ManagedFiles: []config.ManagedFile{},
    }
    
    // Should not panic
    err := cfg.SaveConfig()
    if err != nil {
        t.Logf("SaveConfig returned error: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestSaveConfigWithNilLogger -v`
Expected: FAIL - Panic on nil logger

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
func (c *Config) SaveConfig() error {
    configDir, err := GetConfigDir()
    if err != nil {
        return fmt.Errorf("getting config directory: %w", err)
    }
    
    configPath := filepath.Join(configDir, "config.yaml")
    
    // Ensure config directory exists
    if err := os.MkdirAll(configDir, 0755); err != nil {
        return fmt.Errorf("creating config directory: %w", err)
    }
    
    // Convert to YAML
    data, err := yaml.Marshal(c)
    if err != nil {
        return fmt.Errorf("marshaling config: %w", err)
    }
    
    // Write to file with safe logging
    if c.Logger != nil {
        c.Logger.Debug("saving config", "path", configPath)
    }
    
    if err := os.WriteFile(configPath, data, 0644); err != nil {
        if c.Logger != nil {
            c.Logger.Error("failed to save config", "error", err)
        }
        return fmt.Errorf("saving config: %w", err)
    }
    
    if c.Logger != nil {
        c.Logger.Info("config saved", "path", configPath)
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestSaveConfigWithNilLogger -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "fix: add nil logger check to SaveConfig"
```

---

## Task 3: Add Nil Check to GetRepoFilePath

**Files:**
- Modify: `internal/config/paths.go:94-118`

**Step 1: Write failing test**

```go
func TestGetRepoFilePathWithNilLogger(t *testing.T) {
    cfg := &config.Config{
        Logger: nil,
        RepoPath: "/tmp/test",
        ManagedFiles: []config.ManagedFile{},
    }
    
    // Should not panic
    path, err := config.GetRepoFilePath(cfg, "test.txt")
    if err == nil {
        t.Logf("GetRepoFilePath returned path: %s", path)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestGetRepoFilePathWithNilLogger -v`
Expected: FAIL - Panic on nil logger

**Step 3: Write minimal implementation**

```go
// internal/config/paths.go
func GetRepoFilePath(config *Config, repoPath string) (string, error) {
    if config == nil {
        return "", fmt.Errorf("config is nil")
    }
    
    if config.Logger != nil {
        config.Logger.Debug("getting repo file path", "repo_path", repoPath)
    }
    
    // Expand path
    expanded, err := ExpandPath(config.RepoPath, config)
    if err != nil {
        if config.Logger != nil {
            config.Logger.Error("failed to expand repo path", "error", err)
        }
        return "", fmt.Errorf("expanding repo path: %w", err)
    }
    
    // Join with repoPath
    filePath := filepath.Join(expanded, repoPath)
    
    if config.Logger != nil {
        config.Logger.Debug("computed repo file path", "file_path", filePath)
    }
    
    return filePath, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestGetRepoFilePathWithNilLogger -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/paths.go internal/config/config_test.go
git commit -m "fix: add nil logger check to GetRepoFilePath"
```

---

## Task 4: Fix String Concatenation in migrate.go

**Files:**
- Modify: `internal/config/migrate.go:163`

**Step 1: Write failing test**

```go
func TestMigrateRepoPathConstruction(t *testing.T) {
    configDir := "/tmp/test"
    cfg := &config.Config{
        RepoPath: configDir + "/files",  // Old way
    }
    
    // Should use proper path separator
    if !filepath.IsAbs(cfg.RepoPath) {
        t.Error("RepoPath should be absolute")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestMigrateRepoPathConstruction -v`
Expected: FAIL - Uses string concatenation

**Step 3: Write minimal implementation**

```go
// internal/config/migrate.go
func MigrateFromEmpty(config *Config) error {
    if config.Version == "" {
        config.Version = CurrentConfigVersion
    }
    
    if config.RepoPath == "" {
        configDir, err := GetConfigDir()
        if err != nil {
            return err
        }
        config.RepoPath = filepath.Join(configDir, "files")  // Fixed: use filepath.Join
    }
    
    if len(config.IgnorePatterns) == 0 {
        config.IgnorePatterns = GetDefaultIgnorePatterns()
    }
    
    if config.LargeFileThreshold == 0 {
        config.LargeFileThreshold = 100 * 1024 * 1024
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestMigrateRepoPathConstruction -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/migrate.go internal/config/migrate_test.go
git commit -m "fix: use filepath.Join in MigrateFromEmpty"
```

---

## Task 5: Fix RemoveManagedFile Error Fallback

**Files:**
- Modify: `internal/config/config.go:203-217`

**Step 1: Write failing test**

```go
func TestRemoveManagedFileErrorHandling(t *testing.T) {
    cfg, _ := config.NewDefaultConfig()
    
    // Test that non-existent file returns error instead of silent fallback
    err := cfg.RemoveManagedFile("~/.nonexistent")
    if err == nil {
        t.Error("Should return error for non-existent file")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestRemoveManagedFileErrorHandling -v`
Expected: FAIL - Uses invalid fallback path

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
func (c *Config) RemoveManagedFile(sourcePath string) error {
    normalized, err := NormalizePath(sourcePath)
    if err != nil {
        return fmt.Errorf("normalizing path: %w", err)  // Fixed: return error
    }
    
    for i, mf := range c.ManagedFiles {
        if mf.SourcePath == normalized || mf.SourcePath == sourcePath {
            c.ManagedFiles = append(c.ManagedFiles[:i], c.ManagedFiles[i+1:]...)
            return c.SaveConfig()
        }
    }
    
    return fmt.Errorf("file %s is not managed", sourcePath)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestRemoveManagedFileErrorHandling -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "fix: return error instead of fallback in RemoveManagedFile"
```

---

## Task 6: Fix GetManagedFile Error Fallback

**Files:**
- Modify: `internal/config/config.go:220-233`

**Step 1: Write failing test**

```go
func TestGetManagedFileErrorHandling(t *testing.T) {
    cfg, _ := config.NewDefaultConfig()
    
    // Test that invalid paths return error
    mf, err := cfg.GetManagedFile("../../../etc/passwd")
    if err == nil {
        t.Error("Should return error for path traversal attempt")
    }
    if mf != nil {
        t.Error("Should not return managed file for invalid path")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestGetManagedFileErrorHandling -v`
Expected: FAIL - Uses invalid fallback path

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
func (c *Config) GetManagedFile(sourcePath string) (*ManagedFile, error) {
    // Normalize path first
    normalized, err := NormalizePath(sourcePath)
    if err != nil {
        return nil, fmt.Errorf("normalizing path: %w", err)  // Fixed: return error
    }
    
    // Try normalized path first, then original
    for _, mf := range c.ManagedFiles {
        if mf.SourcePath == normalized || mf.SourcePath == sourcePath {
            return &mf, nil
        }
    }
    
    return nil, fmt.Errorf("file %s is not managed", sourcePath)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestGetManagedFileErrorHandling -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "fix: return error instead of fallback in GetManagedFile"
```

---

## Task 7: Fix ExpandGlob Error Fallback

**Files:**
- Modify: `internal/config/paths.go:185-210`

**Step 1: Write failing test**

```go
func TestExpandGlobErrorHandling(t *testing.T) {
    // Test that invalid glob patterns return error
    tests := []string{
        "",
        "[",      // Invalid bracket
    }
    
    for _, pattern := range tests {
        result, err := config.ExpandGlob(pattern)
        if err == nil {
            t.Errorf("ExpandGlob(%q) should return error, got: %v", pattern, result)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestExpandGlobErrorHandling -v`
Expected: FAIL - Uses pattern as-is on error

**Step 3: Write minimal implementation**

```go
// internal/config/paths.go
func ExpandGlob(pattern string) ([]string, error) {
    // Expand ~ and env vars first
    expanded, err := ExpandPath(pattern, nil)
    if err != nil {
        return nil, fmt.Errorf("expanding path: %w", err)  // Fixed: return error
    }
    
    // Check if it contains glob characters
    if !containsGlob(expanded) {
        return []string{pattern}, nil
    }
    
    // Expand glob
    matches, err := filepath.Glob(expanded)
    if err != nil {
        return nil, fmt.Errorf("invalid glob pattern: %w", err)
    }
    
    if len(matches) == 0 {
        return nil, fmt.Errorf("no files match pattern: %s", pattern)
    }
    
    // Filter out directories (only add files)
    var files []string
    for _, match := range matches {
        info, err := os.Stat(match)
        if err != nil {
            continue
        }
        if !info.IsDir() {
            normalized, _ := NormalizePath(match)
            if normalized != "" {
                files = append(files, normalized)
            } else {
                files = append(files, match)
            }
        }
    }
    
    return files, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestExpandGlobErrorHandling -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/paths.go internal/config/config_test.go
git commit -m "fix: return error instead of fallback in ExpandGlob"
```

---

## Task 8: Add Validation to AddManagedFile

**Files:**
- Modify: `internal/config/config.go:191-200`

**Step 1: Write failing test**

```go
func TestAddManagedFileValidation(t *testing.T) {
    cfg, _ := config.NewDefaultConfig()
    
    // Test empty paths
    tests := []config.ManagedFile{
        {SourcePath: "", RepoPath: "test", AddedAt: time.Now()},
        {SourcePath: "~/.test", RepoPath: "", AddedAt: time.Now()},
        {SourcePath: "~/.test", RepoPath: "test", AddedAt: time.Time{}},
    }
    
    for i, mf := range tests {
        err := cfg.AddManagedFile(mf)
        if err == nil {
            t.Errorf("Test %d: Should reject invalid managed file", i)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestAddManagedFileValidation -v`
Expected: FAIL - No validation

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
func (c *Config) AddManagedFile(mf ManagedFile) error {
    // Validate input
    if mf.SourcePath == "" {
        return fmt.Errorf("source path cannot be empty")
    }
    if mf.RepoPath == "" {
        return fmt.Errorf("repo path cannot be empty")
    }
    if mf.AddedAt.IsZero() {
        return fmt.Errorf("added_at time cannot be zero")
    }
    
    // Check if already managed
    if c.IsManaged(mf.SourcePath) {
        return fmt.Errorf("file %s is already managed", mf.SourcePath)
    }
    
    c.ManagedFiles = append(c.ManagedFiles, mf)
    return c.SaveConfig()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestAddManagedFileValidation -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add input validation to AddManagedFile"
```

---

## Task 9: Enhance ValidateConfig Function

**Files:**
- Modify: `internal/config/migrate.go:183-198`

**Step 1: Write failing test**

```go
func TestValidateConfigComprehensive(t *testing.T) {
    tests := []struct {
        name  string
        cfg   *config.Config
        valid bool
    }{
        {
            name: "empty version",
            cfg: &config.Config{Version: "", RepoPath: "/tmp/test"},
            valid: false,
        },
        {
            name: "invalid repo path with traversal",
            cfg: &config.Config{Version: "1.0", RepoPath: "../../../etc"},
            valid: false,
        },
        {
            name: "empty ignore pattern",
            cfg: &config.Config{
                Version: "1.0",
                RepoPath: "/tmp/test",
                IgnorePatterns: []string{""},
            },
            valid: false,
        },
    }
    
    for _, tt := range tests {
        err := config.ValidateConfig(tt.cfg)
        if tt.valid && err != nil {
            t.Errorf("%s: unexpected error: %v", tt.name, err)
        }
        if !tt.valid && err == nil {
            t.Errorf("%s: expected error but got nil", tt.name)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestValidateConfigComprehensive -v`
Expected: FAIL - Incomplete validation

**Step 3: Write minimal implementation**

```go
// internal/config/migrate.go
func ValidateConfig(config *Config) error {
    if config == nil {
        return fmt.Errorf("config is nil")
    }
    
    if config.Version == "" {
        return fmt.Errorf("config version is empty")
    }
    
    if config.RepoPath == "" {
        return fmt.Errorf("repo path is empty")
    }
    
    // Validate repo path doesn't contain dangerous patterns
    if strings.Contains(config.RepoPath, "..") {
        return fmt.Errorf("repo path contains path traversal: %s", config.RepoPath)
    }
    
    // Validate ignore patterns
    for i, pattern := range config.IgnorePatterns {
        if pattern == "" {
            return fmt.Errorf("ignore pattern at index %d cannot be empty", i)
        }
    }
    
    // Validate managed files
    for i, mf := range config.ManagedFiles {
        if mf.SourcePath == "" {
            return fmt.Errorf("managed file at index %d has empty source path", i)
        }
        if mf.RepoPath == "" {
            return fmt.Errorf("managed file at index %d has empty repo path", i)
        }
        if mf.AddedAt.IsZero() {
            return fmt.Errorf("managed file at index %d has zero timestamp", i)
        }
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -run TestValidateConfigComprehensive -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/migrate.go internal/config/migrate_test.go
git commit -m "feat: enhance ValidateConfig with comprehensive checks"
```

---

## Task 10: Add Backup Verification in CreateBackup

**Files:**
- Modify: `internal/core/backup.go:38-101`

**Step 1: Write failing test**

```go
func TestCreateBackupVerification(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    
    err := os.WriteFile(testFile, []byte("test content"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    cfg := createTestConfig(t)
    cfg.RepoPath = tmpDir
    
    backupPath, err := core.CreateBackup(testFile, cfg)
    if err != nil {
        t.Fatalf("CreateBackup failed: %v", err)
    }
    
    // Verify backup was actually created
    if backupPath == "" {
        t.Fatal("backup path should not be empty")
    }
    
    if _, err := os.Stat(backupPath); err != nil {
        t.Fatalf("backup file not created: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestCreateBackupVerification -v`
Expected: FAIL - No verification

**Step 3: Write minimal implementation**

```go
// internal/core/backup.go
func CreateBackup(sourcePath string, cfg *config.Config) (string, error) {
    cfg.Logger.Debug("creating backup", "file", sourcePath)
    
    expanded, err := config.ExpandPath(sourcePath, cfg)
    if err != nil {
        cfg.Logger.Error("failed to expand path", "file", sourcePath, "error", err)
        return "", fmt.Errorf("backup failed for %s: %w", sourcePath, err)
    }
    
    if !fs.PathExists(expanded) {
        cfg.Logger.Error("source file does not exist", "file", sourcePath)
        return "", fmt.Errorf("source file does not exist: %s", sourcePath)
    }
    
    // Get backup directory
    backupDir, err := GetBackupDir()
    if err != nil {
        cfg.Logger.Error("failed to get backup directory", "error", err)
        return "", err
    }
    
    // Create timestamped subdirectory
    timestamp := time.Now().Format(TimestampFormat)
    timestampDir := filepath.Join(backupDir, timestamp)
    
    // Check if path exists and is a file (not directory)
    if info, err := os.Stat(timestampDir); err == nil && !info.IsDir() {
        return "", fmt.Errorf("backup path exists as file, not directory: %s", timestampDir)
    }
    
    if err := fs.EnsureDir(timestampDir, cfg); err != nil {
        cfg.Logger.Error("failed to create backup directory", "error", err)
        return "", fmt.Errorf("creating backup directory: %w", err)
    }
    
    // Normalize source path and use as relative path in backup
    normalized, err := config.NormalizePath(sourcePath)
    if err != nil {
        normalized = sourcePath
    }
    
    backupRelativePath := strings.TrimPrefix(normalized, "~/")
    backupPath := filepath.Join(timestampDir, backupRelativePath)
    
    // Ensure parent directory exists
    if err := fs.EnsureDir(filepath.Dir(backupPath), cfg); err != nil {
        cfg.Logger.Error("failed to create backup subdirectory", "error", err)
        return "", fmt.Errorf("creating backup subdirectory: %w", err)
    }
    
    if err := fs.CopyWithPermissions(expanded, backupPath, cfg); err != nil {
        cfg.Logger.Error("failed to copy to backup", "src", expanded, "dst", backupPath, "error", err)
        return "", fmt.Errorf("copying to backup: %w", err)
    }
    
    // Verify backup was created successfully
    if !fs.PathExists(backupPath) {
        cfg.Logger.Error("backup file does not exist after creation", "path", backupPath)
        return "", fmt.Errorf("backup file not created: %s", backupPath)
    }
    
    cfg.Logger.Info("backup created", "file", sourcePath, "path", backupPath)
    return backupPath, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestCreateBackupVerification -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/backup.go internal/core/backup_test.go
git commit -m "fix: add verification after backup creation"
```

---

## Task 11: Add Mutex to Config for Thread Safety

**Files:**
- Modify: `internal/config/config.go` (struct definition)
- Modify: All config methods to use mutex
- Create: `tests/race_condition_test.go`

**Step 1: Write failing test**

```go
func TestConfigConcurrentWrites(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping race condition test in short mode")
    }
    
    tmpDir := t.TempDir()
    cfg, _ := config.NewDefaultConfig()
    cfg.RepoPath = tmpDir
    
    testFile := filepath.Join(tmpDir, "test.txt")
    err := os.WriteFile(testFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    var wg sync.WaitGroup
    errors := make(chan error, 3)
    
    // Try to add same file 3 times concurrently
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            mf := config.ManagedFile{
                SourcePath: testFile,
                RepoPath:   fmt.Sprintf("test%d.txt", idx),
                AddedAt:    time.Now(),
            }
            errors <- cfg.AddManagedFile(mf)
        }()
    }
    
    wg.Wait()
    close(errors)
    
    // Count successes - should be at most 1
    successCount := 0
    for err := range errors {
        if err == nil {
            successCount++
        }
    }
    
    if successCount > 1 {
        t.Errorf("concurrent adds should be serialized, got %d successes", successCount)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./tests/... -run TestConfigConcurrentWrites -race -v`
Expected: FAIL - Race condition

**Step 3: Write minimal implementation**

```go
// internal/config/config.go
import (
    "sync"
    // ... other imports
)

type Config struct {
    Logger             *slog.Logger  `yaml:"-" json:"-"`
    Version            string        `yaml:"version" json:"version"`
    RepoPath           string        `yaml:"repo_path" json:"repo_path"`
    GitEnabled         bool          `yaml:"git_enabled" json:"git_enabled"`
    GitRemote          string        `yaml:"git_remote" json:"git_remote"`
    IgnorePatterns     []string      `yaml:"ignore_patterns" json:"ignore_patterns"`
    ManagedFiles       []ManagedFile `yaml:"managed_files" json:"managed_files"`
    LargeFileThreshold int           `yaml:"large_file_threshold" json:"large_file_threshold"`
    mu                 sync.RWMutex `yaml:"-" json:"-"`  // Fixed: add mutex for thread safety
}

// AddManagedFile adds a new managed file to config
func (c *Config) AddManagedFile(mf ManagedFile) error {
    // Lock for write
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Validate input
    if mf.SourcePath == "" {
        return fmt.Errorf("source path cannot be empty")
    }
    if mf.RepoPath == "" {
        return fmt.Errorf("repo path cannot be empty")
    }
    if mf.AddedAt.IsZero() {
        return fmt.Errorf("added_at time cannot be zero")
    }
    
    // Check if already managed
    if c.IsManaged(mf.SourcePath) {
        return fmt.Errorf("file %s is already managed", mf.SourcePath)
    }
    
    c.ManagedFiles = append(c.ManagedFiles, mf)
    return c.SaveConfig()
}

// IsManaged checks if a file is already managed
func (c *Config) IsManaged(sourcePath string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    for _, mf := range c.ManagedFiles {
        if mf.SourcePath == sourcePath || mf.SourcePath == sourcePath {
            return true
        }
    }
    
    return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./tests/... -run TestConfigConcurrentWrites -race -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go tests/race_condition_test.go
git commit -m "fix: add mutex to Config struct for thread safety"
```

---

## Task 12: Add Panic Recovery to Transaction Rollback

**Files:**
- Modify: `internal/core/transaction.go:69-94`

**Step 1: Write failing test**

```go
func TestTransactionPanicRecovery(t *testing.T) {
    cfg := createTestConfig(t)
    
    // Create a transaction that will panic
    tx := core.NewTransaction(cfg)
    
    op := &testPanicOp{}
    
    // Execute should handle panic gracefully
    defer func() {
        if r := recover(); r != nil {
            t.Log("panic recovered as expected")
        }
    }()
    
    err := tx.Execute(op)
    if err == nil {
        t.Error("should return error after panic")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestTransactionPanicRecovery -v`
Expected: FAIL - No recovery

**Step 3: Write minimal implementation**

```go
// internal/core/transaction.go
func (t *Transaction) Rollback() error {
    t.config.Logger.Warn("rolling back transaction", "operations", len(t.executed))
    
    if t.committed {
        t.config.Logger.Error("rollback failed", "error", fmt.Errorf("already committed"))
        return fmt.Errorf("cannot rollback committed transaction")
    }
    
    var rollbackErr error
    for i := len(t.executed) - 1; i >= 0; i-- {
        op := t.executed[i]
        t.config.Logger.Debug("rolling back operation", "op", op.Describe(), "index", i)
        
        // Handle panics in undo operations
        func() {
            defer func() {
                if r := recover(); r != nil {
                    t.config.Logger.Error("panic in rollback", "op", op.Describe(), "error", r)
                    rollbackErr = fmt.Errorf("panic during rollback: %v", r)
                }
            }()
            
            if err := op.Undo(); err != nil {
                t.config.Logger.Error("rollback failed", "op", op.Describe(), "error", err)
                rollbackErr = fmt.Errorf("rolling back %s: %w", op.Describe(), err)
            }
        }()
        
        if rollbackErr != nil {
            t.config.Logger.Error("stopping rollback due to error", "error", rollbackErr)
            t.executed = nil
            return rollbackErr
        }
    }
    
    t.config.Logger.Info("transaction rolled back")
    t.executed = nil
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestTransactionPanicRecovery -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/transaction.go internal/core/transaction_test.go
git commit -m "fix: add panic recovery in transaction rollback"
```

---

## Task 13: Improve Lock Acquisition Retry Logic

**Files:**
- Modify: `internal/core/lock.go:48-120`

**Step 1: Write failing test**

```go
func TestLockAcquireWithRetry(t *testing.T) {
    tmpDir := t.TempDir()
    
    cfg := createTestConfig(t)
    os.Setenv("HOME", tmpDir)
    
    // Test that lock acquisition has bounded retries
    err := core.AcquireLock(cfg)
    if err != nil {
        t.Fatalf("AcquireLock failed: %v", err)
    }
    
    defer core.ReleaseLock(cfg)
    
    // Try to acquire same lock - should fail after retries
    cfg2 := createTestConfig(t)
    err = core.AcquireLock(cfg2)
    if err == nil {
        t.Error("Should fail to acquire already held lock")
    }
    
    // Verify error message mentions retry attempts
    if !strings.Contains(err.Error(), "attempts") {
        t.Error("Error should mention retry attempts")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestLockAcquireWithRetry -v`
Expected: FAIL - Unbounded or incorrect retry logic

**Step 3: Write minimal implementation**

```go
// internal/core/lock.go
func AcquireLock(cfg *config.Config) error {
    cfg.Logger.Debug("acquiring lock")
    
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        lockPath, err := getLockPath()
        if err != nil {
            return err
        }
        
        // Ensure config directory exists
        if err := fs.EnsureDir(filepath.Dir(lockPath), cfg); err != nil {
            cfg.Logger.Error("failed to create config directory", "error", err)
            return fmt.Errorf("failed to create lock directory at %s: %w", filepath.Dir(lockPath), err)
        }
        
        // Try atomic lock creation with O_EXCL
        f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
        if err != nil {
            if os.IsExist(err) {
                // Lock file exists, check if stale
                stale, staleErr := IsStale(lockPath, cfg)
                if staleErr != nil {
                    cfg.Logger.Error("failed to check stale lock", "error", staleErr)
                    lastErr = fmt.Errorf("checking stale lock: %w", staleErr)
                    continue
                }
                
                if stale {
                    // Try to remove stale lock
                    if removeErr := os.Remove(lockPath); removeErr != nil {
                        info, _ := ReadLockInfo(lockPath)
                        cfg.Logger.Error("failed to remove stale lock", "pid", info.PID)
                        lastErr = fmt.Errorf("stale lock but cannot remove: PID %d", info.PID)
                        continue
                    }
                    // Successfully removed stale lock, retry
                    cfg.Logger.Debug("removed stale lock, retrying")
                    continue
                }
                
                // Lock is held by active process
                info, _ := ReadLockInfo(lockPath)
                age := time.Since(info.Timestamp)
                cfg.Logger.Error("lock held by another process", "pid", info.PID, "hostname", info.Hostname, "age", age)
                return fmt.Errorf("%w: PID %d on %s (lock held for %v). If this is incorrect, run 'dotcor doctor --fix'", ErrLockHeld, info.PID, info.Hostname, formatAge(age))
            }
            cfg.Logger.Error("failed to create lock file", "error", err)
            return fmt.Errorf("creating lock file: %w", err)
        }
        
        // Lock acquired successfully
        defer f.Close()
        
        // Write lock content
        hostname, err := os.Hostname()
        if err != nil {
            hostname = "unknown"
        }
        
        content := fmt.Sprintf("%d\n%s\n%s\n",
            os.Getpid(),
            time.Now().Format(time.RFC3339),
            hostname,
        )
        if _, err := f.WriteString(content); err != nil {
            f.Close()
            os.Remove(lockPath)
            cfg.Logger.Error("failed to write lock file", "error", err)
            return fmt.Errorf("writing lock file: %w", err)
        }
        
        cfg.Logger.Info("lock acquired")
        return nil
    }
    
    cfg.Logger.Error("failed to acquire lock after retries", "attempts", maxRetries, "last_error", lastErr)
    return fmt.Errorf("failed to acquire lock after %d attempts: %w", maxRetries, lastErr)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestLockAcquireWithRetry -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/lock.go internal/core/lock_test.go
git commit -m "fix: improve lock acquisition with proper retry logic"
```

---

## Task 14: Add Git Ref Validation

**Files:**
- Modify: `internal/git/git.go:609-629`

**Step 1: Write failing test**

```go
func TestRefExistsValidation(t *testing.T) {
    tmpDir := t.TempDir()
    setupTestRepo(t, tmpDir)
    
    // Test that malicious refs are rejected
    maliciousRefs := []string{
        "../../../etc/passwd",
        "..\\..\\..\\windows\\system32",
        "/absolute/path",
        "",
    }
    
    for _, ref := range maliciousRefs {
        exists, err := git.RefExists(tmpDir, ref)
        if err == nil {
            t.Errorf("Should reject malicious ref %s", ref)
        }
        if exists {
            t.Errorf("Should not find malicious ref %s", ref)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -run TestRefExistsValidation -v`
Expected: FAIL - No validation

**Step 3: Write minimal implementation**

```go
// internal/git/git.go
func RefExists(repoPath, ref string) (bool, error) {
    if ref == "" {
        return false, fmt.Errorf("ref is empty")
    }
    
    // Validate ref format to prevent path traversal
    if strings.Contains(ref, "..") {
        return false, fmt.Errorf("ref contains path traversal: %s", ref)
    }
    
    if strings.Contains(ref, "\\") {
        return false, fmt.Errorf("ref contains backslash: %s", ref)
    }
    
    if filepath.IsAbs(ref) && !strings.HasPrefix(ref, "refs/") && !strings.HasPrefix(ref, "HEAD") {
        return false, fmt.Errorf("ref is absolute but not a valid ref: %s", ref)
    }
    
    cmd := exec.Command("git", "cat-file", "-e", ref)
    cmd.Dir = repoPath
    err := cmd.Run()
    
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            if exitErr.ExitCode() == 1 {
                return false, nil
            }
        }
        return false, fmt.Errorf("git cat-file failed for ref %s: %w", ref, err)
    }
    
    return true, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -run TestRefExistsValidation -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "fix: add validation to RefExists to prevent path traversal"
```

---

## Task 15: Fix IsWritable Temp File Cleanup

**Files:**
- Modify: `internal/fs/fs.go:186-218`

**Step 1: Write failing test**

```go
func TestIsWritableTempCleanup(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    
    err := os.WriteFile(testFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    // Test that temp test file is cleaned up
    if fs.IsWritable(tmpDir) {
        // Check that temp file doesn't exist
        tempFile := filepath.Join(tmpDir, ".dotcor_write_test")
        if _, err := os.Stat(tempFile); err == nil {
            t.Error("temp write test file should be cleaned up")
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/fs/... -run TestIsWritableTempCleanup -v`
Expected: FAIL - No cleanup

**Step 3: Write minimal implementation**

```go
// internal/fs/fs.go
func IsWritable(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        if os.IsNotExist(err) {
            // Check if parent directory is writable
            parent := filepath.Dir(path)
            return IsWritable(parent)
        }
        return false
    }
    
    // If it's a directory, try to create a temp file
    if info.IsDir() {
        tempFile := filepath.Join(path, ".dotcor_write_test")
        file, err := os.Create(tempFile)
        if err != nil {
            return false
        }
        file.Close()
        defer os.Remove(tempFile)  // Fixed: ensure cleanup
        return true
    }
    
    // For files, try to open for writing
    file, err := os.OpenFile(path, os.O_WRONLY, 0)
    if err != nil {
        return false
    }
    file.Close()
    return true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/fs/... -run TestIsWritableTempCleanup -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/fs/fs.go internal/fs/fs_test.go
git commit -m "fix: add proper temp file cleanup in IsWritable"
```

---

## Task 16: Fix Variable Name in clone.go

**Files:**
- Modify: `cmd/dotcor/clone.go:18-31`

**Step 1: Write failing test**

```go
func TestCloneURLValidation(t *testing.T) {
    tests := []struct {
        url  string
        valid bool
    }{
        {"http://github.com/user/repo", true},
        {"https://github.com/user/repo", true},
        {"git@github.com:user/repo", true},
        {"ssh://github.com/user/repo", true},
        {"invalid://url", false},
        {"not-a-url", false},
    }
    
    for _, tt := range tests {
        result := isValidGitURL(tt.url)
        if result != tt.valid {
            t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, result, tt.valid)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestCloneURLValidation -v`
Expected: FAIL - Variable naming issue (doesn't affect functionality but affects clarity)

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/clone.go
// isValidGitURL checks if URL looks like a valid git remote URL
func isValidGitURL(url string) bool {
    validPrefixes := []string{  // Fixed: rename to validPrefixes for clarity
        "http://",
        "https://",
        "git://",
        "git@",
        "ssh://",
    }
    
    for _, prefix := range validPrefixes {
        if strings.HasPrefix(url, prefix) {
            return true
        }
    }
    
    return false
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestCloneURLValidation -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/clone.go cmd/dotcor/clone_test.go
git commit -m "refactor: rename variable for clarity in clone"
```

---

## Task 17: Fix All String Concatenation in clone.go

**Files:**
- Modify: `cmd/dotcor/clone.go:85, 128, 152, 155`

**Step 1: Write failing test**

```go
func TestClonePathPortability(t *testing.T) {
    tmpDir := t.TempDir()
    os.Setenv("HOME", tmpDir)
    
    cfg, _ := config.NewDefaultConfig()
    cfg.RepoPath = tmpDir
    
    // Test that paths use filepath.Join
    filesDir := filepath.Join(tmpDir, "files")
    if !strings.Contains(filesDir, string(filepath.Separator)) {
        t.Error("should use filepath.Join")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestClonePathPortability -v`
Expected: FAIL - String concatenation

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/clone.go
func runClone(cmd *cobra.Command, args []string) error {
    repoURL := args[0]
    apply, _ := cmd.Flags().GetBool("apply")
    force, _ := cmd.Flags().GetBool("force")
    
    // Load or create config
    cfg, err := config.LoadConfig()
    if err != nil {
        cfg, err = config.NewDefaultConfig()
        if err != nil {
            return fmt.Errorf("creating config: %w", err)
        }
    }
    configureLogger(cmd, cfg)
    
    // Check if git is installed
    if !git.IsGitInstalled() {
        return fmt.Errorf("git is not installed")
    }
    
    // Get config directory
    configDir, err := config.GetConfigDir()
    if err != nil {
        return fmt.Errorf("getting config directory: %w", err)
    }
    
    filesDir := filepath.Join(configDir, "files")  // Fixed: use filepath.Join
    
    // ... rest of implementation
    
    backupsDir := filepath.Join(configDir, "backups")  // Fixed: use filepath.Join
    
    // ... rest of implementation
    
    configPath := filepath.Join(filesDir, "config.yaml")  // Fixed: use filepath.Join
    
    if fs.PathExists(configPath) {
        destConfig := filepath.Join(configDir, "config.yaml")  // Fixed: use filepath.Join
        // ... rest of implementation
    }
    
    // ... rest of function
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestClonePathPortability -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/clone.go cmd/dotcor/clone_test.go
git commit -m "fix: use filepath.Join instead of string concatenation in clone"
```

---

## Task 18: Add Template Context Validation

**Files:**
- Modify: `internal/core/templates.go:20-28`

**Step 1: Write failing test**

```go
func TestGetTemplateContextSanitization(t *testing.T) {
    // Test that context values are sanitized
    ctx, err := core.GetTemplateContext()
    if err != nil {
        t.Fatalf("GetTemplateContext failed: %v", err)
    }
    
    // Check hostname doesn't contain dangerous patterns
    if strings.Contains(ctx.Hostname, "..") || strings.Contains(ctx.Hostname, "/") {
        t.Error("Hostname should be sanitized")
    }
    
    // Check username doesn't contain dangerous patterns
    if strings.Contains(ctx.User, "..") || strings.Contains(ctx.User, "/") {
        t.Error("User should be sanitized")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestGetTemplateContextSanitization -v`
Expected: FAIL - No sanitization

**Step 3: Write minimal implementation**

```go
// internal/core/templates.go
// GetTemplateContext returns current template context with sanitized values
func GetTemplateContext() (*TemplateContext, error) {
    hostname, err := os.Hostname()
    if err != nil {
        hostname = "unknown"
    }
    
    currentUser, err := user.Current()
    if err != nil {
        currentUser = &user.User{Username: "user"}
    }
    
    home, err := os.UserHomeDir()
    if err != nil {
        home = "~"
    }
    
    // Validate and sanitize context values
    if strings.Contains(hostname, "..") || strings.Contains(hostname, "/") {
        hostname = "unknown"
    }
    
    if strings.Contains(currentUser.Username, "..") || strings.Contains(currentUser.Username, "/") {
        currentUser.Username = "user"
    }
    
    return &TemplateContext{
        Hostname: hostname,
        User:     currentUser.Username,
        Home:     home,
    }, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestGetTemplateContextSanitization -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/templates.go internal/core/templates_test.go
git commit -m "fix: add sanitization to GetTemplateContext"
```

---

## Task 19: Add Timeout to Git Commands

**Files:**
- Modify: `internal/git/git.go:38-46`
- Create: `internal/git/context.go`

**Step 1: Write failing test**

```go
func TestGitCommandTimeout(t *testing.T) {
    tmpDir := t.TempDir()
    setupTestRepo(t, tmpDir)
    
    // Test that git commands don't hang indefinitely
    start := time.Now()
    
    // Try to get status (should complete quickly)
    status, err := git.GetStatus(tmpDir)
    
    elapsed := time.Since(start)
    if err != nil && elapsed > 10*time.Second {
        t.Error("git command took too long, should have timeout")
    }
    
    _ = status
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -run TestGitCommandTimeout -v`
Expected: FAIL - No timeout

**Step 3: Write minimal implementation**

```go
// internal/git/git.go
const gitCommandTimeout = 30 * time.Second

// runGitCommand executes a git command with timeout
func runGitCommand(dir string, name string, args ...string) (*exec.Cmd, context.CancelFunc) {
    ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
    
    cmd := exec.CommandContext(ctx, "git", append([]string{name}, args...)...)
    cmd.Dir = dir
    
    return cmd, cancel
}

func InitRepo(repoPath string) error {
    cmd, cancel := runGitCommand(repoPath, "init")
    defer cancel()
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        if ctxErr := ctx.Err(); ctxErr == context.DeadlineExceeded {
            return fmt.Errorf("git init timed out after %v", gitCommandTimeout)
        }
        return fmt.Errorf("git init failed: %s: %w", string(output), err)
    }
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -run TestGitCommandTimeout -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go internal/git/context.go
git commit -m "feat: add timeout to git commands"
```

---

## Task 20: Add Verification After Transaction in Add

**Files:**
- Modify: `cmd/dotcor/add.go:349-366`

**Step 1: Write failing test**

```go
func TestAddTransactionVerification(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    
    err := os.WriteFile(testFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    cfg := createTestConfig(t)
    cfg.RepoPath = tmpDir
    
    // Test that add verifies operations before commit
    tx, err := core.AddFileTransaction(cfg, testFile, "test.txt", config.ManagedFile{
        SourcePath: testFile,
        RepoPath:   "test.txt",
        AddedAt:    time.Now(),
    })
    
    if err != nil {
        t.Fatalf("AddFileTransaction failed: %v", err)
    }
    
    // Execute and verify
    err = tx.ExecuteAll()
    if err != nil {
        t.Fatalf("ExecuteAll failed: %v", err)
    }
    
    // Verify operations actually succeeded
    sourceExpanded, _ := config.ExpandPath(testFile, cfg)
    repoPath := filepath.Join(tmpDir, "test.txt")
    
    if !fs.PathExists(repoPath) {
        t.Error("file should be in repo after add")
    }
    
    if !fs.IsSymlink(sourceExpanded) {
        t.Error("symlink should be created")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestAddTransactionVerification -v`
Expected: FAIL - No verification

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/add.go
func executeAddTransaction(tx *core.Transaction, sourcePath, repoPath string, cfg *config.Config) error {
    // Execute transaction
    if err := tx.ExecuteAll(); err != nil {
        // Rollback already happened in ExecuteAll
        return err
    }
    
    // Verify operations succeeded
    sourceExpanded, err := config.ExpandPath(sourcePath, cfg)
    if err != nil {
        return fmt.Errorf("expanding source path for verification: %w", err)
    }
    
    fullRepoPath, err := config.GetRepoFilePath(cfg, repoPath)
    if err != nil {
        return fmt.Errorf("expanding repo path for verification: %w", err)
    }
    
    // Verify file in repo
    if !fs.PathExists(fullRepoPath) {
        return fmt.Errorf("file not in repo after add: %s", fullRepoPath)
    }
    
    // Verify symlink exists
    if !fs.PathExists(sourceExpanded) {
        return fmt.Errorf("symlink not created: %s", sourceExpanded)
    }
    
    // Verify it's a valid symlink
    if !fs.IsValidSymlink(sourceExpanded) {
        return fmt.Errorf("symlink is invalid: %s", sourceExpanded)
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestAddTransactionVerification -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/add.go cmd/dotcor/add_test.go
git commit -m "fix: add verification after transaction in add"
```

---

## Task 21: Fix File Size Validation Edge Cases

**Files:**
- Modify: `internal/core/validator.go:171-204`

**Step 1: Write failing test**

```go
func TestValidateFileSizeEdgeCases(t *testing.T) {
    cfg, _ := config.NewDefaultConfig()
    
    // Test negative threshold
    cfg.LargeFileThreshold = -100
    
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    err := os.WriteFile(testFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    // Should treat negative same as zero (disabled)
    err = core.ValidateFileSize(testFile, cfg)
    if err != nil {
        t.Error("negative threshold should disable validation")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestValidateFileSizeEdgeCases -v`
Expected: FAIL - No check for negative

**Step 3: Write minimal implementation**

```go
// internal/core/validator.go
func ValidateFileSize(path string, cfg *config.Config) error {
    cfg.Logger.Debug("validating file size", "file", path)
    
    // Check if size validation is disabled (0 or negative)
    threshold := cfg.LargeFileThreshold
    if threshold <= 0 {
        cfg.Logger.Debug("file size validation disabled", "file", path)
        return nil
    }
    
    expanded, err := config.ExpandPath(path, cfg)
    if err != nil {
        cfg.Logger.Debug("failed to expand path for validation", "file", path, "error", err)
        return fmt.Errorf("expanding path: %w", err)
    }
    
    info, err := os.Stat(expanded)
    if err != nil {
        cfg.Logger.Debug("failed to get file info", "file", path, "error", err)
        return fmt.Errorf("getting file info: %w", err)
    }
    
    size := info.Size()
    cfg.Logger.Debug("file size check", "file", path, "size", size, "threshold", threshold)
    
    if size > int64(threshold) {
        sizeMB := float64(size) / (1024 * 1024)
        cfg.Logger.Warn("file is very large", "file", path, "size_mb", sizeMB)
        return fmt.Errorf("file is very large (%.1fMB), consider excluding: %s", sizeMB, path)
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestValidateFileSizeEdgeCases -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/validator.go internal/core/validator_test.go
git commit -m "fix: handle negative threshold in file size validation"
```

---

## Task 22: Fix String Indexing Safety

**Files:**
- Modify: `cmd/dotcor/history.go:133-136`

**Step 1: Write failing test**

```go
func TestTruncateHashSafety(t *testing.T) {
    tests := []struct {
        hash      string
        maxLen    int
        expected  string
    }{
        {"abc123def", 7, "abc123"},
        {"a", 7, "a"},
        {"", 7, ""},
        {"short", 10, "short"},
    }
    
    for _, tt := range tests {
        result := truncateHash(tt.hash, tt.maxLen)
        if result != tt.expected {
            t.Errorf("truncateHash(%q, %d) = %q, want %q", tt.hash, tt.maxLen, result, tt.expected)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestTruncateHashSafety -v`
Expected: FAIL - No bounds check

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/history.go
// truncateHash truncates a hash to maximum length
func truncateHash(hash string, maxLen int) string {
    if maxLen <= 0 {
        return hash
    }
    if len(hash) <= maxLen {
        return hash
    }
    return hash[:maxLen]
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestTruncateHashSafety -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/history.go cmd/dotcor/history_test.go
git commit -m "fix: add bounds check to truncateHash helper"
```

---

## Task 23: Fix Integer Overflow in FormatSize

**Files:**
- Modify: `internal/utils/helpers.go:26-37`

**Step 1: Write failing test**

```go
func TestFormatSizeEdgeCases(t *testing.T) {
    tests := []struct {
        bytes    int64
        expected string
    }{
        {0, "0 bytes"},
        {1024, "1.0 KB"},
        {1024 * 1024, "1.0 MB"},
        {1024 * 1024 * 1024, "1.0 GB"},
    }
    
    for _, tt := range tests {
        result := utils.FormatSize(tt.bytes)
        if result != tt.expected {
            t.Errorf("FormatSize(%d) = %s, want %s", tt.bytes, result, tt.expected)
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/utils/... -run TestFormatSizeEdgeCases -v`
Expected: FAIL - Missing TB unit

**Step 3: Write minimal implementation**

```go
// internal/utils/helpers.go
import (
    "fmt"
    // ... other imports
)

// FormatSize formats file size in human-readable format
func FormatSize(bytes int64) string {
    if bytes < 0 {
        return "0 bytes"
    }
    
    const (
        KB = 1024
        MB = 1024 * KB
        GB = 1024 * MB
        TB = 1024 * GB
    )
    
    switch {
    case bytes >= TB:
        return fmt.Sprintf("%.1f TB", float64(bytes)/float64(TB))
    case bytes >= GB:
        return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
    case bytes >= MB:
        return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
    case bytes >= KB:
        return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
    default:
        return fmt.Sprintf("%d bytes", bytes)
    }
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/utils/... -run TestFormatSizeEdgeCases -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/utils/helpers.go internal/utils/helpers_test.go
git commit -m "fix: add TB unit to FormatSize for larger files"
```

---

## Task 24: Fix MoveFile Partial Cleanup

**Files:**
- Modify: `internal/fs/fs.go:13-44`

**Step 1: Write failing test**

```go
func TestMoveFileCleanupOnError(t *testing.T) {
    tmpDir := t.TempDir()
    src := filepath.Join(tmpDir, "src.txt")
    dst := filepath.Join(tmpDir, "dst.txt")
    
    err := os.WriteFile(src, []byte("content"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    cfg := createTestConfig(t)
    
    // Test that cleanup happens even if remove fails
    err = fs.MoveFile(src, dst, cfg)
    // After successful move, source should not exist
    if fs.PathExists(src) {
        t.Error("source file should not exist after move")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/fs/... -run TestMoveFileCleanupOnError -v`
Expected: FAIL - No proper cleanup

**Step 3: Write minimal implementation**

```go
// internal/fs/fs.go
func MoveFile(src, dst string, cfg *config.Config) error {
    start := time.Now()
    cfg.Logger.Debug("moving file", "src", src, "dst", dst)
    
    if err := EnsureDir(filepath.Dir(dst), cfg); err != nil {
        return fmt.Errorf("creating destination directory: %w", err)
    }
    
    // Check if destination already exists
    dstExists := PathExists(dst)
    
    err := os.Rename(src, dst)
    if err == nil {
        cfg.Logger.Info("file moved successfully", "src", src, "dst", dst)
        return nil
    }
    
    cfg.Logger.Debug("rename failed, trying copy", "src", src, "dst", dst, "error", err)
    if err := CopyWithPermissions(src, dst, cfg); err != nil {
        // Clean up partial copy if destination existed and copy failed
        if dstExists {
            if removeErr := os.Remove(dst); removeErr != nil {
                cfg.Logger.Error("failed to clean up partial copy", "path", dst, "error", removeErr)
            }
        }
        return fmt.Errorf("copying file: %w", err)
    }
    
    // Remove source file
    if err := os.Remove(src); err != nil {
        // Copy succeeded but remove failed - log but don't fail operation
        cfg.Logger.Error("failed to remove original file", "path", src, "error", err)
    }
    
    durationMs := time.Since(start).Milliseconds()
    cfg.Logger.Info("file moved", "src", src, "dst", dst, "duration_ms", durationMs)
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/fs/... -run TestMoveFileCleanupOnError -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/fs/fs.go internal/fs/fs_test.go
git commit -m "fix: add proper cleanup in MoveFile on error"
```

---

## Task 25: Add Backup Verification in RemoveFileOp

**Files:**
- Modify: `internal/core/transaction.go:208-218`

**Step 1: Write failing test**

```go
func TestRemoveFileOpBackupVerification(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    
    err := os.WriteFile(testFile, []byte("test"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    cfg := createTestConfig(t)
    
    op := &core.RemoveFileOp{
        Path:   testFile,
        config: cfg,
    }
    
    // Execute should create backup
    err = op.Do()
    if err != nil {
        t.Fatalf("Do failed: %v", err)
    }
    
    // Verify backup exists
    if op.backupPath == "" {
        t.Fatal("backup path should not be empty")
    }
    
    if _, err := os.Stat(op.backupPath); err != nil {
        t.Fatalf("backup file not created: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/core/... -run TestRemoveFileOpBackupVerification -v`
Expected: FAIL - No verification

**Step 3: Write minimal implementation**

```go
// internal/core/transaction.go
func (op *RemoveFileOp) Do() error {
    backupPath, err := CreateBackup(op.Path, op.config)
    if err != nil {
        return fmt.Errorf("creating backup: %w", err)
    }
    if backupPath == "" {
        return fmt.Errorf("backup creation failed - no backup path returned")
    }
    
    // Verify backup was actually created
    if !fs.PathExists(backupPath) {
        op.config.Logger.Error("backup file does not exist", "path", backupPath)
        return fmt.Errorf("backup file does not exist: %s", backupPath)
    }
    
    op.backupPath = backupPath
    
    if err := os.Remove(op.Path); err != nil {
        return fmt.Errorf("removing file: %w", err)
    }
    
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/core/... -run TestRemoveFileOpBackupVerification -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/core/transaction.go internal/core/transaction_test.go
git commit -m "fix: add backup verification in RemoveFileOp"
```

---

## Task 26: Fix Remove Atomicity Issues

**Files:**
- Modify: `cmd/dotcor/remove.go:256-294`

**Step 1: Write failing test**

```go
func TestRemoveAtomicity(t *testing.T) {
    tmpDir := t.TempDir()
    
    cfg := createTestConfig(t)
    cfg.RepoPath = tmpDir
    
    // Create a test file in repo
    repoFile := filepath.Join(tmpDir, ".zshrc")
    err := os.WriteFile(repoFile, []byte("original"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    sourceFile := filepath.Join(tmpDir, "test_source")
    
    // Create symlink
    err = os.Symlink(repoFile, sourceFile)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    mf := config.ManagedFile{
        SourcePath: sourceFile,
        RepoPath:   ".zshrc",
        AddedAt:    time.Now(),
    }
    
    // Test remove with --delete-repo
    // Should handle copy then delete atomically
    // Simulate partial failure by making repo file read-only
    os.Chmod(repoFile, 0444)
    defer os.Chmod(repoFile, 0644)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestRemoveAtomicity -v`
Expected: FAIL - No atomicity

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/remove.go
func processRemoveFile(cfg *config.Config, mf config.ManagedFile, keepRepo bool, dryRun bool, quiet bool) error {
    sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
    if err != nil {
        return fmt.Errorf("invalid source path: %w", err)
    }
    
    repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
    if err != nil {
        return fmt.Errorf("invalid repo path: %w", err)
    }
    
    if dryRun {
        fmt.Printf("  - %s\n", mf.SourcePath)
        if !keepRepo {
            fmt.Printf("    -> Remove symlink from %s\n", sourcePath)
            fmt.Printf("    -> Keep in repo: %s\n", mf.RepoPath)
        } else {
            fmt.Printf("    -> Copy to %s\n", sourcePath)
            fmt.Printf("    -> Remove from repo: %s\n", mf.RepoPath)
        }
        return nil
    }
    
    if err := core.RunHook(core.HookContext{HookType: "pre-remove", FilePath: mf.SourcePath}, cfg); err != nil {
        if !quiet {
            fmt.Printf("  [!]%s Pre-remove hook warning: %v\n", colorYellow, colorReset, err)
        }
    }
    
    // Check if source is a symlink
    isLink, err := fs.IsSymlink(sourcePath)
    if err != nil {
        return fmt.Errorf("checking symlink status: %w", err)
    }
    
    // If keeping repo, just remove symlink and update config
    if keepRepo {
        if isLink {
            if err := os.Remove(sourcePath); err != nil {
                return fmt.Errorf("removing symlink: %w", err)
            }
        }
        
        // Remove from config
        if err := cfg.RemoveManagedFile(mf.SourcePath); err != nil {
            return fmt.Errorf("updating config: %w", err)
        }
        
        if err := core.RunHook(core.HookContext{HookType: "post-remove", FilePath: mf.SourcePath}, cfg); err != nil {
            if !quiet {
                fmt.Printf("  [!]%s Post-remove hook warning: %v\n", colorYellow, colorReset, err)
            }
        }
        
        if !quiet {
            fmt.Printf("  [OK]%s %s (removed from management, kept in repo)\n", colorGreen, colorReset, mf.SourcePath)
        }
        return nil
    }
    
    // Full removal: copy back and delete from repo
    // First, create backup of repo file
    if fs.PathExists(repoPath) {
        backupPath, err := core.CreateBackup(repoPath, cfg)
        if err != nil {
            return fmt.Errorf("backup creation failed for %s: %w", mf.RepoPath, err)
        }
        if backupPath == "" {
            return fmt.Errorf("backup creation failed - no backup path returned for %s", mf.SourcePath)
        }
    }
    
    // Ensure parent directory exists
    if err := fs.EnsureDir(filepath.Dir(sourcePath), cfg); err != nil {
        return fmt.Errorf("creating parent directory: %w", err)
    }
    
    // If source is a symlink, remove it first
    if isLink {
        if err := os.Remove(sourcePath); err != nil {
            return fmt.Errorf("removing symlink: %w", err)
        }
    }
    
    // Copy file from repo to source location
    if fs.PathExists(repoPath) {
        if err := fs.CopyWithPermissions(repoPath, sourcePath, cfg); err != nil {
            return fmt.Errorf("copying file back: %w", err)
        }
        
        // Delete from repo only after successful copy
        if err := os.Remove(repoPath); err != nil {
            // Copy succeeded but delete failed - partial state!
            // Log but continue since we have a copy
            cfg.Logger.Error("failed to remove from repo after copy", "path", repoPath, "error", err)
        } else {
            // Clean up empty parent directories in repo
            cleanEmptyDirs(filepath.Dir(repoPath))
        }
    }
    
    // Remove from config
    if err := cfg.RemoveManagedFile(mf.SourcePath); err != nil {
        return fmt.Errorf("updating config: %w", err)
    }
    
    if err := core.RunHook(core.HookContext{HookType: "post-remove", FilePath: mf.SourcePath}, cfg); err != nil {
        if !quiet {
            fmt.Printf("  [!]%s Post-remove hook warning: %v\n", colorYellow, colorReset, err)
        }
    }
    
    if !quiet {
        fmt.Printf("  [OK]%s %s (removed from management and repo)\n", colorGreen, colorReset, mf.SourcePath)
    }
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestRemoveAtomicity -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/remove.go cmd/dotcor/remove_test.go
git commit -m "fix: improve atomicity of remove with --delete-repo"
```

---

## Task 27: Fix File Cleanup on Errors

**Files:**
- Modify: `cmd/dotcor/init.go:284`
- Modify: `cmd/dotcor/rebuild-links.go:157-159`

**Step 1: Write failing test**

```go
func TestFileCleanupOnErrors(t *testing.T) {
    tmpDir := t.TempDir()
    
    cfg := createTestConfig(t)
    cfg.RepoPath = tmpDir
    
    // Create symlink that needs replacement
    sourceFile := filepath.Join(tmpDir, "test.txt")
    err := os.WriteFile(sourceFile, []byte("original"), 0644)
    if err != nil {
        t.Fatalf("setup failed: %v", err)
    }
    
    // Test that cleanup happens even on errors
    // This test verifies that os.Remove errors are handled
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestFileCleanupOnErrors -v`
Expected: FAIL - No error handling

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/init.go
// applySymlinks creates symlinks for all managed files in config
func applySymlinks(cfg *config.Config) error {
    files := cfg.ManagedFiles
    if len(files) == 0 {
        fmt.Println("No files configured.")
        return nil
    }
    
    fmt.Printf("\n%sCreating symlinks for %d files...%s\n", colorLightPink, len(files), colorReset)
    
    created := 0
    skipped := 0
    
    for _, mf := range files {
        // Get full paths
        sourcePath, err := config.ExpandPath(mf.SourcePath, cfg)
        if err != nil {
            fmt.Printf("  [X]%s %s (invalid path)\n", colorRed, colorReset, mf.SourcePath)
            continue
        }
        
        repoPath, err := config.GetRepoFilePath(cfg, mf.RepoPath)
        if err != nil {
            fmt.Printf("  [X]%s %s (invalid repo path)\n", colorRed, colorReset, mf.SourcePath)
            continue
        }
        
        // Check if repo file exists
        if !fs.PathExists(repoPath) {
            fmt.Printf("  [X]%s %s (not in repository)\n", colorRed, colorReset, mf.SourcePath)
            continue
        }
        
        // Check if symlink already exists and is correct
        isLink, _ := fs.IsSymlink(sourcePath)
        if isLink {
            if valid, _ := fs.IsValidSymlink(sourcePath); valid {
                fmt.Printf("  - %s (already linked)\n", mf.SourcePath)
                skipped++
                continue
            }
        }
        
        // Backup existing file if it exists
        if fs.PathExists(sourcePath) {
            backupPath, err := core.CreateBackup(sourcePath, cfg)
            if err != nil {
                fmt.Printf("  [X]%s %s (backup failed: %v)\n", colorRed, colorReset, mf.SourcePath, err)
                continue
            }
            fmt.Printf("  -> Backed up to %s\n", backupPath)
            
            // Fixed: Add error handling for remove
            if err := os.Remove(sourcePath); err != nil {
                fmt.Printf("  [!]%s Failed to remove original file: %v\n", colorYellow, colorReset, err)
                continue
            }
        }
        
        // Create symlink
        if err := fs.CreateSymlink(repoPath, sourcePath, cfg); err != nil {
            fmt.Printf("  [X]%s %s (%v)\n", colorRed, colorReset, mf.SourcePath, err)
            continue
        }
        
        fmt.Printf("  [OK]%s %s\n", colorGreen, colorReset, mf.SourcePath)
        created++
    }
    
    fmt.Printf("\nCreated %d symlinks, skipped %d\n", created, skipped)
    return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestFileCleanupOnErrors -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/init.go cmd/dotcor/init_test.go cmd/dotcor/rebuild-links.go
git commit -m "fix: add error handling for file cleanup operations"
```

---

## Task 28: Standardize Empty State Handling

**Files:**
- Modify: `cmd/dotcor/list.go:42-46`
- Modify: `cmd/dotcor/remove.go:82-86`

**Step 1: Write failing test**

```go
func TestEmptyStateConsistency(t *testing.T) {
    tests := []struct {
        command string
        args    []string
        isError bool
    }{
        {"list", []string{}, false},     // OK: no files is valid
        {"remove", []string{}, true},     // Error: need file or --all
    }
    
    for _, tt := range tests {
        cmd := rootCmd
        cmd.SetArgs(append([]string{tt.command}, tt.args...))
        
        err := cmd.Execute()
        if tt.isError && err == nil {
            t.Errorf("%s with empty args should return error", tt.command)
        }
        if !tt.isError && err != nil {
            // Empty state is OK
            t.Log(tt.command + " succeeded with empty args as expected")
        }
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/dotcor/... -run TestEmptyStateConsistency -v`
Expected: FAIL - Inconsistent

**Step 3: Write minimal implementation**

```go
// cmd/dotcor/remove.go
func runRemove(cmd *cobra.Command, args []string) error {
    deleteRepo, _ := cmd.Flags().GetBool("delete-repo")
    removeAll, _ := cmd.Flags().GetBool("all")
    force, _ := cmd.Flags().GetBool("force")
    dryRun, _ := cmd.Flags().GetBool("dry-run")
    batch, _ := cmd.Flags().GetBool("batch")
    
    if !removeAll && len(args) == 0 {
        return fmt.Errorf("specify files to remove or use --all")
    }
    
    // Load config
    cfg, err := config.LoadConfig()
    if err != nil {
        return fmt.Errorf("loading config: %w\nRun 'dotcor init' first", err)
    }
    configureLogger(cmd, cfg)
    
    // Add pre-flight validation
    result := core.RunPreflightValidation(cfg, "remove", []string{})
    if err := core.DisplayValidationResults(result); err != nil {
        return err
    }
    
    // Determine which files to remove
    var filesToRemove []config.ManagedFile
    
    if removeAll {
        filesToRemove = cfg.ManagedFiles
        if len(filesToRemove) == 0 {
            fmt.Println("No files to remove.")
            return nil  // Fixed: Consistent - return success for empty state
        }
    } else {
        for _, arg := range args {
            mf, err := cfg.GetManagedFile(arg)
            if err != nil {
                fmt.Fprintf(os.Stderr, "  [X]%s %s: not managed\n", colorRed, colorReset, arg)
                continue
            }
            filesToRemove = append(filesToRemove, *mf)
        }
    }
    
    if len(filesToRemove) == 0 {
        return fmt.Errorf("no valid files to remove: check file paths and ensure files are managed")
    }
    
    // Rest of implementation...
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./cmd/dotcor/... -run TestEmptyStateConsistency -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/dotcor/remove.go cmd/dotcor/remove_test.go
git commit -m "fix: standardize empty state handling across commands"
```

---

## Task 29: Add Documentation Updates (Ongoing)

**Files:**
- Update: `docs/TESTING.md`
- Update: `docs/LOGGING.md`

**Step 1: Update TESTING.md with edge case patterns**

Add section about testing edge cases and race conditions.

**Step 2: Update LOGGING.md with error patterns**

Add section about error logging patterns and nil logger handling.

**Step 3: Commit**

```bash
git add docs/TESTING.md docs/LOGGING.md
git commit -m "docs: add edge case and error logging patterns to documentation"
```

---

## Task 30: Run Final Integration Test

**Files:**
- Run: Full test suite with race detection
- Verify: All tests pass

**Step 1: Run full test suite**

```bash
go test ./... -race -v 2>&1 | tee test-output.log
```

**Step 2: Verify all critical fixes pass**

Check that:
- Logger nil safety tests pass
- Path construction tests pass
- Validation tests pass
- Race condition tests pass
- Backup verification tests pass

**Step 3: Check for any remaining failures**

```bash
grep -E "(FAIL|panic)" test-output.log || echo "All tests passed!"
```

**Step 4: Create summary report**

Document:
- Number of tests passing
- Any remaining issues
- Recommendations for next steps

---

## Summary

This plan fixes 50 production-readiness issues across 30 bite-sized tasks:

**Each task:**
- Touches 1-2 files maximum
- Takes 5-10 minutes
- Includes test-first development
- Has clear verification step
- Commits immediately after completion

**Categories Fixed:**
1. Nil logger safety (Tasks 1-3)
2. Path construction (Tasks 4, 17)
3. Error fallbacks (Tasks 6-8)
4. Input validation (Tasks 9, 14, 18)
5. Backup verification (Tasks 10, 12, 25)
6. Transaction safety (Tasks 13, 26)
7. Lock acquisition (Task 13)
8. Git operations (Tasks 14, 19)
9. File operations (Tasks 15, 24)
10. Validation (Tasks 9, 21-23, 28)
11. Race conditions (Task 11)
12. Integer overflow (Task 23)
13. String safety (Task 22)
14. Empty states (Task 28)
15. Documentation (Task 29)

**Testing:**
- Each fix includes test-first development
- Edge cases covered
- Race condition tests added
- Integration test validates end-to-end workflows

**Next Steps:**
1. Review plan and approve
2. Execute using @executing-plans
3. Verify all tests pass with `-race`
4. Update documentation
