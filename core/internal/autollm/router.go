package autollm

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/normanking/cortex/internal/eval"
	"github.com/normanking/cortex/internal/knowledge"
	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/prompts"
)

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTER
// ═══════════════════════════════════════════════════════════════════════════════

// Router selects the appropriate model for each request using a two-lane system.
// Fast lane: Local → Groq → Cheap Cloud (default)
// Smart lane: Frontier models (user-requested or forced by constraints)
type Router struct {
	config          RouterConfig
	models          map[string]*eval.ModelCapability // Indexed by model name
	availability    *AvailabilityChecker
	scorer          *eval.CapabilityScorer
	promptStore     *prompts.Store // Build-time optimized prompts
	passiveRetriever *memory.PassiveRetriever // Passive knowledge retrieval for Fast Lane
	log             *logging.Logger

	// Phase 2.5: Learned confidence-based routing
	outcomeStore  OutcomeStore         // Historical outcome data for learning (optional)
	learnedConfig LearnedRoutingConfig // Thresholds for confidence adjustment

	// Lane usage counters (atomic for thread safety)
	fastLaneCount  int64
	smartLaneCount int64
}

// NewRouter creates a new model router without knowledge fabric.
// For passive retrieval support, use NewRouterWithKnowledge instead.
func NewRouter(config RouterConfig, availability *AvailabilityChecker) *Router {
	return NewRouterWithKnowledge(config, availability, nil)
}

// NewRouterWithKnowledge creates a new model router with passive retrieval support.
// If fabric is nil, passive retrieval will be disabled.
func NewRouterWithKnowledge(config RouterConfig, availability *AvailabilityChecker, fabric knowledge.KnowledgeFabric) *Router {
	r := &Router{
		config:        config,
		models:        make(map[string]*eval.ModelCapability),
		availability:  availability,
		scorer:        eval.NewCapabilityScorer(),
		promptStore:   prompts.Load(), // Load build-time optimized prompts
		log:           logging.Global(),
		learnedConfig: DefaultLearnedRoutingConfig(), // Phase 2.5 defaults
	}

	// Initialize passive retriever if knowledge fabric is provided
	if fabric != nil {
		retrieverConfig := memory.DefaultPassiveRetrievalConfig()
		r.passiveRetriever = memory.NewPassiveRetriever(fabric, retrieverConfig)
		r.log.Info("[Router] Passive retrieval enabled (50ms timeout)")
	}

	// Pre-populate model capabilities from registry
	r.loadModelCapabilities()

	return r
}

// loadModelCapabilities pre-fetches capabilities for all configured models.
func (r *Router) loadModelCapabilities() {
	allModels := append(r.config.FastModels, r.config.SmartModels...)

	for _, modelName := range allModels {
		provider := r.detectProvider(modelName)
		cap := r.scorer.GetCapabilities(provider, modelName)
		if cap != nil {
			r.models[modelName] = cap
		}
	}
}

// SetOutcomeStore sets the outcome store for learned confidence-based routing.
// When set, the router uses historical outcome data to adjust routing decisions.
// This enables Phase 2.5 (Learned Routing) in the routing algorithm.
func (r *Router) SetOutcomeStore(store OutcomeStore) {
	r.outcomeStore = store
	if store != nil {
		r.log.Info("[Router] Outcome store enabled for learned routing (Phase 2.5)")
	}
}

