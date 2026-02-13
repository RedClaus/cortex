package llm

import (
	"fmt"
	"strings"
	"sync"
)

// MetricsRegistry tracks all MetricsProvider instances for aggregated reporting.
type MetricsRegistry struct {
	mu        sync.RWMutex
	providers map[string]*MetricsProvider
}

// globalRegistry is the singleton metrics registry.
var globalRegistry = &MetricsRegistry{
	providers: make(map[string]*MetricsProvider),
}

// Register adds a MetricsProvider to the global registry.
func (r *MetricsRegistry) Register(provider *MetricsProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Get retrieves a specific provider's MetricsProvider.
func (r *MetricsRegistry) Get(name string) *MetricsProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.providers[name]
}

// GetAll returns all registered MetricsProviders.
func (r *MetricsRegistry) GetAll() map[string]*MetricsProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*MetricsProvider, len(r.providers))
	for k, v := range r.providers {
		result[k] = v
	}
	return result
}

// GetAllMetrics returns aggregated metrics from all providers.
func (r *MetricsRegistry) GetAllMetrics() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]interface{}, len(r.providers))
	for name, provider := range r.providers {
		result[name] = provider.GetMetrics()
	}
	return result
}

// GetSummary returns high-level summary across all providers.
func (r *MetricsRegistry) GetSummary() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var (
		totalCalls   int64
		totalErrors  int64
		totalTokens  int64
		localCalls   int64
		cloudCalls   int64
	)

	localProviders := map[string]bool{"ollama": true}

	for name, provider := range r.providers {
		metrics := provider.GetMetrics()

		if calls, ok := metrics["total_calls"].(int64); ok {
			totalCalls += calls
			if localProviders[name] {
				localCalls += calls
			} else {
				cloudCalls += calls
			}
		}
		if errors, ok := metrics["total_errors"].(int64); ok {
			totalErrors += errors
		}
		if tokens, ok := metrics["total_tokens"].(int64); ok {
			totalTokens += tokens
		}
	}

	localRate := float64(0)
	if totalCalls > 0 {
		localRate = float64(localCalls) / float64(totalCalls)
	}

	return map[string]interface{}{
		"total_calls":   totalCalls,
		"total_errors":  totalErrors,
		"total_tokens":  totalTokens,
		"local_calls":   localCalls,
		"cloud_calls":   cloudCalls,
		"local_rate":    localRate,
		"provider_count": len(r.providers),
	}
}

// Reset clears all metrics across all providers.
func (r *MetricsRegistry) Reset() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		provider.Reset()
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// PACKAGE-LEVEL FUNCTIONS (Access global registry)
// ═══════════════════════════════════════════════════════════════════════════════

// RegisterMetricsProvider adds a provider to the global registry.
func RegisterMetricsProvider(provider *MetricsProvider) {
	globalRegistry.Register(provider)
}

// GetMetricsProvider retrieves a specific provider from the global registry.
func GetMetricsProvider(name string) *MetricsProvider {
	return globalRegistry.Get(name)
}

// GetAllMetrics returns metrics from all registered providers.
func GetAllMetrics() map[string]interface{} {
	return globalRegistry.GetAllMetrics()
}

// GetMetricsSummary returns high-level summary across all providers.
func GetMetricsSummary() map[string]interface{} {
	return globalRegistry.GetSummary()
}

// ResetAllMetrics clears metrics across all providers.
func ResetAllMetrics() {
	globalRegistry.Reset()
}

// GlobalRegistry returns the global metrics registry instance.
func GlobalRegistry() *MetricsRegistry {
	return globalRegistry
}

// ═══════════════════════════════════════════════════════════════════════════════
// COST TRACKING
// ═══════════════════════════════════════════════════════════════════════════════

// CostSummary holds aggregated cost information.
type CostSummary struct {
	TotalCalls       int64
	TotalTokens      int64
	InputTokens      int64
	OutputTokens     int64
	LocalCalls       int64
	CloudCalls       int64
	EstimatedCostUSD float64
	ByProvider       map[string]ProviderCostSummary
}

