// Package vision provides adapter for cognitive pipeline integration.
// CR-005: Vision Adapter
//
// This file provides the RouterAdapter that implements cognitive.VisionRouter,
// allowing the vision Router to be used with the cognitive pipeline.
package vision

import (
	"context"
)

// RouterAdapter wraps a Router to implement the cognitive.VisionRouter interface.
// This allows the vision package to be used with the cognitive pipeline without
// creating circular dependencies.
type RouterAdapter struct {
	router *Router
}

// NewRouterAdapter creates a new adapter for the vision router.
func NewRouterAdapter(router *Router) *RouterAdapter {
	return &RouterAdapter{router: router}
}

// CognitiveVisionRequest matches cognitive.VisionRequest
type CognitiveVisionRequest struct {
	Image    []byte `json:"image"`
	MimeType string `json:"mime_type"`
	Prompt   string `json:"prompt"`
}

// CognitiveVisionResponse matches cognitive.VisionResponse
type CognitiveVisionResponse struct {
	Content      string `json:"content"`
	Provider     string `json:"provider"`
	ProcessedMs  int64  `json:"processed_ms"`
	UsedFallback bool   `json:"used_fallback"`
}

// Analyze implements the VisionRouter interface for cognitive pipeline.
func (a *RouterAdapter) Analyze(ctx context.Context, req interface{}) (interface{}, error) {
	// Type assert to extract fields (cognitive.VisionRequest)
	// Using interface{} to avoid import cycle
	type visionReq interface {
		GetImage() []byte
		GetMimeType() string
		GetPrompt() string
	}

	// Handle the cognitive.VisionRequest struct directly
	var image []byte
	var mimeType, prompt string

	// Use reflection-free approach: check for common field access patterns
	switch r := req.(type) {
	case *AnalyzeRequest:
		image = r.Image
		mimeType = r.MimeType
		prompt = r.Prompt
	case *CognitiveVisionRequest:
		image = r.Image
		mimeType = r.MimeType
		prompt = r.Prompt
	default:
		// Fallback: try to extract from map or struct
		if m, ok := req.(map[string]interface{}); ok {
			if v, ok := m["image"].([]byte); ok {
				image = v
			}
			if v, ok := m["mime_type"].(string); ok {
				mimeType = v
			}
			if v, ok := m["prompt"].(string); ok {
				prompt = v
			}
		}
	}

	resp, err := a.router.Analyze(ctx, &AnalyzeRequest{
		Image:    image,
		MimeType: mimeType,
		Prompt:   prompt,
	})
	if err != nil {
		return nil, err
	}

	return &CognitiveVisionResponse{
		Content:      resp.Content,
		Provider:     resp.Provider,
		ProcessedMs:  resp.ProcessedMs,
		UsedFallback: resp.UsedFallback,
	}, nil
}

// IsEnabled returns whether vision is enabled.
func (a *RouterAdapter) IsEnabled() bool {
	return a.router.IsEnabled()
}

// GetRouter returns the underlying router.
func (a *RouterAdapter) GetRouter() *Router {
	return a.router
}
