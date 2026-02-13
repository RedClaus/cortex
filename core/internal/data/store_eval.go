package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/normanking/cortex/internal/eval"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CONVERSATION LOG OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateConversationLog inserts a new conversation log.
func (s *Store) CreateConversationLog(ctx context.Context, log *eval.ConversationLog) error {
	query := `
		INSERT INTO conversation_logs (
			request_id, session_id, parent_request_id,
			provider, model, model_tier,
			prompt, system_prompt, context_tokens,
			task_type, complexity_score,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		log.RequestID,
		nullString(log.SessionID),
		nullString(log.ParentRequestID),
		log.Provider,
		log.Model,
		nullString(log.ModelTier),
		log.Prompt,
		nullString(log.SystemPrompt),
		log.ContextTokens,
		nullString(log.TaskType),
		log.ComplexityScore,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("insert conversation log: %w", err)
	}

	return nil
}

// UpdateConversationLog updates an existing conversation log with response data.
func (s *Store) UpdateConversationLog(ctx context.Context, log *eval.ConversationLog) error {
	query := `
		UPDATE conversation_logs SET
			response = ?,
			completion_tokens = ?,
			total_tokens = ?,
			duration_ms = ?,
			time_to_first_token_ms = ?,
			success = ?,
			error_code = ?,
			error_message = ?,
			had_timeout = ?,
			had_repetition = ?,
			had_tool_failure = ?,
			had_truncation = ?,
			had_json_error = ?
		WHERE request_id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		nullString(log.Response),
		log.CompletionTokens,
		log.TotalTokens,
		log.DurationMs,
		log.TimeToFirstTokenMs,
		boolToInt(log.Success),
		nullString(log.ErrorCode),
		nullString(log.ErrorMessage),
		boolToInt(log.HadTimeout),
		boolToInt(log.HadRepetition),
		boolToInt(log.HadToolFailure),
		boolToInt(log.HadTruncation),
		boolToInt(log.HadJSONError),
		log.RequestID,
	)

	if err != nil {
		return fmt.Errorf("update conversation log: %w", err)
	}

	return nil
}

