// Package agent implements Pinky's agentic loop with tool calling.
package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/brain"
	"github.com/normanking/pinky/internal/memory"
	"github.com/normanking/pinky/internal/permissions"
	"github.com/normanking/pinky/internal/persona"
	"github.com/normanking/pinky/internal/tools"
)

// Loop orchestrates the agent's processing cycle.
// It receives messages, reasons with the brain, executes tools
// with permission checks, and returns responses.
type Loop struct {
	brain       brain.Brain
	tools       *tools.Registry
	permissions *permissions.Service

	// Callbacks for UI integration
	onApprovalNeeded func(req *permissions.ApprovalRequest) (*permissions.ApprovalResponse, error)
	onToolStart      func(name, command string)
	onToolComplete   func(name string, output *tools.ToolOutput)
	onThinking       func(content string)
	onResponse       func(content string)

	// Session state
	mu            sync.RWMutex
	conversations map[string]*Conversation // keyed by userID
	maxToolCalls  int                       // max tool calls per turn (prevents infinite loops)
}

// Conversation tracks an ongoing conversation with a user.
type Conversation struct {
	UserID   string
	Messages []brain.Message
	Persona  *persona.Persona
}

// Config configures the agent loop.
type Config struct {
	Brain        brain.Brain
	Tools        *tools.Registry
	Permissions  *permissions.Service
	MaxToolCalls int
}

// New creates a new agent loop.
func New(cfg Config) *Loop {
	maxCalls := cfg.MaxToolCalls
	if maxCalls == 0 {
		maxCalls = 10 // sensible default
	}

	return &Loop{
		brain:         cfg.Brain,
		tools:         cfg.Tools,
		permissions:   cfg.Permissions,
		conversations: make(map[string]*Conversation),
		maxToolCalls:  maxCalls,
	}
}

// SetApprovalHandler sets the callback for requesting user approval.
func (l *Loop) SetApprovalHandler(fn func(*permissions.ApprovalRequest) (*permissions.ApprovalResponse, error)) {
	l.onApprovalNeeded = fn
}

// SetToolStartHandler sets the callback for when a tool starts executing.
func (l *Loop) SetToolStartHandler(fn func(name, command string)) {
	l.onToolStart = fn
}

// SetToolCompleteHandler sets the callback for when a tool finishes.
func (l *Loop) SetToolCompleteHandler(fn func(name string, output *tools.ToolOutput)) {
	l.onToolComplete = fn
}

// SetThinkingHandler sets the callback for streaming thinking content.
func (l *Loop) SetThinkingHandler(fn func(content string)) {
	l.onThinking = fn
}

// SetResponseHandler sets the callback for final responses.
func (l *Loop) SetResponseHandler(fn func(content string)) {
	l.onResponse = fn
}

// Request represents an incoming message to process.
type Request struct {
	UserID     string
	Content    string
	Channel    string
	Persona    *persona.Persona
	WorkingDir string
}

// Response represents the agent's response.
type Response struct {
	Content      string
	ToolsUsed    []ToolExecution
	Error        error
	TotalTokens  int
	ResponseTime time.Duration
}

// ToolExecution records a tool that was executed.
type ToolExecution struct {
	Name     string
	Command  string
	Success  bool
	Output   string
	Duration time.Duration
}

