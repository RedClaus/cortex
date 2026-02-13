package agent

import (
	"context"
	"testing"

	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/permissions"
	"github.com/normanking/pinky/internal/tools"
)

// mockBrain is a simple brain implementation for testing.
type mockBrain struct {
	thinkResponse *brain.ThinkResponse
	thinkErr      error
	memories      []brain.Memory
}

func (m *mockBrain) Think(ctx context.Context, req *brain.ThinkRequest) (*brain.ThinkResponse, error) {
	if m.thinkErr != nil {
		return nil, m.thinkErr
	}
	return m.thinkResponse, nil
}

func (m *mockBrain) ThinkStream(ctx context.Context, req *brain.ThinkRequest) (<-chan *brain.ThinkChunk, error) {
	return nil, nil
}

func (m *mockBrain) Remember(ctx context.Context, memory *brain.Memory) error {
	m.memories = append(m.memories, *memory)
	return nil
}

func (m *mockBrain) Recall(ctx context.Context, query string, limit int) ([]brain.Memory, error) {
	return m.memories, nil
}

func (m *mockBrain) Ping(ctx context.Context) error {
	return nil
}

func (m *mockBrain) Mode() brain.BrainMode {
	return brain.ModeEmbedded
}

func TestNew(t *testing.T) {
	mockBrn := &mockBrain{}
	registry := tools.NewRegistry()
	permSvc := permissions.NewService(permissions.TierSome)

	loop := New(Config{
		Brain:       mockBrn,
		Tools:       registry,
		Permissions: permSvc,
	})

	if loop == nil {
		t.Fatal("expected non-nil loop")
	}

	if loop.brain != mockBrn {
		t.Error("expected brain to be set")
	}

	if loop.maxToolCalls != 10 {
		t.Errorf("expected default maxToolCalls of 10, got %d", loop.maxToolCalls)
	}
}

func TestProcess_SimpleResponse(t *testing.T) {
	mockBrn := &mockBrain{
		thinkResponse: &brain.ThinkResponse{
			Content: "Hello! How can I help you?",
			Done:    true,
		},
	}

	registry := tools.NewRegistry()
	permSvc := permissions.NewService(permissions.TierUnrestricted)

	loop := New(Config{
		Brain:       mockBrn,
		Tools:       registry,
		Permissions: permSvc,
	})

	var receivedResponse string
	loop.SetResponseHandler(func(content string) {
		receivedResponse = content
	})

	resp, err := loop.Process(context.Background(), &Request{
		UserID:  "test-user",
		Content: "Hello",
		Channel: "test",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "Hello! How can I help you?" {
		t.Errorf("expected response content 'Hello! How can I help you?', got '%s'", resp.Content)
	}

	if receivedResponse != "Hello! How can I help you?" {
		t.Errorf("response handler was not called with correct content")
	}

	if len(resp.ToolsUsed) != 0 {
		t.Errorf("expected no tools used, got %d", len(resp.ToolsUsed))
	}
}

func TestProcess_WithToolCall(t *testing.T) {
	callCount := 0
	mockBrn := &mockBrain{
		thinkResponse: &brain.ThinkResponse{
			Content:   "",
			ToolCalls: []brain.ToolCall{},
		},
	}

	// First call returns a tool call, second returns final response
	mockBrn.thinkResponse = &brain.ThinkResponse{
		ToolCalls: []brain.ToolCall{
			{
				ID:     "call-1",
				Tool:   "shell",
				Input:  map[string]any{"command": "echo hello"},
				Reason: "Testing echo",
			},
		},
	}

	registry := tools.NewDefaultRegistry(nil)
	permSvc := permissions.NewService(permissions.TierUnrestricted)

	loop := New(Config{
		Brain:       mockBrn,
		Tools:       registry,
		Permissions: permSvc,
	})

	// Override Think to return different responses
	originalThink := mockBrn.thinkResponse
	loop.brain = &mockBrainSequence{
		responses: []*brain.ThinkResponse{
			originalThink,
			{Content: "Done! The command output was: hello", Done: true},
		},
		current: &callCount,
	}

	resp, err := loop.Process(context.Background(), &Request{
		UserID:     "test-user",
		Content:    "Run echo hello",
		Channel:    "test",
		WorkingDir: "/tmp",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.ToolsUsed) != 1 {
		t.Errorf("expected 1 tool used, got %d", len(resp.ToolsUsed))
	}

	if resp.ToolsUsed[0].Name != "shell" {
		t.Errorf("expected tool 'shell', got '%s'", resp.ToolsUsed[0].Name)
	}
}

// mockBrainSequence returns different responses in sequence.
type mockBrainSequence struct {
	responses []*brain.ThinkResponse
	current   *int
}

func (m *mockBrainSequence) Think(ctx context.Context, req *brain.ThinkRequest) (*brain.ThinkResponse, error) {
	idx := *m.current
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	*m.current++
	return m.responses[idx], nil
}

func (m *mockBrainSequence) ThinkStream(ctx context.Context, req *brain.ThinkRequest) (<-chan *brain.ThinkChunk, error) {
	return nil, nil
}

func (m *mockBrainSequence) Remember(ctx context.Context, memory *brain.Memory) error {
	return nil
}

func (m *mockBrainSequence) Recall(ctx context.Context, query string, limit int) ([]brain.Memory, error) {
	return nil, nil
}

func (m *mockBrainSequence) Ping(ctx context.Context) error {
	return nil
}

func (m *mockBrainSequence) Mode() brain.BrainMode {
	return brain.ModeEmbedded
}

func TestClearConversation(t *testing.T) {
	mockBrn := &mockBrain{
		thinkResponse: &brain.ThinkResponse{
			Content: "Response 1",
			Done:    true,
		},
	}

	registry := tools.NewRegistry()
	permSvc := permissions.NewService(permissions.TierSome)

	loop := New(Config{
		Brain:       mockBrn,
		Tools:       registry,
		Permissions: permSvc,
	})

	// Process a message to create conversation
	_, err := loop.Process(context.Background(), &Request{
		UserID:  "test-user",
		Content: "Hello",
		Channel: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify conversation exists
	loop.mu.RLock()
	_, exists := loop.conversations["test-user"]
	loop.mu.RUnlock()

	if !exists {
		t.Error("expected conversation to exist")
	}

	// Clear conversation
	loop.ClearConversation("test-user")

	// Verify conversation is cleared
	loop.mu.RLock()
	_, exists = loop.conversations["test-user"]
	loop.mu.RUnlock()

	if exists {
		t.Error("expected conversation to be cleared")
	}
}

func TestBuildToolSpecs(t *testing.T) {
	mockBrn := &mockBrain{}
	registry := tools.NewDefaultRegistry(nil)
	permSvc := permissions.NewService(permissions.TierSome)

	loop := New(Config{
		Brain:       mockBrn,
		Tools:       registry,
		Permissions: permSvc,
	})

	specs := loop.buildToolSpecs()

	if len(specs) == 0 {
		t.Error("expected at least one tool spec")
	}

	// Check that shell tool has command parameter
	var foundShell bool
	for _, spec := range specs {
		if spec.Name == "shell" {
			foundShell = true
			if _, ok := spec.Parameters["command"]; !ok {
				t.Error("expected shell tool to have command parameter")
			}
		}
	}

	if !foundShell {
		t.Error("expected to find shell tool in specs")
	}
}
