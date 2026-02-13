package autollm

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
)

// SpeculativeExecutor runs a fast model in parallel with the primary model.
// The fast model provides an immediate draft response while the primary model
// generates the full quality response.
type SpeculativeExecutor struct {
	fastProvider   llm.Provider // Small, fast model (e.g., Groq, phi3:mini)
	primaryProvider llm.Provider // Primary model (e.g., qwen2.5-coder:14b)

	// Model names
	fastModel    string
	primaryModel string

	// Configuration
	fastTimeout     time.Duration // Max time to wait for fast model
	acceptThreshold float64       // Confidence threshold to accept fast response

	// Metrics
	mu                sync.RWMutex
	fastAccepted      int64
	fastRejected      int64
	primaryUsed       int64
	avgFastLatency    time.Duration
	avgPrimaryLatency time.Duration

	log *logging.Logger
}

// SpeculativeConfig configures the speculative executor.
type SpeculativeConfig struct {
	FastProvider    llm.Provider
	PrimaryProvider llm.Provider
	FastModel       string
	PrimaryModel    string
	FastTimeout     time.Duration
	AcceptThreshold float64
}

// NewSpeculativeExecutor creates a new speculative executor.
func NewSpeculativeExecutor(cfg SpeculativeConfig) *SpeculativeExecutor {
	if cfg.FastTimeout == 0 {
		cfg.FastTimeout = 2 * time.Second
	}
	if cfg.AcceptThreshold == 0 {
		cfg.AcceptThreshold = 0.7
	}

	return &SpeculativeExecutor{
		fastProvider:    cfg.FastProvider,
		primaryProvider: cfg.PrimaryProvider,
		fastModel:       cfg.FastModel,
		primaryModel:    cfg.PrimaryModel,
		fastTimeout:     cfg.FastTimeout,
		acceptThreshold: cfg.AcceptThreshold,
		log:             logging.Global(),
	}
}

// SpeculativeResult contains both fast and primary responses.
type SpeculativeResult struct {
	FastResponse    string        `json:"fast_response"`
	PrimaryResponse string        `json:"primary_response"`
	FastAccepted    bool          `json:"fast_accepted"`
	FastLatency     time.Duration `json:"fast_latency"`
	PrimaryLatency  time.Duration `json:"primary_latency"`
	TotalLatency    time.Duration `json:"total_latency"`
}

// Execute runs both models and returns the best response.
// If fast model returns quickly and response is high-confidence, use it.
// Otherwise, wait for primary model.
func (s *SpeculativeExecutor) Execute(ctx context.Context, req *llm.ChatRequest) (*SpeculativeResult, error) {
	start := time.Now()
	result := &SpeculativeResult{}

	// Channels for responses
	fastCh := make(chan *llm.ChatResponse, 1)
	fastErrCh := make(chan error, 1)
	primaryCh := make(chan *llm.ChatResponse, 1)
	primaryErrCh := make(chan error, 1)

	// Start fast model (with timeout)
	fastCtx, fastCancel := context.WithTimeout(ctx, s.fastTimeout)
	go func() {
		defer fastCancel()
		fastStart := time.Now()

		fastReq := &llm.ChatRequest{
			Model:        s.fastModel,
			Messages:     req.Messages,
			SystemPrompt: req.SystemPrompt,
			MaxTokens:    req.MaxTokens,
			Temperature:  0.3, // Lower temperature for consistency
		}

		resp, err := s.fastProvider.Chat(fastCtx, fastReq)
		result.FastLatency = time.Since(fastStart)

		if err != nil {
			fastErrCh <- err
			return
		}
		fastCh <- resp
	}()

	// Start primary model in parallel
	go func() {
		primaryStart := time.Now()

		primaryReq := &llm.ChatRequest{
			Model:        s.primaryModel,
			Messages:     req.Messages,
			SystemPrompt: req.SystemPrompt,
			MaxTokens:    req.MaxTokens,
			Temperature:  req.Temperature,
		}

		resp, err := s.primaryProvider.Chat(ctx, primaryReq)
		result.PrimaryLatency = time.Since(primaryStart)

		if err != nil {
			primaryErrCh <- err
			return
		}
		primaryCh <- resp
	}()

	// Wait for fast model first
	var fastResp *llm.ChatResponse
	select {
	case fastResp = <-fastCh:
		result.FastResponse = fastResp.Content
		s.log.Debug("[Speculative] Fast model returned in %v", result.FastLatency)

	case err := <-fastErrCh:
		s.log.Debug("[Speculative] Fast model error: %v", err)

	case <-fastCtx.Done():
		s.log.Debug("[Speculative] Fast model timeout after %v", s.fastTimeout)
	}

	// Check if fast response is acceptable
	if fastResp != nil && s.isHighConfidence(fastResp.Content) {
		result.FastAccepted = true
		s.updateMetrics(true, result.FastLatency, 0)

		// Still wait for primary in background for verification (optional)
		go func() {
			select {
			case primaryResp := <-primaryCh:
				if !s.responsesMatch(result.FastResponse, primaryResp.Content) {
					s.log.Info("[Speculative] Fast/Primary mismatch - logging for analysis")
				}
			case <-primaryErrCh:
				// Primary failed, fast was correct choice
			case <-ctx.Done():
				// Context cancelled
			}
		}()

		result.TotalLatency = time.Since(start)
		s.log.Info("[Speculative] ACCEPTED fast response (latency: %v)", result.FastLatency)
		return result, nil
	}

	// Fast wasn't acceptable, wait for primary
	select {
	case primaryResp := <-primaryCh:
		result.PrimaryResponse = primaryResp.Content
		result.FastAccepted = false
		s.updateMetrics(false, result.FastLatency, result.PrimaryLatency)

	case err := <-primaryErrCh:
		// Primary failed - if we have fast response, use it as fallback
		if result.FastResponse != "" {
			result.FastAccepted = true
			result.TotalLatency = time.Since(start)
			s.log.Warn("[Speculative] Primary failed, using fast fallback: %v", err)
			return result, nil
		}
		return nil, err

	case <-ctx.Done():
		// Context cancelled - if we have fast response, use it
		if result.FastResponse != "" {
			result.FastAccepted = true
			result.TotalLatency = time.Since(start)
			s.log.Warn("[Speculative] Context cancelled, using fast response")
			return result, nil
		}
		return nil, ctx.Err()
	}

	result.TotalLatency = time.Since(start)
	s.log.Info("[Speculative] Used primary response (fast: %v, primary: %v)", result.FastLatency, result.PrimaryLatency)
	return result, nil
}

