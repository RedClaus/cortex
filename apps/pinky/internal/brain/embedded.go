// Package brain provides EmbeddedBrain implementation for in-process LLM inference.
package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/normanking/pinky/internal/config"
)

// MemoryStore interface for persistent memory storage.
// This is implemented by memory.Store in the memory package.
type MemoryStore interface {
	Store(ctx context.Context, mem *Memory) error
	Recall(ctx context.Context, query string, opts MemoryRecallOptions) ([]Memory, error)
	GetRecent(ctx context.Context, userID string, limit int) ([]Memory, error)
	Close() error
}

// MemoryRecallOptions configures memory recall behavior.
type MemoryRecallOptions struct {
	UserID        string
	Limit         int
	MinImportance float64
	Types         []MemoryType
	Since         time.Time
	Until         time.Time
	TimeContext   any // *memory.TemporalContext, passed as any to avoid import cycle
}

// EmbeddedBrain implements Brain using local LLM inference (e.g., ollama).
type EmbeddedBrain struct {
	cfg          config.InferenceConfig
	client       *http.Client
	memoryStore  MemoryStore    // Optional persistent store
	memories     []Memory       // In-memory fallback
	mu           sync.RWMutex
	currentLane  string         // Current active lane (can be switched at runtime)
	autoLLM      bool           // Whether AutoLLM routing is enabled

	// Circuit breaker for fault tolerance
	circuitBreakers *CircuitBreakerRegistry
	fallbackConfig  *FallbackConfig
}

// LaneInfo provides information about an available lane
type LaneInfo struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
	Model  string `json:"model"`
	Active bool   `json:"active"`
}

// NewEmbeddedBrain creates a new embedded brain with local inference.
func NewEmbeddedBrain(cfg config.InferenceConfig) *EmbeddedBrain {
	return &EmbeddedBrain{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute, // LLM responses can be slow
		},
		memories:        make([]Memory, 0),
		currentLane:     cfg.DefaultLane,
		autoLLM:         cfg.AutoLLM,
		circuitBreakers: NewCircuitBreakerRegistry(DefaultCircuitBreakerConfig()),
	}
}

// SetLane switches to a specific lane (fast, local, smart, etc.)
func (b *EmbeddedBrain) SetLane(name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.cfg.Lanes[name]; !ok {
		return fmt.Errorf("lane '%s' not found (available: %v)", name, b.GetLaneNames())
	}
	b.currentLane = name
	return nil
}

// GetLane returns the current active lane name
func (b *EmbeddedBrain) GetLane() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentLane
}

// GetLaneNames returns a list of all available lane names
func (b *EmbeddedBrain) GetLaneNames() []string {
	names := make([]string, 0, len(b.cfg.Lanes))
	for name := range b.cfg.Lanes {
		names = append(names, name)
	}
	return names
}

// GetLanes returns information about all available lanes
func (b *EmbeddedBrain) GetLanes() []LaneInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lanes := make([]LaneInfo, 0, len(b.cfg.Lanes))
	for name, lane := range b.cfg.Lanes {
		lanes = append(lanes, LaneInfo{
			Name:   name,
			Engine: lane.Engine,
			Model:  lane.Model,
			Active: name == b.currentLane,
		})
	}
	return lanes
}

// GetAvailableModels returns available models for a given lane from the inference backend.
func (b *EmbeddedBrain) GetAvailableModels(laneName string) ([]string, error) {
	b.mu.RLock()
	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		b.mu.RUnlock()
		return nil, fmt.Errorf("lane not found: %s", laneName)
	}
	b.mu.RUnlock()

	switch lane.Engine {
	case "ollama":
		url := b.getLaneURL(&lane)
		if url == "" {
			url = "http://localhost:11434"
		}

		resp, err := http.Get(url + "/api/tags")
		if err != nil {
			return nil, fmt.Errorf("failed to fetch ollama models: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
		}

		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		models := make([]string, 0, len(result.Models))
		for _, m := range result.Models {
			models = append(models, m.Name)
		}
		return models, nil

	case "vllm":
		return b.GetVLLMModels(laneName)

	default:
		// Cloud APIs don't support dynamic model listing
		return []string{lane.Model}, nil
	}
}

// SetAutoLLM enables or disables automatic lane selection
func (b *EmbeddedBrain) SetAutoLLM(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.autoLLM = enabled
}

