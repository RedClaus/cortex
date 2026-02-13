// Package a2a provides Pinky-compatible REST endpoints for the A2A server.
// These endpoints allow Pinky's RemoteBrain to communicate with CortexBrain.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalagent "github.com/normanking/cortex/internal/agent"
	"github.com/normanking/cortex/internal/logging"
	pkgagent "github.com/normanking/cortex/pkg/agent"
	"github.com/normanking/cortex/pkg/brain"
)

// PinkyThinkRequest matches Pinky's remote brain request format.
type PinkyThinkRequest struct {
	UserID      string        `json:"user_id,omitempty"`
	Messages    []PinkyMessage `json:"messages"`
	Tools       []PinkyTool    `json:"tools,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream"`
}

// PinkyMessage represents a message in Pinky format.
type PinkyMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PinkyTool represents a tool in Pinky format.
type PinkyTool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]any    `json:"parameters,omitempty"`
}

// PinkyThinkResponse matches Pinky's expected response format.
type PinkyThinkResponse struct {
	Content   string           `json:"content"`
	ToolCalls []PinkyToolCall  `json:"tool_calls,omitempty"`
	Reasoning string           `json:"reasoning,omitempty"`
	Done      bool             `json:"done"`
	Error     string           `json:"error,omitempty"`
}

// PinkyToolCall represents a tool call in Pinky format.
type PinkyToolCall struct {
	ID    string         `json:"id"`
	Tool  string         `json:"tool"`
	Input map[string]any `json:"input"`
}

// PinkyCompatHandler handles Pinky-compatible REST endpoints.
type PinkyCompatHandler struct {
	brain       *brain.Executive
	executor    *internalagent.Executor
	router      *pkgagent.Router      // Agent router for local/frontier routing
	localBrain  pkgagent.BrainInterface
	memoryStore MemoryStoreInterface  // For user memory retrieval
	log         *logging.Logger
}

// MemoryStoreInterface defines the interface for user memory retrieval and skill storage.
type MemoryStoreInterface interface {
	GetUserMemory(ctx context.Context, userID string) (*UserMemoryData, error)
	// Skill memory methods (optional - implementations may return nil/error if not supported)
	StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error
	SearchSkills(ctx context.Context, userID, query string, limit int) ([]SkillMemory, error)
}

// SkillMemory represents a remembered successful action.
type SkillMemory struct {
	Intent    string            `json:"intent"`
	Tool      string            `json:"tool"`
	Params    map[string]string `json:"params"`
	Success   bool              `json:"success"`
	Timestamp time.Time         `json:"timestamp"`
}

// UserMemoryData holds user memory for injection into context.
type UserMemoryData struct {
	Name           string
	Role           string
	CustomFacts    []string
	PrefersConcise bool
	PrefersVerbose bool
}

// NewPinkyCompatHandler creates a new Pinky compatibility handler.
func NewPinkyCompatHandler(brainExec *brain.Executive, memoryStore MemoryStoreInterface) *PinkyCompatHandler {
	log := logging.Global()

	// Create local brain wrapper
	localBrain := pkgagent.NewLocalBrain(brainExec)

	// Create skill store adapter
	var skillStore pkgagent.SkillStore
	if memoryStore != nil {
		skillStore = NewSkillStoreAdapter(memoryStore)
	}

	// Try to create frontier brain (optional - depends on API key)
	var frontierBrain pkgagent.BrainInterface
	frontierCfg := pkgagent.FrontierConfig{
		Provider: "anthropic",
		Model:    "claude-sonnet-4-20250514",
	}
	fb, err := pkgagent.NewFrontierBrain(frontierCfg)
	if err == nil {
		frontierBrain = fb
		log.Info("[PinkyCompat] Frontier brain (Claude) enabled")
	} else {
		log.Info("[PinkyCompat] Frontier brain not available: %v", err)
	}

	// Create router with both brains
	routerCfg := pkgagent.DefaultRouterConfig()
	router := pkgagent.NewRouter(localBrain, frontierBrain, skillStore, routerCfg)

	return &PinkyCompatHandler{
		brain:       brainExec,
		executor:    internalagent.NewExecutor(""),
		router:      router,
		localBrain:  localBrain,
		memoryStore: memoryStore,
		log:         log,
	}
}

// RegisterRoutes registers Pinky-compatible routes on the mux.
func (h *PinkyCompatHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("POST /v1/think", h.handleThink)
	mux.HandleFunc("POST /v1/memory", h.handleMemoryStore)
	mux.HandleFunc("POST /v1/memory/search", h.handleMemorySearch)
}

// processWithRouter uses the Agent Router to intelligently route between local and frontier brains.
// This method provides skill-aware routing: simple queries go local, complex queries may go frontier,
// and successful frontier executions are captured as skills for future local handling.
func (h *PinkyCompatHandler) processWithRouter(ctx context.Context, userID, prompt, systemPrompt string, history []PinkyMessage) (string, *pkgagent.RouteDecision, error) {
	// Convert Pinky messages to agent messages
	agentHistory := make([]pkgagent.Message, 0, len(history))
	for _, msg := range history {
		agentHistory = append(agentHistory, pkgagent.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Build brain input
	input := &pkgagent.BrainInput{
		Query:               prompt,
		SystemPrompt:        systemPrompt,
		ConversationHistory: agentHistory,
		MaxTokens:           4096,
		Temperature:         0.7,
	}

	// Process through router (routes to local or frontier based on complexity/skills)
	result, decision, err := h.router.Process(ctx, userID, input)
	if err != nil {
		return "", decision, err
	}

	h.log.Info("[PinkyCompat] Router decision: %s (%s, confidence: %.2f)",
		decision.Brain, decision.Reason, decision.Confidence)

	return result.Content, decision, nil
}

// handleHealth returns a simple health check response.
func (h *PinkyCompatHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"server": "cortex-brain",
	})
}

