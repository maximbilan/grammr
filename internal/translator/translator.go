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
		return fmt.Errorf("stream error: %w", err)
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream recv error: %w", err)
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
		return "", fmt.Errorf("completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return resp.Choices[0].Message.Content, nil
}
