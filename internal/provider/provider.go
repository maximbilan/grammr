package provider

import (
	"context"
)

// Provider defines the interface for AI providers (OpenAI, Anthropic, etc.)
type Provider interface {
	// StreamChat streams a chat completion response
	StreamChat(ctx context.Context, model string, messages []Message, onChunk func(string)) error
	
	// Chat performs a non-streaming chat completion
	Chat(ctx context.Context, model string, messages []Message) (string, error)
}

// Message represents a chat message
type Message struct {
	Role    string
	Content string
}

// Role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)
