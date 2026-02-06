# DotCor Implementation Plan

A detailed, step-by-step implementation plan for building DotCor - a symlink-based dotfile manager with Git automation.

---

## Overview

DotCor is a CLI-first dotfile manager built in Go. It uses **symlinks** to keep your dotfiles in a Git repository while making them accessible in their original locations.

**Core Design Decisions:**
- **Storage:** Symlink-based (files live in repo, symlinks point to them)
- **Symlink type:** Relative paths (portable across machines and mount points)
- **File granularity:** Individual files with recursive directory support
- **Safety:** Automatic backups before destructive operations + transaction rollback
- **Concurrency:** File-based locking to prevent concurrent operations
- **Git:** Automatic commits after every operation with robust error handling
- **Conflicts:** Let Git handle merges (we provide helpful messages)
- **Config:** YAML format with Viper, versioned for migrations
- **Paths:** Normalize to `~` for portability
- **Cross-platform:** macOS/Linux native, Windows requires Developer Mode (NO FALLBACK)
- **Security:** Secret detection and ignore patterns

**Key Differentiator:** GNU Stow's simplicity + Git automation + production-grade safety guarantees

---

## Project Structure

```
dotcor/
├── cmd/
│   └── dotcor/
│       ├── main.go           # Entry point + Cobra root command
│       ├── init.go           # dotcor init (with interactive mode)
│       ├── add.go            # dotcor add <file> (with glob support)
│       ├── remove.go         # dotcor remove <file>
│       ├── list.go           # dotcor list
│       ├── status.go         # dotcor status
│       ├── sync.go           # dotcor sync (with diff preview)
│       ├── restore.go        # dotcor restore <file>
│       ├── history.go        # dotcor history <file>
│       ├── diff.go           # dotcor diff [file] (NEW)
│       ├── adopt.go          # dotcor adopt <file> (NEW)
│       ├── doctor.go         # dotcor doctor (health check)
│       ├── rebuild.go        # dotcor rebuild-config (NEW)
│       ├── clone.go          # dotcor clone <url>
│       └── cleanup.go        # dotcor cleanup-backups
│
├── internal/
│   ├── config/
│   │   ├── config.go         # Config struct, Load/Save operations
│   │   ├── paths.go          # Path normalization utilities
│   │   └── migrate.go        # Config version migrations (NEW)
│   │
│   ├── core/
│   │   ├── linker.go         # Symlink creation/removal logic
│   │   ├── validator.go      # File/path validation + secret detection
│   │   ├── backup.go         # Backup/restore operations
│   │   ├── lock.go           # File-based locking with stale detection
│   │   ├── transaction.go    # Transaction/rollback semantics (NEW)
│   │   └── ignore.go         # Ignore pattern matching (NEW)
│   │
│   ├── fs/
│   │   ├── fs.go             # File operations (move, copy)
│   │   └── symlink.go        # Cross-platform symlink handling (NO COPY FALLBACK)
│   │
│   └── git/
│       └── git.go            # Git command wrapper with diff support
│
├── go.mod
├── go.sum
├── PLAN.md                    # This file
└── README.md                  # Project documentation
```

---

## Data Model

### Config File Structure

**Location:** `~/.dotcor/config.yaml`

```yaml
version: "1.0"                   # Config schema version for migrations
repo_path: ~/.dotcor/files
git_enabled: true
git_remote: ""                   # Optional remote URL

# Files/patterns to never add (checked before adding)
ignore_patterns:
  - "*.log"
  - "*.swp"
  - "*_history"
  - ".env"
  - ".env.*"
  - "*.key"
  - "*.pem"
  - "id_rsa"
  - "id_ed25519"

managed_files:
  - source_path: ~/.zshrc
    repo_path: shell/zshrc
    added_at: 2025-01-04T10:30:00Z
    platforms: []                # Empty = all platforms, or ["darwin", "linux", "windows", "wsl"]
    has_uncommitted: false       # Track if add succeeded but commit failed

  - source_path: ~/.config/nvim/init.vim
    repo_path: nvim/init.vim
    added_at: 2025-01-04T10:31:00Z
    platforms: []
    has_uncommitted: false
```

### Directory Layout

**Location:** `~/.dotcor/`

```
~/.dotcor/
├── config.yaml              # Metadata: which files are managed
├── .lock                    # Lock file: {PID}\n{timestamp}\n{hostname}
├── backups/                 # Timestamped backups before destructive operations
│   ├── 2025-01-04_10-30-15/
│   │   └── zshrc
│   └── 2025-01-04_11-45-30/
│       └── gitconfig
└── files/                   # Git repository with actual dotfiles
    ├── .git/
    ├── shell/
    │   ├── zshrc            ← actual file
    │   └── bashrc           ← actual file
    └── nvim/
        └── init.vim         ← actual file

# System locations (RELATIVE symlinks):
~/.zshrc                     → .dotcor/files/shell/zshrc (relative)
~/.bashrc                    → .dotcor/files/shell/bashrc (relative)
~/.config/nvim/init.vim      → ../../.dotcor/files/nvim/init.vim (relative)
```

**Important:** Symlinks use **relative paths** for portability across machines, backups, and different mount points.

---

## Key Architectural Decisions

### 1. Relative vs Absolute Symlinks

**Decision:** Use relative symlinks

**Rationale:**
- Portable across machines with different home directory paths
- Survives backup/restore operations
- Works with different mount points
- No hardcoded absolute paths

**Implementation:** Use `filepath.Rel()` to compute relative path from symlink to target

### 2. Directory Management

**Decision:** Support individual files AND recursive directory adds via glob patterns

**Rationale:**
- Prevents accidental conflicts when multiple files in `~/.config/` need management
- Glob patterns provide flexibility: `dotcor add ~/.config/nvim/**/*.lua`
- Users can choose granularity

**Implementation:**
- `dotcor add ~/.zshrc` - single file
- `dotcor add ~/.config/nvim/*.lua` - glob pattern
- `dotcor add ~/.config/nvim --recursive` - all files in directory

### 3. Safety-First Operations with Transactions

**Decision:** Create backups AND use transaction/rollback semantics

**Rationale:**
- Backups protect against user mistakes
- Transactions protect against partial failures (power loss, disk full, etc.)
- Never leave system in broken state

**Implementation:**
- Backup to `~/.dotcor/backups/{timestamp}/`
- Transaction wrapper around all multi-step operations
- Automatic rollback on any failure

### 4. Concurrency Control with Stale Lock Detection

**Decision:** File-based locking with automatic stale lock detection

**Rationale:**
- Prevents config corruption from parallel operations
- Stale locks (from crashes) are automatically detected and clearable
- Simple implementation with `~/.dotcor/.lock`

**Implementation:**
- Lock file contains: PID, timestamp, hostname
- Check if process is alive using `os.FindProcess()` + signal test
- Offer to clear stale locks in error messages

### 5. Git Auto-Commit Error Handling

**Decision:** Never fail operations due to Git errors, but track uncommitted state

**Rationale:**
- User's files are more important than version control
- Users can always commit manually later
- Prominently warn about uncommitted changes

**Implementation:**
- Add `has_uncommitted` field to ManagedFile
- Show warnings in `dotcor status`
- Retry uncommitted files on `dotcor sync`

### 6. Windows: No Copy Fallback

**Decision:** REQUIRE symlink support on Windows (Developer Mode or Admin)