// GetAutoLLM returns whether AutoLLM routing is enabled
func (b *EmbeddedBrain) GetAutoLLM() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.autoLLM
}

// selectLane chooses the appropriate lane based on request complexity (when AutoLLM is enabled)
// Priority order for high-performance local inference:
//   1. vLLM (if available and circuit closed) - fastest local inference with continuous batching
//   2. ollama - standard local inference
//   3. groq/openai - cloud fallback
func (b *EmbeddedBrain) selectLane(req *ThinkRequest) *config.Lane {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// If AutoLLM is disabled, use the current lane
	if !b.autoLLM {
		lane, ok := b.cfg.Lanes[b.currentLane]
		if !ok {
			return nil
		}
		return &lane
	}

	// AutoLLM: Route based on estimated complexity
	complexity := b.estimateComplexity(req)

	var laneName string
	switch {
	case complexity >= 0.7:
		// Complex task: prefer vLLM (high throughput) or smart lane
		laneName = b.selectBestLane([]string{"vllm", "smart", "fast"})
	case complexity >= 0.3:
		// Medium task: prefer vLLM or fast lane
		laneName = b.selectBestLane([]string{"vllm", "fast", "local"})
	default:
		// Simple task: prefer local (lowest latency) or vLLM
		laneName = b.selectBestLane([]string{"local", "vllm", "fast"})
	}

	if laneName == "" {
		laneName = b.currentLane
	}

	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		return nil
	}
	return &lane
}

// selectBestLane returns the first available lane from the preference list,
// considering circuit breaker state for each lane.
func (b *EmbeddedBrain) selectBestLane(preferences []string) string {
	for _, name := range preferences {
		if _, ok := b.cfg.Lanes[name]; !ok {
			continue
		}

		// Check circuit breaker for this lane
		cb := b.circuitBreakers.Get(name)
		if cb.Allow() {
			return name
		}
	}

	// Fall back to current lane
	return b.currentLane
}

// estimateComplexity analyzes a request and returns a complexity score (0.0 - 1.0)
func (b *EmbeddedBrain) estimateComplexity(req *ThinkRequest) float64 {
	if req == nil || len(req.Messages) == 0 {
		return 0.0
	}

	// Get the last user message
	var lastMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastMessage = req.Messages[i].Content
			break
		}
	}

	if lastMessage == "" {
		return 0.0
	}

	complexity := 0.0
	msgLen := len(lastMessage)

	// Length-based complexity
	if msgLen > 500 {
		complexity += 0.3
	} else if msgLen > 200 {
		complexity += 0.15
	}

	// Keyword-based complexity indicators
	complexKeywords := []string{
		"analyze", "explain", "compare", "design", "architect",
		"implement", "debug", "refactor", "optimize", "review",
		"complex", "detailed", "comprehensive", "in-depth",
		"multiple", "several", "all", "every", "entire",
	}

	simpleKeywords := []string{
		"what is", "how to", "quick", "simple", "just",
		"hello", "hi", "thanks", "yes", "no", "ok",
	}

	msgLower := strings.ToLower(lastMessage)

	for _, kw := range complexKeywords {
		if strings.Contains(msgLower, kw) {
			complexity += 0.1
		}
	}

	for _, kw := range simpleKeywords {
		if strings.Contains(msgLower, kw) {
			complexity -= 0.1
		}
	}

	// Clamp to 0.0 - 1.0
	if complexity < 0.0 {
		complexity = 0.0
	}
	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

// NewEmbeddedBrainWithStore creates a new embedded brain with a persistent memory store.
func NewEmbeddedBrainWithStore(cfg config.InferenceConfig, store MemoryStore) *EmbeddedBrain {
	return &EmbeddedBrain{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
		memoryStore:     store,
		memories:        make([]Memory, 0),
		currentLane:     cfg.DefaultLane,
		autoLLM:         cfg.AutoLLM,
		circuitBreakers: NewCircuitBreakerRegistry(DefaultCircuitBreakerConfig()),
	}
}

// Mode returns ModeEmbedded.
func (b *EmbeddedBrain) Mode() BrainMode {
	return ModeEmbedded
}

