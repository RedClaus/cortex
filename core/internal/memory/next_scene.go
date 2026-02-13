// Package memory provides enhanced memory capabilities for Cortex.
// This file implements Next-Scene Prediction from CR-025 (Phase 2).
//
// Next-Scene Prediction anticipates memory needs and preloads relevant cubes
// BEFORE the user completes their input. This is inspired by MemOS research
// and aligns with the brain's predictive coding in the prefrontal cortex.
//
// Key characteristics:
// - 50ms timeout budget (must not impact TTFT)
// - Parallel retrieval with early termination on context cancellation
// - Cache to avoid redundant searches during typing
// - Background prefetch worker for speculative loading
package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// ============================================================================
// TYPES
// ============================================================================

// PredictionSignals extracted from input to guide retrieval.
// Only includes fields that are actively used (per audit).
type PredictionSignals struct {
	hasCodeIntent bool
	hasToolIntent bool
}

// MemCubeSearcher is the interface for cube storage search operations.
// This will be implemented by MemCubeStore when Phase 1 is complete.
type MemCubeSearcher interface {
	// SearchSimilar finds cubes similar to query, optionally filtered by type.
	// If cubeType is empty, searches all types.
	SearchSimilar(ctx context.Context, query string, cubeType CubeType, limit int) ([]*MemCube, error)
}

// ============================================================================
// NEXT SCENE PREDICTOR
// ============================================================================

// NextScenePredictor anticipates memory needs and preloads relevant cubes.
// It is designed for low-latency operation (50ms budget) and integrates with
// the PassiveRetriever to inject predicted memories into Fast Lane context.
//
// Brain Alignment: Mirrors predictive coding in the prefrontal cortex,
// where the brain anticipates upcoming stimuli based on patterns.
type NextScenePredictor struct {
	store      MemCubeSearcher
	cache      *sync.Map // map[string][]*MemCube - predicted cubes by input
	prefetchCh chan string
	closeCh    chan struct{}
	wg         sync.WaitGroup
}

// NewNextScenePredictor creates a predictor with a background prefetch worker.
// The worker enables speculative loading of related memories.
func NewNextScenePredictor(store MemCubeSearcher) *NextScenePredictor {
	nsp := &NextScenePredictor{
		store:      store,
		cache:      &sync.Map{},
		prefetchCh: make(chan string, 100),
		closeCh:    make(chan struct{}),
	}

	// Start background prefetch worker
	nsp.wg.Add(1)
	go nsp.prefetchWorker()

	return nsp
}

// Predict analyzes input and returns predicted relevant cubes.
// Called asynchronously as user types (debounced by caller).
//
// Design decisions:
// - 10 char minimum: Too short inputs lack signal for meaningful prediction
// - 50ms timeout: Matches PassiveRetriever budget to not impact TTFT
// - LoadOrStore: Avoids TOCTOU race condition (per audit fix)
// - Parallel goroutines: Maximize retrieval within timeout
// - Context cancellation checks: Enable early termination
func (nsp *NextScenePredictor) Predict(ctx context.Context, partialInput string) []*MemCube {
	// Skip inputs too short to predict meaningfully
	if len(partialInput) < 10 {
		return nil
	}

	// Check cache first using LoadOrStore to avoid TOCTOU race.
	// If another goroutine is already computing for this input,
	// we'll get their result or a nil placeholder.
	placeholder := ([]*MemCube)(nil)
	if cached, loaded := nsp.cache.LoadOrStore(partialInput, placeholder); loaded {
		if cached != nil {
			return cached.([]*MemCube)
		}
		// Another goroutine set placeholder but hasn't finished yet.
		// Rather than wait, we proceed with our own computation.
		// The last writer wins, which is fine for caching.
	}

	// Extract signals from partial input to guide retrieval strategy
	signals := nsp.extractSignals(partialInput)

	// Parallel retrieval with 50ms timeout (matches PassiveRetriever)
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	var cubes []*MemCube
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Search for relevant skills if code intent detected
	if signals.hasCodeIntent {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Check for context cancellation before starting work
			select {
			case <-ctx.Done():
				return
			default:
			}

			skills, err := nsp.store.SearchSimilar(ctx, partialInput, CubeTypeSkill, 3)
			if err != nil {
				log.Debug().Err(err).Msg("next-scene: skill search failed")
				return
			}

			// Check for context cancellation before appending
			select {
			case <-ctx.Done():
				return
			default:
			}

			mu.Lock()
			cubes = append(cubes, skills...)
			mu.Unlock()
		}()
	}

	// Always search for relevant knowledge
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Check for context cancellation before starting work
		select {
		case <-ctx.Done():
			return
		default:
		}

		knowledge, err := nsp.store.SearchSimilar(ctx, partialInput, CubeTypeText, 5)
		if err != nil {
			log.Debug().Err(err).Msg("next-scene: knowledge search failed")
			return
		}

		// Check for context cancellation before appending
		select {
		case <-ctx.Done():
			return
		default:
		}

		mu.Lock()
		cubes = append(cubes, knowledge...)
		mu.Unlock()
	}()

	// Search for tool patterns if command-like input detected
	if signals.hasToolIntent {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Check for context cancellation before starting work
			select {
			case <-ctx.Done():
				return
			default:
			}

			tools, err := nsp.store.SearchSimilar(ctx, partialInput, CubeTypeTool, 2)
			if err != nil {
				log.Debug().Err(err).Msg("next-scene: tool search failed")
				return
			}

			// Check for context cancellation before appending
			select {
			case <-ctx.Done():
				return
			default:
			}

			mu.Lock()
			cubes = append(cubes, tools...)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Cache results for subsequent calls with same input
	if len(cubes) > 0 {
		nsp.cache.Store(partialInput, cubes)
	}

	log.Debug().
		Str("input", truncatePredictorInput(partialInput)).
		Int("cubes_found", len(cubes)).
		Bool("code_intent", signals.hasCodeIntent).
		Bool("tool_intent", signals.hasToolIntent).
		Msg("next-scene prediction completed")

	return cubes
}

