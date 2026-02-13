package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TimeoutConfig defines the 3-phase timeout system for Ollama.
// Phase 1 (Connection): Time to establish HTTP connection and send headers
// Phase 2 (First Token): Time to receive first token after request sent (model loading happens here)
// Phase 3 (Streaming): Max time between tokens during response streaming
type TimeoutConfig struct {
	ConnectionTimeout time.Duration // Time to establish HTTP connection (default: 30s)
	FirstTokenTimeout time.Duration // Time to receive first token after connection (default: 120s for cold start)
	StreamIdleTimeout time.Duration // Max time between tokens during streaming (default: 30s, detects stalled streams)
}

// DefaultTimeoutConfig returns sensible defaults for Ollama timeouts.
// These defaults are tuned for local connections with cold start scenarios.
// Cold start (model loading) can take 30-90+ seconds depending on model size and hardware.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ConnectionTimeout: 30 * time.Second,  // Connection establishment
		FirstTokenTimeout: 120 * time.Second, // Increased: cold start model loading can take 60-90s
		StreamIdleTimeout: 30 * time.Second,  // Increased: allows for slower token generation
	}
}

// RemoteTimeoutConfig returns timeout configuration optimized for remote Ollama servers.
// Remote servers may need longer timeouts due to:
// - Network latency
// - Model loading on cold start (can take 60-180+ seconds for large models like 70B)
// - Multiple users sharing the same server (queued requests)
// - Network jitter and packet loss
func RemoteTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		ConnectionTimeout: 60 * time.Second,  // Allow for network latency
		FirstTokenTimeout: 300 * time.Second, // 5 minutes for cold start + queue time
		StreamIdleTimeout: 60 * time.Second,  // More lenient for network jitter
	}
}

// isRemoteEndpoint checks if the Ollama endpoint is a remote server (not localhost).
func isRemoteEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false // Assume local if can't parse
	}
	host := u.Hostname()
	// Local addresses
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}
	// Local Docker/container addresses
	if host == "host.docker.internal" || host == "docker.for.mac.localhost" {
		return false
	}
	// Any other address is considered remote
	return true
}

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	config        *ProviderConfig
	client        *http.Client
	timeoutConfig TimeoutConfig
}

// OllamaOption is a functional option for configuring OllamaProvider.
type OllamaOption func(*OllamaProvider)

// WithTimeoutConfig sets custom timeout configuration for the Ollama provider.
func WithTimeoutConfig(cfg TimeoutConfig) OllamaOption {
	return func(p *OllamaProvider) {
		p.timeoutConfig = cfg
		// Update transport timeout to match
		if transport, ok := p.client.Transport.(*http.Transport); ok {
			transport.ResponseHeaderTimeout = cfg.ConnectionTimeout
		}
	}
}

// WithConnectionTimeout sets the connection timeout.
func WithConnectionTimeout(d time.Duration) OllamaOption {
	return func(p *OllamaProvider) {
		p.timeoutConfig.ConnectionTimeout = d
		if transport, ok := p.client.Transport.(*http.Transport); ok {
			transport.ResponseHeaderTimeout = d
		}
	}
}

// WithFirstTokenTimeout sets the first token (cold start) timeout.
func WithFirstTokenTimeout(d time.Duration) OllamaOption {
	return func(p *OllamaProvider) {
		p.timeoutConfig.FirstTokenTimeout = d
	}
}

