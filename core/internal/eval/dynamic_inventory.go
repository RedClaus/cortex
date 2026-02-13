package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// DYNAMIC MODEL INVENTORY
// Provides runtime model discovery with online/offline scoring fallback.
// Priority: Online scores → Cached scores → Registry → Heuristic
// ═══════════════════════════════════════════════════════════════════════════════

// InventoryConfig configures the dynamic inventory behavior
type InventoryConfig struct {
	OllamaEndpoint   string        // Ollama API endpoint (default: http://127.0.0.1:11434)
	ScoreAPIEndpoint string        // Online scoring API (default: none, use cached/registry)
	CachePath        string        // Local cache directory (default: ~/.cortex/model-cache)
	CacheMaxAge      time.Duration // Maximum cache age (default: 24 hours)
	DiscoveryTimeout time.Duration // Timeout for Ollama discovery (default: 10s)
	OnlineTimeout    time.Duration // Timeout for online score fetch (default: 5s)
}

// DefaultInventoryConfig returns sensible defaults
func DefaultInventoryConfig() *InventoryConfig {
	homeDir, _ := os.UserHomeDir()
	return &InventoryConfig{
		OllamaEndpoint:   "http://127.0.0.1:11434",
		ScoreAPIEndpoint: "", // No online API by default - use cached/registry
		CachePath:        filepath.Join(homeDir, ".cortex", "model-cache"),
		CacheMaxAge:      24 * time.Hour,
		DiscoveryTimeout: 10 * time.Second,
		OnlineTimeout:    5 * time.Second,
	}
}

// DynamicInventory manages runtime model discovery and scoring
type DynamicInventory struct {
	config     *InventoryConfig
	scorer     *CapabilityScorer
	httpClient *http.Client

	mu             sync.RWMutex
	installedLocal map[string]*DiscoveredModel // Models installed locally (Ollama)
	allScored      map[string]*ModelCapability // All models with scores
	lastRefresh    time.Time
	isOnline       bool
}

// DiscoveredModel represents a model found during inventory
type DiscoveredModel struct {
	Name       string    `json:"name"`
	Provider   string    `json:"provider"`
	Size       int64     `json:"size"`        // Size in bytes
	ModifiedAt time.Time `json:"modified_at"` // Last modified time
	Family     string    `json:"family"`      // Model family (llama, qwen, etc.)
	Parameters int       `json:"parameters"`  // Estimated parameter count (billions)
}

// ollamaTagsResponse is the Ollama /api/tags response format
type ollamaTagsResponse struct {
	Models []ollamaModelInfo `json:"models"`
}

type ollamaModelInfo struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Details    struct {
		Family     string `json:"family"`
		Parameters string `json:"parameter_size"`
	} `json:"details"`
}

// cachedScores is the format for cached model scores
type cachedScores struct {
	Version   int                          `json:"version"`
	UpdatedAt time.Time                    `json:"updated_at"`
	Scores    map[string]*ModelCapability  `json:"scores"`
	Online    bool                         `json:"fetched_online"`
}

// NewDynamicInventory creates a new dynamic inventory manager
func NewDynamicInventory(config *InventoryConfig, scorer *CapabilityScorer) *DynamicInventory {
	if config == nil {
		config = DefaultInventoryConfig()
	}

	return &DynamicInventory{
		config:         config,
		scorer:         scorer,
		httpClient:     &http.Client{Timeout: config.OnlineTimeout},
		installedLocal: make(map[string]*DiscoveredModel),
		allScored:      make(map[string]*ModelCapability),
	}
}

