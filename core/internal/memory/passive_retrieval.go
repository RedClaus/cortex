package memory

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/pkg/types"
)

// PassiveRetriever performs automatic knowledge lookup for Fast Lane.
// This is the key mechanism that prevents "split-brain" - letting Fast Lane
// access knowledge stored by Smart Lane without explicit tool calls.
//
// How it works:
// 1. User query arrives, routed to Fast Lane
// 2. Backend performs vector/keyword search (not LLM)
// 3. Results with similarity >= threshold are injected into context
// 4. Fast Lane LLM responds with relevant knowledge
type PassiveRetriever struct {
	fabric  knowledge.KnowledgeFabric
	config  PassiveRetrievalConfig
	metrics *PassiveMetrics
}

// PassiveRetrievalConfig configures passive retrieval behavior.
type PassiveRetrievalConfig struct {
	Enabled       bool    `json:"enabled"`
	MaxResults    int     `json:"max_results"`     // Default: 3
	MinTrustScore float64 `json:"min_trust_score"` // Default: 0.5 (trust-based filtering)
	MaxTokensToAdd int    `json:"max_tokens_to_add"` // Default: 300
	TimeoutMs     int     `json:"timeout_ms"`      // Default: 50ms - must be FAST
}

// DefaultPassiveRetrievalConfig returns sensible defaults.
// These values are tuned for minimal latency impact on Fast Lane.
func DefaultPassiveRetrievalConfig() PassiveRetrievalConfig {
	return PassiveRetrievalConfig{
		Enabled:        true,
		MaxResults:     3,
		MinTrustScore:  0.5, // Only return trusted knowledge
		MaxTokensToAdd: 300, // ~75 tokens per result max
		TimeoutMs:      50,  // Must not impact TTFT
	}
}

// PassiveMetrics tracks passive retrieval performance.
type PassiveMetrics struct {
	TotalSearches   int64   // Total passive searches performed
	TotalHits       int64   // Searches that found relevant results
	TotalMisses     int64   // Searches with no relevant results
	TotalTimeouts   int64   // Searches that timed out
	AvgLatencyMs    float64 // Rolling average latency
	latencySum      int64
}

// NewPassiveRetriever creates a new passive retriever.
func NewPassiveRetriever(fabric knowledge.KnowledgeFabric, config PassiveRetrievalConfig) *PassiveRetriever {
	return &PassiveRetriever{
		fabric:  fabric,
		config:  config,
		metrics: &PassiveMetrics{},
	}
}

// Retrieve searches for relevant knowledge based on user message.
// This is called BEFORE the Fast Lane LLM, not by the LLM.
//
// Important: This method is designed to fail silently - passive retrieval
// is an enhancement, not a requirement. If it fails or times out,
// the Fast Lane response proceeds without injected knowledge.
func (pr *PassiveRetriever) Retrieve(
	ctx context.Context,
	userMessage string,
	projectID string,
) ([]PassiveResult, error) {
	if !pr.config.Enabled {
		return nil, nil
	}

	// Skip very short queries - not enough signal for meaningful search
	if len(strings.TrimSpace(userMessage)) < 5 {
		return nil, nil
	}

	start := time.Now()
	atomic.AddInt64(&pr.metrics.TotalSearches, 1)

	// Strict timeout - must be fast to not impact TTFT
	ctx, cancel := context.WithTimeout(ctx, time.Duration(pr.config.TimeoutMs)*time.Millisecond)
	defer cancel()

	// Perform search using existing knowledge fabric
	results, err := pr.fabric.Search(ctx, userMessage, types.SearchOptions{
		Limit:    pr.config.MaxResults,
		MinTrust: pr.config.MinTrustScore,
		// Note: We don't filter by project scope here - personal knowledge
		// takes priority via the fabric's built-in prioritization
	})

	latencyMs := time.Since(start).Milliseconds()
	pr.updateLatency(latencyMs)

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		atomic.AddInt64(&pr.metrics.TotalTimeouts, 1)
		log.Debug().
			Int64("latency_ms", latencyMs).
			Msg("passive retrieval timed out")
		return nil, nil // Fail silently
	}

	// Handle other errors
	if err != nil {
		log.Debug().Err(err).Msg("passive retrieval failed")
		return nil, nil // Fail silently
	}

	// No results
	if results == nil || len(results.Items) == 0 {
		atomic.AddInt64(&pr.metrics.TotalMisses, 1)
		log.Debug().
			Str("query", truncateQuery(userMessage)).
			Int64("latency_ms", latencyMs).
			Msg("passive retrieval found no matches")
		return nil, nil
	}

	atomic.AddInt64(&pr.metrics.TotalHits, 1)

	// Convert to PassiveResults with token budget enforcement
	passiveResults := pr.convertResults(results.Items)

	log.Debug().
		Str("query", truncateQuery(userMessage)).
		Int("found", len(passiveResults)).
		Str("tier", results.Tier.String()).
		Int64("latency_ms", latencyMs).
		Msg("passive retrieval completed")

	return passiveResults, nil
}

// convertResults converts KnowledgeItems to PassiveResults.
// Enforces token budget and formats for injection.
func (pr *PassiveRetriever) convertResults(items []*types.KnowledgeItem) []PassiveResult {
	var results []PassiveResult
	totalTokens := 0

	for _, item := range items {
		if item == nil {
			continue
		}
		// Format the result as a concise summary
		summary := pr.formatResult(*item)
		tokens := estimateTokens(summary)

		// Check token budget
		if totalTokens+tokens > pr.config.MaxTokensToAdd {
			break
		}

		results = append(results, PassiveResult{
			ID:         item.ID,
			Summary:    summary,
			Confidence: item.TrustScore,
		})

		totalTokens += tokens
	}

	return results
}

