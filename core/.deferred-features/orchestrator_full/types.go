// Package orchestrator provides the central coordination layer for Cortex.
// It wires together the router, tools, knowledge fabric, and platform detection
// to process user requests through a unified pipeline.
package orchestrator

import (
	"context"
	"time"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/fingerprint"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/router"
	"github.com/normanking/cortex/internal/tools"
	"github.com/normanking/cortex/pkg/types"
)

// RequestType identifies the kind of user request.
type RequestType string

const (
	RequestChat    RequestType = "chat"    // Conversational request
	RequestCommand RequestType = "command" // Direct tool execution
	RequestQuery   RequestType = "query"   // Knowledge lookup
)

// Request represents an incoming user request.
type Request struct {
	// ID uniquely identifies this request.
	ID string `json:"id"`

	// Type categorizes the request.
	Type RequestType `json:"type"`

	// Input is the user's message or command.
	Input string `json:"input"`

	// SessionID groups related requests.
	SessionID string `json:"session_id,omitempty"`

	// Context provides additional information.
	Context *RequestContext `json:"context,omitempty"`

	// Timestamp when the request was received.
	Timestamp time.Time `json:"timestamp"`
}

// RequestContext provides additional context for processing.
type RequestContext struct {
	// WorkingDir is the current directory.
	WorkingDir string `json:"working_dir,omitempty"`

	// Fingerprint is the detected platform info.
	Fingerprint *fingerprint.Fingerprint `json:"fingerprint,omitempty"`

	// PreviousMessages for conversation context (deprecated: use History).
	PreviousMessages []Message `json:"previous_messages,omitempty"`

	// History contains the conversation history for LLM context.
	History []Message `json:"history,omitempty"`

	// ActiveFile being edited (if any).
	ActiveFile string `json:"active_file,omitempty"`

	// Tags for filtering knowledge.
	Tags []string `json:"tags,omitempty"`

	// ModelOverride allows specifying a different model for this request.
	ModelOverride string `json:"model_override,omitempty"`

	// ProviderOverride allows specifying a different provider for this request.
	// This is used when auto-switching between local (ollama) and cloud (anthropic, openai) models.
	ProviderOverride string `json:"provider_override,omitempty"`

	// OnAgentStep is called for each step during agentic execution.
	// This enables streaming output to show reasoning in real-time.
	OnAgentStep agent.StepCallback `json:"-"`

	// UnrestrictedMode disables safety-related restrictions in the system prompt.
	// When enabled, the LLM will attempt to help with requests it might otherwise refuse.
	UnrestrictedMode bool `json:"unrestricted_mode,omitempty"`

	// VoiceMode indicates the request came from voice input.
	// When enabled, responses should be concise and conversational,
	// suitable for text-to-speech output. The system should provide
	// only the final answer, not intermediate steps or verbose explanations.
	VoiceMode bool `json:"voice_mode,omitempty"`

	// AgentSystemPrompt is an optional system prompt from a Voice Agent.
	// When set, this prompt is prepended to the regular system prompt,
	// allowing Voice Agents to customize the LLM's behavior/persona.
	// (CR-016 Phase 3)
	AgentSystemPrompt string `json:"agent_system_prompt,omitempty"`
}

// Message represents a conversation message.
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Response represents the orchestrator's response.
type Response struct {
	// ID matches the request ID.
	RequestID string `json:"request_id"`

	// Success indicates if processing succeeded.
	Success bool `json:"success"`

	// Content is the main response text.
	Content string `json:"content"`

	// ToolResults contains results from tool executions.
	ToolResults []*tools.ToolResult `json:"tool_results,omitempty"`

	// KnowledgeUsed lists knowledge items that informed the response.
	KnowledgeUsed []*types.KnowledgeItem `json:"knowledge_used,omitempty"`

	// Routing shows how the request was classified.
	Routing *router.RoutingDecision `json:"routing,omitempty"`

	// Error contains error details if Success is false.
	Error string `json:"error,omitempty"`

	// Metadata contains additional response data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Duration of the full processing pipeline.
	Duration time.Duration `json:"duration"`

	// Timestamp when the response was generated.
	Timestamp time.Time `json:"timestamp"`

	// TokenMetrics tracks inference token usage.
	TokenMetrics *TokenMetrics `json:"token_metrics,omitempty"`
}

// TokenMetrics tracks token usage for inference.
type TokenMetrics struct {
	// LocalTokens is tokens used by local inference (MLX, Ollama).
	LocalTokens int `json:"local_tokens"`

	// LocalProvider is the local provider name (e.g., "mlx", "ollama").
	LocalProvider string `json:"local_provider,omitempty"`

	// LocalModel is the local model used.
	LocalModel string `json:"local_model,omitempty"`

	// ExternalTokens is tokens used by external/cloud inference.
	ExternalTokens int `json:"external_tokens"`

	// ExternalProvider is the external provider name (e.g., "anthropic", "openai").
	ExternalProvider string `json:"external_provider,omitempty"`

	// ExternalModel is the external model used.
	ExternalModel string `json:"external_model,omitempty"`

	// TotalTokens is the sum of all tokens used.
	TotalTokens int `json:"total_tokens"`
}

