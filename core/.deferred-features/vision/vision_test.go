// Package vision provides tests for the vision package.
// CR-005: Visual Cortex Unit Tests
package vision

import (
	"context"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Provider Interface Tests
// ═══════════════════════════════════════════════════════════════════════════════

// mockProvider implements Provider for testing.
type mockProvider struct {
	name         string
	healthy      bool
	analyzeFunc  func(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error)
	capabilities []Capability
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) IsHealthy() bool { return m.healthy }

func (m *mockProvider) Capabilities() []Capability { return m.capabilities }

func (m *mockProvider) Analyze(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, req)
	}
	return &AnalyzeResponse{
		Content:    "Mock analysis result",
		Provider:   m.name,
		TokensUsed: 100,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Request Validation Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name      string
		req       *AnalyzeRequest
		maxSizeMB int
		wantErr   bool
		errType   error
	}{
		{
			name: "valid request",
			req: &AnalyzeRequest{
				Image:    []byte("fake image data"),
				MimeType: "image/png",
				Prompt:   "What is this?",
			},
			maxSizeMB: 10,
			wantErr:   false,
		},
		{
			name: "empty image",
			req: &AnalyzeRequest{
				Image:    nil,
				MimeType: "image/png",
				Prompt:   "What is this?",
			},
			maxSizeMB: 10,
			wantErr:   true,
		},
		{
			name: "empty prompt",
			req: &AnalyzeRequest{
				Image:    []byte("fake image data"),
				MimeType: "image/png",
				Prompt:   "",
			},
			maxSizeMB: 10,
			wantErr:   true,
		},
		{
			name: "image too large",
			req: &AnalyzeRequest{
				Image:    make([]byte, 11*1024*1024), // 11MB
				MimeType: "image/png",
				Prompt:   "What is this?",
			},
			maxSizeMB: 10,
			wantErr:   true,
			errType:   ErrImageTooLarge,
		},
		{
			name: "invalid mime type",
			req: &AnalyzeRequest{
				Image:    []byte("fake image data"),
				MimeType: "text/plain",
				Prompt:   "What is this?",
			},
			maxSizeMB: 10,
			wantErr:   true,
			errType:   ErrInvalidImageFormat,
		},
		{
			name: "empty mime type is ok",
			req: &AnalyzeRequest{
				Image:    []byte("fake image data"),
				MimeType: "", // Will be detected
				Prompt:   "What is this?",
			},
			maxSizeMB: 10,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req, tt.maxSizeMB)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && err != tt.errType {
				t.Errorf("ValidateRequest() error = %v, want %v", err, tt.errType)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Config Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("Expected Enabled to be true")
	}
	if cfg.OllamaURL != "http://127.0.0.1:11434" {
		t.Errorf("Expected OllamaURL to be http://127.0.0.1:11434, got %s", cfg.OllamaURL)
	}
	if cfg.FastModel != "moondream" {
		t.Errorf("Expected FastModel to be moondream, got %s", cfg.FastModel)
	}
	if cfg.SmartModel != "minicpm-v" {
		t.Errorf("Expected SmartModel to be minicpm-v, got %s", cfg.SmartModel)
	}
	if cfg.MaxImageSizeMB != 10 {
		t.Errorf("Expected MaxImageSizeMB to be 10, got %d", cfg.MaxImageSizeMB)
	}
	if !cfg.EnableFallback {
		t.Error("Expected EnableFallback to be true")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want Config
	}{
		{
			name: "empty config gets defaults",
			cfg:  Config{},
			want: Config{
				OllamaURL:           "http://127.0.0.1:11434",
				FastModel:           "moondream",
				SmartModel:          "minicpm-v",
				MaxImageSizeMB:      10,
				FastModelTimeout:    10 * time.Second,
				SmartModelTimeout:   30 * time.Second,
				HealthCheckInterval: 30 * time.Second,
			},
		},
		{
			name: "custom values preserved",
			cfg: Config{
				OllamaURL:           "http://custom:8080",
				FastModel:           "custom-fast",
				SmartModel:          "custom-smart",
				MaxImageSizeMB:      20,
				FastModelTimeout:    5 * time.Second,
				SmartModelTimeout:   60 * time.Second,
				HealthCheckInterval: 60 * time.Second,
			},
			want: Config{
				OllamaURL:           "http://custom:8080",
				FastModel:           "custom-fast",
				SmartModel:          "custom-smart",
				MaxImageSizeMB:      20,
				FastModelTimeout:    5 * time.Second,
				SmartModelTimeout:   60 * time.Second,
				HealthCheckInterval: 60 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Validate()
			if tt.cfg.OllamaURL != tt.want.OllamaURL {
				t.Errorf("OllamaURL = %v, want %v", tt.cfg.OllamaURL, tt.want.OllamaURL)
			}
			if tt.cfg.FastModel != tt.want.FastModel {
				t.Errorf("FastModel = %v, want %v", tt.cfg.FastModel, tt.want.FastModel)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Router Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestRouterRouting(t *testing.T) {
	fastProvider := &mockProvider{
		name:    "moondream",
		healthy: true,
		capabilities: []Capability{
			CapabilityClassification,
			CapabilityDescription,
		},
	}

	smartProvider := &mockProvider{
		name:    "minicpm-v",
		healthy: true,
		capabilities: []Capability{
			CapabilityOCR,
			CapabilityCodeAnalysis,
		},
	}

	router := NewRouter(fastProvider, smartProvider, DefaultConfig())

	tests := []struct {
		name             string
		prompt           string
		expectedProvider string
	}{
		// Fast Lane prompts
		{"simple description", "What is this?", "moondream"},
		{"describe", "Describe this image", "moondream"},
		{"is this", "Is this a cat?", "moondream"},

		// Smart Lane prompts (OCR/Code triggers)
		{"read text", "Read the text in this image", "minicpm-v"},
		{"code analysis", "Debug this code", "minicpm-v"},
		{"error", "What's the error here?", "minicpm-v"},
		{"terminal", "What's in this terminal output?", "minicpm-v"},
		{"kubernetes", "Show me the kubernetes pods status", "minicpm-v"},
		{"analyze chart", "Analyze this chart data", "minicpm-v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp, err := router.Analyze(ctx, &AnalyzeRequest{
				Image:    []byte("fake image"),
				MimeType: "image/png",
				Prompt:   tt.prompt,
			})

			if err != nil {
				t.Fatalf("Analyze() error = %v", err)
			}

			if resp.Provider != tt.expectedProvider {
				t.Errorf("Expected provider %s, got %s for prompt: %q",
					tt.expectedProvider, resp.Provider, tt.prompt)
			}
		})
	}
}

func TestRouterFallback(t *testing.T) {
	fastProvider := &mockProvider{
		name:    "moondream",
		healthy: true,
	}

	smartProvider := &mockProvider{
		name:    "minicpm-v",
		healthy: false, // Smart provider is unhealthy
	}

	config := DefaultConfig()
	config.EnableFallback = true
	router := NewRouter(fastProvider, smartProvider, config)

	ctx := context.Background()
	// This prompt would normally route to smart, but smart is unhealthy
	resp, err := router.Analyze(ctx, &AnalyzeRequest{
		Image:    []byte("fake image"),
		MimeType: "image/png",
		Prompt:   "Read the code in this screenshot", // Smart lane trigger
	})

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	// Should fallback to fast
	if resp.Provider != "moondream" {
		t.Errorf("Expected fallback to moondream, got %s", resp.Provider)
	}

	if !resp.UsedFallback {
		t.Error("Expected UsedFallback to be true")
	}
}

func TestRouterDisabled(t *testing.T) {
	fastProvider := &mockProvider{name: "moondream", healthy: true}
	smartProvider := &mockProvider{name: "minicpm-v", healthy: true}

	config := DefaultConfig()
	config.Enabled = false
	router := NewRouter(fastProvider, smartProvider, config)

	ctx := context.Background()
	_, err := router.Analyze(ctx, &AnalyzeRequest{
		Image:    []byte("fake image"),
		MimeType: "image/png",
		Prompt:   "What is this?",
	})

	if err != ErrVisionDisabled {
		t.Errorf("Expected ErrVisionDisabled, got %v", err)
	}
}

func TestRouterHealth(t *testing.T) {
	fastProvider := &mockProvider{name: "moondream", healthy: true}
	smartProvider := &mockProvider{name: "minicpm-v", healthy: false}

	router := NewRouter(fastProvider, smartProvider, DefaultConfig())

	ctx := context.Background()
	health := router.Health(ctx)

	if !health["moondream"].Healthy {
		t.Error("Expected moondream to be healthy")
	}

	if health["minicpm-v"].Healthy {
		t.Error("Expected minicpm-v to be unhealthy")
	}

	if !health["minicpm-v"].Fallback {
		t.Error("Expected minicpm-v to have fallback enabled")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Capability Tests
// ═══════════════════════════════════════════════════════════════════════════════

func TestHasCapability(t *testing.T) {
	caps := []Capability{CapabilityOCR, CapabilityCodeAnalysis}

	if !HasCapability(caps, CapabilityOCR) {
		t.Error("Expected to have OCR capability")
	}

	if HasCapability(caps, CapabilityChartReading) {
		t.Error("Expected NOT to have ChartReading capability")
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Integration-style Tests (with mock Ollama)
// ═══════════════════════════════════════════════════════════════════════════════

func TestEndToEndWithMocks(t *testing.T) {
	// Create mock providers that simulate real behavior
	fastProvider := &mockProvider{
		name:    "moondream",
		healthy: true,
		analyzeFunc: func(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
			return &AnalyzeResponse{
				Content:    "This is an image of a cat.",
				Provider:   "moondream",
				TokensUsed: 50,
			}, nil
		},
	}

	smartProvider := &mockProvider{
		name:    "minicpm-v",
		healthy: true,
		analyzeFunc: func(ctx context.Context, req *AnalyzeRequest) (*AnalyzeResponse, error) {
			return &AnalyzeResponse{
				Content:    "The terminal shows: ERROR: Connection refused on port 5432. This indicates PostgreSQL is not running.",
				Provider:   "minicpm-v",
				TokensUsed: 150,
			}, nil
		},
	}

	router := NewRouter(fastProvider, smartProvider, DefaultConfig())
	ctx := context.Background()

	// Test simple query -> fast lane
	t.Run("simple query uses fast lane", func(t *testing.T) {
		resp, err := router.Analyze(ctx, &AnalyzeRequest{
			Image:    []byte("fake cat image"),
			MimeType: "image/jpeg",
			Prompt:   "What is this?",
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Provider != "moondream" {
			t.Errorf("Expected moondream, got %s", resp.Provider)
		}
		if resp.Content != "This is an image of a cat." {
			t.Errorf("Unexpected content: %s", resp.Content)
		}
	})

	// Test technical query -> smart lane
	t.Run("technical query uses smart lane", func(t *testing.T) {
		resp, err := router.Analyze(ctx, &AnalyzeRequest{
			Image:    []byte("fake terminal screenshot"),
			MimeType: "image/png",
			Prompt:   "What error is shown in this terminal?",
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Provider != "minicpm-v" {
			t.Errorf("Expected minicpm-v, got %s", resp.Provider)
		}
		if resp.Content == "" {
			t.Error("Expected non-empty content")
		}
	})
}