// handleThink processes a think request with tool support.
func (h *PinkyCompatHandler) handleThink(w http.ResponseWriter, r *http.Request) {
	// Recover from any panics
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("[PinkyCompat] Panic recovered: %v", r)
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("internal error: %v", r))
		}
	}()

	h.log.Info("[PinkyCompat] /v1/think request received")

	var req PinkyThinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	// Convert Pinky messages to Brain input - preserve conversation history
	var input string
	var conversationHistory strings.Builder

	for _, msg := range req.Messages {
		if msg.Role == "user" {
			input = msg.Content // Keep track of latest user message
			conversationHistory.WriteString("User: ")
			conversationHistory.WriteString(msg.Content)
			conversationHistory.WriteString("\n")
		} else if msg.Role == "assistant" {
			conversationHistory.WriteString("Assistant: ")
			conversationHistory.WriteString(msg.Content)
			conversationHistory.WriteString("\n")
		}
	}

	if input == "" {
		h.writeError(w, http.StatusBadRequest, "no user message found")
		return
	}

	// Store conversation context for Brain to use
	conversationContext := conversationHistory.String()

	ctx := r.Context()
	if req.UserID != "" {
		ctx = context.WithValue(ctx, "user_id", req.UserID)
	}

	var content string
	var toolCalls []PinkyToolCall

	// Check if this looks like a shell command
	if h.looksLikeShellCommand(input) {
		h.log.Info("[PinkyCompat] Detected shell command: %s", input)
		cmdResult := h.executor.Execute(ctx, &internalagent.ToolCall{
			Name:   "run_command",
			Params: map[string]string{"command": input},
		})

		toolCalls = append(toolCalls, PinkyToolCall{
			ID:    "call_1",
			Tool:  "run_command",
			Input: map[string]any{"command": input, "_success": cmdResult.Success},
		})

		if cmdResult.Success {
			content = cmdResult.Output
		} else {
			content = fmt.Sprintf("Command failed: %s", cmdResult.Error)
		}

		// Return early with command result
		resp := PinkyThinkResponse{
			Content:   content,
			ToolCalls: toolCalls,
			Done:      true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Use agentic approach: let the LLM reason about which tools to use
	enableTools := h.shouldEnableToolsForQuery(input)
	h.log.Info("[PinkyCompat] Query: %s, tools enabled: %v", input[:min(50, len(input))], enableTools)

	if enableTools {
		// Detect query intent and execute tools directly (bypasses Brain.Process blackboard issue)
		h.log.Info("[PinkyCompat] Using direct tool execution")

		// Detect what tool to use based on query
		detectedTool, toolParams := h.detectToolFromQuery(input)

		if detectedTool != "" {
			h.log.Info("[PinkyCompat] Detected tool: %s", detectedTool)

			// Handle application generation specially - it's a multi-step process
			if detectedTool == "generate_application" {
				h.log.Info("[PinkyCompat] Routing to Application Generator")
				content, appToolCalls := h.handleAppGeneration(ctx, input)
				toolCalls = append(toolCalls, appToolCalls...)

				resp := PinkyThinkResponse{
					Content:   content,
					ToolCalls: toolCalls,
					Done:      true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}

			toolResult := h.executor.Execute(ctx, &internalagent.ToolCall{
				Name:   detectedTool,
				Params: toolParams,
			})

			toolCalls = append(toolCalls, PinkyToolCall{
				ID:    "call_1",
				Tool:  detectedTool,
				Input: stringMapToAnyMap(toolParams),
			})

			if toolResult.Success {
				// Generate a summary of what was done
				content = h.summarizeAction(detectedTool, toolParams, toolResult.Output)

				// Store successful action as a skill for future reference
				if h.memoryStore != nil && req.UserID != "" {
					if err := h.memoryStore.StoreSkill(ctx, req.UserID, input, detectedTool, toolParams, true); err != nil {
						h.log.Debug("[PinkyCompat] Failed to store skill: %v", err)
					} else {
						h.log.Info("[PinkyCompat] Skill stored: %s -> %s", input[:min(30, len(input))], detectedTool)
					}
				}
			} else {
				content = fmt.Sprintf("I encountered an error: %s", toolResult.Error)
			}
		} else {
			// No specific tool detected - search skill memory for similar past actions
			var skillMatched bool
			if h.memoryStore != nil && req.UserID != "" {
				skills, err := h.memoryStore.SearchSkills(ctx, req.UserID, input, 3)
				if err == nil && len(skills) > 0 {
					// Found a similar past action - use the same tool
					bestMatch := skills[0]
					h.log.Info("[PinkyCompat] Skill recall: '%s' -> %s", bestMatch.Intent[:min(30, len(bestMatch.Intent))], bestMatch.Tool)

					// Execute the remembered tool with adapted parameters
					adaptedParams := h.adaptSkillParams(bestMatch.Params, input)
					toolResult := h.executor.Execute(ctx, &internalagent.ToolCall{
						Name:   bestMatch.Tool,
						Params: adaptedParams,
					})

					toolCalls = append(toolCalls, PinkyToolCall{
						ID:    "call_1",
						Tool:  bestMatch.Tool,
						Input: stringMapToAnyMap(adaptedParams),
					})

					if toolResult.Success {
						content = h.summarizeAction(bestMatch.Tool, adaptedParams, toolResult.Output)
						skillMatched = true
					}
				}
			}

			// Fall back to Brain reasoning if skill memory didn't help
			if !skillMatched {
				// Check if this needs real-world info (web search) or can be answered by Brain
				needsWebSearch := h.needsRealWorldInfo(input)

				if needsWebSearch {
					h.log.Info("[PinkyCompat] Query needs real-world info, using web search")
					searchResult := h.executor.Execute(ctx, &internalagent.ToolCall{
						Name:   "web_search",
						Params: map[string]string{"query": input},
					})

					toolCalls = append(toolCalls, PinkyToolCall{
						ID:    "call_1",
						Tool:  "web_search",
						Input: map[string]any{"query": input},
					})

					if searchResult.Success {
						content = extractSummary(searchResult.Output)
						if content == searchResult.Output {
							if len(searchResult.Output) > 500 {
								content = searchResult.Output[:500] + "..."
							} else {
								content = searchResult.Output
							}
						}
					} else {
						content = fmt.Sprintf("I couldn't find information about that: %s", searchResult.Error)
					}
				} else {
					// Use Agent Router for intelligent routing between local and frontier brains
					// The router checks skill memory first, then routes based on complexity
					h.log.Info("[PinkyCompat] Using Agent Router for reasoning/knowledge question")
					enrichedPrompt := h.enrichPromptWithMemory(ctx, req.UserID, input)

					// Build prompt with conversation context
					var brainPrompt string
					if conversationContext != "" && len(req.Messages) > 1 {
						brainPrompt = fmt.Sprintf("CONVERSATION HISTORY:\n%s\n\nUSER QUESTION: %s\n\nAnswer the question directly and concisely.",
							conversationContext, enrichedPrompt)
					} else {
						brainPrompt = enrichedPrompt
					}

					// Use router for intelligent brain selection
					routedContent, decision, err := h.processWithRouter(ctx, req.UserID, brainPrompt, "", req.Messages)
					if err != nil {
						h.log.Debug("[PinkyCompat] Router processing failed: %v, falling back to local", err)
						// Fallback to direct local brain call
						result, localErr := h.brain.Process(ctx, brainPrompt)
						if localErr != nil {
							content = "I encountered an error processing your request."
						} else {
							content = contentToString(result.FinalContent)
						}
					} else {
						content = routedContent
						if decision != nil && decision.MatchedSkill != nil {
							h.log.Info("[PinkyCompat] Answered using learned skill: %s", decision.MatchedSkill.Intent[:min(30, len(decision.MatchedSkill.Intent))])
						}
					}
				}
			}
		}
	} else {
		// Pure meta/identity question - answer directly without tools
		// Enrich the prompt with user memory context
		memoryContext := h.buildMemoryContext(ctx, req.UserID)

		// Try to answer personal questions directly from memory
		if memoryContext != "" {
			directAnswer := h.tryAnswerFromMemory(input, memoryContext)
			if directAnswer != "" {
				h.log.Info("[PinkyCompat] Answered from memory context")
				content = directAnswer
			}
		}

		// If we couldn't answer from memory, try the Agent Router
		if content == "" {
			enrichedPrompt := h.enrichPromptWithMemory(ctx, req.UserID, input)
			// Inject current date/time as authoritative system context
			currentTime := time.Now()
			timeContext := fmt.Sprintf("SYSTEM TIME (authoritative): Today is %s. The current time is %s.",
				currentTime.Format("Monday, January 2, 2006"),
				currentTime.Format("3:04 PM MST"))

			// Build full prompt with conversation history for context
			var metaPrompt string
			if conversationContext != "" && len(req.Messages) > 1 {
				// Include conversation history so Brain knows what was discussed
				metaPrompt = fmt.Sprintf("%s\n\nCONVERSATION HISTORY:\n%s\nUSER CONTEXT:\n%s\n\nRespond to the user's latest message. Be conversational and aware of the discussion above. Answer concisely.",
					timeContext, conversationContext, enrichedPrompt)
			} else {
				metaPrompt = fmt.Sprintf("%s\n\nAnswer concisely in 1-3 sentences. Give the direct answer only. Use the user context if relevant.\n\nQuestion: %s",
					timeContext, enrichedPrompt)
			}

			// Use router for intelligent brain selection (with panic recovery)
			func() {
				defer func() {
					if r := recover(); r != nil {
						h.log.Error("[PinkyCompat] Router panic: %v", r)
						// Fallback to memory-based answer if available
						if memoryContext != "" {
							content = "Based on what I know about you, let me check... " + h.summarizeMemory(memoryContext, input)
						} else {
							content = "I'm Pinky, your AI assistant. I'm here to help you with any questions or tasks."
						}
					}
				}()
				routedContent, decision, err := h.processWithRouter(ctx, req.UserID, metaPrompt, "", req.Messages)
				if err != nil {
					h.log.Error("[PinkyCompat] Router process error: %v", err)
					content = "I encountered an error processing your request."
				} else {
					content = routedContent
					if decision != nil {
						h.log.Info("[PinkyCompat] Meta query processed via %s: %s", decision.Brain, decision.Reason)
					}
				}
			}()
		}
	}

	// Convert Brain response to Pinky format
	resp := PinkyThinkResponse{
		Content:   content,
		ToolCalls: toolCalls,
		Done:      true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// looksLikeShellCommand checks if the input appears to be a shell command.
// This function is intentionally conservative - it only returns true for
// inputs that are clearly shell commands, not paths mentioned in conversation.
func (h *PinkyCompatHandler) looksLikeShellCommand(input string) bool {
	input = strings.TrimSpace(input)
	if input == "" {
		return false
	}

	// Get the first word of the input
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}
	firstWord := strings.ToLower(parts[0])

	// Quick check: if input looks like natural language, skip shell detection
	// Natural language indicators: polite words, articles, common sentence starters
	naturalLanguagePatterns := []string{"please", "can you", "could you", "help me", "i want", "i need"}
	inputLower := strings.ToLower(input)
	for _, pattern := range naturalLanguagePatterns {
		if strings.Contains(inputLower, pattern) {
			return false
		}
	}

	// "make" is ambiguous - it's both a build tool AND a common English verb
	// Only treat as shell command if followed by typical make arguments
	if firstWord == "make" {
		if len(parts) == 1 {
			return true // bare "make" is likely the build tool
		}
		secondWord := strings.ToLower(parts[1])
		// Common make targets/flags: clean, build, test, all, install, -f, -j, -C
		makeArgs := []string{"clean", "build", "test", "all", "install", "check", "dist", "-f", "-j", "-c", "-n", "-k"}
		for _, arg := range makeArgs {
			if secondWord == arg || strings.HasPrefix(secondWord, "-") {
				return true
			}
		}
		// "make a", "make new", "make the" = natural language
		naturalFollowers := []string{"a", "an", "the", "new", "my", "some", "sure"}
		for _, nf := range naturalFollowers {
			if secondWord == nf {
				return false
			}
		}
		return true // other make targets
	}

	// Common shell commands that should be executed directly
	shellCommands := []string{
		"ls", "cd", "pwd", "cat", "head", "tail", "grep", "find", "echo",
		"mkdir", "rm", "cp", "mv", "touch", "chmod", "chown",
		"ps", "top", "kill", "df", "du", "free", "uname",
		"git", "npm", "yarn", "pnpm", "go", "python", "python3", "pip", "pip3",
		"docker", "kubectl", "cargo", "rustc",
		"curl", "wget", "ssh", "scp", "tar", "zip", "unzip",
		"which", "whereis", "whoami", "date", "cal", "uptime",
		"brew", "apt", "yum", "dnf", "pacman",
		"node", "deno", "bun", "ruby", "perl", "php",
		"vim", "nano", "less", "more", "wc", "sort", "uniq", "cut", "awk", "sed",
		"env", "export", "source", ".", "alias",
	}

	// Check if it starts with a common shell command
	for _, cmd := range shellCommands {
		if firstWord == cmd {
			return true
		}
	}

	// Check for relative executable paths like ./script.sh
	// These are explicit commands to run something
	if strings.HasPrefix(input, "./") {
		return true
	}

	// DO NOT treat absolute paths as shell commands!
	// A path like "/Users/normanking/folder" is NOT a command.
	// Only treat as command if it looks like an explicit executable invocation:
	// - Path ends with common executable extensions (.sh, .py, .rb, .pl)
	// - Path has command-line arguments after it
	// - Path contains /bin/ or /sbin/ (system executables)
	if strings.HasPrefix(input, "/") {
		// Check if this looks like an executable invocation
		path := parts[0]
		hasArgs := len(parts) > 1

		// System executable paths
		if strings.Contains(path, "/bin/") || strings.Contains(path, "/sbin/") {
			return true
		}

		// Executable file extensions with arguments
		execExtensions := []string{".sh", ".py", ".rb", ".pl", ".bash", ".zsh"}
		for _, ext := range execExtensions {
			if strings.HasSuffix(path, ext) && hasArgs {
				return true
			}
		}

		// If it's just a bare path (no args, no exec extension), it's NOT a command
		// The user is probably just referencing the path in conversation
		return false
	}

	// Check for common shell patterns (pipes, chaining, redirection)
	if strings.Contains(input, "|") || strings.Contains(input, "&&") || strings.Contains(input, ">>") {
		return true
	}

	return false
}

// shouldEnableToolsForQuery checks if the query might need tools.
// AGENTIC APPROACH: Default to enabling tools unless clearly conversational.
// The Brain should decide what actions to take, not pattern matching.
func (h *PinkyCompatHandler) shouldEnableToolsForQuery(input string) bool {
	inputLower := strings.ToLower(input)

	// Only disable tools for clearly conversational/meta queries
	// Everything else gets tool access - let the Brain decide what to use

	// Pure greetings - no action needed
	pureGreetings := []string{
		"hello", "hi", "hey", "good morning", "good afternoon", "good evening",
		"how are you", "what's up", "thank you", "thanks", "goodbye", "bye",
	}
	trimmedLower := strings.TrimSpace(inputLower)
	for _, greeting := range pureGreetings {
		// Only match if the query IS the greeting (not contains)
		if trimmedLower == greeting || trimmedLower == greeting+"!" || trimmedLower == greeting+"." {
			return false
		}
	}

	// Meta questions about the AI itself - conversational
	if strings.Contains(inputLower, "who are you") ||
		strings.Contains(inputLower, "what are you") ||
		strings.Contains(inputLower, "your name") ||
		strings.Contains(inputLower, "introduce yourself") {
		return false
	}

	// Very short queries that are likely just questions (< 10 chars)
	if len(trimmedLower) < 10 {
		return false
	}

	// DEFAULT: Enable tools - let the system be agentic
	// The Brain will decide what tools to use based on understanding the request
	return true
}

// extractSummary extracts just the summary from web search results XML.
func extractSummary(searchResults string) string {
	// Look for <summary>...</summary> tags
	startTag := "<summary>"
	endTag := "</summary>"

	startIdx := strings.Index(searchResults, startTag)
	if startIdx == -1 {
		return searchResults // No summary tag, return as-is
	}

	startIdx += len(startTag)
	endIdx := strings.Index(searchResults[startIdx:], endTag)
	if endIdx == -1 {
		return searchResults // No closing tag, return as-is
	}

	summary := strings.TrimSpace(searchResults[startIdx : startIdx+endIdx])
	return summary
}

// extractHeadlines extracts actual headlines from search result content.
func extractHeadlines(searchResults string) string {
	var headlines []string
	remaining := searchResults

	// First try to extract headlines from <content> sections (marked with ## in markdown)
	for i := 0; i < 10 && len(headlines) < 5; i++ {
		contentStart := strings.Index(remaining, "<content>")
		if contentStart == -1 {
			break
		}
		remaining = remaining[contentStart+9:]
		contentEnd := strings.Index(remaining, "</content>")
		if contentEnd == -1 {
			break
		}
		content := remaining[:contentEnd]
		remaining = remaining[contentEnd:]

		// Look for markdown headlines (## Headline)
		lines := strings.Split(content, "##")
		for _, line := range lines[1:] { // Skip first empty element
			line = strings.TrimSpace(line)
			// Take first sentence/phrase (up to period or newline)
			if idx := strings.Index(line, "."); idx > 0 && idx < 100 {
				line = line[:idx]
			}
			if idx := strings.Index(line, "\n"); idx > 0 {
				line = line[:idx]
			}
			line = strings.TrimSpace(line)
			// Filter out generic navigation items
			if line != "" && len(line) > 10 && len(line) < 120 &&
				!strings.Contains(strings.ToLower(line), "news from") &&
				!strings.Contains(strings.ToLower(line), "latest updates") &&
				!strings.Contains(strings.ToLower(line), "top stories") {
				headlines = append(headlines, fmt.Sprintf("%d. %s", len(headlines)+1, line))
				if len(headlines) >= 5 {
					break
				}
			}
		}
	}

	// If no markdown headlines found, try extracting from summary
	if len(headlines) == 0 {
		summary := extractSummary(searchResults)
		if summary != "" && summary != searchResults {
			return summary
		}
	}

	if len(headlines) == 0 {
		return "No specific headlines found. Please check the latest news sources for Tokyo."
	}
	return strings.Join(headlines, "\n")
}

// detectToolFromQuery analyzes the query and returns the appropriate tool and parameters.
// Uses semantic understanding of intent rather than exact phrase matching.
func (h *PinkyCompatHandler) detectToolFromQuery(input string) (string, map[string]string) {
	inputLower := strings.ToLower(input)

	// Helper: check if input contains any of the words as WHOLE WORDS
	// This prevents false matches like "normanking" containing "rm"
	containsAny := func(words ...string) bool {
		// Split input into words for word-boundary matching
		inputWords := strings.Fields(inputLower)
		for _, w := range words {
			// For multi-word phrases, check substring
			if strings.Contains(w, " ") {
				if strings.Contains(inputLower, w) {
					return true
				}
				continue
			}
			// For single words, check word boundaries
			for _, inputWord := range inputWords {
				// Exact match or word starts/ends with the target
				// Handle punctuation by trimming common suffixes
				cleanWord := strings.TrimRight(inputWord, ".,!?;:")
				if cleanWord == w {
					return true
				}
			}
		}
		return false
	}

	// Extract path early - many operations need it
	path := h.extractPathFromQuery(input)

	// === FILE/FOLDER OPERATIONS ===

	// Create folder/directory - semantic: "create" + "folder/directory" anywhere
	if containsAny("create", "make", "new", "set up", "setup", "prepare", "initialize", "init") && containsAny("folder", "directory", "dir", "structure") {
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Creating folder %s", path)
			return "run_command", map[string]string{"command": "mkdir -p " + path}
		}
	}

	// Delete folder/directory
	if containsAny("delete", "remove", "rm") && containsAny("folder", "directory", "dir") {
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Removing folder %s", path)
			return "run_command", map[string]string{"command": "rm -rf " + path}
		}
	}

	// Change directory / navigate
	if containsAny("change", "go", "navigate", "switch", "cd") && containsAny("to", "into", "folder", "directory") {
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Changing to %s", path)
			return "run_command", map[string]string{"command": "cd " + path + " && pwd"}
		}
	}

	// List directory contents
	if containsAny("list", "show", "what's in", "ls") && containsAny("folder", "directory", "files", "contents") {
		targetPath := path
		if targetPath == "" {
			targetPath = "."
		}
		h.log.Info("[PinkyCompat] Agentic action: Listing %s", targetPath)
		return "list_directory", map[string]string{"path": targetPath}
	}

	// Current directory
	if containsAny("what folder", "what directory", "current folder", "current directory", "where are we", "pwd", "which folder") {
		return "run_command", map[string]string{"command": "pwd"}
	}

	// Read file
	if containsAny("read", "show", "display", "cat", "view") && containsAny("file", "contents") {
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Reading file %s", path)
			return "read_file", map[string]string{"path": path}
		}
	}

	// Create file - detect "create/make/write" + "file" + filename pattern
	if containsAny("create", "make", "write") && containsAny("file") {
		// Extract filename from patterns like "called X", "named X", or just a filename with extension
		filename := h.extractFilenameFromQuery(input)
		if filename != "" {
			h.log.Info("[PinkyCompat] Agentic action: Creating file %s", filename)
			// Generate appropriate content based on file type
			content := h.generateFileTemplate(filename, input)
			return "write_file", map[string]string{"path": filename, "content": content}
		}
	}

	// === IDE / CODING OPERATIONS ===

	// Open IDE/editor
	if containsAny("open", "launch", "start") && containsAny("ide", "editor", "vscode", "vs code", "cursor", "coding") {
		targetPath := path
		if targetPath == "" {
			targetPath = "."
		}
		if strings.Contains(inputLower, "cursor") {
			h.log.Info("[PinkyCompat] Agentic action: Opening Cursor at %s", targetPath)
			return "run_command", map[string]string{"command": "cursor " + targetPath}
		}
		h.log.Info("[PinkyCompat] Agentic action: Opening VS Code at %s", targetPath)
		return "run_command", map[string]string{"command": "code " + targetPath}
	}

	// Create app/project with PRD - trigger full application generation
	if containsAny("create", "build", "make", "generate") && containsAny("app", "application", "project") {
		// Check if this is a full PRD-based generation request
		if h.isAppGenerationRequest(input) {
			h.log.Info("[PinkyCompat] Detected PRD-based application generation request")
			return "generate_application", map[string]string{"input": input}
		}
		// Simple project creation (just folder + IDE)
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Creating project at %s", path)
			return "run_command", map[string]string{"command": "mkdir -p " + path + " && cd " + path + " && code ."}
		}
	}

	// Write code/script
	if containsAny("write", "create") && containsAny("code", "script", "program") {
		if path != "" {
			h.log.Info("[PinkyCompat] Agentic action: Opening for coding at %s", path)
			return "run_command", map[string]string{"command": "cd " + path + " && code ."}
		}
	}

	// File/directory queries - look for path-like patterns or file-related questions
	if strings.Contains(inputLower, "files in") ||
		strings.Contains(inputLower, "folder") ||
		strings.Contains(inputLower, "directory") ||
		strings.Contains(inputLower, "what's in") ||
		strings.Contains(inputLower, "list ") ||
		strings.Contains(inputLower, "show me") && (strings.Contains(inputLower, "files") || strings.Contains(inputLower, "folder")) {

		// Try to extract path from query
		path := h.extractPathFromQuery(input)
		if path != "" {
			return "list_directory", map[string]string{"path": path}
		}
	}

	// Read file queries
	if strings.Contains(inputLower, "read ") && strings.Contains(inputLower, "file") ||
		strings.Contains(inputLower, "contents of") ||
		strings.Contains(inputLower, "what does") && strings.Contains(inputLower, "say") {

		path := h.extractPathFromQuery(input)
		if path != "" {
			return "read_file", map[string]string{"path": path}
		}
	}

	// System info queries that need shell commands
	if strings.Contains(inputLower, "disk space") ||
		strings.Contains(inputLower, "memory") && strings.Contains(inputLower, "usage") ||
		strings.Contains(inputLower, "cpu") ||
		strings.Contains(inputLower, "uptime") ||
		strings.Contains(inputLower, "processes") {

		// Map to appropriate command
		var cmd string
		if strings.Contains(inputLower, "disk space") {
			cmd = "df -h"
		} else if strings.Contains(inputLower, "memory") {
			cmd = "vm_stat"
		} else if strings.Contains(inputLower, "uptime") {
			cmd = "uptime"
		} else if strings.Contains(inputLower, "processes") {
			cmd = "ps aux | head -20"
		} else {
			cmd = "top -l 1 | head -10"
		}
		return "run_command", map[string]string{"command": cmd}
	}

	// Hardware/circuit design queries - route to GateFlow
	// Includes both highly specific domain terms AND general circuit design requests
	if strings.Contains(inputLower, "verilog") ||
		strings.Contains(inputLower, "systemverilog") ||
		strings.Contains(inputLower, "rtl") ||
		strings.Contains(inputLower, "fpga") ||
		strings.Contains(inputLower, "asic") ||
		strings.Contains(inputLower, "fsm") && strings.Contains(inputLower, "design") ||
		strings.Contains(inputLower, "fifo") && (strings.Contains(inputLower, "design") || strings.Contains(inputLower, "create")) ||
		strings.Contains(inputLower, "testbench") ||
		strings.Contains(inputLower, "synthesis") ||
		strings.Contains(inputLower, "hardware design") ||
		strings.Contains(inputLower, "module") && strings.Contains(inputLower, "create") ||
		// General circuit design requests
		strings.Contains(inputLower, "circuit") ||
		strings.Contains(inputLower, "schematic") ||
		strings.Contains(inputLower, "electronics design") ||
		strings.Contains(inputLower, "digital design") ||
		(strings.Contains(inputLower, "build") && (strings.Contains(inputLower, "clock") || strings.Contains(inputLower, "timer") || strings.Contains(inputLower, "counter"))) {
		return "gateflow", map[string]string{"query": input}
	}

	// Web search for real-world info (weather, news, places, etc.)
	if strings.Contains(inputLower, "weather") ||
		strings.Contains(inputLower, "news") ||
		strings.Contains(inputLower, "restaurant") ||
		strings.Contains(inputLower, "price") ||
		strings.Contains(inputLower, "score") ||
		strings.Contains(inputLower, "today") ||
		strings.Contains(inputLower, "latest") {
		return "web_search", map[string]string{"query": input}
	}

	// No specific tool detected - return empty (will fallback to web search)
	return "", nil
}

