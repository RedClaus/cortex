// Package cognitive provides the cognitive architecture layer for Cortex.
// It implements template-based response generation, semantic routing via embeddings,
// runtime distillation from frontier models, and a self-improving feedback loop.
package cognitive

import (
	"context"
	"encoding/binary"
	"math"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// TemplateStatus represents the lifecycle state of a template.
type TemplateStatus string

const (
	// StatusProbation indicates a newly created template that needs grading.
	StatusProbation TemplateStatus = "probation"

	// StatusValidated indicates a template that has passed initial grading.
	StatusValidated TemplateStatus = "validated"

	// StatusPromoted indicates a high-confidence template used in production.
	StatusPromoted TemplateStatus = "promoted"

	// StatusDeprecated indicates a template that failed too many times.
	StatusDeprecated TemplateStatus = "deprecated"
)

// TemplateSourceType indicates how a template was created.
type TemplateSourceType string

const (
	// SourceDistillation indicates the template was created by runtime distillation.
	SourceDistillation TemplateSourceType = "distillation"

	// SourceManual indicates the template was manually created.
	SourceManual TemplateSourceType = "manual"

	// SourceImported indicates the template was imported from external source.
	SourceImported TemplateSourceType = "imported"
)

// TaskType classifies the type of task a template handles.
type TaskType string

const (
	TaskGeneral        TaskType = "general"
	TaskCodeGen        TaskType = "code_gen"
	TaskDebug          TaskType = "debug"
	TaskReview         TaskType = "review"
	TaskPlanning       TaskType = "planning"
	TaskInfrastructure TaskType = "infrastructure"
	TaskExplain        TaskType = "explain"
	TaskRefactor       TaskType = "refactor"
)

// Template represents a reusable response template in the cognitive system.
type Template struct {
	// Identity
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// Intent matching
	Intent         string    `json:"intent"`                    // Natural language intent
	IntentEmbedding Embedding `json:"intent_embedding,omitempty"` // Vector representation
	IntentKeywords []string  `json:"intent_keywords,omitempty"`  // Fallback keywords

	// Template body (Go text/template syntax)
	TemplateBody  string `json:"template_body"`
	ExampleOutput string `json:"example_output,omitempty"`

	// Variable schema (flat JSON Schema - no nested objects)
	VariableSchema string `json:"variable_schema"`

	// GBNF Grammar (pre-computed for constrained generation)
	GBNFGrammar string `json:"gbnf_grammar,omitempty"`

	// Classification
	TaskType TaskType `json:"task_type"`
	Domain   string   `json:"domain,omitempty"`

	// Lifecycle
	Status TemplateStatus `json:"status"`

	// Quality metrics
	ConfidenceScore float64 `json:"confidence_score"`
	ComplexityScore int     `json:"complexity_score"`

	// Usage tracking
	UseCount     int `json:"use_count"`
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`

	// Source tracking
	SourceType      TemplateSourceType `json:"source_type"`
	SourceModel     string             `json:"source_model,omitempty"`
	SourceRequestID string             `json:"source_request_id,omitempty"`

	// Timestamps
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	PromotedAt   *time.Time `json:"promoted_at,omitempty"`
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
}

// SuccessRate returns the success rate as a percentage (0-100).
func (t *Template) SuccessRate() float64 {
	total := t.SuccessCount + t.FailureCount
	if total == 0 {
		return 50.0 // Default to 50% when no data
	}
	return float64(t.SuccessCount) / float64(total) * 100.0
}

// IsActive returns true if the template should be used for routing.
func (t *Template) IsActive() bool {
	return t.Status == StatusPromoted || t.Status == StatusValidated
}

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Embedding represents a vector embedding (float32 slice).
type Embedding []float32

// DefaultEmbeddingDim is the dimension for nomic-embed-text model.
const DefaultEmbeddingDim = 768

// ToBytes serializes an embedding to a byte slice for database storage.
func (e Embedding) ToBytes() []byte {
	if len(e) == 0 {
		return nil
	}
	buf := make([]byte, len(e)*4)
	for i, v := range e {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// EmbeddingFromBytes deserializes a byte slice back to an embedding.
func EmbeddingFromBytes(data []byte) Embedding {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	e := make(Embedding, len(data)/4)
	for i := range e {
		e[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return e
}

// CosineSimilarity computes the cosine similarity between two embeddings.
// Returns a value between -1 and 1, where 1 means identical direction.
func (e Embedding) CosineSimilarity(other Embedding) float64 {
	if len(e) != len(other) || len(e) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range e {
		a, b := float64(e[i]), float64(other[i])
		dotProduct += a * b
		normA += a * a
		normB += b * b
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// Normalize returns a unit-length version of the embedding.
func (e Embedding) Normalize() Embedding {
	if len(e) == 0 {
		return e
	}

	var norm float64
	for _, v := range e {
		norm += float64(v) * float64(v)
	}

	if norm == 0 {
		return e
	}

	norm = math.Sqrt(norm)
	result := make(Embedding, len(e))
	for i, v := range e {
		result[i] = float32(float64(v) / norm)
	}
	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// TEMPLATE MATCHING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// SimilarityLevel categorizes the confidence of a template match.
type SimilarityLevel string

const (
	// SimilarityHigh indicates strong match (>= 0.85).
	SimilarityHigh SimilarityLevel = "high"

	// SimilarityMedium indicates moderate match (0.70 - 0.84).
	SimilarityMedium SimilarityLevel = "medium"

	// SimilarityLow indicates weak match (0.50 - 0.69).
	SimilarityLow SimilarityLevel = "low"

	// SimilarityNoMatch indicates no viable match (< 0.50).
	SimilarityNoMatch SimilarityLevel = "no_match"
)

// Thresholds for similarity levels.
const (
	ThresholdHigh   = 0.85
	ThresholdMedium = 0.70
	ThresholdLow    = 0.50
)

// GetSimilarityLevel returns the level for a given similarity score.
func GetSimilarityLevel(score float64) SimilarityLevel {
	switch {
	case score >= ThresholdHigh:
		return SimilarityHigh
	case score >= ThresholdMedium:
		return SimilarityMedium
	case score >= ThresholdLow:
		return SimilarityLow
	default:
		return SimilarityNoMatch
	}
}

// TemplateMatch represents a matched template with its similarity score.
type TemplateMatch struct {
	Template        *Template       `json:"template"`
	SimilarityScore float64         `json:"similarity_score"`
	SimilarityLevel SimilarityLevel `json:"similarity_level"`
	MatchMethod     string          `json:"match_method"` // "embedding", "keyword", "exact"
}

// ═══════════════════════════════════════════════════════════════════════════════
// USAGE AND GRADING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// UsageLog records a single use of a template.
type UsageLog struct {
	ID         int64  `json:"id"`
	TemplateID string `json:"template_id"`
	SessionID  string `json:"session_id,omitempty"`
	RequestID  string `json:"request_id,omitempty"`

	// Input/Output
	UserInput          string `json:"user_input"`
	ExtractedVariables string `json:"extracted_variables,omitempty"` // JSON
	RenderedOutput     string `json:"rendered_output,omitempty"`

	// Matching details
	SimilarityScore float64 `json:"similarity_score,omitempty"`
	MatchMethod     string  `json:"match_method,omitempty"`

	// Outcome
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message,omitempty"`
	UserFeedback string `json:"user_feedback,omitempty"`

	// Timing (milliseconds)
	LatencyMs    int `json:"latency_ms,omitempty"`
	ExtractionMs int `json:"extraction_ms,omitempty"`
	RenderingMs  int `json:"rendering_ms,omitempty"`
	TotalMs      int `json:"total_ms,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// GradeType represents the outcome of grading a template execution.
type GradeType string

const (
	GradePass    GradeType = "pass"
	GradeFail    GradeType = "fail"
	GradePartial GradeType = "partial"
)

// GradingResult records the outcome of grading a template execution.
type GradingResult struct {
	ID         int64  `json:"id"`
	TemplateID string `json:"template_id"`
	UsageLogID *int64 `json:"usage_log_id,omitempty"`

	// Grading details
	GraderModel string    `json:"grader_model"`
	Grade       GradeType `json:"grade"`
	GradeReason string    `json:"grade_reason,omitempty"`

	// Detailed scores
	CorrectnessScore  float64 `json:"correctness_score,omitempty"`
	CompletenessScore float64 `json:"completeness_score,omitempty"`

	// Confidence adjustment
	ConfidenceDelta float64 `json:"confidence_delta,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// TemplateMetrics contains aggregated usage metrics for a template.
type TemplateMetrics struct {
	TemplateID   string  `json:"template_id"`
	UseCount     int     `json:"use_count"`
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`

	// Grading metrics
	PassCount    int `json:"pass_count"`
	FailCount    int `json:"fail_count"`
	PartialCount int `json:"partial_count"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// DISTILLATION TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// DistillationRequest tracks a request that triggered frontier model distillation.
type DistillationRequest struct {
	ID string `json:"id"`

	// Request details
	UserInput string   `json:"user_input"`
	TaskType  TaskType `json:"task_type,omitempty"`

	// Routing decision
	SimilarityScore float64 `json:"similarity_score,omitempty"`
	RouteReason     string  `json:"route_reason,omitempty"`

	// Frontier response
	FrontierModel string `json:"frontier_model"`
	Solution      string `json:"solution,omitempty"`

	// Distillation result
	TemplateCreated bool   `json:"template_created"`
	TemplateID      string `json:"template_id,omitempty"`
	ExtractionError string `json:"extraction_error,omitempty"`

	// Safety valve outcomes
	CompilationPassed bool `json:"compilation_passed"`
	SchemaValid       bool `json:"schema_valid"`
	GrammarGenerated  bool `json:"grammar_generated"`

	// Timing (milliseconds)
	FrontierMs   int `json:"frontier_ms,omitempty"`
	ExtractionMs int `json:"extraction_ms,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// DistillationResult contains the outcome of a distillation operation.
type DistillationResult struct {
	// The solution provided to the user
	Solution string `json:"solution"`

	// Extracted template (nil if extraction failed)
	Template *Template `json:"template,omitempty"`

	// Errors during extraction (nil if successful)
	ExtractionError error `json:"-"`

	// Safety valve results
	CompilationPassed bool `json:"compilation_passed"`
	SchemaValid       bool `json:"schema_valid"`
	GrammarGenerated  bool `json:"grammar_generated"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// RouteDecision indicates the outcome of semantic routing.
type RouteDecision string

const (
	// RouteTemplate indicates a template match was found.
	RouteTemplate RouteDecision = "template"

	// RouteNovel indicates no match; requires frontier model + distillation.
	RouteNovel RouteDecision = "novel"

	// RouteFallback indicates embedding unavailable; use keyword/FTS fallback.
	RouteFallback RouteDecision = "fallback"
)

// ModelTier categorizes LLM models by capability and cost.
type ModelTier string

const (
	// TierLocal indicates local models (Ollama) - fastest, cheapest.
	TierLocal ModelTier = "local"

	// TierMid indicates mid-tier cloud models (Claude Haiku, GPT-4o-mini).
	TierMid ModelTier = "mid"

	// TierAdvanced indicates advanced cloud models (Claude Sonnet, GPT-4o).
	TierAdvanced ModelTier = "advanced"

	// TierFrontier indicates frontier models (Claude Opus, o1) - slowest, most expensive.
	TierFrontier ModelTier = "frontier"
)

// RoutingResult contains the full routing decision.
type RoutingResult struct {
	Decision RouteDecision `json:"decision"`

	// Template match (if RouteTemplate)
	Match *TemplateMatch `json:"match,omitempty"`

	// Model selection
	RecommendedTier  ModelTier `json:"recommended_tier"`
	RecommendedModel string    `json:"recommended_model,omitempty"`

	// Metadata
	InputEmbedding  Embedding `json:"-"` // Computed embedding (not serialized)
	ProcessingMs    int       `json:"processing_ms,omitempty"`
	EmbeddingFailed bool      `json:"embedding_failed,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// METRICS TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// CognitiveMetrics tracks system-wide cognitive architecture performance.
type CognitiveMetrics struct {
	Date string `json:"date"` // YYYY-MM-DD

	// Request tracking
	TotalRequests   int64 `json:"total_requests"`
	TemplateHits    int64 `json:"template_hits"`
	TemplateMisses  int64 `json:"template_misses"`
	TemplateHitRate float64 `json:"template_hit_rate"`

	// Model usage
	LocalModelCalls int64   `json:"local_model_calls"`
	FrontierCalls   int64   `json:"frontier_calls"`
	LocalModelRate  float64 `json:"local_model_rate"`

	// Distillation
	DistillationAttempts  int64 `json:"distillation_attempts"`
	DistillationSuccesses int64 `json:"distillation_successes"`

	// Grading
	TotalGrades   int `json:"total_grades"`
	PassGrades    int `json:"pass_grades"`
	FailGrades    int `json:"fail_grades"`
	PartialGrades int `json:"partial_grades"`

	// Performance
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`

	// Lifecycle
	Promotions   int `json:"promotions"`
	Deprecations int `json:"deprecations"`
}

// HitRate returns the template hit rate as a percentage.
func (m *CognitiveMetrics) HitRate() float64 {
	total := int64(m.TemplateHits) + int64(m.TemplateMisses)
	if total == 0 {
		return 0
	}
	return float64(m.TemplateHits) / float64(total) * 100.0
}

// DistillationSuccessRate returns the distillation success rate as a percentage.
func (m *CognitiveMetrics) DistillationSuccessRate() float64 {
	if m.DistillationAttempts == 0 {
		return 0
	}
	return float64(m.DistillationSuccesses) / float64(m.DistillationAttempts) * 100.0
}

// SystemHealth provides an overview of cognitive system health.
type SystemHealth struct {
	// Template counts by status
	TotalTemplates      int `json:"total_templates"`
	ProbationTemplates  int `json:"probation_templates"`
	ValidatedTemplates  int `json:"validated_templates"`
	PromotedTemplates   int `json:"promoted_templates"`
	DeprecatedTemplates int `json:"deprecated_templates"`

	// Performance metrics
	AvgConfidenceScore float64 `json:"avg_confidence_score"`
	AvgSuccessRate     float64 `json:"avg_success_rate"`

	// Recent activity
	TemplatesCreatedToday  int `json:"templates_created_today"`
	TemplatesPromotedToday int `json:"templates_promoted_today"`

	// System status
	EmbeddingModelAvailable bool   `json:"embedding_model_available"`
	LastEmbeddingError      string `json:"last_embedding_error,omitempty"`

	GeneratedAt time.Time `json:"generated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// VARIABLE TYPES (for template extraction)
// ═══════════════════════════════════════════════════════════════════════════════

// VariableType defines the supported types in variable schemas.
type VariableType string

const (
	VarString  VariableType = "string"
	VarNumber  VariableType = "number"
	VarInteger VariableType = "integer"
	VarBoolean VariableType = "boolean"
	VarEnum    VariableType = "enum"
	VarArray   VariableType = "array"
)

// Variable describes a single variable in a template schema.
type Variable struct {
	Name        string       `json:"name"`
	Type        VariableType `json:"type"`
	Description string       `json:"description,omitempty"`
	Required    bool         `json:"required"`
	Default     interface{}  `json:"default,omitempty"`
	Enum        []string     `json:"enum,omitempty"` // For enum type
	Items       *Variable    `json:"items,omitempty"` // For array type (element schema)
}

// VariableSchema represents the full schema for template variables.
type VariableSchema struct {
	Variables []Variable `json:"variables"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// PROMOTION TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// PromotionReport summarizes the results of a promotion cycle.
type PromotionReport struct {
	RunAt time.Time `json:"run_at"`

	// Templates processed
	CandidatesEvaluated int `json:"candidates_evaluated"`
	PromotedCount       int `json:"promoted_count"`
	DeprecatedCount     int `json:"deprecated_count"`

	// Details
	PromotedIDs   []string `json:"promoted_ids,omitempty"`
	DeprecatedIDs []string `json:"deprecated_ids,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// SHARED LLM INTERFACES
// ═══════════════════════════════════════════════════════════════════════════════

// ChatMessage represents a message for LLM chat completion.
// This is the canonical message type used across cognitive subpackages.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

// SimpleChatProvider defines a minimal interface for chat completion.
// Use this for packages that need simple message-in, string-out semantics.
// For full provider features (Name, Available, etc.), use llm.Provider.
type SimpleChatProvider interface {
	Chat(ctx context.Context, messages []ChatMessage, systemPrompt string) (string, error)
}
