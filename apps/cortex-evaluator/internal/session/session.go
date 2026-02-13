// Package session provides the core session management for brainstorming sessions.
package session

import (
	"time"

	"github.com/google/uuid"
)

// Session represents a brainstorming session with a target project.
type Session struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	ProjectPath string     `json:"project_path"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

// Chat represents a saved conversation within a session.
type Chat struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Name      string    `json:"name"`
	Messages  string    `json:"messages"` // JSON-encoded messages
	CreatedAt time.Time `json:"created_at"`
}

// NewChat creates a new chat with the given name and messages.
func NewChat(sessionID, name, messages string) *Chat {
	return &Chat{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Name:      name,
		Messages:  messages,
		CreatedAt: time.Now(),
	}
}

// NewSession creates a new session with the given name and project path.
// It initializes the session with a new UUID and timestamps.
func NewSession(name, projectPath string) *Session {
	now := time.Now()
	return &Session{
		ID:          uuid.New().String(),
		Name:        name,
		ProjectPath: projectPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Archive marks the session as archived with the current timestamp.
func (s *Session) Archive() {
	now := time.Now()
	s.ArchivedAt = &now
	s.UpdatedAt = now
}

// IsArchived returns true if the session has been archived.
func (s *Session) IsArchived() bool {
	return s.ArchivedAt != nil
}
