package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// Session represents a coding session
type Session struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	ProjectPath string       `json:"project_path"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Files       []SessionFile `json:"files"`
	Messages    []SessionMessage `json:"messages"`
	Cursor      CursorState  `json:"cursor"`
}

// SessionFile represents an open file in a session
type SessionFile struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Content     string    `json:"content,omitempty"`
	Line        int       `json:"line"`
	Column      int       `json:"column"`
	Modified    bool      `json:"modified"`
	LastOpened  time.Time `json:"last_opened"`
}

// SessionMessage represents a chat message in a session
type SessionMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// CursorState represents the cursor position
type CursorState struct {
	FilePath   string `json:"file_path"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	ScrollLine int    `json:"scroll_line"`
}

// Manager handles session management
type Manager struct {
	sessionDir   string
	currentSession *Session
	autoSaveTick  time.Duration
	autoSaveChan  chan bool
	stopChan     chan struct{}
	callback     SessionCallback
}

// SessionCallback is called on session events
type SessionCallback interface {
	OnSessionSave(session *Session)
	OnSessionLoad(session *Session)
	OnSessionError(err error)
}

// NoopCallback is a no-op callback
type NoopCallback struct{}

// OnSessionSave implements SessionCallback
func (n NoopCallback) OnSessionSave(session *Session) {}

// OnSessionLoad implements SessionCallback
func (n NoopCallback) OnSessionLoad(session *Session) {}

// OnSessionError implements SessionCallback
func (n NoopCallback) OnSessionError(err error) {}

// Option configures the manager
type Option func(*Manager)

// WithAutoSave configures auto-save
func WithAutoSave(enabled bool, interval time.Duration) Option {
	return func(m *Manager) {
		if enabled {
			m.autoSaveTick = interval
		} else {
			m.autoSaveTick = 0
		}
	}
}

// WithCallback sets the session callback
func WithCallback(cb SessionCallback) Option {
	return func(m *Manager) {
		m.callback = cb
	}
}

// NewManager creates a new session manager
func NewManager(opts ...Option) (*Manager, error) {
	m := &Manager{
		autoSaveTick: 30 * time.Second,
		stopChan:     make(chan struct{}),
		callback:     NoopCallback{},
	}
	
	// Apply options
	for _, opt := range opts {
		opt(m)
	}
	
	// Set session directory
	m.sessionDir = getSessionDir()
	
	return m, nil
}

// getSessionDir returns the session directory
func getSessionDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	
	dir := filepath.Join(home, ".local", "share", "cortex-coder", "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}
	
	return dir
}

// CreateSession creates a new session
func (m *Manager) CreateSession(projectPath, name string) (*Session, error) {
	session := &Session{
		ID:          uuid.New().String(),
		Name:        name,
		ProjectPath: projectPath,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Files:       make([]SessionFile, 0),
		Messages:    make([]SessionMessage, 0),
	}
	
	m.currentSession = session
	return session, nil
}

// LoadSession loads a session from file
func (m *Manager) LoadSession(sessionID string) (*Session, error) {
	path := m.getSessionPath(sessionID)
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}
	
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}
	
	m.currentSession = &session
	m.callback.OnSessionLoad(&session)
	
	return &session, nil
}

