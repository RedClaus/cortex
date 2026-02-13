package cognitive

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Lane represents the processing lane for a request.
type Lane string

const (
	// FastLane uses local models for quick responses (~200ms).
	// CRITICAL: No thinking step allowed on Fast Lane.
	FastLane Lane = "fast"

	// SmartLane uses frontier models for complex queries (~2-4s).
	// Thinking step is allowed and encouraged for complex queries.
	SmartLane Lane = "smart"
)

// LLMProvider is an interface for LLM completions.
// This allows the pipeline to work with any LLM backend.
type LLMProvider interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest is an LLM completion request.
type CompletionRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Model       string // Optional model override
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// CompletionResponse is an LLM completion response.
type CompletionResponse struct {
	Content    string
	TokensUsed int
	Model      string
}

// Pipeline handles lane-gated cognitive processing.
type Pipeline struct {
	fastLLM     LLMProvider // Ollama/local
	smartLLM    LLMProvider // Claude/GPT-4
	modeTracker *ModeTracker
	config      PipelineConfig
}

// PipelineConfig defines pipeline behavior.
type PipelineConfig struct {
	// Thinking configuration
	EnableThinking      bool
	MaxThinkingTokens   int
	ThinkingTemperature float64

	// Timeout configuration
	FastLaneTimeout  time.Duration
	SmartLaneTimeout time.Duration

	// Complexity heuristics for auto-routing
	ComplexityKeywords []string
	MinMessageLength   int // Messages longer than this may warrant thinking

	// Model configuration
	FastModel  string // e.g., "llama3.2:1b"
	SmartModel string // e.g., "claude-sonnet-4-20250514"
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() PipelineConfig {
	return PipelineConfig{
		EnableThinking:      true,
		MaxThinkingTokens:   1000,
		ThinkingTemperature: 0.3,
		FastLaneTimeout:     500 * time.Millisecond,
		SmartLaneTimeout:    30 * time.Second,
		ComplexityKeywords: []string{
			"debug", "error", "why", "how should", "best approach",
			"architecture", "design", "compare", "tradeoff", "explain",
			"what's wrong", "doesn't work", "help me understand",
			"analyze", "review", "evaluate",
		},
		MinMessageLength: 500,
		FastModel:        "llama3.2:1b",
		SmartModel:       "claude-sonnet-4-20250514",
	}
}

// PipelineRequest is the input to the pipeline.
type PipelineRequest struct {
	SystemPrompt   string
	Message        string
	History        []Message
	ConversationID string

	// Optional overrides
	ForceLane     *Lane // Override router decision
	ForceThinking *bool // Override thinking decision
}

// PipelineResponse is the output from the pipeline.
type PipelineResponse struct {
	Content      string `json:"content"`
	Lane         Lane   `json:"lane"`
	Model        string `json:"model"`
	ThinkingUsed bool   `json:"thinking_used"`
	Thinking     string `json:"thinking,omitempty"` // The internal monologue (debug only)
	LatencyMs    int64  `json:"latency_ms"`
	TokensUsed   int    `json:"tokens_used"`
}

// NewPipeline creates a new cognitive pipeline.
func NewPipeline(
	fastLLM, smartLLM LLMProvider,
	modeTracker *ModeTracker,
	config PipelineConfig,
) *Pipeline {
	return &Pipeline{
		fastLLM:     fastLLM,
		smartLLM:    smartLLM,
		modeTracker: modeTracker,
		config:      config,
	}
}

// Process handles a request through the appropriate lane.
func (p *Pipeline) Process(ctx context.Context, req *PipelineRequest) (*PipelineResponse, error) {
	start := time.Now()

	// 1. Determine lane (from override or auto-routing)
	lane := p.determineLane(req)

	// 2. Set appropriate timeout
	var cancel context.CancelFunc
	if lane == FastLane {
		ctx, cancel = context.WithTimeout(ctx, p.config.FastLaneTimeout)
	} else {
		ctx, cancel = context.WithTimeout(ctx, p.config.SmartLaneTimeout)
	}
	defer cancel()

	// 3. Decide if thinking should be used
	useThinking := p.shouldUseThinking(req, lane)

	// 4. Execute appropriate path
	var response *PipelineResponse
	var err error

	if useThinking {
		response, err = p.processWithThinking(ctx, req, lane)
	} else {
		response, err = p.processDirect(ctx, req, lane)
	}

	if err != nil {
		return nil, err
	}

	response.LatencyMs = time.Since(start).Milliseconds()
	return response, nil
}

// determineLane decides which lane to use based on message complexity.
func (p *Pipeline) determineLane(req *PipelineRequest) Lane {
	if req.ForceLane != nil {
		return *req.ForceLane
	}

	// Auto-routing based on message content
	msgLower := strings.ToLower(req.Message)
	wordCount := len(strings.Fields(req.Message))

	// Check complexity keywords FIRST (fix for the routing bug)
	for _, keyword := range p.config.ComplexityKeywords {
		if strings.Contains(msgLower, keyword) {
			return SmartLane
		}
	}

	// Then check length thresholds
	if wordCount > 50 {
		return SmartLane
	}

	// Short, simple queries go to fast lane
	return FastLane
}

// shouldUseThinking decides if internal monologue adds value.
// CRITICAL: Returns false for Fast Lane regardless of other factors.
func (p *Pipeline) shouldUseThinking(req *PipelineRequest, lane Lane) bool {
	// HARD RULE: No thinking on Fast Lane
	if lane == FastLane {
		return false
	}

	// Check if thinking is enabled globally
	if !p.config.EnableThinking {
		return false
	}

	// Check for override
	if req.ForceThinking != nil {
		return *req.ForceThinking
	}

	// Heuristics for when thinking helps
	msgLower := strings.ToLower(req.Message)

	// Check complexity keywords
	for _, keyword := range p.config.ComplexityKeywords {
		if strings.Contains(msgLower, keyword) {
			return true
		}
	}

	// Long messages may warrant thinking
	if len(req.Message) > p.config.MinMessageLength {
		return true
	}

	return false
}

// processDirect is the Fast Lane path - no cognitive overhead.
func (p *Pipeline) processDirect(
	ctx context.Context,
	req *PipelineRequest,
	lane Lane,
) (*PipelineResponse, error) {
	provider := p.selectProvider(lane)

	messages := make([]Message, 0, len(req.History)+2)
	messages = append(messages, Message{Role: "system", Content: req.SystemPrompt})
	messages = append(messages, req.History...)
	messages = append(messages, Message{Role: "user", Content: req.Message})

	model := p.config.FastModel
	if lane == SmartLane {
		model = p.config.SmartModel
	}

	response, err := provider.Complete(ctx, &CompletionRequest{
		Messages:    messages,
		MaxTokens:   2000,
		Temperature: 0.7,
		Model:       model,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	return &PipelineResponse{
		Content:      response.Content,
		Lane:         lane,
		Model:        response.Model,
		ThinkingUsed: false,
		TokensUsed:   response.TokensUsed,
	}, nil
}

// processWithThinking is the Smart Lane path - includes internal monologue.
func (p *Pipeline) processWithThinking(
	ctx context.Context,
	req *PipelineRequest,
	lane Lane,
) (*PipelineResponse, error) {
	provider := p.selectProvider(lane)

	// Step 1: Internal Monologue (thinking)
	thinkingPrompt := p.buildThinkingPrompt(req)

	thinkingResp, err := provider.Complete(ctx, &CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: req.SystemPrompt},
			{Role: "user", Content: thinkingPrompt},
		},
		MaxTokens:   p.config.MaxThinkingTokens,
		Temperature: p.config.ThinkingTemperature,
		Model:       p.config.SmartModel,
	})
	if err != nil {
		// Fallback to direct if thinking fails
		return p.processDirect(ctx, req, lane)
	}

	// Step 2: Generate response informed by thinking
	responsePrompt := p.buildResponsePrompt(req, thinkingResp.Content)

	messages := make([]Message, 0, len(req.History)+2)
	messages = append(messages, Message{Role: "system", Content: req.SystemPrompt})
	messages = append(messages, req.History...)
	messages = append(messages, Message{Role: "user", Content: responsePrompt})

	response, err := provider.Complete(ctx, &CompletionRequest{
		Messages:    messages,
		MaxTokens:   2000,
		Temperature: 0.7,
		Model:       p.config.SmartModel,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM response generation failed: %w", err)
	}

	return &PipelineResponse{
		Content:      response.Content,
		Lane:         lane,
		Model:        response.Model,
		ThinkingUsed: true,
		Thinking:     thinkingResp.Content,
		TokensUsed:   thinkingResp.TokensUsed + response.TokensUsed,
	}, nil
}

func (p *Pipeline) buildThinkingPrompt(req *PipelineRequest) string {
	return fmt.Sprintf(`Before responding to the user, analyze the request step by step:

1. What is the user actually asking for?
2. What context from the conversation is relevant?
3. What are the key technical considerations?
4. What's the best approach to answer this?

User's message: %s

Think through this carefully (your analysis will inform your response but won't be shown to the user):`, req.Message)
}

func (p *Pipeline) buildResponsePrompt(req *PipelineRequest, thinking string) string {
	return fmt.Sprintf(`Your internal analysis (not shown to user):
<thinking>
%s
</thinking>

Now respond to the user concisely and helpfully based on your analysis.

User: %s`, thinking, req.Message)
}

func (p *Pipeline) selectProvider(lane Lane) LLMProvider {
	if lane == FastLane {
		return p.fastLLM
	}
	return p.smartLLM
}

// BuildSystemPrompt constructs the complete system prompt with mode augmentation.
func BuildSystemPrompt(systemPrompt string, mode *BehavioralMode) string {
	if mode == nil || mode.PromptAugment == "" {
		return systemPrompt
	}

	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n")
	sb.WriteString(mode.PromptAugment)
	return sb.String()
}

// BehavioralMode is imported from facets package but redefined here
// to avoid circular imports. In production, use facets.BehavioralMode.
type BehavioralMode struct {
	ID            string
	Name          string
	PromptAugment string
}
