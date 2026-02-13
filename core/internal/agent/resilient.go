// Package agent provides agentic capabilities for Cortex.
// This file implements a resilient agent with automatic timeout recovery.
package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// RESILIENT AGENT
// ═══════════════════════════════════════════════════════════════════════════════

// ResilientAgent wraps an Agent with timeout recovery capabilities.
type ResilientAgent struct {
	primaryAgent   *Agent
	fallbackAgents map[string]*Agent // provider -> agent
	analyzer       *RecoveryAnalyzer
	config         *ResilientConfig
	log            *logging.Logger
	onLearn        LearningCallback
}

// ResilientConfig configures the resilient agent.
type ResilientConfig struct {
	// Primary LLM settings
	PrimaryProvider string
	PrimaryModel    string
	PrimaryEndpoint string

	// Fallback LLMs (provider -> LLMProvider)
	FallbackLLMs map[string]LLMProvider

	// Agent settings
	WorkingDir       string
	MaxSteps         int
	OnStep           StepCallback
	Knowledge        *KnowledgeContext    // Optional knowledge context for enhanced responses
	UnrestrictedMode bool                 // Disables safety restrictions in system prompt
	MemoryTools      MemoryToolsInterface // Optional memory tools for MemGPT-style operations
	MemoryUserID     string               // User ID for memory operations
	UserContext      string               // Pre-built user memory context to inject into prompt
	PersonaIdentity  string               // Persona identity string (replaces default "You are Cortex...")

	// Recovery settings
	RecoveryConfig *RecoveryConfig

	// Learning callback
	OnLearn LearningCallback

	// Supervised agentic mode
	SupervisedConfig  *SupervisedConfig  // Supervised agentic mode configuration
	CheckpointHandler CheckpointHandler  // Handler for supervised checkpoints
}

// NewResilientAgent creates a new resilient agent.
func NewResilientAgent(primaryLLM LLMProvider, cfg *ResilientConfig) *ResilientAgent {
	if cfg == nil {
		cfg = &ResilientConfig{}
	}
	if cfg.MaxSteps == 0 {
		// CR-009: Increased from 10 to 25 to allow more complex multi-step tasks
		// without hitting the step limit prematurely
		cfg.MaxSteps = 25
	}
	if cfg.WorkingDir == "" {
		cfg.WorkingDir = "."
	}

	// Create primary agent
	primaryAgent := New(primaryLLM, &Config{
		WorkingDir:        cfg.WorkingDir,
		MaxSteps:          cfg.MaxSteps,
		OnStep:            cfg.OnStep,
		Knowledge:         cfg.Knowledge,
		UnrestrictedMode:  cfg.UnrestrictedMode,
		MemoryTools:       cfg.MemoryTools,
		MemoryUserID:      cfg.MemoryUserID,
		UserContext:       cfg.UserContext,
		PersonaIdentity:   cfg.PersonaIdentity,
		SupervisedConfig:  cfg.SupervisedConfig,
		CheckpointHandler: cfg.CheckpointHandler,
	})

	// Create fallback agents
	fallbackAgents := make(map[string]*Agent)
	for provider, llm := range cfg.FallbackLLMs {
		fallbackAgents[provider] = New(llm, &Config{
			WorkingDir:        cfg.WorkingDir,
			MaxSteps:          cfg.MaxSteps,
			OnStep:            cfg.OnStep,
			Knowledge:         cfg.Knowledge,
			UnrestrictedMode:  cfg.UnrestrictedMode,
			MemoryTools:       cfg.MemoryTools,
			MemoryUserID:      cfg.MemoryUserID,
			UserContext:       cfg.UserContext,
			PersonaIdentity:   cfg.PersonaIdentity,
			SupervisedConfig:  cfg.SupervisedConfig,
			CheckpointHandler: cfg.CheckpointHandler,
		})
	}

	// Set up recovery config
	recoveryConfig := cfg.RecoveryConfig
	if recoveryConfig == nil {
		recoveryConfig = DefaultRecoveryConfig()
	}
	recoveryConfig.PrimaryEndpoint = cfg.PrimaryEndpoint
	recoveryConfig.PrimaryProvider = cfg.PrimaryProvider
	recoveryConfig.PrimaryModel = cfg.PrimaryModel

	return &ResilientAgent{
		primaryAgent:   primaryAgent,
		fallbackAgents: fallbackAgents,
		analyzer:       NewRecoveryAnalyzer(recoveryConfig),
		config:         cfg,
		log:            logging.Global(),
		onLearn:        cfg.OnLearn,
	}
}

