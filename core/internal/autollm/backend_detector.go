package autollm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// BACKEND DETECTOR
// ═══════════════════════════════════════════════════════════════════════════════

// BackendType represents an LLM backend type.
type BackendType string

const (
	BackendMLX    BackendType = "mlx"    // MLX-LM server (fastest on Apple Silicon)
	BackendOllama BackendType = "ollama" // Ollama (easy setup, moderate speed)
	BackendDnet   BackendType = "dnet"   // dnet (distributed inference)
	BackendNone   BackendType = "none"
)

// BackendInfo contains information about a detected backend.
type BackendInfo struct {
	Type        BackendType
	Endpoint    string
	Available   bool
	ResponseMs  int64  // Latency from health check
	Models      []string
	Error       string
}

// BackendSelection contains the result of backend detection.
type BackendSelection struct {
	Primary     *BackendInfo
	Secondary   *BackendInfo
	Reason      string
}

// BackendDetector detects and benchmarks available LLM backends.
type BackendDetector struct {
	mlxEndpoint    string
	ollamaEndpoint string
	dnetEndpoint   string
	httpClient     *http.Client
}

// NewBackendDetector creates a new backend detector.
func NewBackendDetector(ollamaEndpoint, dnetEndpoint string) *BackendDetector {
	return NewBackendDetectorWithMLX("", ollamaEndpoint, dnetEndpoint)
}

// NewBackendDetectorWithMLX creates a backend detector with explicit MLX endpoint.
func NewBackendDetectorWithMLX(mlxEndpoint, ollamaEndpoint, dnetEndpoint string) *BackendDetector {
	if mlxEndpoint == "" {
		mlxEndpoint = "http://127.0.0.1:8081" // Default mlx-lm port (avoids conflict with A2A on 8080)
	}
	if ollamaEndpoint == "" {
		ollamaEndpoint = "http://127.0.0.1:11434"
	}
	if dnetEndpoint == "" {
		dnetEndpoint = "http://127.0.0.1:9080"
	}

	return &BackendDetector{
		mlxEndpoint:    mlxEndpoint,
		ollamaEndpoint: ollamaEndpoint,
		dnetEndpoint:   dnetEndpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// DetectBackends checks which backends are available and selects the best one.
// Priority order: MLX (fastest on Apple Silicon) > Ollama > dnet
// MLX is 5-10x faster than Ollama on M-series chips.
func (d *BackendDetector) DetectBackends(ctx context.Context) *BackendSelection {
	// Check all backends in parallel
	mlxCh := make(chan *BackendInfo, 1)
	ollamaCh := make(chan *BackendInfo, 1)
	dnetCh := make(chan *BackendInfo, 1)

	go func() {
		mlxCh <- d.probeMLX(ctx)
	}()

	go func() {
		ollamaCh <- d.probeOllama(ctx)
	}()

	go func() {
		dnetCh <- d.probeDnet(ctx)
	}()

	mlx := <-mlxCh
	ollama := <-ollamaCh
	dnet := <-dnetCh

	selection := &BackendSelection{}

	// Determine primary backend based on availability
	// Priority: MLX > Ollama > dnet (MLX is 5-10x faster on Apple Silicon)
	mlxAvailable := mlx != nil && mlx.Available
	ollamaAvailable := ollama != nil && ollama.Available
	dnetAvailable := dnet != nil && dnet.Available

	switch {
	case mlxAvailable:
		// MLX is always preferred when available (fastest on Apple Silicon)
		selection.Primary = mlx
		if ollamaAvailable {
			selection.Secondary = ollama
		} else if dnetAvailable {
			selection.Secondary = dnet
		}
		selection.Reason = fmt.Sprintf("mlx-lm preferred (5-10x faster on Apple Silicon, %dms)", mlx.ResponseMs)

	case ollamaAvailable && dnetAvailable:
		// Both Ollama and dnet available - prefer Ollama for reliability
		const ollamaPreferenceMargin int64 = 100
		if dnet.ResponseMs+ollamaPreferenceMargin < ollama.ResponseMs {
			selection.Primary = dnet
			selection.Secondary = ollama
			selection.Reason = fmt.Sprintf("dnet significantly faster (%dms vs %dms)", dnet.ResponseMs, ollama.ResponseMs)
		} else {
			selection.Primary = ollama
			selection.Secondary = dnet
			selection.Reason = fmt.Sprintf("ollama preferred for reliability (%dms vs %dms)", ollama.ResponseMs, dnet.ResponseMs)
		}

	case ollamaAvailable:
		selection.Primary = ollama
		selection.Reason = "ollama available (mlx, dnet offline)"

	case dnetAvailable:
		selection.Primary = dnet
		selection.Reason = "dnet available (mlx, ollama offline)"

	default:
		selection.Reason = "no local backends available"
	}

	return selection
}

// probeMLX checks if mlx-lm server is running and measures response time.
// mlx-lm uses OpenAI-compatible API on /v1/models and /v1/chat/completions.
func (d *BackendDetector) probeMLX(ctx context.Context) *BackendInfo {
	info := &BackendInfo{
		Type:     BackendMLX,
		Endpoint: d.mlxEndpoint,
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()

	// mlx-lm server uses OpenAI-compatible /v1/models endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", d.mlxEndpoint+"/v1/models", nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()

	info.ResponseMs = time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("status %d", resp.StatusCode)
		return info
	}

	// Parse OpenAI-style models response
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Error = err.Error()
		return info
	}

	info.Available = true
	for _, m := range result.Data {
		info.Models = append(info.Models, m.ID)
	}

	return info
}

// probeOllama checks if Ollama is running and measures response time.
func (d *BackendDetector) probeOllama(ctx context.Context) *BackendInfo {
	info := &BackendInfo{
		Type:     BackendOllama,
		Endpoint: d.ollamaEndpoint,
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", d.ollamaEndpoint+"/api/tags", nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()

	info.ResponseMs = time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("status %d", resp.StatusCode)
		return info
	}

	// Parse models
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Error = err.Error()
		return info
	}

	// Only mark as available if Ollama has at least one model pulled
	// An endpoint responding but with 0 models is not useful as a backend
	if len(result.Models) == 0 {
		info.Error = "no models available (run 'ollama pull <model>' to add one)"
		return info
	}

	info.Available = true
	for _, m := range result.Models {
		info.Models = append(info.Models, m.Name)
	}

	return info
}

