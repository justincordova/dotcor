package config

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Logger         *slog.Logger `yaml:"-"`
	GitRemote      string       `yaml:"git_remote"`
	IgnorePatterns []string     `yaml:"ignore_patterns"`
}

func GetDefaultIgnorePatterns() []string {
	return []string{
		"*.key", "*.pem", "*.p12", "*.pfx",
		".env", ".env.*",
		"id_rsa", "id_rsa.*", "id_ed25519", "id_ed25519.*",
		"*.ppk",

		"*_history", ".lesshst", ".sh_history",

		"*.log",

		"*.swp", "*.swo", "*~", ".*.swp",

		".DS_Store", "Thumbs.db",
	}
}

func GetHomeDir() (string, error) {
	if dir := os.Getenv("DOTCOR_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return home, nil
}

func GetConfigDir() (string, error) {
	if dir := os.Getenv("DOTCOR_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".dotcor"), nil
}

func GetConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ".dotcorrc"), nil
}

func LoadConfig() (*Config, error) {
	configPath, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return NewDefaultConfig()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file from %s: %w", configPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	applyDefaults(&cfg)

	return &cfg, nil
}

// applyDefaults backfills fields a config file may not carry.
//
// ignore_patterns is the important one. A config written before the key
// existed, or hand-edited to drop it, unmarshals to nil — and an empty
// pattern list disables filtering entirely, so ~/.ssh/id_rsa, .env and *.pem
// get swept into the repo and pushed to the remote. Absence must mean "use
// the defaults"; only an explicitly empty list means "filter nothing", which
// yaml distinguishes for us (`ignore_patterns: []` decodes to a non-nil
// empty slice).
//
// The logger default exists because several callers in internal/core call
// cfg.Logger.X with no nil guard; main.go replaces it with the real
// file-backed logger after LoadConfig returns.
func applyDefaults(cfg *Config) {
	if cfg.IgnorePatterns == nil {
		cfg.IgnorePatterns = GetDefaultIgnorePatterns()
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
}

func LoadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	applyDefaults(&cfg)

	return &cfg, nil
}

func NewDefaultConfig() (*Config, error) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return &Config{
		Logger:         logger,
		IgnorePatterns: GetDefaultIgnorePatterns(),
	}, nil
}

func (c *Config) SaveConfig() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	// 0700 to match the repository root created at init: the config file
	// itself is already 0600 and the directory holds the user's dotfiles.
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		if c.Logger != nil {
			c.Logger.Error("failed to write temp config file", "error", err)
		}
		return fmt.Errorf("writing temp config file: %w", err)
	}

	if err := os.Rename(tempPath, configPath); err != nil {
		_ = os.Remove(tempPath)
		if c.Logger != nil {
			c.Logger.Error("failed to rename config file", "error", err)
		}
		return fmt.Errorf("renaming config file: %w", err)
	}

	if c.Logger != nil {
		c.Logger.Info("config saved", "path", configPath)
	}

	return nil
}
