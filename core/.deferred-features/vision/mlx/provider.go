// Package mlx implements vision.Provider using MLX-LM vision models.
// CR-023: CortexEyes - MLX Vision Provider for Apple Silicon
//
// Supports vision models via MLX-LM's OpenAI-compatible API:
// - Qwen2-VL-2B (Fast Lane): Quick analysis, ~1-2s on M-series
// - Qwen2-VL-7B (Smart Lane): More capable, 3-5s on M-series
//
// Local-First: All requests go to local MLX-LM server, images never leave machine.
// Apple Silicon Optimized: 5-10x faster than CPU inference.
package mlx

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/normanking/cortex/internal/vision"
)

// Provider implements vision.Provider using MLX-LM vision models.
type Provider struct {
	baseURL        string
	modelName      string
	httpClient     *http.Client
	healthy        atomic.Bool
	verified       atomic.Bool  // True if we've successfully made an inference call
	lastCheck      atomic.Int64 // Unix timestamp of last health check
	failureCount   atomic.Int32 // Consecutive failures (for backoff)
	capabilities   []vision.Capability
	role           string // "fast" or "smart" for logging
}

const (
	healthCheckInterval = 60  // Seconds between health checks
	maxFailuresBeforeDisable = 5  // Disable after N consecutive failures
)

// NewQwen2VLFastProvider creates a provider for Qwen2-VL-2B (Fast Lane).
// Optimized for speed on Apple Silicon (~1-2s).
func NewQwen2VLFastProvider(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081"
	}
	return &Provider{
		baseURL:   baseURL,
		modelName: "mlx-community/Qwen2-VL-2B-Instruct-4bit",
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		capabilities: []vision.Capability{
			vision.CapabilityClassification,
			vision.CapabilityDescription,
			vision.CapabilityOCR,
		},
		role: "fast",
	}
}

// NewQwen2VLSmartProvider creates a provider for Qwen2-VL-7B (Smart Lane).
// More capable but slower (~3-5s on Apple Silicon).
func NewQwen2VLSmartProvider(baseURL string) *Provider {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081"
	}
	return &Provider{
		baseURL:   baseURL,
		modelName: "mlx-community/Qwen2-VL-7B-Instruct-4bit",
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		capabilities: []vision.Capability{
			vision.CapabilityOCR,
			vision.CapabilityCodeAnalysis,
			vision.CapabilityChartReading,
			vision.CapabilityDescription,
			vision.CapabilityDashboard,
		},
		role: "smart",
	}
}

// NewProvider creates a provider with a custom model name.
func NewProvider(baseURL, modelName string, timeout time.Duration, caps []vision.Capability) *Provider {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081"
	}
	return &Provider{
		baseURL:   baseURL,
		modelName: modelName,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		capabilities: caps,
		role:         "custom",
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return p.modelName
}

// Capabilities returns what this provider excels at.
func (p *Provider) Capabilities() []vision.Capability {
	return p.capabilities
}

