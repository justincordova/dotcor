package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// CurrentConfigVersion is the current schema version
const CurrentConfigVersion = "1.0"

// Config represents the DotCor configuration
type Config struct {
	Version        string        `yaml:"version"`         // Schema version for migrations
	RepoPath       string        `yaml:"repo_path"`       // ~/.dotcor/files
	GitEnabled     bool          `yaml:"git_enabled"`     // Whether Git integration is enabled
	GitRemote      string        `yaml:"git_remote"`      // Optional remote URL
	IgnorePatterns []string      `yaml:"ignore_patterns"` // Files/patterns to never add
	ManagedFiles   []ManagedFile `yaml:"managed_files"`   // List of managed dotfiles
}

// ManagedFile represents a single managed dotfile
type ManagedFile struct {
	SourcePath     string    `yaml:"source_path"`     // ~/.zshrc (normalized, with ~)
	RepoPath       string    `yaml:"repo_path"`       // shell/zshrc (relative to files/)
	AddedAt        time.Time `yaml:"added_at"`        // When the file was added
	Platforms      []string  `yaml:"platforms"`       // ["darwin", "linux"] or empty for all
	HasUncommitted bool      `yaml:"has_uncommitted"` // Track if Git commit failed
}

// GetDefaultIgnorePatterns returns sensible default ignore patterns
func GetDefaultIgnorePatterns() []string {
	return []string{
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
}

// GetConfigDir returns the DotCor config directory path
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".dotcor"), nil
}

// GetConfigPath returns the config file path
func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// LoadConfig loads config from ~/.dotcor/config.yaml
// Returns default config if file doesn't exist
// Handles version migrations automatically
func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Return default config
		return NewDefaultConfig()
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Check if migration is needed
	if cfg.Version != CurrentConfigVersion {
		migratedCfg, err := MigrateConfig(&cfg)
		if err != nil {
			return nil, fmt.Errorf("migrating config: %w", err)
		}
		return migratedCfg, nil
	}

	// Validate loaded config
	if err := ValidateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

// NewDefaultConfig creates a new config with sensible defaults
func NewDefaultConfig() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	return &Config{
		Version:        CurrentConfigVersion,
		RepoPath:       filepath.Join(configDir, "files"),
		GitEnabled:     true,
		IgnorePatterns: GetDefaultIgnorePatterns(),
		ManagedFiles:   []ManagedFile{}, // Explicitly initialize empty slice
	}, nil
}

// SaveConfig atomically writes config to ~/.dotcor/config.yaml
// Uses write-to-temp + rename for atomicity
func (c *Config) SaveConfig() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write to temp file first for atomicity
	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp config file: %w", err)
	}

	// Rename temp to actual (atomic on most filesystems)
	if err := os.Rename(tempPath, configPath); err != nil {
		os.Remove(tempPath) // Clean up temp file on failure
		return fmt.Errorf("renaming config file: %w", err)
	}

	return nil
}

// AddManagedFile adds a new managed file to the config
func (c *Config) AddManagedFile(mf ManagedFile) error {
	// Check if already managed
	if c.IsManaged(mf.SourcePath) {
		return fmt.Errorf("file %s is already managed", mf.SourcePath)
	}

	c.ManagedFiles = append(c.ManagedFiles, mf)
	return c.SaveConfig()
}

// RemoveManagedFile removes a managed file by source path
func (c *Config) RemoveManagedFile(sourcePath string) error {
	normalized, err := NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	for i, mf := range c.ManagedFiles {
		if mf.SourcePath == normalized || mf.SourcePath == sourcePath {
			c.ManagedFiles = append(c.ManagedFiles[:i], c.ManagedFiles[i+1:]...)
			return c.SaveConfig()
		}
	}

	return fmt.Errorf("file %s is not managed", sourcePath)
}

// GetManagedFile retrieves managed file by source path
func (c *Config) GetManagedFile(sourcePath string) (*ManagedFile, error) {
	normalized, err := NormalizePath(sourcePath)
	if err != nil {
		normalized = sourcePath
	}

	for i := range c.ManagedFiles {
		if c.ManagedFiles[i].SourcePath == normalized || c.ManagedFiles[i].SourcePath == sourcePath {
			return &c.ManagedFiles[i], nil
		}
	}

	return nil, fmt.Errorf("file %s is not managed", sourcePath)
}

// IsManaged checks if a file is already managed
func (c *Config) IsManaged(sourcePath string) bool {
	_, err := c.GetManagedFile(sourcePath)
	return err == nil
}

// GetManagedFilesForPlatform returns files that should be linked on current platform
func (c *Config) GetManagedFilesForPlatform() []ManagedFile {
	platform := GetCurrentPlatform()
	result := []ManagedFile{}

	for _, mf := range c.ManagedFiles {
		if ShouldApplyOnPlatform(mf.Platforms, platform) {
			result = append(result, mf)
		}
	}

	return result
}

// MarkAsUncommitted marks a file as having uncommitted changes
func (c *Config) MarkAsUncommitted(sourcePath string) error {
	mf, err := c.GetManagedFile(sourcePath)
	if err != nil {
		return err
	}

	mf.HasUncommitted = true
	return c.SaveConfig()
}

// ClearUncommitted clears the uncommitted flag for a file
func (c *Config) ClearUncommitted(sourcePath string) error {
	mf, err := c.GetManagedFile(sourcePath)
	if err != nil {
		return err
	}

	mf.HasUncommitted = false
	return c.SaveConfig()
}

// GetUncommittedFiles returns all files with uncommitted changes
func (c *Config) GetUncommittedFiles() []ManagedFile {
	result := []ManagedFile{}

	for _, mf := range c.ManagedFiles {
		if mf.HasUncommitted {
			result = append(result, mf)
		}
	}

	return result
}

// GetCurrentPlatform returns current OS identifier
// Returns: "darwin", "linux", "windows", "wsl"
// Detects WSL by checking /proc/version for "Microsoft"
func GetCurrentPlatform() string {
	if runtime.GOOS == "linux" {
		// Check for WSL
		data, err := os.ReadFile("/proc/version")
		if err == nil {
			content := string(data)
			if contains(content, "Microsoft") || contains(content, "WSL") {
				return "wsl"
			}
		}
	}
	return runtime.GOOS
}

// ShouldApplyOnPlatform checks if file should be linked on the given platform
func ShouldApplyOnPlatform(platforms []string, currentPlatform string) bool {
	// Empty platforms means all platforms
	if len(platforms) == 0 {
		return true
	}

	for _, p := range platforms {
		if p == currentPlatform {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
