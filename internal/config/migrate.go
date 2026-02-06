package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// MigrationFunc is a function that migrates config from one version to the next
type MigrationFunc func(*Config) error

// migrations maps version transitions to their migration functions
var migrations = map[string]MigrationFunc{
	// Add future migrations here
	// "1.0->1.1": migrateV10ToV11,
}

// MigrateConfig migrates config from old version to current
// Returns error if migration fails
// Creates backup before migration
func MigrateConfig(config *Config) (*Config, error) {
	// Handle empty/missing version first (pre-v1.0 configs)
	if config.Version == "" {
		if err := MigrateFromEmpty(config); err != nil {
			return nil, fmt.Errorf("migrating from empty config: %w", err)
		}
	}

	// Create backup before migration
	if err := backupConfigForMigration(); err != nil {
		return nil, fmt.Errorf("creating backup before migration: %w", err)
	}

	// Get migration path
	path := GetMigrationPath(config.Version, CurrentConfigVersion)
	if len(path) == 0 && config.Version != CurrentConfigVersion {
		// No migration path but version differs - try to handle gracefully
		// This might happen if we're at a newer version than config
		if IsCompatibleVersion(config.Version) {
			// Just update the version and continue
			config.Version = CurrentConfigVersion
			return config, nil
		}
		return nil, fmt.Errorf("no migration path from version %s to %s", config.Version, CurrentConfigVersion)
	}

	// Apply migrations in order
	for _, migrate := range path {
		if err := migrate(config); err != nil {
			return nil, fmt.Errorf("applying migration: %w", err)
		}
	}

	// Update version
	config.Version = CurrentConfigVersion

	// Validate config after migration
	if err := ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("validation after migration: %w", err)
	}

	// Save migrated config
	if err := config.SaveConfig(); err != nil {
		return nil, fmt.Errorf("saving migrated config: %w", err)
	}

	return config, nil
}

// IsCompatibleVersion checks if config version is compatible with current version
func IsCompatibleVersion(version string) bool {
	// Empty version is treated as v1.0 (initial version)
	if version == "" {
		return true
	}

	// Same version is compatible
	if version == CurrentConfigVersion {
		return true
	}

	// Check if we have a migration path
	path := GetMigrationPath(version, CurrentConfigVersion)
	return len(path) > 0 || version == CurrentConfigVersion
}

// GetMigrationPath returns list of migrations needed from one version to another
func GetMigrationPath(fromVersion, toVersion string) []MigrationFunc {
	// Handle empty/missing version as v1.0
	if fromVersion == "" {
		fromVersion = "1.0"
	}

	// Same version, no migration needed
	if fromVersion == toVersion {
		return nil
	}

	// Build migration path
	// For now, we only have v1.0, so no migrations needed
	// Future versions would be handled here

	var path []MigrationFunc

	// Example of how to add migrations:
	// if fromVersion == "1.0" && toVersion >= "1.1" {
	//     path = append(path, migrateV10ToV11)
	//     fromVersion = "1.1"
	// }
	// if fromVersion == "1.1" && toVersion >= "1.2" {
	//     path = append(path, migrateV11ToV12)
	//     fromVersion = "1.2"
	// }

	return path
}

// backupConfigForMigration creates a backup of the config file before migration
func backupConfigForMigration() error {
	configPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	// Read current config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config for backup: %w", err)
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupPath := configPath + ".backup." + timestamp

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("writing config backup: %w", err)
	}

	return nil
}

// MigrateFromEmpty handles config files without a version field (pre-v1.0)
func MigrateFromEmpty(config *Config) error {
	// Set default values that might be missing
	if config.Version == "" {
		config.Version = CurrentConfigVersion
	}

	if config.RepoPath == "" {
		configDir, err := GetConfigDir()
		if err != nil {
			return err
		}
		config.RepoPath = configDir + "/files"
	}

	if len(config.IgnorePatterns) == 0 {
		config.IgnorePatterns = GetDefaultIgnorePatterns()
	}

	return nil
}

// Example migration function template for future use
// func migrateV10ToV11(config *Config) error {
//     // Add new fields, transform data, etc.
//     return nil
// }

// ValidateConfig checks if config is valid after loading/migration
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

	return nil
}

// ExportConfig exports config to YAML bytes (useful for debugging)
func ExportConfig(config *Config) ([]byte, error) {
	return yaml.Marshal(config)
}
