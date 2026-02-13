package voice

import (
	"strings"
	"testing"
	"time"
)

func TestNewConversationManager_DefaultConfig(t *testing.T) {
	config := DefaultConversationConfig()
	cm := NewConversationManager(config)

	if cm.config.MaxExchanges != 10 {
		t.Errorf("expected MaxExchanges=10, got %d", cm.config.MaxExchanges)
	}
	if cm.config.InactivityTimeout != 5*time.Minute {
		t.Errorf("expected InactivityTimeout=5m, got %v", cm.config.InactivityTimeout)
	}
	if cm.ExchangeCount() != 0 {
		t.Errorf("expected empty exchanges, got %d", cm.ExchangeCount())
	}
}

func TestNewConversationManager_InvalidConfig(t *testing.T) {
	// Zero values should be replaced with defaults
	cm := NewConversationManager(ConversationConfig{})

	if cm.config.MaxExchanges != 10 {
		t.Errorf("expected default MaxExchanges=10, got %d", cm.config.MaxExchanges)
	}
	if cm.config.InactivityTimeout != 5*time.Minute {
		t.Errorf("expected default InactivityTimeout=5m, got %v", cm.config.InactivityTimeout)
	}
}

func TestConversationManager_AddExchange(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{MaxExchanges: 3})

	cm.AddExchange("Hello", "Hi there!")
	if cm.ExchangeCount() != 1 {
		t.Errorf("expected 1 exchange, got %d", cm.ExchangeCount())
	}

	cm.AddExchange("How are you?", "I'm doing well!")
	if cm.ExchangeCount() != 2 {
		t.Errorf("expected 2 exchanges, got %d", cm.ExchangeCount())
	}
}

func TestConversationManager_AddExchange_TrimsOldExchanges(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{MaxExchanges: 2})

	cm.AddExchange("First", "Response 1")
	cm.AddExchange("Second", "Response 2")
	cm.AddExchange("Third", "Response 3")

	if cm.ExchangeCount() != 2 {
		t.Errorf("expected 2 exchanges after trim, got %d", cm.ExchangeCount())
	}

	exchanges := cm.GetExchanges()
	if exchanges[0].UserText != "Second" {
		t.Errorf("expected oldest exchange to be 'Second', got '%s'", exchanges[0].UserText)
	}
	if exchanges[1].UserText != "Third" {
		t.Errorf("expected newest exchange to be 'Third', got '%s'", exchanges[1].UserText)
	}
}

func TestConversationManager_GetContext(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	// Empty context
	ctx := cm.GetContext()
	if ctx != "" {
		t.Errorf("expected empty context for no exchanges, got: %s", ctx)
	}

	cm.AddExchange("What is Go?", "Go is a programming language.")
	ctx = cm.GetContext()

	if !strings.Contains(ctx, "Previous conversation:") {
		t.Error("expected context to contain 'Previous conversation:' header")
	}
	if !strings.Contains(ctx, "What is Go?") {
		t.Error("expected context to contain user text")
	}
	if !strings.Contains(ctx, "Go is a programming language.") {
		t.Error("expected context to contain assistant text")
	}
}

func TestConversationManager_GetContext_TruncatesLongResponses(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	longResponse := strings.Repeat("a", 300)
	cm.AddExchange("Question", longResponse)

	ctx := cm.GetContext()
	if !strings.Contains(ctx, "...") {
		t.Error("expected truncated response to contain '...'")
	}
	// 200 chars + "..." = should not contain the full 300 char response
	if strings.Contains(ctx, longResponse) {
		t.Error("expected response to be truncated")
	}
}

func TestConversationManager_GetRecentContext(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{MaxExchanges: 10})

	cm.AddExchange("First", "R1")
	cm.AddExchange("Second", "R2")
	cm.AddExchange("Third", "R3")

	// Get last 2
	ctx := cm.GetRecentContext(2)

	if strings.Contains(ctx, "First") {
		t.Error("expected 'First' to be excluded from recent context")
	}
	if !strings.Contains(ctx, "Second") {
		t.Error("expected 'Second' to be in recent context")
	}
	if !strings.Contains(ctx, "Third") {
		t.Error("expected 'Third' to be in recent context")
	}
}