// extractPathFromQuery tries to extract a file path from the query.
func (h *PinkyCompatHandler) extractPathFromQuery(input string) string {
	inputLower := strings.ToLower(input)

	// Handle "home folder" / "home directory" / "my home" → user's actual home
	if strings.Contains(inputLower, "home folder") ||
		strings.Contains(inputLower, "home directory") ||
		strings.Contains(inputLower, "my home") ||
		strings.Contains(inputLower, "my folder") {
		// Return user's home directory
		home := os.Getenv("HOME")
		if home == "" {
			home = "/Users/normanking" // Fallback
		}
		return home
	}

	// Common path patterns
	pathPatterns := []string{
		"/Users", "/tmp", "/var", "/etc", "~/",
		"./", "../",
	}

	for _, pattern := range pathPatterns {
		if idx := strings.Index(input, pattern); idx >= 0 {
			// Extract the path (stop at space, question mark, or end)
			remaining := input[idx:]
			endIdx := len(remaining)
			for i, c := range remaining {
				if c == ' ' || c == '?' || c == '\n' {
					endIdx = i
					break
				}
			}
			path := remaining[:endIdx]
			// Clean up common suffixes
			path = strings.TrimSuffix(path, "?")
			path = strings.TrimSuffix(path, ".")

			// Expand ~
			if strings.HasPrefix(path, "~/") {
				home := os.Getenv("HOME")
				if home != "" {
					path = home + path[1:]
				}
			}
			return path
		}
	}

	// Handle /Home → redirect to actual home directory (macOS quirk)
	if strings.Contains(input, "/Home") {
		home := os.Getenv("HOME")
		if home == "" {
			home = "/Users/normanking"
		}
		return home
	}

	// Try to find "in X folder" or "in X directory" pattern
	if idx := strings.Index(inputLower, " in "); idx >= 0 {
		remaining := input[idx+4:]
		// Extract until "folder" or "directory" or end
		words := strings.Fields(remaining)
		if len(words) > 0 {
			// If it looks like a path, use it
			firstWord := words[0]
			if strings.HasPrefix(firstWord, "/") || strings.HasPrefix(firstWord, "~") {
				return firstWord
			}
		}
	}

	return ""
}

