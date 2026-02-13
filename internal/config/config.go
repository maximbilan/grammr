package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	// ConfigDirPerm is the permission for the config directory (0700 = rwx------)
	// Restrictive permissions protect the directory from being accessed by other users
	ConfigDirPerm os.FileMode = 0700
	// ConfigFilePerm is the permission for the config file (0600 = rw-------)
	// Restrictive permissions protect the API key from being read by other users
	ConfigFilePerm os.FileMode = 0600
)

type Config struct {
	APIKey            string `mapstructure:"api_key"`
	Model             string `mapstructure:"model"`
	Theme             string `mapstructure:"theme"`
	ShowDiff          bool   `mapstructure:"show_diff"`
	AutoCopy          bool   `mapstructure:"auto_copy"`
	Mode              string `mapstructure:"mode"`
	Language          string `mapstructure:"language"`
	TranslationLanguage string `mapstructure:"translation_language"`
	CacheEnabled      bool   `mapstructure:"cache_enabled"`
	CacheTTLDays      int    `mapstructure:"cache_ttl_days"`
	RateLimitEnabled  bool   `mapstructure:"rate_limit_enabled"`
	RateLimitRequests int    `mapstructure:"rate_limit_requests"`
	RateLimitWindow   int    `mapstructure:"rate_limit_window_seconds"`
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
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
	viper.SetDefault("translation_language", "")
	viper.SetDefault("cache_enabled", true)
	viper.SetDefault("cache_ttl_days", 7)
	viper.SetDefault("rate_limit_enabled", true)
	viper.SetDefault("rate_limit_requests", 60)      // 60 requests
	viper.SetDefault("rate_limit_window_seconds", 60) // per minute
	viper.SetDefault("request_timeout_seconds", 30)   // 30 seconds default timeout

	// Try to read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; create directory
			if err := os.MkdirAll(configPath, ConfigDirPerm); err != nil {
				return nil, fmt.Errorf("failed to create config directory: %w", err)
			}
			// Return config with defaults
			config := &Config{}
			if err := viper.Unmarshal(config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal default config: %w", err)
			}
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
	if err := os.MkdirAll(configPath, ConfigDirPerm); err != nil {
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
	viper.Set("translation_language", cfg.TranslationLanguage)
	viper.Set("cache_enabled", cfg.CacheEnabled)
	viper.Set("cache_ttl_days", cfg.CacheTTLDays)
	viper.Set("rate_limit_enabled", cfg.RateLimitEnabled)
	viper.Set("rate_limit_requests", cfg.RateLimitRequests)
	viper.Set("rate_limit_window_seconds", cfg.RateLimitWindow)
	viper.Set("request_timeout_seconds", cfg.RequestTimeoutSeconds)

	configFile := filepath.Join(configPath, "config.yaml")
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Set restrictive permissions on the config file to protect API key
	if err := os.Chmod(configFile, ConfigFilePerm); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

func Set(key, value string) error {
	if key == "" {
		return fmt.Errorf("config key cannot be empty")
	}

	// Sanitize key to prevent injection
	key = strings.TrimSpace(key)
	if strings.ContainsAny(key, " \t\n\r") {
		return fmt.Errorf("config key contains invalid characters")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".grammr")
	if err := os.MkdirAll(configPath, ConfigDirPerm); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)

	// Try to read existing config (ignore error if file doesn't exist)
	_ = viper.ReadInConfig()

	viper.Set(key, value)

	configFile := filepath.Join(configPath, "config.yaml")
	if err := viper.WriteConfigAs(configFile); err != nil {
		// If file doesn't exist, try SafeWriteConfigAs
		if err := viper.SafeWriteConfigAs(configFile); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	// Set restrictive permissions on the config file to protect API key
	if err := os.Chmod(configFile, ConfigFilePerm); err != nil {
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}

	return nil
}

func Get(key string) interface{} {
	if key == "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(home, ".grammr")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configPath)
	_ = viper.ReadInConfig() // Ignore error if config doesn't exist
	return viper.Get(key)
}
