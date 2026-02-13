// Package types defines shared types used across all Cortex modules.
package types

import "time"

// ═══════════════════════════════════════════════════════════════════════════════
// TOKEN ESTIMATION
// ═══════════════════════════════════════════════════════════════════════════════

// CharsPerToken is the heuristic for token estimation (~4 chars per token).
// This is a common approximation for English text with LLM tokenizers.
const CharsPerToken = 4

// EstimateTokens provides a rough token estimate for a given text.
func EstimateTokens(text string) int {
	return len(text) / CharsPerToken
}

// ═══════════════════════════════════════════════════════════════════════════════
// KNOWLEDGE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Scope defines the visibility/ownership tier of a knowledge item.
type Scope string

const (
	ScopeGlobal   Scope = "global"   // Read-only, admin-pushed (company policies)
	ScopeTeam     Scope = "team"     // Shared, trust-weighted merge
	ScopePersonal Scope = "personal" // Private, local-wins on conflict
)

// KnowledgeType defines the category of knowledge.
type KnowledgeType string

const (
	TypeSOP      KnowledgeType = "sop"      // Standard Operating Procedure
	TypeLesson   KnowledgeType = "lesson"   // Learned lesson (what to do/avoid)
	TypePattern  KnowledgeType = "pattern"  // Code/config pattern
	TypeSession  KnowledgeType = "session"  // Recorded session
	TypeDocument KnowledgeType = "document" // General documentation
)

// KnowledgeItem represents a piece of stored knowledge.
type KnowledgeItem struct {
	ID      string        `json:"id"`
	Type    KnowledgeType `json:"type"`
	Title   string        `json:"title,omitempty"`
	Content string        `json:"content"`
	Tags    []string      `json:"tags,omitempty"`

	// Attribution
	Scope      Scope  `json:"scope"`
	TeamID     string `json:"team_id,omitempty"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"author_name,omitempty"`

	// Quality signals
	Confidence   float64 `json:"confidence"`  // 0.0 - 1.0
	TrustScore   float64 `json:"trust_score"` // 0.0 - 1.0
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
	AccessCount  int     `json:"access_count"`

	// Sync metadata
	Version      int       `json:"version"`
	RemoteID     string    `json:"remote_id,omitempty"`
	SyncStatus   string    `json:"sync_status"` // pending, synced, conflict, local_only
	LastSyncedAt time.Time `json:"last_synced_at,omitempty"`

	// Temporal
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastAccessedAt *time.Time `json:"last_accessed_at,omitempty"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"` // Soft delete
}

// ═══════════════════════════════════════════════════════════════════════════════
// RETRIEVAL TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// RetrievalTier indicates which retrieval strategy was used.
type RetrievalTier int

const (
	TierStrict   RetrievalTier = iota + 1 // Exact match, high confidence
	TierFuzzy                             // FTS + trust-weighted
	TierFallback                          // No strong match, use LLM
)

func (t RetrievalTier) String() string {
	switch t {
	case TierStrict:
		return "Strict Match"
	case TierFuzzy:
		return "Fuzzy Match"
	case TierFallback:
		return "Fallback (AI)"
	default:
		return "Unknown"
	}
}

// RetrievalResult contains the results of a knowledge search.
type RetrievalResult struct {
	Items       []*KnowledgeItem `json:"items"`
	Tier        RetrievalTier    `json:"tier"`
	Confidence  float64          `json:"confidence"`
	Explanation string           `json:"explanation"`
}

// SearchOptions configures knowledge retrieval.
type SearchOptions struct {
	Tiers     []Scope  // Which tiers to search (default: all)
	Types     []string // Filter by knowledge type
	Tags      []string // Filter by tags
	ProjectID string   // Filter by project
	Limit     int      // Max results (default: 10)
	MinTrust  float64  // Minimum trust score (default: 0.0)
}