// GetResponse returns the best available response from the result.
func (r *SpeculativeResult) GetResponse() string {
	if r.FastAccepted {
		return r.FastResponse
	}
	return r.PrimaryResponse
}

// isHighConfidence checks if the fast response looks reliable.
func (s *SpeculativeExecutor) isHighConfidence(response string) bool {
	// Empty or very short responses are low confidence
	if len(response) < 20 {
		return false
	}

	// Responses that are too long might be rambling
	if len(response) > 5000 {
		return false
	}

	lower := strings.ToLower(response)

	// Hedging language indicates uncertainty
	hedgingPhrases := []string{
		"i'm not sure",
		"i'm not certain",
		"i don't know",
		"i cannot",
		"i can't",
		"maybe",
		"perhaps",
		"possibly",
		"it depends",
		"i think",
		"i believe",
		"might be",
		"could be",
	}

	for _, phrase := range hedgingPhrases {
		if strings.Contains(lower, phrase) {
			return false
		}
	}

	// Error indicators
	errorPhrases := []string{
		"error",
		"failed",
		"unable to",
		"cannot process",
		"invalid",
		"sorry",
	}

	for _, phrase := range errorPhrases {
		if strings.HasPrefix(lower, phrase) {
			return false
		}
	}

	return true
}

// responsesMatch checks if fast and primary responses are semantically similar.
func (s *SpeculativeExecutor) responsesMatch(fast, primary string) bool {
	// Simple heuristic: check if they share significant content
	// A more sophisticated version would use embeddings

	fastLower := strings.ToLower(fast)
	primaryLower := strings.ToLower(primary)

	// Extract key phrases (simple word-based comparison)
	fastWords := strings.Fields(fastLower)
	primaryWords := strings.Fields(primaryLower)

	if len(fastWords) == 0 || len(primaryWords) == 0 {
		return false
	}

	// Count shared words
	wordSet := make(map[string]bool)
	for _, w := range fastWords {
		if len(w) > 3 { // Skip short words
			wordSet[w] = true
		}
	}

	shared := 0
	for _, w := range primaryWords {
		if wordSet[w] {
			shared++
		}
	}

	// Calculate Jaccard-like similarity
	totalUnique := len(wordSet)
	for _, w := range primaryWords {
		if len(w) > 3 && !wordSet[w] {
			totalUnique++
		}
	}

	if totalUnique == 0 {
		return true
	}

	similarity := float64(shared) / float64(totalUnique)
	return similarity >= 0.3 // 30% word overlap is considered a match
}

// updateMetrics updates the executor's performance metrics.
func (s *SpeculativeExecutor) updateMetrics(fastAccepted bool, fastLatency, primaryLatency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if fastAccepted {
		s.fastAccepted++
	} else {
		s.fastRejected++
		s.primaryUsed++
	}

	// Update rolling averages
	total := s.fastAccepted + s.fastRejected
	if total > 0 {
		s.avgFastLatency = time.Duration((int64(s.avgFastLatency)*int64(total-1) + int64(fastLatency)) / int64(total))
	}

	if s.primaryUsed > 0 && primaryLatency > 0 {
		s.avgPrimaryLatency = time.Duration((int64(s.avgPrimaryLatency)*int64(s.primaryUsed-1) + int64(primaryLatency)) / int64(s.primaryUsed))
	}
}

// Metrics returns current performance metrics.
func (s *SpeculativeExecutor) Metrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := s.fastAccepted + s.fastRejected
	acceptRate := float64(0)
	if total > 0 {
		acceptRate = float64(s.fastAccepted) / float64(total)
	}

	return map[string]interface{}{
		"fast_accepted":       s.fastAccepted,
		"fast_rejected":       s.fastRejected,
		"primary_used":        s.primaryUsed,
		"fast_accept_rate":    acceptRate,
		"avg_fast_latency_ms": s.avgFastLatency.Milliseconds(),
		"avg_primary_latency_ms": s.avgPrimaryLatency.Milliseconds(),
	}
}