// Ping checks if the inference backend is reachable.
func (b *EmbeddedBrain) Ping(ctx context.Context) error {
	lane := b.getDefaultLane()
	if lane == nil {
		return fmt.Errorf("no default lane configured")
	}

	// Cloud APIs (groq, openai, anthropic) don't have simple health endpoints
	switch lane.Engine {
	case "ollama":
		url := b.getLaneURL(lane)
		if url == "" {
			return nil // No URL configured, skip ping
		}
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		resp, err := b.client.Do(req)
		if err != nil {
			return fmt.Errorf("ollama backend unreachable: %w", err)
		}
		defer resp.Body.Close()
		return nil

	case "vllm":
		return b.vllmPing(ctx, lane)

	default:
		// Skip ping for cloud APIs (openai, anthropic, groq)
		return nil
	}
}

// Think processes a request and returns a response.
func (b *EmbeddedBrain) Think(ctx context.Context, req *ThinkRequest) (*ThinkResponse, error) {
	lane := b.selectLane(req)
	if lane == nil {
		return nil, fmt.Errorf("no lane configured (current: %s)", b.currentLane)
	}

	// Build the prompt from messages
	prompt := b.buildPrompt(req)

	// Call the inference backend
	switch lane.Engine {
	case "ollama":
		return b.ollamaGenerate(ctx, lane, prompt, req)
	case "openai":
		return b.openaiGenerate(ctx, lane, prompt, req)
	case "anthropic":
		return b.anthropicGenerate(ctx, lane, prompt, req)
	case "groq":
		return b.groqGenerate(ctx, lane, prompt, req)
	case "vllm":
		return b.vllmGenerate(ctx, lane, prompt, req)
	default:
		return nil, fmt.Errorf("unsupported engine: %s (supported: ollama, openai, anthropic, groq, vllm)", lane.Engine)
	}
}

// ThinkStream returns a channel of response chunks for streaming.
func (b *EmbeddedBrain) ThinkStream(ctx context.Context, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	lane := b.selectLane(req)
	if lane == nil {
		return nil, fmt.Errorf("no lane configured (current: %s)", b.currentLane)
	}

	prompt := b.buildPrompt(req)

	switch lane.Engine {
	case "ollama":
		return b.ollamaStream(ctx, lane, prompt, req)
	case "openai":
		return b.openaiStream(ctx, lane, prompt, req)
	case "anthropic":
		return b.anthropicStream(ctx, lane, prompt, req)
	case "groq":
		return b.groqStream(ctx, lane, prompt, req)
	case "vllm":
		return b.vllmStream(ctx, lane, prompt, req)
	default:
		return nil, fmt.Errorf("unsupported engine: %s (supported: ollama, openai, anthropic, groq, vllm)", lane.Engine)
	}
}

// Remember stores a memory.
func (b *EmbeddedBrain) Remember(ctx context.Context, memory *Memory) error {
	memory.CreatedAt = time.Now()
	memory.AccessedAt = time.Now()

	// Use persistent store if available
	if b.memoryStore != nil {
		return b.memoryStore.Store(ctx, memory)
	}

	// Fall back to in-memory storage
	b.mu.Lock()
	defer b.mu.Unlock()
	b.memories = append(b.memories, *memory)
	return nil
}

// Recall retrieves relevant memories with temporal awareness.
func (b *EmbeddedBrain) Recall(ctx context.Context, query string, limit int) ([]Memory, error) {
	// Use persistent store if available
	if b.memoryStore != nil {
		return b.memoryStore.Recall(ctx, query, MemoryRecallOptions{
			Limit: limit,
		})
	}

	// Fall back to simple in-memory keyword matching
	b.mu.RLock()
	defer b.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []Memory

	for i := range b.memories {
		if strings.Contains(strings.ToLower(b.memories[i].Content), queryLower) {
			results = append(results, b.memories[i])
			if len(results) >= limit {
				break
			}
		}
	}

	return results, nil
}

// RecallWithContext retrieves memories with full temporal context support.
func (b *EmbeddedBrain) RecallWithContext(ctx context.Context, query string, opts MemoryRecallOptions) ([]Memory, error) {
	if b.memoryStore != nil {
		return b.memoryStore.Recall(ctx, query, opts)
	}

	// Fall back to basic recall
	return b.Recall(ctx, query, opts.Limit)
}

// getDefaultLane returns the default inference lane.
func (b *EmbeddedBrain) getDefaultLane() *config.Lane {
	laneName := b.cfg.DefaultLane
	if laneName == "" {
		laneName = "fast"
	}
	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		return nil
	}
	return &lane
}

