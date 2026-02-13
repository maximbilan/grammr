package translator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/maximbilan/grammr/internal/provider"
	"github.com/maximbilan/grammr/internal/ratelimit"
	"github.com/maximbilan/grammr/internal/validation"
)

func TestNewWithRateLimit(t *testing.T) {
	tests := []struct {
		name                string
		prov                provider.Provider
		model               string
		translationLanguage string
		wantErr             bool
	}{
		{
			name:                "valid translator",
			prov:                provider.NewMockProvider(),
			model:               "gpt-4o",
			translationLanguage: "french",
			wantErr:             false,
		},
		{
			name:                "nil provider",
			prov:                nil,
			model:               "gpt-4o",
			translationLanguage: "french",
			wantErr:             true,
		},
		{
			name:                "empty model",
			prov:                provider.NewMockProvider(),
			model:               "",
			translationLanguage: "french",
			wantErr:             true,
		},
		{
			name:                "empty translation language",
			prov:                provider.NewMockProvider(),
			model:               "gpt-4o",
			translationLanguage: "",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewWithRateLimit(tt.prov, tt.model, tt.translationLanguage, nil)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewWithRateLimit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got == nil {
				t.Fatal("NewWithRateLimit() returned nil translator without error")
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Run("uses configured target language", func(t *testing.T) {
		tr := &Translator{translationLanguage: "spanish"}
		prompt := tr.buildPrompt("Hello world")
		if !strings.Contains(strings.ToLower(prompt), "to spanish") {
			t.Fatalf("expected prompt to include target language, got: %q", prompt)
		}
		if !strings.Contains(prompt, "Hello world") {
			t.Fatalf("expected prompt to include source text, got: %q", prompt)
		}
	})

	t.Run("falls back to english when language empty", func(t *testing.T) {
		tr := &Translator{translationLanguage: ""}
		prompt := tr.buildPrompt("Bonjour")
		if !strings.Contains(strings.ToLower(prompt), "to english") {
			t.Fatalf("expected english fallback prompt, got: %q", prompt)
		}
	})
}

func TestTranslate(t *testing.T) {
	mock := provider.NewMockProvider()
	tr, err := NewWithRateLimit(mock, "gpt-4o", "french", nil)
	if err != nil {
		t.Fatalf("NewWithRateLimit() error = %v", err)
	}

	text := "Hello world"
	prompt := tr.buildPrompt(text)
	mock.SetResponse(prompt, "Bonjour le monde")

	got, err := tr.Translate(context.Background(), text)
	if err != nil {
		t.Fatalf("Translate() error = %v", err)
	}
	if got != "Bonjour le monde" {
		t.Fatalf("Translate() = %q, want %q", got, "Bonjour le monde")
	}
}

func TestStreamTranslate(t *testing.T) {
	mock := provider.NewMockProvider()
	tr, err := NewWithRateLimit(mock, "gpt-4o", "german", nil)
	if err != nil {
		t.Fatalf("NewWithRateLimit() error = %v", err)
	}

	text := "How are you?"
	prompt := tr.buildPrompt(text)
	mock.SetResponse(prompt, "Wie geht es dir?")

	var chunks []string
	err = tr.StreamTranslate(context.Background(), text, func(chunk string) {
		chunks = append(chunks, chunk)
	})
	if err != nil {
		t.Fatalf("StreamTranslate() error = %v", err)
	}
	if strings.Join(chunks, "") != "Wie geht es dir?" {
		t.Fatalf("StreamTranslate() output = %q, want %q", strings.Join(chunks, ""), "Wie geht es dir?")
	}
}

func TestTranslateValidationAndRateLimitErrors(t *testing.T) {
	mock := provider.NewMockProvider()
	rl := ratelimit.New(1, time.Minute, time.Second)
	tr, err := NewWithRateLimit(mock, "gpt-4o", "italian", rl)
	if err != nil {
		t.Fatalf("NewWithRateLimit() error = %v", err)
	}

	t.Run("empty text", func(t *testing.T) {
		_, err := tr.Translate(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty text")
		}
	})

	t.Run("text too long", func(t *testing.T) {
		longText := strings.Repeat("a", validation.MaxInputLength+1)
		_, err := tr.Translate(context.Background(), longText)
		if err == nil {
			t.Fatal("expected error for oversized text")
		}
	})

	t.Run("rate limit cancellation", func(t *testing.T) {
		if err := rl.Wait(context.Background()); err != nil {
			t.Fatalf("initial Wait() failed: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := tr.Translate(ctx, "hello")
		if err == nil {
			t.Fatal("expected rate limit cancellation error")
		}
		if !strings.Contains(err.Error(), "rate limit error") {
			t.Fatalf("expected wrapped rate limit error, got %v", err)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestStreamTranslateValidationErrors(t *testing.T) {
	mock := provider.NewMockProvider()
	tr, err := NewWithRateLimit(mock, "gpt-4o", "japanese", nil)
	if err != nil {
		t.Fatalf("NewWithRateLimit() error = %v", err)
	}

	err = tr.StreamTranslate(context.Background(), "", func(string) {})
	if err == nil {
		t.Fatal("expected error for empty text")
	}

	err = tr.StreamTranslate(context.Background(), "hello", nil)
	if err == nil {
		t.Fatal("expected error for nil callback")
	}
}
