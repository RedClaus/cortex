package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/internal/cognitive"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/router"
	"github.com/normanking/cortex/internal/tools"
	"github.com/normanking/cortex/pkg/types"
)

// fingerprintStage detects the platform context.
type fingerprintStage struct {
	o *Orchestrator
}

func (s *fingerprintStage) Name() string { return "fingerprint" }

func (s *fingerprintStage) Execute(ctx context.Context, state *PipelineState) error {
	if !s.o.config.EnableFingerprint {
		return nil
	}

	// Use existing fingerprint from context if available
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		return nil
	}

	// Detect fingerprint
	fp, err := s.o.fpDetector.Detect(ctx)
	if err != nil {
		return nil // Non-fatal
	}

	// Attach to context
	if state.Request.Context == nil {
		state.Request.Context = &RequestContext{}
	}
	state.Request.Context.Fingerprint = fp

	return nil
}

// routingStage classifies the request.
type routingStage struct {
	o *Orchestrator
}

func (s *routingStage) Name() string { return "routing" }

func (s *routingStage) Execute(ctx context.Context, state *PipelineState) error {
	// Build router context from request context
	var routerCtx *router.ProcessContext
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		fp := state.Request.Context.Fingerprint
		if fp.ProjectType != "" {
			routerCtx = &router.ProcessContext{
				Platform: &router.PlatformInfo{
					Vendor: string(fp.Platform),
					Name:   string(fp.ProjectType),
				},
			}
		}
	}

	// Route the request
	state.Routing = s.o.router.Route(state.Request.Input, routerCtx)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// INTROSPECTION STAGE
// CR-018: Metacognitive Self-Awareness
// This stage handles introspection queries ("do you know X?", "what do you know?")
// ═══════════════════════════════════════════════════════════════════════════════

// introspectionStage handles metacognitive introspection queries.
type introspectionStage struct {
	o *Orchestrator
}

func (s *introspectionStage) Name() string { return "introspection" }

