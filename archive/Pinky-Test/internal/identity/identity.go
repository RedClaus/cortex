// Package identity manages cross-channel user identification.
//
// The identity system allows users to link multiple channel accounts (Telegram,
// Discord, Slack, WebUI, TUI) to a single Pinky identity. This enables:
//   - Consistent preferences and memory across channels
//   - Unified permission settings
//   - Cross-channel notifications
//   - Single conversation history
package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// User represents a Pinky user with linked accounts
type User struct {
	ID             string           `json:"id"`
	PrimaryName    string           `json:"primary_name"`
	LinkedAccounts []LinkedAccount  `json:"linked_accounts"`
	Persona        string           `json:"persona"`
	Permissions    string           `json:"permissions"` // tier
	Preferences    UserPreferences  `json:"preferences"`
	CreatedAt      time.Time        `json:"created_at"`
	LastSeenAt     time.Time        `json:"last_seen_at"`
}

// LinkedAccount connects a channel identity to a Pinky user
type LinkedAccount struct {
	Channel    string    `json:"channel"` // "telegram", "discord", "slack", etc.
	ExternalID string    `json:"external_id"`
	Username   string    `json:"username"`
	LinkedAt   time.Time `json:"linked_at"`
	Verified   bool      `json:"verified"`
	Primary    bool      `json:"primary"`
}

// UserPreferences stores user settings
type UserPreferences struct {
	Verbosity string `json:"verbosity"` // "minimal", "normal", "verbose"
	Timezone  string `json:"timezone"`
	Language  string `json:"language"`
}

// LinkCode is a temporary code for linking accounts
type LinkCode struct {
	Code      string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Service manages user identities
type Service struct {
	mu        sync.RWMutex
	users     map[string]*User             // by user ID
	byChannel map[string]map[string]string // channel -> externalID -> userID
	linkCodes map[string]*LinkCode         // code -> LinkCode

	// Event listeners
	listenersMu sync.RWMutex
	listeners   []EventListener
}

// NewService creates a new identity service
func NewService() *Service {
	return &Service{
		users:     make(map[string]*User),
		byChannel: make(map[string]map[string]string),
		linkCodes: make(map[string]*LinkCode),
	}
}

// GetOrCreate finds a user by channel/externalID or creates a new one
func (s *Service) GetOrCreate(channel, externalID, username string) *User {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this account is already linked
	if channelMap, ok := s.byChannel[channel]; ok {
		if userID, ok := channelMap[externalID]; ok {
			if user, ok := s.users[userID]; ok {
				user.LastSeenAt = time.Now()
				return user
			}
		}
	}

	// Create new user
	user := &User{
		ID:          generateID(),
		PrimaryName: username,
		LinkedAccounts: []LinkedAccount{
			{
				Channel:    channel,
				ExternalID: externalID,
				Username:   username,
				LinkedAt:   time.Now(),
				Verified:   true,
				Primary:    true,
			},
		},
		Persona:     "professional",
		Permissions: "some",
		Preferences: UserPreferences{
			Verbosity: "normal",
			Language:  "en",
		},
		CreatedAt:  time.Now(),
		LastSeenAt: time.Now(),
	}

	s.users[user.ID] = user
	if _, ok := s.byChannel[channel]; !ok {
		s.byChannel[channel] = make(map[string]string)
	}
	s.byChannel[channel][externalID] = user.ID

	return user
}

// GenerateLinkCode creates a code for linking accounts
func (s *Service) GenerateLinkCode(userID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := generateShortCode()
	s.linkCodes[code] = &LinkCode{
		Code:      code,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}

	return code
}

// LinkAccount links a new channel account using a code
func (s *Service) LinkAccount(code, channel, externalID, username string) (*User, error) {
	s.mu.Lock()

	linkCode, ok := s.linkCodes[code]
	if !ok || time.Now().After(linkCode.ExpiresAt) {
		s.mu.Unlock()
		return nil, ErrInvalidCode
	}

	user, ok := s.users[linkCode.UserID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrUserNotFound
	}

	// Check if already linked to another user
	if channelMap, ok := s.byChannel[channel]; ok {
		if existingUserID, ok := channelMap[externalID]; ok && existingUserID != user.ID {
			s.mu.Unlock()
			return nil, ErrAlreadyLinked
		}
	}

	// Add linked account
	user.LinkedAccounts = append(user.LinkedAccounts, LinkedAccount{
		Channel:    channel,
		ExternalID: externalID,
		Username:   username,
		LinkedAt:   time.Now(),
		Verified:   true,
		Primary:    false,
	})

	// Update lookup map
	if _, ok := s.byChannel[channel]; !ok {
		s.byChannel[channel] = make(map[string]string)
	}
	s.byChannel[channel][externalID] = user.ID

	// Remove used code
	delete(s.linkCodes, code)

	s.mu.Unlock()

	// Notify listeners (outside lock)
	s.notifyLinked(user, channel, externalID)

	return user, nil
}

// GetUser retrieves a user by ID
func (s *Service) GetUser(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return user, ok
}

// generateID creates a unique user ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateShortCode creates a human-readable link code
func generateShortCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // No confusing chars
	b := make([]byte, 4)
	rand.Read(b)
	code := make([]byte, 8)
	for i := range code {
		if i == 4 {
			code[i] = '-'
			continue
		}
		idx := i
		if i > 4 {
			idx = i - 1
		}
		code[i] = chars[int(b[idx%4])%len(chars)]
	}
	return string(code)
}

