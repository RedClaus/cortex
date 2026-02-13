package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/persona"
	"github.com/normanking/cortex/pkg/types"
)

// ===========================================================================
// LLM STAGE
// ===========================================================================

// llmStage processes natural language queries through the LLM.
type llmStage struct {
	o *Orchestrator
}

func (s *llmStage) Name() string { return "llm" }

func (s *llmStage) Execute(ctx context.Context, state *PipelineState) error {
	log := logging.Global()

	// Skip if no LLM configured
	if s.o.llm == nil && s.o.agentLLM == nil {
		return nil
	}

	// Skip if we already have tool results (tools provided the answer)
	if len(state.ToolResults) > 0 {
		return nil
	}

	// Skip direct command requests (they're handled by tools)
	if state.Request.Type == RequestCommand {
		return nil
	}

	// COGNITIVE FILTER: Route input to appropriate processing mode
	inputPreview := truncateLog(state.Request.Input, 60)

	if isSimpleConversation(state.Request.Input) {
		// Simple conversation: greetings, capability questions, etc.
		log.Info("[Cognitive] 🧠 Classification: SIMPLE_CONVERSATION")
		log.Info("[Cognitive] 💭 Reasoning: Input matches greeting/chat pattern, no tools needed")
		log.Info("[Cognitive] 🎯 Decision: Direct LLM response (fast path)")
		log.Info("[Cognitive] 📝 Input: %s", inputPreview)
		// Use agentLLM for simple chat (no tools) if standard llm is nil
		if s.o.llm == nil && s.o.agentLLM != nil {
			return s.executeSimpleChat(ctx, state)
		}
		// Fall through to standard LLM mode
	} else if isPersonalQuestion(state.Request.Input) && s.o.agenticMode && s.o.agentLLM != nil {
		// CRITICAL: Personal questions need memory context - MUST go through agentic path
		// This check MUST come before canAnswerDirectly() to prevent memory loss
		// "Who is Norman?" should use memory, not be answered from training data
		log.Info("[Cognitive] 🧠 Classification: PERSONAL_MEMORY_QUERY")
		log.Info("[Cognitive] 💭 Reasoning: Personal question requires user memory context")
		log.Info("[Cognitive] 🎯 Decision: Agentic mode with memory injection")
		log.Info("[Cognitive] 📝 Input: %s", inputPreview)
		return s.executeAgentic(ctx, state)
	} else if canAnswerDirectly(state.Request.Input) {
		// Factual question: can be answered from LLM training, no tools needed
		// This is critical for local model efficiency - prevents wasteful tool loops
		// NOTE: Personal questions are filtered out above and sent to agentic mode
		log.Info("[Cognitive] 🧠 Classification: DIRECT_ANSWER")
		log.Info("[Cognitive] 💭 Reasoning: Factual/knowledge question answerable from training data")
		log.Info("[Cognitive] 🎯 Decision: Direct LLM response (no tool waste)")
		log.Info("[Cognitive] 📝 Input: %s", inputPreview)
		// Use agentLLM for simple chat (no tools) if standard llm is nil
		if s.o.llm == nil && s.o.agentLLM != nil {
			return s.executeSimpleChat(ctx, state)
		}
		// Fall through to standard LLM mode
	} else if s.o.agenticMode && s.o.agentLLM != nil {
		// Complex task: may need tools, web search, file operations, etc.
		log.Info("[Cognitive] 🧠 Classification: AGENTIC_TASK")
		log.Info("[Cognitive] 💭 Reasoning: Input requires action/research, tools may be needed")
		log.Info("[Cognitive] 🎯 Decision: Agentic mode with tool access")
		log.Info("[Cognitive] 📝 Input: %s", inputPreview)
		return s.executeAgentic(ctx, state)
	}

	// Standard LLM mode
	if s.o.llm == nil {
		return nil
	}

	// Build context-aware system prompt
	// Try to use ContextBuilder if available, fall back to legacy if not
	systemPrompt := s.buildSystemPrompt(state)
	if s.o.memoryStore != nil {
		systemPrompt = s.buildSystemPromptWithMemory(ctx, state)
	}

	// Build messages with conversation history
	messages := s.buildMessages(state)

	// Convert to types.LLMMessage for shared types
	llmMessages := make([]types.LLMMessage, len(messages))
	for i, msg := range messages {
		llmMessages[i] = types.LLMMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
		}
	}

	// Create LLM request
	req := &LLMRequest{
		SystemPrompt: systemPrompt,
		Messages:     llmMessages,
		MaxTokens:    2048,
		Temperature:  0.7,
	}

	// Get provider and model info for logging
	provider := "unknown"
	model := req.Model
	if state.Request.Context != nil && state.Request.Context.ModelOverride != "" {
		model = state.Request.Context.ModelOverride
	}
	// Try to extract provider from LLM
	if llmInfo, ok := s.o.llm.(interface{ Provider() string }); ok {
		provider = llmInfo.Provider()
	}

	// Log request before LLM call (if eval enabled)
	var requestID string
	if s.o.evalEnabled && s.o.convLogger != nil {
		var complexityScore int
		if state.Cognitive != nil {
			complexityScore = state.Cognitive.ComplexityScore
		}
		var sessionID string
		if state.Request.Context != nil {
			sessionID = state.Request.ID
		}

		logReq := &eval.LogRequest{
			SessionID:       sessionID,
			Provider:        provider,
			Model:           model,
			Prompt:          state.Request.Input,
			SystemPrompt:    systemPrompt,
			ComplexityScore: complexityScore,
		}

		// Use detached context to prevent cancellation from affecting logging
		logCtx, logCancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
		defer logCancel()

		var logErr error
		requestID, logErr = s.o.convLogger.LogRequest(logCtx, logReq)
		if logErr != nil {
			log.Warn("[Eval] Failed to log request: %v", logErr)
		}
	}

	// Create cancellable context for interrupt support (CR-010 Track 3)
	streamCtx, cancel := context.WithCancel(ctx)
	s.o.mu.Lock()
	s.o.currentStreamCtx = streamCtx
	s.o.cancelStream = cancel
	s.o.mu.Unlock()

	defer func() {
		s.o.mu.Lock()
		s.o.cancelStream = nil
		s.o.currentStreamCtx = nil
		s.o.mu.Unlock()
	}()

	// Execute LLM call with timing
	startTime := time.Now()
	log.Debug("[LLM] Calling LLM provider...")
	resp, err := s.o.llm.Chat(streamCtx, req)
	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Log response (if eval enabled)
	if s.o.evalEnabled && s.o.convLogger != nil && requestID != "" {
		logResp := &eval.LogResponse{
			Response:         "",
			DurationMs:       durationMs,
			Success:          err == nil,
			ContextTokens:    0,
			CompletionTokens: 0,
		}
		if resp != nil {
			logResp.Response = resp.Content
			logResp.CompletionTokens = resp.TokensUsed
		}
		if err != nil {
			logResp.ErrorCode = "llm_error"
			logResp.ErrorMessage = err.Error()
		}

		// Use detached context to prevent cancellation from affecting logging
		logCtx, logCancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
		defer logCancel()

		if logErr := s.o.convLogger.LogResponse(logCtx, requestID, logResp); logErr != nil {
			log.Warn("[Eval] Failed to log response: %v", logErr)
		}

		// Run assessment and check for model recommendations
		s.assessAndRecommend(ctx, state, requestID, provider, model, resp, durationMs)
	}

	if err != nil {
		// Record failure outcome
		s.recordRoutingOutcome(state, provider, model, false, durationMs)
		return fmt.Errorf("llm error: %w", err)
	}

	// Store response in state (including provider/model for token metrics)
	state.LLMResponse = resp.Content
	state.LLMTokensUsed = resp.TokensUsed
	state.LLMProvider = provider
	state.LLMModel = model

	// Record success outcome for RoamPal learning
	s.recordRoutingOutcome(state, provider, model, true, durationMs)

	return nil
}

