// Package llm provides Language Model provider implementations for Cortex.
// rate_limiter.go implements per-provider rate limiting to prevent API throttling.
package llm

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter manages API rate limits across multiple providers.
// It implements token bucket algorithm for smooth rate limiting.
// Brain Alignment: This mirrors the brain's resource management - preventing cognitive overload.
type RateLimiter struct {
	mu      sync.RWMutex
	limits  map[string]*ProviderLimits
	buckets map[string]*tokenBucket
	metrics map[string]*RateLimitMetrics
}

// ProviderLimits defines rate limits for a specific provider.
type ProviderLimits struct {
	// RequestsPerMinute limits API calls per minute
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requests_per_minute"`

	// TokensPerMinute limits tokens consumed per minute
	TokensPerMinute int `yaml:"tokens_per_minute" json:"tokens_per_minute"`

	// TokensPerDay limits daily token consumption
	TokensPerDay int64 `yaml:"tokens_per_day" json:"tokens_per_day"`

	// ConcurrentRequests limits parallel API calls
	ConcurrentRequests int `yaml:"concurrent_requests" json:"concurrent_requests"`

	// BurstSize allows temporary bursts above rate limit
	BurstSize int `yaml:"burst_size" json:"burst_size"`
}

// RateLimitMetrics tracks usage statistics for monitoring.
type RateLimitMetrics struct {
	TotalRequests   int64     `json:"total_requests"`
	TotalTokens     int64     `json:"total_tokens"`
	RejectedCount   int64     `json:"rejected_count"`
	LastRequestAt   time.Time `json:"last_request_at"`
	WindowStart     time.Time `json:"window_start"`
	RequestsInWindow int64    `json:"requests_in_window"`
	TokensInWindow   int64    `json:"tokens_in_window"`
}

// tokenBucket implements the token bucket algorithm for rate limiting.
type tokenBucket struct {
	mu            sync.Mutex
	tokens        float64
	maxTokens     float64
	refillRate    float64 // tokens per second
	lastRefill    time.Time
	activeCount   int // concurrent requests
	maxConcurrent int
	waiters       []chan struct{}
}

// DefaultProviderLimits returns default rate limits for known providers.
func DefaultProviderLimits(provider string) *ProviderLimits {
	switch provider {
	case "openai":
		return &ProviderLimits{
			RequestsPerMinute:  60,       // Tier 1 default
			TokensPerMinute:    90000,    // Tier 1 default
			TokensPerDay:       1000000,  // Conservative daily limit
			ConcurrentRequests: 5,
			BurstSize:          10,
		}
	case "anthropic":
		return &ProviderLimits{
			RequestsPerMinute:  60,
			TokensPerMinute:    80000,
			TokensPerDay:       1000000,
			ConcurrentRequests: 5,
			BurstSize:          10,
		}
	case "gemini":
		return &ProviderLimits{
			RequestsPerMinute:  60,
			TokensPerMinute:    100000,
			TokensPerDay:       1500000,
			ConcurrentRequests: 10,
			BurstSize:          15,
		}
	case "grok":
		return &ProviderLimits{
			RequestsPerMinute:  60,
			TokensPerMinute:    100000,
			TokensPerDay:       1000000,
			ConcurrentRequests: 5,
			BurstSize:          10,
		}
	case "groq":
		return &ProviderLimits{
			RequestsPerMinute:  30,  // Groq has stricter limits on free tier
			TokensPerMinute:    6000,
			TokensPerDay:       100000,
			ConcurrentRequests: 2,
			BurstSize:          5,
		}
	case "ollama":
		// Local models - no external rate limits, but prevent overload
		return &ProviderLimits{
			RequestsPerMinute:  120,
			TokensPerMinute:    500000,
			TokensPerDay:       50000000,
			ConcurrentRequests: 2, // Local inference is single-threaded typically
			BurstSize:          5,
		}
	default:
		return &ProviderLimits{
			RequestsPerMinute:  30,
			TokensPerMinute:    50000,
			TokensPerDay:       500000,
			ConcurrentRequests: 3,
			BurstSize:          5,
		}
	}
}