// probeDnet checks if dnet is running and measures response time.
func (d *BackendDetector) probeDnet(ctx context.Context) *BackendInfo {
	info := &BackendInfo{
		Type:     BackendDnet,
		Endpoint: d.dnetEndpoint,
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	start := time.Now()

	// dnet uses OpenAI-compatible API, check /v1/models
	req, err := http.NewRequestWithContext(ctx, "GET", d.dnetEndpoint+"/v1/models", nil)
	if err != nil {
		info.Error = err.Error()
		return info
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	defer resp.Body.Close()

	info.ResponseMs = time.Since(start).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		info.Error = fmt.Sprintf("status %d", resp.StatusCode)
		return info
	}

	// Parse OpenAI-style models response
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		info.Error = err.Error()
		return info
	}

	info.Available = true
	for _, m := range result.Data {
		info.Models = append(info.Models, m.ID)
	}

	return info
}

// GetProviderName returns the config provider name for a backend type.
func (b BackendType) ProviderName() string {
	switch b {
	case BackendMLX:
		return "mlx"
	case BackendOllama:
		return "ollama"
	case BackendDnet:
		return "dnet"
	default:
		return ""
	}
}

// String returns a human-readable backend description.
func (info *BackendInfo) String() string {
	if !info.Available {
		return fmt.Sprintf("%s (offline: %s)", info.Type, info.Error)
	}
	return fmt.Sprintf("%s @ %s (%dms, %d models)", info.Type, info.Endpoint, info.ResponseMs, len(info.Models))
}

// HasModel checks if the backend has a specific model available.
func (info *BackendInfo) HasModel(model string) bool {
	if info == nil || !info.Available {
		return false
	}

	modelLower := strings.ToLower(model)
	for _, m := range info.Models {
		if strings.ToLower(m) == modelLower {
			return true
		}
		// Also check base name (e.g., "qwen3" matches "qwen3:4b")
		if strings.HasPrefix(strings.ToLower(m), strings.Split(modelLower, ":")[0]) {
			return true
		}
	}
	return false
}
