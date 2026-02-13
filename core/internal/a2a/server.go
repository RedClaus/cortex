// Package a2a provides a fully A2A-compliant server that exposes the Brain Executive.
//
// This server implements the A2A Protocol v0.3.0 using the official a2a-go SDK,
// allowing any A2A client to communicate with Cortex.
//
// Supported Features:
//   - Agent Card discovery (/.well-known/agent-card.json)
//   - JSON-RPC 2.0 transport (HTTP POST)
//   - Streaming via SSE
//   - Full task lifecycle management
//   - Text and Data parts
//   - Artifacts for lobe outputs
package a2a

import (
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"github.com/a2aproject/a2a-go/a2asrv/eventqueue"
	"github.com/normanking/cortex/internal/facets"
	"github.com/normanking/cortex/internal/logging"
	"github.com/normanking/cortex/internal/memory"
	"github.com/normanking/cortex/internal/server"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/normanking/cortex/pkg/voice"
)

// voiceOptimizationGuidelines adds voice-specific formatting to rich persona prompts.
// These guidelines ensure natural spoken responses while preserving full personality.
const voiceOptimizationGuidelines = `

## Voice Output Guidelines (IMPORTANT)
This is a voice conversation. Your responses will be spoken aloud via text-to-speech.

Response format for voice:
- Keep responses to 1-3 sentences unless explaining something complex
- Use spoken formats: "three fifteen PM" not "15:15", "about two thousand" not "2,048"
- Start with brief acknowledgment: "Got it" / "On it" / "Sure" / "Let me think..."
- Don't read out file paths, URLs, or code syntax unless specifically asked
- Don't use markdown formatting (no asterisks, backticks, or headers)
- Don't use emojis unless they're part of your established personality
- End naturally without forcing a question every time

What NOT to do in voice responses:
- Don't narrate your thinking process extensively (keep inner thoughts internal)
- Don't repeat the user's question back to them
- Don't use text-only formatting like bullet points or code blocks
- Don't spell out technical terms character by character
`

func init() {
	// Register types with gob for A2A task state serialization
	// These are needed because artifact data contains nested map/slice types
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	gob.Register([]map[string]interface{}{})
	gob.Register(map[string]any{})
	gob.Register([]any{})
	gob.Register([]map[string]any{})
}

// ═══════════════════════════════════════════════════════════════════════════════
// BRAIN EXECUTOR (implements a2asrv.AgentExecutor)
// ═══════════════════════════════════════════════════════════════════════════════

// ChatMessage represents a single message in a conversation
type ChatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// LLMChatProvider interface for multi-turn chat
// BRAIN AUDIT FIX: Updated to support proper conversation history
// instead of injecting history as text into a single user message.
type LLMChatProvider interface {
	Chat(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error)
}

// BrainExecutor adapts the Brain Executive to the A2A AgentExecutor interface.
type BrainExecutor struct {
	brain          *brain.Executive
	log            *logging.Logger
	lessonStore    *LessonStore
	strategicStore *memory.StrategicMemoryStore // For strategic memory/principles
	chatLLM        LLMChatProvider              // For simple conversational responses
}

// NewBrainExecutor creates a new BrainExecutor
func NewBrainExecutor(brainExec *brain.Executive, lessonStore *LessonStore, strategicStore *memory.StrategicMemoryStore) *BrainExecutor {
	return &BrainExecutor{
		brain:          brainExec,
		log:            logging.Global(),
		lessonStore:    lessonStore,
		strategicStore: strategicStore,
	}
}

// SetChatLLM sets the LLM provider for simple conversational responses
func (e *BrainExecutor) SetChatLLM(llm LLMChatProvider) {
	e.chatLLM = llm
}

// isSimpleConversation checks if the input is a simple conversational message
// that doesn't need complex Brain processing.
// DESIGN: Default to simple chat (fast, conversational), only use Brain for explicit complex tasks.
// Simple chat still has memory context via buildConversationMessages().
func isSimpleConversation(input string) bool {
	input = strings.ToLower(strings.TrimSpace(input))

	// Tasks that REQUIRE Brain processing - be specific to avoid false positives
	brainRequired := []string{
		// Explicit memory operations
		"remember this", "don't forget", "recall when", "recall what",
		"what do you know about me", "have we discussed", "did we talk about",
		"what's my name", "whats my name", "who am i",
		// Code and technical tasks
		"write code", "write a function", "debug this", "fix this error",
		"implement", "refactor", "create a class",
		// Document/data analysis
		"analyze this", "summarize this", "compare these", "calculate the",
		// File/system operations
		"read file", "create file", "create directory", "execute command",
		"open the file", "save to file",
	}

	for _, indicator := range brainRequired {
		if strings.Contains(input, indicator) {
			return false
		}
	}

	// Short conversational messages use simple chat (fast, natural responses)
	// This handles: "What's the weather?", "Tell me a joke", "How's it going?", etc.
	if len(input) < 150 {
		return true
	}

	// Longer messages may need Brain for thorough processing
	return false
}

// Execute implements a2asrv.AgentExecutor.
// It processes a message through the Brain Executive and writes events to the queue.
func (e *BrainExecutor) Execute(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	e.log.Info("[A2A] Execute: received request taskID=%s", reqCtx.TaskID)

	// Send working state
	workingEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateWorking, nil)
	if err := queue.Write(ctx, workingEvent); err != nil {
		return fmt.Errorf("failed to write state working: %w", err)
	}

	// Extract text from message
	input := extractTextFromMessage(reqCtx.Message)
	e.log.Debug("[A2A] Execute: processing input length=%d", len(input))

	// Check if this is a simple conversational message that can bypass Brain
	// BRAIN AUDIT FIX: Memory context is now built as proper message turns inside executeSimpleChat
	if e.chatLLM != nil && isSimpleConversation(input) {
		e.log.Debug("[A2A] Execute: using simple chat mode for conversational input")
		return e.executeSimpleChat(ctx, reqCtx, queue, input)
	}

	// For complex queries, we still inject memory context as text for Brain processing
	// (This will be upgraded in a future iteration)
	originalInput := input
	input = e.injectMemoryContextAsText(ctx, reqCtx, input)

	// Process through Brain Executive
	result, err := e.brain.Process(ctx, input)
	if err != nil {
		e.log.Error("[A2A] Execute: brain processing failed: %v", err)
		errorMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.TextPart{Text: fmt.Sprintf("Error: %v", err)})
		failEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateFailed, errorMsg)
		failEvent.Final = true
		return queue.Write(ctx, failEvent)
	}

	// Write artifacts for lobe outputs (streaming style)
	if err := e.writeArtifacts(ctx, reqCtx, queue, result); err != nil {
		e.log.Warn("[A2A] Execute: failed to write some artifacts: %v", err)
	}

	// Create response message with text and metadata
	responseText := contentToString(result.FinalContent)
	responseParts := []a2a.Part{a2a.TextPart{Text: responseText}}

	// Add metadata as data part
	metadata := buildMetadata(result)
	if len(metadata) > 0 {
		responseParts = append(responseParts, a2a.DataPart{Data: metadata})
	}

	responseMsg := a2a.NewMessage(a2a.MessageRoleAgent, responseParts...)

	// Save conversation to lessons for future memory context
	var userID, personaID string
	if reqCtx.Message.Metadata != nil {
		if uid, ok := reqCtx.Message.Metadata["userId"].(string); ok {
			userID = uid
		}
		if pid, ok := reqCtx.Message.Metadata["personaId"].(string); ok {
			personaID = pid
		}
	}
	e.saveConversation(ctx, userID, personaID, originalInput, responseText)

	// Complete the task
	completeEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateCompleted, responseMsg)
	completeEvent.Final = true
	if err := queue.Write(ctx, completeEvent); err != nil {
		return fmt.Errorf("failed to write state completed: %w", err)
	}

	e.log.Info("[A2A] Execute: completed taskID=%s totalTime=%v", reqCtx.TaskID, result.TotalTime)
	return nil
}

