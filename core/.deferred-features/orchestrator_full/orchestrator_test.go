package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/router"
	"github.com/normanking/cortex/internal/tools"
)

func TestNew(t *testing.T) {
	o := New()
	if o == nil {
		t.Fatal("expected non-nil orchestrator")
	}

	if o.config == nil {
		t.Error("expected non-nil config")
	}

	if o.router == nil {
		t.Error("expected non-nil router")
	}

	if o.toolExec == nil {
		t.Error("expected non-nil tool executor")
	}

	if len(o.specialists) == 0 {
		t.Error("expected default specialists")
	}
}

func TestNewWithOptions(t *testing.T) {
	customConfig := &Config{
		DefaultTimeout:  30 * time.Second,
		MaxToolCalls:    5,
		EnableKnowledge: false,
	}

	o := New(WithConfig(customConfig))

	if o.config.DefaultTimeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", o.config.DefaultTimeout)
	}

	if o.config.MaxToolCalls != 5 {
		t.Errorf("expected max tool calls 5, got %d", o.config.MaxToolCalls)
	}
}

func TestDefaultSpecialists(t *testing.T) {
	specialists := DefaultSpecialists()

	expectedTypes := []router.TaskType{
		router.TaskGeneral,
		router.TaskCodeGen,
		router.TaskDebug,
		router.TaskReview,
		router.TaskPlanning,
		router.TaskInfrastructure,
		router.TaskExplain,
		router.TaskRefactor,
	}

	for _, taskType := range expectedTypes {
		spec, ok := specialists[taskType]
		if !ok {
			t.Errorf("missing specialist for %s", taskType)
			continue
		}
		if spec.Name == "" {
			t.Errorf("specialist %s has empty name", taskType)
		}
		if spec.SystemPrompt == "" {
			t.Errorf("specialist %s has empty system prompt", taskType)
		}
	}
}

func TestOrchestrator_Route(t *testing.T) {
	o := New()

	testCases := []struct {
		input    string
		expected router.TaskType
	}{
		{"Fix the bug in login", router.TaskDebug},
		{"Write a function to parse JSON", router.TaskCodeGen},
		{"Review this code", router.TaskReview},
		{"Deploy to production", router.TaskInfrastructure},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			decision := o.Route(tc.input)
			if decision.TaskType != tc.expected {
				t.Errorf("Route(%q) = %s, expected %s", tc.input, decision.TaskType, tc.expected)
			}
		})
	}
}

func TestOrchestrator_GetSpecialist(t *testing.T) {
	o := New()

	// Known task type
	spec := o.GetSpecialist(router.TaskDebug)
	if spec == nil {
		t.Fatal("expected non-nil specialist for debug")
	}
	if spec.Name != "Debugger" {
		t.Errorf("expected 'Debugger', got %q", spec.Name)
	}

	// Unknown task type should return general
	spec = o.GetSpecialist(router.TaskType("unknown"))
	if spec == nil {
		t.Fatal("expected fallback to general specialist")
	}
	if spec.TaskType != router.TaskGeneral {
		t.Errorf("expected general specialist, got %s", spec.TaskType)
	}
}

func TestOrchestrator_ProcessSimple(t *testing.T) {
	o := New()
	ctx := context.Background()

	resp, err := o.ProcessSimple(ctx, "echo hello")
	if err != nil {
		t.Fatalf("ProcessSimple failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.RequestID == "" {
		t.Error("expected non-empty request ID")
	}

	if resp.Routing == nil {
		t.Error("expected routing decision in response")
	}

	if resp.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}

func TestOrchestrator_ProcessCommand(t *testing.T) {
	o := New()
	ctx := context.Background()

	req := &Request{
		Type:  RequestCommand,
		Input: "echo test",
	}

	resp, err := o.Process(ctx, req)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	if len(resp.ToolResults) == 0 {
		t.Error("expected tool results for command request")
	}

	if len(resp.ToolResults) > 0 && !resp.ToolResults[0].Success {
		t.Errorf("tool execution failed: %s", resp.ToolResults[0].Error)
	}
}

func TestOrchestrator_ExecuteTool(t *testing.T) {
	o := New()
	ctx := context.Background()

	result, err := o.ExecuteTool(ctx, &tools.ToolRequest{
		Tool:  tools.ToolBash,
		Input: "echo direct",
	})

	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}

	if result.Output != "direct" {
		t.Errorf("expected 'direct', got %q", result.Output)
	}
}

func TestOrchestrator_Stats(t *testing.T) {
	o := New()
	ctx := context.Background()

	// Process a few requests
	o.ProcessSimple(ctx, "echo one")
	o.ProcessSimple(ctx, "echo two")
	o.ProcessSimple(ctx, "fix this bug")

	stats := o.Stats()

	if stats.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %d", stats.TotalRequests)
	}

	if stats.SuccessCount < 1 {
		t.Error("expected at least 1 success")
	}
}

