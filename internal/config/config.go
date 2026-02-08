package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	APIKey       string `mapstructure:"api_key"`
	Model        string `mapstructure:"model"`
	Theme        string `mapstructure:"theme"`
	ShowDiff     bool   `mapstructure:"show_diff"`
	AutoCopy     bool   `mapstructure:"auto_copy"`
	Mode         string `mapstructure:"mode"`
	Language     string `mapstructure:"language"`
	CacheEnabled bool   `mapstructure:"cache_enabled"`
	CacheTTLDays int    `mapstructure:"cache_ttl_days"`
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".grammr")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)

	// Set defaults
	viper.SetDefault("model", "gpt-4o")
	viper.SetDefault("theme", "dark")
	viper.SetDefault("show_diff", true)
	viper.SetDefault("mode", "casual")
	viper.SetDefault("language", "english")
	viper.SetDefault("cache_enabled", true)
	viper.SetDefault("cache_ttl_days", 7)

	// Try to read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; create directory
			if err := os.MkdirAll(configPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create config directory: %w", err)
			}
			// Return config with defaults
			config := &Config{}
			viper.Unmarshal(config)
			return config, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func Save(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".grammr")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set values
	viper.Set("api_key", cfg.APIKey)
	viper.Set("model", cfg.Model)
	viper.Set("theme", cfg.Theme)
	viper.Set("show_diff", cfg.ShowDiff)
	viper.Set("auto_copy", cfg.AutoCopy)
	viper.Set("mode", cfg.Mode)
	viper.Set("language", cfg.Language)
	viper.Set("cache_enabled", cfg.CacheEnabled)
	viper.Set("cache_ttl_days", cfg.CacheTTLDays)

	configFile := filepath.Join(configPath, "config.yaml")
	return viper.WriteConfigAs(configFile)
}

func Set(key, value string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".grammr")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)

	// Try to read existing config
	viper.ReadInConfig()

	viper.Set(key, value)

	configFile := filepath.Join(configPath, "config.yaml")
	if err := viper.WriteConfigAs(configFile); err != nil {
		// If file doesn't exist, create it
		if err := os.MkdirAll(configPath, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		return viper.SafeWriteConfigAs(configFile)
	}

	return nil
}

func Get(key string) interface{} {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".grammr")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)
	viper.ReadInConfig()
	return viper.Get(key)
}
