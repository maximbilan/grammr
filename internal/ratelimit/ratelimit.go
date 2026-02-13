package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter to prevent excessive API calls
type RateLimiter struct {
	mu          sync.Mutex
	tokens      int           // Current number of tokens available
	maxTokens   int           // Maximum number of tokens
	refillRate  time.Duration // Time between token refills
	lastRefill  time.Time     // Last time tokens were refilled
	minInterval time.Duration // Minimum time between requests
	lastRequest time.Time     // Last request time
}

// New creates a new rate limiter
// maxRequests: maximum number of requests allowed
// perDuration: time window for maxRequests (e.g., 60 requests per minute)
// minInterval: minimum time between requests (prevents burst requests)
func New(maxRequests int, perDuration time.Duration, minInterval time.Duration) *RateLimiter {
	if maxRequests <= 0 {
		maxRequests = 60 // Default: 60 requests
	}
	if perDuration <= 0 {
		perDuration = time.Minute // Default: per minute
	}
	if minInterval <= 0 {
		minInterval = 100 * time.Millisecond // Default: 100ms between requests
	}

	refillRate := perDuration / time.Duration(maxRequests)
	if refillRate <= 0 {
		// Defensive clamp for extreme config values (e.g. 1ms window, huge request count).
		refillRate = time.Nanosecond
	}

	return &RateLimiter{
		tokens:      maxRequests,
		maxTokens:   maxRequests,
		refillRate:  refillRate,
		lastRefill:  time.Now(),
		minInterval: minInterval,
		lastRequest: time.Time{},
	}
}

// Wait blocks until a token is available, respecting rate limits
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()

		// Defensive guard: prevent division-by-zero if a malformed limiter is constructed.
		if rl.refillRate <= 0 {
			rl.refillRate = time.Nanosecond
		}

		now := time.Now()

		// Refill tokens based on elapsed time.
		elapsed := now.Sub(rl.lastRefill)
		if elapsed > 0 {
			tokensToAdd := int(elapsed / rl.refillRate)
			if tokensToAdd > 0 {
				rl.tokens = min(rl.maxTokens, rl.tokens+tokensToAdd)
				rl.lastRefill = now
			}
		}

		// Check minimum interval between requests.
		if !rl.lastRequest.IsZero() {
			timeSinceLastRequest := now.Sub(rl.lastRequest)
			if timeSinceLastRequest < rl.minInterval {
				waitTime := rl.minInterval - timeSinceLastRequest
				rl.mu.Unlock()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(waitTime):
				}
				continue
			}
		}

		// Consume a token if available.
		if rl.tokens > 0 {
			rl.tokens--
			rl.lastRequest = now
			rl.mu.Unlock()
			return nil
		}

		// Wait for next token availability.
		nextTokenTime := rl.lastRefill.Add(rl.refillRate)
		waitTime := nextTokenTime.Sub(now)
		if waitTime <= 0 {
			waitTime = rl.refillRate
			if waitTime <= 0 {
				waitTime = time.Nanosecond
			}
		}

		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return fmt.Errorf("rate limit wait cancelled: %w", ctx.Err())
		case <-time.After(waitTime):
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
