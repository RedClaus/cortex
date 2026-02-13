package session

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cortexhub/cortex-gateway/internal/brain"
)

// Session represents a conversation session
type Session struct {
	ID        string
	UserID    string
	History   []Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Message represents a message in the session
type Message struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// NewSession creates a new session
func NewSession(id, userID string) *Session {
	now := time.Now()
	return &Session{
		ID:        id,
		UserID:    userID,
		History:   []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(role, content string) {
	s.History = append(s.History, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// Store stores the session in CortexBrain
func (s *Session) Store(brainClient *brain.Client) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return brainClient.StoreMemory(&brain.StoreMemoryRequest{
		Content:  string(data),
		AgentID:  "gateway",
		SessionID: s.ID,
	})
}

// LoadSession loads a session from CortexBrain
func LoadSession(id string, brainClient *brain.Client) (*Session, error) {
	resp, err := brainClient.RecallMemory(&brain.RecallMemoryRequest{
		Query:   "session:" + id,
		AgentID: "gateway",
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("session not found")
	}
	var session Session
	err = json.Unmarshal([]byte(resp.Results[0].Content), &session)
	if err != nil {
		return nil, err
	}
	return &session, nil
}