// getLaneURL returns the URL for a lane's inference endpoint.
func (b *EmbeddedBrain) getLaneURL(lane *config.Lane) string {
	if lane.URL != "" {
		return lane.URL
	}
	// Default ollama URL
	if lane.Engine == "ollama" {
		return "http://localhost:11434"
	}
	return ""
}

// defaultSystemPrompt is used when no persona is provided.
const defaultSystemPrompt = `You are Pinky, an intelligent AI assistant that uses tools to help users.

CRITICAL: You have tools available and MUST use them when appropriate:
- For questions about the real world (weather, news, places, people, events): use web_search
- For file/folder operations (count, list, read, write, find): use shell or files
- For running commands or scripts on the user's computer: use shell
- For code execution (calculations, data processing): use code

When the user asks you to do something on their computer, USE A TOOL. Never say "I don't have information" or "I can't do that" - check your available tools and use the appropriate one.

Always be helpful and provide clear, accurate responses.`

// buildPrompt constructs the prompt from messages, persona, and tools.
func (b *EmbeddedBrain) buildPrompt(req *ThinkRequest) string {
	var parts []string

	// Add system prompt from persona, or use default
	if req.Persona != nil && req.Persona.SystemPrompt != "" {
		parts = append(parts, req.Persona.SystemPrompt)
	} else {
		parts = append(parts, defaultSystemPrompt)
	}

	// Add tool descriptions if tools are provided
	if len(req.Tools) > 0 {
		toolsDesc := ToolsDescription(req.Tools)
		if toolsDesc != "" {
			parts = append(parts, toolsDesc)
		}
	}

	// Add memories as context
	for _, mem := range req.Memories {
		parts = append(parts, fmt.Sprintf("[Memory] %s", mem.Content))
	}

	// Add conversation messages
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case "assistant":
			// Add text content if present
			if msg.Content != "" {
				parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
			}
			// Format tool calls so follow-up queries have context
			for _, tc := range msg.ToolCalls {
				parts = append(parts, fmt.Sprintf("Assistant: [Using tool: %s with params: %v]", tc.Tool, tc.Input))
			}
		case "system":
			parts = append(parts, msg.Content)
		case "tool":
			// Include tool results in conversation so follow-up queries work
			for _, tr := range msg.ToolResults {
				if tr.Success {
					parts = append(parts, fmt.Sprintf("[Tool Result (success)]: %s", tr.Output))
				} else {
					parts = append(parts, fmt.Sprintf("[Tool Result (error)]: %s", tr.Error))
				}
			}
		}
	}

	return strings.Join(parts, "\n\n")
}

// buildOpenAIMessages creates properly structured messages for OpenAI/Groq chat APIs.
// This separates system instructions from user/assistant messages for better model understanding.
func (b *EmbeddedBrain) buildOpenAIMessages(req *ThinkRequest) []openaiMessage {
	var messages []openaiMessage

	// Build system message with instructions and tools
	var systemParts []string

	// Add system prompt from persona or use default
	if req.Persona != nil && req.Persona.SystemPrompt != "" {
		systemParts = append(systemParts, req.Persona.SystemPrompt)
	} else {
		systemParts = append(systemParts, defaultSystemPrompt)
	}

	// Add tool descriptions
	if len(req.Tools) > 0 {
		toolsDesc := ToolsDescription(req.Tools)
		if toolsDesc != "" {
			systemParts = append(systemParts, toolsDesc)
		}
	}

	// Add memories to system context
	for _, mem := range req.Memories {
		systemParts = append(systemParts, fmt.Sprintf("[Memory] %s", mem.Content))
	}

	// Add system message
	messages = append(messages, openaiMessage{
		Role:    "system",
		Content: strings.Join(systemParts, "\n\n"),
	})

	// Add conversation messages with proper roles
	for _, msg := range req.Messages {
		switch msg.Role {
		case "user":
			messages = append(messages, openaiMessage{Role: "user", Content: msg.Content})
		case "assistant":
			content := msg.Content
			// Include tool call info if present
			for _, tc := range msg.ToolCalls {
				if content != "" {
					content += "\n"
				}
				content += fmt.Sprintf("[Using tool: %s with params: %v]", tc.Tool, tc.Input)
			}
			if content != "" {
				messages = append(messages, openaiMessage{Role: "assistant", Content: content})
			}
		case "tool":
			// Add tool results as assistant messages showing the result
			for _, tr := range msg.ToolResults {
				var resultContent string
				if tr.Success {
					resultContent = fmt.Sprintf("[Tool Result (success)]: %s", tr.Output)
				} else {
					resultContent = fmt.Sprintf("[Tool Result (error)]: %s", tr.Error)
				}
				messages = append(messages, openaiMessage{Role: "assistant", Content: resultContent})
			}
		}
	}

	return messages
}