// Cancel implements a2asrv.AgentExecutor.
// It cancels a running task.
func (e *BrainExecutor) Cancel(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	e.log.Info("[A2A] Cancel: taskID=%s", reqCtx.TaskID)

	cancelEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateCanceled, nil)
	cancelEvent.Final = true
	return queue.Write(ctx, cancelEvent)
}

// executeSimpleChat handles simple conversational messages without Brain processing
// BRAIN AUDIT FIX: Uses proper multi-turn conversation history instead of text injection
func (e *BrainExecutor) executeSimpleChat(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue, originalInput string) error {
	startTime := time.Now()

	// Get persona and user from message metadata
	personaID := "hannah" // Default to Hannah
	var userID string
	if reqCtx.Message.Metadata != nil {
		if pid, ok := reqCtx.Message.Metadata["personaId"].(string); ok && pid != "" {
			personaID = pid
		}
		if uid, ok := reqCtx.Message.Metadata["userId"].(string); ok {
			userID = uid
		}
	}

	// Get the appropriate persona system prompt from built-in personas
	// These contain rich personality traits, behavioral responses, and communication style
	var systemPrompt string
	persona := facets.GetBuiltInPersona(strings.ToLower(personaID))
	if persona != nil && persona.SystemPrompt != "" {
		// Use the rich built-in persona prompt
		systemPrompt = persona.SystemPrompt
		e.log.Debug("[A2A] Using rich built-in persona for simple chat: %s", persona.Name)

		// Append voice-optimization guidelines for natural speech
		systemPrompt += voiceOptimizationGuidelines
	} else {
		// Fallback to minimal voice prompts if persona not found
		switch strings.ToLower(personaID) {
		case "henry":
			systemPrompt = voice.HenryVoiceSystemPrompt
			e.log.Debug("[A2A] Using fallback Henry voice prompt for simple chat")
		default:
			systemPrompt = voice.HannahVoiceSystemPrompt
			e.log.Debug("[A2A] Using fallback Hannah voice prompt for simple chat (unknown: %s)", personaID)
		}
	}

	// Build proper multi-turn conversation history
	// BRAIN AUDIT FIX: This replaces text injection with structured message turns
	messages := e.buildConversationMessages(ctx, reqCtx, originalInput)
	e.log.Debug("[A2A] Built %d conversation messages for LLM", len(messages))

	// Use persona-specific prompt with proper conversation history
	response, err := e.chatLLM.Chat(ctx, systemPrompt, messages)
	if err != nil {
		e.log.Error("[A2A] Simple chat failed: %v", err)
		errorMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.TextPart{Text: fmt.Sprintf("Error: %v", err)})
		failEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateFailed, errorMsg)
		failEvent.Final = true
		return queue.Write(ctx, failEvent)
	}

	// Save conversation to lessons for future memory context
	e.saveConversation(ctx, userID, personaID, originalInput, response)

	// Create response message
	responseMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.TextPart{Text: response})

	// Complete the task
	completeEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateCompleted, responseMsg)
	completeEvent.Final = true
	if err := queue.Write(ctx, completeEvent); err != nil {
		return fmt.Errorf("failed to write state completed: %w", err)
	}

	e.log.Info("[A2A] Simple chat completed taskID=%s persona=%s messages=%d totalTime=%v", reqCtx.TaskID, personaID, len(messages), time.Since(startTime))
	return nil
}

// saveConversation saves user/assistant messages to lessons for future memory context
func (e *BrainExecutor) saveConversation(ctx context.Context, userID, personaID, userMessage, assistantResponse string) {
	if e.lessonStore == nil {
		return
	}
	if userID == "" || personaID == "" {
		e.log.Debug("[A2A] Cannot save conversation: missing userID or personaID")
		return
	}

	// Use a separate context with longer timeout for saving
	saveCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get or create an active lesson for this user/persona
	lessons, err := e.lessonStore.List(saveCtx, userID, personaID, 1)
	var lessonID string

	if err != nil {
		e.log.Warn("[A2A] Failed to list lessons: %v", err)
		return
	}

	if len(lessons) > 0 && lessons[0].Status == "active" {
		// Use existing active lesson
		lessonID = lessons[0].ID
	} else {
		// Create new lesson
		title := generateLessonTitle(userMessage)
		lesson, err := e.lessonStore.Create(saveCtx, userID, personaID, title)
		if err != nil {
			e.log.Warn("[A2A] Failed to create lesson: %v", err)
			return
		}
		lessonID = lesson.ID
		e.log.Debug("[A2A] Created new lesson %s for user %s", lessonID, userID)
	}

	// Save user message
	if _, err := e.lessonStore.AddMessage(saveCtx, lessonID, "user", userMessage); err != nil {
		e.log.Warn("[A2A] Failed to save user message: %v", err)
		return
	}

	// Save assistant response
	if _, err := e.lessonStore.AddMessage(saveCtx, lessonID, "assistant", assistantResponse); err != nil {
		e.log.Warn("[A2A] Failed to save assistant message: %v", err)
		return
	}

	e.log.Debug("[A2A] Saved conversation to lesson %s", lessonID)
}

// generateLessonTitle creates a title from the first user message
func generateLessonTitle(userMessage string) string {
	// Truncate and clean up for title
	title := userMessage
	if len(title) > 50 {
		title = title[:47] + "..."
	}
	// Remove newlines
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Conversation " + time.Now().Format("Jan 2, 3:04 PM")
	}
	return title
}