// SetLearnedRoutingConfig sets custom thresholds for learned routing.
// Use DefaultLearnedRoutingConfig() for sensible defaults.
func (r *Router) SetLearnedRoutingConfig(config LearnedRoutingConfig) {
	r.learnedConfig = config
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING LOGIC
// ═══════════════════════════════════════════════════════════════════════════════

// Route determines which model to use for a request.
// This implements the four-phase routing algorithm:
//
//	Phase 1: Hard Constraints (Physics) - Cannot override
//	Phase 2: User Intent (Agency) - Respect explicit choice
//	Phase 2.5: Learned Confidence (Adaptive) - Adjust based on historical outcomes
//	Phase 3: Default Fast Lane - Local first, then cloud
func (r *Router) Route(req Request) RoutingDecision {
	return r.RouteWithContext(context.Background(), req)
}

// RouteWithContext determines which model to use for a request with context support.
// Context is used for Phase 2.5 (Learned Routing) to query the outcome store.
func (r *Router) RouteWithContext(ctx context.Context, req Request) RoutingDecision {
	needsVision := len(req.Images) > 0

	// =========================================================================
	// PHASE 1: Hard Constraints (Physics)
	// These override everything - you can't send images to a non-vision model
	// =========================================================================

	// 1a. Vision requirement - check if any available fast model supports vision
	if needsVision {
		hasVisionFast := false
		for _, modelName := range r.config.FastModels {
			m := r.getModel(modelName)
			if m != nil && m.Capabilities.Vision && r.isAvailable(modelName, m.Provider) {
				hasVisionFast = true
				break
			}
		}
		if !hasVisionFast {
			return r.selectSmartModel("vision required, no fast model supports it", true, "vision")
		}
	}

	// 1b. Context length - check per-model (NOT global limit)
	if req.EstimatedTokens > 0 {
		canHandleFast := r.canFastLaneHandle(req.EstimatedTokens, needsVision, req.LocalOnly)
		if !canHandleFast {
			if req.LocalOnly {
				// User demanded local, but no local model can handle it
				// Return best local anyway with warning in reason
				return r.selectFastModel(
					fmt.Sprintf("context %d exceeds local model limits (--local forced)", req.EstimatedTokens),
					req.LocalOnly,
					needsVision,
					0, // No context filter when forced local
				)
			}
			return r.selectSmartModel(
				fmt.Sprintf("context %d exceeds all fast model limits", req.EstimatedTokens),
				true,
				"context_overflow",
			)
		}
	}

	// =========================================================================
	// PHASE 2: User Intent (Agency)
	// Respect explicit mode selection
	// =========================================================================

	if req.Mode == LaneSmart {
		return r.selectSmartModel("user requested --strong", false, "")
	}

	// =========================================================================
	// PHASE 2.5: Learned Confidence (Adaptive)
	// Adjust routing based on historical outcome data
	// =========================================================================

	if r.outcomeStore != nil && req.TaskType != "" {
		decision := r.applyLearnedRouting(ctx, req, needsVision)
		if decision != nil {
			return *decision
		}
	}

	// =========================================================================
	// PHASE 3: Default to Fast Lane
	// Priority: Local → Groq → Cheap Cloud
	// =========================================================================

	return r.selectFastModel("default fast lane", req.LocalOnly, needsVision, req.EstimatedTokens)
}

// applyLearnedRouting applies Phase 2.5 learned confidence adjustments.
// Returns a routing decision if learned data suggests a preference, nil otherwise.
func (r *Router) applyLearnedRouting(ctx context.Context, req Request, needsVision bool) *RoutingDecision {
	// Check if smart lane has significantly better historical performance
	smartLaneConf := CalculateLaneConfidence(ctx, 0.5, LaneSmart, req.TaskType, r.outcomeStore, r.learnedConfig)
	fastLaneConf := CalculateLaneConfidence(ctx, 0.5, LaneFast, req.TaskType, r.outcomeStore, r.learnedConfig)

	// If smart lane has significantly higher learned confidence, consider escalating
	if smartLaneConf.SampleCount >= r.learnedConfig.MinSamplesForConfidence &&
		fastLaneConf.SampleCount >= r.learnedConfig.MinSamplesForConfidence {
		confidenceDiff := smartLaneConf.AdjustedConfidence - fastLaneConf.AdjustedConfidence

		// If smart lane is significantly better (difference > half of max adjustment)
		// and we're not in local-only mode, consider smart lane
		if confidenceDiff > r.learnedConfig.MaxConfidenceAdjustment/2 && !req.LocalOnly {
			r.log.Info("[Phase2.5] Smart lane preferred for task=%s (smart=%.2f, fast=%.2f, diff=%.2f)",
				req.TaskType, smartLaneConf.AdjustedConfidence, fastLaneConf.AdjustedConfidence, confidenceDiff)
			decision := r.selectSmartModel(
				fmt.Sprintf("learned routing: smart lane %.0f%% success vs fast lane %.0f%%",
					smartLaneConf.LearnedConfidence*100, fastLaneConf.LearnedConfidence*100),
				false, "")
			decision.LearnedConfidence = smartLaneConf.AdjustedConfidence
			return &decision
		}
	}

	// Check for specific model preferences in fast lane
	for _, modelName := range r.config.FastModels {
		m := r.getModel(modelName)
		if m == nil {
			continue
		}

		// Skip if doesn't meet basic requirements
		if req.LocalOnly && !IsLocalProvider(m.Provider) {
			continue
		}
		if needsVision && !m.Capabilities.Vision {
			continue
		}
		if req.EstimatedTokens > 0 && m.ContextWindow < req.EstimatedTokens {
			continue
		}
		if !r.isAvailable(modelName, m.Provider) {
			continue
		}

		// Check if this model should be preferred based on learned confidence
		prefer, confidence := ShouldPreferModel(ctx, m.Provider, modelName, req.TaskType, r.outcomeStore, r.learnedConfig)
		if prefer {
			r.log.Info("[Phase2.5] Preferred model %s for task=%s (confidence=%.2f, samples=%d)",
				modelName, req.TaskType, confidence.AdjustedConfidence, confidence.SampleCount)
			atomic.AddInt64(&r.fastLaneCount, 1)
			return &RoutingDecision{
				Model:             modelName,
				Lane:              LaneFast,
				Provider:          m.Provider,
				Reason:            fmt.Sprintf("learned preference: %.0f%% success rate", confidence.LearnedConfidence*100),
				Forced:            false,
				LearnedConfidence: confidence.AdjustedConfidence,
				ModelCapability:   m,
			}
		}

		// Check if this model should be avoided based on poor performance
		avoid, _ := ShouldAvoidModel(ctx, m.Provider, modelName, req.TaskType, r.outcomeStore, r.learnedConfig)
		if avoid {
			r.log.Debug("[Phase2.5] Avoiding model %s for task=%s due to poor historical performance",
				modelName, req.TaskType)
			continue // Skip to next model in priority order
		}
	}

	// No strong preference from learned data, fall through to Phase 3
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// FAST LANE SELECTION
// ═══════════════════════════════════════════════════════════════════════════════

// canFastLaneHandle checks if any fast model can handle the token count.
func (r *Router) canFastLaneHandle(tokens int, needsVision bool, localOnly bool) bool {
	for _, modelName := range r.config.FastModels {
		m := r.getModel(modelName)
		if m == nil {
			continue
		}

		// If local-only, skip cloud models
		if localOnly && !IsLocalProvider(m.Provider) {
			continue
		}

		// If vision required, skip non-vision models
		if needsVision && !m.Capabilities.Vision {
			continue
		}

		// Check availability AND context capacity
		if r.isAvailable(modelName, m.Provider) && m.ContextWindow >= tokens {
			return true
		}
	}
	return false
}

// selectFastModel picks the best available fast model (strict priority order).
// If minContextWindow > 0, models must support at least that many tokens.
func (r *Router) selectFastModel(reason string, localOnly bool, needsVision bool, minContextWindow int) RoutingDecision {
	for _, modelName := range r.config.FastModels {
		m := r.getModel(modelName)
		if m == nil {
			continue
		}

		// Skip cloud if local-only mode
		if localOnly && !IsLocalProvider(m.Provider) {
			continue
		}

		// Skip non-vision models if vision is needed
		if needsVision && !m.Capabilities.Vision {
			continue
		}

		// Skip models with insufficient context window
		if minContextWindow > 0 && m.ContextWindow < minContextWindow {
			continue
		}

		if r.isAvailable(modelName, m.Provider) {
			atomic.AddInt64(&r.fastLaneCount, 1)
			r.log.Info("[Lane-Fast] Selected %s (provider=%s, reason=%s) [total=%d]",
				modelName, m.Provider, reason, atomic.LoadInt64(&r.fastLaneCount))
			return RoutingDecision{
				Model:           modelName,
				Lane:            LaneFast,
				Provider:        m.Provider,
				Reason:          reason,
				Forced:          false,
				ModelCapability: m,
			}
		}
	}

	// No fast models available
	if localOnly {
		r.log.Warn("[Lane-Fast] No local models available (--local specified)")
		// Can't fall through to cloud - return error state
		return RoutingDecision{
			Model:      "",
			Lane:       LaneFast,
			Reason:     "no local models available (--local specified)",
			Forced:     true,
			Constraint: "no_local_models",
		}
	}

	// Fall through to smart lane
	r.log.Info("[Lane-Fast] No fast models available, falling back to smart lane")
	return r.selectSmartModel("no fast models available", true, "no_fast_models")
}

// ═══════════════════════════════════════════════════════════════════════════════
// SMART LANE SELECTION
// ═══════════════════════════════════════════════════════════════════════════════

// selectSmartModel picks the best available smart model.
func (r *Router) selectSmartModel(reason string, forced bool, constraint string) RoutingDecision {
	for _, modelName := range r.config.SmartModels {
		m := r.getModel(modelName)
		if m == nil {
			continue
		}

		if r.isAvailable(modelName, m.Provider) {
			atomic.AddInt64(&r.smartLaneCount, 1)
			r.log.Info("[Lane-Smart] Selected %s (provider=%s, reason=%s) [total=%d]",
				modelName, m.Provider, reason, atomic.LoadInt64(&r.smartLaneCount))
			return RoutingDecision{
				Model:           modelName,
				Lane:            LaneSmart,
				Provider:        m.Provider,
				Reason:          reason,
				Forced:          forced,
				Constraint:      constraint,
				ModelCapability: m,
			}
		}
	}

	// Try default smart model as last resort
	if r.config.DefaultSmartModel != "" {
		m := r.getModel(r.config.DefaultSmartModel)
		provider := ""
		if m != nil {
			provider = m.Provider
		} else {
			provider = r.detectProvider(r.config.DefaultSmartModel)
		}

		if r.isAvailable(r.config.DefaultSmartModel, provider) {
			atomic.AddInt64(&r.smartLaneCount, 1)
			r.log.Info("[Lane-Smart] Selected default %s (provider=%s, reason=%s) [total=%d]",
				r.config.DefaultSmartModel, provider, reason+" (fallback)", atomic.LoadInt64(&r.smartLaneCount))
			return RoutingDecision{
				Model:           r.config.DefaultSmartModel,
				Lane:            LaneSmart,
				Provider:        provider,
				Reason:          reason + " (fallback)",
				Forced:          forced,
				Constraint:      constraint,
				ModelCapability: m,
			}
		}
	}

	r.log.Warn("[Lane-Smart] No smart models available")
	return RoutingDecision{
		Model:      "",
		Lane:       LaneSmart,
		Reason:     "no models available",
		Forced:     true,
		Constraint: "no_models",
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// getModel returns model capabilities, using heuristic if not in cache.
func (r *Router) getModel(modelName string) *eval.ModelCapability {
	if m, ok := r.models[modelName]; ok {
		return m
	}

	// Try to get from scorer (will use heuristic for unknown models)
	provider := r.detectProvider(modelName)
	cap := r.scorer.GetCapabilities(provider, modelName)
	if cap != nil {
		r.models[modelName] = cap
	}
	return cap
}

// isAvailable checks if a model is currently usable.
func (r *Router) isAvailable(modelName string, provider string) bool {
	if r.availability == nil {
		return true // No availability checker, assume available
	}
	return r.availability.IsAvailable(modelName, provider)
}

// detectProvider infers provider from model name.
// Uses availability checker to detect which local backend has the model.
func (r *Router) detectProvider(modelName string) string {
	modelLower := strings.ToLower(modelName)

	// Explicit provider prefix (e.g., "groq/llama-3.1-70b")
	if strings.HasPrefix(modelLower, "groq/") {
		return ProviderGroq
	}

	// Anthropic models
	if strings.Contains(modelLower, "claude") {
		return ProviderAnthropic
	}

	// OpenAI models
	if strings.HasPrefix(modelLower, "gpt") ||
		strings.HasPrefix(modelLower, "o1") ||
		strings.HasPrefix(modelLower, "chatgpt") {
		return ProviderOpenAI
	}

	// Gemini models
	if strings.Contains(modelLower, "gemini") {
		return ProviderGemini
	}

	// Grok models
	if strings.Contains(modelLower, "grok") {
		return ProviderGrok
	}

	// Mistral cloud models (API-only, not local mistral variants)
	if strings.HasPrefix(modelLower, "mistral-large") ||
		strings.HasPrefix(modelLower, "mistral-medium") ||
		strings.HasPrefix(modelLower, "mistral-small") {
		return ProviderMistral
	}

	// MLX models (HuggingFace format with mlx-community prefix)
	if strings.HasPrefix(modelLower, "mlx") ||
		strings.Contains(modelLower, "mlx-community/") {
		return ProviderMLX
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// LOCAL MODEL DETECTION
	// Check which backend actually has the model, prioritizing faster backends
	// Priority: MLX (5-10x faster on Apple Silicon) > dnet > Ollama
	// ═══════════════════════════════════════════════════════════════════════════════

	if r.availability != nil {
		cache := r.availability.GetCache()

		// Check if model exists in any local backend
		// Priority order: MLX > dnet > Ollama
		if cache.MLXOnline {
			if cache.MLXModels[modelName] || cache.MLXModels[modelLower] {
				return ProviderMLX
			}
		}

		if cache.DnetOnline {
			if cache.DnetModels[modelName] || cache.DnetModels[modelLower] {
				return ProviderDnet
			}
		}

		if cache.OllamaOnline {
			if cache.OllamaModels[modelName] || cache.OllamaModels[modelLower] {
				return ProviderOllama
			}
		}

		// Model not found in any backend, but is a local model family
		// Use the primary (fastest) available backend
		localFamilies := []string{"llama", "mistral", "mixtral", "qwen", "phi", "codellama", "deepseek", "tinyllama", "orca", "vicuna", "neural", "wizard", "gemma", "starcoder", "command-r"}
		isLocalFamily := false
		for _, family := range localFamilies {
			if strings.HasPrefix(modelLower, family) {
				isLocalFamily = true
				break
			}
		}

		// Colon-separated tag typically indicates local model (e.g., "qwen3:4b")
		if strings.Contains(modelName, ":") {
			isLocalFamily = true
		}

		if isLocalFamily {
			primaryBackend := r.availability.GetPrimaryLocalBackend()
			if primaryBackend != "" {
				return primaryBackend
			}
		}
	}

	// Fallback: Check heuristics when no availability checker
	// or model doesn't match any known pattern

	// Colon-separated tag indicates local model
	if strings.Contains(modelName, ":") {
		// Default to MLX if available, else try detecting
		if r.availability != nil {
			if r.availability.IsMLXOnline() {
				return ProviderMLX
			}
			if r.availability.IsDnetOnline() {
				return ProviderDnet
			}
		}
		return ProviderOllama // Last resort for tagged models
	}

	// Common local model families - use fastest available backend
	localFamilies := []string{"llama", "mistral", "mixtral", "qwen", "phi", "codellama", "deepseek", "tinyllama", "orca", "vicuna", "neural", "wizard", "gemma", "starcoder", "command-r"}
	for _, family := range localFamilies {
		if strings.HasPrefix(modelLower, family) {
			if r.availability != nil {
				primaryBackend := r.availability.GetPrimaryLocalBackend()
				if primaryBackend != "" {
					return primaryBackend
				}
			}
			return ProviderOllama // Last resort
		}
	}

	// Unknown model - if any local backend is available, use it
	// Otherwise return Ollama as a placeholder (will fail gracefully)
	if r.availability != nil {
		primaryBackend := r.availability.GetPrimaryLocalBackend()
		if primaryBackend != "" {
			return primaryBackend
		}
	}

	return ProviderOllama // Legacy fallback
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMPLETION (CONVENIENCE METHOD)
// ═══════════════════════════════════════════════════════════════════════════════

// Complete routes the request and executes it using the provided LLM client.
func (r *Router) Complete(ctx context.Context, req Request, providers map[string]llm.Provider) (*llm.ChatResponse, RoutingDecision, error) {
	decision := r.Route(req)

	if decision.Model == "" {
		return nil, decision, fmt.Errorf("no model available: %s", decision.Reason)
	}

	provider, ok := providers[decision.Provider]
	if !ok {
		return nil, decision, fmt.Errorf("provider %s not configured", decision.Provider)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// PASSIVE RETRIEVAL (FAST LANE ONLY)
	// ═══════════════════════════════════════════════════════════════════════════════
	var passiveResults []memory.PassiveResult
	if decision.Lane == LaneFast && r.passiveRetriever != nil && req.Prompt != "" {
		// Perform passive retrieval BEFORE LLM call (strict 50ms timeout)
		results, err := r.passiveRetriever.Retrieve(ctx, req.Prompt, "" /* projectID */)
		if err != nil {
			// Passive retrieval failure is non-fatal - log and continue
			r.log.Debug("[PassiveRetrieval] Failed: %v", err)
		} else if len(results) > 0 {
			passiveResults = results
			r.log.Info("[PassiveRetrieval] Found %d relevant knowledge items", len(results))
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// SYSTEM PROMPT CONSTRUCTION
	// ═══════════════════════════════════════════════════════════════════════════════

	// Determine system prompt (optimized prompt takes precedence)
	systemPrompt := req.SystemPrompt
	promptSource := "default"

	if req.TaskType != "" && r.promptStore.Has(req.TaskType) {
		// Get model tier for prompt optimization
		promptTier := r.getPromptTier(decision.ModelCapability)
		optimizedPrompt := r.promptStore.GetTier(req.TaskType, promptTier)

		if optimizedPrompt != "" {
			// Optimized prompt replaces system prompt
			systemPrompt = optimizedPrompt
			promptSource = "promptomatix"

			// Log Promptomatix prompt selection
			promptPreview := optimizedPrompt
			if len(promptPreview) > 100 {
				promptPreview = promptPreview[:100] + "..."
			}
			r.log.Info("[Promptomatix] Using optimized prompt: task=%s, tier=%s, preview=%s",
				req.TaskType, promptTier, promptPreview)
		}
	}

	// Add passive retrieval placeholder for Fast Lane
	if decision.Lane == LaneFast {
		// Ensure system prompt exists before adding placeholder
		if systemPrompt == "" {
			systemPrompt = "You are a helpful AI assistant.\n\n"
		}
		// Add placeholder that will be replaced by passive results
		systemPrompt += "{{PASSIVE_RETRIEVAL}}\n"
	}

	// Log prompt source for tracking
	if promptSource == "default" && systemPrompt != "" {
		promptPreview := systemPrompt
		if len(promptPreview) > 100 {
			promptPreview = promptPreview[:100] + "..."
		}
		r.log.Debug("[Prompt] Using default system prompt: %s", promptPreview)
	}

	// Inject passive retrieval results into system prompt
	if decision.Lane == LaneFast && r.passiveRetriever != nil {
		systemPrompt = r.passiveRetriever.InjectIntoContext(systemPrompt, passiveResults)
	}

	// Build messages
	var messages []llm.Message
	if systemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})
	}
	for _, msg := range req.Messages {
		messages = append(messages, llm.Message{Role: msg.Role, Content: msg.Content})
	}
	if req.Prompt != "" {
		messages = append(messages, llm.Message{Role: "user", Content: req.Prompt})
	}

	llmReq := &llm.ChatRequest{
		Model:    decision.Model,
		Messages: messages,
	}

	resp, err := provider.Chat(ctx, llmReq)
	if err != nil {
		// Check if this is a model not found error (Ollama 404) and we can fallback
		errStr := err.Error()
		isModelNotFound := strings.Contains(errStr, "status 404") ||
			strings.Contains(errStr, "model") && strings.Contains(errStr, "not found")

		// Only fallback if we were using fast lane and smart models are available
		if isModelNotFound && decision.Lane == LaneFast {
			r.log.Warn("[Router] Model %s not found, attempting fallback to smart lane", decision.Model)

			// Try smart lane as fallback
			smartDecision := r.selectSmartModel("model not found fallback", true, "model_not_found")
			if smartDecision.Model != "" {
				smartProvider, ok := providers[smartDecision.Provider]
				if ok {
					// Update model in request and retry
					llmReq.Model = smartDecision.Model
					smartResp, smartErr := smartProvider.Chat(ctx, llmReq)
					if smartErr == nil {
						r.log.Info("[Router] Fallback to %s succeeded", smartDecision.Model)
						return smartResp, smartDecision, nil
					}
					r.log.Warn("[Router] Fallback to %s also failed: %v", smartDecision.Model, smartErr)
				}
			}
		}

		return nil, decision, fmt.Errorf("completion failed with %s: %w", decision.Model, err)
	}

	return resp, decision, nil
}

// getPromptTier maps model tier to prompt tier (small/large).
// Prompt tiers are simplified to match the build-time optimization granularity.
func (r *Router) getPromptTier(cap *eval.ModelCapability) string {
	if cap == nil {
		// Default to large for cloud models when capability unknown
		return "large"
	}

	// Map model tiers to prompt tiers
	switch cap.Tier {
	case eval.TierSmall, eval.TierMedium:
		return "small"
	case eval.TierLarge, eval.TierXL, eval.TierFrontier:
		return "large"
	default:
		return "large" // Default to large tier
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATUS / DEBUG
// ═══════════════════════════════════════════════════════════════════════════════

// Status returns current router status for debugging.
func (r *Router) Status() map[string]interface{} {
	var availableFast, availableSmart []string

	for _, m := range r.config.FastModels {
		model := r.getModel(m)
		provider := ""
		if model != nil {
			provider = model.Provider
		} else {
			provider = r.detectProvider(m)
		}
		if r.isAvailable(m, provider) {
			availableFast = append(availableFast, m)
		}
	}

	for _, m := range r.config.SmartModels {
		model := r.getModel(m)
		provider := ""
		if model != nil {
			provider = model.Provider
		} else {
			provider = r.detectProvider(m)
		}
		if r.isAvailable(m, provider) {
			availableSmart = append(availableSmart, m)
		}
	}

	status := map[string]interface{}{
		"available_fast":    availableFast,
		"available_smart":   availableSmart,
		"fast_count":        len(availableFast),
		"smart_count":       len(availableSmart),
		"fast_lane_usage":   atomic.LoadInt64(&r.fastLaneCount),
		"smart_lane_usage":  atomic.LoadInt64(&r.smartLaneCount),
	}

	if r.availability != nil {
		for k, v := range r.availability.Status() {
			status[k] = v
		}
	}

	return status
}

// GetModelCapability returns the capability info for a specific model.
func (r *Router) GetModelCapability(model string) *eval.ModelCapability {
	return r.getModel(model)
}

// RefreshAvailability refreshes the model availability cache.
func (r *Router) RefreshAvailability(ctx context.Context) error {
	if r.availability == nil {
		return nil
	}
	return r.availability.Refresh(ctx)
}