// Run executes an agentic task with automatic timeout recovery.
func (r *ResilientAgent) Run(ctx context.Context, userMessage string, history []ChatMessage) (*Response, error) {
	startTime := time.Now()
	r.log.Info("[ResilientAgent] Starting task: %s", truncate(userMessage, 50))

	// Track execution state for recovery analysis
	taskContext := &TaskContext{
		Task:             userMessage,
		ConversationSize: len(history),
	}

	// Attempt 1: Try with primary agent
	result, err := r.primaryAgent.Run(ctx, userMessage, history)
	if err == nil {
		// NEW: Check response quality before declaring success
		quality := r.assessResponseQuality(result, userMessage)
		if quality.ShouldFallback {
			r.log.Warn("[ResilientAgent] Quality issue detected: %s (score=%d, type=%s)",
				quality.Reason, quality.Score, quality.IssueType)

			// Emit quality warning event
			if r.config.OnStep != nil {
				r.config.OnStep(&StepEvent{
					Type:    EventThinking,
					Message: fmt.Sprintf("⚠️ Quality issue: %s - switching to frontier model", quality.Reason),
				})
			}

			// Create recovery decision for quality-based fallback
			qualityDecision := &RecoveryDecision{
				Action:           ActionFallback,
				FallbackProvider: "anthropic",
				FallbackModel:    "claude-sonnet-4-20250514",
				Reason:           quality.Reason,
				ShouldLearn:      true,
				LearningNote:     fmt.Sprintf("Quality failover: %s (score=%d)", quality.IssueType, quality.Score),
			}

			// Set fallback provider from config if available
			if r.analyzer != nil && r.analyzer.config != nil {
				for _, fb := range r.analyzer.config.FallbackProviders {
					if fb.Priority == 1 {
						qualityDecision.FallbackProvider = fb.Name
						qualityDecision.FallbackModel = fb.Model
						break
					}
				}
			}

			// Execute fallback
			return r.executeFallback(ctx, userMessage, history, qualityDecision, taskContext)
		}

		// Set provider info on result for primary agent success
		result.Provider = r.config.PrimaryProvider
		result.Model = r.config.PrimaryModel
		r.log.Info("[Cognitive] ✅ LOCAL MODEL USED: %s/%s", r.config.PrimaryProvider, r.config.PrimaryModel)
		return result, nil
	}

	isTimeout := r.isTimeoutError(err)
	isAPIError := r.isRecoverableAPIError(err)

	if !isTimeout && !isAPIError {
		r.log.Error("[ResilientAgent] Non-recoverable error: %v", err)
		return nil, err
	}

	if isAPIError {
		r.log.Warn("[ResilientAgent] API error detected, triggering fallback: %v", err)
	} else {
		r.log.Warn("[ResilientAgent] Primary agent timed out: %v", err)
	}
	taskContext.ElapsedTime = time.Since(startTime)

	// Analyze the timeout and decide recovery strategy
	decision := r.analyzer.AnalyzeTimeout(ctx, err, taskContext)

	// Emit recovery event
	if r.config.OnStep != nil {
		r.config.OnStep(&StepEvent{
			Type:    EventThinking,
			Message: fmt.Sprintf("🔄 Recovery: %s - %s", decision.Action, decision.Reason),
		})
	}

	// Execute recovery action
	var recoveryResult *Response
	var recoveryErr error

	switch decision.Action {
	case ActionFallback:
		recoveryResult, recoveryErr = r.executeFallback(ctx, userMessage, history, decision, taskContext)

	case ActionRetry:
		r.log.Info("[ResilientAgent] Retrying with primary agent...")
		// Create fresh context for retry since original may be expired
		retryCtx, retryCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		recoveryResult, recoveryErr = r.primaryAgent.Run(retryCtx, userMessage, history)
		retryCancel()

	case ActionWaitAndRetry:
		r.log.Info("[ResilientAgent] Waiting %v before retry...", decision.WaitDuration)
		time.Sleep(decision.WaitDuration)
		// Create fresh context for retry since original may be expired
		retryCtx, retryCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		recoveryResult, recoveryErr = r.primaryAgent.Run(retryCtx, userMessage, history)
		retryCancel()

	case ActionAbort:
		return nil, fmt.Errorf("recovery aborted: %s", decision.Reason)

	default:
		// Default to fallback
		recoveryResult, recoveryErr = r.executeFallback(ctx, userMessage, history, decision, taskContext)
	}

	// Record learning if enabled
	if decision.ShouldLearn && r.onLearn != nil {
		r.recordLearning(taskContext, decision, recoveryResult, recoveryErr)
	}

	return recoveryResult, recoveryErr
}

