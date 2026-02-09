package translator

import (
	"context"
	"fmt"
	"io"

	"github.com/sashabaranov/go-openai"
)

type Translator struct {
	client            *openai.Client
	model             string
	translationLanguage string
}

func New(apiKey, model, translationLanguage string) (*Translator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
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
	}, nil
}

func (t *Translator) buildPrompt(text string) string {
	if t.translationLanguage == "" {
		return fmt.Sprintf("Translate the following text to English. Only output the translated text, nothing else.\n\nText to translate:\n%s", text)
	}
	return fmt.Sprintf("Translate the following text to %s. Only output the translated text, nothing else.\n\nText to translate:\n%s", t.translationLanguage, text)
}

func (t *Translator) StreamTranslate(ctx context.Context, text string, onChunk func(string)) error {
	if text == "" {
		return fmt.Errorf("text cannot be empty")
	}

	if onChunk == nil {
		return fmt.Errorf("onChunk callback cannot be nil")
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
