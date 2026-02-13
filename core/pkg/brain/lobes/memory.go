package lobes

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

// MemoryLobe handles information storage and retrieval.
type MemoryLobe struct {
	store MemoryStore
}

// NewMemoryLobe creates a new memory lobe with the given store.
func NewMemoryLobe(store MemoryStore) *MemoryLobe {
	return &MemoryLobe{
		store: store,
	}
}

// ID returns brain.LobeMemory
func (l *MemoryLobe) ID() brain.LobeID {
	return brain.LobeMemory
}

// Process searches memory based on input and adds results to blackboard.
func (l *MemoryLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	start := time.Now()

	query := input.RawInput

	memories, err := l.store.Search(ctx, query, 5)
	if err != nil {
		return nil, err
	}

	for _, mem := range memories {
		bb.AddMemory(mem)
	}

	result := &brain.LobeResult{
		LobeID:  l.ID(),
		Content: memories,
		Meta: brain.LobeMeta{
			StartedAt:  start,
			Duration:   time.Since(start),
			TokensUsed: 0,
			ModelUsed:  "memory-store",
			CacheHit:   false,
		},
		Confidence: 1.0,
	}

	return result, nil
}

// CanHandle returns high confidence for memory-related queries.
func (l *MemoryLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"remember", "recall", "history", "what did", "search"}

	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.95
		}
	}

	return 0.1
}

// ResourceEstimate returns low resource requirements.
func (l *MemoryLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 100,
		EstimatedTime:   100 * time.Millisecond,
		RequiresGPU:     false,
	}
}
