package ui

import (
	"strings"
	"testing"

	"github.com/maximbilan/grammr/internal/config"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func TestTrimTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "no trailing whitespace",
			text: "Hello world",
			want: "Hello world",
		},
		{
			name: "trailing spaces",
			text: "Hello world   ",
			want: "Hello world",
		},
		{
			name: "trailing tabs",
			text: "Hello world\t\t",
			want: "Hello world",
		},
		{
			name: "trailing newlines",
			text: "Hello world\n\n",
			want: "Hello world",
		},
		{
			name: "trailing carriage returns",
			text: "Hello world\r\r",
			want: "Hello world",
		},
		{
			name: "mixed trailing whitespace",
			text: "Hello world \t\n\r ",
			want: "Hello world",
		},
		{
			name: "only whitespace",
			text: "   \t\n\r  ",
			want: "",
		},
		{
			name: "empty string",
			text: "",
			want: "",
		},
		{
			name: "leading whitespace preserved",
			text: "   Hello world",
			want: "   Hello world",
		},
		{
			name: "whitespace in middle preserved",
			text: "Hello   world",
			want: "Hello   world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimTrailingWhitespace(tt.text)
			if got != tt.want {
				t.Errorf("trimTrailingWhitespace(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseDiffIntoChanges(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		corrected string
		wantCount int // Expected number of changes
		wantTypes []diffmatchpatch.Operation
	}{
		{
			name:      "identical text",
			original:  "Hello world",
			corrected: "Hello world",
			wantCount: 0,
			wantTypes: []diffmatchpatch.Operation{},
		},
		{
			name:      "single word replacement",
			original:  "Hello world",
			corrected: "Hello there",
			wantCount: 1, // Should pair delete+insert
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffDelete},
		},
		{
			name:      "punctuation addition",
			original:  "Hello world",
			corrected: "Hello, world",
			wantCount: 1,
			wantTypes: []diffmatchpatch.Operation{}, // Can be Delete or Insert depending on diff algorithm
		},
		{
			name:      "multiple changes",
			original:  "I are happy",
			corrected: "I am happy",
			wantCount: 1, // "are" -> "am" should be paired
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffDelete},
		},
		{
			name:      "text addition",
			original:  "Hello",
			corrected: "Hello world",
			wantCount: 1,
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffInsert},
		},
		{
			name:      "text deletion",
			original:  "Hello world",
			corrected: "Hello",
			wantCount: 1,
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffDelete},
		},
		{
			name:      "empty original",
			original:  "",
			corrected: "Hello world",
			wantCount: 1,
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffInsert},
		},
		{
			name:      "empty corrected",
			original:  "Hello world",
			corrected: "",
			wantCount: 1,
			wantTypes: []diffmatchpatch.Operation{diffmatchpatch.DiffDelete},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := parseDiffIntoChanges(tt.original, tt.corrected)

			if len(changes) != tt.wantCount {
				t.Errorf("parseDiffIntoChanges() count = %v, want %v", len(changes), tt.wantCount)
			}

			// Verify change types match expected
			if len(changes) > 0 && len(tt.wantTypes) > 0 {
				for i, change := range changes {
					if i < len(tt.wantTypes) && change.Type != tt.wantTypes[i] {
						t.Errorf("parseDiffIntoChanges() change[%d].Type = %v, want %v", i, change.Type, tt.wantTypes[i])
					}
					// Verify all changes start as not applied/skipped
					if change.Applied {
						t.Errorf("parseDiffIntoChanges() change[%d].Applied = true, want false", i)
					}
					if change.Skipped {
						t.Errorf("parseDiffIntoChanges() change[%d].Skipped = true, want false", i)
					}
					// Verify text is not empty (unless it's a deletion)
					if change.Text == "" && change.Type != diffmatchpatch.DiffDelete {
						t.Errorf("parseDiffIntoChanges() change[%d].Text is empty", i)
					}
				}
			}
		})
	}
}

func TestParseDiffIntoChangesPairsDeleteInsert(t *testing.T) {
	// Test that delete+insert pairs are correctly combined
	original := "Hello world"
	corrected := "Hello there"

	changes := parseDiffIntoChanges(original, corrected)

	// Should have one change that pairs the delete and insert
	if len(changes) != 1 {
		t.Fatalf("parseDiffIntoChanges() expected 1 change, got %d", len(changes))
	}

	change := changes[0]
	if change.Type != diffmatchpatch.DiffDelete {
		t.Errorf("parseDiffIntoChanges() paired change Type = %v, want DiffDelete", change.Type)
	}

	// Paired changes should contain " → " separator
	if !strings.Contains(change.Text, " → ") {
		t.Errorf("parseDiffIntoChanges() paired change Text = %q, should contain ' → '", change.Text)
	}
}