// SaveSession saves the current session
func (m *Manager) SaveSession() error {
	if m.currentSession == nil {
		return nil
	}
	
	m.currentSession.UpdatedAt = time.Now()
	
	data, err := json.MarshalIndent(m.currentSession, "", "  ")
	if err != nil {
		m.callback.OnSessionError(fmt.Errorf("failed to marshal session: %w", err))
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	path := m.getSessionPath(m.currentSession.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		m.callback.OnSessionError(fmt.Errorf("failed to write session: %w", err))
		return fmt.Errorf("failed to write session: %w", err)
	}
	
	m.callback.OnSessionSave(m.currentSession)
	return nil
}

// SaveSessionAs saves the session with a different name
func (m *Manager) SaveSessionAs(name string) error {
	if m.currentSession == nil {
		return fmt.Errorf("no active session")
	}
	
	m.currentSession.Name = name
	return m.SaveSession()
}

// GetCurrentSession returns the current session
func (m *Manager) GetCurrentSession() *Session {
	return m.currentSession
}

// AddFile adds a file to the session
func (m *Manager) AddFile(path, content string) {
	if m.currentSession == nil {
		return
	}
	
	file := SessionFile{
		Path:       path,
		Name:       filepath.Base(path),
		Content:    content,
		Modified:   false,
		LastOpened: time.Now(),
	}
	
	m.currentSession.Files = append(m.currentSession.Files, file)
}

// RemoveFile removes a file from the session
func (m *Manager) RemoveFile(path string) {
	if m.currentSession == nil {
		return
	}
	
	files := make([]SessionFile, 0)
	for _, f := range m.currentSession.Files {
		if f.Path != path {
			files = append(files, f)
		}
	}
	
	m.currentSession.Files = files
}

// UpdateFile updates a file in the session
func (m *Manager) UpdateFile(path, content string, modified bool) {
	if m.currentSession == nil {
		return
	}
	
	for i, f := range m.currentSession.Files {
		if f.Path == path {
			m.currentSession.Files[i].Content = content
			m.currentSession.Files[i].Modified = modified
			m.currentSession.Files[i].LastOpened = time.Now()
			return
		}
	}
}

// GetFile returns a file by path
func (m *Manager) GetFile(path string) *SessionFile {
	if m.currentSession == nil {
		return nil
	}
	
	for _, f := range m.currentSession.Files {
		if f.Path == path {
			return &f
		}
	}
	
	return nil
}

// AddMessage adds a message to the session
func (m *Manager) AddMessage(role, content string) {
	if m.currentSession == nil {
		return
	}
	
	msg := SessionMessage{
		ID:        uuid.New().String(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	
	m.currentSession.Messages = append(m.currentSession.Messages, msg)
}

// GetMessages returns all messages
func (m *Manager) GetMessages() []SessionMessage {
	if m.currentSession == nil {
		return nil
	}
	return m.currentSession.Messages
}

// SetCursor updates the cursor state
func (m *Manager) SetCursor(filePath string, line, column, scrollLine int) {
	if m.currentSession == nil {
		return
	}
	
	m.currentSession.Cursor = CursorState{
		FilePath:   filePath,
		Line:       line,
		Column:     column,
		ScrollLine: scrollLine,
	}
}

// GetCursor returns the cursor state
func (m *Manager) GetCursor() CursorState {
	if m.currentSession == nil {
		return CursorState{}
	}
	return m.currentSession.Cursor
}

// ListSessions lists all sessions
func (m *Manager) ListSessions() ([]SessionSummary, error) {
	entries, err := os.ReadDir(m.sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}
	
	var summaries []SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		
		sessionID := entry.Name()[:len(entry.Name())-5]
		summary, err := m.GetSessionSummary(sessionID)
		if err != nil {
			continue
		}
		
		summaries = append(summaries, *summary)
	}
	
	return summaries, nil
}

// GetSessionSummary returns a summary of a session
func (m *Manager) GetSessionSummary(sessionID string) (*SessionSummary, error) {
	path := m.getSessionPath(sessionID)
	
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}
	
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}
	
	return &SessionSummary{
		ID:          session.ID,
		Name:        session.Name,
		ProjectPath: session.ProjectPath,
		CreatedAt:   session.CreatedAt,
		UpdatedAt:   session.UpdatedAt,
		FileCount:   len(session.Files),
		MessageCount: len(session.Messages),
	}, nil
}

// DeleteSession deletes a session
func (m *Manager) DeleteSession(sessionID string) error {
	path := m.getSessionPath(sessionID)
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	
	return os.Remove(path)
}

// getSessionPath returns the full path for a session
func (m *Manager) getSessionPath(sessionID string) string {
	return filepath.Join(m.sessionDir, sessionID+".json")
}

// StartAutoSave starts the auto-save timer
func (m *Manager) StartAutoSave() {
	if m.autoSaveTick == 0 {
		return
	}
	
	go func() {
		ticker := time.NewTicker(m.autoSaveTick)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := m.SaveSession(); err != nil {
					m.callback.OnSessionError(err)
				}
			case <-m.stopChan:
				return
			}
		}
	}()
}

// StopAutoSave stops the auto-save timer
func (m *Manager) StopAutoSave() {
	close(m.stopChan)
}

// HasSession returns true if there is an active session
func (m *Manager) HasSession() bool {
	return m.currentSession != nil
}

// SessionSummary is a summary of a session
type SessionSummary struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ProjectPath  string    `json:"project_path"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	FileCount    int       `json:"file_count"`
	MessageCount int       `json:"message_count"`
}

// FormatDuration formats a duration for display
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	
	switch {
	case hours > 24:
		days := hours / 24
		return fmt.Sprintf("%dd ago", days)
	case hours > 0:
		return fmt.Sprintf("%dh ago", hours)
	default:
		return fmt.Sprintf("%dm ago", minutes)
	}
}