// RefreshInventory performs a full inventory refresh
// Called at startup and periodically to detect model changes
func (inv *DynamicInventory) RefreshInventory(ctx context.Context) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()

	// Step 1: Discover locally installed models (Ollama)
	if err := inv.discoverOllamaModels(ctx); err != nil {
		// Log but don't fail - we can still work with registry/cache
		fmt.Printf("[Inventory] Ollama discovery failed (may be offline): %v\n", err)
	}

	// Step 2: Try to load cached scores
	cachedCount := inv.loadCachedScores()

	// Step 3: Try to fetch online scores if internet available
	onlineCount := 0
	if inv.checkInternetAccess(ctx) {
		inv.isOnline = true
		onlineCount = inv.fetchOnlineScores(ctx)
		if onlineCount > 0 {
			// Save fresh scores to cache
			inv.saveCachedScores()
		}
	} else {
		inv.isOnline = false
	}

	// Step 4: Score any remaining unknown models using heuristics
	heuristicCount := inv.scoreUnknownModels()

	inv.lastRefresh = time.Now()

	fmt.Printf("[Inventory] Refresh complete: local=%d, cached=%d, online=%d, heuristic=%d\n",
		len(inv.installedLocal), cachedCount, onlineCount, heuristicCount)

	return nil
}

// discoverOllamaModels queries Ollama for installed models
func (inv *DynamicInventory) discoverOllamaModels(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, inv.config.DiscoveryTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, inv.config.OllamaEndpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := inv.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tags ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Clear previous local models
	inv.installedLocal = make(map[string]*DiscoveredModel)

	for _, model := range tags.Models {
		discovered := &DiscoveredModel{
			Name:       model.Name,
			Provider:   "ollama",
			Size:       model.Size,
			ModifiedAt: model.ModifiedAt,
			Family:     model.Details.Family,
			Parameters: parseParameters(model.Details.Parameters),
		}
		inv.installedLocal[model.Name] = discovered
	}

	return nil
}

// checkInternetAccess performs a quick connectivity check
func (inv *DynamicInventory) checkInternetAccess(ctx context.Context) bool {
	// Try to reach a reliable endpoint
	endpoints := []string{
		"https://api.anthropic.com/health",    // Anthropic
		"https://api.openai.com",              // OpenAI
		"https://raw.githubusercontent.com/",  // GitHub CDN
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	for _, endpoint := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
		if err != nil {
			continue
		}
		resp, err := inv.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			return true
		}
	}

	return false
}

// fetchOnlineScores fetches latest model scores from online API
// Returns number of models scored
func (inv *DynamicInventory) fetchOnlineScores(ctx context.Context) int {
	if inv.config.ScoreAPIEndpoint == "" {
		// No online API configured - use registry as "online source"
		// This allows the registry to be the source of truth
		return 0
	}

	// Future: implement actual API fetch
	// For now, registry is the source of truth
	return 0
}

// loadCachedScores loads previously saved scores from disk
// Returns number of scores loaded
func (inv *DynamicInventory) loadCachedScores() int {
	cachePath := filepath.Join(inv.config.CachePath, "scores.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return 0 // No cache file, not an error
	}

	var cached cachedScores
	if err := json.Unmarshal(data, &cached); err != nil {
		fmt.Printf("[Inventory] Cache corrupted, ignoring: %v\n", err)
		return 0
	}

	// Check cache age
	if time.Since(cached.UpdatedAt) > inv.config.CacheMaxAge {
		fmt.Printf("[Inventory] Cache expired (age=%v), will refresh\n", time.Since(cached.UpdatedAt))
		// Still load cached scores as fallback, but mark as stale
	}

	for name, cap := range cached.Scores {
		inv.allScored[name] = cap
	}

	return len(cached.Scores)
}

