// Package vision provides unified interfaces for vision/image analysis providers.
package vision

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// VisionBackend represents available local vision backends.
type VisionBackend string

const (
	BackendNone   VisionBackend = "none"   // No local vision available
	BackendOllama VisionBackend = "ollama" // Ollama with llava model
	BackendCloud  VisionBackend = "cloud"  // Use cloud providers (OpenAI/Anthropic)
)

// MLXVisionConfig holds configuration for the vision server launcher.
type MLXVisionConfig struct {
	// Model is the vision model to load (for Ollama: llava, for cloud: gpt-4o)
	Model string

	// Host and port for the vision server
	Host string
	Port int

	// Timeouts
	StartupTimeout time.Duration
	HealthTimeout  time.Duration
}

// DefaultMLXVisionConfig returns sensible defaults for the vision server.
func DefaultMLXVisionConfig() MLXVisionConfig {
	return MLXVisionConfig{
		Model:          "llava",
		Host:           "127.0.0.1",
		Port:           11434, // Ollama default port
		StartupTimeout: 120 * time.Second,
		HealthTimeout:  5 * time.Second,
	}
}

// VisionLauncher manages local vision server detection and lifecycle.
// It detects available backends (Ollama with llava) and provides status.
type VisionLauncher struct {
	config     MLXVisionConfig
	httpClient *http.Client
	log        *logging.Logger

	// Detected backend
	backend VisionBackend
	mu      sync.RWMutex
}

// Deprecated: Use NewVisionLauncher instead.
type MLXVisionLauncher = VisionLauncher

// NewVisionLauncher creates a new vision launcher that detects available backends.
func NewVisionLauncher(config MLXVisionConfig) *VisionLauncher {
	return &VisionLauncher{
		config: config,
		httpClient: &http.Client{
			Timeout: config.HealthTimeout,
		},
		log:     logging.Global(),
		backend: BackendNone,
	}
}

// NewMLXVisionLauncher creates a new vision launcher (compatibility alias).
func NewMLXVisionLauncher(config MLXVisionConfig) *VisionLauncher {
	return NewVisionLauncher(config)
}

// Endpoint returns the base URL for the vision server.
func (l *VisionLauncher) Endpoint() string {
	return fmt.Sprintf("http://%s:%d", l.config.Host, l.config.Port)
}

// Backend returns the detected vision backend.
func (l *VisionLauncher) Backend() VisionBackend {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.backend
}

// IsHealthy checks if Ollama is running and has llava available.
func (l *VisionLauncher) IsHealthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	// Check Ollama API
	req, err := http.NewRequestWithContext(ctx, "GET", l.Endpoint()+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// HasVisionModel checks if Ollama has a vision-capable model (llava).
func (l *VisionLauncher) HasVisionModel() bool {
	ctx, cancel := context.WithTimeout(context.Background(), l.config.HealthTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", l.Endpoint()+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := l.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()

	// For now, just return true if Ollama is healthy
	// The actual model check happens when CortexEyes tries to use it
	return true
}

// DetectBackend checks what vision backends are available.
func (l *VisionLauncher) DetectBackend() VisionBackend {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if Ollama is running
	if l.IsHealthy() {
		l.backend = BackendOllama
		l.log.Info("[VisionLauncher] Detected Ollama @ %s", l.Endpoint())
		return l.backend
	}

	// No local backend available - will use cloud
	l.backend = BackendCloud
	l.log.Info("[VisionLauncher] No local vision backend, using cloud fallback")
	return l.backend
}

// EnsureRunning detects available vision backends and returns status.
// Unlike other launchers, this doesn't start a server - it detects what's available.
func (l *VisionLauncher) EnsureRunning(ctx context.Context) error {
	backend := l.DetectBackend()

	switch backend {
	case BackendOllama:
		// Ollama is running - check if llava model is available
		l.log.Info("[VisionLauncher] Using Ollama for vision (model=%s)", l.config.Model)
		return nil
	case BackendCloud:
		// No local backend - CortexEyes will use cloud providers
		return fmt.Errorf("no local vision backend available. CortexEyes will use cloud vision (OpenAI/Anthropic) or run without vision analysis. To enable local vision, install Ollama and run: ollama pull llava")
	default:
		return fmt.Errorf("no vision backend available")
	}
}

// Stop is a no-op for the detection-based launcher.
// Provided for interface compatibility.
func (l *VisionLauncher) Stop() {
	// No-op - we don't manage the server lifecycle
}

// IsRunning returns whether a vision backend is available.
func (l *VisionLauncher) IsRunning() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.backend == BackendOllama
}