// ollamaRequest is the request format for ollama API.
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Options struct {
		Temperature float64 `json:"temperature,omitempty"`
		NumPredict  int     `json:"num_predict,omitempty"`
	} `json:"options,omitempty"`
}

// ollamaResponse is the response format from ollama API.
type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// ollamaGenerate calls ollama's generate endpoint (non-streaming).
func (b *EmbeddedBrain) ollamaGenerate(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (*ThinkResponse, error) {
	url := b.getLaneURL(lane) + "/api/generate"

	ollamaReq := ollamaRequest{
		Model:  lane.Model,
		Prompt: prompt,
		Stream: false,
	}
	if req.Temperature > 0 {
		ollamaReq.Options.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, err
	}

	// Parse any tool calls from the response
	toolCalls, cleanedContent := ParseToolCalls(ollamaResp.Response)

	return &ThinkResponse{
		Content:   cleanedContent,
		ToolCalls: toolCalls,
		Done:      true,
	}, nil
}

// ollamaStream calls ollama's generate endpoint with streaming.
func (b *EmbeddedBrain) ollamaStream(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	url := b.getLaneURL(lane) + "/api/generate"

	ollamaReq := ollamaRequest{
		Model:  lane.Model,
		Prompt: prompt,
		Stream: true,
	}
	if req.Temperature > 0 {
		ollamaReq.Options.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		ollamaReq.Options.NumPredict = req.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(respBody))
	}

	chunks := make(chan *ThinkChunk, 100)

	go func() {
		defer resp.Body.Close()
		defer close(chunks)

		decoder := json.NewDecoder(resp.Body)
		for {
			var ollamaResp ollamaResponse
			if err := decoder.Decode(&ollamaResp); err != nil {
				if err != io.EOF {
					chunks <- &ThinkChunk{Error: err}
				}
				return
			}

			chunks <- &ThinkChunk{
				Content: ollamaResp.Response,
				Done:    ollamaResp.Done,
			}

			if ollamaResp.Done {
				return
			}
		}
	}()

	return chunks, nil
}

// ============================================================================
// OpenAI Engine Implementation
// ============================================================================

type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func (b *EmbeddedBrain) getAPIKey(lane *config.Lane) string {
	key := lane.APIKey
	if key == "" {
		return ""
	}
	// Expand environment variables like ${OPENAI_API_KEY}
	if len(key) > 3 && key[0] == '$' && key[1] == '{' && key[len(key)-1] == '}' {
		envVar := key[2 : len(key)-1]
		return os.Getenv(envVar)
	}
	return key
}

