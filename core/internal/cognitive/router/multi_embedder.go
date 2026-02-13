// Package router provides semantic routing capabilities for the cognitive architecture.
package router

import (
	"context"
	"time"

	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
)

// MultiEmbedder tries multiple embedding backends in order until one succeeds.
// This provides graceful fallback from local (Ollama) to cloud (OpenAI) embedders.
type MultiEmbedder struct {
	embedders   []Embedder
	activeIndex int
	log         *logging.Logger
}

// NewMultiEmbedder creates a new multi-backend embedder.
// Backends are tried in order; the first available one is used.
func NewMultiEmbedder(embedders ...Embedder) *MultiEmbedder {
	log := logging.Global()

	m := &MultiEmbedder{
		embedders:   embedders,
		activeIndex: -1,
		log:         log,
	}

	// Find first available embedder
	for i, e := range embedders {
		if e != nil && e.Available() {
			m.activeIndex = i
			log.Info("[Embedder] Using backend: %s (dimension=%d)", e.ModelName(), e.Dimension())
			break
		}
	}

	if m.activeIndex < 0 {
		log.Warn("[Embedder] No embedding backends available - semantic features disabled")
	}

	return m
}

// active returns the currently active embedder, or nil if none available.
// Re-checks availability in case the current embedder became unavailable (e.g., quota exceeded).
func (m *MultiEmbedder) active() Embedder {
	if m.activeIndex < 0 || m.activeIndex >= len(m.embedders) {
		return nil
	}

	// Check if current embedder is still available
	current := m.embedders[m.activeIndex]
	if current != nil && current.Available() {
		return current
	}

	// Current embedder no longer available - find a new one
	m.log.Debug("[Embedder] Current embedder %s no longer available, searching for fallback", current.ModelName())
	for i, e := range m.embedders {
		if e != nil && e.Available() {
			if i != m.activeIndex {
				m.log.Info("[Embedder] Switching to: %s (previous unavailable)", e.ModelName())
			}
			m.activeIndex = i
			return e
		}
	}

	// No embedders available
	m.log.Warn("[Embedder] No embedding backends currently available")
	m.activeIndex = -1
	return nil
}

// tryFallback attempts to find the next available embedder.
func (m *MultiEmbedder) tryFallback() bool {
	for i := m.activeIndex + 1; i < len(m.embedders); i++ {
		if m.embedders[i] != nil && m.embedders[i].Available() {
			m.activeIndex = i
			m.log.Info("[Embedder] Falling back to: %s", m.embedders[i].ModelName())
			return true
		}
	}
	return false
}

// Embed generates an embedding using the active backend.
func (m *MultiEmbedder) Embed(ctx context.Context, text string) (cognitive.Embedding, error) {
	active := m.active()
	if active == nil {
		return nil, ErrEmbeddingTimeout
	}

	embedding, err := active.Embed(ctx, text)
	if err != nil {
		// Try fallback
		if m.tryFallback() {
			return m.embedders[m.activeIndex].Embed(ctx, text)
		}
		return nil, err
	}
	return embedding, nil
}

// EmbedFast generates an embedding with fast timeout.
func (m *MultiEmbedder) EmbedFast(ctx context.Context, text string) (cognitive.Embedding, error) {
	active := m.active()
	if active == nil {
		return nil, ErrEmbeddingTimeout
	}

	embedding, err := active.EmbedFast(ctx, text)
	if err != nil {
		if m.tryFallback() {
			return m.embedders[m.activeIndex].EmbedFast(ctx, text)
		}
		return nil, err
	}
	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (m *MultiEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]cognitive.Embedding, error) {
	active := m.active()
	if active == nil {
		return nil, ErrEmbeddingTimeout
	}

	embeddings, err := active.EmbedBatch(ctx, texts)
	if err != nil {
		if m.tryFallback() {
			return m.embedders[m.activeIndex].EmbedBatch(ctx, texts)
		}
		return nil, err
	}
	return embeddings, nil
}

// Dimension returns the embedding dimension of the active backend.
func (m *MultiEmbedder) Dimension() int {
	active := m.active()
	if active == nil {
		return cognitive.DefaultEmbeddingDim
	}
	return active.Dimension()
}

// ModelName returns the model name of the active backend.
func (m *MultiEmbedder) ModelName() string {
	active := m.active()
	if active == nil {
		return "none"
	}
	return active.ModelName()
}

// Available returns true if any backend is available.
func (m *MultiEmbedder) Available() bool {
	return m.active() != nil
}

// FastTimeout returns the fast timeout of the active backend.
func (m *MultiEmbedder) FastTimeout() time.Duration {
	active := m.active()
	if active == nil {
		return 5 * time.Second
	}
	return active.FastTimeout()
}

// ActiveBackend returns the name of the currently active backend.
func (m *MultiEmbedder) ActiveBackend() string {
	active := m.active()
	if active == nil {
		return "none"
	}
	return active.ModelName()
}

// BackendCount returns the number of configured backends.
func (m *MultiEmbedder) BackendCount() int {
	return len(m.embedders)
}