**Rationale:**
- Copy mode breaks the fundamental contract (edits don't sync)
- Silent data loss is worse than explicit failure
- Developer Mode is easy to enable on Windows 10+

**Implementation:**
- Detect symlink support on startup
- If not supported, show clear error with instructions
- Exit gracefully rather than provide broken fallback

### 7. Versioned Config for Future Migrations

**Decision:** Include version field in config for schema migrations

**Rationale:**
- Future changes to config structure need migration path
- Users shouldn't lose data on upgrades
- Clear error messages for version mismatches

**Implementation:**
- Current version: `"1.0"`
- Migration functions for each version bump
- Automatic backup before migration

### 8. Security: Ignore Patterns and Secret Detection

**Decision:** Prevent accidental commit of secrets and unwanted files

**Rationale:**
- Easy to accidentally commit `.env` or API keys
- Better to block at add time than discover later
- Provide clear warnings and escape hatch

**Implementation:**
- Default ignore patterns for common secrets
- Content scanning for secret patterns (API keys, tokens, private keys)
- Warnings with confirmation prompts

---

## Implementation Phases

### Phase 1: Core Infrastructure (Build First)

#### 1.1 Config Management (`internal/config/config.go`)

**Data structures:**

```go
package config

import "time"

const CurrentConfigVersion = "1.0"

type Config struct {
    Version         string        `yaml:"version"`           // Schema version
    RepoPath        string        `yaml:"repo_path"`         // ~/.dotcor/files
    GitEnabled      bool          `yaml:"git_enabled"`
    GitRemote       string        `yaml:"git_remote"`
    IgnorePatterns  []string      `yaml:"ignore_patterns"`   // Files to never add
    ManagedFiles    []ManagedFile `yaml:"managed_files"`
}

type ManagedFile struct {
    SourcePath      string    `yaml:"source_path"`       // ~/.zshrc (normalized, with ~)
    RepoPath        string    `yaml:"repo_path"`         // shell/zshrc (relative to files/)
    AddedAt         time.Time `yaml:"added_at"`
    Platforms       []string  `yaml:"platforms"`         // ["darwin", "linux"] or empty for all
    HasUncommitted  bool      `yaml:"has_uncommitted"`   // Track if Git commit failed
}
```

**Required functions:**

```go
// LoadConfig loads config from ~/.dotcor/config.yaml
// Returns default config if file doesn't exist
// Handles version migrations automatically
func LoadConfig() (*Config, error)

// SaveConfig atomically writes config to ~/.dotcor/config.yaml
// Uses write-to-temp + rename for atomicity
func (c *Config) SaveConfig() error

// AddManagedFile adds a new managed file
func (c *Config) AddManagedFile(mf ManagedFile) error

// RemoveManagedFile removes a managed file by source path
func (c *Config) RemoveManagedFile(sourcePath string) error

// GetManagedFile retrieves managed file by source path
func (c *Config) GetManagedFile(sourcePath string) (*ManagedFile, error)

// IsManaged checks if a file is already managed
func (c *Config) IsManaged(sourcePath string) bool

// GetManagedFilesForPlatform returns files that should be linked on current platform
func (c *Config) GetManagedFilesForPlatform() []ManagedFile

// MarkAsUncommitted marks a file as having uncommitted changes
func (c *Config) MarkAsUncommitted(sourcePath string) error

// GetUncommittedFiles returns all files with uncommitted changes
func (c *Config) GetUncommittedFiles() []ManagedFile

// GetDefaultIgnorePatterns returns sensible defaults for ignore patterns
func GetDefaultIgnorePatterns() []string
```

**Default ignore patterns:**

```go
var defaultIgnorePatterns = []string{
    // Secrets
    "*.key", "*.pem", "*.p12", "*.pfx",
    ".env", ".env.*",
    "id_rsa", "id_rsa.*", "id_ed25519", "id_ed25519.*",
    "*.ppk", // PuTTY private keys

    // History files
    "*_history", ".lesshst", ".sh_history",

    // Logs
    "*.log",

    // Temporary/swap files
    "*.swp", "*.swo", "*~", ".*.swp",

    // System files
    ".DS_Store", "Thumbs.db",
}
```

**Implementation notes:**
- Use Viper for YAML parsing
- Default repo path: `~/.dotcor/files`
- Handle missing/corrupted config gracefully (offer rebuild)
- Atomic save: write to temp file, then rename
- Platform detection: use `GetCurrentPlatform()` helper

---

#### 1.2 Config Migrations (`internal/config/migrate.go`) - NEW

**Required functions:**

```go
// MigrateConfig migrates config from old version to current
// Returns error if migration fails
// Creates backup before migration
func MigrateConfig(config *Config) (*Config, error)

// IsCompatibleVersion checks if config version is compatible
func IsCompatibleVersion(version string) bool

// GetMigrationPath returns list of migrations needed
func GetMigrationPath(fromVersion, toVersion string) []MigrationFunc

type MigrationFunc func(*Config) error

// Migration functions for each version bump
func migrateV1ToV2(config *Config) error
// ... future migrations
```

---

#### 1.3 Path Utilities (`internal/config/paths.go`)

**Required functions:**

```go
// NormalizePath converts absolute path to ~ notation
// Example: /Users/you/.zshrc -> ~/.zshrc
func NormalizePath(path string) (string, error)

// ExpandPath converts ~ notation to absolute path
// Example: ~/.zshrc -> /Users/you/.zshrc
// Also handles environment variables: $XDG_CONFIG_HOME, %APPDATA%, etc.
func ExpandPath(path string) (string, error)

// GetRepoFilePath returns full path to file in repo
// Example: shell/zshrc -> /Users/you/.dotcor/files/shell/zshrc
func GetRepoFilePath(config *Config, repoPath string) (string, error)

// GenerateRepoPath creates repo path from source path with optional override
// Example: ~/.config/nvim/init.vim -> nvim/init.vim
// Example: ~/.zshrc -> shell/zshrc
// customPath parameter allows manual override (e.g., "custom/myshell/zshrc")
func GenerateRepoPath(sourcePath string, customPath string) (string, error)

// GetCurrentPlatform returns current OS identifier
// Returns: "darwin", "linux", "windows", "wsl"
// Detects WSL by checking /proc/version for "Microsoft"
func GetCurrentPlatform() string

// ShouldApplyOnPlatform checks if file should be linked on current platform
func ShouldApplyOnPlatform(platforms []string) bool

// ComputeRelativeSymlink computes relative path from symlink to target
// Example: link=~/.zshrc, target=~/.dotcor/files/shell/zshrc
//          returns: .dotcor/files/shell/zshrc
// Validates both paths are on same filesystem
func ComputeRelativeSymlink(linkPath, targetPath string) (string, error)

// ExpandGlob expands glob pattern to list of files
// Example: ~/.config/nvim/*.lua -> [~/.config/nvim/init.lua, ~/.config/nvim/plugins.lua]
func ExpandGlob(pattern string) ([]string, error)
```

**Repo path generation algorithm:**

```
Input: ~/.zshrc
1. Strip home directory: .zshrc
2. Check custom path override (if provided, use it)
3. Check category map for known dotfiles
4. If found in map: use mapped category + stripped name
5. If not found: use "misc" category
6. Output: shell/zshrc (or misc/zshrc if unknown)

Input: ~/.config/nvim/init.vim
1. Strip home directory: .config/nvim/init.vim
2. Check custom path override (if provided, use it)
3. Strip .config/ prefix: nvim/init.vim
4. Output: nvim/init.vim

Input: ~/.local/share/applications/foo.desktop
1. Strip home directory: .local/share/applications/foo.desktop
2. Check custom path override (if provided, use it)
3. Preserve structure: local/share/applications/foo.desktop
4. Output: local/share/applications/foo.desktop
```

**Category mapping:**

```go
var categoryMap = map[string]string{
    // Shell configurations
    ".zshrc":         "shell",
    ".zshenv":        "shell",
    ".zprofile":      "shell",
    ".zsh_history":   "shell",
    ".bashrc":        "shell",
    ".bash_profile":  "shell",
    ".bash_history":  "shell",
    ".profile":       "shell",

    // Git
    ".gitconfig":     "git",
    ".gitignore":     "git",
    ".gitignore_global": "git",

    // Editors
    ".vimrc":         "vim",
    ".vim":           "vim",
    ".nvimrc":        "nvim",

    // Terminal multiplexers
    ".tmux.conf":     "tmux",
    ".screenrc":      "screen",
}

// GetCategoryForFile uses both exact match and prefix matching
func GetCategoryForFile(filename string) string {
    // Exact match first
    if cat, ok := categoryMap[filename]; ok {
        return cat
    }

    // Prefix matching for patterns
    if strings.HasPrefix(filename, ".zsh") {
        return "shell"
    }
    if strings.HasPrefix(filename, ".bash") {
        return "shell"
    }

    // Default category for unknown files
    return "misc"
}
```

**Implementation notes:**
- Use `os.UserHomeDir()` for home directory
- Use `filepath.Clean()` to normalize separators
- Support env variables: `os.ExpandEnv()` handles $VAR and %VAR%
- **WSL detection:** Check `/proc/version` contains "Microsoft" or "WSL"
- Use `filepath.Rel()` for computing relative symlink paths
- Validate paths are on same filesystem before computing relative path

---

#### 1.4 Symlink Operations (`internal/fs/symlink.go`)

**Required functions:**

```go
// CreateSymlink creates a RELATIVE symlink from link to target
// Returns error if symlink fails (NO COPY FALLBACK)
func CreateSymlink(target, link string) error

// RemoveSymlink removes a symlink (validates it's actually a symlink first)
func RemoveSymlink(link string) error

// IsSymlink checks if path is a symlink
func IsSymlink(path string) (bool, error)

// ReadSymlink reads the target of a symlink (returns raw target, may be relative)
func ReadSymlink(link string) (string, error)

// IsValidSymlink checks if symlink exists and points to existing target
// Resolves relative paths to check target existence
func IsValidSymlink(link string) (bool, error)

// SupportsSymlinks checks if current platform supports symlinks
// Windows: requires admin rights or developer mode
// Returns true on macOS/Linux, checks on Windows
func SupportsSymlinks() (bool, error)

// GetSymlinkStatus returns detailed status of a symlink
func GetSymlinkStatus(linkPath string, expectedTarget string) (SymlinkStatus, error)

type SymlinkStatus struct {
    Exists        bool
    IsSymlink     bool
    TargetExists  bool
    PointsToRepo  bool
    IsRelative    bool     // NEW: check if symlink is relative
    ActualTarget  string
}
```

**Symlink creation (NO COPY FALLBACK):**

```go
func CreateSymlink(target, link string) error {
    // Check if platform supports symlinks
    supported, err := SupportsSymlinks()
    if err != nil || !supported {
        return fmt.Errorf("symlinks not supported on this platform. %w", ErrSymlinkUnsupported)
    }

    // Ensure parent directory exists
    if err := EnsureDir(filepath.Dir(link)); err != nil {
        return err
    }

    // Compute RELATIVE path from link to target
    relPath, err := ComputeRelativeSymlink(link, target)
    if err != nil {
        return fmt.Errorf("computing relative path: %w", err)
    }

    // Create symlink with RELATIVE path
    err = os.Symlink(relPath, link)
    if err != nil {
        return fmt.Errorf("creating symlink: %w", err)
    }
    return nil
}

var ErrSymlinkUnsupported = errors.New("symlink support required - enable Developer Mode on Windows")
```

**Windows symlink detection:**

```go
func SupportsSymlinks() (bool, error) {
    if runtime.GOOS != "windows" {
        return true, nil
    }

    // On Windows, test by creating a temporary symlink
    tmpDir := os.TempDir()
    testFile := filepath.Join(tmpDir, "dotcor_test_file")
    testLink := filepath.Join(tmpDir, "dotcor_test_link")

    // Create test file
    if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
        return false, err
    }
    defer os.Remove(testFile)

    // Try to create symlink
    err := os.Symlink(testFile, testLink)
    if err != nil {
        return false, nil
    }
    os.Remove(testLink)
    return true, nil
}
```

---

#### 1.5 File Operations (`internal/fs/fs.go`)

**Required functions:**

```go
// MoveFile moves a file from src to dst, preserving permissions
func MoveFile(src, dst string) error

// CopyFile copies file with permissions and metadata preserved
func CopyFile(src, dst string) error

// CopyWithPermissions copies file preserving all metadata
func CopyWithPermissions(src, dst string) error

// FileExists checks if file exists
func FileExists(path string) bool

// EnsureDir creates directory if it doesn't exist (including parents)
func EnsureDir(path string) error

// IsDirectory checks if path is a directory
func IsDirectory(path string) (bool, error)

// GetFileSize returns file size in bytes
func GetFileSize(path string) (int64, error)

// RemoveFile removes a file or directory
func RemoveFile(path string) error

// GetFilesRecursive returns all files in directory recursively
func GetFilesRecursive(dir string) ([]string, error)
```

**Implementation notes:**
- Preserve file permissions: use `os.Chmod()`
- Preserve timestamps: use `os.Chtimes()`
- Create parent directories: `os.MkdirAll()`
- Use `io.Copy` for efficient file copying

---

#### 1.6 Validation (`internal/core/validator.go`)

**Required functions:**

```go
// ValidateSourceFile checks if source file is valid for adding
func ValidateSourceFile(path string, config *Config) error

// ValidateRepoPath checks if repo path is valid
func ValidateRepoPath(path string) error

// ValidateNotAlreadyManaged checks if file is not already managed
func ValidateNotAlreadyManaged(config *Config, sourcePath string) error

// ValidateNotInDotcorDir checks file isn't inside ~/.dotcor/ (circular)
func ValidateNotInDotcorDir(path string, config *Config) error

// ValidateFileSize checks file isn't unreasonably large (>100MB warning)
func ValidateFileSize(path string) error

// DetectSecrets scans file content for potential secrets (NEW)
func DetectSecrets(path string) (warnings []string, err error)

// ShouldWarnAboutSecrets returns true if file likely contains secrets
func ShouldWarnAboutSecrets(path string, warnings []string) bool
```

**Secret detection patterns:**

```go
var secretPatterns = []string{
    `(?i)api[_-]?key\s*[:=]\s*['"]?[a-zA-Z0-9]{20,}['"]?`,
    `(?i)password\s*[:=]\s*['"]?[^\s'";]{8,}['"]?`,
    `(?i)secret\s*[:=]\s*['"]?[a-zA-Z0-9]{20,}['"]?`,
    `(?i)token\s*[:=]\s*['"]?[a-zA-Z0-9]{20,}['"]?`,
    `-----BEGIN .*PRIVATE KEY-----`,
    `(?i)(aws|azure|gcp).*secret`,
}

func DetectSecrets(path string) (warnings []string, err error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    for _, pattern := range secretPatterns {
        re := regexp.MustCompile(pattern)
        if matches := re.FindAllString(string(content), -1); len(matches) > 0 {
            for _, match := range matches {
                // Truncate match if too long
                if len(match) > 50 {
                    match = match[:50] + "..."
                }
                warnings = append(warnings, match)
            }
        }
    }
    return warnings, nil
}
```

**Validation rules:**

**Source file validation:**
- File must exist
- File must be readable
- Must be a regular file (not a directory)
- Path must be absolute or start with ~ or contain env variables
- Must not already be a symlink pointing to our repo
- Must not already be a symlink pointing elsewhere (can adopt it)
- Must not be inside `~/.dotcor/` (circular reference)
- File size should be reasonable (<100MB, warn if larger)
- Check against ignore patterns
- Scan for secrets (warn if found)

**Repo path validation:**
- Must be relative path (no leading /)
- Must not contain `..` (path traversal)
- Must not contain absolute path components
- Must not be empty

---

#### 1.7 Ignore Patterns (`internal/core/ignore.go`) - NEW

**Required functions:**

```go
// ShouldIgnore checks if file matches any ignore patterns
func ShouldIgnore(path string, patterns []string) (bool, string)

// MatchesPattern checks if path matches glob pattern
func MatchesPattern(path, pattern string) bool

// LoadGitignorePatterns loads patterns from .gitignore-style file (future)
func LoadGitignorePatterns(path string) ([]string, error)
```

**Implementation:**

```go
func ShouldIgnore(path string, patterns []string) (bool, string) {
    filename := filepath.Base(path)

    for _, pattern := range patterns {
        matched, err := filepath.Match(pattern, filename)
        if err != nil {
            continue
        }
        if matched {
            return true, pattern
        }

        // Also check full path for patterns like ".env.*"
        matched, err = filepath.Match(pattern, path)
        if err != nil {
            continue
        }
        if matched {
            return true, pattern
        }
    }
    return false, ""
}
```

---

#### 1.8 Backup Operations (`internal/core/backup.go`)

**Required functions:**

```go
// CreateBackup creates a timestamped backup of a file before destructive operations
// Returns backup path and error
func CreateBackup(sourcePath string) (backupPath string, err error)

// RestoreBackup restores a file from backup
func RestoreBackup(backupPath string, targetPath string) error

// CleanOldBackups removes backups older than specified duration
// Default: keep last 10 backups
func CleanOldBackups(olderThan time.Duration, keepLast int) error

// ListBackups returns list of all backups with timestamps
func ListBackups() ([]BackupInfo, error)

// GetBackupDir returns backup directory path (~/.dotcor/backups)
func GetBackupDir() (string, error)

type BackupInfo struct {
    Timestamp   time.Time
    SourcePath  string
    BackupPath  string
    Size        int64
}
```

**Implementation notes:**
- Backup directory: `~/.dotcor/backups/{timestamp}/`
- Timestamp format: `2006-01-02_15-04-05` (sortable, filesystem-safe)
- Preserve file permissions and metadata in backups
- Always backup before: `add`, `remove`, `restore`
- Never fail operations if backup fails, but warn prominently

---

#### 1.9 Locking with Stale Detection (`internal/core/lock.go`)

**Required functions:**

```go
// AcquireLock acquires file-based lock for dotcor operations
// Returns error if lock is already held
func AcquireLock() error

// ReleaseLock releases the file lock
func ReleaseLock() error

// WithLock executes a function while holding the lock
// Automatically releases lock on completion or panic
func WithLock(fn func() error) error

// IsLocked checks if lock is currently held
func IsLocked() (bool, error)

// IsStale checks if lock file is stale (process dead)
func IsStale(lockPath string) (bool, error)

// ClearStaleLock removes stale lock file
func ClearStaleLock() error
```

**Lock file format:**

```
{PID}
{timestamp}
{hostname}
```

**Stale lock detection:**

```go
func IsStale(lockPath string) (bool, error) {
    content, err := os.ReadFile(lockPath)
    if err != nil {
        return false, err
    }

    lines := strings.Split(string(content), "\n")
    if len(lines) < 3 {
        return true, nil // Malformed lock file
    }

    pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
    if err != nil {
        return true, nil // Invalid PID
    }

    // Check if process exists
    process, err := os.FindProcess(pid)
    if err != nil {
        return true, nil // Process doesn't exist
    }

    // On Unix, try to signal process 0 (doesn't kill it, just checks if alive)
    err = process.Signal(syscall.Signal(0))
    if err != nil {
        return true, nil // Process is dead
    }

    // Process is alive, lock is valid
    return false, nil
}
```

**Implementation notes:**
- Lock file: `~/.dotcor/.lock`
- Use `syscall.Flock()` on Unix systems for atomic locking
- On Windows, use create/delete lock file pattern with retry
- Lock timeout: 30 seconds, then error
- Automatically release on process exit (defer pattern)
- Clear error message when locked:
  ```
  Another dotcor operation is in progress (PID 1234).
  If this is a stale lock, run: dotcor doctor --fix
  ```

---

#### 1.10 Transaction/Rollback (`internal/core/transaction.go`) - NEW

**Required functions:**

```go
// Transaction represents a sequence of operations that can be rolled back
type Transaction struct {
    operations []Operation
    rollbacks  []func() error
}

type Operation interface {
    Do() error        // Execute the operation
    Undo() error      // Rollback the operation
    Describe() string // Human-readable description
}

// NewTransaction creates a new transaction
func NewTransaction() *Transaction

// Execute runs an operation and registers its rollback
func (t *Transaction) Execute(op Operation) error

// Rollback undoes all executed operations in reverse order
func (t *Transaction) Rollback() error

// Commit marks transaction as successful (clears rollback list)
func (t *Transaction) Commit()

// Common operations
type MoveFileOp struct {
    Src, Dst string
}

type CreateSymlinkOp struct {
    Target, Link string
}

type AddToConfigOp struct {
    Config *Config
    File   ManagedFile
}
```

**Example usage in `add` command:**

```go
func runAdd(sourcePath string) error {
    tx := NewTransaction()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
            panic(r)
        }
    }()

    // 1. Create backup (not rolled back - we keep backups)
    backupPath, err := CreateBackup(sourcePath)
    if err != nil {
        return err
    }

    // 2. Move file to repo
    if err := tx.Execute(&MoveFileOp{sourcePath, repoPath}); err != nil {
        return err
    }

    // 3. Create symlink
    if err := tx.Execute(&CreateSymlinkOp{repoPath, sourcePath}); err != nil {
        return err
    }

    // 4. Add to config
    if err := tx.Execute(&AddToConfigOp{config, managedFile}); err != nil {
        return err
    }

    // 5. Git commit (non-critical, don't rollback for this)
    if err := git.AutoCommit("Add " + sourcePath); err != nil {
        // Mark as uncommitted, but don't fail
        config.MarkAsUncommitted(sourcePath)
    }

    // Success - commit transaction (clears rollback)
    tx.Commit()
    return nil
}
```

**Implementation for operations:**

```go
func (op *MoveFileOp) Do() error {
    return os.Rename(op.Src, op.Dst)
}

