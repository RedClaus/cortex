package autollm

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// Default endpoint ports for local inference backends
const (
	DefaultMLXPort    = "8081"
	DefaultOllamaPort = "11434"
	DefaultDnetPort   = "9080"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AVAILABILITY CHECKER
// ═══════════════════════════════════════════════════════════════════════════════

// AvailabilityChecker determines which models are actually usable.
// It caches local backend status and API key availability to avoid repeated checks.
// Supports MLX, Ollama, and dnet backends with automatic fastest-backend detection.
type AvailabilityChecker struct {
	mu         sync.RWMutex
	cache      AvailabilityCache
	ollamaHost string
	mlxHost    string
	dnetHost   string
	cacheTTL   time.Duration
	httpClient *http.Client
	log        *logging.Logger
}

// NewAvailabilityChecker creates a new availability checker with support for
// all local backends (MLX, Ollama, dnet).
func NewAvailabilityChecker(ollamaHost string) *AvailabilityChecker {
	return NewAvailabilityCheckerWithEndpoints(ollamaHost, "", "")
}

// NewAvailabilityCheckerWithEndpoints creates a new availability checker with
// explicit endpoints for all local backends.
func NewAvailabilityCheckerWithEndpoints(ollamaHost, mlxHost, dnetHost string) *AvailabilityChecker {
	if ollamaHost == "" {
		ollamaHost = "http://127.0.0.1:" + DefaultOllamaPort
	}
	if mlxHost == "" {
		mlxHost = "http://127.0.0.1:" + DefaultMLXPort
	}
	if dnetHost == "" {
		dnetHost = "http://127.0.0.1:" + DefaultDnetPort
	}

	return &AvailabilityChecker{
		ollamaHost: ollamaHost,
		mlxHost:    mlxHost,
		dnetHost:   dnetHost,
		cacheTTL:   30 * time.Second, // Refresh every 30s
		cache: AvailabilityCache{
			OllamaModels:   make(map[string]bool),
			MLXModels:      make(map[string]bool),
			DnetModels:     make(map[string]bool),
			CloudProviders: make(map[string]bool),
		},
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		log: logging.Global(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REFRESH METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// Refresh updates the availability cache for all backends.
// This should be called at startup and periodically.
// Priority: MLX > dnet > Ollama (MLX is 5-10x faster on Apple Silicon)
func (c *AvailabilityChecker) Refresh(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check all local backends in parallel for faster startup
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		c.refreshMLX(ctx)
	}()

	go func() {
		defer wg.Done()
		c.refreshDnet(ctx)
	}()

	go func() {
		defer wg.Done()
		c.refreshOllama(ctx)
	}()

	wg.Wait()

	// Determine primary local backend (fastest available)
	// Priority: MLX > dnet > Ollama
	c.cache.PrimaryLocalBackend = ""
	if c.cache.MLXOnline && len(c.cache.MLXModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderMLX
	} else if c.cache.DnetOnline && len(c.cache.DnetModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderDnet
	} else if c.cache.OllamaOnline && len(c.cache.OllamaModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderOllama
	}

	// Check cloud API keys
	c.refreshCloudProviders()

	c.cache.LastRefresh = time.Now().Unix()

	// Log backend detection results
	if c.log != nil {
		c.log.Debug("[AutoLLM] Backend detection: MLX=%v, Ollama=%v, dnet=%v, Primary=%s",
			c.cache.MLXOnline, c.cache.OllamaOnline, c.cache.DnetOnline, c.cache.PrimaryLocalBackend)
	}

	return nil
}

// refreshMLX checks if MLX-LM server is running and which models are available.
// MLX uses OpenAI-compatible API at /v1/models
func (c *AvailabilityChecker) refreshMLX(ctx context.Context) {
	c.cache.MLXOnline = false
	c.cache.MLXModels = make(map[string]bool)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.mlxHost+"/v1/models", nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return // MLX not running
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	c.cache.MLXOnline = true

	// Parse OpenAI-compatible models list
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	for _, m := range result.Data {
		c.cache.MLXModels[m.ID] = true
		// Also store without slashes for matching (e.g., "mlx-community/Llama-3.2-3B" -> "Llama-3.2-3B")
		parts := strings.Split(m.ID, "/")
		if len(parts) > 1 {
			c.cache.MLXModels[parts[len(parts)-1]] = true
		}
	}
}

// refreshDnet checks if dnet distributed inference server is running.
// dnet uses OpenAI-compatible API at /v1/models
func (c *AvailabilityChecker) refreshDnet(ctx context.Context) {
	c.cache.DnetOnline = false
	c.cache.DnetModels = make(map[string]bool)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.dnetHost+"/v1/models", nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return // dnet not running
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	c.cache.DnetOnline = true

	// Parse OpenAI-compatible models list
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	for _, m := range result.Data {
		c.cache.DnetModels[m.ID] = true
		// Also store without slashes for matching
		parts := strings.Split(m.ID, "/")
		if len(parts) > 1 {
			c.cache.DnetModels[parts[len(parts)-1]] = true
		}
	}
}

// refreshOllama checks if Ollama is running and which models are pulled.
func (c *AvailabilityChecker) refreshOllama(ctx context.Context) {
	c.cache.OllamaOnline = false
	c.cache.OllamaModels = make(map[string]bool)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.ollamaHost+"/api/tags", nil)
	if err != nil {
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return // Ollama not running
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	c.cache.OllamaOnline = true

	// Parse the models list
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	for _, m := range result.Models {
		// Store both full name (e.g., "llama3:8b") and base name (e.g., "llama3")
		c.cache.OllamaModels[m.Name] = true
		baseName := strings.Split(m.Name, ":")[0]
		c.cache.OllamaModels[baseName] = true
	}
}

// refreshCloudProviders checks which cloud API keys are configured.
func (c *AvailabilityChecker) refreshCloudProviders() {
	c.cache.CloudProviders = map[string]bool{
		ProviderOpenAI:    os.Getenv("OPENAI_API_KEY") != "",
		ProviderAnthropic: os.Getenv("ANTHROPIC_API_KEY") != "",
		ProviderGoogle:    os.Getenv("GOOGLE_API_KEY") != "" || os.Getenv("GEMINI_API_KEY") != "",
		ProviderGemini:    os.Getenv("GOOGLE_API_KEY") != "" || os.Getenv("GEMINI_API_KEY") != "",
		ProviderMistral:   os.Getenv("MISTRAL_API_KEY") != "",
		ProviderGroq:      os.Getenv("GROQ_API_KEY") != "",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVAILABILITY CHECKS
// ═══════════════════════════════════════════════════════════════════════════════

// IsAvailable checks if a specific model can be used right now.
func (c *AvailabilityChecker) IsAvailable(model string, provider string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch provider {
	case ProviderMLX:
		if !c.cache.MLXOnline {
			return false
		}
		return c.cache.MLXModels[model] || c.modelExistsInMap(model, c.cache.MLXModels)

	case ProviderDnet:
		if !c.cache.DnetOnline {
			return false
		}
		return c.cache.DnetModels[model] || c.modelExistsInMap(model, c.cache.DnetModels)

	case ProviderOllama:
		if !c.cache.OllamaOnline {
			return false
		}
		baseName := strings.Split(model, ":")[0]
		return c.cache.OllamaModels[model] || c.cache.OllamaModels[baseName]

	case "local":
		// For generic "local" provider, check primary backend first, then others
		return c.isLocalModelAvailable(model)

	case ProviderGroq:
		return c.cache.CloudProviders[ProviderGroq]

	case ProviderOpenAI:
		return c.cache.CloudProviders[ProviderOpenAI]

	case ProviderAnthropic:
		return c.cache.CloudProviders[ProviderAnthropic]

	case ProviderGoogle, ProviderGemini:
		return c.cache.CloudProviders[ProviderGoogle]

	case ProviderMistral:
		return c.cache.CloudProviders[ProviderMistral]

	default:
		return false
	}
}

// isLocalModelAvailable checks if a model is available on any local backend.
func (c *AvailabilityChecker) isLocalModelAvailable(model string) bool {
	// Check MLX first (fastest)
	if c.cache.MLXOnline && (c.cache.MLXModels[model] || c.modelExistsInMap(model, c.cache.MLXModels)) {
		return true
	}
	// Check dnet second
	if c.cache.DnetOnline && (c.cache.DnetModels[model] || c.modelExistsInMap(model, c.cache.DnetModels)) {
		return true
	}
	// Check Ollama last
	if c.cache.OllamaOnline {
		baseName := strings.Split(model, ":")[0]
		if c.cache.OllamaModels[model] || c.cache.OllamaModels[baseName] {
			return true
		}
	}
	return false
}

// modelExistsInMap checks if model exists in map with partial matching.
func (c *AvailabilityChecker) modelExistsInMap(model string, m map[string]bool) bool {
	// Direct match
	if m[model] {
		return true
	}
	// Try without version/tag suffix
	parts := strings.Split(model, ":")
	if m[parts[0]] {
		return true
	}
	// Try last path component (e.g., "mlx-community/Llama" -> "Llama")
	pathParts := strings.Split(model, "/")
	if len(pathParts) > 1 {
		if m[pathParts[len(pathParts)-1]] {
			return true
		}
	}
	return false
}

// IsOllamaOnline returns whether Ollama is running.
func (c *AvailabilityChecker) IsOllamaOnline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.OllamaOnline
}

// IsMLXOnline returns whether MLX-LM server is running.
func (c *AvailabilityChecker) IsMLXOnline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.MLXOnline
}

// IsDnetOnline returns whether dnet server is running.
func (c *AvailabilityChecker) IsDnetOnline() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.DnetOnline
}

// HasAnyLocalBackend returns true if any local inference backend is available.
func (c *AvailabilityChecker) HasAnyLocalBackend() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.MLXOnline || c.cache.OllamaOnline || c.cache.DnetOnline
}

// GetPrimaryLocalBackend returns the fastest available local backend provider name.
// Returns empty string if no local backend is available.
func (c *AvailabilityChecker) GetPrimaryLocalBackend() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.PrimaryLocalBackend
}

