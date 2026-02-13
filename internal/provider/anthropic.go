package provider

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements Provider using Anthropic's API
type AnthropicProvider struct {
	client anthropic.Client
}

func toAnthropicMessages(messages []Message) ([]anthropic.MessageParam, []anthropic.TextBlockParam) {
	anthropicMessages := make([]anthropic.MessageParam, 0, len(messages))
	systemPrompt := make([]anthropic.TextBlockParam, 0)

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			systemPrompt = append(systemPrompt, anthropic.TextBlockParam{Text: msg.Content})
			continue
		}
		// Anthropic uses "user" and "assistant" roles.
		if msg.Role == RoleUser {
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		} else if msg.Role == RoleAssistant {
			anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}

	return anthropicMessages, systemPrompt
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey string) (*AnthropicProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	return &AnthropicProvider{
		client: client,
	}, nil
}

// StreamChat streams a chat completion response
func (p *AnthropicProvider) StreamChat(ctx context.Context, model string, messages []Message, onChunk func(string)) error {
	anthropicMessages, systemPrompt := toAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		Messages:  anthropicMessages,
	}

	if len(systemPrompt) > 0 {
		params.System = systemPrompt
	}

	stream := p.client.Messages.NewStreaming(ctx, params)
	message := anthropic.Message{}

	for stream.Next() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return fmt.Errorf("failed to accumulate message: %w", err)
		}

		// Handle content block delta events
		switch eventVariant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch deltaVariant := eventVariant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				if deltaVariant.Text != "" {
					onChunk(deltaVariant.Text)
				}
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// Chat performs a non-streaming chat completion
func (p *AnthropicProvider) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	anthropicMessages, systemPrompt := toAnthropicMessages(messages)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		Messages:  anthropicMessages,
	}

	if len(systemPrompt) > 0 {
		params.System = systemPrompt
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(resp.Content) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	// Extract text content from the first content block
	if textBlock, ok := resp.Content[0].AsAny().(anthropic.TextBlock); ok {
		return textBlock.Text, nil
	}

	return "", fmt.Errorf("unexpected response format")
}
