// Package types provides shared types for the ui package and its subpackages.
// This package exists to break import cycles between ui and ui/modals.
package types

// ModelInfo describes an available AI model.
type ModelInfo struct {
	// ID is the unique identifier for the model (e.g., "gpt-4", "claude-3-opus")
	ID string

	// Name is the human-readable display name
	Name string

	// Provider is the backend provider (openai, anthropic, ollama, etc.)
	Provider string

	// IsLocal indicates if this is a locally-hosted model (Ollama, etc.)
	IsLocal bool

	// Capabilities describes what the model can do (vision, function calling, etc.)
	Capabilities []string

	// MaxTokens is the context window size
	MaxTokens int
}