// formatResult creates a concise summary for injection.
// The format depends on the content type (inferred from structure).
func (pr *PassiveRetriever) formatResult(item types.KnowledgeItem) string {
	// Check if this looks like a command/solution
	if strings.Contains(item.Content, "```") {
		// Extract first code block if present
		parts := strings.SplitN(item.Content, "```", 3)
		if len(parts) >= 2 {
			code := strings.TrimSpace(parts[1])
			// Remove language specifier if present
			if idx := strings.Index(code, "\n"); idx > 0 && idx < 20 {
				code = code[idx+1:]
			}
			code = strings.TrimSpace(code)
			if len(code) > 200 {
				code = code[:200] + "..."
			}
			return fmt.Sprintf("%s: %s", item.Title, code)
		}
	}

	// Check if this looks like an error solution (has "fix" or "solution" in title)
	titleLower := strings.ToLower(item.Title)
	if strings.Contains(titleLower, "fix") || strings.Contains(titleLower, "solution") {
		// This is likely an error solution
		content := strings.TrimSpace(item.Content)
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		return fmt.Sprintf("Known fix for %s: %s", item.Title, content)
	}

	// Default format: Title + truncated content
	content := strings.TrimSpace(item.Content)
	if len(content) > 150 {
		content = content[:150] + "..."
	}

	return fmt.Sprintf("%s: %s", item.Title, content)
}

// InjectIntoContext adds passive results to the system prompt.
// Replaces the {{PASSIVE_RETRIEVAL}} placeholder.
func (pr *PassiveRetriever) InjectIntoContext(
	systemPrompt string,
	results []PassiveResult,
) string {
	if len(results) == 0 {
		// Remove placeholder entirely
		return strings.Replace(systemPrompt, "{{PASSIVE_RETRIEVAL}}\n", "", 1)
	}

	var sb strings.Builder
	sb.WriteString("<relevant_knowledge>\n")
	sb.WriteString("Previously learned information that may be relevant:\n")
	for _, r := range results {
		sb.WriteString(fmt.Sprintf("• %s\n", r.Summary))
	}
	sb.WriteString("</relevant_knowledge>\n")

	return strings.Replace(systemPrompt, "{{PASSIVE_RETRIEVAL}}\n", sb.String(), 1)
}

// updateLatency updates the rolling average latency.
func (pr *PassiveRetriever) updateLatency(latencyMs int64) {
	atomic.AddInt64(&pr.metrics.latencySum, latencyMs)
	total := atomic.LoadInt64(&pr.metrics.TotalSearches)
	if total > 0 {
		pr.metrics.AvgLatencyMs = float64(atomic.LoadInt64(&pr.metrics.latencySum)) / float64(total)
	}
}

// Metrics returns current passive retrieval metrics.
func (pr *PassiveRetriever) Metrics() PassiveMetrics {
	return PassiveMetrics{
		TotalSearches: atomic.LoadInt64(&pr.metrics.TotalSearches),
		TotalHits:     atomic.LoadInt64(&pr.metrics.TotalHits),
		TotalMisses:   atomic.LoadInt64(&pr.metrics.TotalMisses),
		TotalTimeouts: atomic.LoadInt64(&pr.metrics.TotalTimeouts),
		AvgLatencyMs:  pr.metrics.AvgLatencyMs,
	}
}

// HitRate returns the percentage of searches that found results.
func (pr *PassiveRetriever) HitRate() float64 {
	total := atomic.LoadInt64(&pr.metrics.TotalSearches)
	if total == 0 {
		return 0
	}
	hits := atomic.LoadInt64(&pr.metrics.TotalHits)
	return float64(hits) / float64(total) * 100
}

// truncateQuery truncates a query for logging.
func truncateQuery(query string) string {
	if len(query) > 50 {
		return query[:50] + "..."
	}
	return query
}

// RetrievalSummary provides a summary of what was retrieved for debugging.
type RetrievalSummary struct {
	Query           string          `json:"query"`
	ResultCount     int             `json:"result_count"`
	TopResultTitle  string          `json:"top_result_title,omitempty"`
	TopConfidence   float64         `json:"top_confidence,omitempty"`
	LatencyMs       int64           `json:"latency_ms"`
	WasTimeout      bool            `json:"was_timeout"`
}

// RetrieveWithSummary is like Retrieve but returns a summary for debugging.
func (pr *PassiveRetriever) RetrieveWithSummary(
	ctx context.Context,
	userMessage string,
	projectID string,
) ([]PassiveResult, *RetrievalSummary) {
	start := time.Now()

	results, _ := pr.Retrieve(ctx, userMessage, projectID)

	summary := &RetrievalSummary{
		Query:       truncateQuery(userMessage),
		ResultCount: len(results),
		LatencyMs:   time.Since(start).Milliseconds(),
		WasTimeout:  ctx.Err() == context.DeadlineExceeded,
	}

	if len(results) > 0 {
		summary.TopResultTitle = results[0].Summary[:min(50, len(results[0].Summary))]
		summary.TopConfidence = results[0].Confidence
	}

	return results, summary
}
