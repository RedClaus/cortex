package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/pinky/internal/channels"
	"github.com/normanking/pinky/internal/identity"
)

// mockChannel implements channels.Channel for testing.
type mockChannel struct {
	name     string
	enabled  bool
	started  bool
	incoming chan *channels.InboundMessage
	sent     []*channels.OutboundMessage
}

func newMockChannel(name string) *mockChannel {
	return &mockChannel{
		name:     name,
		enabled:  true,
		incoming: make(chan *channels.InboundMessage, 10),
		sent:     make([]*channels.OutboundMessage, 0),
	}
}

func (m *mockChannel) Name() string                                      { return m.name }
func (m *mockChannel) Start(ctx context.Context) error                   { m.started = true; return nil }
func (m *mockChannel) Stop() error                                       { m.started = false; return nil }
func (m *mockChannel) IsEnabled() bool                                   { return m.enabled }
func (m *mockChannel) Incoming() <-chan *channels.InboundMessage         { return m.incoming }
func (m *mockChannel) SupportsMedia() bool                               { return false }
func (m *mockChannel) SupportsButtons() bool                             { return true }
func (m *mockChannel) SupportsThreading() bool                           { return false }
func (m *mockChannel) SendApprovalRequest(userID string, req *channels.ApprovalRequest) error {
	return nil
}
func (m *mockChannel) SendToolOutput(userID string, output *channels.ToolOutput) error {
	return nil
}
func (m *mockChannel) SendMessage(userID string, msg *channels.OutboundMessage) error {
	m.sent = append(m.sent, msg)
	return nil
}
func (m *mockChannel) DismissApproval(userID, requestID string) error {
	return nil
}
func (m *mockChannel) SetApprovalCallback(callback channels.ApprovalCallback) {}
func (m *mockChannel) SupportsEditing() bool                                  { return false }

func TestRouterRegisterChannel(t *testing.T) {
	router := New(Config{
		Identity: identity.NewService(),
	})

	ch := newMockChannel("test")
	router.RegisterChannel(ch)

	got, ok := router.GetChannel("test")
	if !ok {
		t.Fatal("expected channel to be registered")
	}
	if got.Name() != "test" {
		t.Errorf("expected channel name 'test', got %q", got.Name())
	}
}

func TestRouterStartStop(t *testing.T) {
	router := New(Config{
		Identity: identity.NewService(),
	})

	ch := newMockChannel("test")
	router.RegisterChannel(ch)

	ctx := context.Background()
	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !ch.started {
		t.Error("expected channel to be started")
	}

	if err := router.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if ch.started {
		t.Error("expected channel to be stopped")
	}
}

func TestRouterSessionManagement(t *testing.T) {
	identitySvc := identity.NewService()
	router := New(Config{
		Identity:          identitySvc,
		DefaultWorkingDir: "/tmp",
	})

	// Create a user
	user := identitySvc.GetOrCreate("telegram", "123", "testuser")

	// Get or create session (simulating internal behavior)
	router.mu.Lock()
	session := &Session{
		UserID:        user.ID,
		User:          user,
		ActiveChannel: "telegram",
		ChannelID:     "chat123",
		LastMessageAt: time.Now(),
		CreatedAt:     time.Now(),
		WorkingDir:    "/tmp",
	}
	router.sessions[user.ID] = session
	router.mu.Unlock()

	// Verify session retrieval
	got, ok := router.GetSession(user.ID)
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.ActiveChannel != "telegram" {
		t.Errorf("expected active channel 'telegram', got %q", got.ActiveChannel)
	}
	if got.WorkingDir != "/tmp" {
		t.Errorf("expected working dir '/tmp', got %q", got.WorkingDir)
	}
}

func TestRouterSessionStats(t *testing.T) {
	identitySvc := identity.NewService()
	router := New(Config{
		Identity: identitySvc,
	})

	// Add some sessions
	now := time.Now()
	router.mu.Lock()
	router.sessions["user1"] = &Session{
		UserID:        "user1",
		ActiveChannel: "telegram",
		LastMessageAt: now,
	}
	router.sessions["user2"] = &Session{
		UserID:        "user2",
		ActiveChannel: "telegram",
		LastMessageAt: now.Add(-2 * time.Hour),
	}
	router.sessions["user3"] = &Session{
		UserID:        "user3",
		ActiveChannel: "discord",
		LastMessageAt: now.Add(-25 * time.Hour),
	}
	router.mu.Unlock()

	stats := router.SessionStats()

	if stats.TotalSessions != 3 {
		t.Errorf("expected 3 total sessions, got %d", stats.TotalSessions)
	}
	if stats.ActiveLastHour != 1 {
		t.Errorf("expected 1 active in last hour, got %d", stats.ActiveLastHour)
	}
	if stats.ActiveLastDay != 2 {
		t.Errorf("expected 2 active in last day, got %d", stats.ActiveLastDay)
	}
	if stats.ByChannel["telegram"] != 2 {
		t.Errorf("expected 2 telegram sessions, got %d", stats.ByChannel["telegram"])
	}
	if stats.ByChannel["discord"] != 1 {
		t.Errorf("expected 1 discord session, got %d", stats.ByChannel["discord"])
	}
}

func TestRouterCleanupStale(t *testing.T) {
	identitySvc := identity.NewService()
	router := New(Config{
		Identity: identitySvc,
	})

	now := time.Now()
	router.mu.Lock()
	router.sessions["active"] = &Session{
		UserID:        "active",
		LastMessageAt: now,
	}
	router.sessions["stale"] = &Session{
		UserID:        "stale",
		LastMessageAt: now.Add(-48 * time.Hour),
	}
	router.mu.Unlock()

	removed := router.CleanupStale(24 * time.Hour)

	if removed != 1 {
		t.Errorf("expected 1 session removed, got %d", removed)
	}

	if _, ok := router.GetSession("active"); !ok {
		t.Error("expected active session to remain")
	}
	if _, ok := router.GetSession("stale"); ok {
		t.Error("expected stale session to be removed")
	}
}

func TestRouterDisabledChannel(t *testing.T) {
	router := New(Config{
		Identity: identity.NewService(),
	})

	ch := newMockChannel("disabled")
	ch.enabled = false
	router.RegisterChannel(ch)

	ctx := context.Background()
	if err := router.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Disabled channel should not be started
	if ch.started {
		t.Error("expected disabled channel to not be started")
	}

	router.Stop()
}

func TestExtractUsername(t *testing.T) {
	tests := []struct {
		name     string
		msg      *channels.InboundMessage
		expected string
	}{
		{
			name: "username in metadata",
			msg: &channels.InboundMessage{
				UserID:   "123",
				Metadata: map[string]string{"username": "testuser"},
			},
			expected: "testuser",
		},
		{
			name: "display_name in metadata",
			msg: &channels.InboundMessage{
				UserID:   "123",
				Metadata: map[string]string{"display_name": "Test User"},
			},
			expected: "Test User",
		},
		{
			name: "fallback to userID",
			msg: &channels.InboundMessage{
				UserID:   "123",
				Metadata: map[string]string{},
			},
			expected: "123",
		},
		{
			name: "nil metadata",
			msg: &channels.InboundMessage{
				UserID: "456",
			},
			expected: "456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractUsername(tt.msg)
			if got != tt.expected {
				t.Errorf("extractUsername() = %q, want %q", got, tt.expected)
			}
		})
	}
}
