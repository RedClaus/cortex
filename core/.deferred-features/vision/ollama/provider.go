// Package ollama implements vision.Provider using Ollama's vision models.
// CR-005: Visual Cortex - Ollama Vision Provider Implementation
//
// Supports two models:
// - Moondream2 (Fast Lane): Quick classification, ~500ms, CPU-friendly
// - MiniCPM-V 2.6 (Smart Lane): OCR/code analysis, 2-4s, GPU recommended
//
// Local-First: All requests go to local Ollama instance, images never leave machine.
package ollama

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

// Provider implements vision.Provider using Ollama's vision models.
type Provider struct {
	baseURL      string
	modelName    string
	httpClient   *http.Client
	healthy      atomic.Bool
	capabilities []vision.Capability
	role         string // "fast" or "smart" for logging
}

// NewMoondreamProvider creates a provider for Moondream (Fast Lane).
// Hot Path: Optimized for speed (<500ms), CPU-friendly.
//
// Moondream2 excels at:
// - Quick image classification
// - Simple descriptions
// - Yes/no questions about images
func NewMoondreamProvider(baseURL string) *Provider {
	return &Provider{
		baseURL:   baseURL,
		modelName: "moondream",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		capabilities: []vision.Capability{
			vision.CapabilityClassification,
			vision.CapabilityDescription,
		},
		role: "fast",
	}
}

// NewMiniCPMProvider creates a provider for MiniCPM-V 2.6 (Smart Lane).
// Cold Path: Optimized for accuracy, especially OCR and code analysis.
//
// MiniCPM-V 2.6 excels at:
// - OCR (especially small terminal fonts)
// - Code screenshot analysis
// - Chart/graph data extraction
// - K8s/monitoring dashboard reading
// - Complex visual reasoning
func NewMiniCPMProvider(baseURL string) *Provider {
	return &Provider{
		baseURL:   baseURL,
		modelName: "minicpm-v",
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Longer timeout for complex analysis
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
// Use this for testing or alternative vision models.
func NewProvider(baseURL, modelName string, timeout time.Duration, caps []vision.Capability) *Provider {
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

// Analyze sends an image to Ollama for analysis.
// The image is base64-encoded as required by Ollama's API.
func (p *Provider) Analyze(ctx context.Context, req *vision.AnalyzeRequest) (*vision.AnalyzeResponse, error) {
	start := time.Now()

	// Encode image to base64 (Ollama requirement)
	imageB64 := base64.StdEncoding.EncodeToString(req.Image)

	// Build Ollama request
	// YAGNI: Direct struct construction, no request builder pattern
	ollamaReq := OllamaGenerateRequest{
		Model:  p.modelName,
		Prompt: req.Prompt,
		Images: []string{imageB64},
		Stream: false, // We want complete response
		Options: OllamaOptions{
			Temperature: 0.1, // Low temperature for accurate OCR
		},
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	log.Debug().
		Str("model", p.modelName).
		Str("role", p.role).
		Int("image_size", len(req.Image)).
		Str("prompt", truncate(req.Prompt, 50)).
		Msg("sending vision request to Ollama")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		p.healthy.Store(false)
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		p.healthy.Store(false)

		// Check for common errors
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("%w: model '%s' not found - run 'ollama pull %s'",
				vision.ErrModelNotLoaded, p.modelName, p.modelName)
		}

		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var ollamaResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	p.healthy.Store(true)
	processingMs := time.Since(start).Milliseconds()

	log.Debug().
		Str("model", p.modelName).
		Str("role", p.role).
		Int64("processing_ms", processingMs).
		Int("tokens_used", ollamaResp.PromptEvalCount+ollamaResp.EvalCount).
		Msg("vision analysis completed")

	return &vision.AnalyzeResponse{
		Content:    ollamaResp.Response,
		Provider:   p.modelName,
		TokensUsed: ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}, nil
}

// IsHealthy checks if the model is loaded in Ollama.
// Uses /api/show endpoint for a quick check without inference.
func (p *Provider) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if model is available via /api/show
	showReq := OllamaShowRequest{Name: p.modelName}
	body, err := json.Marshal(showReq)
	if err != nil {
		p.healthy.Store(false)
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/show", bytes.NewReader(body))
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
			Msg("vision health check failed")
		return false
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode == http.StatusOK
	p.healthy.Store(healthy)

	if !healthy {
		log.Debug().
			Int("status", resp.StatusCode).
			Str("model", p.modelName).
			Msg("vision model not available")
	}

	return healthy
}

// OllamaGenerateRequest represents a request to Ollama's /api/generate endpoint.
type OllamaGenerateRequest struct {
	Model   string        `json:"model"`
	Prompt  string        `json:"prompt"`
	Images  []string      `json:"images,omitempty"` // Base64-encoded images
	Stream  bool          `json:"stream"`
	Options OllamaOptions `json:"options,omitempty"`
}

// OllamaOptions contains generation options.
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // Max tokens
	TopK        int     `json:"top_k,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// OllamaGenerateResponse represents a response from Ollama's /api/generate endpoint.
type OllamaGenerateResponse struct {
	Model           string `json:"model"`
	Response        string `json:"response"`
	Done            bool   `json:"done"`
	Context         []int  `json:"context,omitempty"`
	TotalDuration   int64  `json:"total_duration"`
	LoadDuration    int64  `json:"load_duration"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	EvalDuration    int64  `json:"eval_duration"`
}

// OllamaShowRequest represents a request to Ollama's /api/show endpoint.
type OllamaShowRequest struct {
	Name string `json:"name"`
}

// truncate truncates a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
