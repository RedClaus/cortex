// Package vision provides unified interfaces for vision/image analysis providers.
// It abstracts Moondream (Fast Lane) and MiniCPM-V (Smart Lane) behind a common interface.
//
// CR-005: Visual Cortex - Hybrid Vision Architecture
// - Fast Lane: Moondream2 (2B) for quick classification (<500ms)
// - Smart Lane: MiniCPM-V 2.6 (8B) for OCR/code analysis (2-4s)
//
// Key Principles:
// - YAGNI: Simple interface, no factories until proven necessary
// - Local-First: All inference via Ollama, images never leave machine
// - Fail Gracefully: Smart Lane unavailable -> fallback to Fast Lane
package vision

import (
	"context"
	"errors"
)

// Common errors
var (
	ErrProviderUnavailable = errors.New("vision provider unavailable")
	ErrModelNotLoaded      = errors.New("vision model not loaded in Ollama")
	ErrImageTooLarge       = errors.New("image exceeds maximum size limit")
	ErrInvalidImageFormat  = errors.New("invalid or unsupported image format")
	ErrTimeout             = errors.New("vision request timeout")
	ErrVisionDisabled      = errors.New("vision is disabled")
)

// Provider is the interface all vision providers must implement.
// YAGNI: Single interface for both models - no factory needed until we have 3+ providers.
type Provider interface {
	// Name returns the provider identifier (e.g., "moondream", "minicpm-v")
	Name() string

	// Analyze processes an image with a prompt and returns the analysis.
	// Hot path (Moondream): ~500ms
	// Cold path (MiniCPM-V): ~2-4s
	Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error)

	// IsHealthy returns true if the model is loaded and ready in Ollama.
	// This should be a quick check, not a full inference test.
	IsHealthy() bool

	// Capabilities returns what this provider excels at.
	Capabilities() []Capability
}

// AnalyzeRequest contains the image and prompt for analysis.
type AnalyzeRequest struct {
	Image    []byte `json:"image"`     // Raw image bytes (PNG, JPEG, WebP)
	MimeType string `json:"mime_type"` // "image/png", "image/jpeg", "image/webp"
	Prompt   string `json:"prompt"`    // User's question about the image
}

// AnalyzeResponse contains the model's analysis result.
type AnalyzeResponse struct {
	Content      string `json:"content"`       // Text response from the model
	Provider     string `json:"provider"`      // Which model was used ("moondream" or "minicpm-v")
	TokensUsed   int    `json:"tokens_used"`   // Image + prompt + response tokens
	ProcessedMs  int64  `json:"processed_ms"`  // Processing time in milliseconds
	UsedFallback bool   `json:"used_fallback"` // True if fell back from Smart to Fast Lane
}

// Capability indicates what a provider excels at.
// Used by the router to make informed routing decisions.
type Capability string

const (
	// CapabilityOCR - reading text from images (terminal, code, logs)
	CapabilityOCR Capability = "ocr"

	// CapabilityCodeAnalysis - understanding and debugging code screenshots
	CapabilityCodeAnalysis Capability = "code_analysis"

	// CapabilityChartReading - extracting data from charts and graphs
	CapabilityChartReading Capability = "chart_reading"

	// CapabilityClassification - quick "what is this?" queries
	CapabilityClassification Capability = "classification"

	// CapabilityDescription - general image description
	CapabilityDescription Capability = "description"

	// CapabilityDashboard - reading K8s, monitoring dashboards
	CapabilityDashboard Capability = "dashboard"
)

// ProviderCapabilities describes what features a provider supports.
type ProviderCapabilities struct {
	Capabilities     []Capability `json:"capabilities"`
	RequiresGPU      bool         `json:"requires_gpu"`       // True if GPU recommended for reasonable speed
	AvgLatencyMs     int          `json:"avg_latency_ms"`     // Average latency in milliseconds
	MaxImageSizeMB   int          `json:"max_image_size_mb"`  // Maximum supported image size
	SupportedFormats []string     `json:"supported_formats"`  // Supported MIME types
}

// ValidMimeTypes contains the supported image MIME types.
var ValidMimeTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true, // First frame only
}

// ValidateRequest validates an analysis request.
func ValidateRequest(req *AnalyzeRequest, maxSizeMB int) error {
	if req.Image == nil || len(req.Image) == 0 {
		return errors.New("image cannot be empty")
	}

	if req.Prompt == "" {
		return errors.New("prompt cannot be empty")
	}

	// Validate image size
	maxBytes := maxSizeMB * 1024 * 1024
	if len(req.Image) > maxBytes {
		return ErrImageTooLarge
	}

	// Validate MIME type
	if req.MimeType != "" && !ValidMimeTypes[req.MimeType] {
		return ErrInvalidImageFormat
	}

	return nil
}

// HasCapability checks if a list of capabilities includes a specific one.
func HasCapability(caps []Capability, target Capability) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}
