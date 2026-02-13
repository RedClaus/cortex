// Package voice provides voice-related types and utilities for Cortex.
// conversation_context.go tracks voice session state for natural conversations (CR-012-A).
package voice

import (
	"strings"
	"time"
)

// VoiceConversationContext tracks voice session state for natural conversation flow.
type VoiceConversationContext struct {
	// Session tracking
	SessionStart time.Time
	TurnCount    int

	// Recent context for natural references
	LastCommand   string
	LastResult    string
	LastDirectory string
	RecentFiles   []string // Last 5 files mentioned
	RecentErrors  []string // Last 3 errors for debugging context

	// User adaptation
	PreferredVerbosity string // Detected from interaction patterns: "terse", "normal", "verbose"
	TechnicalLevel     string // "beginner", "intermediate", "expert"

	// Conversation flow
	PendingConfirmation bool
	PendingAction       string
	FollowUpSuggested   string
}

// NewVoiceConversationContext creates a fresh conversation context.
func NewVoiceConversationContext() *VoiceConversationContext {
	return &VoiceConversationContext{
		SessionStart:       time.Now(),
		PreferredVerbosity: "normal",
		TechnicalLevel:     "intermediate",
		RecentFiles:        make([]string, 0, 5),
		RecentErrors:       make([]string, 0, 3),
	}
}

// BuildContextInjection creates a context string to prepend to user message.
// This helps the LLM understand the current state without explicit user mention.
func (c *VoiceConversationContext) BuildContextInjection() string {
	var parts []string

	// Time context (for greetings, urgency awareness)
	hour := time.Now().Hour()
	if c.TurnCount == 0 {
		if hour < 12 {
			parts = append(parts, "[Time: Morning]")
		} else if hour < 17 {
			parts = append(parts, "[Time: Afternoon]")
		} else {
			parts = append(parts, "[Time: Evening]")
		}
	}

	// Directory context
	if c.LastDirectory != "" {
		parts = append(parts, "[CWD: "+c.LastDirectory+"]")
	}

	// Recent command context (for follow-ups)
	if c.LastCommand != "" && c.TurnCount > 0 {
		parts = append(parts, "[Last command: "+c.LastCommand+"]")
	}

	// Pending confirmation
	if c.PendingConfirmation {
		parts = append(parts, "[Awaiting confirmation for: "+c.PendingAction+"]")
	}

	// Error context (helps with debugging follow-ups)
	if len(c.RecentErrors) > 0 {
		parts = append(parts, "[Recent error: "+c.RecentErrors[len(c.RecentErrors)-1]+"]")
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ") + "\n"
}

// UpdateFromCommand updates context after command execution.
func (c *VoiceConversationContext) UpdateFromCommand(cmd, result string, success bool) {
	c.TurnCount++
	c.LastCommand = cmd
	c.LastResult = truncateStr(result, 200)

	if !success && result != "" {
		c.RecentErrors = append(c.RecentErrors, truncateStr(result, 100))
		if len(c.RecentErrors) > 3 {
			c.RecentErrors = c.RecentErrors[1:]
		}
	}
}

// UpdateDirectory updates the current working directory context.
func (c *VoiceConversationContext) UpdateDirectory(dir string) {
	c.LastDirectory = dir
}

// AddRecentFile adds a file to the recent files list.
func (c *VoiceConversationContext) AddRecentFile(file string) {
	// Avoid duplicates
	for _, f := range c.RecentFiles {
		if f == file {
			return
		}
	}

	c.RecentFiles = append(c.RecentFiles, file)
	if len(c.RecentFiles) > 5 {
		c.RecentFiles = c.RecentFiles[1:]
	}
}

// SetPendingConfirmation marks that we're waiting for user confirmation.
func (c *VoiceConversationContext) SetPendingConfirmation(action string) {
	c.PendingConfirmation = true
	c.PendingAction = action
}

// ClearPendingConfirmation clears the pending confirmation state.
func (c *VoiceConversationContext) ClearPendingConfirmation() {
	c.PendingConfirmation = false
	c.PendingAction = ""
}

// SuggestFollowUp generates contextual follow-up suggestion based on last command.
func (c *VoiceConversationContext) SuggestFollowUp() string {
	cmd := strings.ToLower(c.LastCommand)

	switch {
	case strings.Contains(cmd, "git status"):
		return "Want me to show the diff or commit these changes?"
	case strings.Contains(cmd, "git diff"):
		return "Ready to commit?"
	case strings.Contains(cmd, "git log"):
		return "Want to see details for any of these commits?"
	case strings.Contains(cmd, "docker ps"):
		return "Want to see the logs for any of these containers?"
	case strings.Contains(cmd, "ls") || strings.Contains(cmd, "find"):
		return "Want me to open or edit any of these?"
	case strings.HasPrefix(cmd, "cd "):
		return "" // No follow-up needed for directory changes
	case strings.Contains(cmd, "make build") || strings.Contains(cmd, "go build"):
		return "Build complete. Want me to run it?"
	case strings.Contains(cmd, "npm install") || strings.Contains(cmd, "pip install"):
		return "Dependencies installed. Ready to run?"
	case strings.Contains(cmd, "test"):
		if c.LastResult != "" && strings.Contains(strings.ToLower(c.LastResult), "fail") {
			return "Some tests failed. Want me to show the details?"
		}
		return ""
	default:
		return ""
	}
}

// GetSessionDuration returns how long this voice session has been active.
func (c *VoiceConversationContext) GetSessionDuration() time.Duration {
	return time.Since(c.SessionStart)
}

// IsNewSession returns true if this is a fresh session (no commands yet).
func (c *VoiceConversationContext) IsNewSession() bool {
	return c.TurnCount == 0
}

// DetectVerbosityPreference analyzes user input patterns to adjust verbosity.
func (c *VoiceConversationContext) DetectVerbosityPreference(userInput string) {
	input := strings.ToLower(userInput)

	// Terse indicators
	if strings.Contains(input, "just") ||
		strings.Contains(input, "quickly") ||
		strings.Contains(input, "brief") ||
		len(userInput) < 20 {
		c.PreferredVerbosity = "terse"
		return
	}

	// Verbose indicators
	if strings.Contains(input, "explain") ||
		strings.Contains(input, "why") ||
		strings.Contains(input, "how does") ||
		strings.Contains(input, "tell me more") {
		c.PreferredVerbosity = "verbose"
		return
	}

	// Default to normal
	c.PreferredVerbosity = "normal"
}

// truncateStr truncates a string to maxLen characters with ellipsis.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