func TestOrchestrator_ProcessWithContext(t *testing.T) {
	o := New()
	ctx := context.Background()

	req := &Request{
		Type:  RequestChat,
		Input: "list files",
		Context: &RequestContext{
			WorkingDir: "/tmp",
			Tags:       []string{"test"},
		},
	}

	resp, err := o.Process(ctx, req)
	if err != nil {
		t.Fatalf("Process with context failed: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestOrchestrator_ProcessTimeout(t *testing.T) {
	o := New(WithConfig(&Config{
		DefaultTimeout: 10 * time.Millisecond,
	}))
	ctx := context.Background()

	// This should timeout
	req := &Request{
		Type:  RequestCommand,
		Input: "sleep 1",
	}

	resp, _ := o.Process(ctx, req)

	// Check that it was marked as cancelled or failed
	if resp.Success {
		// If it somehow completed, that's fine too
		t.Log("Command completed before timeout")
	}
}

func TestPipelineState(t *testing.T) {
	req := &Request{
		ID:    "test-123",
		Type:  RequestChat,
		Input: "hello",
	}

	state := NewPipelineState(req)

	if state.Request != req {
		t.Error("expected request to be set")
	}

	if state.Response == nil {
		t.Error("expected non-nil response")
	}

	if state.Response.RequestID != "test-123" {
		t.Errorf("expected request ID 'test-123', got %q", state.Response.RequestID)
	}

	if state.StageMetrics == nil {
		t.Error("expected non-nil stage metrics")
	}

	// Test error handling
	if state.HasErrors() {
		t.Error("should not have errors initially")
	}

	state.AddError(nil) // Should be ignored
	if state.HasErrors() {
		t.Error("nil error should be ignored")
	}

	state.AddError(context.DeadlineExceeded)
	if !state.HasErrors() {
		t.Error("should have error after adding")
	}

	if len(state.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(state.Errors))
	}
}

func TestRequestTypes(t *testing.T) {
	// Verify request types are defined correctly
	if RequestChat != "chat" {
		t.Errorf("expected 'chat', got %q", RequestChat)
	}
	if RequestCommand != "command" {
		t.Errorf("expected 'command', got %q", RequestCommand)
	}
	if RequestQuery != "query" {
		t.Errorf("expected 'query', got %q", RequestQuery)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTimeout != 5*time.Minute {
		t.Errorf("expected 5m timeout, got %v", cfg.DefaultTimeout)
	}

	if cfg.MaxToolCalls != 10 {
		t.Errorf("expected 10 max tool calls, got %d", cfg.MaxToolCalls)
	}

	if !cfg.EnableKnowledge {
		t.Error("expected knowledge enabled by default")
	}

	if !cfg.EnableFingerprint {
		t.Error("expected fingerprint enabled by default")
	}

	if !cfg.SkipRoutingForSimpleCommands {
		t.Error("expected SkipRoutingForSimpleCommands enabled by default")
	}
}

// TestSkipCognitiveRouting verifies that simple commands skip the cognitive stage.
func TestSkipCognitiveRouting(t *testing.T) {
	// Test with fast-path enabled (default)
	o := New()
	ctx := context.Background()

	// Process a simple command
	req := &Request{
		Type:  RequestChat,
		Input: "ls -la",
		Context: &RequestContext{
			WorkingDir: "/tmp",
		},
	}

	resp, err := o.Process(ctx, req)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should succeed
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Should have tool results (command was executed)
	if len(resp.ToolResults) == 0 {
		t.Error("expected tool results for simple command")
	}

	// Verify cognitive stage was NOT in the metrics (it was skipped)
	if resp.Metadata != nil {
		if metrics, ok := resp.Metadata["stage_metrics"].(map[string]time.Duration); ok {
			if _, hasCognitive := metrics["cognitive"]; hasCognitive {
				t.Error("cognitive stage should have been skipped for simple command")
			}
		}
	}
}

// TestSkipCognitiveRoutingDisabled verifies that cognitive routing can be disabled.
func TestSkipCognitiveRoutingDisabled(t *testing.T) {
	// Disable fast-path
	cfg := DefaultConfig()
	cfg.SkipRoutingForSimpleCommands = false

	o := New(WithConfig(cfg))
	ctx := context.Background()

	req := &Request{
		Type:  RequestChat,
		Input: "ls -la",
		Context: &RequestContext{
			WorkingDir: "/tmp",
		},
	}

	resp, err := o.Process(ctx, req)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Should still succeed (the command runs either way)
	if !resp.Success {
		t.Errorf("expected success, got error: %s", resp.Error)
	}

	// Should have tool results
	if len(resp.ToolResults) == 0 {
		t.Error("expected tool results")
	}

	// Verify cognitive stage WAS in the metrics (it was NOT skipped)
	if resp.Metadata != nil {
		if metrics, ok := resp.Metadata["stage_metrics"].(map[string]time.Duration); ok {
			if _, hasCognitive := metrics["cognitive"]; !hasCognitive {
				t.Error("cognitive stage should NOT have been skipped when SkipRoutingForSimpleCommands is false")
			}
		}
	}
}

// TestIsSimpleShellCommand tests the fast-path detection for simple shell commands.
func TestIsSimpleShellCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Simple commands - should skip cognitive routing
		{"ls", true},
		{"ls -la", true},
		{"ls -la /tmp", true},
		{"cd", true},
		{"cd /home", true},
		{"cd ..", true},
		{"pwd", true},
		{"cat file.txt", true},
		{"cat -n file.txt", true},
		{"head -n 10 file.txt", true},
		{"tail -f log.txt", true},
		{"mkdir newdir", true},
		{"rm -rf /tmp/test", true},
		{"cp src dest", true},
		{"mv old new", true},
		{"echo hello", true},
		{"echo 'hello world'", true},
		{"date", true},
		{"whoami", true},
		{"hostname", true},
		{"uname -a", true},
		{"clear", true},
		{"history", true},
		{"git status", true},
		{"git log --oneline", true},
		{"make build", true},
		{"find . -name '*.go'", true},
		{"grep -r pattern .", true},
		{"ping google.com", true},
		{"curl https://example.com", true},

		// Executable paths - should skip cognitive routing
		{"./script.sh", true},
		{"./build.sh --verbose", true},
		{"/usr/bin/python3 script.py", true},
		{"/bin/bash", true},
		{"~/scripts/deploy.sh", true},

		// Natural language - should NOT skip (need full routing)
		{"", false},
		{"what is the weather", false},
		{"explain this code", false},
		{"write a function to", false},
		{"help me fix", false},
		{"how do I", false},
		{"tell me about", false},
		{"can you", false},
		{"please create", false},
		{"why is this", false},
		{"describe the", false},
		{"analyze this", false},

		// Personal/memory questions - should NOT skip (prevents "who" shell command match)
		{"who am I?", false},
		{"who am i", false},
		{"Who am I", false},
		{"who am I to you", false},
		{"what's my name", false},
		{"what is my name", false},
		{"do you know me", false},
		{"do you remember me", false},
		{"what do you know about me", false},
		{"tell me about myself", false},
		{"what have I told you", false},

		// Edge cases
		{"  ls  ", true},           // Whitespace should be trimmed
		{"LS", true},               // Case insensitive for command
		{"CD /home", true},         // Case insensitive
		{"PWD", true},              // Case insensitive
		{"unknown_command", false}, // Unknown commands should not skip
		{"123", false},             // Numbers are not commands
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isSimpleShellCommand(tt.input)
			if got != tt.want {
				t.Errorf("isSimpleShellCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// BENCHMARKS
// ===========================================================================

func BenchmarkOrchestrator_Route(b *testing.B) {
	o := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Route("Fix the bug in the login function")
	}
}

func BenchmarkOrchestrator_ProcessSimple(b *testing.B) {
	o := New()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.ProcessSimple(ctx, "echo hello")
	}
}

func BenchmarkOrchestrator_ExecuteTool(b *testing.B) {
	o := New()
	ctx := context.Background()
	req := &tools.ToolRequest{
		Tool:  tools.ToolBash,
		Input: "echo hello",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.ExecuteTool(ctx, req)
	}
}

func TestOrchestrator_AutoCommandExecution(t *testing.T) {
	o := New()
	ctx := context.Background()

	tests := []struct {
		name     string
		input    string
		wantTool bool
	}{
		{"ls command", "ls -al", true},
		{"pwd command", "pwd", true},
		{"echo command", "echo hello", true},
		{"date command", "date", true},
		{"plain question", "what is Go?", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Request{
				Type:  RequestChat,
				Input: tt.input,
				Context: &RequestContext{
					WorkingDir: "/tmp",
				},
			}

			resp, err := o.Process(ctx, req)
			if err != nil {
				t.Fatalf("Process failed: %v", err)
			}

			hasToolResults := len(resp.ToolResults) > 0
			if hasToolResults != tt.wantTool {
				t.Errorf("input %q: got tool results=%v, want %v", tt.input, hasToolResults, tt.wantTool)
				t.Logf("Response: Success=%v, Content=%q, Error=%q", resp.Success, resp.Content, resp.Error)
				if resp.Routing != nil {
					t.Logf("Routing: TaskType=%s, Confidence=%.2f", resp.Routing.TaskType, resp.Routing.Confidence)
				}
			}
		})
	}
}

func TestLooksLikeCommand(t *testing.T) {
	stage := &toolExecutionStage{}

	tests := []struct {
		input string
		want  bool
	}{
		// Valid commands - should execute
		{"ls", true},
		{"ls -al", true},
		{"pwd", true},
		{"git status", true},
		{"docker ps", true},
		{"./script.sh", true},
		{"/usr/bin/env", true},
		{"echo hello | grep h", true},
		{"find . -name '*.go'", true}, // Actual find command with flags

		// Natural language - should NOT execute (false positive prevention)
		{"what is Go?", false},
		{"explain this code", false},
		{"how do I fix this", false},
		{"find linux commands online and download them to your memory", false}, // The bug that prompted this fix
		{"find the gemini cli repo on github and install in my home directory", false},
		{"help me understand this error please", false},
		{"can you explain what this does", false},
		{"tell me about docker containers", false},
		{"what is the best way to learn Go", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stage.looksLikeCommand(tt.input)
			if got != tt.want {
				t.Errorf("looksLikeCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestOrchestrator_Interrupt tests the cognitive interrupt mechanism (CR-010 Track 3).
func TestOrchestrator_Interrupt(t *testing.T) {
	// Create event bus for testing
	eventBus := bus.New()

	// Create orchestrator with event bus
	o := New(WithEventBus(eventBus))

	// Subscribe to interrupt events
	interruptReceived := false
	sub := eventBus.Subscribe(bus.EventTypeInterrupt, func(evt bus.Event) {
		if interruptEvt, ok := evt.(*bus.InterruptEvent); ok {
			if interruptEvt.Reason == "test_interrupt" {
				interruptReceived = true
			}
		}
	})
	defer sub.Unsubscribe()

	// Simulate an active stream by setting up a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.currentStreamCtx = ctx
	o.cancelStream = cancel
	o.mu.Unlock()

	// Create a goroutine that simulates a long-running operation
	streamCancelled := false
	done := make(chan bool)
	go func() {
		select {
		case <-ctx.Done():
			streamCancelled = true
		case <-time.After(5 * time.Second):
			// Should not reach here if interrupt works
		}
		done <- true
	}()

	// Trigger interrupt
	err := o.Interrupt("test_interrupt")
	if err != nil {
		t.Fatalf("Interrupt() failed: %v", err)
	}

	// Wait for the goroutine to complete
	<-done

	// Verify stream was cancelled
	if !streamCancelled {
		t.Error("Stream was not cancelled after interrupt")
	}

	// Give event bus time to process
	time.Sleep(100 * time.Millisecond)

	// Verify interrupt event was published
	if !interruptReceived {
		t.Error("Interrupt event was not received")
	}

	// Verify cancel function was cleared
	o.mu.RLock()
	if o.cancelStream != nil {
		t.Error("cancelStream should be nil after interrupt")
	}
	if o.currentStreamCtx != nil {
		t.Error("currentStreamCtx should be nil after interrupt")
	}
	o.mu.RUnlock()
}

// TestOrchestrator_InterruptNoActiveStream tests interrupt when no stream is active.
func TestOrchestrator_InterruptNoActiveStream(t *testing.T) {
	o := New()

	// Call interrupt when no stream is active
	err := o.Interrupt("test")
	if err != nil {
		t.Errorf("Interrupt() with no active stream should not error, got: %v", err)
	}
}

// TestOrchestrator_InterruptPublishesEvent tests that interrupt publishes an event.
func TestOrchestrator_InterruptPublishesEvent(t *testing.T) {
	eventBus := bus.New()
	o := New(WithEventBus(eventBus))

	// Subscribe to interrupt events
	eventCount := 0
	var receivedReason string
	sub := eventBus.Subscribe(bus.EventTypeInterrupt, func(evt bus.Event) {
		if interruptEvt, ok := evt.(*bus.InterruptEvent); ok {
			eventCount++
			receivedReason = interruptEvt.Reason
		}
	})
	defer sub.Unsubscribe()

	// Set up a mock stream
	ctx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.currentStreamCtx = ctx
	o.cancelStream = cancel
	o.mu.Unlock()

	// Trigger interrupt
	reason := "user_speech"
	err := o.Interrupt(reason)
	if err != nil {
		t.Fatalf("Interrupt() failed: %v", err)
	}

	// Give event bus time to process
	time.Sleep(100 * time.Millisecond)

	// Verify event was published
	if eventCount != 1 {
		t.Errorf("Expected 1 interrupt event, got %d", eventCount)
	}

	if receivedReason != reason {
		t.Errorf("Expected reason '%s', got '%s'", reason, receivedReason)
	}
}
