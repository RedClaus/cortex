// Package orchestrator provides streaming support for the orchestrator.
//
// This file adds ProcessStream functionality for Salamander integration.
// CR-020: Streaming Pipeline for Salamander TUI
package orchestrator

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// STREAMING TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// StreamChunk represents a single chunk of streaming output.
type StreamChunk struct {
	// Type of chunk: "content", "status", "tool", "error", "complete"
	Type StreamChunkType `json:"type"`

	// Content is the text content (for content type)
	Content string `json:"content,omitempty"`

	// Status for status updates
	Status string `json:"status,omitempty"`

	// ToolName for tool execution updates
	ToolName string `json:"tool_name,omitempty"`

	// ToolOutput for tool execution results
	ToolOutput string `json:"tool_output,omitempty"`

	// Error message if any
	Error string `json:"error,omitempty"`

	// IsFinal indicates this is the last chunk
	IsFinal bool `json:"is_final"`

	// Timestamp when chunk was generated
	Timestamp time.Time `json:"timestamp"`

	// Metadata for additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// StreamChunkType identifies the type of streaming chunk.
type StreamChunkType string

const (
	StreamChunkContent  StreamChunkType = "content"
	StreamChunkStatus   StreamChunkType = "status"
	StreamChunkTool     StreamChunkType = "tool"
	StreamChunkError    StreamChunkType = "error"
	StreamChunkComplete StreamChunkType = "complete"
)

// NewContentChunk creates a content chunk.
func NewContentChunk(content string) *StreamChunk {
	return &StreamChunk{
		Type:      StreamChunkContent,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewStatusChunk creates a status update chunk.
func NewStatusChunk(status string) *StreamChunk {
	return &StreamChunk{
		Type:      StreamChunkStatus,
		Status:    status,
		Timestamp: time.Now(),
	}
}

// NewToolChunk creates a tool execution chunk.
func NewToolChunk(toolName, output string) *StreamChunk {
	return &StreamChunk{
		Type:       StreamChunkTool,
		ToolName:   toolName,
		ToolOutput: output,
		Timestamp:  time.Now(),
	}
}

// NewErrorChunk creates an error chunk.
func NewErrorChunk(err string) *StreamChunk {
	return &StreamChunk{
		Type:      StreamChunkError,
		Error:     err,
		IsFinal:   true,
		Timestamp: time.Now(),
	}
}

// NewCompleteChunk creates a completion chunk.
func NewCompleteChunk(content string) *StreamChunk {
	return &StreamChunk{
		Type:      StreamChunkComplete,
		Content:   content,
		IsFinal:   true,
		Timestamp: time.Now(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STREAMING PIPELINE
// ═══════════════════════════════════════════════════════════════════════════════

// ProcessStream processes a request and streams responses.
// This is the streaming equivalent of Process() for real-time TUI updates.
func (o *Orchestrator) ProcessStream(ctx context.Context, req *Request) (<-chan *StreamChunk, error) {
	chunkCh := make(chan *StreamChunk, 100)

	go o.runStreamingPipeline(ctx, req, chunkCh)

	return chunkCh, nil
}

// runStreamingPipeline executes the pipeline with streaming output.
func (o *Orchestrator) runStreamingPipeline(ctx context.Context, req *Request, chunkCh chan<- *StreamChunk) {
	defer close(chunkCh)

	log := logging.Global()
	start := time.Now()

	// Ensure request has an ID
	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	req.Timestamp = time.Now()

	// Send initial status
	chunkCh <- NewStatusChunk("Processing request...")

	// CR-017 Phase 5: Publish RequestReceived event
	if o.eventBus != nil {
		o.eventBus.Publish(bus.NewRequestReceivedEvent(req.ID, req.Input, req.ID))
	}

	// Create pipeline state
	state := NewPipelineState(req)

	// Apply timeout
	timeout := o.config.DefaultTimeout
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Store cancel function for interrupt
	streamCtx, streamCancel := context.WithCancel(ctx)
	o.mu.Lock()
	o.currentStreamCtx = streamCtx
	o.cancelStream = streamCancel
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.cancelStream = nil
		o.currentStreamCtx = nil
		o.mu.Unlock()
	}()

	// Check for simple shell commands
	skipCognitive := false
	if o.config.SkipRoutingForSimpleCommands && isSimpleShellCommand(req.Input) {
		skipCognitive = true
		chunkCh <- NewStatusChunk("Executing command...")
	}

	// Run pipeline stages with streaming
	var stages []Stage
	if skipCognitive {
		stages = []Stage{
			&fingerprintStage{o: o},
			&routingStage{o: o},
			&knowledgeStage{o: o},
			&streamingToolStage{o: o, chunkCh: chunkCh},
			&streamingLLMStage{o: o, chunkCh: chunkCh},
		}
	} else {
		stages = []Stage{
			&fingerprintStage{o: o},
			&routingStage{o: o},
			&cognitiveStage{o: o},
			&knowledgeStage{o: o},
			&streamingToolStage{o: o, chunkCh: chunkCh},
			&streamingLLMStage{o: o, chunkCh: chunkCh},
		}
	}

	for _, stage := range stages {
		select {
		case <-streamCtx.Done():
			state.Cancelled = true
			state.AddError(streamCtx.Err())
			chunkCh <- NewErrorChunk("Request cancelled")
			return
		default:
			stageStart := time.Now()
			if err := stage.Execute(streamCtx, state); err != nil {
				state.AddError(err)
			}
			state.StageMetrics[stage.Name()] = time.Since(stageStart)
		}

		if state.Cancelled {
			break
		}
	}

	// Build final response
	resp := o.buildResponse(state)
	resp.Duration = time.Since(start)

	// Update stats
	o.updateStats(state, resp)

	// Send completion
	if state.HasErrors() {
		errMsg := ""
		for _, err := range state.Errors {
			errMsg += err.Error() + "; "
		}
		chunkCh <- NewErrorChunk(errMsg)
	} else {
		chunkCh <- NewCompleteChunk(resp.Content)
	}

	// Publish completion event
	if o.eventBus != nil {
		templateID := ""
		if state.Cognitive != nil && state.Cognitive.Template != nil {
			templateID = state.Cognitive.Template.ID
		}
		o.eventBus.Publish(bus.NewResponseGeneratedEvent(
			req.ID,
			resp.Content,
			templateID,
			resp.Duration,
			resp.Success,
		))
	}

	log.Debug("[Orchestrator] Streaming pipeline completed in %v", resp.Duration)
}

// ═══════════════════════════════════════════════════════════════════════════════
// STREAMING STAGES
// ═══════════════════════════════════════════════════════════════════════════════

// streamingToolStage executes tools with streaming output.
type streamingToolStage struct {
	o       *Orchestrator
	chunkCh chan<- *StreamChunk
}

func (s *streamingToolStage) Name() string { return "streaming-tool-execution" }

func (s *streamingToolStage) Execute(ctx context.Context, state *PipelineState) error {
	// Use the base tool stage logic
	baseStage := &toolExecutionStage{o: s.o}

	// Wrap with streaming
	for _, toolReq := range state.ToolRequests {
		s.chunkCh <- &StreamChunk{
			Type:      StreamChunkTool,
			ToolName:  string(toolReq.Tool),
			Status:    "Executing...",
			Timestamp: time.Now(),
		}
	}

	err := baseStage.Execute(ctx, state)

	// Stream tool results
	for _, result := range state.ToolResults {
		s.chunkCh <- &StreamChunk{
			Type:       StreamChunkTool,
			ToolName:   string(result.Tool),
			ToolOutput: result.Output,
			Timestamp:  time.Now(),
			Metadata: map[string]any{
				"success": result.Success,
			},
		}
	}

	return err
}

// streamingLLMStage executes LLM with streaming output.
type streamingLLMStage struct {
	o       *Orchestrator
	chunkCh chan<- *StreamChunk
}

func (s *streamingLLMStage) Name() string { return "streaming-llm" }

func (s *streamingLLMStage) Execute(ctx context.Context, state *PipelineState) error {
	// If we have an agentic mode with step callbacks, use it
	if s.o.agenticMode && s.o.agentLLM != nil {
		// Inject streaming step callback
		if state.Request.Context == nil {
			state.Request.Context = &RequestContext{}
		}

		// Save original callback
		origCallback := state.Request.Context.OnAgentStep

		// Add streaming callback
		state.Request.Context.OnAgentStep = func(event *agent.StepEvent) {
			// Stream the step based on event type
			switch event.Type {
			case agent.EventThinking:
				// Forward actual thinking content instead of generic message
				thinkingMsg := "Thinking..."
				if event.Message != "" {
					thinkingMsg = event.Message
				}
				s.chunkCh <- NewStatusChunk(thinkingMsg)
			case agent.EventToolCall:
				s.chunkCh <- &StreamChunk{
					Type:      StreamChunkTool,
					ToolName:  event.ToolName,
					Status:    "Executing...",
					Timestamp: time.Now(),
				}
			case agent.EventToolResult:
				s.chunkCh <- &StreamChunk{
					Type:       StreamChunkTool,
					ToolName:   event.ToolName,
					ToolOutput: event.Output,
					Timestamp:  time.Now(),
					Metadata: map[string]any{
						"success": event.Success,
					},
				}
			case agent.EventComplete:
				s.chunkCh <- NewContentChunk(event.Message)
			case agent.EventError:
				s.chunkCh <- NewErrorChunk(event.Error)
			default:
				if event.Message != "" {
					s.chunkCh <- NewContentChunk(event.Message)
				}
			}

			// Call original if present
			if origCallback != nil {
				origCallback(event)
			}
		}
	}

	// Execute base LLM stage
	baseLLMStage := &llmStage{o: s.o}
	err := baseLLMStage.Execute(ctx, state)

	if err == nil && state.LLMResponse != "" {
		// For non-streaming LLM, send the full response as a single chunk
		// (The step callback above handles streaming responses)
		if state.Request.Context == nil || state.Request.Context.OnAgentStep == nil {
			s.chunkCh <- NewContentChunk(state.LLMResponse)
		}
	}

	return err
}

// ═══════════════════════════════════════════════════════════════════════════════
// SALAMANDER INTEGRATION INTERFACE
// ═══════════════════════════════════════════════════════════════════════════════

// SalamanderOrchestrator is an adapter that implements DirectOrchestrator
// for use with Salamander's DirectAdapter.
type SalamanderOrchestrator struct {
	o *Orchestrator
}

// NewSalamanderOrchestrator creates a Salamander-compatible orchestrator wrapper.
func NewSalamanderOrchestrator(o *Orchestrator) *SalamanderOrchestrator {
	return &SalamanderOrchestrator{o: o}
}

// Process processes a request (implements DirectOrchestrator.Process).
func (s *SalamanderOrchestrator) Process(ctx context.Context, req *DirectRequest) (*DirectResponse, error) {
	// Convert to internal request
	internalReq := &Request{
		ID:        req.ID,
		Type:      RequestChat,
		Input:     req.Input,
		SessionID: req.ContextID,
	}

	if req.WorkingDir != "" {
		internalReq.Context = &RequestContext{
			WorkingDir: req.WorkingDir,
		}
	}

	// Process
	resp, err := s.o.Process(ctx, internalReq)
	if err != nil {
		return &DirectResponse{
			Success: false,
			Error:   err.Error(),
			TaskID:  req.TaskID,
		}, nil
	}

	// Convert response
	return &DirectResponse{
		Success:   resp.Success,
		Content:   resp.Content,
		TaskID:    req.TaskID,
		ContextID: req.ContextID,
		State:     "completed",
		IsFinal:   true,
	}, nil
}

// ProcessStream processes a request and streams responses (implements DirectOrchestrator.ProcessStream).
func (s *SalamanderOrchestrator) ProcessStream(ctx context.Context, req *DirectRequest) (<-chan *DirectResponse, error) {
	// Convert to internal request
	internalReq := &Request{
		ID:        req.ID,
		Type:      RequestChat,
		Input:     req.Input,
		SessionID: req.ContextID,
	}

	if req.WorkingDir != "" {
		internalReq.Context = &RequestContext{
			WorkingDir: req.WorkingDir,
		}
	}

	// Get streaming channel
	chunkCh, err := s.o.ProcessStream(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	// Convert chunks to DirectResponse
	respCh := make(chan *DirectResponse, 100)
	go func() {
		defer close(respCh)
		for chunk := range chunkCh {
			resp := &DirectResponse{
				TaskID:    req.TaskID,
				ContextID: req.ContextID,
				IsPartial: !chunk.IsFinal,
				IsFinal:   chunk.IsFinal,
			}

			switch chunk.Type {
			case StreamChunkContent:
				resp.Success = true
				resp.Content = chunk.Content
				resp.State = "working"
			case StreamChunkStatus:
				resp.Success = true
				resp.State = chunk.Status
			case StreamChunkTool:
				resp.Success = true
				resp.Content = chunk.ToolOutput
				resp.State = "working"
				resp.Metadata = map[string]any{"tool": chunk.ToolName}
			case StreamChunkError:
				resp.Success = false
				resp.Error = chunk.Error
				resp.State = "failed"
			case StreamChunkComplete:
				resp.Success = true
				resp.Content = chunk.Content
				resp.State = "completed"
			}

			// Goroutine-safe send: check context before blocking on channel
			select {
			case <-ctx.Done():
				return // Clean exit on cancellation
			case respCh <- resp:
				// Sent successfully
			}
		}
	}()

	return respCh, nil
}

// Interrupt cancels the current operation (implements DirectOrchestrator.Interrupt).
func (s *SalamanderOrchestrator) Interrupt(reason string) error {
	return s.o.Interrupt(reason)
}

// IsAvailable returns true if the orchestrator is ready (implements DirectOrchestrator.IsAvailable).
func (s *SalamanderOrchestrator) IsAvailable() bool {
	return true
}

// DirectRequest is a simplified request for Salamander.
// This is defined here to avoid import cycles.
type DirectRequest struct {
	ID         string
	Input      string
	ContextID  string
	TaskID     string
	WorkingDir string
	Metadata   map[string]any
}

// DirectResponse is a simplified response for Salamander.
// This is defined here to avoid import cycles.
type DirectResponse struct {
	Success   bool
	Content   string
	TaskID    string
	ContextID string
	State     string
	Error     string
	Metadata  map[string]any
	IsPartial bool
	IsFinal   bool
}