// Process handles a user message through the full agentic cycle.
func (l *Loop) Process(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	// 1. Get or create conversation
	conv := l.getOrCreateConversation(req.UserID, req.Persona)

	// 2. Add user message to history
	conv.Messages = append(conv.Messages, brain.Message{
		Role:      "user",
		Content:   req.Content,
		Timestamp: time.Now(),
	})

	// 3. Parse temporal context from user message
	temporalCtx := memory.ParseTemporalContext(req.Content, time.Now())

	// 4. Recall relevant memories with temporal awareness
	var memories []brain.Memory
	var recallErr error

	// Try to use RecallWithContext if brain supports it (EmbeddedBrain)
	if brainWithContext, ok := l.brain.(*brain.EmbeddedBrain); ok {
		memories, recallErr = brainWithContext.RecallWithContext(ctx, req.Content, brain.MemoryRecallOptions{
			UserID:      req.UserID,
			Limit:       5,
			TimeContext: temporalCtx,
		})
	} else {
		memories, recallErr = l.brain.Recall(ctx, req.Content, 5)
	}
	if recallErr != nil {
		// Non-fatal, continue without memories
		memories = nil
	}

	// Log if temporal context was detected (for debugging)
	if temporalCtx.HasTimeReference && l.onThinking != nil {
		l.onThinking(fmt.Sprintf("[Temporal context detected: %q -> %s]",
			temporalCtx.RelativeTime, temporalCtx.AbsoluteTime.Format("2006-01-02 15:04")))
	}

	// 5. Build tool specs for brain
	toolSpecs := l.buildToolSpecs()

	// 6. Create think request
	thinkReq := &brain.ThinkRequest{
		UserID:      req.UserID,
		Persona:     conv.Persona,
		Messages:    conv.Messages,
		Memories:    memories,
		Tools:       toolSpecs,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	// 7. Run the agentic loop (think -> tool calls -> think -> ...)
	response := &Response{
		ToolsUsed: make([]ToolExecution, 0),
	}

	toolCallCount := 0
	for {
		// Check for max tool calls
		if toolCallCount >= l.maxToolCalls {
			response.Content = "I've reached the maximum number of tool calls for this turn. Please try again with a simpler request."
			break
		}

		// Think
		thinkResp, err := l.brain.Think(ctx, thinkReq)
		if err != nil {
			return nil, fmt.Errorf("brain think failed: %w", err)
		}

		response.TotalTokens += thinkResp.Usage.TotalTokens

		// Notify thinking if we have reasoning content
		if thinkResp.Reasoning != "" && l.onThinking != nil {
			l.onThinking(thinkResp.Reasoning)
		}

		// If no tool calls, we have a final response
		if len(thinkResp.ToolCalls) == 0 {
			response.Content = thinkResp.Content

			// Add assistant response to conversation
			conv.Messages = append(conv.Messages, brain.Message{
				Role:      "assistant",
				Content:   thinkResp.Content,
				Timestamp: time.Now(),
			})

			// Notify response handler
			if l.onResponse != nil {
				l.onResponse(thinkResp.Content)
			}

			break
		}

		// Process tool calls
		toolResults := make([]brain.ToolResult, 0, len(thinkResp.ToolCalls))
		for _, tc := range thinkResp.ToolCalls {
			toolCallCount++

			result, exec := l.executeTool(ctx, req.UserID, req.WorkingDir, &tc)
			toolResults = append(toolResults, result)
			response.ToolsUsed = append(response.ToolsUsed, exec)
		}

		// Add assistant message with tool calls to conversation
		conv.Messages = append(conv.Messages, brain.Message{
			Role:      "assistant",
			ToolCalls: thinkResp.ToolCalls,
			Timestamp: time.Now(),
		})

		// Add tool results to conversation
		conv.Messages = append(conv.Messages, brain.Message{
			Role:        "tool",
			ToolResults: toolResults,
			Timestamp:   time.Now(),
		})

		// Update think request with new messages for next iteration
		thinkReq.Messages = conv.Messages
	}

	response.ResponseTime = time.Since(start)

	// 8. Persist: store the conversation (already done above with conv.Messages)
	// Future: store in database, update memory, etc.

	return response, nil
}

// executeTool runs a single tool with permission checks.
func (l *Loop) executeTool(ctx context.Context, userID, workingDir string, tc *brain.ToolCall) (brain.ToolResult, ToolExecution) {
	exec := ToolExecution{
		Name:    tc.Tool,
		Command: formatToolCommand(tc),
	}

	result := brain.ToolResult{
		ToolCallID: tc.ID,
	}

	// Get the tool from registry
	tool, ok := l.tools.Get(tc.Tool)
	if !ok {
		result.Success = false
		result.Error = fmt.Sprintf("unknown tool: %s", tc.Tool)
		exec.Success = false
		exec.Output = result.Error
		return result, exec
	}

	// Build tool input
	input := &tools.ToolInput{
		Command:    getCommand(tc.Input),
		Args:       tc.Input,
		WorkingDir: workingDir,
		UserID:     userID,
	}

	// Check permissions
	checkResult := l.permissions.Check(
		userID,
		tc.Tool,
		input.Command,
		workingDir,
		permissions.RiskLevel(tool.RiskLevel()),
	)

	if checkResult.Blocked {
		result.Success = false
		result.Error = fmt.Sprintf("blocked: %s", checkResult.BlockReason)
		exec.Success = false
		exec.Output = result.Error
		return result, exec
	}

	// Request approval if needed
	if checkResult.NeedsApproval {
		if l.onApprovalNeeded == nil {
			result.Success = false
			result.Error = "approval required but no handler configured"
			exec.Success = false
			exec.Output = result.Error
			return result, exec
		}

		approvalReq := l.permissions.CreateApprovalRequest(
			userID,
			tc.Tool,
			input.Command,
			workingDir,
			tc.Reason,
			permissions.RiskLevel(tool.RiskLevel()),
			tc.Input,
		)

		approvalResp, err := l.onApprovalNeeded(approvalReq)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("approval failed: %v", err)
			exec.Success = false
			exec.Output = result.Error
			return result, exec
		}

		if !approvalResp.Approved {
			result.Success = false
			result.Error = "user denied approval"
			exec.Success = false
			exec.Output = result.Error
			return result, exec
		}

		// Handle modified command
		if approvalResp.Modified != "" {
			input.Command = approvalResp.Modified
		}

		// Record approval preferences
		l.permissions.RecordApproval(userID, tc.Tool, approvalResp)
	}

	// Notify tool start
	if l.onToolStart != nil {
		l.onToolStart(tc.Tool, input.Command)
	}

	// Execute the tool
	start := time.Now()
	output, err := tool.Execute(ctx, input)
	exec.Duration = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		exec.Success = false
		exec.Output = err.Error()
	} else {
		result.Success = output.Success
		result.Output = output.Output
		if !output.Success {
			result.Error = output.Error
		}
		exec.Success = output.Success
		exec.Output = output.Output
	}

	// Notify tool complete
	if l.onToolComplete != nil {
		l.onToolComplete(tc.Tool, output)
	}

	return result, exec
}