func (s *introspectionStage) Execute(ctx context.Context, state *PipelineState) error {
	log := logging.Global()

	// Skip if introspection coordinator not configured
	if s.o.introspection == nil || !s.o.introspection.Enabled() {
		return nil
	}

	// Classify the input to see if it's an introspection query
	classification, err := s.o.introspection.Classify(ctx, state.Request.Input)
	if err != nil {
		log.Debug("[Introspection] Classification failed: %v", err)
		return nil // Continue to normal processing
	}

	// If not introspective, skip this stage
	if classification.Type == "not_introspective" {
		return nil
	}

	log.Info("[Introspection] Detected %s query for subject: %s", classification.Type, classification.Subject)

	// Process introspection through the full pipeline
	result, err := s.o.ProcessIntrospection(ctx, state.Request.Input)
	if err != nil {
		log.Warn("[Introspection] Processing failed: %v", err)
		return nil // Fall through to normal processing
	}

	// If handled, set response and skip LLM
	if result.IsHandled && result.Response != "" {
		state.LLMResponse = result.Response
		state.IntrospectionResult = result
		log.Info("[Introspection] Query handled, skipping LLM stage")
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// RAPID GATE STAGE
// CR-026: Reduce AI Prompt Iteration Depth
// This stage evaluates confidence and may return early with a clarifying question.
// ═══════════════════════════════════════════════════════════════════════════════

// rapidGateStage evaluates confidence and decides if clarification is needed.
type rapidGateStage struct {
	o *Orchestrator
}

func (s *rapidGateStage) Name() string { return "rapid_gate" }

func (s *rapidGateStage) Execute(ctx context.Context, state *PipelineState) error {
	log := logging.Global()

	// Get RAPID config (use defaults if not set)
	cfg := s.o.rapidConfig
	if cfg == nil {
		cfg = DefaultRAPIDConfig()
	}

	// Skip RAPID for disabled
	if !cfg.Enabled {
		return nil
	}

	input := state.Request.Input

	// Skip RAPID for simple shell commands
	if cfg.SkipForSimpleCommands && isSimpleShellCommand(input) {
		log.Debug("[RAPID] Simple command - skipping confidence gate")
		return nil
	}

	// Skip RAPID for factual questions (can be answered directly)
	if canAnswerDirectly(input) {
		log.Debug("[RAPID] Factual question - skipping confidence gate")
		return nil
	}

	// Skip RAPID for simple conversations (greetings, etc.)
	if isSimpleConversation(input) {
		log.Debug("[RAPID] Simple conversation - skipping confidence gate")
		return nil
	}

	// Skip RAPID for personal questions (memory lookups)
	if isPersonalQuestion(input) {
		log.Debug("[RAPID] Personal question - skipping confidence gate")
		return nil
	}

	// Skip RAPID in voice mode if configured
	if cfg.SkipInVoiceMode && state.Request.Context != nil && state.Request.Context.VoiceMode {
		log.Debug("[RAPID] Voice mode - skipping confidence gate")
		return nil
	}

	// Initialize decision
	decision := &RAPIDDecision{
		ShouldProceed: true,
		Level:         5, // Default: proceed to execution
	}

	// Level 2: Check routing confidence - but only for truly ambiguous queries
	// Also require the request to be short and vague (not a detailed request)
	if state.Routing != nil {
		decision.ConfidenceScore = state.Routing.Confidence

		// Only trigger clarification for VERY low confidence AND short/vague input
		isVeryLowConfidence := state.Routing.Confidence < cfg.MinConfidence
		isShortInput := len(strings.TrimSpace(input)) < 30
		isAmbiguous := s.isAmbiguousRequest(state)

		if isVeryLowConfidence && isShortInput && isAmbiguous {
			log.Info("[RAPID] Low routing confidence: %.2f < %.2f threshold (short+ambiguous)",
				state.Routing.Confidence, cfg.MinConfidence)

			decision.Level = 2
			decision.ShouldProceed = false
			decision.ClarificationNeeded = true
			decision.ClarificationQuestion = s.generateClarification(state)

			state.RAPIDDecision = decision

			// Early return: Set response and skip remaining stages
			state.LLMResponse = decision.ClarificationQuestion
			log.Info("[RAPID] Returning clarification question, skipping LLM")
			return nil
		}
	}

	// Level 3: Check if complex task needs decomposition (from cognitive stage if run)
	// Note: This runs BEFORE cognitive, so we check fingerprint/routing hints
	if state.Routing != nil && s.isAmbiguousRequest(state) && state.Routing.Confidence < 0.8 {
		log.Debug("[RAPID] Ambiguous request with moderate confidence")
		decision.Assumptions = s.inferAssumptions(state)
	}

	decision.ShouldProceed = true
	state.RAPIDDecision = decision
	return nil
}

// generateClarification creates a compound question based on task type.
func (s *rapidGateStage) generateClarification(state *PipelineState) string {
	taskType := "help you"
	if state.Routing != nil {
		switch state.Routing.TaskType {
		case router.TaskCodeGen:
			taskType = "generate code"
		case router.TaskDebug:
			taskType = "debug an issue"
		case router.TaskReview:
			taskType = "review code"
		case router.TaskExplain:
			taskType = "explain something"
		case router.TaskPlanning:
			taskType = "plan an implementation"
		case router.TaskRefactor:
			taskType = "refactor code"
		case router.TaskInfrastructure:
			taskType = "help with infrastructure"
		}
	}

	// Truncate input for display
	input := state.Request.Input
	if len(input) > 50 {
		input = input[:47] + "..."
	}

	// Generate compound question with smart defaults
	return fmt.Sprintf(`I want to make sure I help you correctly. It sounds like you want me to **%s**.

To give you the best answer, could you clarify:
1. **What specifically** do you need? (the more detail, the better)
2. **What have you tried** so far? (if anything)
3. **What's the context?** (project type, error messages, etc.)

Or just tell me more about "%s" and I'll do my best!`, taskType, input)
}

// isAmbiguousRequest checks if the request is vague or could have multiple interpretations.
func (s *rapidGateStage) isAmbiguousRequest(state *PipelineState) bool {
	input := strings.ToLower(state.Request.Input)

	// Very short requests are often ambiguous
	if len(input) < 20 {
		return true
	}

	// Vague phrases
	vaguePatterns := []string{
		"help me", "fix this", "make it work", "doesn't work",
		"something wrong", "not working", "broken", "issue with",
		"problem with", "can you", "how do i", "what should",
	}
	for _, pattern := range vaguePatterns {
		if strings.Contains(input, pattern) && len(input) < 50 {
			return true
		}
	}

	return false
}

// inferAssumptions generates assumptions when proceeding with incomplete info.
func (s *rapidGateStage) inferAssumptions(state *PipelineState) []string {
	assumptions := []string{}

	// Infer from fingerprint
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		fp := state.Request.Context.Fingerprint
		if fp.ProjectType != "" {
			assumptions = append(assumptions, fmt.Sprintf("Project type: %s", fp.ProjectType))
		}
		if fp.Platform != "" {
			assumptions = append(assumptions, fmt.Sprintf("Platform: %s", fp.Platform))
		}
	}

	// Infer from task type
	if state.Routing != nil {
		assumptions = append(assumptions, fmt.Sprintf("Task type: %s", state.Routing.TaskType))
	}

	return assumptions
}

// knowledgeStage retrieves relevant knowledge.
type knowledgeStage struct {
	o *Orchestrator
}

func (s *knowledgeStage) Name() string { return "knowledge" }

func (s *knowledgeStage) Execute(ctx context.Context, state *PipelineState) error {
	log := logging.Global()

	// Retrieve strategic principles from enhanced memory (CR-015)
	// This happens regardless of knowledge fabric availability
	enhancedStores := s.o.EnhancedMemoryStores()
	if enhancedStores != nil && enhancedStores.Strategic != nil {
		principles, err := GetRelevantPrinciples(ctx, enhancedStores, state.Request.Input, 5)
		if err != nil {
			log.Debug("[Knowledge] Strategic principle retrieval error (non-fatal): %v", err)
		} else if len(principles) > 0 {
			state.StrategicPrinciples = principles
			log.Info("[Knowledge] Retrieved %d strategic principles for: %q",
				len(principles), truncateLog(state.Request.Input, 50))
		}
	}

	if !s.o.config.EnableKnowledge || s.o.fabric == nil {
		return nil
	}

	// Get specialist for knowledge tags
	spec := s.o.GetSpecialist(state.Routing.TaskType)
	tags := spec.KnowledgeTags

	// Add request-specific tags
	if state.Request.Context != nil {
		tags = append(tags, state.Request.Context.Tags...)
	}

	// Query knowledge fabric using Search method
	opts := types.SearchOptions{
		Tags:  tags,
		Limit: 5,
	}

	result, err := s.o.fabric.Search(ctx, state.Request.Input, opts)
	if err != nil {
		return nil // Non-fatal - continue without knowledge
	}

	if result != nil && len(result.Items) > 0 {
		state.Knowledge = result.Items
	}

	return nil
}

// cognitiveStage handles template matching and distillation.
type cognitiveStage struct {
	o *Orchestrator
}

func (s *cognitiveStage) Name() string { return "cognitive" }

func (s *cognitiveStage) Execute(ctx context.Context, state *PipelineState) error {
	// CR-017: Use CognitiveCoordinator if available (preferred)
	if s.o.cognitive != nil && s.o.cognitive.Enabled() {
		return s.executeWithCoordinator(ctx, state)
	}

	// CR-017 Phase 6: Legacy path removed - cognitive architecture now requires CognitiveCoordinator.
	// If cogEnabled but no coordinator, log warning and skip.
	if s.o.cogEnabled {
		log := logging.Global()
		log.Warn("[Cognitive] cogEnabled=true but no CognitiveCoordinator configured - skipping cognitive stage. Use WithCognitiveCoordinator() instead of legacy options.")
	}

	return nil
}

// executeWithCoordinator uses the CognitiveCoordinator for processing (CR-017).
func (s *cognitiveStage) executeWithCoordinator(ctx context.Context, state *PipelineState) error {
	log := logging.Global()
	log.Debug("[Cognitive] Processing request via CognitiveCoordinator")

	// Initialize cognitive result
	state.Cognitive = &CognitiveResult{}

	// Step 1: Try template matching via semantic router
	routingResult, err := s.o.cognitive.Route(ctx, state.Request.Input)
	if err != nil {
		log.Warn("[Cognitive] Router error: %v", err)
		// Continue without template matching
	} else if routingResult != nil && routingResult.Match != nil && routingResult.Match.Template != nil {
		state.Cognitive.TemplateMatch = routingResult.Match
		state.Cognitive.ModelTier = string(routingResult.RecommendedTier)

		// High confidence match - try to use the template
		templateMatch := routingResult.Match
		if templateMatch.SimilarityLevel == cognitive.SimilarityHigh || templateMatch.SimilarityLevel == cognitive.SimilarityMedium {
			log.Info("[Cognitive] Template match: %s (similarity: %.2f)", templateMatch.Template.Name, templateMatch.SimilarityScore)

			// Try to render the template using coordinator
			output, err := s.o.cognitive.RenderTemplateSimple(templateMatch.Template.TemplateBody, map[string]any{
				"input":    state.Request.Input,
				"platform": s.getPlatformInfo(state),
			})
			templateStart := time.Now()
			if err == nil && output != "" {
				state.Cognitive.TemplateUsed = true
				state.Cognitive.Template = templateMatch.Template
				state.Cognitive.RenderedOutput = output

				// Record usage for feedback loop via coordinator
				if feedbackErr := s.o.cognitive.RecordFeedback(ctx, templateMatch.Template.ID, state.Request.Input, output, true, 0); feedbackErr != nil {
					log.Warn("[Cognitive] Feedback recording failed: %v", feedbackErr)
				}

				// CR-017 Phase 5: Publish TemplateUsed event
				if s.o.eventBus != nil {
					latencyMs := int(time.Since(templateStart).Milliseconds())
					s.o.eventBus.Publish(bus.NewTemplateUsedEvent(templateMatch.Template.ID, state.Request.ID, true, latencyMs))
				}

				// Set as LLM response so we skip the LLM stage
				state.LLMResponse = output
				log.Info("[Cognitive] Using template response")
				return nil
			} else if err != nil {
				log.Warn("[Cognitive] Template rendering failed: %v", err)
				// CR-017 Phase 5: Publish TemplateUsed event for failure
				if s.o.eventBus != nil {
					latencyMs := int(time.Since(templateStart).Milliseconds())
					s.o.eventBus.Publish(bus.NewTemplateUsedEvent(templateMatch.Template.ID, state.Request.ID, false, latencyMs))
				}
			}
		}
	}

	// Step 2: Check complexity for decomposition via coordinator
	taskType := s.mapTaskType(state.Routing)
	analysis := s.o.cognitive.Analyze(state.Request.Input, taskType)
	if analysis != nil && analysis.Complexity != nil {
		state.Cognitive.ComplexityScore = analysis.Complexity.Score
		state.Cognitive.NeedsDecomposition = analysis.Complexity.NeedsDecomp

		if analysis.Complexity.NeedsDecomp {
			log.Info("[Cognitive] Complex task detected (score: %d), may need decomposition", analysis.Complexity.Score)
		}
	}

	// Step 3: If no match and this looks like a novel request, flag for distillation
	if state.Cognitive.TemplateMatch == nil || state.Cognitive.TemplateMatch.SimilarityLevel == cognitive.SimilarityNoMatch {
		log.Debug("[Cognitive] No template match, may trigger distillation")
		state.Cognitive.ModelTier = "frontier"
	}

	return nil
}

// getPlatformInfo extracts platform context from the request.
func (s *cognitiveStage) getPlatformInfo(state *PipelineState) map[string]string {
	info := make(map[string]string)
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		fp := state.Request.Context.Fingerprint
		if fp.Platform != "" {
			info["platform"] = string(fp.Platform)
		}
		if fp.ProjectType != "" {
			info["project_type"] = string(fp.ProjectType)
		}
		if fp.ProjectRoot != "" {
			info["project_root"] = fp.ProjectRoot
		}
	}
	if state.Request.Context != nil && state.Request.Context.WorkingDir != "" {
		info["working_dir"] = state.Request.Context.WorkingDir
	}
	return info
}

