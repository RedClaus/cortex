// CR-005: Vision Router with Two-Lane Routing
// Part of the Visual Cortex implementation for Cortex.
//
// Routing Strategy:
// - Simple prompts (what is this?, describe, classify) → Fast Lane (Moondream)
// - Complex prompts (read, code, debug, error, analyze) → Smart Lane (MiniCPM-V)
// - Smart Lane unavailable → Automatic fallback to Fast Lane
package vision

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// Router selects the appropriate vision model based on prompt intent.
// It provides automatic fallback when MiniCPM-V is unavailable.
//
// Hot/Cold Path Architecture:
// - Hot Path (Fast Lane): Moondream for quick queries
// - Cold Path (Smart Lane): MiniCPM-V for OCR/analysis (masked by thinking)
type Router struct {
	fastProvider  Provider // Moondream (always available, CPU-friendly)
	smartProvider Provider // MiniCPM-V (may be nil if GPU unavailable)

	config Config

	// Health check caching (avoid hammering Ollama)
	smartHealthy        atomic.Bool
	lastHealthCheck     time.Time
	healthCheckMu       sync.Mutex
	healthCheckInterval time.Duration

	mu sync.RWMutex
}

// NewRouter creates a vision router with fallback support.
// The smart provider may be nil if GPU is unavailable.
func NewRouter(fast, smart Provider, config Config) *Router {
	// Validate and apply defaults
	config.Validate()

	r := &Router{
		fastProvider:        fast,
		smartProvider:       smart,
		config:              config,
		healthCheckInterval: config.HealthCheckInterval,
	}

	// Initial health check for smart provider
	if smart != nil {
		r.smartHealthy.Store(smart.IsHealthy())
		log.Info().
			Bool("smart_available", r.smartHealthy.Load()).
			Str("fast_model", config.FastModel).
			Str("smart_model", config.SmartModel).
			Msg("vision router initialized")
	} else {
		r.smartHealthy.Store(false)
		log.Info().
			Str("fast_model", config.FastModel).
			Msg("vision router initialized (fast lane only)")
	}

	return r
}

// Analyze routes the request to the appropriate provider based on prompt intent.
func (r *Router) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	if !r.config.Enabled {
		return nil, ErrVisionDisabled
	}

	// Validate request
	if err := ValidateRequest(req, r.config.MaxImageSizeMB); err != nil {
		return nil, err
	}

	// Select provider based on prompt intent
	provider, usedFallback := r.selectProvider(ctx, req.Prompt)
	if provider == nil {
		return nil, ErrProviderUnavailable
	}

	// Set appropriate timeout based on selected provider
	timeout := r.config.FastModelTimeout
	if provider == r.smartProvider {
		timeout = r.config.SmartModelTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Perform analysis
	start := time.Now()
	resp, err := provider.Analyze(ctx, req)
	if err != nil {
		// Attempt fallback to fast provider if smart failed
		if r.shouldFallback(provider, usedFallback, err) {
			log.Warn().
				Err(err).
				Str("provider", provider.Name()).
				Msg("smart vision failed, falling back to fast")

			r.smartHealthy.Store(false)
			provider = r.fastProvider
			usedFallback = true

			// Reset timeout for fast provider
			ctx, cancel = context.WithTimeout(context.Background(), r.config.FastModelTimeout)
			defer cancel()

			resp, err = provider.Analyze(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("all vision providers failed: %w", err)
			}
		} else {
			return nil, err
		}
	}

	// Populate response metadata
	resp.Provider = provider.Name()
	resp.ProcessedMs = time.Since(start).Milliseconds()
	resp.UsedFallback = usedFallback

	log.Debug().
		Str("provider", resp.Provider).
		Int64("processing_ms", resp.ProcessedMs).
		Bool("used_fallback", resp.UsedFallback).
		Msg("vision analysis completed")

	return resp, nil
}

// shouldFallback determines if we should try the fast provider after smart failed.
func (r *Router) shouldFallback(failedProvider Provider, alreadyUsedFallback bool, err error) bool {
	// Don't fallback if disabled
	if !r.config.EnableFallback {
		return false
	}

	// Don't fallback if we already tried
	if alreadyUsedFallback {
		return false
	}

	// Don't fallback if the fast provider failed (nothing left to try)
	if r.fastProvider == nil || failedProvider == r.fastProvider {
		return false
	}

	// Fallback for timeouts, connection errors, model not loaded, etc.
	return true
}

