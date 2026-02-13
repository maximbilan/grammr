package cmd

import "testing"

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