// ProviderCostSummary holds per-provider cost summary.
type ProviderCostSummary struct {
	Calls         int64
	Tokens        int64
	InputTokens   int64
	OutputTokens  int64
	CostUSD       float64
	IsLocal       bool
	AvgLatencyMs  int64
}

// GetCostSummary returns an aggregated cost summary across all providers.
func (r *MetricsRegistry) GetCostSummary() *CostSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summary := &CostSummary{
		ByProvider: make(map[string]ProviderCostSummary),
	}

	for name, provider := range r.providers {
		metrics := provider.GetMetrics()

		calls, _ := metrics["total_calls"].(int64)
		tokens, _ := metrics["total_tokens"].(int64)
		inputTokens, _ := metrics["input_tokens"].(int64)
		outputTokens, _ := metrics["output_tokens"].(int64)
		cost, _ := metrics["estimated_cost"].(float64)
		isLocal, _ := metrics["is_local"].(bool)
		avgLatency, _ := metrics["avg_latency_ms"].(int64)

		summary.TotalCalls += calls
		summary.TotalTokens += tokens
		summary.InputTokens += inputTokens
		summary.OutputTokens += outputTokens
		summary.EstimatedCostUSD += cost

		if isLocal {
			summary.LocalCalls += calls
		} else {
			summary.CloudCalls += calls
		}

		summary.ByProvider[name] = ProviderCostSummary{
			Calls:        calls,
			Tokens:       tokens,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			CostUSD:      cost,
			IsLocal:      isLocal,
			AvgLatencyMs: avgLatency,
		}
	}

	return summary
}

// FormatCostSummary returns a human-readable cost summary.
func (r *MetricsRegistry) FormatCostSummary() string {
	summary := r.GetCostSummary()

	if summary.TotalCalls == 0 {
		return "No LLM calls recorded this session."
	}

	var sb strings.Builder
	sb.WriteString("═══════════════════════════════════════════════════════\n")
	sb.WriteString("                   LLM USAGE SUMMARY                   \n")
	sb.WriteString("═══════════════════════════════════════════════════════\n\n")

	// Totals
	sb.WriteString(fmt.Sprintf("Total Calls:    %d (%d local, %d cloud)\n",
		summary.TotalCalls, summary.LocalCalls, summary.CloudCalls))
	sb.WriteString(fmt.Sprintf("Total Tokens:   %d (in: %d, out: %d)\n",
		summary.TotalTokens, summary.InputTokens, summary.OutputTokens))

	if summary.EstimatedCostUSD > 0 {
		sb.WriteString(fmt.Sprintf("Estimated Cost: $%.4f\n", summary.EstimatedCostUSD))
	} else {
		sb.WriteString("Estimated Cost: $0.00 (all local inference)\n")
	}

	sb.WriteString("\n───────────────────────────────────────────────────────\n")
	sb.WriteString("By Provider:\n\n")

	// Per-provider breakdown
	for name, ps := range summary.ByProvider {
		if ps.Calls == 0 {
			continue
		}
		locality := "local"
		if !ps.IsLocal {
			locality = "cloud"
		}

		if ps.CostUSD > 0 {
			sb.WriteString(fmt.Sprintf("  %-12s %d calls, %d tokens, $%.4f (%s)\n",
				name+":", ps.Calls, ps.Tokens, ps.CostUSD, locality))
		} else {
			sb.WriteString(fmt.Sprintf("  %-12s %d calls, %d tokens, free (%s)\n",
				name+":", ps.Calls, ps.Tokens, locality))
		}
	}

	sb.WriteString("\n═══════════════════════════════════════════════════════\n")

	return sb.String()
}

// GetCostSummaryFormatted is a package-level function to get formatted cost summary.
func GetCostSummaryFormatted() string {
	return globalRegistry.FormatCostSummary()
}