// executeFallback runs the task on a frontier model.
// IMPORTANT: Creates a fresh context for fallback since the original may be expired.
func (r *ResilientAgent) executeFallback(
	ctx context.Context,
	userMessage string,
	history []ChatMessage,
	decision *RecoveryDecision,
	taskContext *TaskContext,
) (*Response, error) {
	provider := decision.FallbackProvider
	r.log.Info("[ResilientAgent] Falling back to %s/%s", provider, decision.FallbackModel)

	// Emit fallback notification
	if r.config.OnStep != nil {
		r.config.OnStep(&StepEvent{
			Type:    EventThinking,
			Message: fmt.Sprintf("🚀 Switching to frontier model (%s) for better performance", provider),
		})
	}

	// Get fallback agent
	fallbackAgent, ok := r.fallbackAgents[provider]
	if !ok {
		// Provide helpful guidance on configuring the API key
		envVarMap := map[string]string{
			"anthropic": "ANTHROPIC_API_KEY",
			"openai":    "OPENAI_API_KEY",
			"gemini":    "GEMINI_API_KEY",
			"grok":      "XAI_API_KEY",
		}
		envVar := envVarMap[provider]
		if envVar == "" {
			envVar = strings.ToUpper(provider) + "_API_KEY"
		}
		return nil, fmt.Errorf("fallback provider %s not configured. To enable:\n"+
			"  1. Add %s=your-key to ~/.cortex/.env\n"+
			"  2. Or use /setkey %s in Cortex TUI\n"+
			"  3. Then restart Cortex", provider, envVar, provider)
	}

	// CRITICAL FIX: Create a fresh context for fallback since the original context
	// may have an expired deadline from the primary timeout. Frontier models are
	// typically faster, so we give them 3 minutes to respond.
	fallbackCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	r.log.Info("[ResilientAgent] Created fresh context with 3m timeout for fallback")

	// Run with fallback using fresh context
	result, err := fallbackAgent.Run(fallbackCtx, userMessage, history)
	if err != nil {
		r.log.Error("[ResilientAgent] Fallback also failed: %v", err)

		// Try other fallbacks with fresh contexts
		for p, agent := range r.fallbackAgents {
			if p == provider {
				continue // Already tried this one
			}
			r.log.Info("[ResilientAgent] Trying alternative fallback: %s", p)

			// Fresh context for each alternative fallback attempt
			altCtx, altCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			result, err = agent.Run(altCtx, userMessage, history)
			altCancel()

			if err == nil {
				// Set provider info on result - look up the correct model for this provider
				result.Provider = p
				result.Model = r.getModelForProvider(p)
				r.log.Info("[Cognitive] 🚀 FRONTIER MODEL USED: %s/%s", p, result.Model)
				r.log.Info("[Cognitive] 💡 Reason: Primary provider timed out, switched to cloud")
				return result, nil
			}
			// Log why this fallback failed before trying the next one
			r.log.Warn("[ResilientAgent] Alternative fallback %s failed: %v", p, err)
		}

		return nil, fmt.Errorf("all fallback providers failed: %w", err)
	}

	// Set provider info on result
	result.Provider = provider
	result.Model = decision.FallbackModel
	r.log.Info("[Cognitive] 🚀 FRONTIER MODEL USED: %s/%s", provider, decision.FallbackModel)
	r.log.Info("[Cognitive] 💡 Reason: Primary provider timed out, switched to cloud")
	return result, nil
}