func (op *MoveFileOp) Undo() error {
    return os.Rename(op.Dst, op.Src)
}

func (op *CreateSymlinkOp) Do() error {
    return CreateSymlink(op.Target, op.Link)
}

func (op *CreateSymlinkOp) Undo() error {
    return os.Remove(op.Link)
}

func (op *AddToConfigOp) Do() error {
    return op.Config.AddManagedFile(op.File)
}

func (op *AddToConfigOp) Undo() error {
    return op.Config.RemoveManagedFile(op.File.SourcePath)
}
```

---

#### 1.11 Git Wrapper (`internal/git/git.go`)

**Required functions:**

```go
// InitRepo initializes git repository in directory
func InitRepo(repoPath string) error

// AutoCommit stages all changes and commits with message
// Returns error but operations should NOT fail if commit fails
func AutoCommit(repoPath, message string) error

// Sync commits all changes and pushes to remote (if configured)
func Sync(repoPath string) error

// HasChanges checks if working tree has uncommitted changes
func HasChanges(repoPath string) (bool, error)

// SetRemote configures git remote
func SetRemote(repoPath, remoteName, remoteURL string) error

// GetStatus returns git status information
func GetStatus(repoPath string) (StatusInfo, error)

// GetFileHistory returns git log for specific file
func GetFileHistory(repoPath, filePath string, limit int) ([]CommitInfo, error)