// NewRateLimiter creates a new rate limiter with default provider limits.
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		limits:  make(map[string]*ProviderLimits),
		buckets: make(map[string]*tokenBucket),
		metrics: make(map[string]*RateLimitMetrics),
	}

	// Initialize default limits for known providers
	for _, provider := range []string{"openai", "anthropic", "gemini", "grok", "groq", "ollama"} {
		rl.SetLimits(provider, DefaultProviderLimits(provider))
	}

	return rl
}

// SetLimits configures rate limits for a provider.
func (r *RateLimiter) SetLimits(provider string, limits *ProviderLimits) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.limits[provider] = limits

	// Create/update token bucket
	refillRate := float64(limits.RequestsPerMinute) / 60.0 // requests per second
	maxTokens := float64(limits.BurstSize)
	if maxTokens < 1 {
		maxTokens = float64(limits.RequestsPerMinute) / 6.0 // 10 second burst
	}

	r.buckets[provider] = &tokenBucket{
		tokens:        maxTokens,
		maxTokens:     maxTokens,
		refillRate:    refillRate,
		lastRefill:    time.Now(),
		maxConcurrent: limits.ConcurrentRequests,
		waiters:       make([]chan struct{}, 0),
	}

	// Initialize metrics
	if _, exists := r.metrics[provider]; !exists {
		r.metrics[provider] = &RateLimitMetrics{
			WindowStart: time.Now(),
		}
	}
}

// Acquire attempts to acquire a rate limit slot for the provider.
// It blocks until a slot is available or context is cancelled.
// Returns error if rate limit is exceeded or context cancelled.
func (r *RateLimiter) Acquire(ctx context.Context, provider string, estimatedTokens int) error {
	r.mu.RLock()
	bucket, exists := r.buckets[provider]
	limits := r.limits[provider]
	metrics := r.metrics[provider]
	r.mu.RUnlock()

	if !exists {
		// No limits configured, allow request
		return nil
	}

	// Check daily token limit
	if limits.TokensPerDay > 0 && metrics != nil {
		r.mu.RLock()
		dailyUsed := metrics.TotalTokens
		r.mu.RUnlock()

		if dailyUsed+int64(estimatedTokens) > limits.TokensPerDay {
			r.recordRejection(provider)
			return fmt.Errorf("daily token limit exceeded for %s: used %d of %d",
				provider, dailyUsed, limits.TokensPerDay)
		}
	}

	// Try to acquire from token bucket
	return bucket.acquire(ctx)
}

// Release releases a rate limit slot after request completion.
// Should be called in defer after successful Acquire.
func (r *RateLimiter) Release(provider string) {
	r.mu.RLock()
	bucket, exists := r.buckets[provider]
	r.mu.RUnlock()

	if exists {
		bucket.release()
	}
}

// RecordUsage records actual token usage after a request completes.
func (r *RateLimiter) RecordUsage(provider string, tokens int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	metrics, exists := r.metrics[provider]
	if !exists {
		metrics = &RateLimitMetrics{WindowStart: time.Now()}
		r.metrics[provider] = metrics
	}

	now := time.Now()
	metrics.TotalRequests++
	metrics.TotalTokens += int64(tokens)
	metrics.LastRequestAt = now

	// Reset window if more than 1 minute has passed
	if now.Sub(metrics.WindowStart) > time.Minute {
		metrics.WindowStart = now
		metrics.RequestsInWindow = 1
		metrics.TokensInWindow = int64(tokens)
	} else {
		metrics.RequestsInWindow++
		metrics.TokensInWindow += int64(tokens)
	}
}

// GetMetrics returns current rate limit metrics for a provider.
func (r *RateLimiter) GetMetrics(provider string) *RateLimitMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, exists := r.metrics[provider]; exists {
		// Return a copy to prevent race conditions
		copy := *m
		return &copy
	}
	return nil
}