// getOrCreateConversation returns an existing conversation or creates a new one.
func (l *Loop) getOrCreateConversation(userID string, persona *persona.Persona) *Conversation {
	l.mu.Lock()
	defer l.mu.Unlock()

	if conv, ok := l.conversations[userID]; ok {
		// Update persona if provided
		if persona != nil {
			conv.Persona = persona
		}
		return conv
	}

	conv := &Conversation{
		UserID:   userID,
		Messages: make([]brain.Message, 0),
		Persona:  persona,
	}
	l.conversations[userID] = conv
	return conv
}

// ClearConversation clears the conversation history for a user.
func (l *Loop) ClearConversation(userID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.conversations, userID)
}

// buildToolSpecs converts registered tools to brain.ToolSpec format.
func (l *Loop) buildToolSpecs() []brain.ToolSpec {
	registeredTools := l.tools.List()
	specs := make([]brain.ToolSpec, 0, len(registeredTools))

	for _, t := range registeredTools {
		spec := brain.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  make(map[string]brain.ParameterSpec),
		}

		// Add standard parameters based on tool category
		switch t.Category() {
		case tools.CategoryShell:
			spec.Parameters["command"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The shell command to execute",
				Required:    true,
			}
		case tools.CategoryFiles:
			spec.Parameters["path"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The file path",
				Required:    true,
			}
			spec.Parameters["operation"] = brain.ParameterSpec{
				Type:        "string",
				Description: "The operation (read, write, delete, etc.)",
				Required:    true,
			}
		}

		specs = append(specs, spec)
	}

	return specs
}

// getCommand extracts the primary command from tool input.
func getCommand(input map[string]any) string {
	if cmd, ok := input["command"].(string); ok {
		return cmd
	}
	return ""
}

// formatToolCommand creates a human-readable command string.
func formatToolCommand(tc *brain.ToolCall) string {
	if cmd, ok := tc.Input["command"].(string); ok {
		return cmd
	}
	return fmt.Sprintf("%s(%v)", tc.Tool, tc.Input)
}
