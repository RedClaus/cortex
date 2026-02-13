// Package server provides the Prism HTTP server for the Cortex control plane.
// Prism is a React SPA embedded in the Go binary using go:embed, accessible via `cortex ui`.
package server

import (
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// SERVER CONFIGURATION
// ═══════════════════════════════════════════════════════════════════════════════

// Config holds Prism server configuration.
type Config struct {
	// Port is the preferred port (default: 7890)
	Port int

	// DevMode enables CORS for local Vite development
	DevMode bool

	// DevOrigin is the Vite dev server origin (default: http://localhost:5173)
	DevOrigin string

	// OpenBrowser automatically opens the browser on startup
	OpenBrowser bool

	// ShutdownTimeout is the graceful shutdown timeout (default: 5s)
	ShutdownTimeout time.Duration
}

// DefaultConfig returns sensible defaults for the Prism server.
func DefaultConfig() *Config {
	return &Config{
		Port:            7890,
		DevMode:         false,
		DevOrigin:       "http://localhost:5173",
		OpenBrowser:     true,
		ShutdownTimeout: 5 * time.Second,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// API RESPONSE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// StatusResponse is returned by GET /api/v1/status.
type StatusResponse struct {
	Version     string    `json:"version"`
	Uptime      string    `json:"uptime"`
	StartedAt   time.Time `json:"started_at"`
	Port        int       `json:"port"`
	DevMode     bool      `json:"dev_mode"`
	TUIActive   bool      `json:"tui_active"`
	ActiveFacet string    `json:"active_facet,omitempty"`

	// Provider status
	Providers ProviderStatus `json:"providers"`

	// Voice bridge status
	Voice VoiceStatus `json:"voice"`

	// System metrics
	Metrics SystemMetrics `json:"metrics"`
}

// VoiceStatus indicates voice bridge connection status.
type VoiceStatus struct {
	Enabled          bool   `json:"enabled"`
	Connected        bool   `json:"connected"`
	OrchestratorURL  string `json:"orchestrator_url,omitempty"`
}

// ProviderStatus indicates which LLM providers are configured.
type ProviderStatus struct {
	Ollama    bool   `json:"ollama"`
	OllamaURL string `json:"ollama_url,omitempty"`
	Anthropic bool   `json:"anthropic"`
	OpenAI    bool   `json:"openai"`
	Gemini    bool   `json:"gemini"`
}

// SystemMetrics contains basic system metrics.
type SystemMetrics struct {
	TemplateCount int     `json:"template_count"`
	TemplateHits  int64   `json:"template_hits"`
	LocalRate     float64 `json:"local_rate"` // Percentage using local models
}

// ═══════════════════════════════════════════════════════════════════════════════
// SSE EVENT TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// SSEEventType identifies server-sent event types.
type SSEEventType string

const (
	// EventStatus indicates a status update.
	EventStatus SSEEventType = "status"

	// EventMetrics indicates metrics update.
	EventMetrics SSEEventType = "metrics"

	// EventTemplate indicates template change (create/update/delete).
	EventTemplate SSEEventType = "template"

	// EventFacet indicates active facet change.
	EventFacet SSEEventType = "facet"

	// EventError indicates an error event.
	EventError SSEEventType = "error"
)

// SSEEvent is a server-sent event payload.
type SSEEvent struct {
	Type      SSEEventType `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	Data      interface{}  `json:"data"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// API ERROR TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// APIError represents a structured API error response.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return e.Message
}

// Common API errors.
var (
	ErrNotFound      = &APIError{Code: 404, Message: "not found"}
	ErrBadRequest    = &APIError{Code: 400, Message: "bad request"}
	ErrInternal      = &APIError{Code: 500, Message: "internal server error"}
	ErrUnauthorized  = &APIError{Code: 401, Message: "unauthorized"}
)

// ═══════════════════════════════════════════════════════════════════════════════
// FACET TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Facet represents a persona/configuration preset.
type Facet struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	SystemPrompt string    `json:"system_prompt,omitempty"`
	Icon         string    `json:"icon,omitempty"`
	Color        string    `json:"color,omitempty"`
	Active       bool      `json:"active"`
	IsBuiltIn    bool      `json:"is_built_in"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// FacetsResponse is returned by GET /api/v1/facets.
type FacetsResponse struct {
	Facets      []Facet `json:"facets"`
	ActiveFacet string  `json:"active_facet,omitempty"`
}

// FacetActivateRequest is the request body for POST /api/v1/facets/:id/activate.
type FacetActivateRequest struct {
	ID string `json:"id"`
}

// CreateFacetRequest is the request body for POST /api/v1/facets.
type CreateFacetRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Icon         string `json:"icon,omitempty"`
	Color        string `json:"color,omitempty"`
}

// UpdateFacetRequest is the request body for PUT /api/v1/facets/:id.
type UpdateFacetRequest struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	Icon         string `json:"icon,omitempty"`
	Color        string `json:"color,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// KnowledgeDocument represents an ingested document in the knowledge base.
type KnowledgeDocument struct {
	ID         string                 `json:"id"`
	Title      string                 `json:"title"`
	Type       string                 `json:"type"` // file, directory, conversation, text
	SourcePath string                 `json:"source_path,omitempty"`
	ChunkCount int                    `json:"chunk_count"`
	SizeBytes  int64                  `json:"size_bytes"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentsResponse is returned by GET /api/v1/knowledge/documents.
type DocumentsResponse struct {
	Documents  []KnowledgeDocument `json:"documents"`
	TotalCount int                 `json:"total_count"`
}

// SearchResult represents a single search result from RAG search.
type SearchResult struct {
	DocumentID    string                 `json:"document_id"`
	DocumentTitle string                 `json:"document_title"`
	ChunkText     string                 `json:"chunk_text"`
	Score         float64                `json:"similarity_score"`
	ChunkIndex    int                    `json:"chunk_index"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// SearchRequest is the request body for POST /api/v1/knowledge/search.
type SearchRequest struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`     // Default: 10
	MinScore float64 `json:"min_score,omitempty"` // Default: 0.7
}

// SearchResponse is returned by POST /api/v1/knowledge/search.
type SearchResponse struct {
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	TotalCount   int            `json:"total_count"`
	SearchTimeMs int64          `json:"search_time_ms"`
}

// IngestRequest is the request body for POST /api/v1/knowledge/ingest.
type IngestRequest struct {
	Type     string                 `json:"type"` // file, directory, text
	Path     string                 `json:"path,omitempty"`
	Content  string                 `json:"content,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IngestResponse is returned by POST /api/v1/knowledge/ingest.
type IngestResponse struct {
	DocumentID string `json:"document_id"`
	Title      string `json:"title"`
	ChunkCount int    `json:"chunk_count"`
	Message    string `json:"message"`
}

// KnowledgeStats contains knowledge base statistics.
type KnowledgeStats struct {
	TotalDocuments int       `json:"total_documents"`
	TotalChunks    int       `json:"total_chunks"`
	TotalSizeBytes int64     `json:"total_size_bytes"`
	LastUpdated    time.Time `json:"last_updated,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIG TYPES (For Prism API - mirrors internal/config/config.go)
// ═══════════════════════════════════════════════════════════════════════════════

// LLMProviderConfig contains configuration for a specific LLM provider.
type LLMProviderConfig struct {
	Endpoint string `json:"endpoint,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Model    string `json:"model,omitempty"`
}

// LLMConfig contains configuration for Language Model providers.
type LLMConfig struct {
	DefaultProvider string                       `json:"default_provider"`
	Providers       map[string]LLMProviderConfig `json:"providers"`
}

// KnowledgeConfigAPI contains knowledge system configuration.
type KnowledgeConfigAPI struct {
	DBPath         string `json:"db_path"`
	DefaultTier    string `json:"default_tier"`
	TrustDecayDays int    `json:"trust_decay_days"`
}

// SyncConfigAPI contains sync configuration.
type SyncConfigAPI struct {
	Enabled   bool   `json:"enabled"`
	Endpoint  string `json:"endpoint"`
	Interval  string `json:"interval"`
	AuthToken string `json:"auth_token,omitempty"`
}

// TUIConfigAPI contains TUI configuration.
type TUIConfigAPI struct {
	Theme        string `json:"theme"`
	VimMode      bool   `json:"vim_mode"`
	SidebarWidth int    `json:"sidebar_width"`
}

// LoggingConfigAPI contains logging configuration.
type LoggingConfigAPI struct {
	Level string `json:"level"`
	File  string `json:"file"`
}

// CognitiveConfigAPI contains cognitive architecture configuration.
type CognitiveConfigAPI struct {
	Enabled                   bool    `json:"enabled"`
	OllamaURL                 string  `json:"ollama_url"`
	EmbeddingModel            string  `json:"embedding_model"`
	FrontierModel             string  `json:"frontier_model"`
	SimilarityThresholdHigh   float64 `json:"similarity_threshold_high"`
	SimilarityThresholdMedium float64 `json:"similarity_threshold_medium"`
	SimilarityThresholdLow    float64 `json:"similarity_threshold_low"`
	ComplexityThreshold       int     `json:"complexity_threshold"`
}

// VoiceConfigAPI contains voice configuration.
type VoiceConfigAPI struct {
	Enabled    bool    `json:"enabled"`
	STTEnabled bool    `json:"stt_enabled"`
	TTSEnabled bool    `json:"tts_enabled"`
	TTSVoice   string  `json:"tts_voice,omitempty"`
	TTSRate    float64 `json:"tts_rate"`
	TTSPitch   float64 `json:"tts_pitch"`
	Language   string  `json:"language"`
}

// ConfigResponse is returned by GET /api/v1/config and POST /api/v1/config.
type ConfigResponse struct {
	LLM       LLMConfig          `json:"llm"`
	Knowledge KnowledgeConfigAPI `json:"knowledge"`
	Sync      SyncConfigAPI      `json:"sync"`
	TUI       TUIConfigAPI       `json:"tui"`
	Logging   LoggingConfigAPI   `json:"logging"`
	Cognitive CognitiveConfigAPI `json:"cognitive"`
	Voice     *VoiceConfigAPI    `json:"voice,omitempty"`
}

// ProviderKeyRequest is the request body for POST /api/v1/providers/:id/key.
type ProviderKeyRequest struct {
	APIKey string `json:"api_key"`
}

// ProviderStatusResponse is returned by GET /api/v1/providers/:id/status.
type ProviderStatusResponseAPI struct {
	Online bool     `json:"online"`
	Models []string `json:"models,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CHAT API TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	ID          string           `json:"id"`
	Role        string           `json:"role"` // "user", "assistant", "system"
	Content     string           `json:"content"`
	Timestamp   time.Time        `json:"timestamp"`
	Model       string           `json:"model,omitempty"`
	PersonaID   string           `json:"persona_id,omitempty"`
	Attachments []ChatAttachment `json:"attachments,omitempty"`
	Routing     *ChatRouting     `json:"routing,omitempty"`
}

// ChatAttachment represents a file attached to a message.
type ChatAttachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // MIME type
	Size int64  `json:"size"`
	URL  string `json:"url,omitempty"`
}

// ChatRouting contains model routing decision info.
type ChatRouting struct {
	Lane      string `json:"lane"`   // "fast" or "smart"
	Model     string `json:"model"`  // Actual model used
	Reason    string `json:"reason"` // Why this model was chosen
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// Conversation represents a chat session.
type Conversation struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
	PersonaID string        `json:"persona_id,omitempty"`
}

// ConversationsResponse is returned by GET /api/v1/chat/conversations.
type ConversationsResponse struct {
	Conversations []Conversation `json:"conversations"`
	Total         int            `json:"total"`
}

// ChatRequest is the request body for POST /api/v1/chat.
type ChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id,omitempty"`
	PersonaID      string `json:"persona_id,omitempty"`
	Lane           string `json:"lane,omitempty"` // "fast", "smart", "auto"
	Stream         bool   `json:"stream,omitempty"`
}

// ChatResponse is returned by POST /api/v1/chat (non-streaming).
type ChatResponse struct {
	Message        ChatMessage  `json:"message"`
	ConversationID string       `json:"conversation_id"`
	Routing        ChatRouting  `json:"routing"`
	ModeInfo       *ModeInfo    `json:"mode_info,omitempty"`
}

// ChatStreamChunk represents a chunk in a streaming response.
type ChatStreamChunk struct {
	Type           string       `json:"type"` // "start", "delta", "end", "error"
	Content        string       `json:"content,omitempty"`
	MessageID      string       `json:"message_id,omitempty"`
	ConversationID string       `json:"conversation_id,omitempty"`
	Routing        *ChatRouting `json:"routing,omitempty"`
	Error          string       `json:"error,omitempty"`
	Usage          *TokenUsage  `json:"usage,omitempty"`
}

// TokenUsage represents token usage statistics.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODE INFO TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ModeInfo is returned with chat responses to show current behavioral mode.
type ModeInfo struct {
	CurrentMode     string               `json:"current_mode"`
	ModeName        string               `json:"mode_name"`
	ModeDescription string               `json:"mode_description,omitempty"`
	PromptAugment   string               `json:"prompt_augment,omitempty"`
	Transition      *ModeTransitionInfo  `json:"transition,omitempty"`
}

// ModeTransitionInfo describes a mode transition that just occurred.
type ModeTransitionInfo struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Trigger     string `json:"trigger"`
	TriggerType string `json:"trigger_type"` // "keyword", "manual", "exit", "reset"
}

// SetModeRequest is the request body for POST /api/v1/chat/conversations/:id/mode.
type SetModeRequest struct {
	ModeID string `json:"mode_id"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA API TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// CreatePersonaRequest is the request body for POST /api/v1/personas.
type CreatePersonaRequest struct {
	Name                string                 `json:"name"`
	Role                string                 `json:"role"`
	Background          string                 `json:"background,omitempty"`
	Traits              []string               `json:"traits,omitempty"`
	Values              []string               `json:"values,omitempty"`
	Expertise           []ExpertiseDomain      `json:"expertise,omitempty"`
	Style               CommunicationStyle     `json:"style,omitempty"`
	Modes               []BehavioralMode       `json:"modes,omitempty"`
	DefaultMode         string                 `json:"default_mode,omitempty"`
	KnowledgeSourceIDs  []string               `json:"knowledge_source_ids,omitempty"`
}

// UpdatePersonaRequest is the request body for PUT /api/v1/personas/:id.
type UpdatePersonaRequest struct {
	Name                string                 `json:"name,omitempty"`
	Role                string                 `json:"role,omitempty"`
	Background          string                 `json:"background,omitempty"`
	Traits              []string               `json:"traits,omitempty"`
	Values              []string               `json:"values,omitempty"`
	Expertise           []ExpertiseDomain      `json:"expertise,omitempty"`
	Style               *CommunicationStyle    `json:"style,omitempty"`
	Modes               []BehavioralMode       `json:"modes,omitempty"`
	DefaultMode         string                 `json:"default_mode,omitempty"`
	KnowledgeSourceIDs  []string               `json:"knowledge_source_ids,omitempty"`
}

// PersonaResponse is returned by persona endpoints.
type PersonaResponse struct {
	ID                  string                 `json:"id"`
	Version             string                 `json:"version"`
	Name                string                 `json:"name"`
	Role                string                 `json:"role"`
	Background          string                 `json:"background,omitempty"`
	Traits              []string               `json:"traits,omitempty"`
	Values              []string               `json:"values,omitempty"`
	Expertise           []ExpertiseDomain      `json:"expertise,omitempty"`
	Style               CommunicationStyle     `json:"style,omitempty"`
	Modes               []BehavioralMode       `json:"modes,omitempty"`
	DefaultMode         string                 `json:"default_mode,omitempty"`
	KnowledgeSourceIDs  []string               `json:"knowledge_source_ids,omitempty"`
	SystemPrompt        string                 `json:"system_prompt,omitempty"`
	CompiledPrompt      string                 `json:"compiled_prompt,omitempty"`
	IsBuiltIn           bool                   `json:"is_built_in"`
	CreatedAt           string                 `json:"created_at"`
	UpdatedAt           string                 `json:"updated_at"`
}

// ExpertiseDomain defines a knowledge area with depth level.
type ExpertiseDomain struct {
	Domain      string   `json:"domain"`
	Depth       string   `json:"depth"`
	Specialties []string `json:"specialties,omitempty"`
	Boundaries  []string `json:"boundaries,omitempty"`
}

// CommunicationStyle defines how the persona communicates.
type CommunicationStyle struct {
	Tone       string   `json:"tone"`
	Verbosity  string   `json:"verbosity"`
	Formatting string   `json:"formatting"`
	Patterns   []string `json:"patterns,omitempty"`
	Avoids     []string `json:"avoids,omitempty"`
}

// BehavioralMode defines a behavioral state with transition triggers.
type BehavioralMode struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	PromptAugment string   `json:"prompt_augment,omitempty"`
	EntryKeywords []string `json:"entry_keywords,omitempty"`
	ExitKeywords  []string `json:"exit_keywords,omitempty"`
	ManualTrigger string   `json:"manual_trigger,omitempty"`
	ForceVerbose  bool     `json:"force_verbose,omitempty"`
	ForceConcise  bool     `json:"force_concise,omitempty"`
	SortOrder     int      `json:"sort_order,omitempty"`
}

// PersonasResponse is returned by GET /api/v1/personas.
type PersonasResponse struct {
	Personas []PersonaResponse `json:"personas"`
	Total    int               `json:"total"`
}

// CompilePromptResponse is returned by POST /api/v1/personas/:id/compile.
type CompilePromptResponse struct {
	SystemPrompt string `json:"system_prompt"`
	UpdatedAt    string `json:"updated_at"`
}

// PreviewPromptResponse is returned by POST /api/v1/personas/:id/preview.
type PreviewPromptResponse struct {
	SystemPrompt string `json:"system_prompt"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODE API TYPES (CR-011)
// ═══════════════════════════════════════════════════════════════════════════════

// ModeResponse is returned by mode endpoints.
type ModeResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ModesResponse is returned by GET /api/v1/modes.
type ModesResponse struct {
	Modes []ModeResponse `json:"modes"`
	Total int            `json:"total"`
}
