package llm

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// COST RATES (per million tokens)
// ═══════════════════════════════════════════════════════════════════════════════

// ProviderCostRates defines cost per million tokens for each provider.
// Input and output costs differ for most cloud providers.
type ProviderCostRates struct {
	InputPerMillion  float64 // Cost per million input tokens
	OutputPerMillion float64 // Cost per million output tokens
}

// CostRates maps provider names to their token costs (USD per million tokens).
// Updated December 2025. Local providers are free.
var CostRates = map[string]ProviderCostRates{
	// Local providers - FREE
	"ollama": {0.0, 0.0},
	"mlx":    {0.0, 0.0},
	"dnet":   {0.0, 0.0},
	"local":  {0.0, 0.0},

	// Cloud providers - priced per million tokens
	"openai":     {2.50, 10.00},  // GPT-4o
	"anthropic":  {3.00, 15.00},  // Claude 3.5 Sonnet
	"claude_max": {0.0, 0.0},     // Subscription - effectively free per call
	"gemini":     {0.075, 0.30},  // Gemini 1.5 Flash
	"groq":       {0.05, 0.08},   // Llama 3.1 70B on Groq
	"grok":       {2.00, 10.00},  // Grok-2
	"mistral":    {0.25, 0.25},   // Mistral Small
	"openrouter": {1.00, 2.00},   // Varies by model, using average
}

// GetCostRate returns the cost rate for a provider.
func GetCostRate(provider string) ProviderCostRates {
	if rate, ok := CostRates[provider]; ok {
		return rate
	}
	// Unknown provider - assume moderate cloud pricing
	return ProviderCostRates{1.0, 2.0}
}

// IsLocalProvider returns true if the provider is local (free).
func IsLocalProvider(provider string) bool {
	switch provider {
	case "ollama", "mlx", "dnet", "local":
		return true
	default:
		return false
	}
}

// MetricsProvider wraps an LLM provider with timing and metrics collection.
type MetricsProvider struct {
	provider Provider
	name     string
	log      *logging.Logger

	// Atomic counters
	totalCalls       int64
	totalErrors      int64
	totalTokens      int64
	totalInputTokens int64 // Prompt tokens
	totalOutputTokens int64 // Completion tokens

	// Protected by mutex
	mu              sync.RWMutex
	totalLatency    time.Duration
	minLatency      time.Duration
	maxLatency      time.Duration
	latencyBuckets  []int64 // Histogram: <100ms, <500ms, <1s, <2s, <5s, 5s+
	modelStats      map[string]*ModelMetrics
	estimatedCostUSD float64 // Running cost estimate
}

// ModelMetrics tracks per-model performance.
type ModelMetrics struct {
	Calls         int64
	TotalLatency  time.Duration
	Errors        int64
	InputTokens   int64
	OutputTokens  int64
	EstimatedCost float64
}

// NewMetricsProvider wraps a provider with metrics collection.
func NewMetricsProvider(provider Provider) *MetricsProvider {
	return &MetricsProvider{
		provider:       provider,
		name:           provider.Name(),
		log:            logging.Global(),
		minLatency:     time.Hour, // Will be replaced on first call
		latencyBuckets: make([]int64, 6),
		modelStats:     make(map[string]*ModelMetrics),
	}
}

// Chat implements Provider interface with metrics.
func (m *MetricsProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Log start
	m.log.Debug("[LLM-Metrics] Starting %s/%s call", m.name, req.Model)

	resp, err := m.provider.Chat(ctx, req)

	latency := time.Since(start)

	// Update atomic counters
	atomic.AddInt64(&m.totalCalls, 1)
	if err != nil {
		atomic.AddInt64(&m.totalErrors, 1)
	}

	// Update protected stats
	m.mu.Lock()
	m.totalLatency += latency

	if latency < m.minLatency {
		m.minLatency = latency
	}
	if latency > m.maxLatency {
		m.maxLatency = latency
	}

	// Update histogram bucket
	switch {
	case latency < 100*time.Millisecond:
		m.latencyBuckets[0]++
	case latency < 500*time.Millisecond:
		m.latencyBuckets[1]++
	case latency < 1*time.Second:
		m.latencyBuckets[2]++
	case latency < 2*time.Second:
		m.latencyBuckets[3]++
	case latency < 5*time.Second:
		m.latencyBuckets[4]++
	default:
		m.latencyBuckets[5]++
	}

	// Per-model stats
	if _, ok := m.modelStats[req.Model]; !ok {
		m.modelStats[req.Model] = &ModelMetrics{}
	}
	m.modelStats[req.Model].Calls++
	m.modelStats[req.Model].TotalLatency += latency
	if err != nil {
		m.modelStats[req.Model].Errors++
	}
	m.mu.Unlock()

	// Update tokens and cost if available
	if resp != nil && resp.TokensUsed > 0 {
		atomic.AddInt64(&m.totalTokens, int64(resp.TokensUsed))
		atomic.AddInt64(&m.totalInputTokens, int64(resp.PromptTokens))
		atomic.AddInt64(&m.totalOutputTokens, int64(resp.CompletionTokens))

		// Calculate cost
		rates := GetCostRate(m.name)
		inputCost := float64(resp.PromptTokens) / 1_000_000.0 * rates.InputPerMillion
		outputCost := float64(resp.CompletionTokens) / 1_000_000.0 * rates.OutputPerMillion
		callCost := inputCost + outputCost

		m.mu.Lock()
		m.estimatedCostUSD += callCost
		if stats, ok := m.modelStats[req.Model]; ok {
			stats.InputTokens += int64(resp.PromptTokens)
			stats.OutputTokens += int64(resp.CompletionTokens)
			stats.EstimatedCost += callCost
		}
		m.mu.Unlock()
	}

	// Log completion
	if err != nil {
		m.log.Warn("[LLM-Metrics] %s/%s FAILED after %v: %v", m.name, req.Model, latency, err)
	} else {
		tokens := 0
		cost := 0.0
		if resp != nil {
			tokens = resp.TokensUsed
			rates := GetCostRate(m.name)
			cost = float64(resp.PromptTokens)/1_000_000.0*rates.InputPerMillion +
				float64(resp.CompletionTokens)/1_000_000.0*rates.OutputPerMillion
		}
		if cost > 0 {
			m.log.Info("[LLM-Metrics] %s/%s completed in %v (%d tokens, $%.6f)", m.name, req.Model, latency, tokens, cost)
		} else {
			m.log.Info("[LLM-Metrics] %s/%s completed in %v (%d tokens, free)", m.name, req.Model, latency, tokens)
		}
	}

	return resp, err
}

