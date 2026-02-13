package introspection

import (
	"context"
	"fmt"

	"github.com/normanking/cortex/internal/tools"
)

// WebSearchAdapter adapts the WebSearchTool to the WebSearcher interface.
// This allows the AcquisitionEngine to use the existing Tavily-based
// web search implementation.
type WebSearchAdapter struct {
	tool *tools.WebSearchTool
}

// NewWebSearchAdapter creates a new adapter for the web search tool.
func NewWebSearchAdapter(tool *tools.WebSearchTool) *WebSearchAdapter {
	return &WebSearchAdapter{tool: tool}
}

// Search implements the WebSearcher interface.
func (a *WebSearchAdapter) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if a.tool == nil {
		return nil, fmt.Errorf("web search tool not configured")
	}

	// Use SearchRaw to get direct access to results
	results, err := a.tool.SearchRaw(ctx, query, maxResults)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	// Convert TavilyResult to WebSearchResult
	wsResults := make([]WebSearchResult, len(results))
	for i, r := range results {
		wsResults[i] = WebSearchResult{
			URL:     r.URL,
			Title:   r.Title,
			Content: r.Content,
			Score:   r.Score,
		}
	}

	return wsResults, nil
}
