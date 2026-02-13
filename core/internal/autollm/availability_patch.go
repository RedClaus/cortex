package autollm

import (
	"os/exec"
)

// Add to the existing refreshCloudProviders method:
// This should be integrated into the existing availability.go file

// CheckClaudeMaxAvailable checks if Claude CLI is installed and authenticated
func (c *AvailabilityChecker) CheckClaudeMaxAvailable() bool {
	// Check if claude CLI is installed
	_, err := exec.LookPath("claude")
	if err != nil {
		_, err = exec.LookPath("claude-code")
		if err != nil {
			return false
		}
	}

	// Try to create adapter to verify it works
	adapter, err := NewClaudeMaxAdapter()
	if err != nil {
		return false
	}

	return adapter.IsAvailable()
}

// Add this to refreshCloudProviders method in availability.go:
// c.cache.CloudProviders["claude-max"] = c.CheckClaudeMaxAvailable()