// HasCloudProvider returns whether a cloud provider API key is configured.
func (c *AvailabilityChecker) HasCloudProvider(provider string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.CloudProviders[provider]
}

// GetOllamaModels returns the list of available Ollama models.
func (c *AvailabilityChecker) GetOllamaModels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var models []string
	seen := make(map[string]bool)
	for model := range c.cache.OllamaModels {
		// Only include full names (with tag), not base names
		if strings.Contains(model, ":") && !seen[model] {
			models = append(models, model)
			seen[model] = true
		}
	}
	return models
}

// GetMLXModels returns the list of available MLX models.
func (c *AvailabilityChecker) GetMLXModels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var models []string
	for model := range c.cache.MLXModels {
		// Only include full paths (with slash), not base names
		if strings.Contains(model, "/") {
			models = append(models, model)
		}
	}
	return models
}

// GetDnetModels returns the list of available dnet models.
func (c *AvailabilityChecker) GetDnetModels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var models []string
	for model := range c.cache.DnetModels {
		if strings.Contains(model, "/") {
			models = append(models, model)
		}
	}
	return models
}

// GetAllLocalModels returns models from all available local backends.
func (c *AvailabilityChecker) GetAllLocalModels() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]string)
	if c.cache.MLXOnline {
		result[ProviderMLX] = c.GetMLXModels()
	}
	if c.cache.OllamaOnline {
		result[ProviderOllama] = c.GetOllamaModels()
	}
	if c.cache.DnetOnline {
		result[ProviderDnet] = c.GetDnetModels()
	}
	return result
}