// getModelForProvider looks up the model configured for a given provider.
// Returns a default model name if not found in config.
func (r *ResilientAgent) getModelForProvider(provider string) string {
	if r.analyzer != nil && r.analyzer.config != nil {
		for _, fb := range r.analyzer.config.FallbackProviders {
			if fb.Name == provider {
				return fb.Model
			}
		}
	}
	// Default models if not found in config
	defaults := map[string]string{
		"anthropic": "claude-sonnet-4-20250514",
		"openai":    "gpt-4o",
		"gemini":    "gemini-1.5-pro",
		"grok":      "grok-3",
	}
	if m, ok := defaults[provider]; ok {
		return m
	}
	return "unknown"
}

// isTimeoutError checks if an error is timeout-related.
func (r *ResilientAgent) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "Timeout") ||
		strings.Contains(errStr, "i/o timeout")
}

// isRecoverableAPIError checks if an error is a recoverable API error that should trigger fallback.
// This includes quota errors, authentication errors, and service unavailability.
func (r *ResilientAgent) isRecoverableAPIError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()

	// HTTP status code patterns that indicate recoverable errors
	recoverablePatterns := []string{
		// Quota/rate limit errors
		"status 429",
		"insufficient_quota",
		"rate_limit",
		"quota exceeded",
		"too many requests",

		// Authentication errors (wrong key, expired, etc.)
		"status 401",
		"status 403",
		"unauthorized",
		"invalid_api_key",
		"authentication",

		// Service unavailability
		"status 500",
		"status 502",
		"status 503",
		"status 504",
		"service unavailable",
		"internal server error",
		"bad gateway",

		// Model capability errors (wrong model type)
		"status 400",
		"does not support",
		"not supported",
		"model not found",

		// Connection errors
		"connection refused",
		"connection reset",
		"no such host",
		"dns lookup",
	}

	errLower := strings.ToLower(errStr)
	for _, pattern := range recoverablePatterns {
		if strings.Contains(errLower, pattern) {
			return true
		}
	}

	return false
}

// recordLearning records timeout event for learning.
func (r *ResilientAgent) recordLearning(
	taskContext *TaskContext,
	decision *RecoveryDecision,
	result *Response,
	err error,
) {
	learning := &TimeoutLearning{
		Timestamp:      time.Now(),
		Task:           taskContext.Task,
		PrimaryModel:   r.config.PrimaryModel,
		Complexity:     r.analyzer.assessComplexity(taskContext),
		StepsCompleted: taskContext.StepsCompleted,
		TimeoutAfter:   taskContext.ElapsedTime,
		RecoveryAction: decision.Action,
		LearningNote:   decision.LearningNote,
	}

	if decision.Action == ActionFallback {
		learning.FallbackUsed = fmt.Sprintf("%s/%s", decision.FallbackProvider, decision.FallbackModel)
		learning.FallbackSuccess = err == nil && result != nil && result.Completed
	}

	r.log.Info("[ResilientAgent] Recording learning: %s -> %s (success=%v)",
		learning.PrimaryModel, learning.FallbackUsed, learning.FallbackSuccess)

	r.onLearn(learning)
}