// injectMemoryContextAsText adds memory context as text for Brain processing
// Now includes both conversation history AND strategic principles
func (e *BrainExecutor) injectMemoryContextAsText(ctx context.Context, reqCtx *a2asrv.RequestContext, input string) string {
	var contextParts []string

	// 1. Get strategic principles relevant to this query
	e.log.Info("[A2A] Searching for relevant principles for query: %s", input[:min(50, len(input))])
	principles := e.getRelevantPrinciples(ctx, input, 5) // Get top 5 relevant principles
	if len(principles) > 0 {
		e.log.Info("[A2A] Found %d principles to inject", len(principles))
		principleContext := e.formatPrinciplesForContext(principles)
		if principleContext != "" {
			contextParts = append(contextParts, principleContext)
			e.log.Debug("[A2A] Principle context: %s", principleContext[:min(200, len(principleContext))])
		}
	} else {
		e.log.Info("[A2A] No relevant principles found")
	}

	// 2. Get recent conversation context
	if e.lessonStore != nil {
		var userID, personaID string
		if reqCtx.Message.Metadata != nil {
			if uid, ok := reqCtx.Message.Metadata["userId"].(string); ok {
				userID = uid
			}
			if pid, ok := reqCtx.Message.Metadata["personaId"].(string); ok {
				personaID = pid
			}
		}

		if userID != "" && personaID != "" {
			recentContext, err := e.lessonStore.GetRecentContext(ctx, userID, personaID, 10)
			if err != nil {
				e.log.Warn("[A2A] Failed to get recent context: %v", err)
			} else if recentContext != "" {
				contextParts = append(contextParts, "# Recent Conversation\n\n"+recentContext)
			}
		}
	}

	// Build final context
	if len(contextParts) > 0 {
		fullContext := strings.Join(contextParts, "\n---\n\n")
		return fullContext + "\n---\n\n# Current Question\n\n" + input
	}

	return input
}

// getRelevantPrinciples retrieves strategic memory principles relevant to the query
func (e *BrainExecutor) getRelevantPrinciples(ctx context.Context, query string, limit int) []memory.StrategicMemory {
	if e.strategicStore == nil {
		e.log.Warn("[A2A] strategicStore is nil, cannot retrieve principles")
		return nil
	}

	e.log.Debug("[A2A] Running FTS search for: %s", query)

	// Use FTS search to find relevant principles
	principles, err := e.strategicStore.SearchFTS(ctx, query, limit)
	if err != nil {
		e.log.Debug("[A2A] FTS search for principles failed: %v, trying category match", err)
		// Fallback: try to get top principles by confidence
		principles, err = e.strategicStore.GetTopPrinciples(ctx, limit)
		if err != nil {
			e.log.Warn("[A2A] Failed to get principles: %v", err)
			return nil
		}
	}

	if len(principles) > 0 {
		e.log.Info("[A2A] Retrieved %d relevant principles for query", len(principles))
	}

	return principles
}

// formatPrinciplesForContext formats strategic memories as context for the LLM
func (e *BrainExecutor) formatPrinciplesForContext(principles []memory.StrategicMemory) string {
	if len(principles) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Strategic Principles\n\n")
	sb.WriteString("Apply these learned principles when relevant:\n\n")

	for i, p := range principles {
		sb.WriteString(fmt.Sprintf("%d. **%s** (confidence: %.0f%%)\n", i+1, p.Category, p.Confidence*100))
		sb.WriteString(fmt.Sprintf("   %s\n\n", p.Principle))
	}

	return sb.String()
}

// buildConversationMessages builds proper message turns from memory context
// BRAIN AUDIT FIX: Returns structured messages instead of text injection
func (e *BrainExecutor) buildConversationMessages(ctx context.Context, reqCtx *a2asrv.RequestContext, currentInput string) []ChatMessage {
	messages := []ChatMessage{}

	if e.lessonStore == nil {
		// No memory, just return current message
		return append(messages, ChatMessage{Role: "user", Content: currentInput})
	}

	// Extract user and persona info from request metadata
	var userID, personaID string
	if reqCtx.Message.Metadata != nil {
		if uid, ok := reqCtx.Message.Metadata["userId"].(string); ok {
			userID = uid
		}
		if pid, ok := reqCtx.Message.Metadata["personaId"].(string); ok {
			personaID = pid
		}
	}

	if userID == "" || personaID == "" {
		e.log.Debug("[A2A] No user/persona metadata for context injection")
		return append(messages, ChatMessage{Role: "user", Content: currentInput})
	}

	e.log.Debug("[A2A] Building conversation messages for user=%s persona=%s", userID, personaID)

	// Get recent conversation history as structured messages
	recentMessages, err := e.lessonStore.GetRecentMessages(ctx, userID, personaID, 10)
	if err != nil {
		e.log.Warn("[A2A] Failed to get recent messages: %v", err)
	} else if len(recentMessages) > 0 {
		// Convert lesson messages to ChatMessages
		for _, msg := range recentMessages {
			messages = append(messages, ChatMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
		e.log.Info("[A2A] Injected %d message turns for user=%s", len(recentMessages), userID)
	}

	// Add current user message
	messages = append(messages, ChatMessage{Role: "user", Content: currentInput})

	return messages
}

// writeArtifacts creates and writes artifacts for lobe outputs
func (e *BrainExecutor) writeArtifacts(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue, result *brain.ExecutionResult) error {
	// Create artifact for classification result
	if result.Classification != nil {
		classificationData := map[string]any{
			"primaryLobe":    string(result.Classification.PrimaryLobe),
			"secondaryLobes": lobeIDsToStrings(result.Classification.SecondaryLobes),
			"riskLevel":      string(result.Classification.RiskLevel),
			"confidence":     result.Classification.Confidence,
			"method":         result.Classification.Method,
		}

		classEvent := a2a.NewArtifactEvent(reqCtx, a2a.DataPart{Data: classificationData})
		classEvent.Artifact.Name = "classification"
		classEvent.Artifact.Description = "Input classification by Executive Classifier"
		if err := queue.Write(ctx, classEvent); err != nil {
			e.log.Warn("[A2A] Failed to write classification artifact: %v", err)
		}
	}

	// Create artifacts for significant lobe results
	for _, lr := range result.LobeResults {
		if lr == nil {
			continue
		}

		// Only create artifacts for lobes with meaningful content
		content := contentToString(lr.Content)
		if content == "" || len(content) < 10 {
			continue
		}

		parts := []a2a.Part{
			a2a.TextPart{Text: content},
			a2a.DataPart{Data: map[string]any{
				"lobeId":     string(lr.LobeID),
				"confidence": lr.Confidence,
				"durationMs": lr.Meta.Duration.Milliseconds(),
				"tokensUsed": lr.Meta.TokensUsed,
				"modelUsed":  lr.Meta.ModelUsed,
				"cacheHit":   lr.Meta.CacheHit,
			}},
		}

		lobeEvent := a2a.NewArtifactEvent(reqCtx, parts...)
		lobeEvent.Artifact.Name = fmt.Sprintf("lobe-%s", lr.LobeID)
		lobeEvent.Artifact.Description = fmt.Sprintf("Output from %s cognitive lobe", lr.LobeID)
		if err := queue.Write(ctx, lobeEvent); err != nil {
			e.log.Warn("[A2A] Failed to write lobe artifact %s: %v", lr.LobeID, err)
		}
	}

	// Create execution summary artifact
	if len(result.Phases) > 0 {
		phases := make([]map[string]any, len(result.Phases))
		for i, p := range result.Phases {
			lobeIDs := make([]string, 0)
			for _, lr := range p.LobeResults {
				if lr != nil {
					lobeIDs = append(lobeIDs, string(lr.LobeID))
				}
			}
			phases[i] = map[string]any{
				"name":       p.PhaseName,
				"durationMs": p.Duration.Milliseconds(),
				"replanned":  p.Replanned,
				"lobes":      lobeIDs,
			}
		}

		summaryEvent := a2a.NewArtifactEvent(reqCtx, a2a.DataPart{Data: map[string]any{
			"phases":       phases,
			"totalTimeMs":  result.TotalTime.Milliseconds(),
			"replanCount":  result.ReplanCount,
			"strategyName": getStrategyName(result.Strategy),
			"totalLobes":   len(result.LobeResults),
		}})
		summaryEvent.Artifact.Name = "execution-summary"
		summaryEvent.Artifact.Description = "Brain Executive execution summary with phase details"
		if err := queue.Write(ctx, summaryEvent); err != nil {
			e.log.Warn("[A2A] Failed to write summary artifact: %v", err)
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

func extractTextFromMessage(msg *a2a.Message) string {
	if msg == nil {
		return ""
	}
	var text string
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case a2a.TextPart:
			text += p.Text + " "
		case *a2a.TextPart:
			text += p.Text + " "
		}
	}
	return text
}

func contentToString(content interface{}) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", v)
	}
}

func buildMetadata(result *brain.ExecutionResult) map[string]any {
	metadata := make(map[string]any)

	metadata["processingTimeMs"] = result.TotalTime.Milliseconds()
	metadata["replanCount"] = result.ReplanCount

	if result.Classification != nil {
		metadata["classification"] = map[string]any{
			"primaryLobe": string(result.Classification.PrimaryLobe),
			"confidence":  result.Classification.Confidence,
			"method":      result.Classification.Method,
		}
	}

	lobeContributions := make(map[string]any)
	for _, lr := range result.LobeResults {
		if lr != nil {
			lobeContributions[string(lr.LobeID)] = map[string]any{
				"confidence": lr.Confidence,
				"durationMs": lr.Meta.Duration.Milliseconds(),
			}
		}
	}
	metadata["lobeContributions"] = lobeContributions

	return metadata
}

func lobeIDsToStrings(ids []brain.LobeID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(id)
	}
	return result
}

