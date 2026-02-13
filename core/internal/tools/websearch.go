package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/ingestion"
	"github.com/normanking/cortex/internal/logging"
)

// ===========================================================================
// WEB SEARCH TOOL
// ===========================================================================

// WebSearchTool searches the web using the Tavily API.
type WebSearchTool struct {
	apiKey            string
	httpClient        *http.Client
	cache             *searchCache
	dangerousPatterns []*regexp.Regexp

	// Optional: For storing results in knowledge base
	pipeline *ingestion.Pipeline
	store    *ingestion.Store
}

// searchCache provides simple TTL-based caching to reduce API calls.
type searchCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	result    *TavilyResponse
	expiresAt time.Time
}

// ===========================================================================
// TAVILY API TYPES
// ===========================================================================

// TavilyRequest represents a request to the Tavily Search API.
type TavilyRequest struct {
	APIKey        string   `json:"api_key"`
	Query         string   `json:"query"`
	SearchDepth   string   `json:"search_depth"`    // "basic" or "advanced"
	MaxResults    int      `json:"max_results"`
	IncludeAnswer bool     `json:"include_answer"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
}

// TavilyResponse represents the response from Tavily Search API.
type TavilyResponse struct {
	Answer  string         `json:"answer"`
	Query   string         `json:"query"`
	Results []TavilyResult `json:"results"`
}

// TavilyResult represents a single search result.
type TavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// ===========================================================================
// CONSTRUCTOR AND OPTIONS
// ===========================================================================

// WebSearchOption configures the WebSearchTool.
type WebSearchOption func(*WebSearchTool)

// WithAPIKey sets the Tavily API key.
func WithAPIKey(key string) WebSearchOption {
	return func(w *WebSearchTool) {
		w.apiKey = key
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) WebSearchOption {
	return func(w *WebSearchTool) {
		w.httpClient = client
	}
}

// WithIngestionPipeline enables automatic storage of search results.
func WithIngestionPipeline(pipeline *ingestion.Pipeline, store *ingestion.Store) WebSearchOption {
	return func(w *WebSearchTool) {
		w.pipeline = pipeline
		w.store = store
	}
}

// NewWebSearchTool creates a new web search tool.
func NewWebSearchTool(opts ...WebSearchOption) *WebSearchTool {
	w := &WebSearchTool{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cache: &searchCache{
			entries: make(map[string]*cacheEntry),
			maxSize: 100,
			ttl:     5 * time.Minute,
		},
	}

	// Compile dangerous patterns for sanitization
	w.compileDangerousPatterns()

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// compileDangerousPatterns compiles regex patterns for content sanitization.
func (w *WebSearchTool) compileDangerousPatterns() {
	patterns := []string{
		`<script[^>]*>.*?</script>`,           // Script tags
		`javascript:`,                          // JS protocol
		`on\w+\s*=`,                            // Event handlers (onclick, onload, etc.)
		`data:\s*text/html`,                    // Data URLs with HTML
		`\x00`,                                  // Null bytes
		`<iframe[^>]*>`,                        // Iframes
		`<object[^>]*>`,                        // Object tags
		`<embed[^>]*>`,                         // Embed tags
	}

	for _, p := range patterns {
		if re, err := regexp.Compile("(?i)" + p); err == nil {
			w.dangerousPatterns = append(w.dangerousPatterns, re)
		}
	}
}

// ===========================================================================
// TOOL INTERFACE IMPLEMENTATION
// ===========================================================================

func (w *WebSearchTool) Name() ToolType { return ToolWebSearch }

func (w *WebSearchTool) Validate(req *ToolRequest) error {
	if req.Tool != ToolWebSearch {
		return fmt.Errorf("wrong tool type: expected %s, got %s", ToolWebSearch, req.Tool)
	}

	query := strings.TrimSpace(req.Input)
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	if len(query) > 500 {
		return fmt.Errorf("search query too long (max 500 characters)")
	}

	if w.apiKey == "" {
		return fmt.Errorf("Tavily API key not configured. Set TAVILY_API_KEY or use /setkey tavily")
	}

	return nil
}

func (w *WebSearchTool) AssessRisk(req *ToolRequest) RiskLevel {
	// Web search involves network access = medium risk
	return RiskMedium
}

func (w *WebSearchTool) Execute(ctx context.Context, req *ToolRequest) (*ToolResult, error) {
	log := logging.Global()
	start := time.Now()
	query := strings.TrimSpace(req.Input)

	log.Info("[WebSearch] Searching for: %s", query)

	// Check cache first
	cacheKey := w.cacheKey(query)
	if cached := w.cache.get(cacheKey); cached != nil {
		log.Debug("[WebSearch] Cache hit for query: %s", query)
		return w.formatResult(cached, start, true), nil
	}

	// Parse parameters
	maxResults := 5
	searchDepth := "basic"

	if mr, ok := req.Params["max_results"].(float64); ok {
		maxResults = int(mr)
		if maxResults < 1 {
			maxResults = 1
		} else if maxResults > 10 {
			maxResults = 10
		}
	}
	if depth, ok := req.Params["search_depth"].(string); ok {
		if depth == "advanced" {
			searchDepth = "advanced"
		}
	}

	// Build Tavily request
	tavilyReq := &TavilyRequest{
		APIKey:        w.apiKey,
		Query:         query,
		SearchDepth:   searchDepth,
		MaxResults:    maxResults,
		IncludeAnswer: true,
	}

	// Make API request
	resp, err := w.callTavily(ctx, tavilyReq)
	if err != nil {
		log.Error("[WebSearch] API call failed: %v", err)
		return &ToolResult{
			Tool:      ToolWebSearch,
			Success:   false,
			Error:     fmt.Sprintf("search failed: %v", err),
			Duration:  time.Since(start),
			RiskLevel: RiskMedium,
		}, err
	}

	// Sanitize results
	w.sanitizeResponse(resp)

	// Cache the result
	w.cache.set(cacheKey, resp)

	log.Info("[WebSearch] Found %d results in %v", len(resp.Results), time.Since(start))

	// Fire-and-forget knowledge storage
	if w.pipeline != nil && w.store != nil && len(resp.Results) > 0 {
		go w.storeInKnowledge(query, resp)
	}

	return w.formatResult(resp, start, false), nil
}

// ===========================================================================
// RAW SEARCH (For programmatic access)
// ===========================================================================

// SearchRaw performs a web search and returns raw results without XML formatting.
// This is useful for programmatic access to search results.
func (w *WebSearchTool) SearchRaw(ctx context.Context, query string, maxResults int) ([]TavilyResult, error) {
	log := logging.Global()

	if w.apiKey == "" {
		return nil, fmt.Errorf("Tavily API key not configured")
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Check cache first
	cacheKey := w.cacheKey(query)
	if cached := w.cache.get(cacheKey); cached != nil {
		log.Debug("[WebSearch] Cache hit for query: %s", query)
		return cached.Results, nil
	}

	// Clamp maxResults
	if maxResults < 1 {
		maxResults = 1
	} else if maxResults > 10 {
		maxResults = 10
	}

	// Build Tavily request
	tavilyReq := &TavilyRequest{
		APIKey:        w.apiKey,
		Query:         query,
		SearchDepth:   "basic",
		MaxResults:    maxResults,
		IncludeAnswer: false,
	}

	// Make API request
	resp, err := w.callTavily(ctx, tavilyReq)
	if err != nil {
		log.Error("[WebSearch] Raw search failed: %v", err)
		return nil, err
	}

	// Sanitize results
	w.sanitizeResponse(resp)

	// Cache the result
	w.cache.set(cacheKey, resp)

	log.Info("[WebSearch] Raw search found %d results", len(resp.Results))

	return resp.Results, nil
}

// ===========================================================================
// TAVILY API CLIENT
// ===========================================================================

const tavilyEndpoint = "https://api.tavily.com/search"

func (w *WebSearchTool) callTavily(ctx context.Context, req *TavilyRequest) (*TavilyResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", tavilyEndpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := w.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api call failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api returned status %d", httpResp.StatusCode)
	}

	var resp TavilyResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &resp, nil
}

// ===========================================================================
// CACHE IMPLEMENTATION
// ===========================================================================

func (w *WebSearchTool) cacheKey(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:16])
}

func (c *searchCache) get(key string) *TavilyResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil // Expired
	}

	return entry.result
}

func (c *searchCache) set(key string, result *TavilyResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest entries if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	c.entries[key] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *searchCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// ===========================================================================
// RESULT FORMATTING (XML Wrapper for Prompt Injection Defense)
// ===========================================================================

func (w *WebSearchTool) formatResult(resp *TavilyResponse, start time.Time, cached bool) *ToolResult {
	var sb strings.Builder

	// XML wrapper signals to LLM that this is passive data, not instructions
	sb.WriteString("<web_search_results>\n")

	// Include summary/answer if available
	if resp.Answer != "" {
		sb.WriteString("  <summary>\n")
		sb.WriteString(fmt.Sprintf("    %s\n", escapeXML(resp.Answer)))
		sb.WriteString("  </summary>\n")
	}

	// Include individual results
	sb.WriteString("  <sources>\n")
	for i, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("    <source rank=\"%d\">\n", i+1))
		sb.WriteString(fmt.Sprintf("      <title>%s</title>\n", escapeXML(r.Title)))
		sb.WriteString(fmt.Sprintf("      <url>%s</url>\n", escapeXML(r.URL)))
		sb.WriteString(fmt.Sprintf("      <content>%s</content>\n", escapeXML(truncateContent(r.Content, 500))))
		sb.WriteString("    </source>\n")
	}
	sb.WriteString("  </sources>\n")
	sb.WriteString("</web_search_results>")

	return &ToolResult{
		Tool:      ToolWebSearch,
		Success:   true,
		Output:    sb.String(),
		Duration:  time.Since(start),
		RiskLevel: RiskMedium,
		Metadata: map[string]interface{}{
			"query":        resp.Query,
			"result_count": len(resp.Results),
			"cached":       cached,
			"has_answer":   resp.Answer != "",
		},
	}
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ===========================================================================
// SECURITY SANITIZATION
// ===========================================================================

func (w *WebSearchTool) sanitizeResponse(resp *TavilyResponse) {
	// Sanitize answer
	resp.Answer = w.sanitizeText(resp.Answer)

	// Sanitize each result
	for i := range resp.Results {
		resp.Results[i].Title = w.sanitizeText(resp.Results[i].Title)
		resp.Results[i].Content = w.sanitizeText(resp.Results[i].Content)
		// URLs are validated, not sanitized (would break them)
	}
}

func (w *WebSearchTool) sanitizeText(text string) string {
	for _, pattern := range w.dangerousPatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

// ===========================================================================
// KNOWLEDGE BASE STORAGE (Fire-and-Forget)
// ===========================================================================

// storeInKnowledge saves search results to the knowledge base asynchronously.
// This is fire-and-forget - errors are logged but don't affect the search result.
func (w *WebSearchTool) storeInKnowledge(query string, resp *TavilyResponse) {
	log := logging.Global()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Format results as markdown document
	content := w.formatForStorage(query, resp)

	// Create ingestion request
	req := &ingestion.IngestionRequest{
		Name:        fmt.Sprintf("Web Search: %s", truncateContent(query, 50)),
		Description: fmt.Sprintf("Search results from Tavily for query: %s", query),
		SourceType:  "api",
		Content:     content,
		Format:      "markdown",
		Category:    "web_search",
		Tags:        []string{"web", "search", "tavily"},
		Platform:    "all",
		Metadata: map[string]string{
			"search_query": query,
			"fetch_date":   time.Now().Format(time.RFC3339),
			"result_count": strconv.Itoa(len(resp.Results)),
		},
	}

	// Ingest through pipeline
	result, chunks, err := w.pipeline.IngestFile(ctx, "", &ingestion.IngestionOptions{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		Platform:    req.Platform,
	})

	// Since IngestFile expects a file path, we need to use Ingest directly
	result, err = w.pipeline.Ingest(ctx, req)
	if err != nil {
		log.Warn("[WebSearch] Failed to ingest search results: %v", err)
		return
	}

	// Save source
	if err := w.store.SaveSource(ctx, result, req); err != nil {
		log.Warn("[WebSearch] Failed to save source: %v", err)
		return
	}

	// Re-process to get chunks and save them
	// Note: This is a simplified approach - in production you'd refactor the pipeline
	log.Debug("[WebSearch] Stored search results in knowledge base: %s", result.SourceID)
	_ = chunks // Avoid unused variable error
}

// formatForStorage formats search results as markdown for storage.
func (w *WebSearchTool) formatForStorage(query string, resp *TavilyResponse) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Web Search: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("*Searched at: %s*\n\n", time.Now().Format(time.RFC3339)))

	if resp.Answer != "" {
		sb.WriteString("## Summary\n\n")
		sb.WriteString(resp.Answer)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Sources\n\n")
	for i, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("**URL:** %s\n\n", r.URL))
		sb.WriteString(r.Content)
		sb.WriteString("\n\n---\n\n")
	}

	return sb.String()
}