// ═══════════════════════════════════════════════════════════════════════════════
// TRUST TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// TrustProfile tracks a user's reliability in a specific domain.
type TrustProfile struct {
	ID           int       `json:"id"`
	UserID       string    `json:"user_id"`
	Domain       string    `json:"domain"` // "cisco", "linux", "python", etc.
	Score        float64   `json:"score"`  // 0.0 - 1.0
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	LastActivity time.Time `json:"last_activity"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// SESSION TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Session represents an interactive conversation session.
type Session struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	Title           string     `json:"title,omitempty"`
	CWD             string     `json:"cwd"`
	PlatformVendor  string     `json:"platform_vendor,omitempty"` // "cisco", "linux"
	PlatformName    string     `json:"platform_name,omitempty"`   // "ios-xe", "ubuntu"
	PlatformVersion string     `json:"platform_version,omitempty"`
	Status          string     `json:"status"` // active, completed, abandoned
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	LastActivityAt  time.Time  `json:"last_activity_at"`
}

// SessionMessage represents a single message in a session.
type SessionMessage struct {
	ID          int       `json:"id"`
	SessionID   string    `json:"session_id"`
	Role        string    `json:"role"` // user, assistant, system, tool
	Content     string    `json:"content"`
	ToolName    string    `json:"tool_name,omitempty"`
	ToolInput   string    `json:"tool_input,omitempty"` // JSON
	ToolOutput  string    `json:"tool_output,omitempty"`
	ToolSuccess *bool     `json:"tool_success,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// FINGERPRINT TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Fingerprint captures the detected environment context.
type Fingerprint struct {
	ProjectType     string            `json:"project_type"`     // node, python, go, rust
	PackageManager  string            `json:"package_manager"`  // npm, yarn, pnpm, pip
	RuntimeVersions map[string]string `json:"runtime_versions"` // node: 20.0.0
	GitContext      *GitContext       `json:"git_context,omitempty"`
	ConfigFiles     []string          `json:"config_files"` // tsconfig.json, etc.
	OS              string            `json:"os"`           // darwin, linux, windows
	Shell           string            `json:"shell"`        // bash, zsh, fish
	DetectedAt      time.Time         `json:"detected_at"`
}

// GitContext contains git repository information.
type GitContext struct {
	Branch    string `json:"branch"`
	RemoteURL string `json:"remote_url,omitempty"`
	IsDirty   bool   `json:"is_dirty"`
	Ahead     int    `json:"ahead"`
	Behind    int    `json:"behind"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Classification represents the result of intent classification.
type Classification struct {
	Category   string   `json:"category"`   // installation, build, debug, etc.
	Confidence float64  `json:"confidence"` // 0.0 - 1.0
	Method     string   `json:"method"`     // "fast" (regex) or "slow" (LLM)
	Signals    []string `json:"signals"`    // What triggered this classification
	Gaps       []Gap    `json:"gaps"`       // Missing context
}

// Gap represents missing information needed for a complete response.
type Gap struct {
	Question string `json:"question"`
	Reason   string `json:"reason"`
	Priority int    `json:"priority"` // 1 = critical, 2 = important, 3 = nice-to-have
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// ToolCall represents a request to execute a tool.
type ToolCall struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}

// ToolResult represents the outcome of a tool execution.
type ToolResult struct {
	CallID   string `json:"call_id"`
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Duration int64  `json:"duration_ms"`
}

// RiskLevel indicates the danger level of a command.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// PreflightResult contains the safety assessment of a command.
type PreflightResult struct {
	Safe             bool      `json:"safe"`
	RiskLevel        RiskLevel `json:"risk_level"`
	Warnings         []string  `json:"warnings,omitempty"`
	RequiresApproval bool      `json:"requires_approval"`
	Suggestions      []string  `json:"suggestions,omitempty"` // Safer alternatives
}

// ═══════════════════════════════════════════════════════════════════════════════
// SYNC TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// SyncResult summarizes a synchronization operation.
type SyncResult struct {
	Pulled    int           `json:"pulled"`
	Pushed    int           `json:"pushed"`
	Conflicts int           `json:"conflicts"`
	Resolved  int           `json:"resolved"`
	Errors    []string      `json:"errors,omitempty"`
	Duration  time.Duration `json:"duration"`
}

// MergeResult represents the outcome of a conflict resolution.
type MergeResult struct {
	Winner     *KnowledgeItem `json:"winner"`
	Resolution string         `json:"resolution"` // local_wins, remote_wins, merged, manual
	Reason     string         `json:"reason"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// LLM TYPES (shared to avoid import cycles)
// ═══════════════════════════════════════════════════════════════════════════════

// LLMMessage represents a conversation message for LLM requests.
type LLMMessage struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// LLMRequest represents a chat request to an AI provider.
type LLMRequest struct {
	// Model to use.
	Model string `json:"model"`

	// SystemPrompt sets the AI's behavior.
	SystemPrompt string `json:"system_prompt"`

	// Messages in the conversation.
	Messages []LLMMessage `json:"messages"`

	// MaxTokens limits response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness.
	Temperature float64 `json:"temperature,omitempty"`
}

// LLMResponse contains the AI's response.
type LLMResponse struct {
	// Content is the response text.
	Content string `json:"content"`

	// Model that generated the response.
	Model string `json:"model"`

	// TokensUsed in the request.
	TokensUsed int `json:"tokens_used,omitempty"`

	// Duration of the API call.
	Duration time.Duration `json:"duration"`
}