// Pipeline defines the processing stages.
type Pipeline interface {
	// Process handles a request through the full pipeline.
	Process(ctx context.Context, req *Request) (*Response, error)
}

// Stage represents a single processing stage.
type Stage interface {
	// Name returns the stage identifier.
	Name() string

	// Execute runs this stage.
	Execute(ctx context.Context, state *PipelineState) error
}

// PipelineState carries data between stages.
type PipelineState struct {
	// Request being processed.
	Request *Request

	// Response being built.
	Response *Response

	// Routing decision from classifier.
	Routing *router.RoutingDecision

	// Knowledge retrieved for context.
	Knowledge []*types.KnowledgeItem

	// StrategicPrinciples retrieved from strategic memory.
	StrategicPrinciples []memory.StrategicMemory

	// Cognitive result from template matching/distillation.
	Cognitive *CognitiveResult

	// ToolRequests to be executed.
	ToolRequests []*tools.ToolRequest

	// ToolResults from executions.
	ToolResults []*tools.ToolResult

	// LLMPrompt built for the AI.
	LLMPrompt string

	// LLMResponse from the AI.
	LLMResponse string

	// LLMTokensUsed tracks token consumption.
	LLMTokensUsed int

	// LLMProvider is the provider used for inference.
	LLMProvider string

	// LLMModel is the model used for inference.
	LLMModel string

	// Errors accumulated during processing.
	Errors []error

	// Metrics for each stage.
	StageMetrics map[string]time.Duration

	// Cancelled indicates if processing was cancelled.
	Cancelled bool

	// RAPIDDecision from confidence evaluation (CR-026: RAPID Framework).
	RAPIDDecision *RAPIDDecision

	// IntrospectionResult from metacognitive processing (CR-018).
	IntrospectionResult *IntrospectionResult
}

// RAPIDDecision captures the RAPID framework confidence evaluation.
// CR-026: Reduce AI Prompt Iteration Depth
type RAPIDDecision struct {
	// ShouldProceed indicates if we have enough confidence to proceed.
	ShouldProceed bool `json:"should_proceed"`

	// ClarificationNeeded indicates we should ask the user for more info.
	ClarificationNeeded bool `json:"clarification_needed"`

	// ClarificationQuestion is the compound question to ask (if needed).
	ClarificationQuestion string `json:"clarification_question,omitempty"`

	// ConfidenceScore is the routing confidence (0-1).
	ConfidenceScore float64 `json:"confidence_score"`

	// Assumptions lists what we're assuming if proceeding with low confidence.
	Assumptions []string `json:"assumptions,omitempty"`

	// Level indicates which RAPID level made the decision (1-5).
	Level int `json:"level"`
}

// CognitiveResult contains the outcome of cognitive processing.
type CognitiveResult struct {
	// TemplateUsed indicates if a template was matched and used.
	TemplateUsed bool `json:"template_used"`

	// Template that was matched (if any).
	Template *cognitive.Template `json:"template,omitempty"`

	// TemplateMatch contains similarity details.
	TemplateMatch *cognitive.TemplateMatch `json:"template_match,omitempty"`

	// RenderedOutput from template (if template was used).
	RenderedOutput string `json:"rendered_output,omitempty"`

	// ExtractedVariables from user input.
	ExtractedVariables map[string]interface{} `json:"extracted_variables,omitempty"`

	// DistillationTriggered indicates if distillation was triggered.
	DistillationTriggered bool `json:"distillation_triggered"`

	// DistillationResult contains the distillation outcome.
	DistillationResult *cognitive.DistillationResult `json:"distillation_result,omitempty"`

	// ComplexityScore of the request.
	ComplexityScore int `json:"complexity_score,omitempty"`

	// NeedsDecomposition indicates if task decomposition is needed.
	NeedsDecomposition bool `json:"needs_decomposition"`

	// ModelTier recommended for this request.
	ModelTier string `json:"model_tier,omitempty"`
}

// NewPipelineState creates a new state for a request.
func NewPipelineState(req *Request) *PipelineState {
	return &PipelineState{
		Request:      req,
		Response:     &Response{RequestID: req.ID, Timestamp: time.Now()},
		StageMetrics: make(map[string]time.Duration),
	}
}

// AddError records an error during processing.
func (s *PipelineState) AddError(err error) {
	if err != nil {
		s.Errors = append(s.Errors, err)
	}
}

// HasErrors returns true if any errors occurred.
func (s *PipelineState) HasErrors() bool {
	return len(s.Errors) > 0
}

