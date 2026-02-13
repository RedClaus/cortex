// Package session provides the core session management for brainstorming sessions.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Common errors for session operations.
var (
	ErrSessionNotFound = errors.New("session not found")
	ErrInvalidID       = errors.New("invalid session ID")
)

// Store defines the interface for session persistence.
type Store interface {
	// Save persists a session to storage.
	Save(session *Session) error

	// Load retrieves a session by ID.
	Load(id string) (*Session, error)

	// List returns saved sessions. If includeArchived is false, archived sessions are excluded.
	List(includeArchived bool) ([]*Session, error)

	// Delete removes a session from storage.
	Delete(id string) error

	// Archive marks a session as archived.
	Archive(id string) error

	// SaveChat persists a chat to storage.
	SaveChat(chat *Chat) error

	// ListChats returns all chats for a session.
	ListChats(sessionID string) ([]*Chat, error)

	// LoadChat retrieves a chat by ID.
	LoadChat(id string) (*Chat, error)

	// DeleteChat removes a chat from storage.
	DeleteChat(id string) error

	// Close releases any resources held by the store.
	Close() error
}

// FileStore implements Store using JSON files in a directory.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a new FileStore that persists sessions to the given directory.
// The directory is created if it doesn't exist.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create sessions directory: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

// Save persists a session to a JSON file.
func (fs *FileStore) Save(session *Session) error {
	if session == nil {
		return errors.New("session cannot be nil")
	}
	if session.ID == "" {
		return ErrInvalidID
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := fs.sessionPath(session.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load retrieves a session by ID from the file system.
func (fs *FileStore) Load(id string) (*Session, error) {
	if id == "" {
		return nil, ErrInvalidID
	}

	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.sessionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// List returns saved sessions, sorted by creation time (newest first).
// If includeArchived is false, archived sessions are excluded from the results.
func (fs *FileStore) List(includeArchived bool) ([]*Session, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Session{}, nil
		}
		return nil, fmt.Errorf("read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(fs.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue // Skip invalid files
		}

		// Filter out archived sessions unless includeArchived is true
		if !includeArchived && session.IsArchived() {
			continue
		}

		sessions = append(sessions, &session)
	}

	// Sort by CreatedAt descending (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

// Delete removes a session from the file system.
func (fs *FileStore) Delete(id string) error {
	if id == "" {
		return ErrInvalidID
	}

	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.sessionPath(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("delete session file: %w", err)
	}

	return nil
}

// Archive marks a session as archived by loading, updating, and saving it.
func (fs *FileStore) Archive(id string) error {
	if id == "" {
		return ErrInvalidID
	}

	// Load the session (this acquires read lock internally)
	session, err := fs.Load(id)
	if err != nil {
		return err
	}

	// Mark as archived
	session.Archive()

	// Save the updated session (this acquires write lock internally)
	return fs.Save(session)
}

// sessionPath returns the file path for a session ID.
func (fs *FileStore) sessionPath(id string) string {
	return filepath.Join(fs.dir, id+".json")
}

// Close releases resources. For FileStore, this is a no-op.
func (fs *FileStore) Close() error {
	return nil
}

// SaveChat is not supported by FileStore.
func (fs *FileStore) SaveChat(chat *Chat) error {
	return errors.New("chat storage not supported by FileStore")
}

// ListChats is not supported by FileStore.
func (fs *FileStore) ListChats(sessionID string) ([]*Chat, error) {
	return nil, errors.New("chat storage not supported by FileStore")
}

// LoadChat is not supported by FileStore.
func (fs *FileStore) LoadChat(id string) (*Chat, error) {
	return nil, errors.New("chat storage not supported by FileStore")
}

// DeleteChat is not supported by FileStore.
func (fs *FileStore) DeleteChat(id string) error {
	return errors.New("chat storage not supported by FileStore")
}