// WithStreamIdleTimeout sets the streaming idle timeout.
func WithStreamIdleTimeout(d time.Duration) OllamaOption {
	return func(p *OllamaProvider) {
		p.timeoutConfig.StreamIdleTimeout = d
	}
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(cfg *ProviderConfig, opts ...OllamaOption) *OllamaProvider {
	if cfg == nil {
		cfg = DefaultConfig("ollama")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://127.0.0.1:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "llama3"
	}

	// Use 3-phase timeout system instead of hard timeout
	// Select timeout config based on whether this is a remote endpoint
	var timeoutConfig TimeoutConfig
	if isRemoteEndpoint(cfg.Endpoint) {
		timeoutConfig = RemoteTimeoutConfig()
	} else {
		timeoutConfig = DefaultTimeoutConfig()
	}

	p := &OllamaProvider{
		config:        cfg,
		timeoutConfig: timeoutConfig,
		client: &http.Client{
			// IMPORTANT: Do NOT set http.Client.Timeout here!
			// Client.Timeout applies to the ENTIRE request lifecycle including body reading.
			// For streaming responses, this causes "context deadline exceeded" errors
			// because the timeout fires while we're still reading the streaming response.
			//
			// Instead, we rely on:
			// 1. ResponseHeaderTimeout for connection + model loading (cold start)
			// 2. FirstTokenTimeout for waiting for the model to start responding (cold start handling)
			// 3. StreamIdleTimeout for detecting stalled streams during token generation
			//
			// This 3-phase approach allows long cold starts while still detecting hangs.
			Transport: &http.Transport{
				// ResponseHeaderTimeout: Time to receive first response headers from Ollama.
				// This MUST be long enough for model loading (cold start can take 60-120s).
				// Using FirstTokenTimeout here since headers arrive when model starts responding.
				ResponseHeaderTimeout: timeoutConfig.FirstTokenTimeout, // Time to receive response headers (includes model loading)
				IdleConnTimeout:       90 * time.Second,                // Keep-alive idle timeout
				TLSHandshakeTimeout:   10 * time.Second,                // TLS negotiation
			},
		},
	}

	// Apply functional options
	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Available checks if Ollama is running and has at least one model.
// An Ollama endpoint with 0 models is not useful as a backend.
func (p *OllamaProvider) Available() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Check if Ollama has at least one model
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	return len(result.Models) > 0
}

// Chat sends a chat request to Ollama using streaming with 3-phase timeout monitoring.
// This provides better timeout handling: connection (30s), first-token (60s), and idle (15s).
func (p *OllamaProvider) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	start := time.Now()

	// Build Ollama request with streaming enabled
	ollamaReq := ollamaChatRequest{
		Model:  req.Model,
		Stream: true, // Use streaming for better timeout control
	}

	if ollamaReq.Model == "" {
		ollamaReq.Model = p.config.Model
	}

	// Convert messages
	for _, msg := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add system prompt as first message if provided
	if req.SystemPrompt != "" {
		ollamaReq.Messages = append([]ollamaMessage{{
			Role:    "system",
			Content: req.SystemPrompt,
		}}, ollamaReq.Messages...)
	}

	// Set options
	ollamaReq.Options.Temperature = req.Temperature
	if ollamaReq.Options.Temperature == 0 {
		ollamaReq.Options.Temperature = p.config.Temperature
	}
	ollamaReq.Options.NumPredict = req.MaxTokens
	if ollamaReq.Options.NumPredict == 0 {
		ollamaReq.Options.NumPredict = p.config.MaxTokens
	}

	// Marshal request
	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request with connection timeout
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request (connection timeout handled by client.Timeout)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// Handle streaming response with TTFT and idle timeout monitoring
	return p.handleStreamingResponse(ctx, resp.Body, start)
}