// assessAndRecommend runs capability assessment and adds recommendation to metadata if needed.
func (s *llmStage) assessAndRecommend(
	ctx context.Context,
	state *PipelineState,
	requestID string,
	provider string,
	model string,
	resp *LLMResponse,
	durationMs int,
) {
	log := logging.Global()

	// Quick recommend if we have basic metrics but no assessor
	if s.o.recommender != nil && s.o.assessor == nil {
		// Check for obvious issues
		hasTimeout := durationMs > 30000 // 30 seconds
		rec := s.o.recommender.QuickRecommend(provider, model, durationMs, hasTimeout, false)

		if rec != nil && eval.ShouldWarnUser(rec) {
			s.addRecommendationToMetadata(state, rec)
			log.Info("[Eval] Quick recommendation: upgrade from %s to %s (reason: %s)",
				rec.CurrentModel, rec.RecommendedModel, rec.Reason)
		}
		return
	}

	// Full assessment if assessor is available
	if s.o.assessor == nil || s.o.convLogger == nil {
		return
	}

	// Use detached context to prevent cancellation from affecting log retrieval
	logCtx, logCancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
	defer logCancel()

	// Get the logged conversation for full assessment
	convLog, err := s.o.convLogger.GetLog(logCtx, requestID)
	if err != nil {
		log.Warn("[Eval] Failed to get conversation log for assessment: %v", err)
		return
	}

	// Run assessment
	assessment := s.o.assessor.AssessAndUpdate(ctx, convLog)

	// Generate recommendation if issues detected
	if s.o.recommender != nil && assessment.HasIssues() {
		var complexityScore int
		if state.Cognitive != nil {
			complexityScore = state.Cognitive.ComplexityScore
		}

		rec := s.o.recommender.Recommend(ctx, provider, model, assessment, complexityScore)

		if rec != nil && eval.ShouldWarnUser(rec) {
			s.addRecommendationToMetadata(state, rec)
			log.Info("[Eval] Model recommendation: upgrade from %s to %s (reason: %s)",
				rec.CurrentModel, rec.RecommendedModel, rec.Reason)
		}
	}
}

// addRecommendationToMetadata adds a model recommendation to the response metadata.
func (s *llmStage) addRecommendationToMetadata(state *PipelineState, rec *eval.Recommendation) {
	if state.Response.Metadata == nil {
		state.Response.Metadata = make(map[string]interface{})
	}

	state.Response.Metadata["model_recommendation"] = map[string]interface{}{
		"current_model":        rec.CurrentModel,
		"current_provider":     rec.CurrentProvider,
		"recommended_model":    rec.RecommendedModel,
		"recommended_provider": rec.RecommendedProvider,
		"reason":               rec.Reason,
		"confidence":           rec.Confidence,
	}
}

