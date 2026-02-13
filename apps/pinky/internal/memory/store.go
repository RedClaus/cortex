// Package memory implements Pinky's memory system with temporal awareness.
package memory

import (
	"context"
	"time"

	"github.com/normanking/pinky/internal/brain"
)

// Store is the interface for memory storage with temporal support.
type Store interface {
	// Store saves a memory to the store.
	Store(ctx context.Context, mem *brain.Memory) error

	// Recall retrieves memories matching a query with temporal awareness.
	Recall(ctx context.Context, query string, opts RecallOptions) ([]brain.Memory, error)

	// GetRecent retrieves the most recent memories for a user.
	GetRecent(ctx context.Context, userID string, limit int) ([]brain.Memory, error)

	// TemporalSearch searches memories with time-aware filtering.
	TemporalSearch(ctx context.Context, userID string, temporal *TemporalContext) ([]brain.Memory, error)

	// SemanticSearch searches memories using embedding similarity.
	// (Placeholder for future vector search implementation)
	SemanticSearch(ctx context.Context, embedding []float64, limit int) ([]brain.Memory, error)

	// Decay reduces importance of old unused memories.
	Decay(ctx context.Context) error

	// Consolidate merges similar memories.
	Consolidate(ctx context.Context) error

	// Prune removes old low-importance memories.
	Prune(ctx context.Context, maxAge time.Duration) error

	// Close closes the store and releases resources.
	Close() error
}

// RecallOptions configures memory recall behavior.
type RecallOptions struct {
	UserID        string
	Limit         int
	MinImportance float64
	Types         []brain.MemoryType
	Since         time.Time
	Until         time.Time
	TimeContext   *TemporalContext
}

// ScoredMemory wraps a memory with its relevance score.
type ScoredMemory struct {
	Memory brain.Memory
	Score  float64
}

// ScoreMemory calculates a relevance score for a memory given a query.
func ScoreMemory(mem *brain.Memory, query string, temporal *TemporalContext) float64 {
	score := mem.Importance

	// Semantic match boost (simple keyword overlap for now)
	score += keywordOverlap(mem.Content, query) * 0.3

	// Temporal match boost
	if temporal != nil && temporal.HasTimeReference {
		temporalScore := TemporalDistance(mem.CreatedAt, temporal)
		score += temporalScore * 0.4
	}

	// Recency boost - more recent memories get a small boost
	daysSinceAccess := time.Since(mem.AccessedAt).Hours() / 24
	recencyBoost := 0.1 / (1 + daysSinceAccess*0.1)
	score += recencyBoost

	// Access frequency boost
	if mem.AccessCount > 0 {
		score += 0.05 * float64(min(mem.AccessCount, 10)) / 10
	}

	return score
}

// keywordOverlap calculates simple keyword overlap between two strings.
func keywordOverlap(content, query string) float64 {
	contentWords := tokenize(content)
	queryWords := tokenize(query)

	if len(queryWords) == 0 {
		return 0
	}

	matches := 0
	for _, qw := range queryWords {
		for i := range contentWords {
			if qw == contentWords[i] {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(queryWords))
}

// tokenize splits text into lowercase words.
func tokenize(text string) []string {
	// Simple word tokenization - could be improved with proper NLP
	words := make([]string, 0)
	word := make([]rune, 0)

	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			word = append(word, toLower(r))
		} else if len(word) > 0 {
			words = append(words, string(word))
			word = word[:0]
		}
	}
	if len(word) > 0 {
		words = append(words, string(word))
	}

	return words
}

// toLower converts a rune to lowercase.
func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}