// extractSignals analyzes input to determine retrieval strategy.
// Returns only actively used signals (hasCodeIntent, hasToolIntent).
func (nsp *NextScenePredictor) extractSignals(input string) PredictionSignals {
	lower := strings.ToLower(input)

	return PredictionSignals{
		hasCodeIntent: strings.Contains(lower, "code") ||
			strings.Contains(lower, "implement") ||
			strings.Contains(lower, "function") ||
			strings.Contains(lower, "fix") ||
			strings.Contains(lower, "bug") ||
			strings.Contains(lower, "error") ||
			strings.Contains(lower, "class") ||
			strings.Contains(lower, "method"),

		hasToolIntent: strings.Contains(lower, "run") ||
			strings.Contains(lower, "execute") ||
			strings.Contains(lower, "command") ||
			strings.Contains(lower, "shell") ||
			strings.Contains(lower, "terminal") ||
			strings.HasPrefix(lower, "/"),
	}
}

// Prefetch queues an input for background speculative loading.
// This is non-blocking - if the queue is full, the request is dropped.
func (nsp *NextScenePredictor) Prefetch(input string) {
	if len(input) < 10 {
		return
	}

	// Non-blocking send - drop if queue is full
	select {
	case nsp.prefetchCh <- input:
	default:
		// Queue full, drop this prefetch request
	}
}

// prefetchWorker processes speculative prefetch requests in the background.
// This enables warming the cache before the user completes typing.
func (nsp *NextScenePredictor) prefetchWorker() {
	defer nsp.wg.Done()

	for {
		select {
		case <-nsp.closeCh:
			return
		case input := <-nsp.prefetchCh:
			// Skip if already cached
			if _, exists := nsp.cache.Load(input); exists {
				continue
			}

			// Use background context with reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)

			// Perform prediction (will cache results)
			_ = nsp.Predict(ctx, input)

			cancel()
		}
	}
}

// ClearCache clears the prediction cache.
// Useful when memory content has changed significantly.
func (nsp *NextScenePredictor) ClearCache() {
	nsp.cache = &sync.Map{}
}

// Close shuts down the predictor and its background worker.
// This should be called when the predictor is no longer needed.
func (nsp *NextScenePredictor) Close() {
	close(nsp.closeCh)
	nsp.wg.Wait()
}

// truncatePredictorInput truncates input for logging.
func truncatePredictorInput(input string) string {
	if len(input) > 40 {
		return input[:40] + "..."
	}
	return input
}

// ============================================================================
// CONTEXT INJECTION
// ============================================================================

// InjectPredictedCubes formats predicted cubes for context injection.
// Returns an empty string if no cubes are provided.
func InjectPredictedCubes(cubes []*MemCube) string {
	if len(cubes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<predicted_context>\n")
	sb.WriteString("Anticipated relevant information:\n")

	for _, cube := range cubes {
		if cube == nil {
			continue
		}

		// Format based on content type
		switch cube.ContentType {
		case CubeTypeSkill:
			sb.WriteString("  [Skill] ")
		case CubeTypeTool:
			sb.WriteString("  [Tool] ")
		default:
			sb.WriteString("  ")
		}

		// Truncate long content
		content := cube.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}

	sb.WriteString("</predicted_context>\n")
	return sb.String()
}
