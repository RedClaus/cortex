package brain

import (
	"context"
	"sync"

	"github.com/normanking/cortex/internal/cognitive/router"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/brain/lobes"
)

func init() {
	// Ensure log is initialized (may be called before adapter.go init)
	if log == nil {
		log = logging.Global()
	}
}

type FactoryConfig struct {
	LLMProvider  llm.Provider
	MemorySystem MemorySystem
	UserID       string
	Embedder     router.Embedder
}

func NewExecutive(cfg FactoryConfig) *brain.Executive {
	log.Info("[Brain] Creating Executive with userID=%s", cfg.UserID)

	llmAdapter := NewLLMAdapter(cfg.LLMProvider)
	log.Debug("[Brain] LLMAdapter created")

	var memoryAdapter lobes.MemoryStore
	if cfg.MemorySystem != nil {
		memoryAdapter = NewMemoryAdapter(cfg.MemorySystem, cfg.UserID)
		log.Debug("[Brain] MemoryAdapter created")
	} else {
		log.Debug("[Brain] MemoryAdapter skipped (no memory system)")
	}

	classifierLLM := &ClassifierLLMAdapter{provider: cfg.LLMProvider}

	var embedder brain.Embedder
	if cfg.Embedder != nil {
		embedder = NewEmbedderAdapter(cfg.Embedder)
		log.Debug("[Brain] EmbedderAdapter created")
	}

	exec := brain.NewExecutive(brain.ExecutiveConfig{
		Embedder:  embedder,
		LLMClient: classifierLLM,
		Cache:     NewSimpleCache(),
	})

	registerLobes(exec, llmAdapter, memoryAdapter)
	log.Info("[Brain] Executive created with lobes registered")

	return exec
}

func registerLobes(exec *brain.Executive, llmAdapter lobes.LLMProvider, memAdapter lobes.MemoryStore) {
	lobeCount := 0

	exec.RegisterLobe(lobes.NewCodingLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewReasoningLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewPlanningLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewCreativityLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewLogicLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewCausalLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewTemporalLobe(llmAdapter))
	exec.RegisterLobe(lobes.NewSpatialLobe(llmAdapter))
	lobeCount += 8

	if memAdapter != nil {
		exec.RegisterLobe(lobes.NewMemoryLobe(memAdapter))
		lobeCount++
	}

	exec.RegisterLobe(lobes.NewAttentionLobe())
	exec.RegisterLobe(lobes.NewInhibitionLobe())
	exec.RegisterLobe(lobes.NewMetacognitionLobe())
	exec.RegisterLobe(lobes.NewSafetyLobe())
	exec.RegisterLobe(lobes.NewSelfKnowledgeLobe())
	exec.RegisterLobe(lobes.NewTextParsingLobe())
	lobeCount += 6

	log.Info("[Brain] Registered %d lobes (memory=%v)", lobeCount, memAdapter != nil)
}

type ClassifierLLMAdapter struct {
	provider llm.Provider
}

func (a *ClassifierLLMAdapter) Classify(ctx context.Context, input string, candidates []brain.LobeID) (*brain.ClassificationResult, error) {
	log.Debug("[Brain] Classifier: classifying input length=%d candidates=%d", len(input), len(candidates))

	if a.provider == nil {
		log.Debug("[Brain] Classifier: no provider, using default classification")
		return defaultClassification(candidates), nil
	}

	candidateList := ""
	for i, c := range candidates {
		if i > 0 {
			candidateList += ", "
		}
		candidateList += string(c)
	}

	req := &llm.ChatRequest{
		SystemPrompt: "You are a request classifier. Given user input and a list of available processing modules (lobes), identify the most appropriate lobe to handle the request. Respond with ONLY the lobe name, nothing else.",
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: "Available lobes: " + candidateList + "\n\nClassify this request: " + input,
			},
		},
		Temperature: 0.0,
	}

	resp, err := a.provider.Chat(ctx, req)
	if err != nil {
		log.Warn("[Brain] Classifier: LLM classification failed, using default: %v", err)
		return defaultClassification(candidates), nil
	}

	lobeID := brain.LobeID(resp.Content)
	for _, c := range candidates {
		if c == lobeID {
			log.Info("[Brain] Classifier: classified as lobe=%s confidence=0.8 method=llm", lobeID)
			return &brain.ClassificationResult{
				PrimaryLobe:    lobeID,
				SecondaryLobes: []brain.LobeID{},
				RiskLevel:      brain.RiskLow,
				Confidence:     0.8,
				Method:         "llm",
			}, nil
		}
	}

	log.Debug("[Brain] Classifier: LLM response %q not in candidates, using default", resp.Content)
	return defaultClassification(candidates), nil
}

func defaultClassification(candidates []brain.LobeID) *brain.ClassificationResult {
	primary := brain.LobeReasoning
	if len(candidates) > 0 {
		primary = candidates[0]
	}
	return &brain.ClassificationResult{
		PrimaryLobe:    primary,
		SecondaryLobes: []brain.LobeID{},
		RiskLevel:      brain.RiskLow,
		Confidence:     0.5,
		Method:         "default",
	}
}

type SimpleCache struct {
	mu    sync.RWMutex
	items map[string]*brain.ClassificationResult
}

func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		items: make(map[string]*brain.ClassificationResult),
	}
}

func (c *SimpleCache) Get(key string) (*brain.ClassificationResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.items[key]
	return result, ok
}

func (c *SimpleCache) Set(key string, result *brain.ClassificationResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = result
}
