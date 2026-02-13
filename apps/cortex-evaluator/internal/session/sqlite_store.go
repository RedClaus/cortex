// Package session provides the core session management for brainstorming sessions.
package session

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite database.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLiteStore that persists sessions to the given database path.
// The parent directory is created if it doesn't exist.
// The sessions table is created if it doesn't exist.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	store := &SQLiteStore{db: db}

	// Run migrations
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return store, nil
}

// migrate creates the sessions and chats tables if they don't exist.
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		project_path TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		archived_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_archived_at ON sessions(archived_at);

	CREATE TABLE IF NOT EXISTS chats (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		name TEXT NOT NULL,
		messages TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_chats_session_id ON chats(session_id);
	CREATE INDEX IF NOT EXISTS idx_chats_created_at ON chats(created_at DESC);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Save persists a session to the database.
func (s *SQLiteStore) Save(session *Session) error {
	if session == nil {
		return errors.New("session cannot be nil")
	}
	if session.ID == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO sessions (id, name, project_path, created_at, updated_at, archived_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		project_path = excluded.project_path,
		updated_at = excluded.updated_at,
		archived_at = excluded.archived_at
	`

	var archivedAt *string
	if session.ArchivedAt != nil {
		t := session.ArchivedAt.Format(time.RFC3339Nano)
		archivedAt = &t
	}

	_, err := s.db.Exec(query,
		session.ID,
		session.Name,
		session.ProjectPath,
		session.CreatedAt.Format(time.RFC3339Nano),
		session.UpdatedAt.Format(time.RFC3339Nano),
		archivedAt,
	)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return nil
}

// Load retrieves a session by ID from the database.
func (s *SQLiteStore) Load(id string) (*Session, error) {
	if id == "" {
		return nil, ErrInvalidID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, name, project_path, created_at, updated_at, archived_at
	FROM sessions
	WHERE id = ?
	`

	var session Session
	var createdAt, updatedAt string
	var archivedAt sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&session.ID,
		&session.Name,
		&session.ProjectPath,
		&createdAt,
		&updatedAt,
		&archivedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("load session: %w", err)
	}

	// Parse timestamps
	session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	if archivedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, archivedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse archived_at: %w", err)
		}
		session.ArchivedAt = &t
	}

	return &session, nil
}

// List returns saved sessions, sorted by creation time (newest first).
// If includeArchived is false, archived sessions are excluded from the results.
func (s *SQLiteStore) List(includeArchived bool) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var query string
	if includeArchived {
		query = `
		SELECT id, name, project_path, created_at, updated_at, archived_at
		FROM sessions
		ORDER BY created_at DESC
		`
	} else {
		query = `
		SELECT id, name, project_path, created_at, updated_at, archived_at
		FROM sessions
		WHERE archived_at IS NULL
		ORDER BY created_at DESC
		`
	}

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*Session
	for rows.Next() {
		var session Session
		var createdAt, updatedAt string
		var archivedAt sql.NullString

		err := rows.Scan(
			&session.ID,
			&session.Name,
			&session.ProjectPath,
			&createdAt,
			&updatedAt,
			&archivedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}

		session.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		session.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)

		if archivedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, archivedAt.String)
			session.ArchivedAt = &t
		}

		sessions = append(sessions, &session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}

	return sessions, nil
}

// Delete removes a session from the database.
func (s *SQLiteStore) Delete(id string) error {
	if id == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// Archive marks a session as archived.
func (s *SQLiteStore) Archive(id string) error {
	if id == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	result, err := s.db.Exec(
		"UPDATE sessions SET archived_at = ?, updated_at = ? WHERE id = ?",
		now.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return fmt.Errorf("archive session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveChat persists a chat to the database.
func (s *SQLiteStore) SaveChat(chat *Chat) error {
	if chat == nil {
		return errors.New("chat cannot be nil")
	}
	if chat.ID == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO chats (id, session_id, name, messages, created_at)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		messages = excluded.messages
	`

	_, err := s.db.Exec(query,
		chat.ID,
		chat.SessionID,
		chat.Name,
		chat.Messages,
		chat.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save chat: %w", err)
	}

	return nil
}

// ListChats returns all chats for a session, sorted by creation time (newest first).
func (s *SQLiteStore) ListChats(sessionID string) ([]*Chat, error) {
	if sessionID == "" {
		return nil, ErrInvalidID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, session_id, name, messages, created_at
	FROM chats
	WHERE session_id = ?
	ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list chats: %w", err)
	}
	defer rows.Close()

	var chats []*Chat
	for rows.Next() {
		var chat Chat
		var createdAt string

		err := rows.Scan(
			&chat.ID,
			&chat.SessionID,
			&chat.Name,
			&chat.Messages,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan chat: %w", err)
		}

		chat.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		chats = append(chats, &chat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chats: %w", err)
	}

	return chats, nil
}

// LoadChat retrieves a chat by ID.
func (s *SQLiteStore) LoadChat(id string) (*Chat, error) {
	if id == "" {
		return nil, ErrInvalidID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, session_id, name, messages, created_at
	FROM chats
	WHERE id = ?
	`

	var chat Chat
	var createdAt string

	err := s.db.QueryRow(query, id).Scan(
		&chat.ID,
		&chat.SessionID,
		&chat.Name,
		&chat.Messages,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("chat not found")
		}
		return nil, fmt.Errorf("load chat: %w", err)
	}

	chat.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return &chat, nil
}

// DeleteChat removes a chat from the database.
func (s *SQLiteStore) DeleteChat(id string) error {
	if id == "" {
		return ErrInvalidID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	result, err := s.db.Exec("DELETE FROM chats WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete chat: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("chat not found")
	}

	return nil
}
