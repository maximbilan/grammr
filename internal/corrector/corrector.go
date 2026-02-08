package corrector

import (
	"context"
	"fmt"
	"io"

	"github.com/sashabaranov/go-openai"
)

type Corrector struct {
	client   *openai.Client
	model    string
	mode     string
	language string
}

func New(apiKey, model, mode, language string) (*Corrector, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Default to English if language is empty
	if language == "" {
		language = "english"
	}

	return &Corrector{
		client:   openai.NewClient(apiKey),
		model:    model,
		mode:     mode,
		language: language,
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

// Correct performs a non-streaming correction (fallback)
func (c *Corrector) Correct(ctx context.Context, text string) (string, error) {
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
		return "", fmt.Errorf("completion error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	return resp.Choices[0].Message.Content, nil
}
