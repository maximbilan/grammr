package corrector

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		model   string
		mode    string
		wantErr bool
	}{
		{
			name:    "valid corrector",
			apiKey:  "test-api-key",
			model:   "gpt-4o",
			mode:    "casual",
			wantErr: false,
		},
		{
			name:    "empty API key",
			apiKey:  "",
			model:   "gpt-4o",
			mode:    "casual",
			wantErr: true,
		},
		{
			name:    "different model",
			apiKey:  "test-api-key",
			model:   "gpt-3.5-turbo",
			mode:    "formal",
			wantErr: false,
		},
		{
			name:    "different mode",
			apiKey:  "test-api-key",
			model:   "gpt-4o",
			mode:    "academic",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrector, err := New(tt.apiKey, tt.model, tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if corrector == nil {
					t.Error("New() returned nil corrector without error")
					return
				}
				if corrector.model != tt.model {
					t.Errorf("New() model = %v, want %v", corrector.model, tt.model)
				}
				if corrector.mode != tt.mode {
					t.Errorf("New() mode = %v, want %v", corrector.mode, tt.mode)
				}
				if corrector.client == nil {
					t.Error("New() client is nil")
				}
			} else {
				if corrector != nil {
					t.Error("New() returned non-nil corrector with error")
				}
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		text     string
		wantMode string // Expected mode used (may differ if invalid)
		wantText string // Text that should be in prompt
	}{
		{
			name:     "casual mode",
			mode:     "casual",
			text:     "Hello world",
			wantMode: "casual",
			wantText: "Hello world",
		},
		{
			name:     "formal mode",
			mode:     "formal",
			text:     "Fix this text",
			wantMode: "formal",
			wantText: "Fix this text",
		},
		{
			name:     "academic mode",
			mode:     "academic",
			text:     "Academic writing",
			wantMode: "academic",
			wantText: "Academic writing",
		},
		{
			name:     "technical mode",
			mode:     "technical",
			text:     "Technical documentation",
			wantMode: "technical",
			wantText: "Technical documentation",
		},
		{
			name:     "invalid mode defaults to casual",
			mode:     "invalid-mode",
			text:     "Some text",
			wantMode: "casual", // Should default to casual
			wantText: "Some text",
		},
		{
			name:     "empty mode defaults to casual",
			mode:     "",
			text:     "Some text",
			wantMode: "casual",
			wantText: "Some text",
		},
		{
			name:     "text with newlines",
			mode:     "casual",
			text:     "Line 1\nLine 2\nLine 3",
			wantMode: "casual",
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "empty text",
			mode:     "casual",
			text:     "",
			wantMode: "casual",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrector, err := New("test-api-key", "gpt-4o", tt.mode)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			prompt := corrector.buildPrompt(tt.text)

			// Verify text is in prompt
			if !strings.Contains(prompt, tt.wantText) {
				t.Errorf("buildPrompt() does not contain text %q. Got: %q", tt.wantText, prompt)
			}

			// Verify mode-specific instructions
			switch tt.wantMode {
			case "casual":
				if !strings.Contains(strings.ToLower(prompt), "casual") {
					t.Errorf("buildPrompt() should contain 'casual' for casual mode")
				}
			case "formal":
				if !strings.Contains(strings.ToLower(prompt), "formal") {
					t.Errorf("buildPrompt() should contain 'formal' for formal mode")
				}
			case "academic":
				if !strings.Contains(strings.ToLower(prompt), "academic") {
					t.Errorf("buildPrompt() should contain 'academic' for academic mode")
				}
			case "technical":
				if !strings.Contains(strings.ToLower(prompt), "technical") {
					t.Errorf("buildPrompt() should contain 'technical' for technical mode")
				}
			}

			// Verify prompt contains instruction to only output corrected text
			if !strings.Contains(strings.ToLower(prompt), "only output") {
				t.Errorf("buildPrompt() should contain instruction to only output corrected text")
			}

			// Verify prompt structure: should have instructions and text
			if !strings.Contains(prompt, "Text to correct:") {
				t.Errorf("buildPrompt() should contain 'Text to correct:' separator")
			}
		})
	}
}

func TestBuildPromptModeSpecificity(t *testing.T) {
	// Test that each mode produces different prompts
	modes := []string{"casual", "formal", "academic", "technical"}
	prompts := make(map[string]string)

	for _, mode := range modes {
		corrector, err := New("test-api-key", "gpt-4o", mode)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		prompts[mode] = corrector.buildPrompt("test text")
	}

	// Verify all prompts are different
	for i, mode1 := range modes {
		for j, mode2 := range modes {
			if i < j && prompts[mode1] == prompts[mode2] {
				t.Errorf("buildPrompt() produces same prompt for %v and %v modes", mode1, mode2)
			}
		}
	}
}