// Common errors
var (
	ErrInvalidCode      = errors.New("invalid or expired link code")
	ErrUserNotFound     = errors.New("user not found")
	ErrAccountNotFound  = errors.New("linked account not found")
	ErrCannotUnlink     = errors.New("cannot unlink the only account")
	ErrAlreadyLinked    = errors.New("account already linked to another user")
	ErrCannotSetPrimary = errors.New("account must be linked before setting as primary")
)

// UnlinkAccount removes a linked account from a user
func (s *Service) UnlinkAccount(userID, channel, externalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	// Find and remove the account
	idx := -1
	for i, acc := range user.LinkedAccounts {
		if acc.Channel == channel && acc.ExternalID == externalID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return ErrAccountNotFound
	}

	// Cannot unlink if it's the only account
	if len(user.LinkedAccounts) == 1 {
		return ErrCannotUnlink
	}

	// If removing primary, make another account primary
	wasPrimary := user.LinkedAccounts[idx].Primary
	user.LinkedAccounts = append(user.LinkedAccounts[:idx], user.LinkedAccounts[idx+1:]...)

	if wasPrimary && len(user.LinkedAccounts) > 0 {
		user.LinkedAccounts[0].Primary = true
		user.PrimaryName = user.LinkedAccounts[0].Username
	}

	// Remove from lookup map
	if channelMap, ok := s.byChannel[channel]; ok {
		delete(channelMap, externalID)
	}

	// Notify listeners
	s.notifyUnlinked(user, channel, externalID)

	return nil
}

// SetPrimaryAccount changes which linked account is the primary
func (s *Service) SetPrimaryAccount(userID, channel, externalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	found := false
	for i := range user.LinkedAccounts {
		if user.LinkedAccounts[i].Channel == channel && user.LinkedAccounts[i].ExternalID == externalID {
			user.LinkedAccounts[i].Primary = true
			user.PrimaryName = user.LinkedAccounts[i].Username
			found = true
		} else {
			user.LinkedAccounts[i].Primary = false
		}
	}

	if !found {
		return ErrCannotSetPrimary
	}

	return nil
}

// GetLinkedAccounts returns all linked accounts for a user
func (s *Service) GetLinkedAccounts(userID string) ([]LinkedAccount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}

	// Return a copy to prevent external modification
	accounts := make([]LinkedAccount, len(user.LinkedAccounts))
	copy(accounts, user.LinkedAccounts)
	return accounts, nil
}

// FindByChannel looks up a user by any of their linked channels
func (s *Service) FindByChannel(channel, externalID string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if channelMap, ok := s.byChannel[channel]; ok {
		if userID, ok := channelMap[externalID]; ok {
			if user, ok := s.users[userID]; ok {
				return user, true
			}
		}
	}
	return nil, false
}

// IsLinked checks if a channel account is already linked to any user
func (s *Service) IsLinked(channel, externalID string) bool {
	_, found := s.FindByChannel(channel, externalID)
	return found
}

// UpdatePreferences updates a user's preferences
func (s *Service) UpdatePreferences(userID string, prefs UserPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.Preferences = prefs
	return nil
}

// UpdatePersona changes the user's persona
func (s *Service) UpdatePersona(userID, persona string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.Persona = persona
	return nil
}

