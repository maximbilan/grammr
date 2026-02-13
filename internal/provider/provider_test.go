package provider

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid API key",
			apiKey:  "sk-test1234567890123456789012345678901234567890",
			wantErr: false,
		},
		{
			name:    "empty API key",
			apiKey:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewOpenAIProvider() returned nil provider without error")
			}
		})
	}
}

func TestNewAnthropicProvider(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		wantErr bool
	}{
		{
			name:    "valid API key",
			apiKey:  "sk-ant-test1234567890123456789012345678901234567890",
			wantErr: false,
		},
		{
			name:    "empty API key",
			apiKey:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAnthropicProvider(tt.apiKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAnthropicProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("NewAnthropicProvider() returned nil provider without error")
			}
		})
	}
}

func TestMockProvider(t *testing.T) {
	t.Run("Chat with mock provider", func(t *testing.T) {
		mock := NewMockProvider()
		mock.SetResponse("test prompt", "test response")

		ctx := context.Background()
		messages := []Message{
			{Role: RoleUser, Content: "test prompt"},
		}

		result, err := mock.Chat(ctx, "test-model", messages)
		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}

		if !strings.Contains(result, "test response") {
			t.Errorf("Chat() = %v, want to contain 'test response'", result)
		}
	})

	t.Run("StreamChat with mock provider", func(t *testing.T) {
		mock := NewMockProvider()
		mock.SetResponse("test prompt", "test response")

		ctx := context.Background()
		messages := []Message{
			{Role: RoleUser, Content: "test prompt"},
		}

		var chunks []string
		err := mock.StreamChat(ctx, "test-model", messages, func(chunk string) {
			chunks = append(chunks, chunk)
		})

		if err != nil {
			t.Fatalf("StreamChat() error = %v", err)
		}

		result := strings.Join(chunks, "")
		if !strings.Contains(result, "test response") {
			t.Errorf("StreamChat() = %v, want to contain 'test response'", result)
		}
	})

	t.Run("StreamChat with empty messages returns error", func(t *testing.T) {
		mock := NewMockProvider()
		err := mock.StreamChat(context.Background(), "test-model", nil, func(string) {})
		if err == nil {
			t.Fatal("StreamChat() with empty messages should return error")
		}
	})

	t.Run("StreamChat respects context cancellation", func(t *testing.T) {
		mock := NewMockProvider()
		mock.SetResponse("test prompt", strings.Repeat("x", 20))

		ctx, cancel := context.WithCancel(context.Background())
		messages := []Message{{Role: RoleUser, Content: "test prompt"}}

		calls := 0
		err := mock.StreamChat(ctx, "test-model", messages, func(string) {
			calls++
			if calls == 1 {
				cancel()
			}
		})
		if err == nil {
			t.Fatal("expected StreamChat() cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Mock provider with no response set", func(t *testing.T) {
		mock := NewMockProvider()

		ctx := context.Background()
		messages := []Message{
			{Role: RoleUser, Content: "unknown prompt"},
		}

		result, err := mock.Chat(ctx, "test-model", messages)
		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}

		if !strings.Contains(result, "Mock response for: unknown prompt") {
			t.Errorf("Chat() = %v, want to contain 'Mock response for: unknown prompt'", result)
		}
	})

	t.Run("Mock provider with empty messages", func(t *testing.T) {
		mock := NewMockProvider()

		ctx := context.Background()
		messages := []Message{}

		_, err := mock.Chat(ctx, "test-model", messages)
		if err == nil {
			t.Error("Chat() with empty messages should return error")
		}
	})

	t.Run("Chat with no user messages uses empty prompt fallback", func(t *testing.T) {
		mock := NewMockProvider()
		ctx := context.Background()
		messages := []Message{
			{Role: RoleAssistant, Content: "assistant only"},
		}

		result, err := mock.Chat(ctx, "test-model", messages)
		if err != nil {
			t.Fatalf("Chat() error = %v", err)
		}
		if !strings.Contains(result, "Mock response for: ") {
			t.Fatalf("unexpected fallback response: %q", result)
		}
	})

	t.Run("StreamChat with no user messages uses empty prompt fallback", func(t *testing.T) {
		mock := NewMockProvider()
		ctx := context.Background()
		messages := []Message{
			{Role: RoleAssistant, Content: "assistant only"},
		}

		var chunks []string
		err := mock.StreamChat(ctx, "test-model", messages, func(chunk string) {
			chunks = append(chunks, chunk)
		})
		if err != nil {
			t.Fatalf("StreamChat() error = %v", err)
		}

		result := strings.Join(chunks, "")
		if !strings.Contains(result, "Mock response for: ") {
			t.Fatalf("unexpected fallback stream response: %q", result)
		}
	})
}

func TestProviderInterface(t *testing.T) {
	// Test that both providers implement the Provider interface
	t.Run("OpenAIProvider implements Provider", func(t *testing.T) {
		var _ Provider = (*OpenAIProvider)(nil)
	})

	t.Run("AnthropicProvider implements Provider", func(t *testing.T) {
		var _ Provider = (*AnthropicProvider)(nil)
	})

	t.Run("MockProvider implements Provider", func(t *testing.T) {
		var _ Provider = (*MockProvider)(nil)
	})
}

func TestToOpenAIMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "user"},
		{Role: RoleAssistant, Content: "assistant"},
		{Role: "unknown", Content: "ignored"},
	}

	got := toOpenAIMessages(messages)
	if len(got) != 3 {
		t.Fatalf("toOpenAIMessages() length = %d, want 3", len(got))
	}

	for i, msg := range got {
		raw, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("failed to marshal openai message %d: %v", i, err)
		}
		if i == 0 && !strings.Contains(string(raw), `"role":"system"`) {
			t.Fatalf("expected system role in first message, got %s", string(raw))
		}
		if i == 1 && !strings.Contains(string(raw), `"role":"user"`) {
			t.Fatalf("expected user role in second message, got %s", string(raw))
		}
		if i == 2 && !strings.Contains(string(raw), `"role":"assistant"`) {
			t.Fatalf("expected assistant role in third message, got %s", string(raw))
		}
	}
}

func TestToAnthropicMessages(t *testing.T) {
	messages := []Message{
		{Role: RoleSystem, Content: "system-1"},
		{Role: RoleUser, Content: "user-1"},
		{Role: RoleAssistant, Content: "assistant-1"},
		{Role: RoleSystem, Content: "system-2"},
		{Role: "unknown", Content: "ignored"},
	}

	anthropicMessages, systemPrompt := toAnthropicMessages(messages)

	if len(systemPrompt) != 2 {
		t.Fatalf("systemPrompt length = %d, want 2", len(systemPrompt))
	}
	if systemPrompt[0].Text != "system-1" || systemPrompt[1].Text != "system-2" {
		t.Fatalf("unexpected system prompt values: %+v", systemPrompt)
	}

	if len(anthropicMessages) != 2 {
		t.Fatalf("anthropicMessages length = %d, want 2", len(anthropicMessages))
	}
}