// RestoreFile restores file from git history
func RestoreFile(repoPath, filePath, ref string) error

// IsGitInstalled checks if git command is available
func IsGitInstalled() bool

// GetRemoteURL returns configured remote URL, or empty if none
func GetRemoteURL(repoPath string) (string, error)

// GetDiff returns unified diff for uncommitted changes (NEW)
func GetDiff(repoPath string) (string, error)

// GetFileDiff returns diff for specific file (NEW)
func GetFileDiff(repoPath, filePath string) (string, error)

type StatusInfo struct {
    HasUncommitted bool
    AheadBy        int
    BehindBy       int
    Branch         string
    RemoteExists   bool
}

type CommitInfo struct {
    Hash      string
    Author    string
    Date      time.Time
    Message   string
}
```

**Diff implementation:**

```go
func GetDiff(repoPath string) (string, error) {
    cmd := exec.Command("git", "diff", "HEAD")
    cmd.Dir = repoPath
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("git diff failed: %w", err)
    }
    return string(output), nil
}

func GetFileDiff(repoPath, filePath string) (string, error) {
    cmd := exec.Command("git", "diff", "HEAD", "--", filePath)
    cmd.Dir = repoPath
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("git diff failed: %w", err)
    }
    return string(output), nil
}
```

**Implementation notes:**
- Use `os/exec.Command("git", ...)`
- Run commands in repo directory with `cmd.Dir = repoPath`
- Capture stderr for error messages
- Check if git is installed at startup
- Auto-commit should be silent on success
- On auto-commit failure, mark as uncommitted but don't fail operation

---

### Phase 2: Commands (Implement in Order)

#### 2.1 `dotcor init` (`cmd/dotcor/init.go`)

**What it does:**
1. Check symlink support (fail if not available)
2. Acquire lock
3. Check if `~/.dotcor` already exists (prevent re-init)
4. If `--interactive`, scan for existing dotfiles and offer to add them
5. Create directory structure:
   - `~/.dotcor/`
   - `~/.dotcor/files/`
   - `~/.dotcor/backups/`
6. Initialize Git repository in `~/.dotcor/files/`
7. Create default `config.yaml` with version and default ignore patterns
8. Optionally create symlinks if config already exists (from clone)
9. Release lock
10. Success message

**Command definition:**

```go
var initCmd = &cobra.Command{
    Use:   "init",
    Short: "Initialize DotCor repository",
    Long:  `Creates ~/.dotcor directory structure and initializes Git repository.`,
    Run:   runInit,
}