// IsCacheStale returns true if the cache should be refreshed.
func (c *AvailabilityChecker) IsCacheStale() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Now().Unix()-c.cache.LastRefresh > int64(c.cacheTTL.Seconds())
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATUS / DEBUG
// ═══════════════════════════════════════════════════════════════════════════════

// GetCache returns a copy of the current cache (for debugging/status).
func (c *AvailabilityChecker) GetCache() AvailabilityCache {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a deep copy to avoid race conditions
	cache := AvailabilityCache{
		OllamaOnline:        c.cache.OllamaOnline,
		OllamaModels:        make(map[string]bool),
		MLXOnline:           c.cache.MLXOnline,
		MLXModels:           make(map[string]bool),
		DnetOnline:          c.cache.DnetOnline,
		DnetModels:          make(map[string]bool),
		PrimaryLocalBackend: c.cache.PrimaryLocalBackend,
		CloudProviders:      make(map[string]bool),
		LastRefresh:         c.cache.LastRefresh,
	}

	for k, v := range c.cache.OllamaModels {
		cache.OllamaModels[k] = v
	}
	for k, v := range c.cache.MLXModels {
		cache.MLXModels[k] = v
	}
	for k, v := range c.cache.DnetModels {
		cache.DnetModels[k] = v
	}
	for k, v := range c.cache.CloudProviders {
		cache.CloudProviders[k] = v
	}

	return cache
}