// UpdatePermissions changes the user's permission tier
func (s *Service) UpdatePermissions(userID, tier string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	user.Permissions = tier
	return nil
}

// CleanupExpiredCodes removes expired link codes
func (s *Service) CleanupExpiredCodes() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0
	for code, linkCode := range s.linkCodes {
		if now.After(linkCode.ExpiresAt) {
			delete(s.linkCodes, code)
			removed++
		}
	}
	return removed
}

// GetPendingLinkCodes returns all pending link codes for a user (for display)
func (s *Service) GetPendingLinkCodes(userID string) []LinkCode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var codes []LinkCode
	for _, lc := range s.linkCodes {
		if lc.UserID == userID && now.Before(lc.ExpiresAt) {
			codes = append(codes, *lc)
		}
	}
	return codes
}

// RevokeLinkCode invalidates a pending link code
func (s *Service) RevokeLinkCode(code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.linkCodes[code]; ok {
		delete(s.linkCodes, code)
		return true
	}
	return false
}

// ListUsers returns all users (for admin purposes)
func (s *Service) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

// DeleteUser removes a user and all their linked accounts
func (s *Service) DeleteUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[userID]
	if !ok {
		return ErrUserNotFound
	}

	// Remove all channel mappings
	for _, acc := range user.LinkedAccounts {
		if channelMap, ok := s.byChannel[acc.Channel]; ok {
			delete(channelMap, acc.ExternalID)
		}
	}

	// Remove any pending link codes
	for code, lc := range s.linkCodes {
		if lc.UserID == userID {
			delete(s.linkCodes, code)
		}
	}

	delete(s.users, userID)
	return nil
}

// -----------------------------------------------------------------------------
// Persistence - Save and Load user data
// -----------------------------------------------------------------------------

// persistedData is the format for saving identity data
type persistedData struct {
	Users     map[string]*User `json:"users"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// Save persists all identity data to disk
func (s *Service) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := persistedData{
		Users:     s.users,
		UpdatedAt: time.Now(),
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write atomically (write to temp, then rename)
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Load reads identity data from disk
func (s *Service) Load(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No file yet, that's OK
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	var data persistedData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Restore users
	s.users = data.Users
	if s.users == nil {
		s.users = make(map[string]*User)
	}

	// Rebuild channel lookup maps
	s.byChannel = make(map[string]map[string]string)
	for userID, user := range s.users {
		for _, acc := range user.LinkedAccounts {
			if _, ok := s.byChannel[acc.Channel]; !ok {
				s.byChannel[acc.Channel] = make(map[string]string)
			}
			s.byChannel[acc.Channel][acc.ExternalID] = userID
		}
	}

	return nil
}

// -----------------------------------------------------------------------------
// Events - notify listeners of identity changes
// -----------------------------------------------------------------------------

// EventType identifies the type of identity event
type EventType string

const (
	EventAccountLinked   EventType = "account_linked"
	EventAccountUnlinked EventType = "account_unlinked"
	EventUserCreated     EventType = "user_created"
	EventUserDeleted     EventType = "user_deleted"
)

// Event represents an identity change event
type Event struct {
	Type       EventType
	UserID     string
	Channel    string
	ExternalID string
	Timestamp  time.Time
}

// EventListener is called when identity events occur
type EventListener func(Event)

// AddEventListener registers a listener for identity events
func (s *Service) AddEventListener(listener EventListener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// notifyLinked sends an account linked event
func (s *Service) notifyLinked(user *User, channel, externalID string) {
	s.listenersMu.RLock()
	defer s.listenersMu.RUnlock()

	event := Event{
		Type:       EventAccountLinked,
		UserID:     user.ID,
		Channel:    channel,
		ExternalID: externalID,
		Timestamp:  time.Now(),
	}
	for _, listener := range s.listeners {
		go listener(event)
	}
}

// notifyUnlinked sends an account unlinked event
func (s *Service) notifyUnlinked(user *User, channel, externalID string) {
	s.listenersMu.RLock()
	defer s.listenersMu.RUnlock()

	event := Event{
		Type:       EventAccountUnlinked,
		UserID:     user.ID,
		Channel:    channel,
		ExternalID: externalID,
		Timestamp:  time.Now(),
	}
	for _, listener := range s.listeners {
		go listener(event)
	}
}
