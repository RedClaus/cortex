// Package voice provides voice interaction management for CortexAvatar.
package voice

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
)

// Exchange represents a user-assistant conversation turn.
type Exchange struct {
	UserText      string    `json:"userText"`
	AssistantText string    `json:"assistantText"`
	Timestamp     time.Time `json:"timestamp"`
}

// ConversationConfig configures the ConversationManager behavior.
type ConversationConfig struct {
	// MaxExchanges is the maximum number of exchanges to retain (default: 10)
	MaxExchanges int
	// InactivityTimeout is the duration after which context expires (default: 5 minutes)
	InactivityTimeout time.Duration
}

// DefaultConversationConfig returns sensible defaults for conversation management.
func DefaultConversationConfig() ConversationConfig {
	return ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 5 * time.Minute,
	}
}

// ConversationManager tracks conversation context for follow-up detection.
// It stores recent exchanges and provides methods to detect contextual references.
type ConversationManager struct {
	mu            sync.RWMutex
	exchanges     []Exchange
	lastActivity  time.Time
	config        ConversationConfig
	followUpWords []string
}

// NewConversationManager creates a new ConversationManager with the given config.
func NewConversationManager(config ConversationConfig) *ConversationManager {
	if config.MaxExchanges <= 0 {
		config.MaxExchanges = 10
	}
	if config.InactivityTimeout <= 0 {
		config.InactivityTimeout = 5 * time.Minute
	}

	return &ConversationManager{
		exchanges:    make([]Exchange, 0, config.MaxExchanges),
		lastActivity: time.Now(),
		config:       config,
		followUpWords: []string{
			// Pronouns referencing previous context
			"it", "that", "this", "they", "them", "those", "these",
			// Reference words
			"again", "also", "too", "more", "another", "same",
			// Continuations
			"what about", "how about", "and", "but", "however",
			// Explicit references
			"you said", "you mentioned", "earlier", "before", "previous",
			"last time", "just now", "a moment ago",
			// Questions about prior content
			"why", "how come", "what do you mean", "can you explain",
			"tell me more", "go on", "continue",
		},
	}
}

// AddExchange records a user/assistant exchange pair.
// It automatically trims old exchanges to stay within MaxExchanges.
func (cm *ConversationManager) AddExchange(userText, assistantText string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Auto-expire if inactive
	if cm.isExpiredLocked() {
		cm.clearLocked()
	}

	exchange := Exchange{
		UserText:      userText,
		AssistantText: assistantText,
		Timestamp:     time.Now(),
	}

	cm.exchanges = append(cm.exchanges, exchange)
	cm.lastActivity = time.Now()

	// Trim to max size
	if len(cm.exchanges) > cm.config.MaxExchanges {
		cm.exchanges = cm.exchanges[len(cm.exchanges)-cm.config.MaxExchanges:]
	}
}

// GetContext returns the formatted conversation history suitable for LLM context.
// Returns empty string if context has expired or is empty.
func (cm *ConversationManager) GetContext() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Check for expiry
	if cm.isExpiredLocked() {
		return ""
	}

	if len(cm.exchanges) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Previous conversation:\n")

	for i, ex := range cm.exchanges {
		fmt.Fprintf(&sb, "[%d] User: %s\n", i+1, ex.UserText)
		// Truncate long assistant responses for context
		assistantText := ex.AssistantText
		if len(assistantText) > 200 {
			assistantText = assistantText[:200] + "..."
		}
		fmt.Fprintf(&sb, "[%d] Assistant: %s\n", i+1, assistantText)
	}

	return sb.String()
}

// GetRecentContext returns the last N exchanges as formatted context.
func (cm *ConversationManager) GetRecentContext(n int) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.isExpiredLocked() {
		return ""
	}

	if len(cm.exchanges) == 0 {
		return ""
	}

	// Get last N exchanges
	start := max(len(cm.exchanges)-n, 0)

	var sb strings.Builder
	sb.WriteString("Recent conversation:\n")

	recentExchanges := cm.exchanges[start:]
	for i, ex := range recentExchanges {
		fmt.Fprintf(&sb, "User: %s\n", ex.UserText)
		assistantText := ex.AssistantText
		if len(assistantText) > 200 {
			assistantText = assistantText[:200] + "..."
		}
		fmt.Fprintf(&sb, "Assistant: %s\n", assistantText)
		if i < len(recentExchanges)-1 {
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// IsFollowUp detects if the given text references previous conversation context.
// It checks for pronouns, reference words, and continuations that suggest
// the utterance is a follow-up to prior exchanges.
func (cm *ConversationManager) IsFollowUp(text string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// No follow-up possible without history
	if len(cm.exchanges) == 0 || cm.isExpiredLocked() {
		return false
	}

	lowerText := strings.ToLower(text)

	// Check for follow-up indicators
	for _, word := range cm.followUpWords {
		// Use word boundary matching for single words
		if len(word) <= 4 {
			// For short words like "it", "that", require word boundaries
			pattern := `\b` + regexp.QuoteMeta(word) + `\b`
			if matched, _ := regexp.MatchString(pattern, lowerText); matched {
				return true
			}
		} else {
			// For phrases, simple contains is sufficient
			if strings.Contains(lowerText, word) {
				return true
			}
		}
	}

	// Check if text starts with a continuation marker
	continuationStarts := []string{"and ", "but ", "so ", "also ", "then ", "ok ", "okay "}
	for _, start := range continuationStarts {
		if strings.HasPrefix(lowerText, start) {
			return true
		}
	}

	// Check for questions that likely reference prior context
	// e.g., "Why?" "How?" when there's recent context
	shortQuestions := []string{"why?", "how?", "what?", "really?", "yes?", "no?"}
	if slices.Contains(shortQuestions, strings.TrimSpace(lowerText)) {
		return true
	}

	return false
}

// ExchangeCount returns the number of stored exchanges.
func (cm *ConversationManager) ExchangeCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.exchanges)
}

// Clear removes all conversation history.
func (cm *ConversationManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.clearLocked()
}

// clearLocked clears exchanges without acquiring lock (caller must hold lock).
func (cm *ConversationManager) clearLocked() {
	cm.exchanges = make([]Exchange, 0, cm.config.MaxExchanges)
}

// IsExpired checks if the conversation has expired due to inactivity.
func (cm *ConversationManager) IsExpired() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.isExpiredLocked()
}

// isExpiredLocked checks expiry without acquiring lock (caller must hold lock).
func (cm *ConversationManager) isExpiredLocked() bool {
	if len(cm.exchanges) == 0 {
		return false // Nothing to expire
	}
	return time.Since(cm.lastActivity) > cm.config.InactivityTimeout
}

// LastActivity returns the timestamp of the most recent activity.
func (cm *ConversationManager) LastActivity() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastActivity
}

// GetExchanges returns a copy of all exchanges.
func (cm *ConversationManager) GetExchanges() []Exchange {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.isExpiredLocked() {
		return nil
	}

	result := make([]Exchange, len(cm.exchanges))
	copy(result, cm.exchanges)
	return result
}

// Touch updates the last activity timestamp without adding an exchange.
// Useful for keeping context alive during long operations.
func (cm *ConversationManager) Touch() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.lastActivity = time.Now()
}

// Config returns the current configuration.
func (cm *ConversationManager) Config() ConversationConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}