// handleStreamingResponse processes Ollama's streaming response with TTFT and idle timeout monitoring.
// It implements the 3-phase timeout system:
// - Phase 1 (connection): Already handled by http.Client.Timeout
// - Phase 2 (first-token): Times out if first token not received within FirstTokenTimeout
// - Phase 3 (streaming): Times out if gap between tokens exceeds StreamIdleTimeout
func (p *OllamaProvider) handleStreamingResponse(ctx context.Context, body io.Reader, start time.Time) (*ChatResponse, error) {
	type streamChunk struct {
		chunk ollamaChatResponse
		err   error
	}

	// Channel for receiving stream chunks
	chunkChan := make(chan streamChunk, 1)

	// Start goroutine to read stream with context awareness
	go func() {
		defer close(chunkChan)
		decoder := json.NewDecoder(body)
		for {
			var chunk ollamaChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err != io.EOF {
					// Goroutine-safe send: check context before blocking on channel
					select {
					case <-ctx.Done():
						return // Clean exit on cancellation
					case chunkChan <- streamChunk{err: err}:
					}
				}
				return
			}
			// Goroutine-safe send: check context before blocking on channel
			select {
			case <-ctx.Done():
				return // Clean exit on cancellation
			case chunkChan <- streamChunk{chunk: chunk}:
			}
			if chunk.Done {
				return
			}
		}
	}()

	// Accumulate response with size limit to prevent memory exhaustion
	var fullContent strings.Builder
	var totalBytes int64
	var modelName string
	var promptTokens, completionTokens int
	firstTokenReceived := false
	firstTokenTimer := time.NewTimer(p.timeoutConfig.FirstTokenTimeout)
	defer firstTokenTimer.Stop()

	var idleTimer *time.Timer

	for {
		var timeout <-chan time.Time
		if !firstTokenReceived {
			// Phase 2: Waiting for first token
			timeout = firstTokenTimer.C
		} else if idleTimer != nil {
			// Phase 3: Monitoring idle time between tokens
			timeout = idleTimer.C
		}

		select {
		case <-ctx.Done():
			// Context cancelled - clean exit
			return nil, ctx.Err()

		case chunk, ok := <-chunkChan:
			if !ok {
				// Channel closed, streaming complete
				if modelName == "" {
					return nil, fmt.Errorf("empty response from Ollama")
				}
				return &ChatResponse{
					Content:          fullContent.String(),
					Model:            modelName,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TokensUsed:       promptTokens + completionTokens,
					Duration:         time.Since(start),
					FinishReason:     "stop",
				}, nil
			}

			if chunk.err != nil {
				return nil, fmt.Errorf("decode stream chunk: %w", chunk.err)
			}

			// First token received
			if !firstTokenReceived {
				firstTokenReceived = true
				firstTokenTimer.Stop()
				// Initialize idle timer for subsequent tokens
				idleTimer = time.NewTimer(p.timeoutConfig.StreamIdleTimeout)
				defer idleTimer.Stop()
			} else if idleTimer != nil {
				// Reset idle timer on each token
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(p.timeoutConfig.StreamIdleTimeout)
			}

			// Accumulate content with size limit check
			if chunk.chunk.Message.Content != "" {
				contentLen := int64(len(chunk.chunk.Message.Content))
				if totalBytes+contentLen > MaxStreamedResponseSize {
					return nil, fmt.Errorf("response size exceeded limit (%d bytes) - possible runaway generation", MaxStreamedResponseSize)
				}
				totalBytes += contentLen
				fullContent.WriteString(chunk.chunk.Message.Content)
			}

			// Store metadata from final chunk
			if chunk.chunk.Done {
				modelName = chunk.chunk.Model
				promptTokens = chunk.chunk.PromptEvalCount
				completionTokens = chunk.chunk.EvalCount
			} else if modelName == "" {
				// Store model name from first chunk
				modelName = chunk.chunk.Model
			}

		case <-timeout:
			if !firstTokenReceived {
				return nil, fmt.Errorf("timeout waiting for first token (waited %v, limit %v) - model may be loading or request stalled",
					time.Since(start), p.timeoutConfig.FirstTokenTimeout)
			}
			return nil, fmt.Errorf("stream idle timeout (no token received for %v) - model appears to have stalled",
				p.timeoutConfig.StreamIdleTimeout)
		}
	}
}

// Ollama API types
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
	Format   string          `json:"format,omitempty"`
	Tools    []OllamaToolDef `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []OllamaToolCall `json:"tool_calls,omitempty"`
}

type OllamaToolDef struct {
	Type     string            `json:"type"`
	Function OllamaFunctionDef `json:"function"`
}

type OllamaFunctionDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type OllamaToolCall struct {
	Function OllamaFunctionCall `json:"function"`
}

type OllamaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaGenerateRequest is used for raw completions with grammar support.
type ollamaGenerateRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	System  string        `json:"system,omitempty"`
	Stream  bool          `json:"stream"`
	Options ollamaOptions `json:"options,omitempty"`
	Grammar string        `json:"grammar,omitempty"` // GBNF grammar for constrained generation
	Format  string        `json:"format,omitempty"`  // "json" for JSON mode
}

type ollamaGenerateResponse struct {
	Model           string `json:"model"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

type ollamaChatResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

// ChatWithTools sends a chat request with native tool calling support.
func (p *OllamaProvider) ChatWithTools(ctx context.Context, req *ChatRequest, tools []OllamaToolDef) (*ChatResponse, error) {
	start := time.Now()

	ollamaReq := ollamaChatRequest{
		Model:  req.Model,
		Stream: true,
		Tools:  tools,
	}

	if ollamaReq.Model == "" {
		ollamaReq.Model = p.config.Model
	}

	for _, msg := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if req.SystemPrompt != "" {
		ollamaReq.Messages = append([]ollamaMessage{{
			Role:    "system",
			Content: req.SystemPrompt,
		}}, ollamaReq.Messages...)
	}

	ollamaReq.Options.Temperature = req.Temperature
	if ollamaReq.Options.Temperature == 0 {
		ollamaReq.Options.Temperature = p.config.Temperature
	}
	ollamaReq.Options.NumPredict = req.MaxTokens
	if ollamaReq.Options.NumPredict == 0 {
		ollamaReq.Options.NumPredict = p.config.MaxTokens
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.Endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return p.handleStreamingResponseWithTools(ctx, resp.Body, start)
}

func (p *OllamaProvider) handleStreamingResponseWithTools(ctx context.Context, body io.Reader, start time.Time) (*ChatResponse, error) {
	type streamChunk struct {
		chunk ollamaChatResponse
		err   error
	}

	chunkChan := make(chan streamChunk, 1)

	// Start goroutine to read stream with context awareness
	go func() {
		defer close(chunkChan)
		decoder := json.NewDecoder(body)
		for {
			var chunk ollamaChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err != io.EOF {
					// Goroutine-safe send: check context before blocking on channel
					select {
					case <-ctx.Done():
						return // Clean exit on cancellation
					case chunkChan <- streamChunk{err: err}:
					}
				}
				return
			}
			// Goroutine-safe send: check context before blocking on channel
			select {
			case <-ctx.Done():
				return // Clean exit on cancellation
			case chunkChan <- streamChunk{chunk: chunk}:
			}
			if chunk.Done {
				return
			}
		}
	}()

	var fullContent strings.Builder
	var modelName string
	var promptTokens, completionTokens int
	var toolCalls []OllamaToolCall
	firstTokenReceived := false
	firstTokenTimer := time.NewTimer(p.timeoutConfig.FirstTokenTimeout)
	defer firstTokenTimer.Stop()

	var idleTimer *time.Timer

	for {
		var timeout <-chan time.Time
		if !firstTokenReceived {
			timeout = firstTokenTimer.C
		} else if idleTimer != nil {
			timeout = idleTimer.C
		}

		select {
		case <-ctx.Done():
			// Context cancelled - clean exit
			return nil, ctx.Err()

		case chunk, ok := <-chunkChan:
			if !ok {
				if modelName == "" {
					return nil, fmt.Errorf("empty response from Ollama")
				}
				chatResp := &ChatResponse{
					Content:          fullContent.String(),
					Model:            modelName,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TokensUsed:       promptTokens + completionTokens,
					Duration:         time.Since(start),
					FinishReason:     "stop",
				}
				if len(toolCalls) > 0 {
					chatResp.FinishReason = "tool_calls"
					chatResp.ToolCalls = convertToolCalls(toolCalls)
				}
				return chatResp, nil
			}

			if chunk.err != nil {
				return nil, fmt.Errorf("decode stream chunk: %w", chunk.err)
			}

			if !firstTokenReceived {
				firstTokenReceived = true
				firstTokenTimer.Stop()
				idleTimer = time.NewTimer(p.timeoutConfig.StreamIdleTimeout)
				defer idleTimer.Stop()
			} else if idleTimer != nil {
				if !idleTimer.Stop() {
					select {
					case <-idleTimer.C:
					default:
					}
				}
				idleTimer.Reset(p.timeoutConfig.StreamIdleTimeout)
			}

			if chunk.chunk.Message.Content != "" {
				fullContent.WriteString(chunk.chunk.Message.Content)
			}

			if len(chunk.chunk.Message.ToolCalls) > 0 {
				toolCalls = append(toolCalls, chunk.chunk.Message.ToolCalls...)
			}

			if chunk.chunk.Done {
				modelName = chunk.chunk.Model
				promptTokens = chunk.chunk.PromptEvalCount
				completionTokens = chunk.chunk.EvalCount
			} else if modelName == "" {
				modelName = chunk.chunk.Model
			}

		case <-timeout:
			if !firstTokenReceived {
				return nil, fmt.Errorf("timeout waiting for first token (waited %v, limit %v)",
					time.Since(start), p.timeoutConfig.FirstTokenTimeout)
			}
			return nil, fmt.Errorf("stream idle timeout (no token for %v)",
				p.timeoutConfig.StreamIdleTimeout)
		}
	}
}

