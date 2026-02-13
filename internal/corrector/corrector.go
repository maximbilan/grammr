package corrector

import (
	"context"
	"fmt"
	"io"

	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/sashabaranov/go-openai"
)

const (
	// MaxInputLength is the maximum allowed length for input text (100K characters)
	// This prevents excessive API costs and potential memory issues
	MaxInputLength = 100000
)

type Corrector struct {
	client     *openai.Client
	model      string
	mode       string
	language   string
	rateLimiter *ratelimit.RateLimiter
}

func New(apiKey, model, mode, language string) (*Corrector, error) {
	return NewWithRateLimit(apiKey, model, mode, language, nil)
}

// NewWithRateLimit creates a new Corrector with an optional rate limiter
func NewWithRateLimit(apiKey, model, mode, language string, rateLimiter *ratelimit.RateLimiter) (*Corrector, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if model == "" {
		return nil, fmt.Errorf("model is required")
	}

	// Default to English if language is empty
	if language == "" {
		language = "english"
	}

	// Validate mode
	validModes := map[string]bool{
		"casual":    true,
		"formal":    true,
		"academic":  true,
		"technical": true,
	}
	if mode != "" && !validModes[mode] {
		// Default to casual for invalid modes
		mode = "casual"
	}

	return &Corrector{
		client:      openai.NewClient(apiKey),
		model:       model,
		mode:        mode,
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

	prompt, ok := prompts[c.mode]
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
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	if len(text) > MaxInputLength {
		return fmt.Errorf("text exceeds maximum length of %d characters (got %d)", MaxInputLength, len(text))
	}

	if onChunk == nil {
		return fmt.Errorf("onChunk callback cannot be nil")
	}

	// Apply rate limiting if enabled
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit error: %w", err)
		}
	}

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: c.buildPrompt(text),
			},
		},
		Stream: true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
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

// Correct performs a non-streaming correction (fallback)
func (c *Corrector) Correct(ctx context.Context, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text cannot be empty")
	}

	if len(text) > MaxInputLength {
		return "", fmt.Errorf("text exceeds maximum length of %d characters (got %d)", MaxInputLength, len(text))
	}

	// Apply rate limiting if enabled
	if c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limit error: %w", err)
		}
	}

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: c.buildPrompt(text),
			},
		},
	}

	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return resp.Choices[0].Message.Content, nil
}
