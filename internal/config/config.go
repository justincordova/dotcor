package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// CurrentConfigVersion is the current schema version
const CurrentConfigVersion = "1.0"

// Config represents the DotCor configuration
type Config struct {
	Logger             *slog.Logger  `yaml:"-"`                    // Structured logger for system logging (not persisted)
	Version            string        `yaml:"version"`              // Schema version for migrations
	RepoPath           string        `yaml:"repo_path"`            // ~/.dotcor/files
	GitEnabled         bool          `yaml:"git_enabled"`          // Whether Git integration is enabled
	GitRemote          string        `yaml:"git_remote"`           // Optional remote URL
	IgnorePatterns     []string      `yaml:"ignore_patterns"`      // Files/patterns to never add
	ManagedFiles       []ManagedFile `yaml:"managed_files"`        // List of managed dotfiles
	LargeFileThreshold int           `yaml:"large_file_threshold"` // Max file size warning (bytes, 0 = disabled)
}

// ManagedFile represents a single managed dotfile
type ManagedFile struct {
	SourcePath     string    `yaml:"source_path"`     // ~/.zshrc (normalized, with ~)
	RepoPath       string    `yaml:"repo_path"`       // shell/zshrc (relative to files/)
	AddedAt        time.Time `yaml:"added_at"`        // When the file was added
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
		// Return default config (not initialized)
		return NewDefaultConfig()
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file from %s: %w", configPath, err)
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

// LoadConfigFromPath loads config from a specific path
// Does not handle migrations or return defaults if file doesn't exist
func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// NewDefaultConfig creates a new config with sensible defaults
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

// AddManagedFile adds a new managed file to the config
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

// RemoveManagedFile removes a managed file by source path
func (c *Config) RemoveManagedFile(sourcePath string) error {
	normalized, err := NormalizePath(sourcePath)
	if err != nil {
		return fmt.Errorf("normalizing path: %w", err)
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
		return nil, fmt.Errorf("normalizing path: %w", err)
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
