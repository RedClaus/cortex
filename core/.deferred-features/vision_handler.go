package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/normanking/cortex/internal/vision"
)

// ═══════════════════════════════════════════════════════════════════════════════
// VISION API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// VisionAnalyzeRequest is the request body for POST /api/v1/vision/analyze.
type VisionAnalyzeRequest struct {
	Image    string `json:"image"`              // Base64-encoded image
	Prompt   string `json:"prompt"`             // User's question about the image
	MimeType string `json:"mime_type,omitempty"` // Optional MIME type (default: image/png)
	Lane     string `json:"lane,omitempty"`     // Optional: "auto", "fast", "smart" (default: "auto")
}

// VisionAnalyzeResponse is the response for POST /api/v1/vision/analyze.
type VisionAnalyzeResponse struct {
	Content      string `json:"content"`       // Text response from the model
	Provider     string `json:"provider"`      // Which model was used
	TokensUsed   int    `json:"tokens_used"`   // Tokens consumed
	ProcessedMs  int64  `json:"processed_ms"`  // Processing time in milliseconds
	UsedFallback bool   `json:"used_fallback"` // True if fell back from Smart to Fast Lane
}

// VisionHealthResponse is the response for GET /api/v1/vision/health.
type VisionHealthResponse struct {
	Enabled   bool                            `json:"enabled"`   // True if vision is enabled
	Providers map[string]VisionProviderHealth `json:"providers"` // Health status of each provider
}

// VisionProviderHealth represents the health status of a single vision provider.
type VisionProviderHealth struct {
	Healthy  bool   `json:"healthy"`            // True if model is loaded and responding
	Role     string `json:"role"`               // "fast_lane" or "smart_lane"
	Model    string `json:"model"`              // Model name
	Fallback bool   `json:"fallback,omitempty"` // True if this provider will use fallback
}

// handleVisionAnalyze handles POST /api/v1/vision/analyze - Analyze an image.
func (p *Prism) handleVisionAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if vision router is initialized
	if p.visionRouter == nil {
		p.writeError(w, http.StatusServiceUnavailable, "vision is not available")
		return
	}

	// Check if vision is enabled
	if !p.visionRouter.IsEnabled() {
		p.writeError(w, http.StatusServiceUnavailable, "vision is disabled")
		return
	}

	// Parse request body
	var req VisionAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Image == "" {
		p.writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	if req.Prompt == "" {
		p.writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	// Decode base64 image
	imageBytes, err := base64.StdEncoding.DecodeString(req.Image)
	if err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid base64 image")
		return
	}

	// Default MIME type to image/png if not provided
	mimeType := req.MimeType
	if mimeType == "" {
		mimeType = "image/png"
	}

	// Create vision analysis request
	visionReq := &vision.AnalyzeRequest{
		Image:    imageBytes,
		MimeType: mimeType,
		Prompt:   req.Prompt,
	}

	// Set timeout for the request (30s for smart lane, 10s for fast lane)
	// We'll use the longer timeout to be safe
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Analyze image
	visionResp, err := p.visionRouter.Analyze(ctx, visionReq)
	if err != nil {
		p.log.Warn("[Prism] Vision analysis failed: %v", err)

		// Check for specific errors
		if err == vision.ErrVisionDisabled {
			p.writeError(w, http.StatusServiceUnavailable, "vision is disabled")
		} else if err == vision.ErrProviderUnavailable {
			p.writeError(w, http.StatusServiceUnavailable, "vision provider unavailable")
		} else if err == vision.ErrModelNotLoaded {
			p.writeError(w, http.StatusServiceUnavailable, "vision model not loaded - run 'ollama pull moondream' or 'ollama pull minicpm-v'")
		} else if err == vision.ErrImageTooLarge {
			p.writeError(w, http.StatusBadRequest, "image exceeds maximum size limit")
		} else if err == vision.ErrInvalidImageFormat {
			p.writeError(w, http.StatusBadRequest, "invalid or unsupported image format")
		} else if err == vision.ErrTimeout {
			p.writeError(w, http.StatusRequestTimeout, "vision request timeout")
		} else {
			p.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	// Build response
	response := VisionAnalyzeResponse{
		Content:      visionResp.Content,
		Provider:     visionResp.Provider,
		TokensUsed:   visionResp.TokensUsed,
		ProcessedMs:  visionResp.ProcessedMs,
		UsedFallback: visionResp.UsedFallback,
	}

	p.log.Info("[Prism] Vision analysis completed: provider=%s, processed_ms=%d, used_fallback=%t",
		response.Provider, response.ProcessedMs, response.UsedFallback)

	p.writeJSON(w, http.StatusOK, response)
}

// handleVisionHealth handles GET /api/v1/vision/health - Check vision health.
func (p *Prism) handleVisionHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if vision router is initialized
	if p.visionRouter == nil {
		response := VisionHealthResponse{
			Enabled:   false,
			Providers: map[string]VisionProviderHealth{},
		}
		p.writeJSON(w, http.StatusOK, response)
		return
	}

	// Get health status from router
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	healthStatus := p.visionRouter.Health(ctx)

	// Convert to response format
	providers := make(map[string]VisionProviderHealth)
	for name, status := range healthStatus {
		providers[name] = VisionProviderHealth{
			Healthy:  status.Healthy,
			Role:     status.Role,
			Model:    status.Model,
			Fallback: status.Fallback,
		}
	}

	response := VisionHealthResponse{
		Enabled:   p.visionRouter.IsEnabled(),
		Providers: providers,
	}

	p.writeJSON(w, http.StatusOK, response)
}
