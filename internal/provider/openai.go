package provider

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAIProvider implements Provider using OpenAI's API
type OpenAIProvider struct {
	client openai.Client
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	return &OpenAIProvider{
		client: openai.NewClient(option.WithAPIKey(apiKey)),
	}, nil
}

// StreamChat streams a chat completion response
func (p *OpenAIProvider) StreamChat(ctx context.Context, model string, messages []Message, onChunk func(string)) error {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content))
		case RoleAssistant:
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content))
		case RoleSystem:
			// System messages are included in the messages array
			openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: openaiMessages,
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	for stream.Next() {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			if chunk.Choices[0].Delta.Content != "" {
				onChunk(chunk.Choices[0].Delta.Content)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	return nil
}

// Chat performs a non-streaming chat completion
func (p *OpenAIProvider) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	// Convert messages to OpenAI format
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case RoleUser:
			openaiMessages = append(openaiMessages, openai.UserMessage(msg.Content))
		case RoleAssistant:
			openaiMessages = append(openaiMessages, openai.AssistantMessage(msg.Content))
		case RoleSystem:
			// System messages are included in the messages array
			openaiMessages = append(openaiMessages, openai.SystemMessage(msg.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: openaiMessages,
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to create completion: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from API")
	}

	// Extract content from the message
	content := resp.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("empty response from API")
	}

	return content, nil
}