// mapTaskType maps router.TaskType to cognitive.TaskType.
func (s *cognitiveStage) mapTaskType(routing *router.RoutingDecision) cognitive.TaskType {
	if routing == nil {
		return cognitive.TaskGeneral
	}
	// Map router task types to cognitive task types
	switch routing.TaskType {
	case router.TaskCodeGen:
		return cognitive.TaskCodeGen
	case router.TaskDebug:
		return cognitive.TaskDebug
	case router.TaskReview:
		return cognitive.TaskReview
	case router.TaskPlanning:
		return cognitive.TaskPlanning
	case router.TaskInfrastructure:
		return cognitive.TaskInfrastructure
	case router.TaskExplain:
		return cognitive.TaskExplain
	case router.TaskRefactor:
		return cognitive.TaskRefactor
	default:
		return cognitive.TaskGeneral
	}
}

// toolExecutionStage executes requested tools.
type toolExecutionStage struct {
	o *Orchestrator
}

func (s *toolExecutionStage) Name() string { return "tool_execution" }

func (s *toolExecutionStage) Execute(ctx context.Context, state *PipelineState) error {
	workingDir := s.getWorkingDir(state)
	input := state.Request.Input

	// Debug logging
	log := logging.Global()
	log.Debug("[ToolExec] Input: %q, WorkingDir: %q", input, workingDir)
	log.Debug("[ToolExec] RequestType: %v, Routing: %v", state.Request.Type, state.Routing != nil)
	if state.Routing != nil {
		log.Debug("[ToolExec] TaskType: %s", state.Routing.TaskType)
	}

	// Check for cd command first - needs special handling
	if isCd, targetDir := s.isCdCommand(input); isCd {
		newDir, err := s.handleCdCommand(targetDir, workingDir)
		if err != nil {
			state.ToolResults = append(state.ToolResults, &tools.ToolResult{
				Tool:    tools.ToolBash,
				Success: false,
				Error:   err.Error(),
			})
			return nil
		}

		// Store new working directory in response metadata
		if state.Response.Metadata == nil {
			state.Response.Metadata = make(map[string]interface{})
		}
		state.Response.Metadata["new_working_dir"] = newDir

		state.ToolResults = append(state.ToolResults, &tools.ToolResult{
			Tool:    tools.ToolBash,
			Success: true,
			Output:  newDir,
		})
		return nil
	}

	// For direct command requests, extract and execute
	if state.Request.Type == RequestCommand {
		toolStart := time.Now()
		result, err := s.o.toolExec.Execute(ctx, &tools.ToolRequest{
			Tool:       tools.ToolBash,
			Input:      state.Request.Input,
			WorkingDir: workingDir,
		})
		toolLatency := time.Since(toolStart)
		if err != nil {
			return err
		}
		state.ToolResults = append(state.ToolResults, result)

		// CR-017 Phase 5: Publish ToolExecutedEventV2 with latency
		if s.o.eventBus != nil {
			s.o.eventBus.Publish(bus.NewToolExecutedEventV2(
				state.Request.ID,
				string(tools.ToolBash),
				map[string]any{"command": state.Request.Input, "working_dir": workingDir},
				result.Success,
				toolLatency,
				nil,
			))
		}
	}

	// Auto-execute shell commands for infrastructure tasks
	// This allows "ls -al", "pwd", "git status" etc. to work without an LLM
	// IMPORTANT: Only auto-execute if it actually looks like a command
	// Being classified as "infrastructure" by the router (e.g., mentioning "linux")
	// is NOT sufficient - we need both infrastructure context AND command-like input
	if state.Request.Type == RequestChat && state.Routing != nil {
		isInfra := state.Routing.TaskType == router.TaskInfrastructure
		isCommand := s.looksLikeCommand(input)
		log.Debug("[ToolExec] isInfrastructure: %v, looksLikeCommand: %v", isInfra, isCommand)

		// Only auto-execute if:
		// 1. Input looks like a command, OR
		// 2. Input is infrastructure-related AND looks like a command
		// This prevents natural language queries like "find linux commands online"
		// from being executed as shell commands
		if isCommand {
			log.Debug("[ToolExec] Executing command: %q in %q", input, workingDir)
			toolStart := time.Now()
			result, err := s.o.toolExec.Execute(ctx, &tools.ToolRequest{
				Tool:       tools.ToolBash,
				Input:      input,
				WorkingDir: workingDir,
			})
			toolLatency := time.Since(toolStart)
			if err != nil {
				log.Error("[ToolExec] Command failed: %v", err)
				state.AddError(err)
				// CR-017 Phase 5: Publish ToolExecutedEventV2 for failure
				if s.o.eventBus != nil {
					s.o.eventBus.Publish(bus.NewToolExecutedEventV2(
						state.Request.ID,
						string(tools.ToolBash),
						map[string]any{"command": input, "working_dir": workingDir},
						false,
						toolLatency,
						err,
					))
				}
			} else {
				log.Debug("[ToolExec] Command succeeded, output length: %d", len(result.Output))
				state.ToolResults = append(state.ToolResults, result)

				// CR-017 Phase 5: Publish ToolExecutedEventV2 with latency
				if s.o.eventBus != nil {
					s.o.eventBus.Publish(bus.NewToolExecutedEventV2(
						state.Request.ID,
						string(tools.ToolBash),
						map[string]any{"command": input, "working_dir": workingDir},
						result.Success,
						toolLatency,
						nil,
					))
				}
			}
		} else {
			log.Debug("[ToolExec] Input not recognized as command, skipping execution")
		}
	}

	// Execute any queued tool requests
	for i, req := range state.ToolRequests {
		if i >= s.o.config.MaxToolCalls {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		toolStart := time.Now()
		result, err := s.o.toolExec.Execute(ctx, req)
		toolLatency := time.Since(toolStart)
		if err != nil {
			state.AddError(err)
			// CR-017 Phase 5: Publish ToolExecutedEventV2 for failure
			if s.o.eventBus != nil {
				s.o.eventBus.Publish(bus.NewToolExecutedEventV2(
					state.Request.ID,
					string(req.Tool),
					map[string]any{"input": req.Input, "working_dir": req.WorkingDir},
					false,
					toolLatency,
					err,
				))
			}
			continue
		}
		state.ToolResults = append(state.ToolResults, result)

		// CR-017 Phase 5: Publish ToolExecutedEventV2 with latency
		if s.o.eventBus != nil {
			s.o.eventBus.Publish(bus.NewToolExecutedEventV2(
				state.Request.ID,
				string(req.Tool),
				map[string]any{"input": req.Input, "working_dir": req.WorkingDir},
				result.Success,
				toolLatency,
				nil,
			))
		}
	}

	return nil
}

// looksLikeCommand checks if input appears to be a shell command.
func (s *toolExecutionStage) looksLikeCommand(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// First, check for natural language patterns that indicate this is NOT a command
	// This prevents false positives like "find linux commands online"
	naturalLanguagePatterns := []string{
		" online", " please", " how to", " what is", " can you", " help me",
		" download them", " to your", " to my", " about ", " information",
		"?", // Questions are almost never commands
	}
	lowerInput := strings.ToLower(input)
	for _, pattern := range naturalLanguagePatterns {
		if strings.Contains(lowerInput, pattern) {
			return false
		}
	}

	// If input has too many words without flags, it's likely natural language
	fields := strings.Fields(input)
	if len(fields) > 6 {
		// Check if there are any flags (words starting with -)
		hasFlags := false
		for _, f := range fields {
			if strings.HasPrefix(f, "-") {
				hasFlags = true
				break
			}
		}
		if !hasFlags {
			return false // Long input without flags is likely natural language
		}
	}

	// Check if input starts with a known shell command (uses package-level shellCommands SSOT)
	if len(fields) == 0 {
		return false
	}
	firstWord := fields[0]
	for _, cmd := range shellCommands {
		if firstWord == cmd {
			return true
		}
	}

	// Check for common patterns
	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "/") {
		return true // Looks like an executable path
	}
	if strings.HasPrefix(input, "~") {
		return true // Home directory path
	}
	if strings.Contains(input, "|") || strings.Contains(input, ">") || strings.Contains(input, "<") {
		return true // Has shell operators
	}
	if strings.Contains(input, "&&") || strings.Contains(input, "||") || strings.Contains(input, ";") {
		return true // Command chaining
	}
	if strings.HasPrefix(input, "$") {
		return true // Variable expansion
	}

	return false
}

