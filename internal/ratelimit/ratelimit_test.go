package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewClampsRefillRate(t *testing.T) {
	rl := New(1_000_000, time.Millisecond, 0)
	if rl.refillRate <= 0 {
		t.Fatalf("refillRate must be > 0, got %v", rl.refillRate)
	}
}

func TestWaitContextCancelDoesNotPanic(t *testing.T) {
	rl := New(1, time.Minute, time.Second)

	// Consume the only token so the second call must wait.
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("first Wait() failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Wait() panicked on cancelled context: %v", r)
		}
	}()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestWaitHandlesMalformedLimiter(t *testing.T) {
	// Simulate a malformed limiter created outside New().
	rl := &RateLimiter{
		tokens:      0,
		maxTokens:   0,
		refillRate:  0,
		lastRefill:  time.Now(),
		minInterval: time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Wait() panicked with zero refillRate: %v", r)
		}
	}()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
