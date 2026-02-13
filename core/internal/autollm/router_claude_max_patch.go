package autollm

import "strings"

// detectProviderWithClaudeMax is an enhanced version of detectProvider that
// checks if Claude Max should be used for Claude models.
func (r *Router) detectProviderWithClaudeMax(modelName string) string {
	modelLower := strings.ToLower(modelName)

	// Check for Claude models first
	if strings.Contains(modelLower, "claude") {
		// If UseClaudeMax is enabled and claude-code CLI is available, use it
		if r.config.UseClaudeMax && r.availability != nil && r.availability.cache.ClaudeMaxAvailable {
			return ProviderClaudeMax
		}
		// Otherwise fall back to standard Anthropic API
		return ProviderAnthropic
	}

	// For non-Claude models, use the original detection logic
	return r.detectProvider(modelName)
}