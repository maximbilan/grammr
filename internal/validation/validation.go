package validation

import (
	"fmt"
	"strings"
)

const (
	// MaxInputLength is the maximum allowed length for input text (100K characters)
	// This prevents excessive API costs and potential memory issues
	MaxInputLength = 100000
)

// ValidateAPIKey validates the format of an OpenAI API key
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	// OpenAI API keys typically start with "sk-" and are 51 characters long
	// But we'll be lenient: at least 20 chars and starts with "sk-"
	if len(apiKey) < 20 {
		return fmt.Errorf("API key appears to be invalid (too short)")
	}
	if !strings.HasPrefix(apiKey, "sk-") {
		return fmt.Errorf("API key must start with 'sk-'")
	}
	return nil
}

// ValidateTextInput validates text input for API calls
func ValidateTextInput(text string, onChunk interface{}) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}
	if len(text) > MaxInputLength {
		return fmt.Errorf("text exceeds maximum length of %d characters (got %d)", MaxInputLength, len(text))
	}
	if onChunk == nil {
		return fmt.Errorf("onChunk callback cannot be nil")
	}
	return nil
}