func getStrategyName(strategy *brain.ThinkingStrategy) string {
	if strategy == nil {
		return "unknown"
	}
	return strategy.Name
}

// ═══════════════════════════════════════════════════════════════════════════════
// SERVER
// ═══════════════════════════════════════════════════════════════════════════════

// Server wraps the A2A server infrastructure
type Server struct {
	executor       *BrainExecutor
	handler        a2asrv.RequestHandler
	mux            *http.ServeMux
	server         *http.Server
	log            *logging.Logger
	card           *a2a.AgentCard
	llmProxy       *LLMProxy
	personaStore   *facets.PersonaStore
	lessonStore    *LessonStore
	strategicStore *memory.StrategicMemoryStore
}

// AuthHandlersInterface defines the interface for auth handlers
type AuthHandlersInterface interface {
	RegisterRoutes(mux *http.ServeMux)
	GetUserPersonas(w http.ResponseWriter, r *http.Request)
	AssignPersona(w http.ResponseWriter, r *http.Request)
	UnassignPersona(w http.ResponseWriter, r *http.Request)
	SetDefaultPersona(w http.ResponseWriter, r *http.Request)
}

// ServerConfig configures the A2A server
type ServerConfig struct {
	AgentName        string
	AgentDescription string
	AgentVersion     string
	Port             int
	AuthHandlers     AuthHandlersInterface
	DB               interface{}
	PersonaStore     *facets.PersonaStore
	ChatLLM          LLMChatProvider           // For simple conversational responses
	Embedder         memory.Embedder           // For strategic memory embeddings (optional)
	CoreMemoryStore  *memory.CoreMemoryStore   // For user memory retrieval
}

// coreMemoryAdapter adapts CoreMemoryStore to MemoryStoreInterface.
type coreMemoryAdapter struct {
	store *memory.CoreMemoryStore
}

