package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AGENT
// ═══════════════════════════════════════════════════════════════════════════════

// LLMProvider interface for the agent to call the LLM.
type LLMProvider interface {
	Chat(ctx context.Context, messages []ChatMessage, systemPrompt string) (string, error)
}

// StreamingLLMProvider extends LLMProvider with streaming support.
type StreamingLLMProvider interface {
	LLMProvider
	// ChatStream is like Chat but calls onToken for each token as it's generated.
	ChatStream(ctx context.Context, messages []ChatMessage, systemPrompt string, onToken func(token string)) (string, error)
}

// ChatMessage represents a message for the LLM.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StepCallback is called for each step the agent takes.
// This enables streaming output to show reasoning in real-time.
type StepCallback func(event *StepEvent)

// StepEvent represents an event during agent execution.
type StepEvent struct {
	Type      StepEventType `json:"type"`
	Step      int           `json:"step"`
	Message   string        `json:"message"`
	ToolName  string        `json:"tool_name,omitempty"`
	ToolInput string        `json:"tool_input,omitempty"`
	Output    string        `json:"output,omitempty"`
	Success   bool          `json:"success,omitempty"`
	Error     string        `json:"error,omitempty"`
}

// StepEventType identifies the type of step event.
type StepEventType string

const (
	EventThinking   StepEventType = "thinking"    // Agent is reasoning
	EventStreaming  StepEventType = "streaming"   // Streaming token from LLM
	EventToolCall   StepEventType = "tool_call"   // Agent is calling a tool
	EventToolResult StepEventType = "tool_result" // Tool returned a result
	EventComplete   StepEventType = "complete"    // Agent finished
	EventError      StepEventType = "error"       // Error occurred
	EventLoopExit   StepEventType = "loop_exit"   // Early exit due to detected loop
	EventCheckpoint StepEventType = "checkpoint"  // Supervised mode checkpoint
)

// ═══════════════════════════════════════════════════════════════════════════════
// LOOP DETECTION (CR-024: Efficiency for small local models)
// Prevents small models from spinning on repeated tool calls without progress
// ═══════════════════════════════════════════════════════════════════════════════

// loopDetector tracks tool calls to detect spinning/futile loops.
// Small local models (3B params) often get stuck calling the same tools
// repeatedly or calling tools that return empty results.
type loopDetector struct {
	recentCalls  []string // Ring buffer of recent tool call signatures
	emptyResults int      // Count of consecutive empty/failed results
	maxRecent    int      // Size of ring buffer (default: 5)
	log          *logging.Logger
}

// newLoopDetector creates a new loop detector.
func newLoopDetector(log *logging.Logger) *loopDetector {
	return &loopDetector{
		recentCalls:  make([]string, 0, 5),
		emptyResults: 0,
		maxRecent:    5,
		log:          log,
	}
}

// recordCall records a tool call signature for duplicate detection.
func (d *loopDetector) recordCall(toolName string, params map[string]string) string {
	// Create a signature for the tool call
	paramsJSON, _ := json.Marshal(params)
	signature := fmt.Sprintf("%s:%s", toolName, string(paramsJSON))

	// Add to ring buffer
	if len(d.recentCalls) >= d.maxRecent {
		d.recentCalls = d.recentCalls[1:] // Remove oldest
	}
	d.recentCalls = append(d.recentCalls, signature)

	return signature
}

// recordResult records the result quality of a tool call.
func (d *loopDetector) recordResult(result *ToolResult) {
	if !result.Success || d.isEmptyResult(result) {
		d.emptyResults++
	} else {
		d.emptyResults = 0 // Reset on successful result with content
	}
}

// isEmptyResult checks if a tool result is effectively empty.
func (d *loopDetector) isEmptyResult(result *ToolResult) bool {
	if result.Output == "" {
		return true
	}
	// Check for common "not found" patterns
	lower := strings.ToLower(result.Output)
	emptyPatterns := []string{
		"no files found",
		"not found",
		"no results",
		"no matches",
		"empty",
		"0 results",
		"nothing found",
	}
	for _, pattern := range emptyPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	// Very short outputs are often useless
	return len(strings.TrimSpace(result.Output)) < 10
}

