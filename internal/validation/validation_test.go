package validation

import (
	"strings"
	"testing"
)

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid openai key",
			apiKey:  "sk-12345678901234567890",
			wantErr: false,
		},
		{
			name:    "valid anthropic-style key",
			apiKey:  "sk-ant-12345678901234567890",
			wantErr: false,
		},
		{
			name:    "empty key",
			apiKey:  "",
			wantErr: true,
		},
		{
			name:    "too short",
			apiKey:  "sk-short",
			wantErr: true,
		},
		{
			name:    "invalid prefix",
			apiKey:  "abc-12345678901234567890",
			wantErr: true,
		},
		{
			name:    "leading whitespace invalidates prefix",
			apiKey:  " sk-12345678901234567890",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKey(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAPIKey(%q) error = %v, wantErr %v", tt.apiKey, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTextInput(t *testing.T) {
	validChunk := func(string) {}
	var nilChunk func(string)

	tests := []struct {
		name    string
		text    string
		onChunk interface{}
		wantErr bool
	}{
		{
			name:    "valid input",
			text:    "Hello world",
			onChunk: validChunk,
			wantErr: false,
		},
		{
			name:    "boundary max input length",
			text:    strings.Repeat("a", MaxInputLength),
			onChunk: validChunk,
			wantErr: false,
		},
		{
			name:    "empty text",
			text:    "",
			onChunk: validChunk,
			wantErr: true,
		},
		{
			name:    "input too long",
			text:    strings.Repeat("a", MaxInputLength+1),
			onChunk: validChunk,
			wantErr: true,
		},
		{
			name:    "nil callback",
			text:    "Hello world",
			onChunk: nil,
			wantErr: true,
		},
		{
			name:    "typed nil callback",
			text:    "Hello world",
			onChunk: nilChunk,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTextInput(tt.text, tt.onChunk)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTextInput() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