func TestConversationManager_GetRecentContext_RequestMoreThanAvailable(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	cm.AddExchange("Only one", "Response")

	// Request 5 but only 1 exists
	ctx := cm.GetRecentContext(5)

	if !strings.Contains(ctx, "Only one") {
		t.Error("expected the single exchange to be in context")
	}
}

func TestConversationManager_IsFollowUp_NoHistory(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	// No history means no follow-up possible
	if cm.IsFollowUp("What about that?") {
		t.Error("expected IsFollowUp=false with no history")
	}
}

func TestConversationManager_IsFollowUp_Pronouns(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())
	cm.AddExchange("Tell me about Go", "Go is great!")

	tests := []struct {
		text     string
		expected bool
	}{
		{"What is it used for?", true},
		{"Tell me more about that", true},
		{"How does this work?", true},
		{"What do they recommend?", true},
		{"Tell me about Python", false},
	}

	for _, tc := range tests {
		result := cm.IsFollowUp(tc.text)
		if result != tc.expected {
			t.Errorf("IsFollowUp(%q) = %v, want %v", tc.text, result, tc.expected)
		}
	}
}

func TestConversationManager_IsFollowUp_ContinuationStarts(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())
	cm.AddExchange("Tell me about Go", "Go is great!")

	tests := []struct {
		text     string
		expected bool
	}{
		{"And what about performance?", true},
		{"But is it fast?", true},
		{"So it's compiled?", true},
		{"Also, what about memory?", true},
		{"Then how do I install it?", true},
		{"What languages exist?", false},
	}

	for _, tc := range tests {
		result := cm.IsFollowUp(tc.text)
		if result != tc.expected {
			t.Errorf("IsFollowUp(%q) = %v, want %v", tc.text, result, tc.expected)
		}
	}
}

func TestConversationManager_IsFollowUp_ShortQuestions(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())
	cm.AddExchange("Go is compiled and statically typed", "Yes, that's correct!")

	tests := []struct {
		text     string
		expected bool
	}{
		{"Why?", true},
		{"How?", true},
		{"What?", true},
		{"Really?", true},
	}

	for _, tc := range tests {
		result := cm.IsFollowUp(tc.text)
		if result != tc.expected {
			t.Errorf("IsFollowUp(%q) = %v, want %v", tc.text, result, tc.expected)
		}
	}
}

func TestConversationManager_IsFollowUp_ExplicitReferences(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())
	cm.AddExchange("Go is great", "Indeed!")

	tests := []struct {
		text     string
		expected bool
	}{
		{"You said it's great, but why?", true},
		{"You mentioned something about Go", true},
		{"Earlier you talked about this", true},
		{"What was that last time?", true},
		{"Tell me more about Go", true},
		{"Can you explain that?", true},
	}

	for _, tc := range tests {
		result := cm.IsFollowUp(tc.text)
		if result != tc.expected {
			t.Errorf("IsFollowUp(%q) = %v, want %v", tc.text, result, tc.expected)
		}
	}
}

func TestConversationManager_Clear(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	cm.AddExchange("Hello", "Hi!")
	cm.AddExchange("How are you?", "Great!")

	if cm.ExchangeCount() != 2 {
		t.Errorf("expected 2 exchanges before clear, got %d", cm.ExchangeCount())
	}

	cm.Clear()

	if cm.ExchangeCount() != 0 {
		t.Errorf("expected 0 exchanges after clear, got %d", cm.ExchangeCount())
	}
}