// Name implements Provider interface.
func (m *MetricsProvider) Name() string {
	return m.name
}

// Available implements Provider interface.
func (m *MetricsProvider) Available() bool {
	return m.provider.Available()
}

// GetMetrics returns current metrics.
func (m *MetricsProvider) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls := atomic.LoadInt64(&m.totalCalls)
	errors := atomic.LoadInt64(&m.totalErrors)
	inputTokens := atomic.LoadInt64(&m.totalInputTokens)
	outputTokens := atomic.LoadInt64(&m.totalOutputTokens)

	avgLatency := time.Duration(0)
	if calls > 0 {
		avgLatency = m.totalLatency / time.Duration(calls)
	}

	errorRate := float64(0)
	if calls > 0 {
		errorRate = float64(errors) / float64(calls)
	}

	// Build model breakdown
	modelBreakdown := make(map[string]interface{})
	for model, stats := range m.modelStats {
		avgModelLatency := time.Duration(0)
		if stats.Calls > 0 {
			avgModelLatency = stats.TotalLatency / time.Duration(stats.Calls)
		}
		modelBreakdown[model] = map[string]interface{}{
			"calls":          stats.Calls,
			"errors":         stats.Errors,
			"avg_latency_ms": avgModelLatency.Milliseconds(),
			"input_tokens":   stats.InputTokens,
			"output_tokens":  stats.OutputTokens,
			"cost_usd":       stats.EstimatedCost,
		}
	}

	return map[string]interface{}{
		"provider":         m.name,
		"is_local":         IsLocalProvider(m.name),
		"total_calls":      calls,
		"total_errors":     errors,
		"error_rate":       errorRate,
		"total_tokens":     atomic.LoadInt64(&m.totalTokens),
		"input_tokens":     inputTokens,
		"output_tokens":    outputTokens,
		"estimated_cost":   m.estimatedCostUSD,
		"avg_latency_ms":   avgLatency.Milliseconds(),
		"min_latency_ms":   m.minLatency.Milliseconds(),
		"max_latency_ms":   m.maxLatency.Milliseconds(),
		"latency_histogram": map[string]int64{
			"<100ms": m.latencyBuckets[0],
			"<500ms": m.latencyBuckets[1],
			"<1s":    m.latencyBuckets[2],
			"<2s":    m.latencyBuckets[3],
			"<5s":    m.latencyBuckets[4],
			"5s+":    m.latencyBuckets[5],
		},
		"models": modelBreakdown,
	}
}

// GetCostSummary returns a human-readable cost summary.
func (m *MetricsProvider) GetCostSummary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	calls := atomic.LoadInt64(&m.totalCalls)
	tokens := atomic.LoadInt64(&m.totalTokens)

	if calls == 0 {
		return fmt.Sprintf("%s: no calls", m.name)
	}

	if IsLocalProvider(m.name) {
		return fmt.Sprintf("%s: %d calls, %d tokens (free)", m.name, calls, tokens)
	}

	return fmt.Sprintf("%s: %d calls, %d tokens, $%.4f", m.name, calls, tokens, m.estimatedCostUSD)
}

// Reset clears all metrics.
func (m *MetricsProvider) Reset() {
	atomic.StoreInt64(&m.totalCalls, 0)
	atomic.StoreInt64(&m.totalErrors, 0)
	atomic.StoreInt64(&m.totalTokens, 0)
	atomic.StoreInt64(&m.totalInputTokens, 0)
	atomic.StoreInt64(&m.totalOutputTokens, 0)

	m.mu.Lock()
	m.totalLatency = 0
	m.minLatency = time.Hour
	m.maxLatency = 0
	m.latencyBuckets = make([]int64, 6)
	m.modelStats = make(map[string]*ModelMetrics)
	m.estimatedCostUSD = 0
	m.mu.Unlock()
}

// Unwrap returns the underlying provider.
func (m *MetricsProvider) Unwrap() Provider {
	return m.provider
}