func init() {
    initCmd.Flags().Bool("apply", false, "Create symlinks from existing config (for new machine setup)")
    initCmd.Flags().Bool("interactive", false, "Interactively select existing dotfiles to add")
}
```

**Interactive mode:**

```
$ dotcor init --interactive

Checking for existing dotfiles in your home directory...

Found 12 common dotfiles:
  [x] ~/.zshrc
  [x] ~/.bashrc
  [x] ~/.gitconfig
  [ ] ~/.vimrc
  [x] ~/.tmux.conf
  [ ] ~/.ssh/config (ignored - contains secrets)

Use arrow keys to toggle, Enter to continue, Ctrl+C to skip.

Add 4 selected files? [Y/n]: y

✓ Created ~/.dotcor/
✓ Initialized Git repository
✓ Adding selected files...
  ✓ ~/.zshrc → shell/zshrc
  ✓ ~/.bashrc → shell/bashrc
  ✓ ~/.gitconfig → git/gitconfig
  ✓ ~/.tmux.conf → tmux/tmux.conf
✓ Committed to Git

DotCor setup complete! 4 dotfiles managed.
```

**Common dotfiles to scan:**

```go
var commonDotfiles = []string{
    "~/.zshrc",
    "~/.bashrc",
    "~/.bash_profile",
    "~/.gitconfig",
    "~/.vimrc",
    "~/.tmux.conf",
    "~/.config/nvim/init.vim",
    "~/.config/alacritty/alacritty.yml",
    "~/.config/kitty/kitty.conf",
}
```

**Error handling:**
- Symlinks not supported:
  ```
  ✗ Symlinks not supported on this platform.

  Windows users: Enable Developer Mode
    Settings → Update & Security → For developers → Developer Mode

  Then restart your terminal and try again.
  ```
- Already initialized: "DotCor is already initialized at ~/.dotcor. Use 'dotcor status' to check."
- Git not installed: "⚠ Git not found. Installing Git is recommended for version control."

---

#### 2.2 `dotcor add <file>` (`cmd/dotcor/add.go`)

**What it does:**
1. Expand glob patterns if provided
2. For each file:
   a. Acquire lock
   b. Validate file (exists, not directory, not ignored, check secrets)
   c. Check if already managed
   d. Generate repo path (or use --path override)
   e. Start transaction
   f. Create backup
   g. Move file to repo
   h. Create relative symlink
   i. Add to config
   j. Commit transaction
   k. Git commit (mark uncommitted if fails)
   l. Release lock
   m. Success message

**Command definition:**

```go
var addCmd = &cobra.Command{
    Use:   "add [file]...",
    Short: "Add dotfiles to DotCor",
    Long:  `Add dotfiles to DotCor. Supports glob patterns like ~/.config/nvim/*.lua`,
    Args:  cobra.MinimumNArgs(1),
    Run:   runAdd,
}

func init() {
    addCmd.Flags().String("path", "", "Custom repo path (e.g., 'custom/myconfig')")
    addCmd.Flags().StringSlice("platforms", []string{}, "Platform restrictions (darwin, linux, windows, wsl)")
    addCmd.Flags().Bool("recursive", false, "Add all files in directory recursively")
    addCmd.Flags().Bool("force", false, "Skip confirmation prompts (use with caution)")
}
```

**Glob pattern support:**

```bash
# Single file
dotcor add ~/.zshrc