// GetAllMetrics returns metrics for all providers.
func (r *RateLimiter) GetAllMetrics() map[string]*RateLimitMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*RateLimitMetrics)
	for k, v := range r.metrics {
		copy := *v
		result[k] = &copy
	}
	return result
}

// ResetDaily resets daily counters (call at midnight or startup).
func (r *RateLimiter) ResetDaily() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, m := range r.metrics {
		m.TotalTokens = 0
		m.TotalRequests = 0
		m.RejectedCount = 0
	}
}

// CanProceed checks if a request can proceed without blocking.
func (r *RateLimiter) CanProceed(provider string, estimatedTokens int) bool {
	r.mu.RLock()
	bucket, exists := r.buckets[provider]
	limits := r.limits[provider]
	metrics := r.metrics[provider]
	r.mu.RUnlock()

	if !exists {
		return true
	}

	// Check daily limit
	if limits.TokensPerDay > 0 && metrics != nil {
		if metrics.TotalTokens+int64(estimatedTokens) > limits.TokensPerDay {
			return false
		}
	}

	return bucket.canProceed()
}

// WaitTime returns estimated wait time before a request can proceed.
func (r *RateLimiter) WaitTime(provider string) time.Duration {
	r.mu.RLock()
	bucket, exists := r.buckets[provider]
	r.mu.RUnlock()

	if !exists {
		return 0
	}

	return bucket.waitTime()
}

// recordRejection increments the rejection counter.
func (r *RateLimiter) recordRejection(provider string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, exists := r.metrics[provider]; exists {
		m.RejectedCount++
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TOKEN BUCKET IMPLEMENTATION
// ─────────────────────────────────────────────────────────────────────────────

// acquire blocks until a token is available or context is cancelled.
func (tb *tokenBucket) acquire(ctx context.Context) error {
	tb.mu.Lock()

	// Refill bucket based on elapsed time
	tb.refill()

	// Check concurrent limit
	if tb.maxConcurrent > 0 && tb.activeCount >= tb.maxConcurrent {
		// Wait for a slot
		waiter := make(chan struct{})
		tb.waiters = append(tb.waiters, waiter)
		tb.mu.Unlock()

		select {
		case <-waiter:
			tb.mu.Lock()
			tb.activeCount++
			tb.mu.Unlock()
			return nil
		case <-ctx.Done():
			// Remove from waiters
			tb.mu.Lock()
			for i, w := range tb.waiters {
				if w == waiter {
					tb.waiters = append(tb.waiters[:i], tb.waiters[i+1:]...)
					break
				}
			}
			tb.mu.Unlock()
			return ctx.Err()
		}
	}

	// Check token availability
	if tb.tokens < 1 {
		// Calculate wait time
		waitTime := time.Duration((1 - tb.tokens) / tb.refillRate * float64(time.Second))
		tb.mu.Unlock()

		select {
		case <-time.After(waitTime):
			tb.mu.Lock()
			tb.refill()
			if tb.tokens >= 1 {
				tb.tokens--
				tb.activeCount++
				tb.mu.Unlock()
				return nil
			}
			tb.mu.Unlock()
			return fmt.Errorf("rate limit exceeded after wait")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	tb.tokens--
	tb.activeCount++
	tb.mu.Unlock()
	return nil
}

// release returns a slot to the bucket.
func (tb *tokenBucket) release() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.activeCount--

	// Notify a waiter if any
	if len(tb.waiters) > 0 {
		waiter := tb.waiters[0]
		tb.waiters = tb.waiters[1:]
		close(waiter)
	}
}

// refill adds tokens based on elapsed time (must be called with lock held).
func (tb *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now
}

// canProceed checks if a request can proceed immediately.
func (tb *tokenBucket) canProceed() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.maxConcurrent > 0 && tb.activeCount >= tb.maxConcurrent {
		return false
	}

	return tb.tokens >= 1
}

// waitTime returns estimated time until a token is available.
func (tb *tokenBucket) waitTime() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens >= 1 {
		return 0
	}

	return time.Duration((1 - tb.tokens) / tb.refillRate * float64(time.Second))
}
