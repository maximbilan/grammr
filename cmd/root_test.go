package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestIsSensitiveConfigKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{name: "openai key", key: "api_key", want: true},
		{name: "anthropic key", key: "anthropic_api_key", want: true},
		{name: "openai key uppercase and spaces", key: " API_KEY ", want: true},
		{name: "non-sensitive key", key: "model", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveConfigKey(tt.key)
			if got != tt.want {
				t.Fatalf("isSensitiveConfigKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "long key", value: "sk-ant-1234567890", want: "sk-a***7890"},
		{name: "short key", value: "short", want: "***"},
		{name: "empty key", value: "", want: "***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.value)
			if got != tt.want {
				t.Fatalf("maskSecret(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = originalStdout

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	_ = r.Close()

	return string(data)
}

func TestSetCmdMasksSensitiveValues(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		viper.Reset()
	}()
	_ = os.Setenv("HOME", tmpHome)
	viper.Reset()

	rawKey := "sk-ant-12345678901234567890"
	out := captureStdout(t, func() {
		setCmd.Run(setCmd, []string{"anthropic_api_key", rawKey})
	})

	if !strings.Contains(out, "Set anthropic_api_key = sk-a***7890") {
		t.Fatalf("expected masked output, got: %q", out)
	}
	if strings.Contains(out, rawKey) {
		t.Fatalf("raw key leaked in output: %q", out)
	}

	configFile := filepath.Join(tmpHome, ".grammr", "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestGetCmdMasksSensitiveValues(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		viper.Reset()
	}()
	_ = os.Setenv("HOME", tmpHome)
	viper.Reset()

	// Use setCmd to create/configure file through the same command surface.
	_ = captureStdout(t, func() {
		setCmd.Run(setCmd, []string{"api_key", "sk-12345678901234567890"})
	})

	out := captureStdout(t, func() {
		getCmd.Run(getCmd, []string{"api_key"})
	})
	if !strings.Contains(out, "api_key = sk-1***7890") {
		t.Fatalf("expected masked get output, got: %q", out)
	}
	if strings.Contains(out, "sk-12345678901234567890") {
		t.Fatalf("raw key leaked in get output: %q", out)
	}
}

func TestInitCmdCreatesConfigFile(t *testing.T) {
	tmpHome := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
		viper.Reset()
	}()
	_ = os.Setenv("HOME", tmpHome)
	viper.Reset()

	out := captureStdout(t, func() {
		initCmd.Run(initCmd, nil)
	})

	if !strings.Contains(out, "Configuration initialized at") {
		t.Fatalf("expected init success output, got: %q", out)
	}

	configFile := filepath.Join(tmpHome, ".grammr", "config.yaml")
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("expected init to create config file: %v", err)
	}
}
