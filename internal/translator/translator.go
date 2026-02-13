package translator

import (
	"context"
	"fmt"
	"io"

	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/maximbilan/grammr/internal/validation"
	"github.com/sashabaranov/go-openai"
)


type Translator struct {
	client            *openai.Client
	model             string
	translationLanguage string
	rateLimiter       *ratelimit.RateLimiter
}

// NewWithRateLimit creates a new Translator with an optional rate limiter
func NewWithRateLimit(apiKey, model, translationLanguage string, rateLimiter *ratelimit.RateLimiter) (*Translator, error) {
	if err := validation.ValidateAPIKey(apiKey); err != nil {
		return nil, err
	}

	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if translationLanguage == "" {
		return nil, fmt.Errorf("translation language is required")
	}

	return &Translator{
		client:            openai.NewClient(apiKey),
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

	req := openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: t.buildPrompt(text),
			},
		},
		Stream: true,
	}

	stream, err := t.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer func() {
		stream.Close() // Ignore close errors
	}()

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream receive error: %w", err)
		}

		if len(response.Choices) > 0 {
			chunk := response.Choices[0].Delta.Content
			if chunk != "" {
				onChunk(chunk)
			}
		}
	}

	return nil
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

	req := openai.ChatCompletionRequest{
		Model: t.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: t.buildPrompt(text),
			},
		},
	}

	resp, err := t.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return resp.Choices[0].Message.Content, nil
}
