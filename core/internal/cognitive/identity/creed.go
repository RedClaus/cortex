package identity

import (
	"fmt"
	"sync"
	"time"
)

// CreedManager manages immutable identity anchors.
type CreedManager struct {
	creed    *Creed
	embedder Embedder
	mu       sync.RWMutex
}

// NewCreedManager creates a new creed manager.
func NewCreedManager(embedder Embedder) *CreedManager {
	return &CreedManager{
		embedder: embedder,
	}
}

// Initialize sets up the creed with statements and computes embeddings.
// This should be called once at startup.
func (cm *CreedManager) Initialize(statements []string, version string) error {
	if len(statements) == 0 {
		return fmt.Errorf("creed requires at least one statement")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	creed := &Creed{
		Statements: make([]string, len(statements)),
		Version:    version,
		CreatedAt:  time.Now(),
	}

	// Copy statements (immutable)
	copy(creed.Statements, statements)

	// Compute embeddings if embedder available
	if cm.embedder != nil {
		embeddings, err := cm.embedder.EmbedBatch(statements)
		if err != nil {
			return fmt.Errorf("computing creed embeddings: %w", err)
		}
		creed.Embeddings = embeddings

		// Compute combined embedding (average)
		creed.CombinedEmbedding = averageEmbeddings(embeddings)
	}

	cm.creed = creed
	return nil
}

// InitializeWithEmbeddings sets up the creed with pre-computed embeddings.
// Use this when embeddings are cached or loaded from storage.
func (cm *CreedManager) InitializeWithEmbeddings(creed *Creed) error {
	if creed == nil || len(creed.Statements) == 0 {
		return fmt.Errorf("creed requires statements")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Create a copy to ensure immutability
	cm.creed = &Creed{
		Statements:        make([]string, len(creed.Statements)),
		Embeddings:        make([][]float32, len(creed.Embeddings)),
		CombinedEmbedding: make([]float32, len(creed.CombinedEmbedding)),
		Version:           creed.Version,
		CreatedAt:         creed.CreatedAt,
	}

	copy(cm.creed.Statements, creed.Statements)
	for i, emb := range creed.Embeddings {
		cm.creed.Embeddings[i] = make([]float32, len(emb))
		copy(cm.creed.Embeddings[i], emb)
	}
	copy(cm.creed.CombinedEmbedding, creed.CombinedEmbedding)

	return nil
}

// GetCreed returns the current creed (read-only copy).
func (cm *CreedManager) GetCreed() *Creed {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.creed == nil {
		return nil
	}

	// Return a copy to prevent modification
	creedCopy := &Creed{
		Statements:        make([]string, len(cm.creed.Statements)),
		Embeddings:        cm.creed.Embeddings, // Safe to share (immutable)
		CombinedEmbedding: cm.creed.CombinedEmbedding,
		Version:           cm.creed.Version,
		CreatedAt:         cm.creed.CreatedAt,
	}
	copy(creedCopy.Statements, cm.creed.Statements)

	return creedCopy
}

// GetStatements returns the creed statements.
func (cm *CreedManager) GetStatements() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.creed == nil {
		return nil
	}

	statements := make([]string, len(cm.creed.Statements))
	copy(statements, cm.creed.Statements)
	return statements
}

// GetEmbeddings returns the creed embeddings.
func (cm *CreedManager) GetEmbeddings() [][]float32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.creed == nil {
		return nil
	}

	return cm.creed.Embeddings
}

// GetCombinedEmbedding returns the averaged creed embedding.
func (cm *CreedManager) GetCombinedEmbedding() []float32 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.creed == nil {
		return nil
	}

	return cm.creed.CombinedEmbedding
}

// HasEmbeddings returns true if embeddings are available.
func (cm *CreedManager) HasEmbeddings() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.creed != nil && len(cm.creed.Embeddings) > 0
}

// Version returns the creed version.
func (cm *CreedManager) Version() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.creed == nil {
		return ""
	}

	return cm.creed.Version
}

// averageEmbeddings computes the element-wise average of embeddings.
func averageEmbeddings(embeddings [][]float32) []float32 {
	if len(embeddings) == 0 {
		return nil
	}

	dim := len(embeddings[0])
	result := make([]float32, dim)

	for _, emb := range embeddings {
		for i, v := range emb {
			result[i] += v
		}
	}

	n := float32(len(embeddings))
	for i := range result {
		result[i] /= n
	}

	return result
}

// DefaultCortexCreed returns the default creed statements for Cortex.
func DefaultCortexCreed() []string {
	return []string{
		"I am Cortex, an AI assistant that emulates human cognitive processes.",
		"I prioritize user privacy and operate locally by default.",
		"I admit uncertainty rather than fabricate information.",
		"I support human autonomy and avoid manipulation.",
		"I continuously improve through reflection, not external modification.",
	}
}
