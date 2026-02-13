package corrector

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		model    string
		style    string
		language string
		wantErr  bool
	}{
		{
			name:     "valid corrector",
			apiKey:   "sk-test1234567890123456789012345678901234567890",
			model:    "gpt-4o",
			style:    "casual",
			language: "english",
			wantErr:  false,
		},
		{
			name:     "empty API key",
			apiKey:   "",
			model:    "gpt-4o",
			style:    "casual",
			language: "english",
			wantErr:  true,
		},
		{
			name:     "invalid API key (too short)",
			apiKey:   "sk-short",
			model:    "gpt-4o",
			style:    "casual",
			language: "english",
			wantErr:  true,
		},
		{
			name:     "invalid API key (wrong prefix)",
			apiKey:   "invalid-key-123456789012345678901234567890",
			model:    "gpt-4o",
			style:    "casual",
			language: "english",
			wantErr:  true,
		},
		{
			name:     "different model",
			apiKey:   "sk-test1234567890123456789012345678901234567890",
			model:    "gpt-3.5-turbo",
			style:    "formal",
			language: "english",
			wantErr:  false,
		},
		{
			name:     "different mode",
			apiKey:   "sk-test1234567890123456789012345678901234567890",
			model:    "gpt-4o",
			style:    "academic",
			language: "english",
			wantErr:  false,
		},
		{
			name:     "empty language defaults to english",
			apiKey:   "sk-test1234567890123456789012345678901234567890",
			model:    "gpt-4o",
			style:    "casual",
			language: "",
			wantErr:  false,
		},
		{
			name:     "non-english language",
			apiKey:   "sk-test1234567890123456789012345678901234567890",
			model:    "gpt-4o",
			style:    "casual",
			language: "spanish",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrector, err := New(tt.apiKey, tt.model, tt.style, tt.language)
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
				if corrector.style != tt.style {
					t.Errorf("New() style = %v, want %v", corrector.style, tt.style)
				}
				expectedLang := tt.language
				if expectedLang == "" {
					expectedLang = "english"
				}
				if corrector.language != expectedLang {
					t.Errorf("New() language = %v, want %v", corrector.language, expectedLang)
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
		style    string
		text     string
		wantStyle string // Expected style used (may differ if invalid)
		wantText string // Text that should be in prompt
	}{
		{
			name:     "casual style",
			style:    "casual",
			text:     "Hello world",
			wantStyle: "casual",
			wantText: "Hello world",
		},
		{
			name:     "formal style",
			style:    "formal",
			text:     "Fix this text",
			wantStyle: "formal",
			wantText: "Fix this text",
		},
		{
			name:     "academic style",
			style:    "academic",
			text:     "Academic writing",
			wantStyle: "academic",
			wantText: "Academic writing",
		},
		{
			name:     "technical style",
			style:    "technical",
			text:     "Technical documentation",
			wantStyle: "technical",
			wantText: "Technical documentation",
		},
		{
			name:     "invalid style defaults to casual",
			style:    "invalid-style",
			text:     "Some text",
			wantStyle: "casual", // Should default to casual
			wantText: "Some text",
		},
		{
			name:     "empty style defaults to casual",
			style:    "",
			text:     "Some text",
			wantStyle: "casual",
			wantText: "Some text",
		},
		{
			name:     "text with newlines",
			style:    "casual",
			text:     "Line 1\nLine 2\nLine 3",
			wantStyle: "casual",
			wantText: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "empty text",
			style:    "casual",
			text:     "",
			wantStyle: "casual",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrector, err := New("sk-test1234567890123456789012345678901234567890", "gpt-4o", tt.style, "english")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			prompt := corrector.buildPrompt(tt.text)

			// Verify text is in prompt
			if !strings.Contains(prompt, tt.wantText) {
				t.Errorf("buildPrompt() does not contain text %q. Got: %q", tt.wantText, prompt)
			}

			// Verify style-specific instructions
			switch tt.wantStyle {
			case "casual":
				if !strings.Contains(strings.ToLower(prompt), "casual") {
					t.Errorf("buildPrompt() should contain 'casual' for casual style")
				}
			case "formal":
				if !strings.Contains(strings.ToLower(prompt), "formal") {
					t.Errorf("buildPrompt() should contain 'formal' for formal style")
				}
			case "academic":
				if !strings.Contains(strings.ToLower(prompt), "academic") {
					t.Errorf("buildPrompt() should contain 'academic' for academic style")
				}
			case "technical":
				if !strings.Contains(strings.ToLower(prompt), "technical") {
					t.Errorf("buildPrompt() should contain 'technical' for technical style")
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

func TestBuildPromptStyleSpecificity(t *testing.T) {
	// Test that each style produces different prompts
	styles := []string{"casual", "formal", "academic", "technical"}
	prompts := make(map[string]string)

	for _, style := range styles {
		corrector, err := New("sk-test1234567890123456789012345678901234567890", "gpt-4o", style, "english")
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		prompts[style] = corrector.buildPrompt("test text")
	}

	// Verify all prompts are different
	for i, style1 := range styles {
		for j, style2 := range styles {
			if i < j && prompts[style1] == prompts[style2] {
				t.Errorf("buildPrompt() produces same prompt for %v and %v styles", style1, style2)
			}
		}
	}
}

func TestBuildPromptWithLanguage(t *testing.T) {
	tests := []struct {
		name     string
		language string
		wantLang string // Expected language in prompt
	}{
		{
			name:     "english language",
			language: "english",
			wantLang: "", // English shouldn't add language instruction
		},
		{
			name:     "spanish language",
			language: "spanish",
			wantLang: "spanish",
		},
		{
			name:     "french language",
			language: "french",
			wantLang: "french",
		},
		{
			name:     "empty language defaults to english",
			language: "",
			wantLang: "", // Should default to english, no instruction
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corrector, err := New("sk-test1234567890123456789012345678901234567890", "gpt-4o", "casual", tt.language)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			prompt := corrector.buildPrompt("test text")

			if tt.wantLang == "" {
				// Should not contain language instruction for English
				if strings.Contains(strings.ToLower(prompt), "the text is in") {
					t.Errorf("buildPrompt() should not contain language instruction for English")
				}
			} else {
				// Should contain language instruction
				if !strings.Contains(strings.ToLower(prompt), tt.wantLang) {
					t.Errorf("buildPrompt() should contain language %q in prompt. Got: %q", tt.wantLang, prompt)
				}
			}
		})
	}
}