// Status returns a human-readable status summary.
func (c *AvailabilityChecker) Status() map[string]interface{} {
	cache := c.GetCache()

	// Count unique models per backend
	ollamaCount := 0
	for model := range cache.OllamaModels {
		if strings.Contains(model, ":") {
			ollamaCount++
		}
	}

	mlxCount := 0
	for model := range cache.MLXModels {
		if strings.Contains(model, "/") {
			mlxCount++
		}
	}

	dnetCount := 0
	for model := range cache.DnetModels {
		if strings.Contains(model, "/") {
			dnetCount++
		}
	}

	// List available cloud providers
	var cloudAvailable []string
	for provider, available := range cache.CloudProviders {
		if available {
			cloudAvailable = append(cloudAvailable, provider)
		}
	}

	return map[string]interface{}{
		"primary_local_backend": cache.PrimaryLocalBackend,
		"mlx_online":            cache.MLXOnline,
		"mlx_model_count":       mlxCount,
		"ollama_online":         cache.OllamaOnline,
		"ollama_model_count":    ollamaCount,
		"dnet_online":           cache.DnetOnline,
		"dnet_model_count":      dnetCount,
		"cloud_providers":       cloudAvailable,
		"last_refresh_seconds":  time.Now().Unix() - cache.LastRefresh,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// ENDPOINT CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// SetAPIKey stores an API key for a provider (runtime only, not persisted).
// This allows programmatic key configuration beyond environment variables.
func (c *AvailabilityChecker) SetAPIKey(provider string, hasKey bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.CloudProviders[provider] = hasKey
}

// SetOllamaEndpoint updates the Ollama endpoint.
func (c *AvailabilityChecker) SetOllamaEndpoint(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ollamaHost = endpoint
}

// SetMLXEndpoint updates the MLX-LM server endpoint.
func (c *AvailabilityChecker) SetMLXEndpoint(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mlxHost = endpoint
}

// SetDnetEndpoint updates the dnet server endpoint.
func (c *AvailabilityChecker) SetDnetEndpoint(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dnetHost = endpoint
}

// SetAllEndpoints updates all local backend endpoints at once.
func (c *AvailabilityChecker) SetAllEndpoints(ollama, mlx, dnet string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ollama != "" {
		c.ollamaHost = ollama
	}
	if mlx != "" {
		c.mlxHost = mlx
	}
	if dnet != "" {
		c.dnetHost = dnet
	}
}