// selectProvider implements prompt-based routing.
// YAGNI: Simple keyword matching, no ML-based intent detection.
func (r *Router) selectProvider(ctx context.Context, prompt string) (Provider, bool) {
	promptLower := strings.ToLower(prompt)

	// ─────────────────────────────────────────────────────────────────────────
	// SMART LANE TRIGGERS (OCR, code, analysis tasks)
	// These require MiniCPM-V's superior text recognition
	// ─────────────────────────────────────────────────────────────────────────
	smartTriggers := []string{
		// OCR tasks
		"read", "text in", "what does it say", "transcribe", "ocr",
		"words", "characters", "letters",
		// Code analysis
		"code", "debug", "error", "exception", "stack trace", "bug", "fix",
		"syntax", "function", "variable", "class", "method",
		// Technical content
		"terminal", "console", "log", "output", "command", "shell", "bash",
		"stdout", "stderr",
		// Data extraction
		"chart", "graph", "table", "data", "values", "numbers", "extract",
		"axis", "legend", "bar", "line", "pie",
		// Analysis tasks
		"analyze", "explain", "understand", "parse", "why", "how does",
		"what's wrong", "what went wrong",
		// DevOps specific
		"kubernetes", "k8s", "pod", "container", "dashboard", "docker",
		"monitoring", "grafana", "prometheus", "metrics",
		// Screenshot analysis
		"screenshot", "screen", "window", "ui", "interface",
	}

	for _, trigger := range smartTriggers {
		if strings.Contains(promptLower, trigger) {
			// Check if smart provider is healthy
			if r.smartProvider != nil && r.isSmartHealthy(ctx) {
				return r.smartProvider, false
			}
			// Fallback to fast if smart unavailable
			log.Debug().
				Str("trigger", trigger).
				Msg("smart lane trigger detected but unavailable, using fast lane")
			return r.fastProvider, true
		}
	}

	// ─────────────────────────────────────────────────────────────────────────
	// FAST LANE (simple queries, classification)
	// Default to Moondream for speed
	// ─────────────────────────────────────────────────────────────────────────
	return r.fastProvider, false
}

// isSmartHealthy checks smart provider health with caching.
// Prevents hammering Ollama with health checks on every request.
func (r *Router) isSmartHealthy(ctx context.Context) bool {
	if r.smartProvider == nil {
		return false
	}

	// Use cached result if fresh
	r.healthCheckMu.Lock()
	if time.Since(r.lastHealthCheck) < r.healthCheckInterval {
		healthy := r.smartHealthy.Load()
		r.healthCheckMu.Unlock()
		return healthy
	}
	r.healthCheckMu.Unlock()

	// Perform fresh health check (with lock to prevent thundering herd)
	r.healthCheckMu.Lock()
	defer r.healthCheckMu.Unlock()

	// Double-check after acquiring lock (another goroutine may have checked)
	if time.Since(r.lastHealthCheck) < r.healthCheckInterval {
		return r.smartHealthy.Load()
	}

	healthy := r.smartProvider.IsHealthy()
	r.smartHealthy.Store(healthy)
	r.lastHealthCheck = time.Now()

	log.Debug().
		Bool("healthy", healthy).
		Str("provider", r.smartProvider.Name()).
		Msg("vision smart provider health check")

	return healthy
}

// Health returns health status of all vision providers.
func (r *Router) Health(ctx context.Context) map[string]HealthStatus {
	result := make(map[string]HealthStatus)

	// Check fast provider
	if r.fastProvider != nil {
		healthy := r.fastProvider.IsHealthy()
		result[r.fastProvider.Name()] = HealthStatus{
			Healthy:  healthy,
			Role:     "fast_lane",
			Model:    r.config.FastModel,
			Fallback: false,
		}
	}

	// Check smart provider
	if r.smartProvider != nil {
		healthy := r.isSmartHealthy(ctx)
		result[r.smartProvider.Name()] = HealthStatus{
			Healthy:  healthy,
			Role:     "smart_lane",
			Model:    r.config.SmartModel,
			Fallback: !healthy && r.config.EnableFallback,
		}
	}

	return result
}

// HealthStatus represents the health status of a vision provider.
type HealthStatus struct {
	Healthy  bool   `json:"healthy"`            // True if model is loaded and responding
	Role     string `json:"role"`               // "fast_lane" or "smart_lane"
	Model    string `json:"model"`              // Model name (e.g., "moondream", "minicpm-v")
	Fallback bool   `json:"fallback,omitempty"` // True if this provider will use fallback
}

// GetConfig returns the current router configuration.
func (r *Router) GetConfig() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the router configuration.
func (r *Router) SetConfig(config Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	config.Validate()
	r.config = config
	r.healthCheckInterval = config.HealthCheckInterval
}

// IsEnabled returns whether vision is enabled.
func (r *Router) IsEnabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.Enabled
}

// GetFastProvider returns the fast lane provider.
func (r *Router) GetFastProvider() Provider {
	return r.fastProvider
}

// GetSmartProvider returns the smart lane provider (may be nil).
func (r *Router) GetSmartProvider() Provider {
	return r.smartProvider
}

// Stats returns router statistics.
type RouterStats struct {
	FastProviderHealthy  bool   `json:"fast_provider_healthy"`
	SmartProviderHealthy bool   `json:"smart_provider_healthy"`
	SmartProviderModel   string `json:"smart_provider_model"`
	FastProviderModel    string `json:"fast_provider_model"`
	FallbackEnabled      bool   `json:"fallback_enabled"`
}

// Stats returns current router statistics.
func (r *Router) Stats() RouterStats {
	return RouterStats{
		FastProviderHealthy:  r.fastProvider != nil && r.fastProvider.IsHealthy(),
		SmartProviderHealthy: r.smartHealthy.Load(),
		SmartProviderModel:   r.config.SmartModel,
		FastProviderModel:    r.config.FastModel,
		FallbackEnabled:      r.config.EnableFallback,
	}
}
