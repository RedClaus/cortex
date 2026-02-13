package security

import (
	"sync"
	"time"
)

const (
	// DefaultLockTimeout is the default session timeout duration
	DefaultLockTimeout = 5 * time.Minute
)

// Session manages the unlock state and timeout
type Session struct {
	mu             sync.RWMutex
	unlocked       bool
	lastActivity   time.Time
	timeout        time.Duration
	stopChan       chan struct{}
	checkerRunning bool
	closed         bool
}

// NewSession creates a new session manager
func NewSession() *Session {
	return &Session{
		timeout:  DefaultLockTimeout,
		stopChan: make(chan struct{}),
	}
}

// Unlock marks the session as unlocked
func (s *Session) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.unlocked = true
	s.lastActivity = time.Now()

	// Start the timeout checker only if not already running
	if !s.checkerRunning {
		s.checkerRunning = true
		go s.timeoutChecker()
	}
}

// Lock marks the session as locked
func (s *Session) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unlocked = false
	// Note: we don't stop the checker here - it will just do nothing while locked
	// This avoids the complexity of coordinating goroutine start/stop
}

// IsUnlocked returns true if the session is currently unlocked
func (s *Session) IsUnlocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.unlocked {
		return false
	}

	// Check if timeout has passed
	if time.Since(s.lastActivity) > s.timeout {
		return false
	}

	return true
}

// Touch updates the last activity time (call on user interaction)
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.unlocked {
		s.lastActivity = time.Now()
	}
}

// SetTimeout sets the session timeout duration
func (s *Session) SetTimeout(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.timeout = d
}

// GetTimeout returns the current timeout duration
func (s *Session) GetTimeout() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.timeout
}

// TimeRemaining returns the time remaining before auto-lock
func (s *Session) TimeRemaining() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.unlocked {
		return 0
	}

	remaining := s.timeout - time.Since(s.lastActivity)
	if remaining < 0 {
		return 0
	}

	return remaining
}

// timeoutChecker runs in the background to auto-lock
func (s *Session) timeoutChecker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.unlocked && time.Since(s.lastActivity) > s.timeout {
				s.unlocked = false
			}
			s.mu.Unlock()
		case <-s.stopChan:
			return
		}
	}
}

// Close cleans up the session
func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.unlocked = false
	s.mu.Unlock()

	// Signal the checker goroutine to stop
	// Use a select to avoid blocking if no goroutine is running
	select {
	case s.stopChan <- struct{}{}:
	default:
	}
}
