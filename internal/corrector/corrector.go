package corrector

import (
	"context"
	"fmt"

	"github.com/maximbilan/grammr/internal/provider"
	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/maximbilan/grammr/internal/validation"
)


type Corrector struct {
	provider    provider.Provider
	model       string
	style       string
	language    string
	rateLimiter *ratelimit.RateLimiter
}

// New creates a new Corrector with a provider
func New(prov provider.Provider, model, style, language string) (*Corrector, error) {
	return NewWithRateLimit(prov, model, style, language, nil)
}

// NewWithRateLimit creates a new Corrector with an optional rate limiter
func NewWithRateLimit(prov provider.Provider, model, style, language string, rateLimiter *ratelimit.RateLimiter) (*Corrector, error) {
	if prov == nil {
		return nil, fmt.Errorf("provider is required")
	}

	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Default to English if language is empty
	if language == "" {
		language = "english"
	}

	// Validate style
	validStyles := map[string]bool{
		"casual":    true,
		"formal":    true,
		"academic":  true,
		"technical": true,
	}
	if style != "" && !validStyles[style] {
		// Default to casual for invalid styles
		style = "casual"
	}

	return &Corrector{
		provider:    prov,
		model:       model,
		style:       style,
		language:    language,
		rateLimiter: rateLimiter,
	}, nil
}

func (c *Corrector) buildPrompt(text string) string {
	prompts := map[string]string{
		"casual": `Fix grammar, spelling, and punctuation. Keep it casual and natural.
Only output the corrected text, nothing else.`,

		"formal": `Fix grammar, spelling, and punctuation. Make it more formal and professional.
Only output the corrected text, nothing else.`,

		"academic": `Fix grammar, spelling, and punctuation. Use academic writing style.
Only output the corrected text, nothing else.`,

		"technical": `Fix grammar, spelling, and punctuation. Maintain technical accuracy.
Only output the corrected text, nothing else.`,
	}

	prompt, ok := prompts[c.style]
	if !ok {
		prompt = prompts["casual"]
	}

	// Add language instruction if not English
	languageInstruction := ""
	if c.language != "" && c.language != "english" {
		languageInstruction = fmt.Sprintf(" The text is in %s. Correct it in %s.\n", c.language, c.language)
	}

	return fmt.Sprintf("%s%s\nText to correct:\n%s", prompt, languageInstruction, text)
}

func (c *Corrector) StreamCorrect(ctx context.Context, text string, onChunk func(string)) error {
	if err := validation.ValidateTextInput(text, onChunk); err != nil {
		return err
	}

	// Apply rate limiting if enabled
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit error: %w", err)
		}
	}

	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: c.buildPrompt(text),
		},
	}

	return c.provider.StreamChat(ctx, c.model, messages, onChunk)
}

// Correct performs a non-streaming correction (fallback)
func (c *Corrector) Correct(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text cannot be empty")
	}

	if len(text) > validation.MaxInputLength {
		return "", fmt.Errorf("text exceeds maximum length of %d characters (got %d)", validation.MaxInputLength, len(text))
	}

	// Apply rate limiting if enabled
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit error: %w", err)
		}
	}

	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: c.buildPrompt(text),
		},
	}

	return c.provider.Chat(ctx, c.model, messages)
}