// Analyze sends an image to MLX-LM for analysis using OpenAI-compatible API.
func (p *Provider) Analyze(ctx context.Context, req *vision.AnalyzeRequest) (*vision.AnalyzeResponse, error) {
	// Check if we should skip due to too many failures
	if p.failureCount.Load() >= maxFailuresBeforeDisable {
		// Check if enough time has passed to retry
		lastCheck := p.lastCheck.Load()
		if time.Now().Unix()-lastCheck < int64(healthCheckInterval) {
			return nil, fmt.Errorf("MLX vision provider disabled after %d failures (will retry in %ds)",
				maxFailuresBeforeDisable, healthCheckInterval-(int(time.Now().Unix()-lastCheck)))
		}
		// Reset for retry
		p.failureCount.Store(0)
		p.lastCheck.Store(time.Now().Unix())
	}

	start := time.Now()

	// Encode image to base64 data URL format
	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = "image/png"
	}
	imageDataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(req.Image))

	// Build OpenAI-compatible request with vision
	mlxReq := OpenAIChatRequest{
		Model: p.modelName,
		Messages: []OpenAIMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "image_url",
						ImageURL: &ImageURL{
							URL: imageDataURL,
						},
					},
					{
						Type: "text",
						Text: req.Prompt,
					},
				},
			},
		},
		MaxTokens:   1024,
		Temperature: 0.1, // Low temperature for accurate analysis
	}

	body, err := json.Marshal(mlxReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	log.Debug().
		Str("model", p.modelName).
		Str("role", p.role).
		Int("image_size", len(req.Image)).
		Str("prompt", truncate(req.Prompt, 50)).
		Msg("sending vision request to MLX-LM")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		p.failureCount.Add(1)
		return nil, fmt.Errorf("MLX-LM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		p.healthy.Store(false)
		p.failureCount.Add(1)

		// Provide actionable error message for 404
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("MLX vision model not served (404): server may be running a different model. Start vision server with: python -m mlx_lm.server --model %s --port 8082", p.modelName)
		}
		return nil, fmt.Errorf("MLX-LM error %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var mlxResp OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&mlxResp); err != nil {
		p.failureCount.Add(1)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Success! Reset failure count and mark as verified
	p.healthy.Store(true)
	p.verified.Store(true)
	p.failureCount.Store(0)
	processingMs := time.Since(start).Milliseconds()

	// Extract content from response
	content := ""
	if len(mlxResp.Choices) > 0 {
		content = mlxResp.Choices[0].Message.Content
	}

	tokensUsed := mlxResp.Usage.PromptTokens + mlxResp.Usage.CompletionTokens

	log.Debug().
		Str("model", p.modelName).
		Str("role", p.role).
		Int64("processing_ms", processingMs).
		Int("tokens_used", tokensUsed).
		Msg("MLX vision analysis completed")

	return &vision.AnalyzeResponse{
		Content:     content,
		Provider:    p.modelName,
		TokensUsed:  tokensUsed,
		ProcessedMs: processingMs,
	}, nil
}

// IsHealthy checks if the model is available AND can serve requests.
// This does a real inference test, not just a model listing check.
func (p *Provider) IsHealthy() bool {
	// If we've already verified this provider works, trust the cached state
	if p.verified.Load() && p.healthy.Load() {
		return true
	}

	// If too many failures, don't bother checking
	if p.failureCount.Load() >= maxFailuresBeforeDisable {
		lastCheck := p.lastCheck.Load()
		if time.Now().Unix()-lastCheck < int64(healthCheckInterval) {
			return false
		}
	}

	// Do a lightweight inference test with a tiny text prompt
	// This verifies the model is actually loaded and serving
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build a minimal text-only request to test the endpoint
	testReq := OpenAIChatRequest{
		Model: p.modelName,
		Messages: []OpenAIMessage{
			{
				Role: "user",
				Content: []ContentPart{
					{
						Type: "text",
						Text: "hi",
					},
				},
			},
		},
		MaxTokens:   1,
		Temperature: 0,
	}

	body, err := json.Marshal(testReq)
	if err != nil {
		p.healthy.Store(false)
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		p.healthy.Store(false)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		p.healthy.Store(false)
		log.Debug().
			Err(err).
			Str("model", p.modelName).
			Msg("MLX vision health check failed - server unreachable")
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		p.healthy.Store(false)
		log.Debug().
			Str("model", p.modelName).
			Str("url", p.baseURL).
			Msg("MLX vision health check failed - model not served (404)")
		return false
	}

	if resp.StatusCode != http.StatusOK {
		p.healthy.Store(false)
		respBody, _ := io.ReadAll(resp.Body)
		log.Debug().
			Str("model", p.modelName).
			Int("status", resp.StatusCode).
			Str("body", string(respBody)).
			Msg("MLX vision health check failed")
		return false
	}

	// Successfully got a response - model is available and serving
	p.healthy.Store(true)
	p.verified.Store(true)
	p.lastCheck.Store(time.Now().Unix())
	log.Debug().
		Str("model", p.modelName).
		Msg("MLX vision health check passed")
	return true
}

// IsVerified returns true if this provider has successfully completed at least one inference.
func (p *Provider) IsVerified() bool {
	return p.verified.Load()
}

// FailureCount returns the current consecutive failure count.
func (p *Provider) FailureCount() int32 {
	return p.failureCount.Load()
}

// OpenAI-compatible API types for MLX-LM

type OpenAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string        `json:"role"`
	Content []ContentPart `json:"content"`
}

type ContentPart struct {
	Type     string    `json:"type"` // "text" or "image_url"
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", or "auto"
}

type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// truncate truncates a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