// extractFilenameFromQuery extracts a filename from queries like "called helloworld.py" or "named test.txt"
func (h *PinkyCompatHandler) extractFilenameFromQuery(input string) string {
	inputLower := strings.ToLower(input)

	// Look for patterns like "called X", "named X"
	patterns := []string{"called ", "named ", "file "}
	for _, pattern := range patterns {
		if idx := strings.Index(inputLower, pattern); idx >= 0 {
			remaining := input[idx+len(pattern):]
			words := strings.Fields(remaining)
			if len(words) > 0 {
				filename := words[0]
				// Clean up punctuation
				filename = strings.TrimRight(filename, ".,;:!?")
				// Check if it looks like a filename (has extension)
				if strings.Contains(filename, ".") {
					return filename
				}
			}
		}
	}

	// Look for any word with a file extension
	extensions := []string{".py", ".js", ".ts", ".go", ".java", ".c", ".cpp", ".h", ".rb", ".rs",
		".txt", ".md", ".json", ".yaml", ".yml", ".xml", ".html", ".css", ".sh", ".bash"}
	words := strings.Fields(input)
	for _, word := range words {
		cleanWord := strings.TrimRight(word, ".,;:!?")
		for _, ext := range extensions {
			if strings.HasSuffix(strings.ToLower(cleanWord), ext) {
				return cleanWord
			}
		}
	}

	return ""
}

