package brain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ClassificationResult holds the output of intent classification.
type ClassificationResult struct {
	PrimaryLobe    LobeID    `json:"primary_lobe"`
	SecondaryLobes []LobeID  `json:"secondary_lobes"`
	RiskLevel      RiskLevel `json:"risk_level"`
	Confidence     float64   `json:"confidence"`
	Method         string    `json:"method"` // "regex", "embedding", "llm"
}

// ExecutiveClassifier routes inputs to the appropriate lobes.
type ExecutiveClassifier struct {
	patterns  map[LobeID][]*regexp.Regexp
	embedder  Embedder  // interface for embedding provider
	llmClient LLMClient // interface for LLM fallback
	cache     ClassificationCache
}

// Embedder provides embedding vectors for similarity matching.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

// LLMClient provides LLM inference for complex classification.
type LLMClient interface {
	Classify(ctx context.Context, input string, candidates []LobeID) (*ClassificationResult, error)
}

// ClassificationCache caches classification results.
type ClassificationCache interface {
	Get(key string) (*ClassificationResult, bool)
	Set(key string, result *ClassificationResult)
}

// NewExecutiveClassifier creates a classifier with default patterns.
func NewExecutiveClassifier(embedder Embedder, llm LLMClient, cache ClassificationCache) *ExecutiveClassifier {
	c := &ExecutiveClassifier{
		patterns:  make(map[LobeID][]*regexp.Regexp),
		embedder:  embedder,
		llmClient: llm,
		cache:     cache,
	}
	c.initDefaultPatterns()
	return c
}

// Classify determines which lobe(s) should handle the input.
func (c *ExecutiveClassifier) Classify(ctx context.Context, input string) (*ClassificationResult, error) {
	// Tier 1: Fast regex patterns (instant, zero LLM cost)
	if res, ok := c.classifyByRegex(input); ok {
		return res, nil
	}

	// Check cache before expensive operations
	cacheKey := strings.TrimSpace(input)
	if c.cache != nil {
		if res, ok := c.cache.Get(cacheKey); ok {
			return res, nil
		}
	}

	// Tier 2: Embedding similarity (fast, cached)
	if res, ok := c.classifyByEmbedding(ctx, input); ok {
		c.cache.Set(cacheKey, res)
		return res, nil
	}

	// Tier 3: LLM classification (expensive, last resort)
	res, err := c.classifyByLLM(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("llm classification failed: %w", err)
	}

	c.cache.Set(cacheKey, res)
	return res, nil
}

// classifyByRegex tries tier 1 regex matching.
func (c *ExecutiveClassifier) classifyByRegex(input string) (*ClassificationResult, bool) {
	inputLower := strings.ToLower(input)

	for lobe, patterns := range c.patterns {
		for _, p := range patterns {
			if p.MatchString(inputLower) {
				return &ClassificationResult{
					PrimaryLobe:    lobe,
					SecondaryLobes: []LobeID{}, // Could be populated if we matched multiple lobes
					RiskLevel:      RiskLow,    // Regex matches are usually safe/known commands
					Confidence:     1.0,
					Method:         "regex",
				}, true
			}
		}
	}
	return nil, false
}

// classifyByEmbedding tries tier 2 embedding similarity.
func (c *ExecutiveClassifier) classifyByEmbedding(ctx context.Context, input string) (*ClassificationResult, bool) {
	if c.embedder == nil {
		return nil, false
	}

	_, err := c.embedder.Embed(ctx, input)
	if err != nil {
		// Log error? For now, just fall through to LLM
		return nil, false
	}

	// TODO: Compare vector with Lobe centroids.
	// Since we don't have the lobe centroids defined in this context,
	// we fall through to the LLM. In a full implementation, we would
	// compute cosine similarity against stored vectors for each LobeID.

	return nil, false
}

// classifyByLLM uses tier 3 LLM classification.
func (c *ExecutiveClassifier) classifyByLLM(ctx context.Context, input string) (*ClassificationResult, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("no llm client available")
	}

	// We pass all valid lobes as candidates
	candidates := AllLobes()
	return c.llmClient.Classify(ctx, input, candidates)
}

// initDefaultPatterns sets up regex patterns for each lobe.
func (c *ExecutiveClassifier) initDefaultPatterns() {
	// Helper to add patterns
	add := func(lobe LobeID, patterns ...string) {
		for _, p := range patterns {
			// case-insensitive compilation is handled by checking lowercased input or (?i)
			// Using (?i) for safety if input isn't lowercased
			re := regexp.MustCompile("(?i)" + p)
			c.patterns[lobe] = append(c.patterns[lobe], re)
		}
	}

	// LobeCoding
	add(LobeCoding, "write code", "implement", "fix bug", "refactor", "\\.go$", "\\.py$", "\\.js$", "\\.ts$", "function", "class", "struct")

	// LobeMemory
	add(LobeMemory, "remember", "recall", "what did I say", "history", "search memory", "find in docs")

	// LobePlanning
	add(LobePlanning, "plan", "break down", "steps to", "how should I", "roadmap", "strategy")

	// LobeSafety
	add(LobeSafety, "delete", "rm -rf", "sudo", "password", "credential", "secret", "key")

	// LobeReasoning
	add(LobeReasoning, "why", "explain", "analyze", "compare", "evaluate", "reason")

	// LobeCreativity
	add(LobeCreativity, "brainstorm", "ideas", "creative", "generate", "imagine", "story")
}
