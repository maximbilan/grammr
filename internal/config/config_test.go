package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	// Set HOME to temp directory
	os.Setenv("HOME", tmpDir)

	t.Run("load with defaults when config file doesn't exist", func(t *testing.T) {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg == nil {
			t.Fatal("Load() returned nil config")
		}

		// Verify defaults
		if cfg.Model != "gpt-4o" {
			t.Errorf("Load() Model = %v, want gpt-4o", cfg.Model)
		}
		if cfg.Theme != "dark" {
			t.Errorf("Load() Theme = %v, want dark", cfg.Theme)
		}
		if cfg.ShowDiff != true {
			t.Errorf("Load() ShowDiff = %v, want true", cfg.ShowDiff)
		}
		if cfg.Mode != "casual" {
			t.Errorf("Load() Mode = %v, want casual", cfg.Mode)
		}
		if cfg.CacheEnabled != true {
			t.Errorf("Load() CacheEnabled = %v, want true", cfg.CacheEnabled)
		}
		if cfg.CacheTTLDays != 7 {
			t.Errorf("Load() CacheTTLDays = %v, want 7", cfg.CacheTTLDays)
		}

		// Verify config directory was created
		configPath := filepath.Join(tmpDir, ".grammr")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Errorf("Load() config directory was not created: %v", configPath)
		}
	})

	t.Run("load existing config file", func(t *testing.T) {
		viper.Reset()
		configPath := filepath.Join(tmpDir, ".grammr")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		// Create a config file
		configFile := filepath.Join(configPath, "config.yaml")
		configContent := `api_key: test-key
model: gpt-3.5-turbo
theme: light
show_diff: false
mode: formal
cache_enabled: false
cache_ttl_days: 14
`
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg == nil {
			t.Fatal("Load() returned nil config")
		}

		// Verify loaded values
		if cfg.APIKey != "test-key" {
			t.Errorf("Load() APIKey = %v, want test-key", cfg.APIKey)
		}
		if cfg.Model != "gpt-3.5-turbo" {
			t.Errorf("Load() Model = %v, want gpt-3.5-turbo", cfg.Model)
		}
		if cfg.Theme != "light" {
			t.Errorf("Load() Theme = %v, want light", cfg.Theme)
		}
		if cfg.ShowDiff != false {
			t.Errorf("Load() ShowDiff = %v, want false", cfg.ShowDiff)
		}
		if cfg.Mode != "formal" {
			t.Errorf("Load() Mode = %v, want formal", cfg.Mode)
		}
		if cfg.CacheEnabled != false {
			t.Errorf("Load() CacheEnabled = %v, want false", cfg.CacheEnabled)
		}
		if cfg.CacheTTLDays != 14 {
			t.Errorf("Load() CacheTTLDays = %v, want 14", cfg.CacheTTLDays)
		}
	})
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	os.Setenv("HOME", tmpDir)

	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "save complete config",
			cfg: &Config{
				APIKey:       "test-api-key",
				Model:        "gpt-4",
				Theme:        "light",
				ShowDiff:     false,
				AutoCopy:     true,
				Mode:         "academic",
				CacheEnabled: true,
				CacheTTLDays: 10,
			},
		},
		{
			name: "save minimal config",
			cfg: &Config{
				APIKey: "minimal-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			err := Save(tt.cfg)
			if err != nil {
				t.Fatalf("Save() error = %v, want nil", err)
			}

			// Verify config file was created
			configPath := filepath.Join(tmpDir, ".grammr")
			configFile := filepath.Join(configPath, "config.yaml")
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				t.Errorf("Save() config file was not created: %v", configFile)
				return
			}

			// Reload and verify
			viper.Reset()
			loaded, err := Load()
			if err != nil {
				t.Fatalf("Load() after Save() error = %v", err)
			}

			if loaded.APIKey != tt.cfg.APIKey {
				t.Errorf("Save() APIKey = %v, want %v", loaded.APIKey, tt.cfg.APIKey)
			}
			if tt.cfg.Model != "" && loaded.Model != tt.cfg.Model {
				t.Errorf("Save() Model = %v, want %v", loaded.Model, tt.cfg.Model)
			}
			if tt.cfg.Theme != "" && loaded.Theme != tt.cfg.Theme {
				t.Errorf("Save() Theme = %v, want %v", loaded.Theme, tt.cfg.Theme)
			}
			if loaded.ShowDiff != tt.cfg.ShowDiff {
				t.Errorf("Save() ShowDiff = %v, want %v", loaded.ShowDiff, tt.cfg.ShowDiff)
			}
			if loaded.AutoCopy != tt.cfg.AutoCopy {
				t.Errorf("Save() AutoCopy = %v, want %v", loaded.AutoCopy, tt.cfg.AutoCopy)
			}
			if tt.cfg.Mode != "" && loaded.Mode != tt.cfg.Mode {
				t.Errorf("Save() Mode = %v, want %v", loaded.Mode, tt.cfg.Mode)
			}
			if loaded.CacheEnabled != tt.cfg.CacheEnabled {
				t.Errorf("Save() CacheEnabled = %v, want %v", loaded.CacheEnabled, tt.cfg.CacheEnabled)
			}
			if loaded.CacheTTLDays != tt.cfg.CacheTTLDays {
				t.Errorf("Save() CacheTTLDays = %v, want %v", loaded.CacheTTLDays, tt.cfg.CacheTTLDays)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	os.Setenv("HOME", tmpDir)

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "set api_key",
			key:   "api_key",
			value: "new-api-key",
		},
		{
			name:  "set model",
			key:   "model",
			value: "gpt-3.5-turbo",
		},
		{
			name:  "set mode",
			key:   "mode",
			value: "technical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			err := Set(tt.key, tt.value)
			if err != nil {
				t.Fatalf("Set() error = %v, want nil", err)
			}

			// Verify value was set
			viper.Reset()
			got := Get(tt.key)
			if got != tt.value {
				t.Errorf("Set() Get(%v) = %v, want %v", tt.key, got, tt.value)
			}
		})
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	os.Setenv("HOME", tmpDir)

	t.Run("get default value", func(t *testing.T) {
		viper.Reset()
		// Load config to set defaults
		_, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got := Get("model")
		if got != "gpt-4o" {
			t.Errorf("Get() model = %v, want gpt-4o", got)
		}
	})

	t.Run("get non-existent key", func(t *testing.T) {
		viper.Reset()
		_, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got := Get("non_existent_key")
		if got != nil {
			t.Errorf("Get() non-existent key = %v, want nil", got)
		}
	})

	t.Run("get set value", func(t *testing.T) {
		viper.Reset()
		err := Set("test_key", "test_value")
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		got := Get("test_key")
		if got != "test_value" {
			t.Errorf("Get() test_key = %v, want test_value", got)
		}
	})
}

func TestLoadWithInvalidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	os.Setenv("HOME", tmpDir)

	// Create config directory and invalid YAML file
	configPath := filepath.Join(tmpDir, ".grammr")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	configFile := filepath.Join(configPath, "config.yaml")
	invalidYAML := `invalid: yaml: content: [unclosed`
	if err := os.WriteFile(configFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	// Load should return an error for invalid YAML
	_, err := Load()
	if err == nil {
		t.Error("Load() with invalid YAML should return error")
	}
}
