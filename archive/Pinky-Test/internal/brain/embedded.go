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
	"github.com/normanking/pinky/internal/models"
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
	cfg            config.InferenceConfig
	client         *http.Client
	memoryStore    MemoryStore       // Optional persistent store
	memories       []Memory          // In-memory fallback
	mu             sync.RWMutex
	currentLane    string            // Current active lane (can be switched at runtime)
	autoLLM        bool              // Whether AutoLLM routing is enabled
	modelRegistry  *models.Registry  // Model provider registry
	configPath     string            // Path to config file for persistence
}

// LaneInfo provides information about an available lane
type LaneInfo struct {
	Name   string
	Engine string
	Model  string
	Active bool
}

// NewEmbeddedBrain creates a new embedded brain with local inference.
func NewEmbeddedBrain(cfg config.InferenceConfig) *EmbeddedBrain {
	return &EmbeddedBrain{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute, // LLM responses can be slow
		},
		memories:      make([]Memory, 0),
		currentLane:   cfg.DefaultLane,
		autoLLM:       cfg.AutoLLM,
		modelRegistry: models.NewRegistry(),
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

// SetModel updates the model for a specific lane and persists the change to config
func (b *EmbeddedBrain) SetModel(laneName, model string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		return fmt.Errorf("lane '%s' not found (available: %v)", laneName, b.GetLaneNames())
	}

	// Update the lane's model
	lane.Model = model
	b.cfg.Lanes[laneName] = lane

	// Persist to config
	return b.persistConfig()
}

// GetModelsForLane returns available models for a specific lane using the model registry
func (b *EmbeddedBrain) GetModelsForLane(ctx context.Context, laneName string) ([]models.ModelInfo, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		return nil, fmt.Errorf("lane '%s' not found (available: %v)", laneName, b.GetLaneNames())
	}

	if b.modelRegistry == nil {
		return nil, fmt.Errorf("model registry not initialized")
	}

	return b.modelRegistry.ListModels(ctx, lane.Engine)
}

// persistConfig saves the current configuration to the config file
func (b *EmbeddedBrain) persistConfig() error {
	// Build a full config from the inference config
	cfg := &config.Config{
		Version:   1,
		Inference: b.cfg,
	}

	return cfg.Save(b.configPath)
}

// SetConfigPath sets the path for config persistence
func (b *EmbeddedBrain) SetConfigPath(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.configPath = path
}

// selectLane chooses the appropriate lane based on request complexity (when AutoLLM is enabled)
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
		// Complex task: use smart lane if available
		if _, ok := b.cfg.Lanes["smart"]; ok {
			laneName = "smart"
		} else {
			laneName = b.currentLane
		}
	case complexity >= 0.3:
		// Medium task: use fast lane if available
		if _, ok := b.cfg.Lanes["fast"]; ok {
			laneName = "fast"
		} else {
			laneName = b.currentLane
		}
	default:
		// Simple task: use local lane if available
		if _, ok := b.cfg.Lanes["local"]; ok {
			laneName = "local"
		} else {
			laneName = b.currentLane
		}
	}

	lane, ok := b.cfg.Lanes[laneName]
	if !ok {
		return nil
	}
	return &lane
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
		memoryStore:   store,
		memories:      make([]Memory, 0),
		modelRegistry: models.NewRegistry(),
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

	url := b.getLaneURL(lane)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("inference backend unreachable: %w", err)
	}
	defer resp.Body.Close()

	return nil
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
	default:
		return nil, fmt.Errorf("unsupported engine: %s (supported: ollama, openai, anthropic, groq)", lane.Engine)
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
	default:
		return nil, fmt.Errorf("unsupported engine: %s (supported: ollama, openai, anthropic, groq)", lane.Engine)
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

// buildPrompt constructs the prompt from messages and persona.
func (b *EmbeddedBrain) buildPrompt(req *ThinkRequest) string {
	var parts []string

	// Add system prompt from persona
	if req.Persona != nil && req.Persona.SystemPrompt != "" {
		parts = append(parts, req.Persona.SystemPrompt)
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
			parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
		case "system":
			parts = append(parts, msg.Content)
		}
	}

	return strings.Join(parts, "\n\n")
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

	return &ThinkResponse{
		Content: ollamaResp.Response,
		Done:    true,
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

	messages := []openaiMessage{{Role: "user", Content: prompt}}

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

	return &ThinkResponse{
		Content: openaiResp.Choices[0].Message.Content,
		Done:    true,
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

	messages := []openaiMessage{{Role: "user", Content: prompt}}

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

	return &ThinkResponse{
		Content: anthropicResp.Content[0].Text,
		Done:    true,
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
