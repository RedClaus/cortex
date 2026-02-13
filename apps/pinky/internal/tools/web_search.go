// Package tools provides the web search tool using Tavily API.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// WebSearchTool searches the web using Tavily API for real-time information.
type WebSearchTool struct {
	client  *http.Client
	apiKey  string
	baseURL string
}

// WebSearchConfig configures the web search tool.
type WebSearchConfig struct {
	APIKey  string
	Timeout time.Duration
}

// DefaultWebSearchConfig returns sensible defaults.
func DefaultWebSearchConfig() *WebSearchConfig {
	// Try to get API key from environment
	apiKey := os.Getenv("TAVILY_API_KEY")

	return &WebSearchConfig{
		APIKey:  apiKey,
		Timeout: 30 * time.Second,
	}
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(cfg *WebSearchConfig) *WebSearchTool {
	if cfg == nil {
		cfg = DefaultWebSearchConfig()
	}

	return &WebSearchTool{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		apiKey:  cfg.APIKey,
		baseURL: "https://api.tavily.com",
	}
}

func (t *WebSearchTool) Name() string           { return "web_search" }
func (t *WebSearchTool) Category() ToolCategory { return CategoryWeb }
func (t *WebSearchTool) RiskLevel() RiskLevel   { return RiskLow }

func (t *WebSearchTool) Description() string {
	return "Search the web for current, factual information. Use this whenever you need real-world knowledge you don't have, including information about places, people, businesses, events, or anything that requires up-to-date data."
}

// Spec returns the tool specification for LLM function calling.
func (t *WebSearchTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        t.Name(),
		Description: t.Description(),
		Category:    t.Category(),
		RiskLevel:   t.RiskLevel(),
		Parameters: &ParamSchema{
			Type: "object",
			Properties: map[string]*ParamProp{
				"query": {
					Type:        "string",
					Description: "The search query (e.g., 'current weather in San Francisco', 'Super Bowl 2025 score')",
				},
				"include_answer": {
					Type:        "boolean",
					Description: "Whether to include a direct answer summary (default: true)",
					Default:     true,
				},
				"max_results": {
					Type:        "integer",
					Description: "Maximum number of search results to return (default: 5)",
					Default:     5,
				},
			},
			Required: []string{"query"},
		},
	}
}

// Validate checks if the input is valid.
func (t *WebSearchTool) Validate(input *ToolInput) error {
	if input == nil {
		return errors.New("input is nil")
	}

	query, ok := input.Args["query"].(string)
	if !ok || query == "" {
		return errors.New("query is required")
	}

	if len(query) > 1000 {
		return errors.New("query too long (max 1000 characters)")
	}

	return nil
}

// tavilyRequest is the request format for Tavily Search API.
type tavilyRequest struct {
	APIKey         string `json:"api_key"`
	Query          string `json:"query"`
	SearchDepth    string `json:"search_depth,omitempty"`
	IncludeAnswer  bool   `json:"include_answer"`
	IncludeImages  bool   `json:"include_images"`
	MaxResults     int    `json:"max_results"`
}

// tavilyResponse is the response from Tavily Search API.
type tavilyResponse struct {
	Answer  string          `json:"answer"`
	Query   string          `json:"query"`
	Results []tavilyResult  `json:"results"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// Execute performs the web search.
func (t *WebSearchTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
	// Check for API key
	apiKey := t.apiKey
	if apiKey == "" {
		apiKey = os.Getenv("TAVILY_API_KEY")
	}
	if apiKey == "" {
		return &ToolOutput{
			Success: false,
			Error:   "TAVILY_API_KEY not configured. Set it in environment or config.",
		}, nil
	}

	query := input.Args["query"].(string)

	// Parse optional parameters
	includeAnswer := true
	if ia, ok := input.Args["include_answer"].(bool); ok {
		includeAnswer = ia
	}

	maxResults := 5
	if mr, ok := input.Args["max_results"].(float64); ok {
		maxResults = int(mr)
	} else if mr, ok := input.Args["max_results"].(int); ok {
		maxResults = mr
	}

	// Build request
	reqBody := tavilyRequest{
		APIKey:        apiKey,
		Query:         query,
		SearchDepth:   "basic",
		IncludeAnswer: includeAnswer,
		IncludeImages: false,
		MaxResults:    maxResults,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to build request: %v", err),
		}, nil
	}

	// Make request
	req, err := http.NewRequestWithContext(ctx, "POST", t.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := t.client.Do(req)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("search request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Tavily API error %d: %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse response
	var tavilyResp tavilyResponse
	if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
		return &ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to parse response: %v", err),
		}, nil
	}

	// Format output
	var output strings.Builder
	output.WriteString(fmt.Sprintf("Web Search Results for: %s\n\n", query))

	// Include direct answer if available
	if tavilyResp.Answer != "" {
		output.WriteString("**Answer:**\n")
		output.WriteString(tavilyResp.Answer)
		output.WriteString("\n\n")
	}

	// Include search results
	if len(tavilyResp.Results) > 0 {
		output.WriteString("**Sources:**\n")
		for i, result := range tavilyResp.Results {
			output.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, result.Title, result.URL))
			if result.Content != "" {
				// Truncate long content
				content := result.Content
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				output.WriteString(fmt.Sprintf("   %s\n", content))
			}
			output.WriteString("\n")
		}
	}

	return &ToolOutput{
		Success:  true,
		Output:   output.String(),
		Duration: duration,
	}, nil
}