// SetWorkingDir updates the working directory for all agents.
func (r *ResilientAgent) SetWorkingDir(dir string) {
	r.primaryAgent.SetWorkingDir(dir)
	for _, agent := range r.fallbackAgents {
		agent.SetWorkingDir(dir)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUALITY-BASED FAILOVER
// ═══════════════════════════════════════════════════════════════════════════════

// QualityAssessment represents the result of response quality analysis.
type QualityAssessment struct {
	ShouldFallback bool   // Whether to trigger fallback
	Reason         string // Human-readable reason
	Score          int    // Quality score (0-100)
	IssueType      string // Type of issue detected
}

// assessResponseQuality analyzes a response to detect quality issues that warrant fallback.
// This catches scenarios where small models fail to use tools properly.
func (r *ResilientAgent) assessResponseQuality(result *Response, originalRequest string) QualityAssessment {
	if result == nil {
		return QualityAssessment{ShouldFallback: true, Reason: "nil response", Score: 0, IssueType: "nil"}
	}

	// Check 0: Empty or near-empty response (catches garbage/empty responses from small models)
	trimmed := strings.TrimSpace(result.Message)
	if len(trimmed) < 20 {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "response was empty or too short",
			Score:          5,
			IssueType:      "empty_response",
		}
	}

	// Check 0.5: Response is just echoing the user's input (common with confused small models)
	if len(originalRequest) > 20 && len(trimmed) < len(originalRequest)*2 {
		// Check if response contains significant portion of the original request
		lowerResponse := strings.ToLower(trimmed)
		lowerRequest := strings.ToLower(originalRequest)
		// If more than 70% of the response is just the request echoed back
		if strings.Contains(lowerResponse, lowerRequest[:min(len(lowerRequest), 50)]) {
			return QualityAssessment{
				ShouldFallback: true,
				Reason:         "response appears to echo user input",
				Score:          10,
				IssueType:      "echo_response",
			}
		}
	}

	// Check 1: No tool calls when input looks like a command
	if looksLikeCommand(originalRequest) && len(result.ToolsUsed) == 0 {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "no tool calls for command request",
			Score:          20,
			IssueType:      "no_tools",
		}
	}

	// Check 2: Model refused to execute (over-aligned safety)
	if containsRefusalPatterns(result.Message) {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "model refused to execute request",
			Score:          15,
			IssueType:      "refusal",
		}
	}

	// Check 3: Response is just echoing/predicting output instead of executing
	if containsPredictionPatterns(result.Message) {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "model predicting output instead of executing",
			Score:          10,
			IssueType:      "prediction",
		}
	}

	// Check 4: Response is repetitive (a sign of model confusion)
	if isRepetitive(result.Message) {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "repetitive output detected",
			Score:          30,
			IssueType:      "repetitive",
		}
	}

	// Check 5: Very short response for complex request
	if looksComplex(originalRequest) && len(result.Message) < 50 && len(result.ToolsUsed) == 0 {
		return QualityAssessment{
			ShouldFallback: true,
			Reason:         "inadequate response for complex request",
			Score:          35,
			IssueType:      "shallow",
		}
	}

	return QualityAssessment{ShouldFallback: false, Score: 100, IssueType: "none"}
}