// generateFileTemplate creates appropriate content based on file type
func (h *PinkyCompatHandler) generateFileTemplate(filename, query string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.TrimSuffix(filename, ext)

	switch ext {
	case ".py":
		// Check if hello world was requested
		if strings.Contains(strings.ToLower(query), "hello") {
			return `#!/usr/bin/env python3
"""` + name + ` - A simple Python script."""

def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()
`
		}
		return `#!/usr/bin/env python3
"""` + name + ` - A Python script."""

def main():
    pass

if __name__ == "__main__":
    main()
`
	case ".js":
		return `// ` + name + `.js

function main() {
    console.log("Hello, World!");
}

main();
`
	case ".go":
		return `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
`
	case ".sh", ".bash":
		return `#!/bin/bash
# ` + name + `

echo "Hello, World!"
`
	case ".html":
		return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + name + `</title>
</head>
<body>
    <h1>Hello, World!</h1>
</body>
</html>
`
	default:
		// For txt, md, and unknown types, create empty or minimal content
		return ""
	}
}

// buildAgenticPrompt creates a prompt that includes tool descriptions so the LLM can reason about when to use them.
func (h *PinkyCompatHandler) buildAgenticPrompt(input string) string {
	return fmt.Sprintf(`You are a helpful AI assistant with access to the following tools. Use them when you need information you don't have.

## Available Tools

### web_search
Search the web for current information, news, weather, prices, events, or any real-world knowledge you don't have.
To use: <tool>web_search</tool><params>{"query": "your search query"}</params>

### run_command
Execute a shell command to get system information, list files, check disk space, etc.
To use: <tool>run_command</tool><params>{"command": "your command"}</params>

### read_file
Read the contents of a file.
To use: <tool>read_file</tool><params>{"path": "/path/to/file"}</params>

### list_directory
List files and directories in a path.
To use: <tool>list_directory</tool><params>{"path": "/path/to/directory"}</params>

## Instructions
- If you don't have the information needed to answer the question, use a tool to get it
- For questions about current events, weather, news, sports, prices - use web_search
- For questions about files, directories, system info - use run_command, read_file, or list_directory
- For questions about places, restaurants, businesses - use web_search
- Answer directly if you have the knowledge without needing tools
- If you use a tool, include ONLY the tool call in your response

User's question: %s`, input)
}

// stringMapToAnyMap converts map[string]string to map[string]any for JSON serialization.
func stringMapToAnyMap(m map[string]string) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// handleMemoryStore stores a memory (stub for now).
func (h *PinkyCompatHandler) handleMemoryStore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleMemorySearch searches memories (stub for now).
func (h *PinkyCompatHandler) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"memories": []any{},
	})
}

// writeError writes an error response.
func (h *PinkyCompatHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(PinkyThinkResponse{
		Error: message,
		Done:  true,
	})
}

// Note: contentToString is defined in server.go

// needsRealWorldInfo determines if a query requires current/real-world information
// that should be fetched via web search rather than answered by Brain reasoning.
func (h *PinkyCompatHandler) needsRealWorldInfo(input string) bool {
	inputLower := strings.ToLower(input)

	// Keywords that indicate need for current/real-world information
	realWorldKeywords := []string{
		"weather", "forecast", "temperature",
		"news", "latest", "today's", "current events",
		"stock price", "stock market", "crypto price",
		"sports score", "game score", "match result",
		"restaurant", "hotel", "store near",
		"open now", "hours of operation",
		"release date", "when does", "when is",
		"how much does", "price of", "cost of",
		"who won", "who is winning",
		"traffic", "directions to",
	}

	for _, keyword := range realWorldKeywords {
		if strings.Contains(inputLower, keyword) {
			return true
		}
	}

	// If the question is about general knowledge, math, coding, explanation - use Brain
	// These don't need web search
	brainKeywords := []string{
		"what is", "how do", "explain", "define",
		"calculate", "compute", "solve",
		"write code", "write a", "create a",
		"why", "how does", "what does",
		"compare", "difference between",
		"help me", "can you",
	}

	for _, keyword := range brainKeywords {
		if strings.Contains(inputLower, keyword) {
			return false
		}
	}

	// Default: if we're not sure, use Brain (avoids unnecessary web searches)
	return false
}

// buildMemoryContext retrieves user memory and builds a context string for injection.
func (h *PinkyCompatHandler) buildMemoryContext(ctx context.Context, userID string) string {
	if h.memoryStore == nil || userID == "" {
		return ""
	}

	mem, err := h.memoryStore.GetUserMemory(ctx, userID)
	if err != nil {
		h.log.Warn("[PinkyCompat] Failed to get user memory: %v", err)
		return ""
	}

	if mem == nil {
		return ""
	}

	var sb strings.Builder
	hasContext := false

	// Add user name if known
	if mem.Name != "" {
		sb.WriteString(fmt.Sprintf("User's name: %s\n", mem.Name))
		hasContext = true
	}

	// Add user role if known
	if mem.Role != "" {
		sb.WriteString(fmt.Sprintf("User's role: %s\n", mem.Role))
		hasContext = true
	}

	// Add preferences
	if mem.PrefersConcise {
		sb.WriteString("User prefers concise responses.\n")
		hasContext = true
	}
	if mem.PrefersVerbose {
		sb.WriteString("User prefers detailed responses.\n")
		hasContext = true
	}

	// Add custom facts
	if len(mem.CustomFacts) > 0 {
		sb.WriteString("\nKnown facts about this user:\n")
		for _, fact := range mem.CustomFacts {
			sb.WriteString(fmt.Sprintf("- %s\n", fact))
		}
		hasContext = true
	}

	if !hasContext {
		return ""
	}

	return sb.String()
}

// enrichPromptWithMemory adds user memory context to the prompt.
func (h *PinkyCompatHandler) enrichPromptWithMemory(ctx context.Context, userID, prompt string) string {
	memoryContext := h.buildMemoryContext(ctx, userID)
	if memoryContext == "" {
		return prompt
	}

	h.log.Info("[PinkyCompat] Enriching prompt with user memory for user=%s", userID)
	return fmt.Sprintf("<user_context>\n%s</user_context>\n\n%s", memoryContext, prompt)
}

// tryAnswerFromMemory attempts to answer a question directly from memory facts.
// Also handles conversational queries and circuit/design queries when Brain.Process is unavailable.
func (h *PinkyCompatHandler) tryAnswerFromMemory(question, memoryContext string) string {
	// All queries go through Brain.Process for cognitive reasoning.
	// The Brain uses memory context, temporal awareness, and multi-lobe processing
	// to generate thoughtful responses rather than pattern-matched canned replies.
	//
	// Memory context is already enriched into the prompt sent to Brain.Process,
	// and system time is injected as authoritative context for date/time awareness.
	_ = memoryContext // Memory is used by Brain.Process, not here
	_ = question      // Question is processed by cognitive layers
	return ""         // Forces all queries through Brain.Process
}