// UpdateConversationAssessment updates a conversation log with assessment results.
func (s *Store) UpdateConversationAssessment(ctx context.Context, requestID string, assessment *eval.Assessment) error {
	now := time.Now()

	query := `
		UPDATE conversation_logs SET
			capability_score = ?,
			recommended_upgrade = ?,
			assessment_reason = ?,
			assessed_at = ?
		WHERE request_id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		assessment.CapabilityScore,
		nullString(assessment.RecommendedUpgrade),
		nullString(assessment.UpgradeReason),
		now,
		requestID,
	)

	if err != nil {
		return fmt.Errorf("update conversation assessment: %w", err)
	}

	return nil
}

// GetConversationLog retrieves a conversation log by request ID.
func (s *Store) GetConversationLog(ctx context.Context, requestID string) (*eval.ConversationLog, error) {
	query := `
		SELECT
			id, request_id, session_id, parent_request_id,
			provider, model, model_tier,
			prompt, system_prompt, context_tokens,
			response, completion_tokens, total_tokens,
			duration_ms, time_to_first_token_ms,
			task_type, complexity_score,
			success, error_code, error_message,
			had_timeout, had_repetition, had_tool_failure, had_truncation, had_json_error,
			capability_score, recommended_upgrade, assessment_reason,
			created_at, assessed_at
		FROM conversation_logs
		WHERE request_id = ?
	`

	var log eval.ConversationLog
	var sessionID, parentRequestID, modelTier, systemPrompt sql.NullString
	var response, taskType, errorCode, errorMessage sql.NullString
	var recommendedUpgrade, assessmentReason sql.NullString
	var capabilityScore sql.NullFloat64
	var assessedAt sql.NullTime
	var success, hadTimeout, hadRepetition, hadToolFailure, hadTruncation, hadJSONError int
	// These columns can be NULL when the log is created but not yet updated with response data
	var completionTokens, totalTokens, durationMs, timeToFirstTokenMs, contextTokens, complexityScore sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, requestID).Scan(
		&log.ID, &log.RequestID, &sessionID, &parentRequestID,
		&log.Provider, &log.Model, &modelTier,
		&log.Prompt, &systemPrompt, &contextTokens,
		&response, &completionTokens, &totalTokens,
		&durationMs, &timeToFirstTokenMs,
		&taskType, &complexityScore,
		&success, &errorCode, &errorMessage,
		&hadTimeout, &hadRepetition, &hadToolFailure, &hadTruncation, &hadJSONError,
		&capabilityScore, &recommendedUpgrade, &assessmentReason,
		&log.CreatedAt, &assessedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("conversation log not found: %s", requestID)
		}
		return nil, fmt.Errorf("get conversation log: %w", err)
	}

	// Map nullable string fields
	log.SessionID = sessionID.String
	log.ParentRequestID = parentRequestID.String
	log.ModelTier = modelTier.String
	log.SystemPrompt = systemPrompt.String
	log.Response = response.String
	log.TaskType = taskType.String
	log.ErrorCode = errorCode.String
	log.ErrorMessage = errorMessage.String
	log.RecommendedUpgrade = recommendedUpgrade.String
	log.AssessmentReason = assessmentReason.String

	// Map nullable int fields (these are NULL until UpdateConversationLog is called)
	if completionTokens.Valid {
		log.CompletionTokens = int(completionTokens.Int64)
	}
	if totalTokens.Valid {
		log.TotalTokens = int(totalTokens.Int64)
	}
	if durationMs.Valid {
		log.DurationMs = int(durationMs.Int64)
	}
	if timeToFirstTokenMs.Valid {
		log.TimeToFirstTokenMs = int(timeToFirstTokenMs.Int64)
	}
	if contextTokens.Valid {
		log.ContextTokens = int(contextTokens.Int64)
	}
	if complexityScore.Valid {
		log.ComplexityScore = int(complexityScore.Int64)
	}

	// Map boolean fields
	log.Success = success == 1
	log.HadTimeout = hadTimeout == 1
	log.HadRepetition = hadRepetition == 1
	log.HadToolFailure = hadToolFailure == 1
	log.HadTruncation = hadTruncation == 1
	log.HadJSONError = hadJSONError == 1

	// Map nullable float/time fields
	if capabilityScore.Valid {
		log.CapabilityScore = capabilityScore.Float64
	}
	if assessedAt.Valid {
		log.AssessedAt = &assessedAt.Time
	}

	return &log, nil
}

// ListConversationLogs retrieves recent conversation logs.
func (s *Store) ListConversationLogs(ctx context.Context, limit int) ([]*eval.ConversationLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT
			id, request_id, session_id,
			provider, model, model_tier,
			duration_ms, success,
			had_timeout, had_repetition, had_tool_failure,
			capability_score, recommended_upgrade,
			created_at
		FROM conversation_logs
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversation logs: %w", err)
	}
	defer rows.Close()

	var logs []*eval.ConversationLog
	for rows.Next() {
		var log eval.ConversationLog
		var sessionID, modelTier, recommendedUpgrade sql.NullString
		var capabilityScore sql.NullFloat64
		var success, hadTimeout, hadRepetition, hadToolFailure int

		err := rows.Scan(
			&log.ID, &log.RequestID, &sessionID,
			&log.Provider, &log.Model, &modelTier,
			&log.DurationMs, &success,
			&hadTimeout, &hadRepetition, &hadToolFailure,
			&capabilityScore, &recommendedUpgrade,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan conversation log: %w", err)
		}

		log.SessionID = sessionID.String
		log.ModelTier = modelTier.String
		log.RecommendedUpgrade = recommendedUpgrade.String
		log.Success = success == 1
		log.HadTimeout = hadTimeout == 1
		log.HadRepetition = hadRepetition == 1
		log.HadToolFailure = hadToolFailure == 1

		if capabilityScore.Valid {
			log.CapabilityScore = capabilityScore.Float64
		}

		logs = append(logs, &log)
	}

	return logs, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// MODEL METRICS OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// UpdateModelMetrics updates or inserts daily model metrics.
func (s *Store) UpdateModelMetrics(ctx context.Context, log *eval.ConversationLog) error {
	date := log.CreatedAt.Format("2006-01-02")

	// Try to update existing record
	query := `
		INSERT INTO model_metrics (
			provider, model, model_tier, date,
			total_requests, successful_requests, failed_requests,
			timeout_count, repetition_count, tool_failure_count,
			total_duration_ms, min_duration_ms, max_duration_ms,
			total_prompt_tokens, total_completion_tokens,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model, date) DO UPDATE SET
			total_requests = total_requests + 1,
			successful_requests = successful_requests + excluded.successful_requests,
			failed_requests = failed_requests + excluded.failed_requests,
			timeout_count = timeout_count + excluded.timeout_count,
			repetition_count = repetition_count + excluded.repetition_count,
			tool_failure_count = tool_failure_count + excluded.tool_failure_count,
			total_duration_ms = total_duration_ms + excluded.total_duration_ms,
			min_duration_ms = CASE
				WHEN min_duration_ms IS NULL OR excluded.min_duration_ms < min_duration_ms
				THEN excluded.min_duration_ms
				ELSE min_duration_ms
			END,
			max_duration_ms = CASE
				WHEN max_duration_ms IS NULL OR excluded.max_duration_ms > max_duration_ms
				THEN excluded.max_duration_ms
				ELSE max_duration_ms
			END,
			total_prompt_tokens = total_prompt_tokens + excluded.total_prompt_tokens,
			total_completion_tokens = total_completion_tokens + excluded.total_completion_tokens,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	successCount := 0
	failCount := 0
	if log.Success {
		successCount = 1
	} else {
		failCount = 1
	}

	timeoutCount := 0
	if log.HadTimeout {
		timeoutCount = 1
	}
	repetitionCount := 0
	if log.HadRepetition {
		repetitionCount = 1
	}
	toolFailureCount := 0
	if log.HadToolFailure {
		toolFailureCount = 1
	}

	_, err := s.db.ExecContext(ctx, query,
		log.Provider, log.Model, log.ModelTier, date,
		successCount, failCount,
		timeoutCount, repetitionCount, toolFailureCount,
		log.DurationMs, log.DurationMs, log.DurationMs,
		log.ContextTokens, log.CompletionTokens,
		now, now,
	)

	if err != nil {
		return fmt.Errorf("update model metrics: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// UPGRADE EVENT OPERATIONS
// ═══════════════════════════════════════════════════════════════════════════════

// CreateUpgradeEvent records a model upgrade recommendation.
func (s *Store) CreateUpgradeEvent(ctx context.Context, event *eval.UpgradeEvent) error {
	query := `
		INSERT INTO model_upgrade_events (
			conversation_log_id, request_id,
			from_provider, from_model, from_tier,
			to_provider, to_model, to_tier,
			reason, issue_type, capability_score,
			user_action, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		event.ConversationLogID,
		event.RequestID,
		event.FromProvider,
		event.FromModel,
		event.FromTier,
		event.ToProvider,
		event.ToModel,
		event.ToTier,
		event.Reason,
		event.IssueType,
		event.CapabilityScore,
		"pending",
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("create upgrade event: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// boolToInt converts a boolean to 0/1 for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