// looksLikeCommand returns true if the input appears to be requesting command execution.
func looksLikeCommand(input string) bool {
	lower := strings.ToLower(input)
	trimmed := strings.TrimSpace(input)

	// ═══════════════════════════════════════════════════════════════════════════════
	// PATH-BASED DETECTION
	// Commands starting with paths are almost always meant to be executed
	// ═══════════════════════════════════════════════════════════════════════════════

	// Executable paths (absolute, relative, home)
	if strings.HasPrefix(trimmed, "/") ||
		strings.HasPrefix(trimmed, "./") ||
		strings.HasPrefix(trimmed, "~/") {
		return true
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// CLI TOOL DETECTION
	// Common CLI tools that indicate command execution intent
	// ═══════════════════════════════════════════════════════════════════════════════

	// CLI tools that start the input (case-insensitive prefix match)
	cliToolPrefixes := []string{
		// Our own CLI
		"cortex ",

		// Database tools
		"sqlite3 ", "mysql ", "psql ", "mongosh ", "redis-cli ",

		// Container/orchestration
		"docker ", "docker-compose ", "podman ", "kubectl ", "helm ",
		"terraform ", "ansible ",

		// Package managers
		"npm ", "npx ", "yarn ", "pnpm ", "pip ", "pip3 ",
		"cargo ", "go ", "gem ", "composer ", "brew ",
		"apt ", "apt-get ", "yum ", "dnf ", "pacman ",

		// Version control
		"git ", "gh ", "svn ",

		// Build tools
		"make ", "cmake ", "gradle ", "mvn ", "bazel ",

		// Runtime/interpreters
		"node ", "python ", "python3 ", "ruby ", "perl ",
		"java ", "javac ", "rustc ", "gcc ", "clang ",

		// Network tools
		"curl ", "wget ", "ssh ", "scp ", "rsync ",
		"ping ", "traceroute ", "netstat ", "nc ", "nmap ",

		// File operations
		"cat ", "ls ", "cd ", "mkdir ", "rm ", "cp ", "mv ",
		"chmod ", "chown ", "touch ", "head ", "tail ", "grep ",
		"find ", "locate ", "tar ", "zip ", "unzip ", "gzip ",

		// System tools
		"ps ", "top ", "htop ", "kill ", "pkill ", "systemctl ",
		"service ", "journalctl ", "dmesg ", "lsof ", "df ", "du ",

		// Text processing
		"echo ", "printf ", "sed ", "awk ", "sort ", "uniq ",
		"wc ", "cut ", "tr ", "xargs ",

		// Editors (when used as commands)
		"vim ", "vi ", "nano ", "emacs ", "code ",
	}

	for _, prefix := range cliToolPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// SHELL SYNTAX DETECTION
	// Patterns that indicate shell command syntax
	// ═══════════════════════════════════════════════════════════════════════════════

	shellPatterns := []string{
		"&&", "||", " | ", ";", "$(", "${",
		"for ", "while ", " done",
		" > ", " >> ", " < ", " 2>&1",
	}
	for _, p := range shellPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// IMPERATIVE ACTION DETECTION
	// Words at the start that suggest wanting an action performed
	// ═══════════════════════════════════════════════════════════════════════════════

	imperativePatterns := []string{
		"run ", "execute ", "create ", "delete ", "install ",
		"build ", "test ", "deploy ", "start ", "stop ",
		"read ", "write ", "open ", "close ", "search ",
		"list ", "show ", "find ", "get ", "set ",
		"analyze ", "analyse ", "check ", "verify ", "scan ",
		"download ", "upload ", "fetch ", "push ", "pull ",
		"init ", "initialize ", "setup ", "configure ",
		// Web search patterns
		"look up ", "google ", "search for ", "search the web ",
	}

	// Questions that require web search or tool use
	webSearchPatterns := []string{
		"what's the weather", "what is the weather",
		"what's the temperature", "what is the temperature",
		"weather in ", "temperature in ",
		"current news", "latest news",
		"what time is it in", "current time in",
		"stock price", "exchange rate",
	}
	for _, p := range webSearchPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	for _, p := range imperativePatterns {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}

	return false
}

// containsPredictionPatterns detects if response is predicting output instead of executing.
func containsPredictionPatterns(response string) bool {
	lower := strings.ToLower(response)

	// Patterns indicating prediction instead of execution
	patterns := []string{
		"the output will be",
		"this will output",
		"the result will be",
		"this command will",
		"running this will",
		"would output",
		"would result in",
		"would produce",
		"will print",
		"will display",
		"will show",
		"the command would",
	}

	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// Check for code block with predicted output (e.g., showing what bash would output)
	// without any tool call markers
	if strings.Contains(lower, "```") && !strings.Contains(response, "<tool>") {
		// If there's a code block showing output but no tool use, likely prediction
		outputIndicators := []string{"output:", "result:", "returns:", "prints:"}
		for _, ind := range outputIndicators {
			if strings.Contains(lower, ind) {
				return true
			}
		}
	}

	return false
}

// isRepetitive checks if the response contains repetitive patterns.
func isRepetitive(response string) bool {
	if len(response) < 100 {
		return false
	}

	// Check for repeated phrases (3+ occurrences of same 20+ char substring)
	words := strings.Fields(response)
	if len(words) < 10 {
		return false
	}

	// Build phrase frequency map (3-word phrases)
	phrases := make(map[string]int)
	for i := 0; i < len(words)-2; i++ {
		phrase := strings.Join(words[i:i+3], " ")
		if len(phrase) > 15 { // Only count substantial phrases
			phrases[phrase]++
		}
	}

	// If any phrase repeats 3+ times, it's repetitive
	for _, count := range phrases {
		if count >= 3 {
			return true
		}
	}

	// Check for repeating lines
	lines := strings.Split(response, "\n")
	lineCount := make(map[string]int)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 20 {
			lineCount[trimmed]++
		}
	}
	for _, count := range lineCount {
		if count >= 3 {
			return true
		}
	}

	return false
}