// isCdCommand checks if input is a cd command and returns the target directory.
func (s *toolExecutionStage) isCdCommand(input string) (bool, string) {
	input = strings.TrimSpace(input)
	if !strings.HasPrefix(input, "cd") {
		return false, ""
	}

	fields := strings.Fields(input)
	if len(fields) == 0 || fields[0] != "cd" {
		return false, ""
	}

	if len(fields) == 1 {
		// Just "cd" - go to home directory
		return true, "~"
	}

	// Handle "cd -" (previous directory) - we don't track this, so go home
	if fields[1] == "-" {
		return true, "~"
	}

	return true, fields[1]
}

// handleCdCommand resolves the target directory and validates it exists.
func (s *toolExecutionStage) handleCdCommand(targetDir, currentDir string) (string, error) {
	// Expand home directory
	if strings.HasPrefix(targetDir, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cd: cannot determine home directory: %v", err)
		}
		if targetDir == "~" {
			targetDir = home
		} else {
			// Handle ~/something
			targetDir = filepath.Join(home, targetDir[2:])
		}
	}

	// Handle relative paths
	if !filepath.IsAbs(targetDir) {
		if currentDir == "" {
			// If no current dir, use process working directory
			cwd, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("cd: cannot determine current directory: %v", err)
			}
			currentDir = cwd
		}
		targetDir = filepath.Join(currentDir, targetDir)
	}

	// Clean the path (resolve . and ..)
	targetDir = filepath.Clean(targetDir)

	// Validate the directory exists
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("cd: no such file or directory: %s", targetDir)
		}
		return "", fmt.Errorf("cd: %v", err)
	}

	// Validate it's a directory
	if !info.IsDir() {
		return "", fmt.Errorf("cd: not a directory: %s", targetDir)
	}

	return targetDir, nil
}

func (s *toolExecutionStage) getWorkingDir(state *PipelineState) string {
	if state.Request.Context != nil && state.Request.Context.WorkingDir != "" {
		return state.Request.Context.WorkingDir
	}
	if state.Request.Context != nil && state.Request.Context.Fingerprint != nil {
		return state.Request.Context.Fingerprint.ProjectRoot
	}
	return ""
}