// generateCircuitDesignResponse creates a helpful response for circuit design queries.
func (h *PinkyCompatHandler) generateCircuitDesignResponse(question string) string {
	questionLower := strings.ToLower(question)

	// Clock/timer circuit
	if strings.Contains(questionLower, "clock") || strings.Contains(questionLower, "timer") {
		return `Here's a basic digital clock circuit design:

**Components needed:**
- 555 Timer IC (for 1Hz clock signal)
- CD4017 Decade Counter (for seconds)
- CD4510 BCD Counter (for minutes/hours)
- 7447 BCD to 7-Segment Decoder
- 7-Segment LED Displays
- Resistors, capacitors

**Basic circuit flow:**
1. 555 Timer generates 1Hz square wave
2. Signal feeds CD4017 decade counter for 0-9 seconds
3. Carry output feeds minute counter (CD4510)
4. Carry from minutes feeds hour counter
5. BCD outputs go through 7447 decoders to displays

**Verilog implementation:**
` + "```" + `verilog
module clock_counter(
    input clk_1hz,
    input reset,
    output [3:0] seconds_ones,
    output [2:0] seconds_tens,
    output [3:0] minutes_ones,
    output [2:0] minutes_tens,
    output [3:0] hours_ones,
    output [1:0] hours_tens
);
    // Counter logic with rollover
    reg [3:0] sec_ones = 0;
    reg [2:0] sec_tens = 0;

    always @(posedge clk_1hz or posedge reset) begin
        if (reset) begin
            sec_ones <= 0;
            sec_tens <= 0;
        end else begin
            if (sec_ones == 9) begin
                sec_ones <= 0;
                if (sec_tens == 5)
                    sec_tens <= 0;
                else
                    sec_tens <= sec_tens + 1;
            end else
                sec_ones <= sec_ones + 1;
        end
    end
endmodule
` + "```" + `

Would you like me to expand on any part of this design?`
	}

	// LED circuit
	if strings.Contains(questionLower, "led") {
		return `Here's a basic LED circuit design:

**Simple LED with resistor:**
- LED forward voltage: ~2V (red), ~3V (blue/white)
- Current limiting resistor: R = (Vcc - Vf) / If
- For 5V supply, red LED (20mA): R = (5-2)/0.02 = 150Ω

**LED blinker with 555:**
1. 555 timer in astable mode
2. R1, R2, C1 set frequency
3. Output drives LED through resistor

Would you like a specific LED circuit design?`
	}

	// Counter circuit
	if strings.Contains(questionLower, "counter") {
		return `Here's a basic counter circuit design:

**4-bit binary counter with 74LS193:**
- CLR: Clear/reset input
- UP: Count up clock input
- DOWN: Count down clock input
- QA-QD: Binary outputs
- CASCADE: Carry/borrow for chaining

**Verilog:**
` + "```" + `verilog
module counter_4bit(
    input clk,
    input reset,
    input enable,
    output reg [3:0] count
);
    always @(posedge clk or posedge reset) begin
        if (reset)
            count <= 4'b0000;
        else if (enable)
            count <= count + 1;
    end
endmodule
` + "```" + `

Would you like a more specific counter design?`
	}

	// Generic circuit response
	return `I can help with digital circuit design! Here are some areas I can assist with:

- **Clock/Timer circuits**: 555 timer configurations, crystal oscillators
- **Counter circuits**: Ripple counters, synchronous counters, decade counters
- **Display circuits**: 7-segment drivers, LED matrices
- **Logic circuits**: Gates, multiplexers, decoders, flip-flops
- **Memory circuits**: Latches, registers, simple RAM

What specific circuit would you like me to design? Please specify:
1. The function (counter, timer, display, etc.)
2. The technology (TTL ICs, CMOS, FPGA/Verilog)
3. Any constraints (voltage, speed, power)`
}

// adaptSkillParams takes stored skill parameters and adapts them for the new input.
// For example, if the stored skill used path /tmp/old, but new input mentions /tmp/new,
// we update the path parameter accordingly.
func (h *PinkyCompatHandler) adaptSkillParams(storedParams map[string]string, newInput string) map[string]string {
	adapted := make(map[string]string)
	for k, v := range storedParams {
		adapted[k] = v
	}

	// Try to extract a new path from the current input
	newPath := h.extractPathFromQuery(newInput)
	if newPath == "" {
		return adapted
	}

	// Update path-related parameters
	if _, hasPath := adapted["path"]; hasPath {
		adapted["path"] = newPath
	}

	// Update command if it contains a path
	if cmd, hasCmd := adapted["command"]; hasCmd {
		// Replace old path in command with new path
		// Look for common command patterns
		if strings.HasPrefix(cmd, "mkdir -p ") {
			adapted["command"] = "mkdir -p " + newPath
		} else if strings.HasPrefix(cmd, "rm -rf ") {
			adapted["command"] = "rm -rf " + newPath
		} else if strings.HasPrefix(cmd, "cd ") {
			adapted["command"] = "cd " + newPath + " && pwd"
		} else if strings.HasPrefix(cmd, "code ") {
			adapted["command"] = "code " + newPath
		} else if strings.HasPrefix(cmd, "cursor ") {
			adapted["command"] = "cursor " + newPath
		}
	}

	return adapted
}

// summarizeAction generates a human-friendly summary of what action was taken.
func (h *PinkyCompatHandler) summarizeAction(tool string, params map[string]string, output string) string {
	cmd := params["command"]
	path := params["path"]

	// Generate summary based on what was done
	switch {
	case strings.HasPrefix(cmd, "mkdir"):
		// Extract path from mkdir command
		parts := strings.Split(cmd, " ")
		if len(parts) >= 3 {
			path = parts[len(parts)-1]
		}
		return fmt.Sprintf("Done! I created the folder: %s", path)

	case strings.HasPrefix(cmd, "rm -rf"):
		parts := strings.Split(cmd, " ")
		if len(parts) >= 3 {
			path = parts[len(parts)-1]
		}
		return fmt.Sprintf("Done! I removed the folder: %s", path)

	case strings.HasPrefix(cmd, "cd "):
		// Extract destination from cd command
		parts := strings.Split(cmd, "&&")
		if len(parts) > 0 {
			cdPart := strings.TrimPrefix(strings.TrimSpace(parts[0]), "cd ")
			return fmt.Sprintf("Done! Changed to directory: %s", cdPart)
		}
		return "Done! Changed directory."

	case strings.HasPrefix(cmd, "code ") || strings.HasPrefix(cmd, "cursor "):
		editor := "VS Code"
		if strings.HasPrefix(cmd, "cursor") {
			editor = "Cursor"
		}
		return fmt.Sprintf("Done! Opened %s for editing.", editor)

	case tool == "list_directory":
		if output != "" {
			lines := strings.Split(output, "\n")
			if len(lines) > 10 {
				return fmt.Sprintf("Here are the contents of %s:\n%s\n... and %d more items", path, strings.Join(lines[:10], "\n"), len(lines)-10)
			}
			return fmt.Sprintf("Here are the contents of %s:\n%s", path, output)
		}
		return fmt.Sprintf("The directory %s is empty.", path)

	case tool == "read_file":
		if output != "" {
			if len(output) > 500 {
				return fmt.Sprintf("Here's the content of %s:\n%s\n... (truncated)", path, output[:500])
			}
			return fmt.Sprintf("Here's the content of %s:\n%s", path, output)
		}
		return fmt.Sprintf("The file %s is empty.", path)

	case tool == "web_search":
		if output != "" {
			return output
		}
		return "I searched but couldn't find relevant information."

	case tool == "run_command":
		if output != "" {
			return fmt.Sprintf("Done! Here's the output:\n%s", output)
		}
		return "Done! The command completed successfully."

	default:
		if output != "" {
			return output
		}
		return "Done!"
	}
}

// summarizeMemory creates a brief summary of memory context relevant to a question.
func (h *PinkyCompatHandler) summarizeMemory(memoryContext, question string) string {
	questionLower := strings.ToLower(question)

	// Check for color-related questions
	if strings.Contains(questionLower, "color") {
		if strings.Contains(strings.ToLower(memoryContext), "blue") {
			return "I remember that you like the color blue."
		}
	}

	// Check for pet-related questions
	if strings.Contains(questionLower, "dog") || strings.Contains(questionLower, "pet") {
		if strings.Contains(memoryContext, "Zero") || strings.Contains(memoryContext, "Sakura") {
			return "I know you have dogs named Zero and Sakura."
		}
	}

	// Generic fallback
	return "I have some information about you in my memory."
}

// ═══════════════════════════════════════════════════════════════════════════════
// APPLICATION GENERATOR - Multi-step agentic application creation
// ═══════════════════════════════════════════════════════════════════════════════

// AppGenerationRequest represents a request to generate an application from a PRD.
type AppGenerationRequest struct {
	Name      string   `json:"name"`
	PRD       string   `json:"prd"`
	TechHints []string `json:"tech_hints,omitempty"`
	BasePath  string   `json:"base_path,omitempty"`
}