// containsRefusalPatterns detects if the model refused to execute instead of using tools.
// This catches cases where models are over-aligned on safety and refuse legitimate requests.
// NOTE: Only checks for tool-execution refusals, not capability questions.
func containsRefusalPatterns(response string) bool {
	lower := strings.ToLower(response)

	// Skip refusal check if this looks like an answer to a capability question
	// (e.g., "do you speak Japanese?" → "I don't have the capability..." is valid)
	capabilityAnswerIndicators := []string{
		"speak", "language", "understand", "translate",
		"see images", "hear", "vision", "audio",
	}
	for _, indicator := range capabilityAnswerIndicators {
		if strings.Contains(lower, indicator) {
			return false // This is answering a capability question, not refusing
		}
	}

	// Tool-execution refusal patterns (more specific)
	refusalPatterns := []string{
		// Direct refusals to execute tasks
		"i'm sorry, but i can't help with that",
		"i'm sorry, but i cannot help with that",
		"i cannot help with this request",
		"i can't help with this request",
		"i refuse to",
		"i will not",
		"i won't do that",

		// Deflections that suggest unwillingness to use tools
		"you would need to run this yourself",
		"you'll need to run this yourself",
		"you can run this command yourself",
		"please run this yourself",
		"i cannot execute commands",
		"i cannot run commands",

		// Safety refusals
		"this could be harmful",
		"this is potentially dangerous",
		"i cannot assist with potentially",
	}

	for _, pattern := range refusalPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// looksComplex checks if a request appears to require significant processing.
func looksComplex(input string) bool {
	lower := strings.ToLower(input)

	// Multi-step indicators
	complexPatterns := []string{
		"and then", "after that", "next,", "finally,",
		"step 1", "step 2", "first,", "second,",
		"multiple", "several", "all ", "each ",
		"refactor", "implement", "design", "architecture",
	}
	for _, p := range complexPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// Long requests are usually complex
	if len(input) > 200 {
		return true
	}

	// Multiple sentences suggest complexity
	sentenceEnders := regexp.MustCompile(`[.!?]\s`)
	if len(sentenceEnders.FindAllString(input, -1)) >= 2 {
		return true
	}

	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// EVENT TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Additional step event type for recovery
const (
	EventRecovery StepEventType = "recovery" // Recovery in progress
)