func (b *EmbeddedBrain) openaiGenerate(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (*ThinkResponse, error) {
	url := lane.URL
	if url == "" {
		url = "https://api.openai.com/v1/chat/completions"
	}

	apiKey := b.getAPIKey(lane)
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured (set api_key in lane config or OPENAI_API_KEY env var)")
	}

	// Build proper message array with system and user roles
	messages := b.buildOpenAIMessages(req)

	// DEBUG: Log message structure
	fmt.Printf("[DEBUG] OpenAI request - %d messages, %d tools in request\n", len(messages), len(req.Tools))
	for i, m := range messages {
		preview := m.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("[DEBUG] Message %d [%s]: %s\n", i, m.Role, preview)
	}

	openaiReq := openaiRequest{
		Model:    lane.Model,
		Messages: messages,
		Stream:   false,
	}
	if req.Temperature > 0 {
		openaiReq.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI error %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, err
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	// Parse any tool calls from the response
	toolCalls, cleanedContent := ParseToolCalls(openaiResp.Choices[0].Message.Content)

	return &ThinkResponse{
		Content:   cleanedContent,
		ToolCalls: toolCalls,
		Done:      true,
	}, nil
}

func (b *EmbeddedBrain) openaiStream(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	url := lane.URL
	if url == "" {
		url = "https://api.openai.com/v1/chat/completions"
	}

	apiKey := b.getAPIKey(lane)
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Build proper message array with system and user roles
	messages := b.buildOpenAIMessages(req)

	openaiReq := openaiRequest{
		Model:    lane.Model,
		Messages: messages,
		Stream:   true,
	}
	if req.Temperature > 0 {
		openaiReq.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("OpenAI error %d: %s", resp.StatusCode, string(respBody))
	}

	chunks := make(chan *ThinkChunk, 100)

	go func() {
		defer resp.Body.Close()
		defer close(chunks)

		reader := resp.Body
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					chunks <- &ThinkChunk{Error: err}
				}
				return
			}
			data := string(buf[:n])
			// Parse SSE events
			for _, line := range strings.Split(data, "\n") {
				if strings.HasPrefix(line, "data: ") {
					jsonData := strings.TrimPrefix(line, "data: ")
					if jsonData == "[DONE]" {
						chunks <- &ThinkChunk{Done: true}
						return
					}
					var openaiResp openaiResponse
					if err := json.Unmarshal([]byte(jsonData), &openaiResp); err != nil {
						continue
					}
					if len(openaiResp.Choices) > 0 {
						chunks <- &ThinkChunk{
							Content: openaiResp.Choices[0].Delta.Content,
							Done:    openaiResp.Choices[0].FinishReason == "stop",
						}
					}
				}
			}
		}
	}()

	return chunks, nil
}

// ============================================================================
// Anthropic Engine Implementation
// ============================================================================

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

func (b *EmbeddedBrain) anthropicGenerate(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (*ThinkResponse, error) {
	url := lane.URL
	if url == "" {
		url = "https://api.anthropic.com/v1/messages"
	}

	apiKey := b.getAPIKey(lane)
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Anthropic API key not configured (set api_key in lane config or ANTHROPIC_API_KEY env var)")
	}

	messages := []anthropicMessage{{Role: "user", Content: prompt}}

	maxTokens := 4096
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	anthropicReq := anthropicRequest{
		Model:     lane.Model,
		Messages:  messages,
		MaxTokens: maxTokens,
		Stream:    false,
	}
	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic error %d: %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, err
	}

	if len(anthropicResp.Content) == 0 {
		return nil, fmt.Errorf("Anthropic returned no content")
	}

	// Parse any tool calls from the response
	toolCalls, cleanedContent := ParseToolCalls(anthropicResp.Content[0].Text)

	return &ThinkResponse{
		Content:   cleanedContent,
		ToolCalls: toolCalls,
		Done:      true,
	}, nil
}

func (b *EmbeddedBrain) anthropicStream(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	// For now, use non-streaming and return as single chunk
	resp, err := b.anthropicGenerate(ctx, lane, prompt, req)
	if err != nil {
		return nil, err
	}

	chunks := make(chan *ThinkChunk, 1)
	go func() {
		defer close(chunks)
		chunks <- &ThinkChunk{Content: resp.Content, Done: true}
	}()
	return chunks, nil
}

// ============================================================================
// Groq Engine Implementation (OpenAI-compatible API)
// ============================================================================

func (b *EmbeddedBrain) groqGenerate(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (*ThinkResponse, error) {
	// Groq uses OpenAI-compatible API
	// Check for Groq API key first
	apiKey := b.getAPIKey(lane)
	if apiKey == "" {
		// Try GROQ_API_KEY env var as fallback
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Groq API key not configured (set api_key in lane config or GROQ_API_KEY env var)")
	}

	// Create a modified lane for OpenAI-compatible call
	groqLane := &config.Lane{
		Engine: lane.Engine,
		Model:  lane.Model,
		URL:    lane.URL,
		APIKey: apiKey,
	}
	if groqLane.URL == "" {
		groqLane.URL = "https://api.groq.com/openai/v1/chat/completions"
	}

	return b.openaiGenerate(ctx, groqLane, prompt, req)
}

func (b *EmbeddedBrain) groqStream(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	// Check for Groq API key first
	apiKey := b.getAPIKey(lane)
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Groq API key not configured (set api_key in lane config or GROQ_API_KEY env var)")
	}

	// Create a modified lane for OpenAI-compatible call
	groqLane := &config.Lane{
		Engine: lane.Engine,
		Model:  lane.Model,
		URL:    lane.URL,
		APIKey: apiKey,
	}
	if groqLane.URL == "" {
		groqLane.URL = "https://api.groq.com/openai/v1/chat/completions"
	}

	return b.openaiStream(ctx, groqLane, prompt, req)
}