func TestConversationManager_InactivityExpiry(t *testing.T) {
	// Short timeout for testing
	cm := NewConversationManager(ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 50 * time.Millisecond,
	})

	cm.AddExchange("Hello", "Hi!")

	// Should not be expired immediately
	if cm.IsExpired() {
		t.Error("expected not expired immediately after add")
	}

	ctx := cm.GetContext()
	if ctx == "" {
		t.Error("expected context before expiry")
	}

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	if !cm.IsExpired() {
		t.Error("expected expired after timeout")
	}

	// GetContext should return empty after expiry
	ctx = cm.GetContext()
	if ctx != "" {
		t.Errorf("expected empty context after expiry, got: %s", ctx)
	}

	// IsFollowUp should return false after expiry
	if cm.IsFollowUp("What about that?") {
		t.Error("expected IsFollowUp=false after expiry")
	}
}

func TestConversationManager_AddExchange_ClearsOnExpiry(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 50 * time.Millisecond,
	})

	cm.AddExchange("Old", "Response")

	// Wait for expiry
	time.Sleep(60 * time.Millisecond)

	// Add new exchange should clear old ones
	cm.AddExchange("New", "Fresh response")

	if cm.ExchangeCount() != 1 {
		t.Errorf("expected 1 exchange after expiry+add, got %d", cm.ExchangeCount())
	}

	exchanges := cm.GetExchanges()
	if exchanges[0].UserText != "New" {
		t.Errorf("expected new exchange, got '%s'", exchanges[0].UserText)
	}
}

func TestConversationManager_Touch(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 100 * time.Millisecond,
	})

	cm.AddExchange("Hello", "Hi!")
	initialActivity := cm.LastActivity()

	time.Sleep(30 * time.Millisecond)
	cm.Touch()

	if cm.LastActivity().Before(initialActivity) || cm.LastActivity().Equal(initialActivity) {
		t.Error("expected LastActivity to be updated after Touch()")
	}

	// Should not be expired because we touched it
	time.Sleep(80 * time.Millisecond)
	if cm.IsExpired() {
		t.Error("expected not expired after Touch()")
	}
}

func TestConversationManager_GetExchanges_ReturnsCopy(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	cm.AddExchange("Test", "Response")

	exchanges := cm.GetExchanges()
	exchanges[0].UserText = "Modified"

	// Original should be unchanged
	original := cm.GetExchanges()
	if original[0].UserText != "Test" {
		t.Error("expected GetExchanges to return a copy, not the original")
	}
}

func TestConversationManager_GetExchanges_AfterExpiry(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 50 * time.Millisecond,
	})

	cm.AddExchange("Test", "Response")
	time.Sleep(60 * time.Millisecond)

	exchanges := cm.GetExchanges()
	if exchanges != nil {
		t.Errorf("expected nil after expiry, got %v", exchanges)
	}
}

func TestConversationManager_Config(t *testing.T) {
	config := ConversationConfig{
		MaxExchanges:      5,
		InactivityTimeout: 10 * time.Minute,
	}
	cm := NewConversationManager(config)

	retrieved := cm.Config()
	if retrieved.MaxExchanges != 5 {
		t.Errorf("expected MaxExchanges=5, got %d", retrieved.MaxExchanges)
	}
	if retrieved.InactivityTimeout != 10*time.Minute {
		t.Errorf("expected InactivityTimeout=10m, got %v", retrieved.InactivityTimeout)
	}
}

func TestConversationManager_ConcurrentAccess(t *testing.T) {
	cm := NewConversationManager(DefaultConversationConfig())

	done := make(chan bool)

	// Writer goroutine
	go func() {
		for range 100 {
			cm.AddExchange("Question", "Answer")
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for range 100 {
			_ = cm.GetContext()
			_ = cm.IsFollowUp("What about that?")
			_ = cm.ExchangeCount()
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// If we got here without panic, concurrent access is safe
}

func TestConversationManager_EmptyExchangeExpiry(t *testing.T) {
	cm := NewConversationManager(ConversationConfig{
		MaxExchanges:      10,
		InactivityTimeout: 50 * time.Millisecond,
	})

	// Empty conversation should never be "expired"
	time.Sleep(60 * time.Millisecond)

	if cm.IsExpired() {
		t.Error("expected empty conversation to not be 'expired'")
	}
}