// GetUserMemory implements MemoryStoreInterface by wrapping CoreMemoryStore.
func (a *coreMemoryAdapter) GetUserMemory(ctx context.Context, userID string) (*UserMemoryData, error) {
	if a.store == nil {
		return nil, nil
	}

	mem, err := a.store.GetUserMemory(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Convert memory.UserMemory to UserMemoryData
	result := &UserMemoryData{
		Name:           mem.Name,
		Role:           mem.Role,
		PrefersConcise: mem.PrefersConcise,
		PrefersVerbose: mem.PrefersVerbose,
	}

	// Extract custom facts
	for _, fact := range mem.CustomFacts {
		result.CustomFacts = append(result.CustomFacts, fact.Fact)
	}

	return result, nil
}

// StoreSkill stores a successful action as a skill for future reference.
// Currently stores skills as custom facts in user memory.
func (a *coreMemoryAdapter) StoreSkill(ctx context.Context, userID, intent, tool string, params map[string]string, success bool) error {
	if a.store == nil {
		return nil
	}

	// Format skill as a fact
	paramStr := ""
	for k, v := range params {
		if paramStr != "" {
			paramStr += ", "
		}
		paramStr += k + "=" + v
	}

	// Truncate intent for readability
	intentShort := intent
	if len(intentShort) > 50 {
		intentShort = intentShort[:50] + "..."
	}

	status := "succeeded"
	if !success {
		status = "failed"
	}

	skillFact := fmt.Sprintf("Skill: '%s' -> %s [%s] (%s)", intentShort, tool, paramStr, status)

	// Store as a custom fact using AppendUserFact
	return a.store.AppendUserFact(ctx, userID, skillFact, "skill_learning")
}

// SearchSkills searches for similar skills the agent has used before.
// Currently searches custom facts for skill patterns.
func (a *coreMemoryAdapter) SearchSkills(ctx context.Context, userID, query string, limit int) ([]SkillMemory, error) {
	if a.store == nil {
		return nil, nil
	}

	// Get user memory and filter for skill facts
	mem, err := a.store.GetUserMemory(ctx, userID)
	if err != nil {
		return nil, err
	}

	var skills []SkillMemory
	queryLower := strings.ToLower(query)

	for _, fact := range mem.CustomFacts {
		if strings.HasPrefix(fact.Fact, "Skill:") {
			// Check if this skill is relevant to the query
			factLower := strings.ToLower(fact.Fact)
			// Simple keyword matching
			keywords := []string{"create", "folder", "file", "open", "read", "write", "delete", "list", "search"}
			matches := false
			for _, kw := range keywords {
				if strings.Contains(factLower, kw) && strings.Contains(queryLower, kw) {
					matches = true
					break
				}
			}
			if matches || strings.Contains(factLower, queryLower) {
				skill := SkillMemory{
					Intent:    fact.Fact,
					Success:   strings.Contains(fact.Fact, "succeeded"),
					Timestamp: time.Now(),
				}
				// Extract tool name
				if idx := strings.Index(fact.Fact, " -> "); idx > 0 {
					rest := fact.Fact[idx+4:]
					if endIdx := strings.Index(rest, " ["); endIdx > 0 {
						skill.Tool = rest[:endIdx]
					}
				}
				skills = append(skills, skill)
				if len(skills) >= limit {
					break
				}
			}
		}
	}

	return skills, nil
}

// NewServer creates a new A2A server using the official SDK
func NewServer(brainExec *brain.Executive, cfg *ServerConfig) *Server {
	if cfg == nil {
		cfg = &ServerConfig{
			AgentName:        "Cortex Brain",
			AgentDescription: "AI-Powered Assistant with Brain Executive (14+ Cognitive Lobes)",
			AgentVersion:     "2.0.0",
			Port:             8080,
		}
	}

	// Initialize LessonStore early so we can pass it to the executor
	var lessonStore *LessonStore
	var strategicStore *memory.StrategicMemoryStore
	if db, ok := cfg.DB.(*sql.DB); ok && db != nil {
		lessonStore = NewLessonStore(db)
		strategicStore = memory.NewStrategicMemoryStore(db, cfg.Embedder)
	}

	executor := NewBrainExecutor(brainExec, lessonStore, strategicStore)

	// Set chat LLM for simple conversational responses
	if cfg.ChatLLM != nil {
		executor.SetChatLLM(cfg.ChatLLM)
	}

	// Create comprehensive agent card
	agentCard := &a2a.AgentCard{
		Name:               cfg.AgentName,
		Description:        cfg.AgentDescription,
		Version:            cfg.AgentVersion,
		ProtocolVersion:    "0.3",
		URL:                fmt.Sprintf("http://localhost:%d/", cfg.Port),
		PreferredTransport: a2a.TransportProtocolJSONRPC,
		Capabilities: a2a.AgentCapabilities{
			Streaming:              true,
			PushNotifications:      true,
			StateTransitionHistory: true,
		},
		DefaultInputModes:  []string{"text", "application/json"},
		DefaultOutputModes: []string{"text", "application/json"},
		Skills: []a2a.AgentSkill{
			{
				ID:          "coding",
				Name:        "Software Development",
				Description: "Write, review, debug, and explain code in any programming language. Supports file operations and shell commands.",
				Tags:        []string{"code", "programming", "development", "debugging", "refactoring"},
				Examples:    []string{"Write a Python function to sort a list", "Debug this Go code", "Explain how this algorithm works"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text", "application/json"},
			},
			{
				ID:          "reasoning",
				Name:        "Complex Reasoning",
				Description: "Analyze complex problems, think through multi-step solutions, and provide detailed explanations with logical justification.",
				Tags:        []string{"analysis", "reasoning", "problem-solving", "logic", "critical-thinking"},
				Examples:    []string{"Analyze the pros and cons of this approach", "Why is this solution better?"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "planning",
				Name:        "Task Planning",
				Description: "Break down complex tasks into actionable steps, create project plans, and manage task dependencies.",
				Tags:        []string{"planning", "organization", "tasks", "project-management"},
				Examples:    []string{"Create a plan to migrate this codebase", "Break down this feature into tasks"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text", "application/json"},
			},
			{
				ID:          "knowledge",
				Name:        "Knowledge Retrieval",
				Description: "Search and retrieve relevant information from knowledge base, memory systems, and learned context.",
				Tags:        []string{"knowledge", "search", "retrieval", "memory", "context"},
				Examples:    []string{"What did we discuss about the API design?", "Find related code patterns"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "creativity",
				Name:        "Creative Generation",
				Description: "Generate creative content, brainstorm ideas, and propose innovative solutions to problems.",
				Tags:        []string{"creativity", "generation", "ideas", "brainstorming", "innovation"},
				Examples:    []string{"Brainstorm names for this project", "Suggest alternative approaches"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
			{
				ID:          "tools",
				Name:        "Tool Execution",
				Description: "Execute shell commands, perform file operations, interact with system tools, and run automated workflows.",
				Tags:        []string{"tools", "shell", "files", "system", "automation"},
				Examples:    []string{"Run the test suite", "List files in this directory", "Create a new file"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text", "application/json"},
			},
			{
				ID:          "safety",
				Name:        "Safety Assessment",
				Description: "Evaluate requests for potential risks, harmful content, and ensure responsible AI behavior.",
				Tags:        []string{"safety", "security", "risk-assessment", "ethics"},
				InputModes:  []string{"text"},
				OutputModes: []string{"application/json"},
			},
			{
				ID:          "metacognition",
				Name:        "Self-Reflection",
				Description: "Monitor and evaluate own reasoning processes, identify limitations, and improve response quality.",
				Tags:        []string{"metacognition", "self-reflection", "quality", "improvement"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text"},
			},
		},
	}

	// Create handler with the SDK
	handler := a2asrv.NewHandler(executor)

	// Set up HTTP routing
	mux := http.NewServeMux()
	mux.Handle("/", a2asrv.NewJSONRPCHandler(handler))
	mux.Handle(a2asrv.WellKnownAgentCardPath, a2asrv.NewStaticAgentCardHandler(agentCard))
	// Also handle the alternate path
	mux.Handle("/.well-known/agent.json", a2asrv.NewStaticAgentCardHandler(agentCard))

	// Create and register LLM proxy for external agents
	llmProxy := NewLLMProxy()
	llmProxy.InitializeProviders()
	llmProxy.SetLessonStore(lessonStore) // Enable memory context injection
	llmProxy.RegisterRoutes(mux)

	// Register API key config routes
	llmProxy.GetKeyManager().RegisterConfigRoutes(mux)

	// Register auth routes if provided
	if cfg.AuthHandlers != nil {
		cfg.AuthHandlers.RegisterRoutes(mux)
		// Register user-persona routes
		mux.HandleFunc("GET /api/v1/users/{userId}/personas", cfg.AuthHandlers.GetUserPersonas)
		mux.HandleFunc("POST /api/v1/users/{userId}/personas/{personaId}", cfg.AuthHandlers.AssignPersona)
		mux.HandleFunc("DELETE /api/v1/users/{userId}/personas/{personaId}", cfg.AuthHandlers.UnassignPersona)
		mux.HandleFunc("PUT /api/v1/users/{userId}/personas/{personaId}/default", cfg.AuthHandlers.SetDefaultPersona)
	}

	// Initialize PersonaStore if DB provided
	var personaStore *facets.PersonaStore
	if cfg.PersonaStore != nil {
		personaStore = cfg.PersonaStore
	}

	srv := &Server{
		executor:       executor,
		handler:        handler,
		mux:            mux,
		log:            logging.Global(),
		card:           agentCard,
		llmProxy:       llmProxy,
		personaStore:   personaStore,
		lessonStore:    lessonStore,
		strategicStore: strategicStore,
	}

	// Register persona routes
	if personaStore != nil {
		mux.HandleFunc("GET /api/v1/personas", srv.handleListPersonas)
		mux.HandleFunc("GET /api/v1/personas/{id}", srv.handleGetPersona)
	}

	// Register lesson routes
	if lessonStore != nil {
		mux.HandleFunc("GET /api/v1/lessons", srv.handleListLessons)
		mux.HandleFunc("POST /api/v1/lessons", srv.handleCreateLesson)
		mux.HandleFunc("GET /api/v1/lessons/{id}", srv.handleGetLesson)
		mux.HandleFunc("DELETE /api/v1/lessons/{id}", srv.handleDeleteLesson)
		mux.HandleFunc("POST /api/v1/lessons/{id}/messages", srv.handleAddLessonMessage)
	}

	// Register strategic memory routes
	if strategicStore != nil {
		mux.HandleFunc("GET /api/v1/strategic-memory", srv.handleListStrategicMemory)
		mux.HandleFunc("POST /api/v1/strategic-memory", srv.handleCreateStrategicMemory)
		mux.HandleFunc("GET /api/v1/strategic-memory/{id}", srv.handleGetStrategicMemory)
	}

	// Register LLM metrics routes (for monitoring apps)
	server.RegisterMetricsRoutes(mux)

	// Register Pinky-compatible REST endpoints
	var memAdapter MemoryStoreInterface
	if cfg.CoreMemoryStore != nil {
		memAdapter = &coreMemoryAdapter{store: cfg.CoreMemoryStore}
	}
	pinkyHandler := NewPinkyCompatHandler(brainExec, memAdapter)
	pinkyHandler.RegisterRoutes(mux)

	return srv
}

// ServeHTTP implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	s.mux.ServeHTTP(w, r)
}

// Start starts the server
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s,
	}

	s.log.Info("[A2A] ═══════════════════════════════════════════════════════════════")
	s.log.Info("[A2A] Cortex A2A Server")
	s.log.Info("[A2A] ═══════════════════════════════════════════════════════════════")
	s.log.Info("[A2A] Agent: %s v%s", s.card.Name, s.card.Version)
	s.log.Info("[A2A] Protocol: A2A v%s", s.card.ProtocolVersion)
	s.log.Info("[A2A] Transport: %s", s.card.PreferredTransport)
	s.log.Info("[A2A] ───────────────────────────────────────────────────────────────")
	s.log.Info("[A2A] Capabilities:")
	s.log.Info("[A2A]   Streaming:         %v", s.card.Capabilities.Streaming)
	s.log.Info("[A2A]   Push Notifications: %v", s.card.Capabilities.PushNotifications)
	s.log.Info("[A2A]   State History:      %v", s.card.Capabilities.StateTransitionHistory)
	s.log.Info("[A2A] ───────────────────────────────────────────────────────────────")
	s.log.Info("[A2A] Skills (%d):", len(s.card.Skills))
	for _, skill := range s.card.Skills {
		s.log.Info("[A2A]   • %s: %s", skill.ID, skill.Name)
	}
	s.log.Info("[A2A] ───────────────────────────────────────────────────────────────")
	s.log.Info("[A2A] Endpoints:")
	s.log.Info("[A2A]   Agent Card:     http://localhost%s/.well-known/agent-card.json", addr)
	s.log.Info("[A2A]   JSON-RPC:       POST http://localhost%s/", addr)
	s.log.Info("[A2A]   LLM Providers:  GET http://localhost%s/api/llm/providers", addr)
	s.log.Info("[A2A]   LLM Chat:       POST http://localhost%s/api/llm/chat", addr)
	s.log.Info("[A2A]   Config:         GET/PUT http://localhost%s/api/config/providers", addr)
	s.log.Info("[A2A]   Auth:           POST http://localhost%s/api/auth/{login,register,refresh}", addr)
	s.log.Info("[A2A]   User Personas:  GET/POST/DELETE http://localhost%s/api/v1/users/:id/personas", addr)
	s.log.Info("[A2A] ═══════════════════════════════════════════════════════════════")

	return s.server.ListenAndServe()
}

// Stop stops the server gracefully
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	s.log.Info("[A2A] Shutting down server...")
	return s.server.Shutdown(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════════
// PERSONA HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// PersonaResponse is the JSON response format for a persona
type PersonaResponse struct {
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Background   string   `json:"background,omitempty"`
	Traits       []string `json:"traits,omitempty"`
	Values       []string `json:"values,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	IsBuiltIn    bool     `json:"is_built_in"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
}

// PersonasListResponse is the JSON response for listing personas
type PersonasListResponse struct {
	Personas []PersonaResponse `json:"personas"`
	Total    int               `json:"total"`
}

// handleListPersonas handles GET /api/v1/personas
func (s *Server) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	if s.personaStore == nil {
		s.writeError(w, http.StatusInternalServerError, "persona store not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check for role filter
	roleFilter := r.URL.Query().Get("role")

	var personas []*facets.PersonaCore
	var err error

	if roleFilter != "" {
		personas, err = s.personaStore.ListByRole(ctx, roleFilter)
	} else {
		personas, err = s.personaStore.List(ctx)
	}

	if err != nil {
		s.log.Warn("[A2A] Failed to list personas: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list personas")
		return
	}

	// Convert to response format
	responses := make([]PersonaResponse, len(personas))
	for i, p := range personas {
		responses[i] = PersonaResponse{
			ID:           p.ID,
			Version:      p.Version,
			Name:         p.Name,
			Role:         p.Role,
			Background:   p.Background,
			Traits:       p.Traits,
			Values:       p.Values,
			SystemPrompt: p.SystemPrompt,
			IsBuiltIn:    p.IsBuiltIn,
			CreatedAt:    p.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
		}
	}

	response := PersonasListResponse{
		Personas: responses,
		Total:    len(responses),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleGetPersona handles GET /api/v1/personas/{id}
func (s *Server) handleGetPersona(w http.ResponseWriter, r *http.Request) {
	if s.personaStore == nil {
		s.writeError(w, http.StatusInternalServerError, "persona store not configured")
		return
	}

	personaID := r.PathValue("id")
	if personaID == "" {
		s.writeError(w, http.StatusBadRequest, "persona ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	persona, err := s.personaStore.Get(ctx, personaID)
	if err != nil {
		s.log.Warn("[A2A] Failed to get persona %s: %v", personaID, err)
		s.writeError(w, http.StatusNotFound, "persona not found")
		return
	}

	response := PersonaResponse{
		ID:           persona.ID,
		Version:      persona.Version,
		Name:         persona.Name,
		Role:         persona.Role,
		Background:   persona.Background,
		Traits:       persona.Traits,
		Values:       persona.Values,
		SystemPrompt: persona.SystemPrompt,
		IsBuiltIn:    persona.IsBuiltIn,
		CreatedAt:    persona.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    persona.UpdatedAt.Format(time.RFC3339),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// writeJSON writes a JSON response
func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message})
}

// ═══════════════════════════════════════════════════════════════════════════════
// LESSON HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// LessonResponse is the JSON response format for a lesson
type LessonResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	PersonaID    string `json:"persona_id"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	Status       string `json:"status"`
	MessageCount int    `json:"message_count"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// LessonMessageResponse is the JSON response format for a lesson message
type LessonMessageResponse struct {
	ID        int    `json:"id"`
	LessonID  string `json:"lesson_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// LessonsListResponse is the JSON response for listing lessons
type LessonsListResponse struct {
	Lessons []LessonResponse `json:"lessons"`
	Total   int              `json:"total"`
}

// LessonDetailResponse includes lesson and messages
type LessonDetailResponse struct {
	Lesson   LessonResponse          `json:"lesson"`
	Messages []LessonMessageResponse `json:"messages"`
}

// handleListLessons handles GET /api/v1/lessons
func (s *Server) handleListLessons(w http.ResponseWriter, r *http.Request) {
	if s.lessonStore == nil {
		s.writeError(w, http.StatusInternalServerError, "lesson store not configured")
		return
	}

	// Get user ID from query param or auth header
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		s.writeError(w, http.StatusBadRequest, "user_id required")
		return
	}

	personaID := r.URL.Query().Get("persona_id")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	lessons, err := s.lessonStore.List(ctx, userID, personaID, 50)
	if err != nil {
		s.log.Warn("[A2A] Failed to list lessons: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list lessons")
		return
	}

	// Convert to response format
	responses := make([]LessonResponse, len(lessons))
	for i, l := range lessons {
		responses[i] = LessonResponse{
			ID:           l.ID,
			UserID:       l.UserID,
			PersonaID:    l.PersonaID,
			Title:        l.Title,
			Summary:      l.Summary,
			Status:       l.Status,
			MessageCount: l.MessageCount,
			CreatedAt:    l.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    l.UpdatedAt.Format(time.RFC3339),
		}
	}

	s.writeJSON(w, http.StatusOK, LessonsListResponse{
		Lessons: responses,
		Total:   len(responses),
	})
}

// handleCreateLesson handles POST /api/v1/lessons
func (s *Server) handleCreateLesson(w http.ResponseWriter, r *http.Request) {
	if s.lessonStore == nil {
		s.writeError(w, http.StatusInternalServerError, "lesson store not configured")
		return
	}

	var req struct {
		UserID    string `json:"user_id"`
		PersonaID string `json:"persona_id"`
		Title     string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" || req.PersonaID == "" {
		s.writeError(w, http.StatusBadRequest, "user_id and persona_id required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	lesson, err := s.lessonStore.Create(ctx, req.UserID, req.PersonaID, req.Title)
	if err != nil {
		s.log.Warn("[A2A] Failed to create lesson: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create lesson")
		return
	}

	s.writeJSON(w, http.StatusCreated, LessonResponse{
		ID:           lesson.ID,
		UserID:       lesson.UserID,
		PersonaID:    lesson.PersonaID,
		Title:        lesson.Title,
		Summary:      lesson.Summary,
		Status:       lesson.Status,
		MessageCount: lesson.MessageCount,
		CreatedAt:    lesson.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    lesson.UpdatedAt.Format(time.RFC3339),
	})
}

// handleGetLesson handles GET /api/v1/lessons/{id}
func (s *Server) handleGetLesson(w http.ResponseWriter, r *http.Request) {
	if s.lessonStore == nil {
		s.writeError(w, http.StatusInternalServerError, "lesson store not configured")
		return
	}

	lessonID := r.PathValue("id")
	if lessonID == "" {
		s.writeError(w, http.StatusBadRequest, "lesson ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	lesson, err := s.lessonStore.Get(ctx, lessonID)
	if err != nil {
		s.log.Warn("[A2A] Failed to get lesson %s: %v", lessonID, err)
		s.writeError(w, http.StatusNotFound, "lesson not found")
		return
	}

	messages, err := s.lessonStore.GetMessages(ctx, lessonID)
	if err != nil {
		s.log.Warn("[A2A] Failed to get lesson messages: %v", err)
		messages = []*LessonMessage{}
	}

	// Convert messages to response format
	msgResponses := make([]LessonMessageResponse, len(messages))
	for i, msg := range messages {
		msgResponses[i] = LessonMessageResponse{
			ID:        msg.ID,
			LessonID:  msg.LessonID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format(time.RFC3339),
		}
	}

	s.writeJSON(w, http.StatusOK, LessonDetailResponse{
		Lesson: LessonResponse{
			ID:           lesson.ID,
			UserID:       lesson.UserID,
			PersonaID:    lesson.PersonaID,
			Title:        lesson.Title,
			Summary:      lesson.Summary,
			Status:       lesson.Status,
			MessageCount: lesson.MessageCount,
			CreatedAt:    lesson.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    lesson.UpdatedAt.Format(time.RFC3339),
		},
		Messages: msgResponses,
	})
}

// handleDeleteLesson handles DELETE /api/v1/lessons/{id}
func (s *Server) handleDeleteLesson(w http.ResponseWriter, r *http.Request) {
	if s.lessonStore == nil {
		s.writeError(w, http.StatusInternalServerError, "lesson store not configured")
		return
	}

	lessonID := r.PathValue("id")
	if lessonID == "" {
		s.writeError(w, http.StatusBadRequest, "lesson ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := s.lessonStore.Delete(ctx, lessonID); err != nil {
		s.log.Warn("[A2A] Failed to delete lesson %s: %v", lessonID, err)
		s.writeError(w, http.StatusNotFound, "lesson not found")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleAddLessonMessage handles POST /api/v1/lessons/{id}/messages
func (s *Server) handleAddLessonMessage(w http.ResponseWriter, r *http.Request) {
	if s.lessonStore == nil {
		s.writeError(w, http.StatusInternalServerError, "lesson store not configured")
		return
	}

	lessonID := r.PathValue("id")
	if lessonID == "" {
		s.writeError(w, http.StatusBadRequest, "lesson ID required")
		return
	}

	var req struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role == "" || req.Content == "" {
		s.writeError(w, http.StatusBadRequest, "role and content required")
		return
	}

	if req.Role != "user" && req.Role != "assistant" {
		s.writeError(w, http.StatusBadRequest, "role must be 'user' or 'assistant'")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	msg, err := s.lessonStore.AddMessage(ctx, lessonID, req.Role, req.Content)
	if err != nil {
		s.log.Warn("[A2A] Failed to add message to lesson %s: %v", lessonID, err)
		s.writeError(w, http.StatusInternalServerError, "failed to add message")
		return
	}

	s.writeJSON(w, http.StatusCreated, LessonMessageResponse{
		ID:        msg.ID,
		LessonID:  msg.LessonID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt.Format(time.RFC3339),
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// STRATEGIC MEMORY HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// StrategicMemoryRequest is the JSON request format for creating strategic memory
type StrategicMemoryRequest struct {
	Principle      string   `json:"principle"`
	Category       string   `json:"category"`
	TriggerPattern string   `json:"trigger_pattern"`
	Tier           string   `json:"tier,omitempty"`
	Confidence     float64  `json:"confidence,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

// StrategicMemoryResponse is the JSON response format for strategic memory
type StrategicMemoryResponse struct {
	ID             string  `json:"id"`
	Principle      string  `json:"principle"`
	Category       string  `json:"category"`
	TriggerPattern string  `json:"trigger_pattern"`
	Tier           string  `json:"tier"`
	SuccessCount   int     `json:"success_count"`
	FailureCount   int     `json:"failure_count"`
	ApplyCount     int     `json:"apply_count"`
	SuccessRate    float64 `json:"success_rate"`
	Confidence     float64 `json:"confidence"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// StrategicMemoryListResponse is the JSON response for listing strategic memories
type StrategicMemoryListResponse struct {
	Memories []StrategicMemoryResponse `json:"memories"`
	Total    int                       `json:"total"`
}

// handleListStrategicMemory handles GET /api/v1/strategic-memory
func (s *Server) handleListStrategicMemory(w http.ResponseWriter, r *http.Request) {
	if s.strategicStore == nil {
		s.writeError(w, http.StatusInternalServerError, "strategic memory store not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check for category filter
	category := r.URL.Query().Get("category")
	tier := r.URL.Query().Get("tier")
	limit := 100 // Default limit

	var memories []memory.StrategicMemory
	var err error

	if category != "" {
		memories, err = s.strategicStore.GetByCategory(ctx, category, limit)
	} else if tier != "" {
		memories, err = s.strategicStore.GetByTier(ctx, memory.MemoryTier(tier), limit)
	} else {
		memories, err = s.strategicStore.List(ctx, limit)
	}

	if err != nil {
		s.log.Warn("[A2A] Failed to list strategic memories: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to list strategic memories")
		return
	}

	responses := make([]StrategicMemoryResponse, len(memories))
	for i, mem := range memories {
		responses[i] = StrategicMemoryResponse{
			ID:             mem.ID,
			Principle:      mem.Principle,
			Category:       mem.Category,
			TriggerPattern: mem.TriggerPattern,
			Tier:           string(mem.Tier),
			SuccessCount:   mem.SuccessCount,
			FailureCount:   mem.FailureCount,
			ApplyCount:     mem.ApplyCount,
			SuccessRate:    mem.SuccessRate,
			Confidence:     mem.Confidence,
			CreatedAt:      mem.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      mem.UpdatedAt.Format(time.RFC3339),
		}
	}

	s.writeJSON(w, http.StatusOK, StrategicMemoryListResponse{
		Memories: responses,
		Total:    len(responses),
	})
}

// handleCreateStrategicMemory handles POST /api/v1/strategic-memory
func (s *Server) handleCreateStrategicMemory(w http.ResponseWriter, r *http.Request) {
	if s.strategicStore == nil {
		s.writeError(w, http.StatusInternalServerError, "strategic memory store not configured")
		return
	}

	var req StrategicMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Principle == "" {
		s.writeError(w, http.StatusBadRequest, "principle is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Create the strategic memory
	mem := &memory.StrategicMemory{
		Principle:      req.Principle,
		Category:       req.Category,
		TriggerPattern: req.TriggerPattern,
		Confidence:     req.Confidence,
	}

	// Set tier if provided
	if req.Tier != "" {
		mem.Tier = memory.MemoryTier(req.Tier)
	}

	if err := s.strategicStore.Create(ctx, mem); err != nil {
		s.log.Warn("[A2A] Failed to create strategic memory: %v", err)
		s.writeError(w, http.StatusInternalServerError, "failed to create strategic memory")
		return
	}

	s.log.Info("[A2A] Created strategic memory: %s (category=%s)", mem.ID, mem.Category)

	s.writeJSON(w, http.StatusCreated, StrategicMemoryResponse{
		ID:             mem.ID,
		Principle:      mem.Principle,
		Category:       mem.Category,
		TriggerPattern: mem.TriggerPattern,
		Tier:           string(mem.Tier),
		SuccessCount:   mem.SuccessCount,
		FailureCount:   mem.FailureCount,
		ApplyCount:     mem.ApplyCount,
		SuccessRate:    mem.SuccessRate,
		Confidence:     mem.Confidence,
		CreatedAt:      mem.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      mem.UpdatedAt.Format(time.RFC3339),
	})
}

// handleGetStrategicMemory handles GET /api/v1/strategic-memory/{id}
func (s *Server) handleGetStrategicMemory(w http.ResponseWriter, r *http.Request) {
	if s.strategicStore == nil {
		s.writeError(w, http.StatusInternalServerError, "strategic memory store not configured")
		return
	}

	memID := r.PathValue("id")
	if memID == "" {
		s.writeError(w, http.StatusBadRequest, "memory ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	mem, err := s.strategicStore.Get(ctx, memID)
	if err != nil {
		s.log.Warn("[A2A] Failed to get strategic memory %s: %v", memID, err)
		s.writeError(w, http.StatusNotFound, "strategic memory not found")
		return
	}

	s.writeJSON(w, http.StatusOK, StrategicMemoryResponse{
		ID:             mem.ID,
		Principle:      mem.Principle,
		Category:       mem.Category,
		TriggerPattern: mem.TriggerPattern,
		Tier:           string(mem.Tier),
		SuccessCount:   mem.SuccessCount,
		FailureCount:   mem.FailureCount,
		ApplyCount:     mem.ApplyCount,
		SuccessRate:    mem.SuccessRate,
		Confidence:     mem.Confidence,
		CreatedAt:      mem.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      mem.UpdatedAt.Format(time.RFC3339),
	})
}
