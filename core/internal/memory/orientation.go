// Package memory provides enhanced memory capabilities for Cortex.
// This file implements the Orientation Engine for agent "wakeup" and identity management.
// CR-015: Cortex Memory Enhancement - Phase 3: Orientation Engine & Identity
package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// IDENTITY TYPES
// ============================================================================

// AgentIdentity represents the persistent identity of the agent.
type AgentIdentity struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CoreValues    []string  `json:"core_values"`
	CurrentGoal   string    `json:"current_goal"`
	Mood          string    `json:"mood"`
	PersonaPrompt string    `json:"persona_prompt"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SessionSummary represents a brief summary of a past session.
type SessionSummary struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	Outcome   string    `json:"outcome"`
	Timestamp time.Time `json:"timestamp"`
}

// OrientationContext contains all contextual information for agent wakeup.
type OrientationContext struct {
	Identity       AgentIdentity     `json:"identity"`
	ActiveTopics   []Topic           `json:"active_topics"`
	TopPrinciples  []StrategicMemory `json:"top_principles"`
	RecentGoals    []string          `json:"recent_goals"`
	SessionHistory []SessionSummary  `json:"session_history"`
}

// ============================================================================
// ORIENTATION ENGINE
// ============================================================================

// OrientationEngine handles agent "wakeup" and loads identity, context, and principles.
type OrientationEngine struct {
	db             *sql.DB
	topicStore     *TopicStore
	strategicStore *StrategicMemoryStore
}

// NewOrientationEngine creates a new orientation engine.
func NewOrientationEngine(db *sql.DB, topicStore *TopicStore, strategicStore *StrategicMemoryStore) *OrientationEngine {
	return &OrientationEngine{
		db:             db,
		topicStore:     topicStore,
		strategicStore: strategicStore,
	}
}

// WakeUp loads the full orientation context for the agent.
// This is called at the start of a session to provide the agent with its identity,
// active context, and guiding principles.
func (e *OrientationEngine) WakeUp(ctx context.Context) (*OrientationContext, error) {
	orientation := &OrientationContext{}

	// Load identity
	identity, err := e.loadIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("wakeup: load identity: %w", err)
	}
	orientation.Identity = *identity

	// Load active topics (limit to 5 most recent)
	activeTopics, err := e.loadActiveTopics(ctx, 5)
	if err != nil {
		// Non-fatal: continue without topics
		activeTopics = []Topic{}
	}
	orientation.ActiveTopics = activeTopics

	// Load top principles (limit to 5 most reliable)
	if e.strategicStore != nil {
		principles, err := e.strategicStore.GetTopPrinciples(ctx, 5)
		if err != nil {
			// Non-fatal: continue without principles
			principles = []StrategicMemory{}
		}
		orientation.TopPrinciples = principles
	}

	// Load recent sessions (limit to 10)
	sessions, err := e.loadRecentSessions(ctx, 10)
	if err != nil {
		// Non-fatal: continue without session history
		sessions = []SessionSummary{}
	}
	orientation.SessionHistory = sessions

	// Derive recent goals from sessions
	orientation.RecentGoals = make([]string, 0, len(sessions))
	for _, s := range sessions {
		if s.Goal != "" {
			orientation.RecentGoals = append(orientation.RecentGoals, s.Goal)
		}
	}

	// If no current goal is set, derive from sessions
	if orientation.Identity.CurrentGoal == "" {
		orientation.Identity.CurrentGoal = e.deriveCurrentGoal(sessions)
	}

	return orientation, nil
}

// GenerateSystemPreamble builds a system prompt section from the orientation context.
func (e *OrientationEngine) GenerateSystemPreamble(orientation *OrientationContext) string {
	var sb strings.Builder

	// Identity introduction
	sb.WriteString(fmt.Sprintf("You are %s, a terminal AI assistant.\n\n", orientation.Identity.Name))

	// Core values
	if len(orientation.Identity.CoreValues) > 0 {
		sb.WriteString("## Core Values\n")
		for _, value := range orientation.Identity.CoreValues {
			sb.WriteString(fmt.Sprintf("- %s\n", value))
		}
		sb.WriteString("\n")
	}

	// Current focus/goal
	if orientation.Identity.CurrentGoal != "" {
		sb.WriteString("## Current Focus\n")
		sb.WriteString(orientation.Identity.CurrentGoal)
		sb.WriteString("\n\n")
	}

	// Active topics
	if len(orientation.ActiveTopics) > 0 {
		sb.WriteString("## Active Topics\n")
		for _, topic := range orientation.ActiveTopics {
			if topic.Description != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", topic.Name, topic.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", topic.Name))
			}
		}
		sb.WriteString("\n")
	}

	// Guiding principles
	if len(orientation.TopPrinciples) > 0 {
		sb.WriteString("## Guiding Principles\n")
		for _, principle := range orientation.TopPrinciples {
			successPct := int(principle.SuccessRate * 100)
			sb.WriteString(fmt.Sprintf("- %s (%d%% success)\n", principle.Principle, successPct))
		}
		sb.WriteString("\n")
	}

	// Recent sessions
	if len(orientation.SessionHistory) > 0 {
		sb.WriteString("## Recent Sessions\n")
		for _, session := range orientation.SessionHistory {
			if session.Goal != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", session.Goal, session.Outcome))
			}
		}
	}

	return sb.String()
}

// GetIdentity loads and returns the agent identity.
func (e *OrientationEngine) GetIdentity(ctx context.Context) (*AgentIdentity, error) {
	return e.loadIdentity(ctx)
}

// UpdateIdentity updates specific fields of the agent identity.
func (e *OrientationEngine) UpdateIdentity(ctx context.Context, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	// Build SET clause dynamically
	setClauses := make([]string, 0, len(updates)+1)
	args := make([]interface{}, 0, len(updates)+2)

	for field, value := range updates {
		switch field {
		case "name":
			setClauses = append(setClauses, "name = ?")
			args = append(args, value)
		case "core_values":
			setClauses = append(setClauses, "core_values = ?")
			if values, ok := value.([]string); ok {
				jsonBytes, err := json.Marshal(values)
				if err != nil {
					return fmt.Errorf("update identity: marshal core_values: %w", err)
				}
				args = append(args, string(jsonBytes))
			} else {
				args = append(args, value)
			}
		case "current_goal":
			setClauses = append(setClauses, "current_goal = ?")
			args = append(args, value)
		case "mood":
			setClauses = append(setClauses, "mood = ?")
			args = append(args, value)
		case "persona_prompt":
			setClauses = append(setClauses, "persona_prompt = ?")
			args = append(args, value)
		default:
			return fmt.Errorf("update identity: unknown field: %s", field)
		}
	}

	// Always update the timestamp
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().Format(time.RFC3339))

	// Add WHERE clause arg
	args = append(args, "henry")

	query := fmt.Sprintf("UPDATE agent_identity SET %s WHERE id = ?", strings.Join(setClauses, ", "))

	result, err := e.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update identity: exec failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update identity: rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("update identity: identity not found")
	}

	return nil
}

// UpdateMood updates the agent's mood based on session sentiment.
func (e *OrientationEngine) UpdateMood(ctx context.Context, sessionSentiment float64) error {
	mood := deriveMoodFromSentiment(sessionSentiment)
	return e.UpdateIdentity(ctx, map[string]any{"mood": mood})
}

// UpdateCurrentGoal sets the agent's current goal.
func (e *OrientationEngine) UpdateCurrentGoal(ctx context.Context, goal string) error {
	return e.UpdateIdentity(ctx, map[string]any{"current_goal": goal})
}

// ClearCurrentGoal clears the agent's current goal.
func (e *OrientationEngine) ClearCurrentGoal(ctx context.Context) error {
	return e.UpdateIdentity(ctx, map[string]any{"current_goal": ""})
}

// ============================================================================
// PRIVATE HELPER METHODS
// ============================================================================

// loadIdentity loads the agent identity from the database.
func (e *OrientationEngine) loadIdentity(ctx context.Context) (*AgentIdentity, error) {
	query := `
		SELECT id, name, core_values, current_goal, mood, persona_prompt, updated_at
		FROM agent_identity
		WHERE id = 'henry'
	`

	row := e.db.QueryRowContext(ctx, query)

	var identity AgentIdentity
	var coreValuesJSON, currentGoal, mood, personaPrompt sql.NullString
	var updatedAt string

	err := row.Scan(
		&identity.ID,
		&identity.Name,
		&coreValuesJSON,
		&currentGoal,
		&mood,
		&personaPrompt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		// Return default identity if not found
		return &AgentIdentity{
			ID:         "henry",
			Name:       "Henry",
			CoreValues: []string{"Be helpful", "Be concise", "Respect privacy"},
			Mood:       "neutral",
			UpdatedAt:  time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load identity: scan failed: %w", err)
	}

	// Parse core values JSON
	if coreValuesJSON.Valid && coreValuesJSON.String != "" {
		if err := json.Unmarshal([]byte(coreValuesJSON.String), &identity.CoreValues); err != nil {
			// Fallback to default values on parse error
			identity.CoreValues = []string{"Be helpful", "Be concise", "Respect privacy"}
		}
	} else {
		identity.CoreValues = []string{}
	}

	// Handle nullable strings
	identity.CurrentGoal = currentGoal.String
	identity.Mood = mood.String
	if identity.Mood == "" {
		identity.Mood = "neutral"
	}
	identity.PersonaPrompt = personaPrompt.String

	// Parse timestamp
	if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		identity.UpdatedAt = t
	} else if t, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
		identity.UpdatedAt = t
	} else {
		identity.UpdatedAt = time.Now()
	}

	return &identity, nil
}

// loadActiveTopics loads active topics from the TopicStore.
func (e *OrientationEngine) loadActiveTopics(ctx context.Context, limit int) ([]Topic, error) {
	if e.topicStore == nil {
		return []Topic{}, nil
	}
	return e.topicStore.GetActiveTopics(ctx, limit)
}

// loadRecentSessions loads recent session summaries from the sessions table.
func (e *OrientationEngine) loadRecentSessions(ctx context.Context, limit int) ([]SessionSummary, error) {
	query := `
		SELECT id, title, status, last_activity_at
		FROM sessions
		ORDER BY last_activity_at DESC
		LIMIT ?
	`

	rows, err := e.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("load recent sessions: query failed: %w", err)
	}
	defer rows.Close()

	var sessions []SessionSummary
	for rows.Next() {
		var s SessionSummary
		var title, status sql.NullString
		var lastActivityAt sql.NullString

		err := rows.Scan(&s.ID, &title, &status, &lastActivityAt)
		if err != nil {
			continue
		}

		// Use title as goal (sessions don't have explicit goal field)
		s.Goal = title.String

		// Map status to outcome
		switch status.String {
		case "completed":
			s.Outcome = "completed"
		case "abandoned":
			s.Outcome = "abandoned"
		case "active":
			s.Outcome = "in_progress"
		default:
			s.Outcome = status.String
		}

		// Parse timestamp
		if lastActivityAt.Valid && lastActivityAt.String != "" {
			if t, err := time.Parse(time.RFC3339, lastActivityAt.String); err == nil {
				s.Timestamp = t
			} else if t, err := time.Parse("2006-01-02 15:04:05", lastActivityAt.String); err == nil {
				s.Timestamp = t
			}
		}

		sessions = append(sessions, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load recent sessions: rows error: %w", err)
	}

	// If database returned no sessions, try loading from JSON files
	if len(sessions) == 0 {
		jsonSessions, err := e.loadSessionsFromJSONFiles(limit)
		if err == nil && len(jsonSessions) > 0 {
			return jsonSessions, nil
		}
		// Non-fatal: if JSON loading fails, just return empty slice
	}

	return sessions, nil
}

// sessionJSONFile represents the structure of a session JSON file.
type sessionJSONFile struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// loadSessionsFromJSONFiles loads session summaries from JSON files in ~/.cortex/sessions/
func (e *OrientationEngine) loadSessionsFromJSONFiles(limit int) ([]SessionSummary, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	sessionsDir := filepath.Join(homeDir, ".cortex", "sessions")

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionSummary{}, nil
		}
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	// Collect session files with mod times
	type sessionFileInfo struct {
		path    string
		modTime time.Time
	}
	var sessionFiles []sessionFileInfo

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(sessionsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessionFiles = append(sessionFiles, sessionFileInfo{path: filePath, modTime: info.ModTime()})
	}

	// Sort by modification time (most recent first)
	sort.Slice(sessionFiles, func(i, j int) bool {
		return sessionFiles[i].modTime.After(sessionFiles[j].modTime)
	})

	// Limit files to process
	if len(sessionFiles) > limit {
		sessionFiles = sessionFiles[:limit]
	}

	var sessions []SessionSummary
	for _, sf := range sessionFiles {
		data, err := os.ReadFile(sf.path)
		if err != nil {
			continue
		}

		var sessionFile sessionJSONFile
		if err := json.Unmarshal(data, &sessionFile); err != nil {
			continue
		}

		summary := SessionSummary{
			ID:      sessionFile.ID,
			Goal:    sessionFile.Title,
			Outcome: "completed",
		}

		if !sessionFile.CreatedAt.IsZero() {
			summary.Timestamp = sessionFile.CreatedAt
		} else if !sessionFile.UpdatedAt.IsZero() {
			summary.Timestamp = sessionFile.UpdatedAt
		} else {
			summary.Timestamp = sf.modTime
		}

		if summary.Goal == "" {
			continue
		}
		sessions = append(sessions, summary)
	}

	return sessions, nil
}

// deriveCurrentGoal finds the most recent incomplete session goal.
func (e *OrientationEngine) deriveCurrentGoal(sessions []SessionSummary) string {
	for _, s := range sessions {
		// Look for sessions that are in progress (not completed)
		if s.Outcome == "in_progress" || s.Outcome == "active" {
			if s.Goal != "" {
				return s.Goal
			}
		}
	}
	return ""
}

// deriveMoodFromSentiment converts a sentiment score to a mood string.
func deriveMoodFromSentiment(sentiment float64) string {
	switch {
	case sentiment > 0.5:
		return "positive"
	case sentiment < -0.5:
		return "frustrated"
	default:
		return "neutral"
	}
}
