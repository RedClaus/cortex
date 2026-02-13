package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONVERSATION LOGGER INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// ConversationLogger provides logging of LLM interactions.
type ConversationLogger interface {
	// LogRequest logs the start of an LLM interaction.
	// Returns a unique request ID for correlation.
	LogRequest(ctx context.Context, req *LogRequest) (requestID string, err error)

	// LogResponse logs the completion of an LLM interaction.
	LogResponse(ctx context.Context, requestID string, resp *LogResponse) error

	// LogError logs an error that occurred during LLM interaction.
	LogError(ctx context.Context, requestID string, errCode, errMsg string) error

	// GetLog retrieves a conversation log by request ID.
	GetLog(ctx context.Context, requestID string) (*ConversationLog, error)

	// ListLogs retrieves recent conversation logs.
	ListLogs(ctx context.Context, limit int) ([]*ConversationLog, error)

	// ExportToJSON exports a conversation log to a JSON file.
	// Returns the filepath of the exported file.
	ExportToJSON(ctx context.Context, requestID string) (filepath string, err error)
}

// ═══════════════════════════════════════════════════════════════════════════════
// STORE INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// LogStore defines the storage operations needed by the logger.
// This is implemented by data.Store.
type LogStore interface {
	CreateConversationLog(ctx context.Context, log *ConversationLog) error
	UpdateConversationLog(ctx context.Context, log *ConversationLog) error
	GetConversationLog(ctx context.Context, requestID string) (*ConversationLog, error)
	ListConversationLogs(ctx context.Context, limit int) ([]*ConversationLog, error)
	UpdateModelMetrics(ctx context.Context, log *ConversationLog) error
}

// ═══════════════════════════════════════════════════════════════════════════════
// SQLITE LOGGER IMPLEMENTATION
// ═══════════════════════════════════════════════════════════════════════════════

// SQLiteLogger implements ConversationLogger using SQLite storage.
type SQLiteLogger struct {
	store   LogStore
	dataDir string // Base directory for JSON exports (~/.cortex)
}

// NewSQLiteLogger creates a new SQLite-backed conversation logger.
func NewSQLiteLogger(store LogStore, dataDir string) *SQLiteLogger {
	return &SQLiteLogger{
		store:   store,
		dataDir: dataDir,
	}
}

// LogRequest logs the start of an LLM interaction.
func (l *SQLiteLogger) LogRequest(ctx context.Context, req *LogRequest) (string, error) {
	// Generate unique request ID
	requestID := generateRequestID()

	// Classify model tier
	tier := ClassifyModelTier(req.Provider, req.Model)

	log := &ConversationLog{
		RequestID:       requestID,
		SessionID:       req.SessionID,
		ParentRequestID: req.ParentRequestID,
		Provider:        req.Provider,
		Model:           req.Model,
		ModelTier:       tier.String(),
		Prompt:          req.Prompt,
		SystemPrompt:    req.SystemPrompt,
		TaskType:        req.TaskType,
		ComplexityScore: req.ComplexityScore,
		CreatedAt:       time.Now(),
	}

	if err := l.store.CreateConversationLog(ctx, log); err != nil {
		return "", fmt.Errorf("create conversation log: %w", err)
	}

	return requestID, nil
}

// LogResponse logs the completion of an LLM interaction.
func (l *SQLiteLogger) LogResponse(ctx context.Context, requestID string, resp *LogResponse) error {
	// Get existing log to update
	log, err := l.store.GetConversationLog(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get conversation log: %w", err)
	}

	// Update with response data
	log.Response = resp.Response
	log.ContextTokens = resp.ContextTokens
	log.CompletionTokens = resp.CompletionTokens
	log.TotalTokens = resp.ContextTokens + resp.CompletionTokens
	log.DurationMs = resp.DurationMs
	log.Success = resp.Success

	if !resp.Success {
		log.ErrorCode = resp.ErrorCode
		log.ErrorMessage = resp.ErrorMessage
	}

	if err := l.store.UpdateConversationLog(ctx, log); err != nil {
		return fmt.Errorf("update conversation log: %w", err)
	}

	// Update aggregated metrics
	if err := l.store.UpdateModelMetrics(ctx, log); err != nil {
		// Log but don't fail - metrics are secondary
		fmt.Fprintf(os.Stderr, "Warning: failed to update model metrics: %v\n", err)
	}

	return nil
}

// LogError logs an error that occurred during LLM interaction.
func (l *SQLiteLogger) LogError(ctx context.Context, requestID string, errCode, errMsg string) error {
	log, err := l.store.GetConversationLog(ctx, requestID)
	if err != nil {
		return fmt.Errorf("get conversation log: %w", err)
	}

	log.Success = false
	log.ErrorCode = errCode
	log.ErrorMessage = errMsg

	if err := l.store.UpdateConversationLog(ctx, log); err != nil {
		return fmt.Errorf("update conversation log: %w", err)
	}

	return nil
}

// GetLog retrieves a conversation log by request ID.
func (l *SQLiteLogger) GetLog(ctx context.Context, requestID string) (*ConversationLog, error) {
	return l.store.GetConversationLog(ctx, requestID)
}

// ListLogs retrieves recent conversation logs.
func (l *SQLiteLogger) ListLogs(ctx context.Context, limit int) ([]*ConversationLog, error) {
	return l.store.ListConversationLogs(ctx, limit)
}

// ExportToJSON exports a conversation log to a JSON file.
func (l *SQLiteLogger) ExportToJSON(ctx context.Context, requestID string) (string, error) {
	log, err := l.store.GetConversationLog(ctx, requestID)
	if err != nil {
		return "", fmt.Errorf("get conversation log: %w", err)
	}

	// Create directory structure: ~/.cortex/logs/conversations/YYYY-MM-DD/
	dateDir := log.CreatedAt.Format("2006-01-02")
	exportDir := filepath.Join(l.dataDir, "logs", "conversations", dateDir)

	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", fmt.Errorf("create export directory: %w", err)
	}

	// Create JSON file
	exportPath := filepath.Join(exportDir, fmt.Sprintf("%s.json", requestID))

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal conversation log: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0644); err != nil {
		return "", fmt.Errorf("write JSON file: %w", err)
	}

	return exportPath, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// generateRequestID creates a unique request identifier.
// Format: req-{uuid} for easy identification in logs.
func generateRequestID() string {
	return fmt.Sprintf("req-%s", uuid.New().String()[:8])
}

// ═══════════════════════════════════════════════════════════════════════════════
// NO-OP LOGGER (for testing/disabled mode)
// ═══════════════════════════════════════════════════════════════════════════════

// NoOpLogger is a logger that does nothing (for testing or when logging is disabled).
type NoOpLogger struct{}

// NewNoOpLogger creates a new no-op logger.
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}

func (l *NoOpLogger) LogRequest(ctx context.Context, req *LogRequest) (string, error) {
	return generateRequestID(), nil
}

func (l *NoOpLogger) LogResponse(ctx context.Context, requestID string, resp *LogResponse) error {
	return nil
}

func (l *NoOpLogger) LogError(ctx context.Context, requestID string, errCode, errMsg string) error {
	return nil
}

func (l *NoOpLogger) GetLog(ctx context.Context, requestID string) (*ConversationLog, error) {
	return nil, fmt.Errorf("no-op logger: logs not available")
}

func (l *NoOpLogger) ListLogs(ctx context.Context, limit int) ([]*ConversationLog, error) {
	return nil, nil
}

func (l *NoOpLogger) ExportToJSON(ctx context.Context, requestID string) (string, error) {
	return "", fmt.Errorf("no-op logger: export not available")
}
