package translator

import (
	"context"
	"fmt"

	"github.com/maximbilan/grammr/internal/provider"
	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/maximbilan/grammr/internal/validation"
)


type Translator struct {
	provider          provider.Provider
	model             string
	translationLanguage string
	rateLimiter       *ratelimit.RateLimiter
}

// NewWithRateLimit creates a new Translator with an optional rate limiter
func NewWithRateLimit(prov provider.Provider, model, translationLanguage string, rateLimiter *ratelimit.RateLimiter) (*Translator, error) {
	if prov == nil {
		return nil, fmt.Errorf("provider is required")
	}

	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if translationLanguage == "" {
		return nil, fmt.Errorf("translation language is required")
	}

	return &Translator{
		provider:          prov,
		model:             model,
		translationLanguage: translationLanguage,
		rateLimiter:       rateLimiter,
	}, nil
}

func (t *Translator) buildPrompt(text string) string {
	if t.translationLanguage == "" {
		return fmt.Sprintf("Translate the following text to English. Only output the translated text, nothing else.\n\nText to translate:\n%s", text)
	}
	return fmt.Sprintf("Translate the following text to %s. Only output the translated text, nothing else.\n\nText to translate:\n%s", t.translationLanguage, text)
}

func (t *Translator) StreamTranslate(ctx context.Context, text string, onChunk func(string)) error {
	if err := validation.ValidateTextInput(text, onChunk); err != nil {
		return err
	}

	// Apply rate limiting if enabled
	if t.rateLimiter != nil {
		if err := t.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit error: %w", err)
		}
	}

	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: t.buildPrompt(text),
		},
	}

	return t.provider.StreamChat(ctx, t.model, messages, onChunk)
}

// Translate performs a non-streaming translation (fallback)
func (t *Translator) Translate(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text cannot be empty")
	}

	if len(text) > validation.MaxInputLength {
		return "", fmt.Errorf("text exceeds maximum length of %d characters (got %d)", validation.MaxInputLength, len(text))
	}

	// Apply rate limiting if enabled
	if t.rateLimiter != nil {
		if err := t.rateLimiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit error: %w", err)
		}
	}

	messages := []provider.Message{
		{
			Role:    provider.RoleUser,
			Content: t.buildPrompt(text),
		},
	}

	return t.provider.Chat(ctx, t.model, messages)
}
