package session

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	name := "Test Brainstorm"
	projectPath := "/path/to/project"

	session := NewSession(name, projectPath)

	// Verify ID is a valid UUID (36 chars with hyphens)
	if len(session.ID) != 36 {
		t.Errorf("expected ID to be UUID format (36 chars), got %d chars", len(session.ID))
	}

	// Verify name is set correctly
	if session.Name != name {
		t.Errorf("expected Name %q, got %q", name, session.Name)
	}

	// Verify project path is set correctly
	if session.ProjectPath != projectPath {
		t.Errorf("expected ProjectPath %q, got %q", projectPath, session.ProjectPath)
	}

	// Verify timestamps are set and equal (for a new session)
	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set, got zero time")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set, got zero time")
	}
	if !session.CreatedAt.Equal(session.UpdatedAt) {
		t.Errorf("expected CreatedAt and UpdatedAt to be equal for new session")
	}
}

func TestNewSession_UniqueIDs(t *testing.T) {
	session1 := NewSession("Session 1", "/path/1")
	session2 := NewSession("Session 2", "/path/2")

	if session1.ID == session2.ID {
		t.Error("expected different sessions to have unique IDs")
	}
}

func TestNewSession_TimestampIsRecent(t *testing.T) {
	before := time.Now()
	session := NewSession("Test", "/path")
	after := time.Now()

	if session.CreatedAt.Before(before) || session.CreatedAt.After(after) {
		t.Errorf("expected CreatedAt to be between test start and end times")
	}
}

func TestSession_FieldsExist(t *testing.T) {
	// This test verifies the Session struct has all required fields
	// by attempting to access them (compile-time check)
	session := &Session{
		ID:          "test-id",
		Name:        "Test Session",
		ProjectPath: "/test/path",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Access each field to verify they exist
	_ = session.ID
	_ = session.Name
	_ = session.ProjectPath
	_ = session.CreatedAt
	_ = session.UpdatedAt
}