// shouldTerminate returns true if loop conditions are detected.
// Returns (shouldTerminate, reason)
func (d *loopDetector) shouldTerminate(currentSignature string) (bool, string) {
	// Check for repeated identical call
	duplicateCount := 0
	for _, sig := range d.recentCalls {
		if sig == currentSignature {
			duplicateCount++
		}
	}
	if duplicateCount >= 2 {
		d.log.Warn("[LoopDetector] Repeated identical tool call detected: %s (count: %d)", currentSignature[:min(60, len(currentSignature))], duplicateCount)
		return true, fmt.Sprintf("Repeated identical tool call (%s) detected %d times - stopping to avoid infinite loop", extractToolName(currentSignature), duplicateCount)
	}

	// Check for accumulating empty results
	if d.emptyResults >= 3 {
		d.log.Warn("[LoopDetector] %d consecutive empty/failed tool results - model is spinning", d.emptyResults)
		return true, fmt.Sprintf("Tool calls returning no useful results (%d consecutive failures) - stopping to provide direct answer", d.emptyResults)
	}

	return false, ""
}

// extractToolName extracts the tool name from a signature
func extractToolName(signature string) string {
	if idx := strings.Index(signature, ":"); idx > 0 {
		return signature[:idx]
	}
	return signature
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Agent orchestrates multi-step tool execution.
type Agent struct {
	llm                LLMProvider
	executor           *Executor
	workingDir         string
	maxSteps           int
	log                *logging.Logger
	onStep             StepCallback       // Callback for streaming output
	knowledge          *KnowledgeContext  // Optional knowledge context
	unrestrictedMode   bool               // Disables safety restrictions
	userContext        string             // User memory context (name, facts, preferences)
	personaIdentity    string             // Persona identity string (e.g., "You are Hannah...")
	modelName          string             // Model name for tier selection (e.g., "llama3.2:3b")
	supervisedConfig   SupervisedConfig   // Supervised agentic mode configuration
	checkpointHandler  CheckpointHandler  // Handler for supervised checkpoints
	pendingGuidance    string             // User guidance from checkpoint to inject
}

// Config configures the agent.
type Config struct {
	WorkingDir        string
	MaxSteps          int                  // Maximum tool execution steps (default: auto-detected from model)
	OnStep            StepCallback         // Callback for each step (enables streaming)
	Knowledge         *KnowledgeContext    // Optional knowledge context for enhanced responses
	UnrestrictedMode  bool                 // Disables safety restrictions in system prompt
	MemoryTools       MemoryToolsInterface // Optional memory tools for MemGPT-style operations
	MemoryUserID      string               // User ID for memory operations (default: "default-user")
	UserContext       string               // Pre-built user memory context to inject into prompt
	PersonaIdentity   string               // Persona identity string (replaces default "You are Cortex...")
	ModelName         string               // Model name for auto-detecting maxSteps (e.g., "llama3.2:3b")
	SupervisedConfig  *SupervisedConfig    // Supervised agentic mode configuration (nil = use defaults)
	CheckpointHandler CheckpointHandler    // Handler for supervised checkpoints (required for supervised mode)
}

// MaxStepsForModel returns the appropriate maxSteps based on model size.
// Small models (< 4B params) get fewer steps to fail fast and trigger fallback.
// CR-024: Efficiency optimization for local models.
func MaxStepsForModel(modelName string) int {
	modelLower := strings.ToLower(modelName)

	// Detect model size from common patterns
	// Tiny models (< 3B): very limited, fail fast
	if strings.Contains(modelLower, "1b") ||
		strings.Contains(modelLower, "0.5b") ||
		strings.Contains(modelLower, "0.6b") ||
		strings.Contains(modelLower, "1.5b") ||
		strings.Contains(modelLower, "2b") {
		return 4 // Very limited - fail fast
	}

	// Small models (3-4B): limited tool-calling ability
	if strings.Contains(modelLower, "3b") ||
		strings.Contains(modelLower, "4b") {
		return 6 // Limited - fail faster than default
	}

	// Medium-small models (7-8B): decent capability
	if strings.Contains(modelLower, "7b") ||
		strings.Contains(modelLower, "8b") {
		return 10 // Moderate
	}

	// Medium models (13-14B): good capability
	if strings.Contains(modelLower, "13b") ||
		strings.Contains(modelLower, "14b") {
		return 15 // Good
	}

	// Large models (30B+): high capability
	if strings.Contains(modelLower, "30b") ||
		strings.Contains(modelLower, "32b") ||
		strings.Contains(modelLower, "33b") ||
		strings.Contains(modelLower, "34b") ||
		strings.Contains(modelLower, "70b") ||
		strings.Contains(modelLower, "72b") ||
		strings.Contains(modelLower, "405b") {
		return 25 // Full capability
	}

	// Frontier models (Claude, GPT-4, Gemini): high capability
	frontierPatterns := []string{
		"claude", "gpt-4", "gpt4", "o1", "gemini", "grok",
	}
	for _, pattern := range frontierPatterns {
		if strings.Contains(modelLower, pattern) {
			return 25 // Full capability
		}
	}

	// Default: moderate steps for unknown models
	return 12
}

// New creates a new Agent.
func New(llm LLMProvider, cfg *Config) *Agent {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.MaxSteps == 0 {
		// CR-024: Use model-aware step limits for efficiency
		// Small models fail fast, large models get more steps
		if cfg.ModelName != "" {
			cfg.MaxSteps = MaxStepsForModel(cfg.ModelName)
		} else {
			// CR-009: Default to moderate steps when model unknown
			cfg.MaxSteps = 12
		}
	}
	if cfg.WorkingDir == "" {
		cfg.WorkingDir = "."
	}

	// Initialize supervised config with defaults if not provided
	supervisedCfg := DefaultSupervisedConfig()
	if cfg.SupervisedConfig != nil {
		supervisedCfg = *cfg.SupervisedConfig
	}

	executor := NewExecutor(cfg.WorkingDir)

	// Wire memory tools if configured
	if cfg.MemoryTools != nil {
		userID := cfg.MemoryUserID
		if userID == "" {
			userID = "default-user"
		}
		executor.SetMemoryTools(cfg.MemoryTools, userID)
	}

	return &Agent{
		llm:               llm,
		executor:          executor,
		workingDir:        cfg.WorkingDir,
		maxSteps:          cfg.MaxSteps,
		log:               logging.Global(),
		onStep:            cfg.OnStep,
		knowledge:         cfg.Knowledge,
		unrestrictedMode:  cfg.UnrestrictedMode,
		userContext:       cfg.UserContext,
		personaIdentity:   cfg.PersonaIdentity,
		modelName:         cfg.ModelName, // CR-024: Store model name for tier selection
		supervisedConfig:  supervisedCfg,
		checkpointHandler: cfg.CheckpointHandler,
	}
}

// emit sends a step event to the callback if configured.
func (a *Agent) emit(event *StepEvent) {
	if a.onStep != nil {
		a.onStep(event)
	}
}

// handleCheckpoint creates and processes a checkpoint, returning the user's response.
// Returns nil if in autonomous mode or no handler is configured.
func (a *Agent) handleCheckpoint(ctx context.Context, reason CheckpointReason, reasonMsg string, step int, response *Response, lastError string) (*CheckpointResponse, error) {
	// Skip checkpoint in autonomous mode
	if a.supervisedConfig.Mode == AgenticModeAutonomous {
		return nil, nil
	}

	// Skip if no handler configured
	if a.checkpointHandler == nil {
		a.log.Debug("[Agent] No checkpoint handler configured, using default behavior")
		return nil, nil
	}

	// Check if this checkpoint type is enabled
	switch reason {
	case CheckpointLoopDetected, CheckpointEmptyResults:
		if !a.supervisedConfig.CheckpointOnLoop {
			return nil, nil
		}
	case CheckpointToolError:
		if !a.supervisedConfig.CheckpointOnError {
			return nil, nil
		}
	case CheckpointStepLimit:
		if !a.supervisedConfig.CheckpointOnStepLimit {
			return nil, nil
		}
	}

	// Build checkpoint
	cp := NewCheckpoint(reason, reasonMsg, step, a.maxSteps)
	cp.ToolsUsed = response.ToolsUsed
	cp.TokensUsed = response.TokensUsed
	cp.LastError = lastError

	// Build progress summary from steps
	for _, s := range response.Steps {
		if s.ToolCall != nil {
			summary := fmt.Sprintf("Called %s", s.ToolCall.Name)
			if s.ToolResult != nil && !s.ToolResult.Success {
				summary += " (failed)"
			}
			cp.Progress = append(cp.Progress, summary)
		}
	}

	// Emit checkpoint event
	a.emit(&StepEvent{
		Type:    EventCheckpoint,
		Step:    step,
		Message: FormatCheckpoint(cp),
	})

	// Call handler and wait for response
	a.log.Info("[Agent] Checkpoint: %s (waiting for user input)", reason)
	cpResponse, err := a.checkpointHandler(ctx, cp)
	if err != nil {
		return nil, fmt.Errorf("checkpoint handler error: %w", err)
	}

	if cpResponse == nil {
		// Handler returned nil - use default (abort for supervised)
		a.log.Info("[Agent] Checkpoint handler returned nil, aborting")
		return &CheckpointResponse{Action: CheckpointAbort}, nil
	}

	a.log.Info("[Agent] Checkpoint response: %s", cpResponse.Action)
	return cpResponse, nil
}

// SetCheckpointHandler sets the handler for supervised checkpoints.
func (a *Agent) SetCheckpointHandler(handler CheckpointHandler) {
	a.checkpointHandler = handler
}

// SetSupervisedMode enables or changes the supervised mode.
func (a *Agent) SetSupervisedMode(mode AgenticMode) {
	a.supervisedConfig.Mode = mode
}

// Response represents an agent response.
type Response struct {
	Message    string   // Final message to user
	Steps      []Step   // Steps taken
	ToolsUsed  []string // Tools that were used
	Completed  bool     // Whether task completed successfully
	StepsCount int      // Number of steps taken
	Provider   string   // Provider that generated the response (e.g., "ollama", "anthropic")
	Model      string   // Model that generated the response (e.g., "qwen3:4b", "claude-sonnet-4")
	TokensUsed int      // Total tokens used across all LLM calls
}

// TokenAccumulator is an optional interface that LLMProviders can implement
// to track token usage across multiple calls.
type TokenAccumulator interface {
	// GetTotalTokens returns the total tokens used since last reset.
	GetTotalTokens() int
	// ResetTokens resets the token counter.
	ResetTokens()
}

// Step represents a single step in the agent's execution.
type Step struct {
	Thought    string      // Agent's reasoning
	ToolCall   *ToolCall   // Tool called (if any)
	ToolResult *ToolResult // Result (if tool was called)
}

// Run executes an agentic task.
func (a *Agent) Run(ctx context.Context, userMessage string, history []ChatMessage) (*Response, error) {
	a.log.Info("[Agent] Starting agentic task: %s", truncate(userMessage, 50))

	// Reset token counter if the LLM supports token tracking
	if accumulator, ok := a.llm.(TokenAccumulator); ok {
		accumulator.ResetTokens()
	}

	response := &Response{
		Steps:     make([]Step, 0),
		ToolsUsed: make([]string, 0),
	}

	// Capture tokens at the end of execution (deferred for all return paths)
	defer func() {
		if accumulator, ok := a.llm.(TokenAccumulator); ok {
			response.TokensUsed = accumulator.GetTotalTokens()
			a.log.Info("[Agent] Total tokens used: %d", response.TokensUsed)
		}
	}()

	// Build conversation with system prompt including user context, persona, and model-aware tier
	// CR-024: Use model-aware tier selection for efficiency (tiny tier for small models)
	a.log.Info("[Agent] Building prompt with userContext length: %d, personaIdentity length: %d, model: %s", len(a.userContext), len(a.personaIdentity), a.modelName)
	systemPrompt := SystemPromptWithModel(a.workingDir, a.knowledge, a.unrestrictedMode, a.userContext, a.personaIdentity, a.modelName)
	a.log.Info("[Agent] Final system prompt length: %d", len(systemPrompt))
	if len(a.userContext) > 0 {
		a.log.Info("[Agent] UserContext preview: %s", truncate(a.userContext, 200))
	}

	// Start with history + new user message
	messages := append(history, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	// DEBUG: Log history being used
	a.log.Info("[Agent] History contains %d messages", len(history))
	for i, msg := range history {
		a.log.Info("[Agent] History[%d]: role=%s, content=%s", i, msg.Role, truncate(msg.Content, 100))
	}

	// Initialize loop detector (CR-024: efficiency for small local models)
	loopDetect := newLoopDetector(a.log)

	// Track timing for long-running checkpoint
	startTime := time.Now()
	longRunningCheckpointTriggered := false
	longRunningTimeout := 60 * time.Second // Default 60s
	if a.supervisedConfig.LongRunningTimeout > 0 {
		longRunningTimeout = time.Duration(a.supervisedConfig.LongRunningTimeout) * time.Second
	}

	// Agentic loop
	for step := 0; step < a.maxSteps; step++ {
		// Check for context cancellation at the start of each step
		select {
		case <-ctx.Done():
			a.log.Info("[Agent] Context cancelled, stopping at step %d", step+1)
			a.emit(&StepEvent{
				Type:    EventComplete,
				Step:    step + 1,
				Message: "Task cancelled by user",
			})
			response.Message = "Task cancelled by user"
			response.Completed = false
			response.StepsCount = step
			return response, ctx.Err()
		default:
			// Continue with the loop
		}

		// Check for long-running task checkpoint (only trigger once)
		elapsed := time.Since(startTime)
		if !longRunningCheckpointTriggered && elapsed > longRunningTimeout && a.checkpointHandler != nil {
			longRunningCheckpointTriggered = true
			a.log.Info("[Agent] Long-running checkpoint triggered at %.1fs", elapsed.Seconds())

			checkpoint := NewCheckpoint(
				CheckpointLongRunning,
				fmt.Sprintf("Task has been running for %.0f seconds", elapsed.Seconds()),
				step,
				a.maxSteps,
			)
			checkpoint.LastAction = fmt.Sprintf("Step %d in progress", step+1)
			checkpoint.ToolsUsed = response.ToolsUsed
			checkpoint.TokensUsed = response.TokensUsed

			a.emit(&StepEvent{
				Type:    EventCheckpoint,
				Step:    step + 1,
				Message: checkpoint.ReasonMessage,
			})

			cpResponse, err := a.checkpointHandler(ctx, checkpoint)
			if err != nil {
				a.log.Warn("[Agent] Checkpoint handler error: %v", err)
			}

			if cpResponse != nil {
				switch cpResponse.Action {
				case CheckpointAbort:
					a.log.Info("[Agent] User chose to abort long-running task")
					response.Message = "Task aborted by user after " + fmt.Sprintf("%.0fs", elapsed.Seconds())
					response.Completed = false
					response.StepsCount = step
					return response, nil
				case CheckpointDifferentApproach:
					a.log.Info("[Agent] User requested different approach")
					// Add guidance to try a different method
					messages = append(messages, ChatMessage{
						Role:    "system",
						Content: "The user has requested you try a different approach. The current method is taking too long. Please reconsider and try a simpler or more direct method to complete this task.",
					})
				case CheckpointEscalate:
					a.log.Info("[Agent] User requested escalation (would require model switch)")
					// For now, just add guidance - actual escalation handled by orchestrator
					messages = append(messages, ChatMessage{
						Role:    "system",
						Content: "Please provide the best answer you can with your current capabilities. Be more direct and efficient.",
					})
				case CheckpointWait:
					a.log.Info("[Agent] User chose to wait, continuing...")
					// Reset timeout for another interval
					startTime = time.Now()
					longRunningCheckpointTriggered = false
				}
			}
		}

		a.log.Debug("[Agent] Step %d/%d", step+1, a.maxSteps)

		// Extract last user message for context
		lastUserMsg := ""
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == "user" {
				lastUserMsg = messages[i].Content
				break
			}
		}

		// Emit thinking event with descriptive context
		thinkingMsg := fmt.Sprintf("Step %d: Analyzing request...", step+1)
		if lastUserMsg != "" {
			// Show what we're analyzing (truncated for display)
			preview := lastUserMsg
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			thinkingMsg = fmt.Sprintf("Analyzing: %q", preview)
		}
		a.emit(&StepEvent{
			Type:    EventThinking,
			Step:    step + 1,
			Message: thinkingMsg,
		})

		a.log.Info("[LLM-Prompt] Step %d: %s", step+1, truncate(lastUserMsg, 200))

		// Call LLM - use streaming if available to show inner dialogue
		var llmResponse string
		var err error

		if streamingLLM, ok := a.llm.(StreamingLLMProvider); ok {
			// Use streaming to show real-time thinking
			llmResponse, err = streamingLLM.ChatStream(ctx, messages, systemPrompt, func(token string) {
				// Emit streaming token for real-time display
				a.emit(&StepEvent{
					Type:    EventStreaming,
					Step:    step + 1,
					Message: token,
				})
			})
		} else {
			// Fallback to non-streaming
			llmResponse, err = a.llm.Chat(ctx, messages, systemPrompt)
		}

		if err != nil {
			a.emit(&StepEvent{
				Type:    EventError,
				Step:    step + 1,
				Message: "LLM call failed",
				Error:   err.Error(),
			})
			return nil, fmt.Errorf("LLM error: %w", err)
		}

		// Log LLM response
		a.log.Info("[LLM-Response] Step %d: %s", step+1, truncate(llmResponse, 300))

		// Parse tool calls from response
		toolCalls, cleanedResponse := ParseToolCalls(llmResponse)
		a.log.Debug("[Agent] ParseToolCalls: found %d tool calls, cleanedResponse len=%d", len(toolCalls), len(cleanedResponse))

		currentStep := Step{
			Thought: cleanedResponse,
		}

		// Emit the agent's reasoning with a useful summary
		if len(toolCalls) > 0 {
			// Show which tools the agent decided to use
			toolNames := make([]string, 0, len(toolCalls))
			for _, tc := range toolCalls {
				toolNames = append(toolNames, tc.Name)
			}
			thinkingSummary := fmt.Sprintf("Planning to use: %s", strings.Join(toolNames, ", "))
			a.log.Debug("[Agent] Emitting EventThinking: %s", thinkingSummary)
			a.emit(&StepEvent{
				Type:    EventThinking,
				Step:    step + 1,
				Message: thinkingSummary,
			})
		} else if cleanedResponse != "" {
			// No tools - show truncated reasoning
			summary := cleanedResponse
			if len(summary) > 100 {
				summary = summary[:100] + "..."
			}
			a.log.Debug("[Agent] Emitting EventThinking with reasoning: %s", summary)
			a.emit(&StepEvent{
				Type:    EventThinking,
				Step:    step + 1,
				Message: fmt.Sprintf("Formulating response: %s", summary),
			})
		} else {
			a.log.Debug("[Agent] cleanedResponse is empty, skipping EventThinking emission")
		}

		// If no tool calls, we're done
		if len(toolCalls) == 0 {
			a.log.Info("[Agent] No tool calls, task complete after %d steps", step+1)
			a.emit(&StepEvent{
				Type:    EventComplete,
				Step:    step + 1,
				Message: "Task completed",
			})
			response.Message = cleanedResponse
			response.Completed = true
			response.StepsCount = step + 1
			response.Steps = append(response.Steps, currentStep)
			return response, nil
		}

		// Execute the first tool call
		toolCall := toolCalls[0]
		currentStep.ToolCall = toolCall

		// Log cognitive reasoning for tool selection
		paramsJSON, _ := json.Marshal(toolCall.Params)
		a.log.Info("[Cognitive] 🔧 Tool Selected: %s", toolCall.Name)
		a.log.Info("[Cognitive] 💭 Reasoning: LLM determined this tool is needed to complete the task")
		a.log.Info("[Cognitive] 📋 Parameters: %s", truncate(string(paramsJSON), 100))

		// Emit tool call event
		a.emit(&StepEvent{
			Type:      EventToolCall,
			Step:      step + 1,
			Message:   fmt.Sprintf("Calling tool: %s", toolCall.Name),
			ToolName:  toolCall.Name,
			ToolInput: string(paramsJSON),
		})

		a.log.Info("[Agent] Executing tool: %s", toolCall.Name)
		result := a.executor.Execute(ctx, toolCall)
		currentStep.ToolResult = result

		// Emit tool result event
		a.emit(&StepEvent{
			Type:     EventToolResult,
			Step:     step + 1,
			Message:  fmt.Sprintf("Tool %s completed", toolCall.Name),
			ToolName: toolCall.Name,
			Output:   truncate(result.Output, 500),
			Success:  result.Success,
			Error:    result.Error,
		})

		response.Steps = append(response.Steps, currentStep)
		response.ToolsUsed = append(response.ToolsUsed, toolCall.Name)

		// CR-024: Loop detection for small model efficiency
		// Record this tool call and result for pattern detection
		callSignature := loopDetect.recordCall(toolCall.Name, toolCall.Params)
		loopDetect.recordResult(result)

		// Check if we should terminate early due to detected loop
		if shouldTerminate, reason := loopDetect.shouldTerminate(callSignature); shouldTerminate {
			a.log.Warn("[Agent] Loop detected: %s", reason)

			// In supervised mode, ask the user what to do
			cpReason := CheckpointLoopDetected
			if loopDetect.emptyResults >= 3 {
				cpReason = CheckpointEmptyResults
			}

			cpResponse, cpErr := a.handleCheckpoint(ctx, cpReason, reason, step+1, response, "")
			if cpErr != nil {
				return nil, cpErr
			}

			// Handle checkpoint response
			if cpResponse != nil {
				switch cpResponse.Action {
				case CheckpointContinue:
					// User wants to continue - add more steps
					additionalSteps := cpResponse.AdditionalSteps
					if additionalSteps == 0 {
						additionalSteps = 5
					}
					a.maxSteps += additionalSteps
					loopDetect.recentCalls = nil // Reset loop detector
					loopDetect.emptyResults = 0
					a.log.Info("[Agent] User chose to continue, added %d steps (new max: %d)", additionalSteps, a.maxSteps)
					continue // Continue the loop

				case CheckpointGuide:
					// User provided guidance - inject it as a new message
					if cpResponse.Guidance != "" {
						a.pendingGuidance = cpResponse.Guidance
						messages = append(messages, ChatMessage{
							Role:    "user",
							Content: fmt.Sprintf("[User guidance]: %s", cpResponse.Guidance),
						})
						loopDetect.recentCalls = nil // Reset loop detector
						loopDetect.emptyResults = 0
						a.log.Info("[Agent] User provided guidance: %s", truncate(cpResponse.Guidance, 100))
						continue // Continue the loop with guidance
					}

				case CheckpointEscalate:
					// User wants to escalate - signal this in the response
					response.Message = fmt.Sprintf("Task requires escalation to a more capable model. Progress so far: %d steps completed.", step+1)
					response.Completed = false
					response.StepsCount = step + 1
					// The caller should detect this and switch to frontier model
					return response, fmt.Errorf("escalation_requested")

				case CheckpointAbort:
					// User chose to abort
					a.emit(&StepEvent{
						Type:    EventComplete,
						Step:    step + 1,
						Message: "Task aborted by user at checkpoint",
					})
					response.Message = fmt.Sprintf("Task aborted. Completed %d steps. %s", step+1, cleanedResponse)
					response.Completed = false
					response.StepsCount = step + 1
					return response, nil
				}
			}

			// No checkpoint response (autonomous mode or no handler) - use old behavior
			// CR-024: Don't emit verbose technical messages to user
			a.emit(&StepEvent{
				Type:    EventLoopExit,
				Step:    step + 1,
				Message: "Switching to direct answer",
			})

			// CR-024: Simplified user-facing message without technical details
			if cleanedResponse != "" && len(cleanedResponse) >= 20 {
				response.Message = cleanedResponse
			} else {
				response.Message = "I wasn't able to complete this task with the available tools. Please try rephrasing your request or breaking it into smaller steps."
			}
			response.Completed = true
			response.StepsCount = step + 1
			return response, nil
		}

		// Add assistant message and tool result to conversation
		messages = append(messages, ChatMessage{
			Role:    "assistant",
			Content: llmResponse,
		})
		messages = append(messages, ChatMessage{
			Role:    "user", // Tool results go as user messages for most LLMs
			Content: FormatToolResult(result),
		})
	}

	// Max steps reached - ask user in supervised mode
	a.log.Warn("[Agent] Max steps (%d) reached", a.maxSteps)

	cpResponse, cpErr := a.handleCheckpoint(ctx, CheckpointStepLimit,
		fmt.Sprintf("Reached maximum steps (%d)", a.maxSteps), a.maxSteps, response, "")
	if cpErr != nil {
		return nil, cpErr
	}

	// Handle checkpoint response
	if cpResponse != nil {
		switch cpResponse.Action {
		case CheckpointContinue:
			// User wants to continue - add more steps and loop again
			additionalSteps := cpResponse.AdditionalSteps
			if additionalSteps == 0 {
				additionalSteps = 10 // More steps for step limit checkpoint
			}
			oldMax := a.maxSteps
			a.maxSteps += additionalSteps
			a.log.Info("[Agent] User chose to continue, added %d steps (new max: %d)", additionalSteps, a.maxSteps)

			// Continue execution with a recursive call (simplified)
			// In practice, we should restructure this to use a loop variable
			// For now, we'll emit and return for the caller to retry
			a.emit(&StepEvent{
				Type:    EventThinking,
				Step:    oldMax + 1,
				Message: fmt.Sprintf("Continuing with %d more steps...", additionalSteps),
			})

			// Return with a special marker that the caller can use to retry
			response.Message = ""
			response.Completed = false
			response.StepsCount = oldMax
			return response, fmt.Errorf("continue_requested:%d", additionalSteps)

		case CheckpointGuide:
			// User provided guidance
			if cpResponse.Guidance != "" {
				additionalSteps := 5
				a.maxSteps += additionalSteps
				a.log.Info("[Agent] User provided guidance at step limit: %s", truncate(cpResponse.Guidance, 100))
				response.Message = ""
				response.Completed = false
				response.StepsCount = a.maxSteps - additionalSteps
				return response, fmt.Errorf("guidance_requested:%s", cpResponse.Guidance)
			}

		case CheckpointAbort:
			a.emit(&StepEvent{
				Type:    EventComplete,
				Step:    a.maxSteps,
				Message: "Task aborted by user at step limit",
			})
			response.Message = fmt.Sprintf("Task aborted at step limit. Completed %d steps.", a.maxSteps)
			response.Completed = false
			response.StepsCount = a.maxSteps
			return response, nil
		}
	}

	// Default behavior (autonomous mode)
	a.emit(&StepEvent{
		Type:    EventComplete,
		Step:    a.maxSteps,
		Message: "Maximum steps reached",
	})
	response.Message = "I've reached the maximum number of steps. Here's what I've done so far."
	response.Completed = false
	response.StepsCount = a.maxSteps

	return response, nil
}

// RunSingleStep executes a single step (useful for streaming responses).
func (a *Agent) RunSingleStep(ctx context.Context, userMessage string, history []ChatMessage) (string, *ToolCall, error) {
	systemPrompt := SystemPromptFull(a.workingDir, a.knowledge, a.unrestrictedMode)

	messages := append(history, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	// Call LLM
	llmResponse, err := a.llm.Chat(ctx, messages, systemPrompt)
	if err != nil {
		return "", nil, fmt.Errorf("LLM error: %w", err)
	}

	// Parse tool calls
	toolCalls, cleanedResponse := ParseToolCalls(llmResponse)

	if len(toolCalls) > 0 {
		return cleanedResponse, toolCalls[0], nil
	}

	return cleanedResponse, nil, nil
}

// ExecuteTool executes a single tool call.
func (a *Agent) ExecuteTool(ctx context.Context, call *ToolCall) *ToolResult {
	return a.executor.Execute(ctx, call)
}

// SetWorkingDir updates the working directory.
func (a *Agent) SetWorkingDir(dir string) {
	a.workingDir = dir
	a.executor = NewExecutor(dir)
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
