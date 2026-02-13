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
		if cfg.ShowDiff != true {
			t.Errorf("Load() ShowDiff = %v, want true", cfg.ShowDiff)
		}
		if cfg.Style != "casual" {
			t.Errorf("Load() Style = %v, want casual", cfg.Style)
		}
		if cfg.CacheEnabled != true {
			t.Errorf("Load() CacheEnabled = %v, want true", cfg.CacheEnabled)
		}
		if cfg.CacheTTLDays != 7 {
			t.Errorf("Load() CacheTTLDays = %v, want 7", cfg.CacheTTLDays)
		}
		if cfg.Language != "english" {
			t.Errorf("Load() Language = %v, want english", cfg.Language)
		}
		if cfg.TranslationLanguage != "" {
			t.Errorf("Load() TranslationLanguage = %v, want empty string", cfg.TranslationLanguage)
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
show_diff: false
mode: formal
cache_enabled: false
cache_ttl_days: 14
translation_language: spanish
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
		if cfg.ShowDiff != false {
			t.Errorf("Load() ShowDiff = %v, want false", cfg.ShowDiff)
		}
		if cfg.Style != "formal" {
			t.Errorf("Load() Style = %v, want formal", cfg.Style)
		}
		if cfg.CacheEnabled != false {
			t.Errorf("Load() CacheEnabled = %v, want false", cfg.CacheEnabled)
		}
		if cfg.CacheTTLDays != 14 {
			t.Errorf("Load() CacheTTLDays = %v, want 14", cfg.CacheTTLDays)
		}
		if cfg.TranslationLanguage != "spanish" {
			t.Errorf("Load() TranslationLanguage = %v, want spanish", cfg.TranslationLanguage)
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
				ShowDiff:     false,
				AutoCopy:     true,
				Style:        "academic",
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
			if loaded.ShowDiff != tt.cfg.ShowDiff {
				t.Errorf("Save() ShowDiff = %v, want %v", loaded.ShowDiff, tt.cfg.ShowDiff)
			}
			if loaded.AutoCopy != tt.cfg.AutoCopy {
				t.Errorf("Save() AutoCopy = %v, want %v", loaded.AutoCopy, tt.cfg.AutoCopy)
			}
			if tt.cfg.Style != "" && loaded.Style != tt.cfg.Style {
				t.Errorf("Save() Style = %v, want %v", loaded.Style, tt.cfg.Style)
			}
			if loaded.CacheEnabled != tt.cfg.CacheEnabled {
				t.Errorf("Save() CacheEnabled = %v, want %v", loaded.CacheEnabled, tt.cfg.CacheEnabled)
			}
			if loaded.CacheTTLDays != tt.cfg.CacheTTLDays {
				t.Errorf("Save() CacheTTLDays = %v, want %v", loaded.CacheTTLDays, tt.cfg.CacheTTLDays)
			}
			if tt.cfg.Language != "" && loaded.Language != tt.cfg.Language {
				t.Errorf("Save() Language = %v, want %v", loaded.Language, tt.cfg.Language)
			}
			if tt.cfg.TranslationLanguage != "" && loaded.TranslationLanguage != tt.cfg.TranslationLanguage {
				t.Errorf("Save() TranslationLanguage = %v, want %v", loaded.TranslationLanguage, tt.cfg.TranslationLanguage)
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
			name:  "set style (backward compatible with mode)",
			key:   "mode",
			value: "technical",
			// Note: Get("mode") will return nil since we map it to "style"
			// This test verifies Set works, but Get("mode") won't work after mapping
		},
		{
			name:  "set style",
			key:   "style",
			value: "academic",
		},
		{
			name:  "set translation_language",
			key:   "translation_language",
			value: "french",
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
			// If key was "mode", it was mapped to "style", so check "style" instead
			checkKey := tt.key
			if tt.key == "mode" {
				checkKey = "style"
			}
			got := Get(checkKey)
			if got != tt.value {
				t.Errorf("Set() Get(%v) = %v, want %v", checkKey, got, tt.value)
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

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "openai provider uses api_key",
			cfg: Config{
				Provider: "openai",
				APIKey:   "sk-openai-123",
			},
			want: "sk-openai-123",
		},
		{
			name: "anthropic provider uses anthropic_api_key",
			cfg: Config{
				Provider:        "anthropic",
				APIKey:          "sk-openai-fallback",
				AnthropicAPIKey: "sk-ant-123",
			},
			want: "sk-ant-123",
		},
		{
			name: "anthropic provider falls back to api_key",
			cfg: Config{
				Provider: "anthropic",
				APIKey:   "sk-openai-fallback",
			},
			want: "sk-openai-fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.GetAPIKey()
			if got != tt.want {
				t.Fatalf("GetAPIKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSaveAndSetPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", originalHome)
		viper.Reset()
	}()

	os.Setenv("HOME", tmpDir)

	cfg := &Config{
		Provider: "openai",
		APIKey:   "sk-12345678901234567890",
		Model:    "gpt-4o",
		Style:    "casual",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	configDir := filepath.Join(tmpDir, ".grammr")
	configFile := filepath.Join(configDir, "config.yaml")

	dirInfo, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("failed to stat config directory: %v", err)
	}
	if dirInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("config directory should not be accessible by group/others, got mode %o", dirInfo.Mode().Perm())
	}

	fileInfo, err := os.Stat(configFile)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if fileInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("config file should not be accessible by group/others, got mode %o", fileInfo.Mode().Perm())
	}

	if err := Set("model", "gpt-4o-mini"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	fileInfo, err = os.Stat(configFile)
	if err != nil {
		t.Fatalf("failed to stat config file after Set(): %v", err)
	}
	if fileInfo.Mode().Perm()&0077 != 0 {
		t.Fatalf("config file should remain restricted after Set(), got mode %o", fileInfo.Mode().Perm())
	}
}

func TestSetRejectsInvalidKeys(t *testing.T) {
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
		{name: "empty key", key: "", value: "x"},
		{name: "key with space", key: "api key", value: "x"},
		{name: "key with tab", key: "api\tkey", value: "x"},
		{name: "key with newline", key: "api\nkey", value: "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Set(tt.key, tt.value)
			if err == nil {
				t.Fatalf("Set(%q, %q) expected error, got nil", tt.key, tt.value)
			}
		})
	}
}
