package lobes

import (
	"context"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// LLMProvider defines the interface for language model interactions.
type LLMProvider interface {
	Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error)
}

// MemoryStore defines the interface for memory operations.
type MemoryStore interface {
	Search(ctx context.Context, query string, limit int) ([]brain.Memory, error)
	Store(ctx context.Context, content string, metadata map[string]string) error
}