// ============================================================================
// vLLM Engine Implementation (OpenAI-compatible API for local inference)
// ============================================================================
//
// vLLM provides high-performance LLM inference with:
// - Continuous batching for 5-10x throughput
// - PagedAttention for efficient memory usage
// - OpenAI-compatible API at /v1/chat/completions
//
// Configuration example:
//   lanes:
//     vllm:
//       engine: vllm
//       model: mistralai/Mistral-7B-Instruct-v0.2
//       url: http://192.168.1.186:8000
//
// No API key required for local deployments.

// vllmGenerate calls vLLM's OpenAI-compatible chat completions endpoint.
// Uses circuit breaker for fault tolerance and automatic fallback.
func (b *EmbeddedBrain) vllmGenerate(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (*ThinkResponse, error) {
	// Check circuit breaker
	cb := b.circuitBreakers.Get("vllm")
	if !cb.Allow() {
		// Circuit is open, try fallback
		return b.tryFallback(ctx, prompt, req, "vllm circuit open")
	}

	url := lane.URL
	if url == "" {
		url = "http://localhost:8000"
	}
	url += "/v1/chat/completions"

	// Build proper message array for chat completions
	messages := b.buildOpenAIMessages(req)

	vllmReq := openaiRequest{
		Model:    lane.Model,
		Messages: messages,
		Stream:   false,
	}
	if req.Temperature > 0 {
		vllmReq.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		vllmReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// vLLM may optionally have an API key for secured deployments
	apiKey := b.getAPIKey(lane)
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		cb.RecordFailure()
		// Try fallback on connection failure
		return b.tryFallback(ctx, prompt, req, fmt.Sprintf("vLLM request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		cb.RecordFailure()
		return b.tryFallback(ctx, prompt, req, fmt.Sprintf("vLLM error %d: %s", resp.StatusCode, string(respBody)))
	}

	var vllmResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&vllmResp); err != nil {
		cb.RecordFailure()
		return nil, fmt.Errorf("failed to decode vLLM response: %w", err)
	}

	if len(vllmResp.Choices) == 0 {
		cb.RecordFailure()
		return nil, fmt.Errorf("vLLM returned no choices")
	}

	// Success! Record it
	cb.RecordSuccess()

	// Parse any tool calls from the response
	toolCalls, cleanedContent := ParseToolCalls(vllmResp.Choices[0].Message.Content)

	return &ThinkResponse{
		Content:   cleanedContent,
		ToolCalls: toolCalls,
		Done:      true,
	}, nil
}

// tryFallback attempts to use a fallback lane when the primary fails.
func (b *EmbeddedBrain) tryFallback(ctx context.Context, prompt string, req *ThinkRequest, reason string) (*ThinkResponse, error) {
	// Try lanes in order: ollama (fast), groq, openai
	fallbackOrder := []string{"fast", "local", "smart"}

	for _, laneName := range fallbackOrder {
		lane, ok := b.cfg.Lanes[laneName]
		if !ok {
			continue
		}

		// Don't fall back to vLLM (that's what failed)
		if lane.Engine == "vllm" {
			continue
		}

		// Check this lane's circuit breaker too
		cb := b.circuitBreakers.Get(laneName)
		if !cb.Allow() {
			continue
		}

		fmt.Printf("[vLLM Fallback] %s → trying lane '%s' (%s)\n", reason, laneName, lane.Engine)

		switch lane.Engine {
		case "ollama":
			return b.ollamaGenerate(ctx, &lane, prompt, req)
		case "openai":
			return b.openaiGenerate(ctx, &lane, prompt, req)
		case "groq":
			return b.groqGenerate(ctx, &lane, prompt, req)
		case "anthropic":
			return b.anthropicGenerate(ctx, &lane, prompt, req)
		}
	}

	return nil, fmt.Errorf("vLLM failed (%s) and no fallback lanes available", reason)
}

// vllmStream calls vLLM's streaming chat completions endpoint.
// Uses circuit breaker for fault tolerance.
func (b *EmbeddedBrain) vllmStream(ctx context.Context, lane *config.Lane, prompt string, req *ThinkRequest) (<-chan *ThinkChunk, error) {
	// Check circuit breaker
	cb := b.circuitBreakers.Get("vllm")
	if !cb.Allow() {
		// Circuit is open, try fallback (non-streaming)
		resp, err := b.tryFallback(ctx, prompt, req, "vllm circuit open (streaming)")
		if err != nil {
			return nil, err
		}
		// Convert to streaming response
		chunks := make(chan *ThinkChunk, 1)
		go func() {
			defer close(chunks)
			chunks <- &ThinkChunk{Content: resp.Content, ToolCalls: resp.ToolCalls, Done: true}
		}()
		return chunks, nil
	}

	url := lane.URL
	if url == "" {
		url = "http://localhost:8000"
	}
	url += "/v1/chat/completions"

	// Build proper message array for chat completions
	messages := b.buildOpenAIMessages(req)

	vllmReq := openaiRequest{
		Model:    lane.Model,
		Messages: messages,
		Stream:   true,
	}
	if req.Temperature > 0 {
		vllmReq.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		vllmReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(vllmReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// vLLM may optionally have an API key
	apiKey := b.getAPIKey(lane)
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := b.client.Do(httpReq)
	if err != nil {
		cb.RecordFailure()
		// Try fallback (non-streaming for simplicity)
		fallbackResp, fallbackErr := b.tryFallback(ctx, prompt, req, fmt.Sprintf("vLLM stream failed: %v", err))
		if fallbackErr != nil {
			return nil, fmt.Errorf("vLLM request failed: %w (fallback also failed: %v)", err, fallbackErr)
		}
		chunks := make(chan *ThinkChunk, 1)
		go func() {
			defer close(chunks)
			chunks <- &ThinkChunk{Content: fallbackResp.Content, ToolCalls: fallbackResp.ToolCalls, Done: true}
		}()
		return chunks, nil
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cb.RecordFailure()
		return nil, fmt.Errorf("vLLM error %d: %s", resp.StatusCode, string(respBody))
	}

	chunks := make(chan *ThinkChunk, 100)

	go func() {
		defer resp.Body.Close()
		defer close(chunks)

		reader := resp.Body
		buf := make([]byte, 4096)
		hasReceivedData := false

		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					if !hasReceivedData {
						cb.RecordFailure()
					}
					chunks <- &ThinkChunk{Error: err}
				} else if hasReceivedData {
					cb.RecordSuccess()
				}
				return
			}

			hasReceivedData = true
			data := string(buf[:n])

			// Parse SSE events (same format as OpenAI)
			for _, line := range strings.Split(data, "\n") {
				if strings.HasPrefix(line, "data: ") {
					jsonData := strings.TrimPrefix(line, "data: ")
					if jsonData == "[DONE]" {
						cb.RecordSuccess()
						chunks <- &ThinkChunk{Done: true}
						return
					}
					var vllmResp openaiResponse
					if err := json.Unmarshal([]byte(jsonData), &vllmResp); err != nil {
						continue
					}
					if len(vllmResp.Choices) > 0 {
						chunks <- &ThinkChunk{
							Content: vllmResp.Choices[0].Delta.Content,
							Done:    vllmResp.Choices[0].FinishReason == "stop",
						}
					}
				}
			}
		}
	}()

	return chunks, nil
}

// vllmPing checks if the vLLM server is reachable.
func (b *EmbeddedBrain) vllmPing(ctx context.Context, lane *config.Lane) error {
	url := lane.URL
	if url == "" {
		url = "http://localhost:8000"
	}

	// vLLM exposes /health for health checks
	req, err := http.NewRequestWithContext(ctx, "GET", url+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("vLLM server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("vLLM server unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// GetVLLMModels lists available models from the vLLM server.
func (b *EmbeddedBrain) GetVLLMModels(laneName string) ([]string, error) {
	b.mu.RLock()
	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		b.mu.RUnlock()
		return nil, fmt.Errorf("lane not found: %s", laneName)
	}
	b.mu.RUnlock()

	if lane.Engine != "vllm" {
		return []string{lane.Model}, nil
	}

	url := lane.URL
	if url == "" {
		url = "http://localhost:8000"
	}

	resp, err := http.Get(url + "/v1/models")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch vLLM models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vLLM returned status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}