// AppTask represents a single task in the application generation process.
type AppTask struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tool        string            `json:"tool"`
	Params      map[string]string `json:"params"`
	Status      string            `json:"status"` // pending, in_progress, completed, failed, needs_clarification
	Output      string            `json:"output,omitempty"`
	Error       string            `json:"error,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
}

// AppGenerationState tracks the state of an application generation process.
type AppGenerationState struct {
	Request      AppGenerationRequest `json:"request"`
	Tasks        []AppTask            `json:"tasks"`
	CurrentTask  int                  `json:"current_task"`
	Status       string               `json:"status"` // planning, executing, needs_clarification, completed, failed
	Question     string               `json:"question,omitempty"`
	Options      []string             `json:"options,omitempty"`
	FilesCreated []string             `json:"files_created,omitempty"`
	Summary      string               `json:"summary,omitempty"`
}

// ParsedPRD represents structured information extracted from a PRD.
type ParsedPRD struct {
	AppName     string   `json:"app_name"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Entities    []string `json:"entities"`
	TechStack   []string `json:"tech_stack"`
	Constraints []string `json:"constraints"`
}

// isAppGenerationRequest checks if the input is requesting application generation.
func (h *PinkyCompatHandler) isAppGenerationRequest(input string) bool {
	inputLower := strings.ToLower(input)

	// Must have creation intent
	hasCreationIntent := strings.Contains(inputLower, "create") ||
		strings.Contains(inputLower, "build") ||
		strings.Contains(inputLower, "generate") ||
		strings.Contains(inputLower, "make")

	// Must mention application/app/project
	hasAppMention := strings.Contains(inputLower, "application") ||
		strings.Contains(inputLower, "app ") ||
		strings.Contains(inputLower, "project")

	// Must have PRD or detailed requirements
	hasPRD := strings.Contains(inputLower, "prd") ||
		strings.Contains(inputLower, "requirements") ||
		strings.Contains(inputLower, "spec") ||
		strings.Contains(inputLower, "features:") ||
		strings.Contains(inputLower, "## ") || // Markdown headers indicate structured doc
		len(input) > 500 // Long input likely contains requirements

	return hasCreationIntent && hasAppMention && hasPRD
}

// extractAppName extracts the application name from the request.
func (h *PinkyCompatHandler) extractAppName(input string) string {
	inputLower := strings.ToLower(input)

	// Look for patterns like "called X", "named X", "'X'"
	patterns := []struct {
		prefix string
		suffix string
	}{
		{"called ", " "},
		{"named ", " "},
		{"'", "'"},
		{"\"", "\""},
		{"application ", " with"},
		{"app ", " with"},
		{"project ", " with"},
	}

	for _, p := range patterns {
		if idx := strings.Index(inputLower, p.prefix); idx >= 0 {
			start := idx + len(p.prefix)
			remaining := input[start:]
			if endIdx := strings.Index(remaining, p.suffix); endIdx > 0 {
				name := strings.TrimSpace(remaining[:endIdx])
				if name != "" && len(name) < 50 {
					return name
				}
			}
		}
	}

	return "MyApp" // Default name
}

// extractPRD extracts the PRD content from the input.
func (h *PinkyCompatHandler) extractPRD(input string) string {
	inputLower := strings.ToLower(input)

	// Look for PRD markers
	markers := []string{"prd:", "requirements:", "spec:", "features:", "## overview", "## features"}
	for _, marker := range markers {
		if idx := strings.Index(inputLower, marker); idx >= 0 {
			return strings.TrimSpace(input[idx:])
		}
	}

	// If no marker found, look for the first markdown header or use everything after "with"
	if idx := strings.Index(inputLower, "with "); idx >= 0 {
		return strings.TrimSpace(input[idx+5:])
	}

	return input
}

// parsePRD analyzes the PRD and extracts structured information.
func (h *PinkyCompatHandler) parsePRD(ctx context.Context, appName, prdContent string) (*ParsedPRD, error) {
	parsed := &ParsedPRD{
		AppName:   appName,
		Features:  []string{},
		Entities:  []string{},
		TechStack: []string{},
	}

	prdLower := strings.ToLower(prdContent)

	// Extract description (first paragraph or overview section)
	if idx := strings.Index(prdLower, "overview"); idx >= 0 {
		lines := strings.Split(prdContent[idx:], "\n")
		for i, line := range lines {
			if i > 0 && !strings.HasPrefix(strings.TrimSpace(line), "#") && strings.TrimSpace(line) != "" {
				parsed.Description = strings.TrimSpace(line)
				break
			}
		}
	}

	// Extract features (look for bullet points after "features" header)
	if idx := strings.Index(prdLower, "feature"); idx >= 0 {
		lines := strings.Split(prdContent[idx:], "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") {
				feature := strings.TrimLeft(trimmed, "-*• ")
				if feature != "" {
					parsed.Features = append(parsed.Features, feature)
				}
			}
		}
	}

	// Detect tech stack from keywords
	techKeywords := map[string]string{
		"react":      "React",
		"nextjs":     "Next.js",
		"next.js":    "Next.js",
		"typescript": "TypeScript",
		"javascript": "JavaScript",
		"python":     "Python",
		"fastapi":    "FastAPI",
		"django":     "Django",
		"express":    "Express",
		"postgres":   "PostgreSQL",
		"mongodb":    "MongoDB",
		"redis":      "Redis",
		"docker":     "Docker",
		"node":       "Node.js",
		"go":         "Go",
		"golang":     "Go",
		"rust":       "Rust",
	}

	for keyword, tech := range techKeywords {
		if strings.Contains(prdLower, keyword) {
			parsed.TechStack = append(parsed.TechStack, tech)
		}
	}

	// Default tech stack if none detected
	if len(parsed.TechStack) == 0 {
		parsed.TechStack = []string{"TypeScript", "Node.js"}
	}

	// Extract entities (look for nouns that might be data models)
	entityKeywords := []string{"user", "task", "item", "product", "order", "message", "post", "comment"}
	for _, keyword := range entityKeywords {
		if strings.Contains(prdLower, keyword) {
			parsed.Entities = append(parsed.Entities, strings.Title(keyword))
		}
	}

	return parsed, nil
}

// generateAppTasks creates a task list from the parsed PRD.
func (h *PinkyCompatHandler) generateAppTasks(parsed *ParsedPRD, basePath string) []AppTask {
	if basePath == "" {
		basePath = "/tmp"
	}
	projectPath := filepath.Join(basePath, parsed.AppName)

	tasks := []AppTask{
		{
			ID:          "1-setup",
			Name:        "Create project structure",
			Description: "Initialize the project directory and basic structure",
			Tool:        "run_command",
			Params:      map[string]string{"command": fmt.Sprintf("mkdir -p %s/{src,tests,docs}", projectPath)},
			Status:      "pending",
		},
		{
			ID:          "2-readme",
			Name:        "Create README",
			Description: "Generate project README with description and setup instructions",
			Tool:        "write_file",
			Params: map[string]string{
				"path":    filepath.Join(projectPath, "README.md"),
				"content": h.generateReadme(parsed),
			},
			Status:    "pending",
			DependsOn: []string{"1-setup"},
		},
	}

	// Add package.json for TypeScript/JavaScript projects
	if containsTech(parsed.TechStack, "TypeScript", "JavaScript", "Node.js", "React", "Next.js") {
		tasks = append(tasks, AppTask{
			ID:          "3-package",
			Name:        "Create package.json",
			Description: "Initialize Node.js package with dependencies",
			Tool:        "write_file",
			Params: map[string]string{
				"path":    filepath.Join(projectPath, "package.json"),
				"content": h.generatePackageJSON(parsed),
			},
			Status:    "pending",
			DependsOn: []string{"1-setup"},
		})
	}

	// Add main entry file
	taskID := len(tasks) + 1
	mainFile := h.generateMainFile(parsed, projectPath)
	tasks = append(tasks, AppTask{
		ID:          fmt.Sprintf("%d-main", taskID),
		Name:        "Create main entry point",
		Description: "Generate the main application entry point",
		Tool:        "write_file",
		Params: map[string]string{
			"path":    mainFile.Path,
			"content": mainFile.Content,
		},
		Status:    "pending",
		DependsOn: []string{"1-setup"},
	})

	// Add entity/model files if detected
	for i, entity := range parsed.Entities {
		taskID++
		tasks = append(tasks, AppTask{
			ID:          fmt.Sprintf("%d-model-%s", taskID, strings.ToLower(entity)),
			Name:        fmt.Sprintf("Create %s model", entity),
			Description: fmt.Sprintf("Generate the %s data model", entity),
			Tool:        "write_file",
			Params: map[string]string{
				"path":    filepath.Join(projectPath, "src", "models", strings.ToLower(entity)+".ts"),
				"content": h.generateModel(entity, parsed),
			},
			Status:    "pending",
			DependsOn: []string{"1-setup"},
		})
		if i >= 4 {
			break // Limit to 5 entities
		}
	}

	// Add IDE opening task at the end
	tasks = append(tasks, AppTask{
		ID:          fmt.Sprintf("%d-ide", len(tasks)+1),
		Name:        "Open in IDE",
		Description: "Open the project in VS Code",
		Tool:        "run_command",
		Params:      map[string]string{"command": "code " + projectPath},
		Status:      "pending",
		DependsOn:   []string{fmt.Sprintf("%d-main", taskID)},
	})

	return tasks
}