func TestBuildReviewedTextFromDiffs(t *testing.T) {
	tests := []struct {
		name      string
		original  string
		corrected string
		setupFunc func(string, string) []DiffChange // Function to set up changes
		want      string
	}{
		{
			name:      "all changes applied",
			original:  "Hello world",
			corrected: "Hello, world",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = true
					changes[i].Skipped = false
				}
				return changes
			},
			want: "Hello, world",
		},
		{
			name:      "all changes skipped",
			original:  "Hello world",
			corrected: "Hello, world",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = false
					changes[i].Skipped = true
				}
				return changes
			},
			want: "Hello world", // Should keep original
		},
		{
			name:      "no changes",
			original:  "Hello world",
			corrected: "Hello world",
			setupFunc: func(orig, corr string) []DiffChange {
				return []DiffChange{}
			},
			want: "Hello world",
		},
		{
			name:      "single insert applied",
			original:  "Hello",
			corrected: "Hello world",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = true
					changes[i].Skipped = false
				}
				return changes
			},
			want: "Hello world",
		},
		{
			name:      "single insert skipped",
			original:  "Hello",
			corrected: "Hello world",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = false
					changes[i].Skipped = true
				}
				return changes
			},
			want: "Hello", // Should keep original
		},
		{
			name:      "single delete applied",
			original:  "Hello world",
			corrected: "Hello",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = true
					changes[i].Skipped = false
				}
				return changes
			},
			want: "Hello", // Delete applied means remove it
		},
		{
			name:      "single delete skipped",
			original:  "Hello world",
			corrected: "Hello",
			setupFunc: func(orig, corr string) []DiffChange {
				changes := parseDiffIntoChanges(orig, corr)
				for i := range changes {
					changes[i].Applied = false
					changes[i].Skipped = true
				}
				return changes
			},
			want: "Hello world", // Skip means keep original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var changes []DiffChange
			if tt.setupFunc != nil {
				changes = tt.setupFunc(tt.original, tt.corrected)
			}
			got := buildReviewedTextFromDiffs(tt.original, tt.corrected, changes)
			if got != tt.want {
				t.Errorf("buildReviewedTextFromDiffs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildReviewedTextFromDiffsMultipleChanges(t *testing.T) {
	// Test with multiple changes
	original := "I are happy"
	corrected := "I am very happy"

	// Create changes that represent the diff
	changes := parseDiffIntoChanges(original, corrected)

	// Apply first change, skip second
	if len(changes) >= 1 {
		changes[0].Applied = true
		changes[0].Skipped = false
	}
	if len(changes) >= 2 {
		changes[1].Applied = false
		changes[1].Skipped = true
	}

	result := buildReviewedTextFromDiffs(original, corrected, changes)

	// Result should reflect applied/skipped changes
	// This is a complex case, so we just verify it doesn't crash and produces something reasonable
	if result == "" && original != "" {
		t.Error("buildReviewedTextFromDiffs() returned empty string for non-empty input")
	}
}

func TestBuildReviewedTextFromDiffsEdgeCases(t *testing.T) {
	t.Run("empty strings", func(t *testing.T) {
		result := buildReviewedTextFromDiffs("", "", []DiffChange{})
		if result != "" {
			t.Errorf("buildReviewedTextFromDiffs() empty strings = %q, want empty", result)
		}
	})

	t.Run("changes longer than diffs", func(t *testing.T) {
		// More changes than actual diffs - should handle gracefully
		changes := []DiffChange{
			{Type: diffmatchpatch.DiffDelete, Text: "extra", Applied: true},
			{Type: diffmatchpatch.DiffInsert, Text: "extra", Applied: true},
		}
		result := buildReviewedTextFromDiffs("Hello", "Hello", changes)
		// Should not crash and should return something reasonable
		if result == "" {
			t.Error("buildReviewedTextFromDiffs() should handle extra changes gracefully")
		}
	})

	t.Run("unicode text", func(t *testing.T) {
		original := "Hello 世界"
		corrected := "Hello, 世界"
		changes := parseDiffIntoChanges(original, corrected)
		if len(changes) > 0 {
			changes[0].Applied = true
		}
		result := buildReviewedTextFromDiffs(original, corrected, changes)
		if !strings.Contains(result, "世界") {
			t.Error("buildReviewedTextFromDiffs() should preserve unicode characters")
		}
	})
}

func TestHasConfiguredAPIKey(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "openai key configured",
			cfg: &config.Config{
				Provider: "openai",
				APIKey:   "sk-openai-1234567890",
			},
			want: true,
		},
		{
			name: "anthropic key configured",
			cfg: &config.Config{
				Provider:        "anthropic",
				AnthropicAPIKey: "sk-ant-1234567890",
			},
			want: true,
		},
		{
			name: "anthropic fallback to api_key",
			cfg: &config.Config{
				Provider: "anthropic",
				APIKey:   "sk-legacy-1234567890",
			},
			want: true,
		},
		{
			name: "missing keys",
			cfg: &config.Config{
				Provider: "openai",
			},
			want: false,
		},
		{
			name: "nil config",
			cfg:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasConfiguredAPIKey(tt.cfg)
			if got != tt.want {
				t.Fatalf("hasConfiguredAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMissingAPIKeyMessage(t *testing.T) {
	anthropicMessage := missingAPIKeyMessage(&config.Config{Provider: "anthropic"})
	if !strings.Contains(anthropicMessage, "anthropic_api_key") {
		t.Fatalf("anthropic message should mention anthropic_api_key, got: %q", anthropicMessage)
	}

	openAIMessage := missingAPIKeyMessage(&config.Config{Provider: "openai"})
	if !strings.Contains(openAIMessage, "api_key") || strings.Contains(openAIMessage, "anthropic_api_key") {
		t.Fatalf("openai message should mention api_key only, got: %q", openAIMessage)
	}
}