func convertToolCalls(calls []OllamaToolCall) []ToolCallResult {
	result := make([]ToolCallResult, len(calls))
	for i, call := range calls {
		result[i] = ToolCallResult{
			Name:      call.Function.Name,
			Arguments: string(call.Function.Arguments),
		}
	}
	return result
}

// OllamaModel represents a model available on an Ollama server.
type OllamaModel struct {
	Name       string `json:"name"`
	Model      string `json:"model"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// ollamaTagsResponse represents the /api/tags response.
type ollamaTagsResponse struct {
	Models []OllamaModel `json:"models"`
}

// Warmup sends a minimal request to pre-load the model into memory.
// This is useful to avoid cold start latency on the first real request.
// Call this during initialization if you want faster first responses.
// The warmup uses the model's configured default or falls back to the provider's model.
func (p *OllamaProvider) Warmup(ctx context.Context) error {
	// Use a minimal prompt to just load the model
	req := &ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 1, // Minimize response time
	}

	// Create a timeout context for warmup (use FirstTokenTimeout since that's the cold start phase)
	warmupCtx, cancel := context.WithTimeout(ctx, p.timeoutConfig.FirstTokenTimeout)
	defer cancel()

	_, err := p.Chat(warmupCtx, req)
	if err != nil {
		return fmt.Errorf("warmup failed: %w", err)
	}
	return nil
}

// WarmupAsync starts model warmup in the background.
// It returns immediately and does not block. Errors are logged but not returned.
// This is useful for pre-loading the model before the user's first query,
// avoiding 30-90+ second cold start delays.
func (p *OllamaProvider) WarmupAsync(ctx context.Context) {
	go func() {
		start := time.Now()
		if err := p.Warmup(ctx); err != nil {
			// Log but don't fail - warmup is optional optimization
			// The first real request will just experience cold start latency
			// Note: In production, logging would go to file to avoid TUI corruption
		} else {
			duration := time.Since(start)
			// Warmup succeeded - model is now loaded and ready
			_ = duration // Duration info available for debugging if needed
		}
	}()
}

// SetTimeoutConfig allows overriding the timeout configuration after creation.
// This is useful for adjusting timeouts based on runtime conditions.
func (p *OllamaProvider) SetTimeoutConfig(cfg TimeoutConfig) {
	p.timeoutConfig = cfg
	// Update transport timeout
	if transport, ok := p.client.Transport.(*http.Transport); ok {
		transport.ResponseHeaderTimeout = cfg.ConnectionTimeout
	}
}

// FetchOllamaModels fetches the list of models from an Ollama server at the given endpoint.
// This is a standalone function that can be used without creating a full provider.
func FetchOllamaModels(endpoint string) ([]OllamaModel, error) {
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Ollama at %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readLimitedBody(resp.Body, MaxErrorBodySize)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return tagsResp.Models, nil
}