// Specialist defines an AI specialist for a task type.
type Specialist struct {
	// TaskType this specialist handles.
	TaskType router.TaskType `json:"task_type"`

	// Name is a human-readable identifier.
	Name string `json:"name"`

	// SystemPrompt for the AI.
	SystemPrompt string `json:"system_prompt"`

	// Tools available to this specialist.
	Tools []tools.ToolType `json:"tools"`

	// KnowledgeTags for filtering relevant knowledge.
	KnowledgeTags []string `json:"knowledge_tags,omitempty"`
}

// DefaultSpecialists returns the built-in specialists.
func DefaultSpecialists() map[router.TaskType]*Specialist {
	return map[router.TaskType]*Specialist{
		router.TaskGeneral: {
			TaskType:     router.TaskGeneral,
			Name:         "General Assistant",
			SystemPrompt: "You are a helpful AI assistant.",
			Tools:        []tools.ToolType{tools.ToolRead, tools.ToolGlob, tools.ToolGrep},
		},
		router.TaskCodeGen: {
			TaskType:     router.TaskCodeGen,
			Name:         "Code Generator",
			SystemPrompt: "You are an expert programmer. Write clean, efficient, well-documented code.",
			Tools:        []tools.ToolType{tools.ToolRead, tools.ToolWrite, tools.ToolEdit, tools.ToolGlob, tools.ToolGrep, tools.ToolBash},
		},
		router.TaskDebug: {
			TaskType:      router.TaskDebug,
			Name:          "Debugger",
			SystemPrompt:  "You are an expert debugger. Analyze errors systematically and provide clear fixes.",
			Tools:         []tools.ToolType{tools.ToolRead, tools.ToolEdit, tools.ToolBash, tools.ToolGrep},
			KnowledgeTags: []string{"debugging", "errors", "troubleshooting"},
		},
		router.TaskReview: {
			TaskType:      router.TaskReview,
			Name:          "Code Reviewer",
			SystemPrompt:  "You are an expert code reviewer. Identify issues, suggest improvements, and ensure best practices.",
			Tools:         []tools.ToolType{tools.ToolRead, tools.ToolGlob, tools.ToolGrep},
			KnowledgeTags: []string{"review", "best-practices", "standards"},
		},
		router.TaskPlanning: {
			TaskType:      router.TaskPlanning,
			Name:          "Architect",
			SystemPrompt:  "You are a software architect. Design systems, plan implementations, and consider trade-offs.",
			Tools:         []tools.ToolType{tools.ToolRead, tools.ToolGlob, tools.ToolGrep},
			KnowledgeTags: []string{"architecture", "design", "patterns"},
		},
		router.TaskInfrastructure: {
			TaskType:      router.TaskInfrastructure,
			Name:          "DevOps Engineer",
			SystemPrompt:  "You are a DevOps expert. Configure infrastructure, deploy services, and manage systems safely.",
			Tools:         []tools.ToolType{tools.ToolBash, tools.ToolRead, tools.ToolWrite, tools.ToolEdit},
			KnowledgeTags: []string{"infrastructure", "devops", "deployment"},
		},
		router.TaskExplain: {
			TaskType:     router.TaskExplain,
			Name:         "Explainer",
			SystemPrompt: "You are a teacher. Explain concepts clearly with examples and analogies.",
			Tools:        []tools.ToolType{tools.ToolRead, tools.ToolGlob, tools.ToolGrep},
		},
		router.TaskRefactor: {
			TaskType:      router.TaskRefactor,
			Name:          "Refactorer",
			SystemPrompt:  "You are a refactoring expert. Improve code structure while preserving behavior.",
			Tools:         []tools.ToolType{tools.ToolRead, tools.ToolEdit, tools.ToolGlob, tools.ToolGrep, tools.ToolBash},
			KnowledgeTags: []string{"refactoring", "patterns", "clean-code"},
		},
	}
}

// LLMProvider defines the interface for AI providers.
// Uses shared types from pkg/types to avoid import cycles.
type LLMProvider interface {
	Chat(ctx context.Context, req *types.LLMRequest) (*types.LLMResponse, error)
	Name() string
	Available() bool
}

// LLMRequest is an alias for types.LLMRequest for backward compatibility.
type LLMRequest = types.LLMRequest

// LLMResponse is an alias for types.LLMResponse for backward compatibility.
type LLMResponse = types.LLMResponse

// OrchestratorStats tracks processing metrics.
type OrchestratorStats struct {
	TotalRequests     int64                     `json:"total_requests"`
	SuccessCount      int64                     `json:"success_count"`
	FailureCount      int64                     `json:"failure_count"`
	AvgDuration       time.Duration             `json:"avg_duration"`
	TotalToolCalls    int64                     `json:"total_tool_calls"`
	TotalLLMCalls     int64                     `json:"total_llm_calls"`
	KnowledgeHits     int64                     `json:"knowledge_hits"`
	RouteDistribution map[router.TaskType]int64 `json:"route_distribution"`
}
