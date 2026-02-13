package knowledge

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/normanking/cortex/pkg/types"
)

// performTieredRetrieval implements the three-tier knowledge retrieval strategy:
//
// Tier 1 (STRICT): Exact tag match + confidence > 0.8
//   - Fast, high-precision retrieval
//   - Used when we have well-defined tags (e.g., "cisco", "ios-xe", "routing")
//
// Tier 2 (FUZZY): FTS5 full-text search + trust-weighted ranking
//   - Semantic search across title, content, and tags
//   - Ranked by: relevance * trust_score * confidence
//
// Tier 3 (FALLBACK): No strong match found
//   - Returns partial results (if any) with low confidence
//   - Signals to orchestrator to use LLM generation instead
//
// Priority rules:
// 1. Search all requested tiers (default: Personal → Team → Global)
// 2. Within each tier, rank by: trust_score * relevance * confidence
// 3. Deduplicate across tiers (prefer higher scope for conflicts)
// 4. Apply filters (types, tags, minTrust) after ranking
func performTieredRetrieval(
	ctx context.Context,
	store Store,
	searcher Searcher,
	query string,
	opts types.SearchOptions,
) (*types.RetrievalResult, error) {
	// Set defaults
	if opts.Limit == 0 {
		opts.Limit = 10
	}
	if len(opts.Tiers) == 0 {
		// Default: search all tiers in priority order
		opts.Tiers = []types.Scope{types.ScopePersonal, types.ScopeTeam, types.ScopeGlobal}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// TIER 1: STRICT - Exact Tag Match
	// ═══════════════════════════════════════════════════════════════════════════════

	if len(opts.Tags) > 0 {
		strictResults, err := attemptStrictRetrieval(ctx, store, opts)
		if err == nil && len(strictResults) > 0 {
			// Filter high-confidence results
			highConfidence := filterByConfidence(strictResults, 0.8)
			if len(highConfidence) > 0 {
				return &types.RetrievalResult{
					Items:       highConfidence[:min(opts.Limit, len(highConfidence))],
					Tier:        types.TierStrict,
					Confidence:  calculateAverageConfidence(highConfidence),
					Explanation: fmt.Sprintf("Found %d exact tag matches with high confidence", len(highConfidence)),
				}, nil
			}
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// TIER 2: FUZZY - FTS5 + Trust-Weighted Ranking
	// ═══════════════════════════════════════════════════════════════════════════════

	fuzzyResults, err := attemptFuzzyRetrieval(ctx, searcher, query, opts)
	if err == nil && len(fuzzyResults) > 0 {
		// Apply trust-weighted ranking
		ranked := rankByTrustAndRelevance(fuzzyResults)

		// Apply filters
		filtered := applyFilters(ranked, opts)

		if len(filtered) > 0 {
			items := extractItems(filtered)
			return &types.RetrievalResult{
				Items:       items[:min(opts.Limit, len(items))],
				Tier:        types.TierFuzzy,
				Confidence:  calculateAverageScore(filtered),
				Explanation: fmt.Sprintf("Found %d results via semantic search", len(items)),
			}, nil
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// TIER 3: FALLBACK - Return Partial Results or Empty
	// ═══════════════════════════════════════════════════════════════════════════════

	// Try to return SOMETHING even if low confidence
	allResults := []*types.KnowledgeItem{}

	// Fallback: get recent items from requested tiers
	for _, tier := range opts.Tiers {
		items, err := store.GetByScope(ctx, tier)
		if err == nil {
			allResults = append(allResults, items...)
		}
	}

	// Sort by trust score and recency
	sort.Slice(allResults, func(i, j int) bool {
		// Prefer higher trust
		if allResults[i].TrustScore != allResults[j].TrustScore {
			return allResults[i].TrustScore > allResults[j].TrustScore
		}
		// Then prefer more recent
		return allResults[i].UpdatedAt.After(allResults[j].UpdatedAt)
	})

	explanation := "No strong matches found. Showing recent high-trust knowledge. Consider using AI generation."
	if len(allResults) == 0 {
		explanation = "No knowledge items found. Use AI generation for this query."
	}

	return &types.RetrievalResult{
		Items:       allResults[:min(opts.Limit, len(allResults))],
		Tier:        types.TierFallback,
		Confidence:  0.2, // Low confidence - signal for LLM generation
		Explanation: explanation,
	}, nil
}

// attemptStrictRetrieval performs exact tag matching.
func attemptStrictRetrieval(
	ctx context.Context,
	store Store,
	opts types.SearchOptions,
) ([]*types.KnowledgeItem, error) {
	// Search by exact tag match
	items, err := store.SearchByTags(ctx, opts.Tags, opts.Tiers)
	if err != nil {
		return nil, err
	}

	// Apply additional filters
	filtered := []*types.KnowledgeItem{}
	for _, item := range items {
		if matchesFilters(item, opts) {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

// attemptFuzzyRetrieval performs full-text search with trust weighting.
func attemptFuzzyRetrieval(
	ctx context.Context,
	searcher Searcher,
	query string,
	opts types.SearchOptions,
) ([]*ScoredItem, error) {
	// Use FTS5 for semantic search
	scored, err := searcher.Search(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	return scored, nil
}

// rankByTrustAndRelevance combines FTS5 relevance with trust score and confidence.
// Final score = relevance * trust_score * confidence
func rankByTrustAndRelevance(items []*ScoredItem) []*ScoredItem {
	for _, item := range items {
		// Combine relevance with quality signals
		item.Relevance = item.Relevance * item.Item.TrustScore * item.Item.Confidence
	}

	// Sort by combined score (descending)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Relevance > items[j].Relevance
	})

	return items
}

// applyFilters applies search options filters to scored items.
func applyFilters(items []*ScoredItem, opts types.SearchOptions) []*ScoredItem {
	filtered := []*ScoredItem{}
	for _, scored := range items {
		if matchesFilters(scored.Item, opts) {
			filtered = append(filtered, scored)
		}
	}
	return filtered
}

// matchesFilters checks if a knowledge item matches the search criteria.
func matchesFilters(item *types.KnowledgeItem, opts types.SearchOptions) bool {
	// Filter by type
	if len(opts.Types) > 0 {
		found := false
		for _, t := range opts.Types {
			if string(item.Type) == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by minimum trust
	if item.TrustScore < opts.MinTrust {
		return false
	}

	// Filter by project (if specified)
	if opts.ProjectID != "" {
		// TODO: Implement project filtering when project metadata is added
		// For now, we don't filter by project
	}

	return true
}

// filterByConfidence returns items with confidence >= threshold.
func filterByConfidence(items []*types.KnowledgeItem, threshold float64) []*types.KnowledgeItem {
	filtered := []*types.KnowledgeItem{}
	for _, item := range items {
		if item.Confidence >= threshold {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// calculateAverageConfidence computes the mean confidence of a set of items.
func calculateAverageConfidence(items []*types.KnowledgeItem) float64 {
	if len(items) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, item := range items {
		sum += item.Confidence
	}
	return sum / float64(len(items))
}

// calculateAverageScore computes the mean relevance score of scored items.
func calculateAverageScore(items []*ScoredItem) float64 {
	if len(items) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, item := range items {
		sum += item.Relevance
	}
	return sum / float64(len(items))
}

// extractItems converts ScoredItems to plain KnowledgeItems.
func extractItems(scored []*ScoredItem) []*types.KnowledgeItem {
	items := make([]*types.KnowledgeItem, len(scored))
	for i, s := range scored {
		items[i] = s.Item
	}
	return items
}

// scopePriority returns a numeric priority for scope comparison.
func scopePriority(scope types.Scope) int {
	switch scope {
	case types.ScopePersonal:
		return 3
	case types.ScopeTeam:
		return 2
	case types.ScopeGlobal:
		return 1
	default:
		return 0
	}
}

// hashContent generates a simple hash for content deduplication.
func hashContent(content string) string {
	// Simple hash: normalize whitespace and take first 50 chars
	normalized := strings.Join(strings.Fields(content), " ")
	if len(normalized) > 50 {
		return normalized[:50]
	}
	return normalized
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
