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

	return &cfg, nil
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
	if err := os.MkdirAll(configDir, 0755); err != nil {
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
