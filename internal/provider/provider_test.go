package provider

import (
	"context"
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
