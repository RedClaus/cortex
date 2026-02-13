// Package channels tests
package channels

import (
	"context"
	"testing"
	"time"
)

// MockChannel implements Channel for testing
type MockChannel struct {
	name           string
	enabled        bool
	started        bool
	stopped        bool
	incoming       chan *InboundMessage
	sentMessages   []*OutboundMessage
	sentApprovals  []*ApprovalRequest
	sentToolOutput []*ToolOutput
	startErr       error
	stopErr        error
}

func NewMockChannel(name string, enabled bool) *MockChannel {
	return &MockChannel{
		name:     name,
		enabled:  enabled,
		incoming: make(chan *InboundMessage, 10),
	}
}

func (m *MockChannel) Name() string {
	return m.name
}

func (m *MockChannel) Start(ctx context.Context) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.started = true
	return nil
}

func (m *MockChannel) Stop() error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopped = true
	return nil
}

func (m *MockChannel) IsEnabled() bool {
	return m.enabled
}

func (m *MockChannel) SendMessage(userID string, msg *OutboundMessage) error {
	m.sentMessages = append(m.sentMessages, msg)
	return nil
}

func (m *MockChannel) Incoming() <-chan *InboundMessage {
	return m.incoming
}

func (m *MockChannel) SendApprovalRequest(userID string, req *ApprovalRequest) error {
	m.sentApprovals = append(m.sentApprovals, req)
	return nil
}

func (m *MockChannel) SendToolOutput(userID string, output *ToolOutput) error {
	m.sentToolOutput = append(m.sentToolOutput, output)
	return nil
}

func (m *MockChannel) SupportsMedia() bool {
	return true
}

func (m *MockChannel) SupportsButtons() bool {
	return true
}

func (m *MockChannel) SupportsThreading() bool {
	return true
}

func (m *MockChannel) SupportsEditing() bool {
	return false
}

func (m *MockChannel) DismissApproval(userID, requestID string) error {
	return nil
}

func (m *MockChannel) SetApprovalCallback(callback ApprovalCallback) {}

func TestMediaType(t *testing.T) {
	tests := []struct {
		mt   MediaType
		want string
	}{
		{MediaImage, "image"},
		{MediaAudio, "audio"},
		{MediaVideo, "video"},
		{MediaDocument, "document"},
	}

	for _, tt := range tests {
		if string(tt.mt) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.mt)
		}
	}
}

func TestButtonStyle(t *testing.T) {
	tests := []struct {
		style ButtonStyle
		want  string
	}{
		{ButtonPrimary, "primary"},
		{ButtonSecondary, "secondary"},
		{ButtonDanger, "danger"},
	}

	for _, tt := range tests {
		if string(tt.style) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.style)
		}
	}
}

func TestMessageFormat(t *testing.T) {
	tests := []struct {
		format MessageFormat
		want   string
	}{
		{FormatPlain, "plain"},
		{FormatMarkdown, "markdown"},
		{FormatCode, "code"},
	}

	for _, tt := range tests {
		if string(tt.format) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, tt.format)
		}
	}
}

func TestNewRouter(t *testing.T) {
	router := NewRouter()
	if router == nil {
		t.Fatal("NewRouter() returned nil")
	}
	if router.channels == nil {
		t.Fatal("router.channels is nil")
	}
}

func TestRouter_Register(t *testing.T) {
	router := NewRouter()

	telegram := NewMockChannel("telegram", true)
	discord := NewMockChannel("discord", true)

	router.Register(telegram)
	router.Register(discord)

	if len(router.channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(router.channels))
	}
}

func TestRouter_Get(t *testing.T) {
	router := NewRouter()
	telegram := NewMockChannel("telegram", true)
	router.Register(telegram)

	// Get existing channel
	ch, ok := router.Get("telegram")
	if !ok {
		t.Fatal("Get() did not find telegram channel")
	}
	if ch.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %q", ch.Name())
	}

	// Get non-existent channel
	_, ok = router.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent channel")
	}
}

func TestRouter_StartAll(t *testing.T) {
	router := NewRouter()

	enabled := NewMockChannel("enabled", true)
	disabled := NewMockChannel("disabled", false)

	router.Register(enabled)
	router.Register(disabled)

	ctx := context.Background()
	if err := router.StartAll(ctx); err != nil {
		t.Fatalf("StartAll() failed: %v", err)
	}

	if !enabled.started {
		t.Error("enabled channel should have been started")
	}
	if disabled.started {
		t.Error("disabled channel should not have been started")
	}
}

func TestRouter_StopAll(t *testing.T) {
	router := NewRouter()

	ch1 := NewMockChannel("ch1", true)
	ch2 := NewMockChannel("ch2", true)

	router.Register(ch1)
	router.Register(ch2)

	if err := router.StopAll(); err != nil {
		t.Fatalf("StopAll() failed: %v", err)
	}

	if !ch1.stopped {
		t.Error("ch1 should have been stopped")
	}
	if !ch2.stopped {
		t.Error("ch2 should have been stopped")
	}
}

func TestInboundMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := &InboundMessage{
		ID:          "msg123",
		UserID:      "user456",
		ChannelName: "telegram",
		ChannelID:   "chat789",
		Content:     "Hello, Pinky!",
		ReplyTo:     "msg100",
		Metadata:    map[string]string{"key": "value"},
		ReceivedAt:  now,
	}

	if msg.ID != "msg123" {
		t.Error("ID not set")
	}
	if msg.UserID != "user456" {
		t.Error("UserID not set")
	}
	if msg.ChannelName != "telegram" {
		t.Error("ChannelName not set")
	}
	if msg.Content != "Hello, Pinky!" {
		t.Error("Content not set")
	}
	if msg.Metadata["key"] != "value" {
		t.Error("Metadata not set")
	}
}

func TestOutboundMessage_Fields(t *testing.T) {
	msg := &OutboundMessage{
		Content: "Hello, user!",
		Buttons: []Button{
			{ID: "btn1", Label: "OK", Style: ButtonPrimary},
		},
		ReplyTo: "msg100",
		Format:  FormatMarkdown,
	}

	if msg.Content != "Hello, user!" {
		t.Error("Content not set")
	}
	if len(msg.Buttons) != 1 {
		t.Error("Buttons not set")
	}
	if msg.Format != FormatMarkdown {
		t.Error("Format not set")
	}
}

func TestMedia_Fields(t *testing.T) {
	media := Media{
		Type:     MediaImage,
		URL:      "https://example.com/image.png",
		Data:     []byte("image data"),
		Filename: "image.png",
		MimeType: "image/png",
	}

	if media.Type != MediaImage {
		t.Error("Type not set")
	}
	if media.URL != "https://example.com/image.png" {
		t.Error("URL not set")
	}
	if media.Filename != "image.png" {
		t.Error("Filename not set")
	}
}

func TestButton_Fields(t *testing.T) {
	btn := Button{
		ID:    "btn1",
		Label: "Click me",
		Style: ButtonDanger,
	}

	if btn.ID != "btn1" {
		t.Error("ID not set")
	}
	if btn.Label != "Click me" {
		t.Error("Label not set")
	}
	if btn.Style != ButtonDanger {
		t.Error("Style not set")
	}
}

func TestApprovalRequest_Fields(t *testing.T) {
	req := ApprovalRequest{
		ID:         "req123",
		Tool:       "shell",
		Command:    "rm -rf /tmp/test",
		RiskLevel:  "high",
		WorkingDir: "/home/user",
		Reason:     "Clean up temp files",
	}

	if req.ID != "req123" {
		t.Error("ID not set")
	}
	if req.Tool != "shell" {
		t.Error("Tool not set")
	}
	if req.RiskLevel != "high" {
		t.Error("RiskLevel not set")
	}
}

func TestToolOutput_Fields(t *testing.T) {
	output := ToolOutput{
		Tool:     "shell",
		Success:  true,
		Output:   "Command completed successfully",
		Duration: 5 * time.Second,
	}

	if output.Tool != "shell" {
		t.Error("Tool not set")
	}
	if !output.Success {
		t.Error("Success not set")
	}
	if output.Duration != 5*time.Second {
		t.Error("Duration not set")
	}
}

func TestMockChannel_SendMessage(t *testing.T) {
	ch := NewMockChannel("test", true)

	msg := &OutboundMessage{Content: "Test message"}
	if err := ch.SendMessage("user1", msg); err != nil {
		t.Fatalf("SendMessage() failed: %v", err)
	}

	if len(ch.sentMessages) != 1 {
		t.Errorf("expected 1 sent message, got %d", len(ch.sentMessages))
	}
}

func TestMockChannel_SendApprovalRequest(t *testing.T) {
	ch := NewMockChannel("test", true)

	req := &ApprovalRequest{Tool: "shell"}
	if err := ch.SendApprovalRequest("user1", req); err != nil {
		t.Fatalf("SendApprovalRequest() failed: %v", err)
	}

	if len(ch.sentApprovals) != 1 {
		t.Errorf("expected 1 approval request, got %d", len(ch.sentApprovals))
	}
}

func TestMockChannel_SendToolOutput(t *testing.T) {
	ch := NewMockChannel("test", true)

	output := &ToolOutput{Tool: "shell", Success: true}
	if err := ch.SendToolOutput("user1", output); err != nil {
		t.Fatalf("SendToolOutput() failed: %v", err)
	}

	if len(ch.sentToolOutput) != 1 {
		t.Errorf("expected 1 tool output, got %d", len(ch.sentToolOutput))
	}
}

func TestMockChannel_Incoming(t *testing.T) {
	ch := NewMockChannel("test", true)

	// Send a message on the incoming channel
	go func() {
		ch.incoming <- &InboundMessage{Content: "Hello"}
	}()

	select {
	case msg := <-ch.Incoming():
		if msg.Content != "Hello" {
			t.Errorf("expected content 'Hello', got %q", msg.Content)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for incoming message")
	}
}

func TestMockChannel_Capabilities(t *testing.T) {
	ch := NewMockChannel("test", true)

	if !ch.SupportsMedia() {
		t.Error("expected SupportsMedia() to return true")
	}
	if !ch.SupportsButtons() {
		t.Error("expected SupportsButtons() to return true")
	}
	if !ch.SupportsThreading() {
		t.Error("expected SupportsThreading() to return true")
	}
}