# Glob pattern
dotcor add ~/.config/nvim/*.lua

# Multiple patterns
dotcor add ~/.zsh* ~/.bash*

# Recursive
dotcor add ~/.config/nvim --recursive

# Will expand to: ~/.config/nvim/init.lua, ~/.config/nvim/plugins.lua, etc.
```

**Secret detection warning:**

```
$ dotcor add ~/.zshrc

⚠ Potential secrets detected:
  Line 15: API_KEY=abc123...
  Line 42: password='secr...

This file may contain sensitive data.
Options:
  1. Cancel and remove secrets
  2. Add to ignore patterns and cancel
  3. Continue anyway (not recommended)

Choice [1]:
```

**Ignore pattern match:**

```
$ dotcor add ~/.env

✗ File matches ignore pattern: ".env"

Files matching this pattern are typically secrets.
To add anyway, use: dotcor add ~/.env --force
```

**Output example:**

```
$ dotcor add ~/.zshrc

✓ Backed up to ~/.dotcor/backups/2025-01-04_10-30-15/zshrc
✓ Added ~/.zshrc
  Moved to: ~/.dotcor/files/shell/zshrc
  Symlink: ~/.zshrc → .dotcor/files/shell/zshrc (relative)
✓ Committed to Git

$ dotcor add ~/.config/nvim/*.lua

Found 3 files to add:
  ~/.config/nvim/init.lua
  ~/.config/nvim/plugins.lua
  ~/.config/nvim/keymaps.lua

✓ Added 3 files
✓ Committed to Git
```

---

#### 2.3 `dotcor adopt <file>` (`cmd/dotcor/adopt.go`) - NEW

**What it does:**
1. Check if file is already a symlink
2. Resolve symlink target
3. Validate target is not already in our repo
4. Copy target file into repo
5. Update symlink to point to repo (make it relative)
6. Add to config
7. Git commit

**Command definition:**

```go
var adoptCmd = &cobra.Command{
    Use:   "adopt [file]",
    Short: "Adopt an existing symlink into DotCor management",
    Long:  `Takes an existing symlink and brings it under DotCor management.`,
    Args:  cobra.ExactArgs(1),
    Run:   runAdopt,
}
```

**Output example:**

```
$ dotcor adopt ~/.zshrc

Existing symlink: ~/.zshrc → ~/dotfiles/zshrc

✓ Copied ~/dotfiles/zshrc to ~/.dotcor/files/shell/zshrc
✓ Updated symlink to relative path
✓ Added to config
✓ Committed to Git

Your dotfile is now managed by DotCor!
```

---

#### 2.4 `dotcor list` (`cmd/dotcor/list.go`)

**What it does:**
1. Load config
2. Display managed files in table format
3. Show count, platform info, and uncommitted status
4. Highlight files with uncommitted changes

**Command definition:**

```go
var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List all managed dotfiles",
    Aliases: []string{"ls"},
    Run:   runList,
}

func init() {
    listCmd.Flags().Bool("uncommitted", false, "Show only files with uncommitted changes")
}
```

**Output example:**

```
Managed dotfiles (3):

SOURCE PATH                     REPO PATH              ADDED AT          PLATFORMS   STATUS
~/.zshrc                        shell/zshrc            Jan 04 10:30      all         ✓
~/.config/nvim/init.vim         nvim/init.vim          Jan 04 10:31      all         ✓
~/Library/Preferences/foo.plist foo.plist              Jan 04 10:32      darwin      ⚠ uncommitted

Legend:
  ✓ = committed to Git
  ⚠ = uncommitted changes (run 'dotcor sync')
```

---

#### 2.5 `dotcor diff` (`cmd/dotcor/diff.go`) - NEW

**What it does:**
1. If file specified: show diff for that file
2. If no file: show diff for all uncommitted changes
3. Use git diff under the hood
4. Colorize output if terminal supports it

**Command definition:**

```go
var diffCmd = &cobra.Command{
    Use:   "diff [file]",
    Short: "Show uncommitted changes",
    Long:  `Show git diff for uncommitted changes. If file specified, show only that file.`,
    Args:  cobra.MaximumNArgs(1),
    Run:   runDiff,
}

func init() {
    diffCmd.Flags().Bool("stat", false, "Show diffstat instead of full diff")
}
```

**Output example:**

```
$ dotcor diff

diff --git a/shell/zshrc b/shell/zshrc
index abc123..def456 100644
--- a/shell/zshrc
+++ b/shell/zshrc
@@ -10,0 +11 @@
+alias ll='ls -la'

$ dotcor diff ~/.zshrc

# Shows diff for just ~/.zshrc
```

---

#### 2.6 `dotcor status` (`cmd/dotcor/status.go`)

**What it does:**
1. Load config
2. For each managed file:
   - Check symlink exists
   - Check target exists
   - Check if relative (warn if absolute)
   - Check if points to correct location
3. Show Git repository status with diff stat
4. Show files with uncommitted changes

**Output example:**

```
Symlinks:
✓ ~/.zshrc                 → .dotcor/files/shell/zshrc
✓ ~/.bashrc                → .dotcor/files/shell/bashrc
✗ ~/.vimrc                 → vim/vimrc (broken: target missing)
! ~/.config/nvim/init.vim  → (not linked - file exists but not a symlink)
⚠ ~/.tmux.conf             → /Users/you/.dotcor/files/tmux/tmux.conf (absolute - should be relative)

Repository:
● 2 uncommitted changes
  shell/zshrc (3+ 1-)
  git/gitconfig (5+ 2-)
↑ 1 commit ahead of origin/main
⚠ 1 file with uncommitted add:
  - ~/.gitconfig (add succeeded but commit failed)

Run 'dotcor diff' to see changes
Run 'dotcor sync' to commit and push
Run 'dotcor doctor' to check for issues
```

---

#### 2.7 `dotcor sync` (`cmd/dotcor/sync.go`)

**What it does:**
1. Acquire lock
2. Check for uncommitted files from failed adds
3. Show diff stats
4. Offer to show full diff
5. Confirm commit
6. Check for deleted files
7. Commit all changes
8. Push to remote (if configured)
9. Clear uncommitted flags
10. Release lock

**Command definition:**

```go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Commit all changes and push to remote",
    Run:   runSync,
}

func init() {
    syncCmd.Flags().Bool("no-push", false, "Commit but don't push to remote")
    syncCmd.Flags().Bool("force", false, "Skip diff preview and confirmation")
    syncCmd.Flags().Bool("message", "m", "", "Custom commit message")
}
```

**Output with diff preview:**

```
$ dotcor sync

Files changed:
  shell/zshrc (3+ 1-)
  git/gitconfig (5+ 2-)

Show diff? [y/N]: y

diff --git a/shell/zshrc b/shell/zshrc
+alias foo='bar'
...

Commit these changes? [Y/n]: y

✓ Committed: "Sync dotfiles - 2025-01-04 15:30"
✓ Pushed to origin/main
✓ Cleared uncommitted flags
```

---

#### 2.8 `dotcor remove <file>` (`cmd/dotcor/remove.go`)

**What it does:**
1. Acquire lock
2. Validate file is managed
3. Start transaction
4. Prompt: "Keep file at {path}? [Y/n]"
5. If yes: copy from repo to original location
6. Remove symlink
7. Prompt: "Delete from repository? [y/N]"
8. If yes: delete from repo
9. Remove from config
10. Commit transaction
11. Git commit
12. Release lock

**Command definition:**

```go
var removeCmd = &cobra.Command{
    Use:   "remove [file]",
    Short: "Stop managing a dotfile",
    Args:  cobra.ExactArgs(1),
    Run:   runRemove,
}

func init() {
    removeCmd.Flags().Bool("keep-file", false, "Keep file at source location (skip prompt)")
    removeCmd.Flags().Bool("delete-from-repo", false, "Delete from repository (skip prompt)")
}
```

**Output:**

```
$ dotcor remove ~/.zshrc

? Keep file at ~/.zshrc? [Y/n]: y
✓ Copied from repository to ~/.zshrc

? Delete from repository? [y/N]: n
✓ File kept in repository at shell/zshrc

✓ Removed from config
✓ Committed to Git
```

---

#### 2.9 `dotcor restore <file>` (`cmd/dotcor/restore.go`)

**What it does:**
1. Acquire lock
2. Validate file is managed
3. If --preview, show diff
4. Create backup of current version
5. Restore file from Git history
6. Release lock

**Command definition:**

```go
var restoreCmd = &cobra.Command{
    Use:   "restore [file]",
    Short: "Restore a dotfile from Git history",
    Args:  cobra.ExactArgs(1),
    Run:   runRestore,
}

func init() {
    restoreCmd.Flags().String("to", "HEAD", "Git reference to restore from")
    restoreCmd.Flags().Bool("preview", false, "Preview changes before restoring")
}
```

**With preview:**

```
$ dotcor restore ~/.zshrc --to=HEAD~5 --preview

Restoring would change:
- alias foo='bar'
+ alias foo='baz'

Proceed? [y/N]: y

✓ Backed up current version to ~/.dotcor/backups/2025-01-04_15-45-30/zshrc
✓ Restored ~/.zshrc from HEAD~5
```

---

#### 2.10 `dotcor history <file>` (`cmd/dotcor/history.go`)

**Command definition:**

```go
var historyCmd = &cobra.Command{
    Use:   "history [file]",
    Short: "Show Git history for a dotfile",
    Args:  cobra.ExactArgs(1),
    Run:   runHistory,
}

func init() {
    historyCmd.Flags().Int("n", 10, "Number of commits to show")
}
```

---

#### 2.11 `dotcor doctor` (`cmd/dotcor/doctor.go`)

**What it does:**
1. Check symlink support
2. Check directory structure
3. Validate config file
4. Check for stale locks
5. Validate all symlinks (relative, working, etc.)
6. Check Git status
7. Scan for issues
8. Offer to fix with --fix flag

**Command definition:**

```go
var doctorCmd = &cobra.Command{
    Use:   "doctor",
    Short: "Check DotCor installation health",
    Run:   runDoctor,
}

func init() {
    doctorCmd.Flags().Bool("fix", false, "Automatically fix issues where possible")
}
```

**Output:**

```
$ dotcor doctor

Running health checks...

✓ Symlink support enabled
✓ Directory structure valid
✓ Config file valid (version 1.0)
✓ No stale locks

Symlinks:
  ✓ 5 symlinks working correctly
  ✗ 1 broken symlink: ~/.vimrc → vim/vimrc (target missing)
  ⚠ 1 absolute symlink: ~/.tmux.conf (should be relative)

Repository:
  ⚠ 2 uncommitted changes
  ⚠ 1 file with uncommitted add: ~/.gitconfig

Issues found: 3
Suggestions:
  - Run 'dotcor remove ~/.vimrc' to clean up broken symlink
  - Recreate ~/.tmux.conf: dotcor remove ~/.tmux.conf && dotcor add ~/.tmux.conf
  - Run 'dotcor sync' to commit changes
```

---

#### 2.12 `dotcor rebuild-config` (`cmd/dotcor/rebuild.go`) - NEW

**What it does:**
1. Backup current config
2. Scan `~/.dotcor/files/` for all files
3. For each file, check if symlink exists in home directory
4. Reconstruct config from actual state
5. Validate and save new config

**Command definition:**

```go
var rebuildCmd = &cobra.Command{
    Use:   "rebuild-config",
    Short: "Rebuild config from actual repository and symlink state",
    Long:  `Useful if config.yaml is corrupted. Scans repo and home directory to reconstruct config.`,
    Run:   runRebuild,
}

func init() {
    rebuildCmd.Flags().Bool("dry-run", false, "Show what would be rebuilt without saving")
}
```

**Output:**

```
$ dotcor rebuild-config

⚠ This will replace your config file.
Backup will be created at: ~/.dotcor/config.yaml.backup

Scanning repository...
Found 5 files in ~/.dotcor/files/

Checking for symlinks...
  ✓ ~/.zshrc → shell/zshrc
  ✓ ~/.bashrc → shell/bashrc
  ✓ ~/.gitconfig → git/gitconfig
  ✗ shell/profile (no symlink found)
  ✓ ~/.tmux.conf → tmux/tmux.conf

Rebuilt config with 4 managed files (1 orphaned file).

? Save new config? [y/N]: y

✓ Backed up old config
✓ Saved new config
✓ Config rebuilt successfully

Orphaned files (in repo but not linked):
  - shell/profile

Use 'dotcor remove' or create symlink manually.
```

---

#### 2.13 `dotcor clone <url>` (`cmd/dotcor/clone.go`)

**Command definition:**

```go
var cloneCmd = &cobra.Command{
    Use:   "clone [url]",
    Short: "Clone dotfiles repository and set up DotCor",
    Args:  cobra.ExactArgs(1),
    Run:   runClone,
}
```

---

#### 2.14 `dotcor cleanup-backups` (`cmd/dotcor/cleanup.go`)

**Command definition:**

```go
var cleanupCmd = &cobra.Command{
    Use:   "cleanup-backups",
    Short: "Remove old backups",
    Run:   runCleanup,
}

func init() {
    cleanupCmd.Flags().Int("keep", 10, "Number of recent backups to keep")
    cleanupCmd.Flags().Bool("force", false, "Skip confirmation prompt")
}
```

---

### Phase 3: Testing & Validation

#### 3.1 Unit Testing Strategy

**Test coverage for each module:**

- All path utilities with edge cases
- Symlink creation and validation
- Transaction rollback scenarios
- Lock acquisition and stale detection
- Secret detection patterns
- Ignore pattern matching
- Config migrations
- Git operations with mocks

**Critical test scenarios:**

```go
// Test transaction rollback on failure
func TestTransactionRollback(t *testing.T) {
    // Simulate failure after 2 operations
    // Verify both operations are rolled back
}

// Test stale lock detection
func TestStaleLockDetection(t *testing.T) {
    // Create lock file with dead PID
    // Verify IsStale returns true
}

// Test secret detection
func TestSecretDetection(t *testing.T) {
    // Test various secret patterns
    // Verify all are caught
}

// Test relative symlink computation
func TestRelativeSymlinks(t *testing.T) {
    // Test various depth combinations
    // Verify correctness
}
```

---

#### 3.2 Integration Testing

**Key flows:**

1. Init → Add → Edit → Sync → Restore
2. Add with glob patterns
3. Adopt existing symlink
4. Transaction rollback on disk full
5. Stale lock recovery
6. Config rebuild after corruption
7. Secret detection and rejection
8. Interactive init with selections

---

#### 3.3 Cross-Platform Testing

**Windows:**
- Test Developer Mode detection
- Verify clear error when symlinks unavailable
- Test WSL detection

**macOS/Linux:**
- Verify relative symlinks work
- Test recursive add
- Test with various shells

---

## Development Order Checklist

**Infrastructure (build first):**
- [ ] `internal/config/config.go` - Config with version and ignore patterns
- [ ] `internal/config/migrate.go` - Version migrations
- [ ] `internal/config/paths.go` - Path utils with glob support
- [ ] `internal/fs/fs.go` - File operations
- [ ] `internal/fs/symlink.go` - Symlinks (NO COPY FALLBACK)
- [ ] `internal/core/validator.go` - Validation + secret detection
- [ ] `internal/core/ignore.go` - Ignore pattern matching
- [ ] `internal/core/backup.go` - Backup operations
- [ ] `internal/core/lock.go` - Locking with stale detection
- [ ] `internal/core/transaction.go` - Transaction/rollback
- [ ] `internal/git/git.go` - Git with diff support

**Commands (build in order):**
- [ ] `cmd/dotcor/main.go` - Cobra setup
- [ ] `cmd/dotcor/init.go` - Init with interactive mode
- [ ] `cmd/dotcor/add.go` - Add with glob support
- [ ] `cmd/dotcor/adopt.go` - Adopt existing symlinks
- [ ] `cmd/dotcor/list.go` - List managed files
- [ ] `cmd/dotcor/diff.go` - Show diffs
- [ ] `cmd/dotcor/status.go` - Status with validation
- [ ] `cmd/dotcor/sync.go` - Sync with preview
- [ ] `cmd/dotcor/remove.go` - Remove files
- [ ] `cmd/dotcor/restore.go` - Restore with preview
- [ ] `cmd/dotcor/history.go` - Show history
- [ ] `cmd/dotcor/doctor.go` - Health check
- [ ] `cmd/dotcor/rebuild.go` - Rebuild config
- [ ] `cmd/dotcor/clone.go` - Clone shortcut
- [ ] `cmd/dotcor/cleanup.go` - Cleanup backups

**Testing:**
- [ ] Unit tests for all modules
- [ ] Integration tests for key flows
- [ ] Transaction rollback tests
- [ ] Secret detection tests
- [ ] Cross-platform testing
- [ ] Concurrent operation tests

---

## Security Considerations

**Secrets:**
- Default ignore patterns for common secret files
- Content scanning for secret patterns
- Warnings with confirmation before adding suspicious files
- Git history scanning tool (future)

**File permissions:**
- Preserve permissions in backups and copies
- Warn about unusual permissions (777, etc.)

**Lock security:**
- Include PID and hostname in lock file
- Verify process ownership before clearing locks

**Git remote:**
- Warn if remote URL is HTTP (suggest HTTPS/SSH)
- Support SSH key-based authentication

---

## Resources

- [Cobra Documentation](https://github.com/spf13/cobra)
- [Viper Documentation](https://github.com/spf13/viper)
- [Go filepath](https://pkg.go.dev/path/filepath)
- [Go os](https://pkg.go.dev/os)
- [Testing in Go](https://go.dev/doc/tutorial/add-a-test)

---

## Architecture Decision Records

**ADR-001: Use relative symlinks**
- **Rationale:** Portability across machines and mount points
- **Trade-off:** More complex path computation

**ADR-002: Individual files with glob support**
- **Rationale:** Flexibility while preventing conflicts
- **Trade-off:** Users must be explicit about what to add

**ADR-003: Never fail on Git errors**
- **Rationale:** Files more important than version control
- **Trade-off:** Need to track uncommitted state

**ADR-004: File-based locking with stale detection**
- **Rationale:** Prevents corruption, graceful recovery
- **Trade-off:** Extra complexity for stale detection

**ADR-005: Automatic backups + transaction rollback**
- **Rationale:** Multiple safety layers
- **Trade-off:** Disk space usage, slightly slower operations

**ADR-006: NO Windows copy mode fallback**
- **Rationale:** Copy mode breaks the contract (edits don't sync)
- **Trade-off:** Windows users must enable Developer Mode

**ADR-007: Versioned config**
- **Rationale:** Future-proof for schema changes
- **Trade-off:** Need migration system

**ADR-008: Secret detection and ignore patterns**
- **Rationale:** Prevent accidental secret commits
- **Trade-off:** False positives possible, user friction

**ADR-009: Bash file hooks in ~/.dotcor/hooks/**
- **Rationale:** Discoverable, editable, doesn't clutter config.yaml
- **Trade-off:** Less programmatic control, relies on files

**ADR-010: Separate managed file lists per profile**
- **Rationale:** Simpler mental model, no inheritance confusion
- **Trade-off:** Can't share base files across profiles

**ADR-011: Simple template substitution (no Go templates)**
- **Rationale:** 90% of use cases are simple variable replacement
- **Trade-off:** No conditional logic in templates (can add later)

**ADR-012: No watch mode daemon**
- **Rationale:** Manual sync is predictable, watch adds complexity
- **Trade-off:** Requires explicit sync command

**ADR-013: Package documentation only (no auto-install)**
- **Rationale:** DotCor manages dotfiles, package managers manage packages
- **Trade-off:** One extra step for users (manual install)


---

## Roadmap

### v1.2 - Hooks System
- Pre/post hooks for add, remove, sync, restore
- Simple bash files in `~/.dotcor/hooks/` directory
- Graceful degradation (skip if hook doesn't exist)
- Hook types: `pre-add`, `post-add`, `pre-remove`, `post-remove`, `pre-sync`, `post-sync`, `pre-restore`, `post-restore`

### v1.3 - Recursive Add
- `--recursive` flag for adding directories
- Preserves directory structure in repo
- Respects ignore patterns
- Progress indicator for large directories

### v1.4 - Simple Template System
- Basic substitution: `{{ .Hostname }}`, `{{ .OS }}`, `{{ .User }}`
- `--template` flag for `add` command
- Templates stored with `.template` extension in repo
- New `dotcor rebuild-links` command to render templates
- No Go template complexity (keep simple substitution)
- Template variables: `.Hostname`, `.OS`, `.User`, `.Home`

### v1.5 - Improved Doctor
- More diagnostic checks (permissions, git config, symlink health)
- Actionable fix suggestions
- `--fix` flag for automatic repairs
- Better output formatting
- Checks: symlink validity, git health, permissions, config validity, locks, git remote, hook permissions

### v1.6 - Polish & Bug Fixes
- Address bugs and issues discovered during v1.2-v1.5 development
- Performance improvements
- UX refinements
- Documentation updates

### v2.0 - Machine Profiles & Package Documentation
- Machine profiles with separate managed files lists per profile
- Profile switching (`dotcor set-profile <name>`)
- List available profiles (`dotcor list-profiles`)
- Package file generation (Brewfile, winget.txt, apt.txt)
- Manual package install (no auto-execution by DotCor)
- Profile structure: Option A - separate lists per profile

### v3.0 - Undecided (Depends on v2.0 Feedback)
**Potential features:**
- TUI interface (if features warrant complexity)
- Migration tools from other dotfile managers (Stow, Chezmoi)
- Better template engine (Go templates vs simple substitution)
- Profile inheritance (base profile + profile-specific additions)

---

