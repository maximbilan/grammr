package provider

import (
	"context"
	"fmt"
)

// MockProvider is a simple mock provider for testing
type MockProvider struct {
	responses map[string]string
}

// NewMockProvider creates a new mock provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		responses: make(map[string]string),
	}
}

// SetResponse sets a mock response for a given prompt
func (m *MockProvider) SetResponse(prompt, response string) {
	m.responses[prompt] = response
}

// StreamChat streams a chat completion response
func (m *MockProvider) StreamChat(ctx context.Context, model string, messages []Message, onChunk func(string)) error {
	if len(messages) == 0 {
		return fmt.Errorf("no messages provided")
	}
	
	// Get the last user message
	var prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			prompt = messages[i].Content
			break
		}
	}
	
	response, ok := m.responses[prompt]
	if !ok {
		response = "Mock response for: " + prompt
	}
	
	// Stream the response character by character
	for _, char := range response {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			onChunk(string(char))
		}
	}
	
	return nil
}

// Chat performs a non-streaming chat completion
func (m *MockProvider) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("no messages provided")
	}
	
	// Get the last user message
	var prompt string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == RoleUser {
			prompt = messages[i].Content
			break
		}
	}
	
	response, ok := m.responses[prompt]
	if !ok {
		response = "Mock response for: " + prompt
	}
	
	return response, nil
}