// FileContent holds path and content for a generated file.
type FileContent struct {
	Path    string
	Content string
}

// generateMainFile creates the main entry point based on tech stack.
func (h *PinkyCompatHandler) generateMainFile(parsed *ParsedPRD, projectPath string) FileContent {
	if containsTech(parsed.TechStack, "TypeScript", "Node.js") {
		return FileContent{
			Path: filepath.Join(projectPath, "src", "index.ts"),
			Content: fmt.Sprintf(`// %s - Main Entry Point
// Generated by Cortex Brain Application Generator

console.log("Starting %s...");

// TODO: Initialize application

export function main() {
  console.log("%s is running!");
}

main();
`, parsed.AppName, parsed.AppName, parsed.AppName),
		}
	}

	if containsTech(parsed.TechStack, "Python") {
		return FileContent{
			Path: filepath.Join(projectPath, "src", "main.py"),
			Content: fmt.Sprintf(`#!/usr/bin/env python3
"""
%s - Main Entry Point
Generated by Cortex Brain Application Generator
"""

def main():
    print("Starting %s...")
    # TODO: Initialize application
    print("%s is running!")

if __name__ == "__main__":
    main()
`, parsed.AppName, parsed.AppName, parsed.AppName),
		}
	}

	// Default to TypeScript
	return FileContent{
		Path: filepath.Join(projectPath, "src", "index.ts"),
		Content: fmt.Sprintf(`// %s
console.log("Hello from %s!");
`, parsed.AppName, parsed.AppName),
	}
}

// generateReadme creates a README.md for the project.
func (h *PinkyCompatHandler) generateReadme(parsed *ParsedPRD) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", parsed.AppName))

	if parsed.Description != "" {
		sb.WriteString(parsed.Description + "\n\n")
	}

	sb.WriteString("## Features\n\n")
	for _, feature := range parsed.Features {
		sb.WriteString(fmt.Sprintf("- %s\n", feature))
	}
	if len(parsed.Features) == 0 {
		sb.WriteString("- Core functionality (coming soon)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Tech Stack\n\n")
	for _, tech := range parsed.TechStack {
		sb.WriteString(fmt.Sprintf("- %s\n", tech))
	}
	sb.WriteString("\n")

	sb.WriteString("## Getting Started\n\n")
	sb.WriteString("```bash\n")
	if containsTech(parsed.TechStack, "Node.js", "TypeScript", "JavaScript") {
		sb.WriteString("npm install\nnpm run dev\n")
	} else if containsTech(parsed.TechStack, "Python") {
		sb.WriteString("pip install -r requirements.txt\npython src/main.py\n")
	} else {
		sb.WriteString("# TODO: Add setup instructions\n")
	}
	sb.WriteString("```\n\n")

	sb.WriteString("---\n*Generated by Cortex Brain Application Generator*\n")

	return sb.String()
}

// generatePackageJSON creates a package.json for Node.js projects.
func (h *PinkyCompatHandler) generatePackageJSON(parsed *ParsedPRD) string {
	name := strings.ToLower(strings.ReplaceAll(parsed.AppName, " ", "-"))
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "0.1.0",
  "description": "%s",
  "main": "src/index.ts",
  "scripts": {
    "dev": "ts-node src/index.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "test": "jest"
  },
  "keywords": [],
  "author": "",
  "license": "MIT",
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0",
    "ts-node": "^10.9.0"
  }
}
`, name, parsed.Description)
}

// generateModel creates a TypeScript model file.
func (h *PinkyCompatHandler) generateModel(entity string, parsed *ParsedPRD) string {
	return fmt.Sprintf(`// %s Model
// Generated by Cortex Brain Application Generator

export interface %s {
  id: string;
  createdAt: Date;
  updatedAt: Date;
  // TODO: Add %s-specific fields
}

export function create%s(data: Partial<%s>): %s {
  return {
    id: crypto.randomUUID(),
    createdAt: new Date(),
    updatedAt: new Date(),
    ...data,
  } as %s;
}
`, entity, entity, entity, entity, entity, entity, entity)
}

// executeAppGeneration runs the agentic loop for application generation.
func (h *PinkyCompatHandler) executeAppGeneration(ctx context.Context, state *AppGenerationState) (*AppGenerationState, error) {
	h.log.Info("[AppGen] Starting execution with %d tasks", len(state.Tasks))

	for i := range state.Tasks {
		task := &state.Tasks[i]

		// Skip completed tasks
		if task.Status == "completed" {
			continue
		}

		// Check dependencies
		depsComplete := true
		for _, depID := range task.DependsOn {
			for _, t := range state.Tasks {
				if t.ID == depID && t.Status != "completed" {
					depsComplete = false
					break
				}
			}
		}
		if !depsComplete {
			continue
		}

		// Execute the task
		state.CurrentTask = i
		task.Status = "in_progress"
		h.log.Info("[AppGen] Executing task %s: %s", task.ID, task.Name)

		result := h.executor.Execute(ctx, &internalagent.ToolCall{
			Name:   task.Tool,
			Params: task.Params,
		})

		if result.Success {
			task.Status = "completed"
			task.Output = result.Output
			h.log.Info("[AppGen] Task %s completed successfully", task.ID)

			// Track created files
			if task.Tool == "write_file" {
				if path, ok := task.Params["path"]; ok {
					state.FilesCreated = append(state.FilesCreated, path)
				}
			}
		} else {
			task.Status = "failed"
			task.Error = result.Error
			h.log.Error("[AppGen] Task %s failed: %s", task.ID, result.Error)

			// Don't fail the whole process for non-critical tasks
			if strings.Contains(task.ID, "setup") {
				state.Status = "failed"
				return state, fmt.Errorf("critical task %s failed: %s", task.Name, result.Error)
			}
		}
	}

	// Check if all tasks are complete
	allComplete := true
	for _, task := range state.Tasks {
		if task.Status != "completed" && task.Status != "failed" {
			allComplete = false
			break
		}
	}

	if allComplete {
		state.Status = "completed"
		state.Summary = fmt.Sprintf("Created %s with %d files:\n", state.Request.Name, len(state.FilesCreated))
		for _, file := range state.FilesCreated {
			state.Summary += fmt.Sprintf("  - %s\n", file)
		}
	}

	return state, nil
}

// handleAppGeneration is the main entry point for application generation requests.
func (h *PinkyCompatHandler) handleAppGeneration(ctx context.Context, input string) (string, []PinkyToolCall) {
	h.log.Info("[AppGen] Processing application generation request")

	// Extract name and PRD
	appName := h.extractAppName(input)
	prdContent := h.extractPRD(input)

	h.log.Info("[AppGen] App name: %s, PRD length: %d", appName, len(prdContent))

	// Parse the PRD
	parsed, err := h.parsePRD(ctx, appName, prdContent)
	if err != nil {
		return fmt.Sprintf("Error parsing PRD: %v", err), nil
	}

	h.log.Info("[AppGen] Parsed PRD - Features: %d, Entities: %d, Tech: %v",
		len(parsed.Features), len(parsed.Entities), parsed.TechStack)

	// Generate tasks
	tasks := h.generateAppTasks(parsed, "")

	// Create initial state
	state := &AppGenerationState{
		Request: AppGenerationRequest{
			Name: appName,
			PRD:  prdContent,
		},
		Tasks:   tasks,
		Status:  "executing",
	}

	// Execute the tasks
	state, err = h.executeAppGeneration(ctx, state)
	if err != nil {
		return fmt.Sprintf("Error during generation: %v", err), nil
	}

	// Build response
	var toolCalls []PinkyToolCall
	for _, task := range state.Tasks {
		if task.Status == "completed" {
			toolCalls = append(toolCalls, PinkyToolCall{
				ID:    task.ID,
				Tool:  task.Tool,
				Input: stringMapToAnyMap(task.Params),
			})
		}
	}

	return state.Summary, toolCalls
}

// containsTech checks if any of the specified technologies are in the tech stack.
func containsTech(stack []string, techs ...string) bool {
	for _, s := range stack {
		for _, t := range techs {
			if strings.EqualFold(s, t) {
				return true
			}
		}
	}
	return false
}