// executeAgentic runs the agent for multi-step tool-using tasks.
// If fallback providers are configured, it uses the ResilientAgent for automatic
// timeout recovery with frontier model fallback.
func (s *llmStage) executeAgentic(ctx context.Context, state *PipelineState) error {
	log := logging.Global()
	log.Info("[Agent] Starting agentic execution")

	// Safety check: Ensure state and request are valid
	if state == nil || state.Request == nil {
		return fmt.Errorf("invalid state: state or request is nil")
	}

	// Get provider and model info for logging
	provider := s.o.primaryProvider
	model := s.o.primaryModel

	// Try to infer provider from model name if not set
	if provider == "" {
		provider = inferProviderFromModel(model)
		if provider == "" {
			provider = "unknown"
		}
	}
	providerOverride := ""
	if state.Request.Context != nil {
		if state.Request.Context.ModelOverride != "" {
			model = state.Request.Context.ModelOverride
		}
		if state.Request.Context.ProviderOverride != "" {
			providerOverride = state.Request.Context.ProviderOverride
		}
	}

	// Determine which LLM to use based on provider override
	agentLLM := s.o.agentLLM
	if providerOverride != "" && providerOverride != s.o.primaryProvider {
		// Check if we have a fallback LLM for the requested provider
		if fallbackLLM, ok := s.o.fallbackLLMs[providerOverride]; ok {
			agentLLM = fallbackLLM
			provider = providerOverride
			// CRITICAL: When switching providers, don't use the primary model if it's incompatible
			// MLX models can't run on Ollama, Ollama models can't run on MLX
			if !isModelCompatibleWithProvider(model, providerOverride) {
				log.Warn("[Agent] Model %s incompatible with provider %s, using provider's default", model, providerOverride)
				// Don't set model - let the fallback LLM use its pre-configured model
			} else {
				log.Info("[Agent] Provider override applied: %s (model: %s)", providerOverride, model)
			}
		} else {
			log.Warn("[Agent] Provider override requested (%s) but no fallback LLM configured, using primary", providerOverride)
		}
	}

	// Apply model override to the agent LLM (only if compatible with the provider)
	if agentLLM != nil && model != s.o.primaryModel {
		if providerOverride == "" || isModelCompatibleWithProvider(model, provider) {
			agentLLM.SetModel(model)
			log.Info("[Agent] Model override applied: %s", model)
		}
	}

	// Log request before agent execution (if eval enabled)
	var requestID string
	if s.o.evalEnabled && s.o.convLogger != nil {
		var complexityScore int
		if state.Cognitive != nil {
			complexityScore = state.Cognitive.ComplexityScore
		}
		var sessionID string
		if state.Request.Context != nil {
			sessionID = state.Request.ID
		}

		// Build system prompt for logging (even though agent builds its own)
		// Try to use ContextBuilder if available
		systemPrompt := s.buildSystemPrompt(state)
		if s.o.memoryStore != nil {
			systemPrompt = s.buildSystemPromptWithMemory(ctx, state)
		}

		logReq := &eval.LogRequest{
			SessionID:       sessionID,
			Provider:        provider,
			Model:           model,
			Prompt:          state.Request.Input,
			SystemPrompt:    systemPrompt,
			ComplexityScore: complexityScore,
		}

		// Use detached context to prevent cancellation from affecting logging
		logCtx, logCancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
		defer logCancel()

		var logErr error
		requestID, logErr = s.o.convLogger.LogRequest(logCtx, logReq)
		if logErr != nil {
			log.Warn("[Eval] Failed to log agentic request: %v", logErr)
		}
	}

	// FIX: Save the logged provider/model for outcome recording.
	// These are the values used to CREATE the conversation_log entry.
	// After escalation, provider/model may change, but outcome_logger
	// needs the ORIGINAL values to find and update the log entry.
	loggedProvider := provider
	loggedModel := model

	// Get working directory
	workingDir := "."
	if state.Request.Context != nil && state.Request.Context.WorkingDir != "" {
		workingDir = state.Request.Context.WorkingDir
	}

	// Get the step callback from request context (for streaming)
	var onStep agent.StepCallback
	if state.Request.Context != nil && state.Request.Context.OnAgentStep != nil {
		onStep = state.Request.Context.OnAgentStep
	}

	// Get unrestricted mode from request context
	unrestrictedMode := false
	if state.Request.Context != nil {
		unrestrictedMode = state.Request.Context.UnrestrictedMode
	}

	// Convert history to agent messages
	var history []agent.ChatMessage
	if state.Request.Context != nil {
		for _, msg := range state.Request.Context.History {
			history = append(history, agent.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Build knowledge context from retrieved knowledge
	var knowledgeCtx *agent.KnowledgeContext
	if len(state.Knowledge) > 0 {
		knowledgeCtx = &agent.KnowledgeContext{
			Items: make([]string, 0, len(state.Knowledge)),
		}
		for _, k := range state.Knowledge {
			knowledgeCtx.Items = append(knowledgeCtx.Items, k.Content)
		}
		log.Info("[Agent] Injecting %d knowledge items into context", len(knowledgeCtx.Items))
	}

	// Build user context from memory store
	var userContext string
	if s.o.memoryStore != nil {
		userMem, err := s.o.memoryStore.GetUserMemory(ctx, "default-user")
		if err == nil && userMem != nil {
			userContext = s.buildUserContextString(userMem)
			if userContext != "" {
				log.Info("[Agent] Injecting user memory context (%d chars)", len(userContext))
			}
		}
	}

	// Get persona identity from PersonaCoordinator
	// This separates IDENTITY (who the agent is) from TASK (what the agent does)
	var personaIdentity string
	if s.o.persona != nil {
		personaIdentity = s.o.persona.BuildSystemPrompt(nil)
		if personaIdentity != "" {
			log.Info("[Agent] Injecting persona identity (%d chars)", len(personaIdentity))
		}
	}

	// AGENTIC CAPABILITY CHECK: Prevent small models from being used for agentic tool tasks
	// Small models (<7B params) hallucinate tool outputs and cannot follow tool-use patterns reliably
	// CR-024-FIX: Auto-escalate to capable model to prevent wasteful loops and incorrect answers
	if isTooSmallForAgentic(model) {
		log.Info("[Agent] Model %s too small for agentic tasks - auto-escalating", model)
		upgraded := false

		// Priority 1: Try Ollama with a medium+ local model (7B+)
		if ollamaFallback, ok := s.o.fallbackLLMs["ollama"]; ok {
			if larger := findAgenticCapableModel(model); larger != "" {
				agentLLM = ollamaFallback
				agentLLM.SetModel(larger)
				model = larger
				provider = "ollama"
				log.Info("[Agent] Upgraded to Ollama/%s for agentic capability", larger)
				upgraded = true
			}
		}

		// Priority 2: Use cloud fallback (grok > anthropic > openai > gemini)
		if !upgraded {
			cloudPriority := []string{"grok", "anthropic", "openai", "gemini"}
			for _, cloudProvider := range cloudPriority {
				if cloudFallback, ok := s.o.fallbackLLMs[cloudProvider]; ok {
					agentLLM = cloudFallback
					provider = cloudProvider
					log.Info("[Agent] Upgraded to %s cloud for agentic capability (local model too small)", cloudProvider)
					upgraded = true
					break
				}
			}
		}

		if !upgraded {
			log.Warn("[Agent] No capable fallback available - agentic task may fail with small model")
		}
	}

	// CONTEXT ENHANCEMENT: For personal/memory questions with small models, switch to capable model
	// Small models (<4B params) struggle with reasoning over injected context
	if isPersonalQuestion(state.Request.Input) && userContext != "" && isSmallModel(model) {
		log.Info("[Agent] Personal question + small model detected - seeking capable fallback")
		upgraded := false

		// Priority 1: Try Ollama with larger local model (if available)
		if ollamaFallback, ok := s.o.fallbackLLMs["ollama"]; ok {
			if larger := findLargerLocalModel(model); larger != "" {
				agentLLM = ollamaFallback
				agentLLM.SetModel(larger)
				model = larger
				provider = "ollama"
				log.Info("[Agent] Upgraded to Ollama/%s for context handling", larger)
				upgraded = true
			}
		}

		// Priority 2: Use cloud fallback (grok > anthropic > openai > gemini)
		if !upgraded {
			cloudPriority := []string{"grok", "anthropic", "openai", "gemini"}
			for _, cloudProvider := range cloudPriority {
				if cloudFallback, ok := s.o.fallbackLLMs[cloudProvider]; ok {
					agentLLM = cloudFallback
					provider = cloudProvider
					log.Info("[Agent] Upgraded to %s cloud for context handling (small local model insufficient)", cloudProvider)
					upgraded = true
					break
				}
			}
		}

		if !upgraded {
			log.Warn("[Agent] No capable fallback available, continuing with small model (context may be ignored)")
		}
	}

	// FAST PATH: Personal questions with user context bypass the agentic loop
	// This prevents small models from tool-looping on simple memory questions
	if userContext != "" && isPersonalQuestion(state.Request.Input) {
		log.Info("[Agent] Personal question detected with user context - using direct answer path")
		response, err := s.directAnswerFromContext(ctx, state, userContext, &agentLLMAdapter{agentLLM})
		if err == nil {
			state.LLMResponse = response
			return nil
		}
		log.Warn("[Agent] Direct answer failed, falling back to agentic: %v", err)
	}

	var result *agent.Response
	var err error

	// Create cancellable context for interrupt support (CR-010 Track 3)
	agentCtx, cancel := context.WithCancel(ctx)
	s.o.mu.Lock()
	s.o.currentStreamCtx = agentCtx
	s.o.cancelStream = cancel
	s.o.mu.Unlock()

	defer func() {
		s.o.mu.Lock()
		s.o.cancelStream = nil
		s.o.currentStreamCtx = nil
		s.o.mu.Unlock()
	}()

	// Execute agent with timing
	startTime := time.Now()

	// Publish AgentStartedEvent (CR-010)
	if s.o.eventBus != nil {
		evt := bus.NewAgentStartedEvent("agent", "orchestrator", state.Request.ID, state.Request.Input)
		evt.RequestID = state.Request.ID
		evt.Model = model
		evt.Provider = provider
		evt.Task = state.Request.Input
		s.o.eventBus.Publish(evt)
	}

	// Use ResilientAgent if fallback providers are configured
	if len(s.o.fallbackLLMs) > 0 {
		log.Info("[Agent] Using resilient agent with %d fallback provider(s)", len(s.o.fallbackLLMs))

		// Convert fallback providers to agent.LLMProvider
		fallbackLLMs := make(map[string]agent.LLMProvider)
		for provider, llm := range s.o.fallbackLLMs {
			fallbackLLMs[provider] = &agentLLMAdapter{llm}
		}

		// Create recovery config (use overridden model if set)
		recoveryConfig := agent.DefaultRecoveryConfig()
		if s.o.primaryEndpoint != "" {
			recoveryConfig.PrimaryEndpoint = s.o.primaryEndpoint
		}
		recoveryConfig.PrimaryProvider = provider // Use overridden provider if set
		recoveryConfig.PrimaryModel = model       // Use overridden model if set

		// Set up fallback providers with priority
		// Grok is preferred as primary cloud fallback due to excellent reasoning capabilities
		recoveryConfig.FallbackProviders = []agent.FallbackProvider{
			{Name: "grok", Model: "grok-3", Priority: 1},
			{Name: "anthropic", Model: "claude-sonnet-4-20250514", Priority: 2},
			{Name: "openai", Model: "gpt-4o", Priority: 3},
			{Name: "gemini", Model: "gemini-1.5-pro", Priority: 4},
		}

		// Create memory tools adapter if memory tools are available
		var memToolsIface agent.MemoryToolsInterface
		if s.o.memoryTools != nil {
			memToolsIface = &memoryToolsAdapter{memoryTools: s.o.memoryTools}
		}

		// Create resilient agent with knowledge context
		// Use the local `agentLLM` variable which may be a fallback provider
		resilientAgent := agent.NewResilientAgent(&agentLLMAdapter{agentLLM}, &agent.ResilientConfig{
			PrimaryProvider:   provider, // Use overridden provider if set
			PrimaryModel:      model,    // Use overridden model if set
			PrimaryEndpoint:   s.o.primaryEndpoint,
			FallbackLLMs:      fallbackLLMs,
			WorkingDir:        workingDir,
			MaxSteps:          agent.MaxStepsForModel(model), // CR-024: Model-aware step limits
			OnStep:            onStep,
			Knowledge:         knowledgeCtx,
			UnrestrictedMode:  unrestrictedMode,
			RecoveryConfig:    recoveryConfig,
			OnLearn:           s.o.onTimeoutLearn,
			MemoryTools:       memToolsIface,
			MemoryUserID:      "default-user",
			UserContext:       userContext,
			PersonaIdentity:   personaIdentity,           // Persona identity from PersonaCoordinator
			SupervisedConfig:  &s.o.supervisedConfig,     // Supervised agentic mode config
			CheckpointHandler: s.o.checkpointHandler,     // Checkpoint handler from TUI
		})

		result, err = resilientAgent.Run(agentCtx, state.Request.Input, history)
	} else {
		// Standard agent without resilience
		// Create memory tools adapter if memory tools are available
		var memToolsIface agent.MemoryToolsInterface
		if s.o.memoryTools != nil {
			memToolsIface = &memoryToolsAdapter{memoryTools: s.o.memoryTools}
		}

		ag := agent.New(agentLLM, &agent.Config{
			WorkingDir:        workingDir,
			MaxSteps:          agent.MaxStepsForModel(model), // CR-024: Model-aware step limits
			ModelName:         model,                         // Pass model name for logging
			OnStep:            onStep,
			Knowledge:         knowledgeCtx,
			UnrestrictedMode:  unrestrictedMode,
			MemoryTools:       memToolsIface,
			MemoryUserID:      "default-user",
			UserContext:       userContext,
			PersonaIdentity:   personaIdentity,           // Persona identity from PersonaCoordinator
			SupervisedConfig:  &s.o.supervisedConfig,     // Supervised agentic mode config
			CheckpointHandler: s.o.checkpointHandler,     // Checkpoint handler from TUI
		})
		result, err = ag.Run(agentCtx, state.Request.Input, history)
	}

	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Handle case where result is nil (all providers failed)
	if result == nil {
		if err != nil {
			errStr := err.Error()
			// Check if this is a missing API key error and provide helpful guidance
			if strings.Contains(errStr, "not configured") {
				state.LLMResponse = fmt.Sprintf("⚠️ LLM provider not configured.\n\n%v\n\n"+
					"**Quick Setup:**\n"+
					"• Use `/setkey <provider>` in the TUI to add an API key\n"+
					"• Or add keys to `~/.cortex/.env`\n\n"+
					"**Supported providers:** anthropic, openai, gemini, grok", err)
			} else {
				state.LLMResponse = fmt.Sprintf("I'm sorry, I couldn't process your request. Error: %v\n\n"+
					"If this persists, check:\n"+
					"• Is Ollama running? (`ollama serve`)\n"+
					"• API keys configured? (`/setkey` or `~/.cortex/.env`)", err)
			}
		} else {
			state.LLMResponse = "I'm sorry, I couldn't process your request. The system is temporarily unavailable.\n\n" +
				"Please check that Ollama is running or configure a cloud provider API key."
		}
		log.Error("[Agent] All providers failed, result is nil: %v", err)
		return nil // Return gracefully instead of crashing
	}

	// Check if a fallback provider was used (result.Provider will be set)
	if result.Provider != "" && result.Provider != provider {
		log.Info("[Cognitive] 🔄 MODEL SWITCH: %s/%s → %s/%s", provider, model, result.Provider, result.Model)
		provider = result.Provider
		model = result.Model
	}

	// Build response with steps information
	var response strings.Builder

	// Check if this is voice mode - need natural, spoken responses
	isVoiceMode := state.Request.Context != nil && state.Request.Context.VoiceMode

	if isVoiceMode {
		// Voice mode: Synthesize a natural, conversational response
		synthesized := s.synthesizeVoiceResponse(ctx, state.Request.Input, result.Message, agentLLM)
		response.WriteString(synthesized)
		log.Debug("[Voice] Synthesized response for TTS: %s", truncateLog(synthesized, 100))
	} else {
		// Text mode: Use the original response
		response.WriteString(result.Message)

		// Add tool execution summary if tools were used (text mode only)
		if len(result.ToolsUsed) > 0 {
			response.WriteString("\n\n---\n")
			response.WriteString(fmt.Sprintf("*Executed %d steps using tools: %s*",
				result.StepsCount,
				strings.Join(uniqueStrings(result.ToolsUsed), ", ")))
		}
	}

	// Log response and run assessment (if eval enabled)
	if s.o.evalEnabled && s.o.convLogger != nil && requestID != "" {
		logResp := &eval.LogResponse{
			Response:         response.String(),
			DurationMs:       durationMs,
			Success:          err == nil,
			ContextTokens:    0, // Agent doesn't expose token counts
			CompletionTokens: 0,
		}
		if err != nil {
			logResp.ErrorCode = "agent_error"
			logResp.ErrorMessage = err.Error()
		}

		// Use detached context to prevent cancellation from affecting logging
		logCtx, logCancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
		defer logCancel()

		if logErr := s.o.convLogger.LogResponse(logCtx, requestID, logResp); logErr != nil {
			log.Warn("[Eval] Failed to log agentic response: %v", logErr)
		}

		// Run assessment and check for model recommendations
		// Note: For agentic mode, we assess the overall task completion,
		// not individual tool steps. This helps detect when the primary model
		// struggles with multi-step reasoning and should be upgraded.
		if err == nil {
			// Create a synthetic LLMResponse for assessment compatibility
			synthResp := &LLMResponse{
				Content:    response.String(),
				TokensUsed: 0, // Agent doesn't track tokens
			}
			s.assessAndRecommend(ctx, state, requestID, provider, model, synthResp, durationMs)
		}
	}

	if err != nil {
		log.Error("[Agent] Execution failed: %v", err)
		// Record failure outcome - use loggedProvider/Model to match the conversation_log entry
		s.recordRoutingOutcome(state, loggedProvider, loggedModel, false, durationMs)
		return fmt.Errorf("agent error: %w", err)
	}

	state.LLMResponse = response.String()
	state.LLMProvider = provider  // Use actual provider (may be escalated)
	state.LLMModel = model        // Use actual model (may be escalated)
	state.LLMTokensUsed = result.TokensUsed // Tokens accumulated by agent

	// Record success outcome for RoamPal learning
	// FIX: Use loggedProvider/Model to match the conversation_log entry created at start
	s.recordRoutingOutcome(state, loggedProvider, loggedModel, true, durationMs)

	// Publish AgentCompletedEvent (CR-010)
	if s.o.eventBus != nil {
		evt := bus.NewAgentCompletedEvent("agent", state.Request.ID, true, duration, 0, len(result.ToolsUsed), nil)
		evt.RequestID = state.Request.ID
		evt.Model = model
		evt.Provider = provider
		evt.StepsCount = result.StepsCount
		evt.ToolsUsed = result.ToolsUsed
		evt.DurationMs = int64(durationMs)
		s.o.eventBus.Publish(evt)
	}

	// CR-025: Capture execution trace for skill learning
	// Only learn from successful agentic executions that used tools
	if s.o.skillLibrary != nil && result != nil && len(result.ToolsUsed) > 0 && state != nil && state.Request != nil {
		trace := memory.ExecutionTrace{
			SessionID:     state.Request.ID,
			TraceID:       fmt.Sprintf("trace_%d", time.Now().UnixNano()),
			UserInput:     state.Request.Input,
			TaskSummary:   extractTaskSummary(state.Request.Input),
			GeneratedCode: extractCodeFromResponse(result.Message),
			Success:       true,
			Confidence:    calculateExecutionConfidence(result),
			LatencyMS:     int64(durationMs),
			DetectedTags:  result.ToolsUsed,
			CreatedAt:     time.Now(),
		}

		// Async learning to not block response
		go func() {
			learnCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.o.skillLibrary.LearnFromExecution(learnCtx, trace); err != nil {
				log.Debug("[CR-025] Skill learning failed: %v", err)
			} else {
				log.Info("[CR-025] Learned skill from execution (confidence: %.2f)", trace.Confidence)
			}
		}()
	}

	log.Info("[Agent] Completed with %d steps using %s/%s, tools: %v", result.StepsCount, provider, model, result.ToolsUsed)
	return nil
}

// executeSimpleChat handles simple conversational inputs without tools.
// Uses agentLLM directly for fast responses to greetings and simple questions.
func (s *llmStage) executeSimpleChat(ctx context.Context, state *PipelineState) error {
	log := logging.Global()
	log.Info("[SimpleChat] Direct LLM call for conversational input")

	// Build a simple conversational system prompt
	systemPrompt := `You are Cortex, a friendly and helpful AI assistant.
Respond naturally and conversationally. Be concise but warm.
For greetings, respond with a friendly greeting.
For questions about yourself, explain that you're Cortex, an AI assistant that helps with coding, system administration, and general questions.`

	// Add persona if available
	if persona := s.o.GetActivePersona(); persona != nil && persona.SystemPrompt != "" {
		systemPrompt = persona.SystemPrompt
	}

	// Build messages
	messages := []agent.ChatMessage{
		{Role: "user", Content: state.Request.Input},
	}

	// Add conversation history if available
	if state.Request.Context != nil && len(state.Request.Context.History) > 0 {
		historyMsgs := make([]agent.ChatMessage, 0, len(state.Request.Context.History)+1)
		for _, msg := range state.Request.Context.History {
			historyMsgs = append(historyMsgs, agent.ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
		historyMsgs = append(historyMsgs, messages[0])
		messages = historyMsgs
	}

	// Call LLM directly (no tools)
	startTime := time.Now()
	response, err := s.o.agentLLM.Chat(ctx, messages, systemPrompt)
	duration := time.Since(startTime)
	durationMs := int(duration.Milliseconds())

	// Get provider/model for outcome recording
	provider := s.o.primaryProvider
	model := s.o.primaryModel

	if err != nil {
		log.Error("[SimpleChat] LLM error: %v", err)
		// Record failure outcome
		s.recordRoutingOutcome(state, provider, model, false, durationMs)
		return fmt.Errorf("simple chat error: %w", err)
	}

	log.Info("[SimpleChat] Response in %v: %s", duration, truncateLog(response, 100))
	state.LLMResponse = response
	// For simple chat, use primary provider info (agentLLM is primary)
	state.LLMProvider = provider
	state.LLMModel = model

	// Record success outcome for RoamPal learning
	s.recordRoutingOutcome(state, provider, model, true, durationMs)

	return nil
}

// uniqueStrings returns unique strings from a slice.
func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// truncateLog truncates a string for logging purposes.
func truncateLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// synthesizeVoiceResponse uses the LLM to convert a raw response into natural spoken language.
// This takes the original question and the raw response, and produces a conversational answer
// suitable for text-to-speech output.
func (s *llmStage) synthesizeVoiceResponse(ctx context.Context, question, rawResponse string, llm agent.LLMProvider) string {
	log := logging.Global()

	// If the response is already short and conversational, return it as-is
	if len(rawResponse) < 200 && !strings.Contains(rawResponse, "---") && !strings.Contains(rawResponse, "```") {
		return rawResponse
	}

	// Build a synthesis prompt
	synthesisPrompt := `You are a voice assistant synthesizing a response for text-to-speech.

Your task: Convert the following response into natural, conversational speech.

RULES:
1. Extract the CORE answer - what the user actually wants to know
2. Be conversational and natural, as if speaking to a friend
3. NO markdown, NO bullet points, NO code blocks, NO asterisks
4. NO technical metadata like "Executed X steps" or tool names
5. Keep it SHORT: 1-3 sentences for simple facts, up to 5 for complex answers
6. Use natural speech patterns (contractions, casual phrasing)
7. For temperatures: include both Fahrenheit and Celsius naturally (e.g., "about 30 degrees Fahrenheit, or minus 1 Celsius")
8. For numbers: round to simple values when appropriate
9. Start with the answer, don't repeat the question
10. Sound human, not robotic

ORIGINAL QUESTION: ` + question + `

RAW RESPONSE TO SYNTHESIZE:
` + rawResponse + `

NATURAL SPOKEN RESPONSE:`

	// Use the same LLM to synthesize the response
	messages := []agent.ChatMessage{
		{Role: "user", Content: synthesisPrompt},
	}

	synthesized, err := llm.Chat(ctx, messages, "You are a helpful voice assistant that speaks naturally.")
	if err != nil {
		log.Warn("[Voice] Failed to synthesize voice response: %v, using original", err)
		// Fall back to stripping metadata from original
		return stripMetadata(rawResponse)
	}

	// Clean up any accidental formatting in the synthesized response
	synthesized = strings.TrimSpace(synthesized)
	synthesized = stripMetadata(synthesized)

	return synthesized
}

// stripMetadata removes technical metadata and formatting from a response.
func stripMetadata(response string) string {
	// Remove the "---" separator and everything after it
	if idx := strings.Index(response, "\n---\n"); idx != -1 {
		response = response[:idx]
	}
	if idx := strings.Index(response, "\n\n---\n"); idx != -1 {
		response = response[:idx]
	}

	// Remove markdown formatting
	response = strings.ReplaceAll(response, "**", "")
	response = strings.ReplaceAll(response, "*", "")
	response = strings.ReplaceAll(response, "`", "")

	// Remove code blocks
	for strings.Contains(response, "```") {
		start := strings.Index(response, "```")
		end := strings.Index(response[start+3:], "```")
		if end == -1 {
			response = response[:start]
			break
		}
		response = response[:start] + response[start+3+end+3:]
	}

	return strings.TrimSpace(response)
}

// agentLLMAdapter adapts AgentLLMProvider to agent.LLMProvider interface.
// It also implements agent.TokenAccumulator if the underlying LLM supports it.
type agentLLMAdapter struct {
	llm AgentLLMProvider
}

// Chat implements agent.LLMProvider.
func (a *agentLLMAdapter) Chat(ctx context.Context, messages []agent.ChatMessage, systemPrompt string) (string, error) {
	return a.llm.Chat(ctx, messages, systemPrompt)
}

// GetTotalTokens implements agent.TokenAccumulator if the underlying LLM supports it.
func (a *agentLLMAdapter) GetTotalTokens() int {
	if accumulator, ok := a.llm.(agent.TokenAccumulator); ok {
		return accumulator.GetTotalTokens()
	}
	return 0
}

// ResetTokens implements agent.TokenAccumulator if the underlying LLM supports it.
func (a *agentLLMAdapter) ResetTokens() {
	if accumulator, ok := a.llm.(agent.TokenAccumulator); ok {
		accumulator.ResetTokens()
	}
}

// memoryToolsAdapter adapts memory.MemoryTools to agent.MemoryToolsInterface.
type memoryToolsAdapter struct {
	memoryTools *memory.MemoryTools
}

// ExecuteTool implements agent.MemoryToolsInterface.
func (m *memoryToolsAdapter) ExecuteTool(ctx context.Context, userID, toolName, argsJSON string) (*agent.MemoryToolResult, error) {
	result, err := m.memoryTools.ExecuteTool(ctx, userID, toolName, argsJSON)
	if err != nil {
		return nil, err
	}
	// Convert memory.ToolResult to agent.MemoryToolResult
	return &agent.MemoryToolResult{
		Success:   result.Success,
		Result:    result.Result,
		Error:     result.Error,
		LatencyMs: result.LatencyMs,
	}, nil
}

// buildSystemPrompt creates a context-aware system prompt.
func (s *llmStage) buildSystemPrompt(state *PipelineState) string {
	var sb strings.Builder

	// CR-016 Phase 3: Inject Voice Agent's system prompt if provided
	// This takes highest priority - the agent prompt replaces the default persona/prompt
	hasAgentPrompt := state.Request.Context != nil && state.Request.Context.AgentSystemPrompt != ""
	if hasAgentPrompt {
		sb.WriteString(state.Request.Context.AgentSystemPrompt)
	} else {
		// CR-011: Use active persona's system prompt if available
		activePersona := s.o.GetActivePersona()
		if activePersona != nil {
			sb.WriteString(activePersona.SystemPrompt)

			// Add mode-specific augmentation if active mode is set
			activeMode := s.o.GetActiveMode()
			if activeMode != persona.ModeNormal && activeMode != "" {
				if mode := activePersona.GetMode(string(activeMode)); mode != nil {
					if mode.PromptAugment != "" {
						sb.WriteString("\n\n")
						sb.WriteString(mode.PromptAugment)
					}
				}
			}
		} else {
			// Base system prompt (fallback if no persona is set)
			sb.WriteString(`You are Cortex, an intelligent AI assistant for software development and system administration.
You help users with coding, debugging, DevOps, and general technical questions.
Be concise, accurate, and helpful. When providing commands, explain what they do.`)
		}
	}

	// Add platform context if available
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		fp := state.Request.Context.Fingerprint
		sb.WriteString("\n\n## Current Environment\n")
		if fp.Platform != "" {
			sb.WriteString(fmt.Sprintf("- Platform: %s\n", fp.Platform))
		}
		if fp.ProjectType != "" {
			sb.WriteString(fmt.Sprintf("- Project Type: %s\n", fp.ProjectType))
		}
		if fp.ProjectRoot != "" {
			sb.WriteString(fmt.Sprintf("- Working Directory: %s\n", fp.ProjectRoot))
		}
	} else if state.Request.Context != nil && state.Request.Context.WorkingDir != "" {
		sb.WriteString(fmt.Sprintf("\n\n## Current Environment\n- Working Directory: %s\n", state.Request.Context.WorkingDir))
	}

	// Add relevant knowledge if available
	if len(state.Knowledge) > 0 {
		sb.WriteString("\n\n## Relevant Knowledge\n")
		for i, k := range state.Knowledge {
			if i >= 3 { // Limit to top 3 to not overload context
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", truncateString(k.Content, 200)))
		}
	}

	// Add routing context
	if state.Routing != nil {
		sb.WriteString(fmt.Sprintf("\n\n## Task Classification\nThis appears to be a %s task.", state.Routing.TaskType))

		// Add specialist guidance
		spec := s.o.GetSpecialist(state.Routing.TaskType)
		if spec != nil && spec.SystemPrompt != "" {
			sb.WriteString("\n\n## Specialist Guidance\n")
			sb.WriteString(spec.SystemPrompt)
		}
	}

	// Voice Mode: Inject voice-optimized prompts (CR-011 Phase 3)
	// Uses the comprehensive voice prompt system from internal/voice
	if state.Request.Context != nil && state.Request.Context.VoiceMode {
		// Prepend voice system prompt before the base prompt
		basePrompt := sb.String()
		sb.Reset()

		// Add the voice system prompt at the top for priority
		sb.WriteString(`You are Cortex, a voice-first terminal AI assistant.

CRITICAL: You are having a SPOKEN conversation. Your responses will be read aloud.

## Voice Response Rules
1. Be conversational, not encyclopedic
2. Lead with the answer - state the key point first
3. Never use formatting (no markdown, bullets, code blocks, asterisks)
4. Numbers and symbols - speak naturally (say "42 degrees" not "42°F", "about 3 percent" not "~3%")
5. Keep it SHORT - 1-3 sentences for simple questions, up to 5 for complex ones
6. Use natural speech markers (Got it, Okay, Sure, Alright, So)
7. Skip pleasantries for commands - just confirm completion
8. Error explanations are concise and actionable
9. NEVER output code blocks - describe verbally instead
10. Avoid abbreviations - spell them out or speak naturally
11. Use contractions like you would in speech (it's, don't, can't)
12. When listing items, say "first", "second", "third" instead of numbered lists
13. For file paths, simplify when possible (say "in the src folder" not full paths)
14. If the task is complete, just say "Done" or "All set"

## Code Guidance for Voice
When discussing code in voice mode:
- NEVER output code blocks - describe what the code does instead
- Explain syntax verbally
- For simple code, describe it step by step
- If asked to write code to a file, do it silently and confirm

## Error Guidance for Voice
When reporting errors:
- State what went wrong in one sentence
- If there's an obvious fix, suggest it immediately
- Don't read stack traces - summarize the problem

`)
		sb.WriteString(basePrompt)
	}

	// Inject strategic principles if available
	if len(state.StrategicPrinciples) > 0 {
		sb.WriteString("\n\n## Strategic Guidelines\n")
		sb.WriteString("Apply these learned principles when relevant:\n\n")
		for i, p := range state.StrategicPrinciples {
			if p.Confidence < 0.5 {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s", p.Principle))
			if p.Category != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", p.Category))
			}
			sb.WriteString("\n")
			if i >= 4 {
				break
			}
		}
	}

	return sb.String()
}

// buildMessages converts request history to LLM messages.
func (s *llmStage) buildMessages(state *PipelineState) []Message {
	messages := make([]Message, 0)

	// Add conversation history if available
	if state.Request.Context != nil && len(state.Request.Context.History) > 0 {
		messages = append(messages, state.Request.Context.History...)
	}

	// Add current user message
	messages = append(messages, Message{
		Role:    "user",
		Content: state.Request.Input,
	})

	return messages
}

func (s *llmStage) buildUserContextString(userMem *memory.UserMemory) string {
	if userMem == nil {
		return ""
	}

	var sb strings.Builder

	if userMem.Name != "" {
		sb.WriteString(fmt.Sprintf("User's name: %s\n", userMem.Name))
	}
	if userMem.Role != "" {
		sb.WriteString(fmt.Sprintf("User's role: %s\n", userMem.Role))
	}
	if userMem.OS != "" {
		sb.WriteString(fmt.Sprintf("User's OS: %s\n", userMem.OS))
	}
	if userMem.Shell != "" {
		sb.WriteString(fmt.Sprintf("User's shell: %s\n", userMem.Shell))
	}
	if userMem.PrefersConcise {
		sb.WriteString("User prefers concise responses.\n")
	}

	if len(userMem.CustomFacts) > 0 {
		sb.WriteString("\nKnown facts about user:\n")
		for _, f := range userMem.CustomFacts {
			sb.WriteString(fmt.Sprintf("- %s\n", f.Fact))
		}
	}

	return sb.String()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// buildRecentHistorySummary extracts key facts from recent conversation history.
// This ensures ephemeral conversation context (like recent travel plans mentioned)
// is available for direct answer responses.
func buildRecentHistorySummary(history []Message, maxExchanges int) string {
	if len(history) == 0 {
		return "No recent conversation."
	}

	var sb strings.Builder
	sb.WriteString("Recent conversation context:\n")

	// Get the last N exchanges (each exchange = 2 messages: user + assistant)
	start := 0
	if len(history) > maxExchanges*2 {
		start = len(history) - maxExchanges*2
	}

	for i := start; i < len(history); i++ {
		msg := history[i]
		role := "User"
		if msg.Role == "assistant" {
			role = "You"
		}
		content := msg.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s: %s\n", role, content))
	}

	return sb.String()
}

// directAnswerFromContext generates a response using user context AND conversation history
// without the agentic loop. This ensures both persistent memory (UserMemory) and
// ephemeral conversation context are available for answering personal questions.
func (s *llmStage) directAnswerFromContext(ctx context.Context, state *PipelineState, userContext string, agentLLM agent.LLMProvider) (string, error) {
	log := logging.Global()
	log.Info("[DirectAnswer] Bypassing agentic loop for personal question")

	// Build recent conversation history summary (last 5 exchanges)
	var historySummary string
	if state.Request.Context != nil && len(state.Request.Context.History) > 0 {
		historySummary = buildRecentHistorySummary(state.Request.Context.History, 5)
		log.Info("[DirectAnswer] Including %d history messages in context", len(state.Request.Context.History))
	} else {
		historySummary = "No recent conversation."
	}

	// Build a simple prompt that focuses on answering from BOTH user context AND conversation history
	systemPrompt := `You are Cortex, a helpful AI assistant with memory about the user.

## USER INFORMATION (PERSISTENT MEMORY)
` + userContext + `

## RECENT CONVERSATION (EPHEMERAL CONTEXT)
` + historySummary + `

## INSTRUCTIONS
- Check BOTH the USER INFORMATION section AND the RECENT CONVERSATION for relevant details
- If the user mentioned something recently in conversation (like travel plans, preferences, topics), USE IT
- Be conversational and direct
- If the information is in either section above, answer immediately
- If the information is NOT in either section, say "I don't have that information stored"
- Do NOT use any tools - just answer from what you know
- Keep responses concise (1-3 sentences for simple questions)`

	messages := []agent.ChatMessage{
		{Role: "user", Content: state.Request.Input},
	}

	response, err := agentLLM.Chat(ctx, messages, systemPrompt)
	if err != nil {
		return "", fmt.Errorf("direct answer failed: %w", err)
	}

	formatted := fmt.Sprintf("[!MEMORY]\n%s\n\n", strings.TrimSpace(response))
	return formatted, nil
}

// isModelCompatibleWithProvider checks if a model can run on a given provider.
// MLX models (mlx-community/*) can't run on Ollama, and vice versa.
func isModelCompatibleWithProvider(model, provider string) bool {
	modelLower := strings.ToLower(model)

	// MLX models only work with MLX provider
	isMlxModel := strings.HasPrefix(modelLower, "mlx") ||
		strings.Contains(modelLower, "mlx-community/")
	if isMlxModel {
		return provider == "mlx"
	}

	// Ollama models (with colon like llama:7b) only work with Ollama
	isOllamaModel := strings.Contains(model, ":") &&
		!strings.Contains(modelLower, "claude") &&
		!strings.Contains(modelLower, "gpt") &&
		!strings.Contains(modelLower, "gemini") &&
		!strings.Contains(modelLower, "grok")
	if isOllamaModel {
		return provider == "ollama"
	}

	// Cloud models
	if strings.Contains(modelLower, "claude") {
		return provider == "anthropic"
	}
	if strings.HasPrefix(modelLower, "gpt") || strings.HasPrefix(modelLower, "o1") {
		return provider == "openai"
	}
	if strings.Contains(modelLower, "gemini") {
		return provider == "gemini" || provider == "google"
	}
	if strings.HasPrefix(modelLower, "grok") {
		return provider == "grok"
	}

	// For unknown models, assume compatible (will fail at runtime if not)
	return true
}

// isSmallModel returns true for models with <4B parameters.
// Small models often struggle with complex reasoning over injected context.
func isSmallModel(model string) bool {
	lower := strings.ToLower(model)
	smallPatterns := []string{
		"1b", "2b", "3b", "1.5b", "0.5b",
		"tiny", "mini", "nano",
		"phi-2", "gemma-2b",
	}
	for _, p := range smallPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// findLargerLocalModel returns a larger model name if available.
// Priority list: Qwen (best tool calling) > Llama > Mistral
func findLargerLocalModel(currentModel string) string {
	largerModels := []string{
		"qwen2.5:7b",
		"qwen2.5:14b",
		"llama3.1:8b",
		"llama3:8b",
		"mistral:7b",
	}

	// Return first model that's different from current (avoids same-size recommendation)
	currentLower := strings.ToLower(currentModel)
	for _, m := range largerModels {
		// Skip if current model family is already this size or larger
		modelFamily := strings.Split(m, ":")[0]
		if strings.Contains(currentLower, modelFamily) {
			continue
		}
		return m
	}

	// Fallback: return first larger model
	if len(largerModels) > 0 {
		return largerModels[0]
	}
	return ""
}

// isTooSmallForAgentic returns true if a model is too small for reliable agentic tool use.
// Models under 7B parameters consistently hallucinate tool outputs and cannot follow
// tool-use patterns reliably. CR-024-FIX: Minimum tier enforcement.
func isTooSmallForAgentic(model string) bool {
	lower := strings.ToLower(model)

	// Small models that should NOT be used for agentic tasks
	tooSmallPatterns := []string{
		"1b", "2b", "3b", "4b",      // Size indicators
		"1.5b", "0.5b", "0.6b",      // Fractional sizes
		"tiny", "mini", "nano",      // Size names
		"phi-2", "gemma-2b",         // Specific small models
		"smollm", "tinyllama",       // Named small models
	}
	for _, p := range tooSmallPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// findAgenticCapableModel returns a model suitable for agentic tool use (7B+).
// Priority: Qwen (62% tool accuracy) > Llama > Mistral
// CR-024-FIX: Ensures minimum capability for agentic tasks.
func findAgenticCapableModel(currentModel string) string {
	// Models proven to work reliably with tool use (7B+ params)
	// Qwen has 62% tool calling accuracy vs Llama's 20%
	agenticCapableModels := []string{
		"qwen2.5-coder:7b",
		"qwen2.5:7b",
		"qwen2.5-coder:14b",
		"qwen2.5:14b",
		"llama3.1:8b",
		"llama3:8b",
		"mistral:7b",
	}

	// Return first capable model (Qwen preferred)
	if len(agenticCapableModels) > 0 {
		return agenticCapableModels[0]
	}
	return ""
}

// recordRoutingOutcome logs the routing outcome for RoamPal learning.
// This is called asynchronously after LLM responses to avoid blocking.
// Safe to call even if outcomeLogger is nil (gracefully skipped).
func (s *llmStage) recordRoutingOutcome(
	state *PipelineState,
	provider string,
	model string,
	success bool,
	latencyMs int,
) {
	// Skip if logger not configured
	if s.o.outcomeLogger == nil {
		return
	}

	log := logging.Global()

	// Build outcome from state and response info
	outcome := &eval.RoutingOutcome{
		ModelSelected:  model,
		OutcomeSuccess: success,
		LatencyMs:      latencyMs,
	}

	// Derive lane from provider (local providers = fast lane, cloud = smart lane)
	if isLocalProvider(provider) {
		outcome.Lane = "fast"
		outcome.Reason = "Local inference (fast lane)"
	} else {
		outcome.Lane = "smart"
		outcome.Reason = "Cloud/frontier model (smart lane)"
	}

	// Derive outcome score (simple initial implementation)
	// 1.0 for success, 0.0 for failure
	// TODO: Future enhancement - derive from response quality metrics
	if success {
		outcome.OutcomeScore = 1.0
	} else {
		outcome.OutcomeScore = 0.0
	}

	// Get task type from routing decision if available
	taskType := ""
	if state.Routing != nil {
		taskType = string(state.Routing.TaskType)
	}

	// Record outcome asynchronously to avoid blocking response
	go func() {
		// Use a detached context with timeout for async logging
		logCtx, logCancel := logging.DetachContextWithTimeout(context.Background(), 5*time.Second)
		defer logCancel()

		if err := s.o.outcomeLogger.LogOutcome(logCtx, outcome, provider, model, taskType, latencyMs); err != nil {
			log.Warn("[OutcomeLogger] Failed to record routing outcome: %v", err)
		} else {
			log.Debug("[OutcomeLogger] Recorded outcome: lane=%s, provider=%s, model=%s, success=%v, latency=%dms",
				outcome.Lane, provider, model, success, latencyMs)
		}
	}()
}

// ============================================================================
// CR-025: EXECUTION TRACE HELPERS
// ============================================================================

// extractTaskSummary extracts a task summary from user input.
// Returns the first sentence or first 100 characters as a summary.
func extractTaskSummary(input string) string {
	// First sentence or first 100 chars
	if idx := strings.Index(input, "."); idx > 0 && idx < 100 {
		return input[:idx+1]
	}
	if len(input) > 100 {
		return input[:100] + "..."
	}
	return input
}

// extractCodeFromResponse extracts code blocks from a response.
// Looks for markdown code blocks (```...```) and returns the first one.
func extractCodeFromResponse(response string) string {
	// Find markdown code blocks
	if idx := strings.Index(response, "```"); idx >= 0 {
		// Skip the opening ``` and any language identifier
		start := idx + 3
		// Find the newline after language identifier (if any)
		if nlIdx := strings.Index(response[start:], "\n"); nlIdx >= 0 {
			start += nlIdx + 1
		}
		// Find the closing ```
		endIdx := strings.Index(response[start:], "```")
		if endIdx > 0 {
			return strings.TrimSpace(response[start : start+endIdx])
		}
	}
	return ""
}

// calculateExecutionConfidence calculates confidence score for an execution.
// Based on tool usage patterns and step count efficiency.
func calculateExecutionConfidence(result *agent.Response) float64 {
	if result == nil {
		return 0.0
	}

	// Base confidence for successful completion
	confidence := 0.7

	// Bonus for using tools (indicates task complexity was handled)
	if len(result.ToolsUsed) > 0 {
		confidence += 0.1
	}

	// Bonus for efficient step count (2-5 steps is optimal)
	if result.StepsCount >= 2 && result.StepsCount <= 5 {
		confidence += 0.1
	}

	// Penalty for too many steps (could indicate struggle)
	if result.StepsCount > 10 {
		confidence -= 0.1
	}

	// Bonus for completion flag
	if result.Completed {
		confidence += 0.05
	}

	// Cap at 0.95 (never 100% confident)
	if confidence > 0.95 {
		confidence = 0.95
	}

	// Floor at 0.0
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}
