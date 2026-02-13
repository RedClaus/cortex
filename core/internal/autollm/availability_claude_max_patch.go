package autollm

import (
	"context"
	"os/exec"
	"sync"
	"time"
)

// refreshClaudeMax checks if claude-code CLI is installed and authenticated.
// This is called as part of the main Refresh() method.
func (c *AvailabilityChecker) refreshClaudeMax(ctx context.Context) {
	c.cache.ClaudeMaxAvailable = false

	// Check if claude-code CLI is installed
	_, err := exec.LookPath("claude-code")
	if err != nil {
		return // claude-code not installed
	}

	// Try to run a simple command to verify it's authenticated
	// Using --version or similar command that doesn't require full auth
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude-code", "--version")
	if err := cmd.Run(); err == nil {
		c.cache.ClaudeMaxAvailable = true
		if c.log != nil {
			c.log.Debug("[AutoLLM] Claude Code CLI detected and available")
		}
	}
}

// Enhanced Refresh method that includes Claude Max detection
func (c *AvailabilityChecker) RefreshWithClaudeMax(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check all backends in parallel
	var wg sync.WaitGroup
	wg.Add(4) // Added one more for Claude Max

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

	go func() {
		defer wg.Done()
		c.refreshClaudeMax(ctx)
	}()

	wg.Wait()

	// Determine primary local backend
	c.cache.PrimaryLocalBackend = ""
	if c.cache.MLXOnline && len(c.cache.MLXModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderMLX
	} else if c.cache.DnetOnline && len(c.cache.DnetModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderDnet
	} else if c.cache.OllamaOnline && len(c.cache.OllamaModels) > 0 {
		c.cache.PrimaryLocalBackend = ProviderOllama
	}

	// Check cloud API keys (including Claude Max)
	c.refreshCloudProviders()

	// If Claude Max is available, override Anthropic availability
	if c.cache.ClaudeMaxAvailable {
		c.cache.CloudProviders[ProviderClaudeMax] = true
	}

	c.cache.LastRefresh = time.Now().Unix()

	// Log backend detection results
	if c.log != nil {
		c.log.Debug("[AutoLLM] Backend detection: MLX=%v, Ollama=%v, dnet=%v, ClaudeMax=%v, Primary=%s",
			c.cache.MLXOnline, c.cache.OllamaOnline, c.cache.DnetOnline, 
			c.cache.ClaudeMaxAvailable, c.cache.PrimaryLocalBackend)
	}

	return nil
}

// IsAvailableWithClaudeMax extends IsAvailable to support Claude Max provider
func (c *AvailabilityChecker) IsAvailableWithClaudeMax(model string, provider string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if provider == ProviderClaudeMax {
		return c.cache.ClaudeMaxAvailable
	}

	// Fall back to original IsAvailable for other providers
	return c.IsAvailable(model, provider)
}