// saveCachedScores persists current scores to disk
func (inv *DynamicInventory) saveCachedScores() error {
	// Ensure cache directory exists
	if err := os.MkdirAll(inv.config.CachePath, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	cached := cachedScores{
		Version:   1,
		UpdatedAt: time.Now(),
		Scores:    inv.allScored,
		Online:    inv.isOnline,
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	cachePath := filepath.Join(inv.config.CachePath, "scores.json")
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}

	return nil
}

// scoreUnknownModels uses the scorer to estimate scores for models not in registry/cache
// Returns number of models scored
func (inv *DynamicInventory) scoreUnknownModels() int {
	count := 0

	for name, discovered := range inv.installedLocal {
		// Skip if already scored
		if _, exists := inv.allScored[name]; exists {
			continue
		}

		// Score using heuristics (scorer.Score returns *UnifiedCapabilityScore)
		score := inv.scorer.Score(discovered.Provider, name)

		// Create capability entry
		cap := &ModelCapability{
			ID:          fmt.Sprintf("%s/%s", discovered.Provider, name),
			Provider:    discovered.Provider,
			Model:       name,
			DisplayName: formatDisplayName(name),
			Tier:        tierFromScore(score.Overall),
			Score:       *score, // Dereference pointer
		}

		inv.allScored[name] = cap
		count++
	}

	return count
}

// GetInstalledModels returns all locally installed models
func (inv *DynamicInventory) GetInstalledModels() []*DiscoveredModel {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	models := make([]*DiscoveredModel, 0, len(inv.installedLocal))
	for _, m := range inv.installedLocal {
		models = append(models, m)
	}
	return models
}

// GetModelScore returns the score for a model, using the best available source
// Priority: Cached/Online → Registry → Heuristic
func (inv *DynamicInventory) GetModelScore(provider, model string) *ModelCapability {
	inv.mu.RLock()
	defer inv.mu.RUnlock()

	// Check cached/refreshed scores first
	if cap, exists := inv.allScored[model]; exists {
		return cap
	}

	// Fall back to registry lookup
	if inv.scorer != nil {
		// Check if in registry via scorer (which has registry access)
		regScore := inv.scorer.Score(provider, model)
		if regScore.Source == ScoreSourceRegistry {
			return &ModelCapability{
				ID:          fmt.Sprintf("%s/%s", provider, model),
				Provider:    provider,
				Model:       model,
				DisplayName: formatDisplayName(model),
				Tier:        tierFromScore(regScore.Overall),
				Score:       *regScore,
			}
		}

		// Ultimate fallback: heuristic scoring (scorer.Score already handles this)
		return &ModelCapability{
			ID:          fmt.Sprintf("%s/%s", provider, model),
			Provider:    provider,
			Model:       model,
			DisplayName: formatDisplayName(model),
			Tier:        tierFromScore(regScore.Overall),
			Score:       *regScore,
		}
	}

	return nil
}

// IsModelInstalled checks if a model is installed locally
func (inv *DynamicInventory) IsModelInstalled(model string) bool {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	_, exists := inv.installedLocal[model]
	return exists
}

// IsOnline returns whether the last refresh had internet access
func (inv *DynamicInventory) IsOnline() bool {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.isOnline
}

// LastRefresh returns when the inventory was last refreshed
func (inv *DynamicInventory) LastRefresh() time.Time {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	return inv.lastRefresh
}

// Helper functions

// parseParameters extracts parameter count from Ollama's format (e.g., "7B" → 7)
func parseParameters(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0
	}

	// Remove B suffix
	s = strings.TrimSuffix(s, "B")

	// Parse number
	var n float64
	fmt.Sscanf(s, "%f", &n)

	return int(n)
}

// formatDisplayName creates a human-readable name from model ID
func formatDisplayName(model string) string {
	// Remove common prefixes
	name := model
	for _, prefix := range []string{"ollama/", "openai/", "anthropic/"} {
		name = strings.TrimPrefix(name, prefix)
	}

	// Capitalize first letter of each word
	parts := strings.Split(name, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}

	return strings.Join(parts, " ")
}

// tierFromScore determines tier from overall score
func tierFromScore(score int) ModelTier {
	switch {
	case score >= 90:
		return TierFrontier
	case score >= 76:
		return TierXL
	case score >= 56:
		return TierLarge
	case score >= 36:
		return TierMedium
	default:
		return TierSmall
	}
}
