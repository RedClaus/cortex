// Package a2a provides API key management endpoints.
//
// This file implements HTTP endpoints for managing LLM provider API keys.
// Keys are stored securely in ~/.cortex/api-keys.yaml and loaded on startup.
//
// Endpoints:
//   - GET /api/config/providers - Get provider configurations (keys masked)
//   - PUT /api/config/providers - Update provider API keys
package a2a

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/normanking/cortex/internal/logging"
	"gopkg.in/yaml.v3"
)

// ═══════════════════════════════════════════════════════════════════════════════
// API KEY STORAGE
// ═══════════════════════════════════════════════════════════════════════════════

// APIKeyConfig represents the stored API key configuration.
type APIKeyConfig struct {
	Providers map[string]ProviderKeyConfig `yaml:"providers" json:"providers"`
}

// ProviderKeyConfig holds configuration for a single provider.
type ProviderKeyConfig struct {
	APIKey   string `yaml:"api_key" json:"apiKey,omitempty"`
	Model    string `yaml:"model" json:"model,omitempty"`
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
}

// ProviderConfigResponse is the response format for provider config (keys masked).
type ProviderConfigResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	Endpoint    string `json:"endpoint,omitempty"`
	HasKey      bool   `json:"hasKey"`
	KeyPreview  string `json:"keyPreview,omitempty"` // Last 4 chars
	Enabled     bool   `json:"enabled"`
	Available   bool   `json:"available"`
	Description string `json:"description"`
}

// UpdateProviderRequest is the request format for updating a provider.
type UpdateProviderRequest struct {
	Providers map[string]ProviderKeyConfig `json:"providers"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// API KEY MANAGER
// ═══════════════════════════════════════════════════════════════════════════════

// APIKeyManager handles API key storage and retrieval.
type APIKeyManager struct {
	configPath string
	config     *APIKeyConfig
	mu         sync.RWMutex
	log        *logging.Logger
	onUpdate   func() // Callback when keys are updated
}

// NewAPIKeyManager creates a new API key manager.
func NewAPIKeyManager(onUpdate func()) *APIKeyManager {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".cortex", "api-keys.yaml")

	manager := &APIKeyManager{
		configPath: configPath,
		config:     &APIKeyConfig{Providers: make(map[string]ProviderKeyConfig)},
		log:        logging.Global(),
		onUpdate:   onUpdate,
	}

	// Load existing config
	manager.load()

	return manager
}

// load reads the config from disk.
func (m *APIKeyManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize with defaults
			m.initDefaults()
			return m.saveUnlocked()
		}
		return err
	}

	var config APIKeyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return err
	}

	m.config = &config

	// Also check environment variables and merge
	m.mergeEnvVars()

	m.log.Info("[APIKeyManager] Loaded %d provider configurations", len(m.config.Providers))
	return nil
}

// initDefaults initializes default provider configurations.
func (m *APIKeyManager) initDefaults() {
	m.config = &APIKeyConfig{
		Providers: map[string]ProviderKeyConfig{
			"gemini": {
				Model:   "gemini-2.0-flash-exp",
				Enabled: true,
			},
			"anthropic": {
				Model:   "claude-sonnet-4-20250514",
				Enabled: true,
			},
			"openai": {
				Model:   "gpt-4o-mini",
				Enabled: true,
			},
			"grok": {
				Model:    "grok-3-fast",
				Endpoint: "https://api.x.ai/v1",
				Enabled:  true,
			},
			"ollama": {
				Model:    "deepseek-r1:latest",
				Endpoint: "http://127.0.0.1:11434",
				Enabled:  true,
			},
		},
	}

	// Merge any environment variables
	m.mergeEnvVars()
}

// mergeEnvVars merges environment variable API keys into the config.
func (m *APIKeyManager) mergeEnvVars() {
	envMappings := map[string]string{
		"gemini":    "GEMINI_API_KEY",
		"anthropic": "ANTHROPIC_API_KEY",
		"openai":    "OPENAI_API_KEY",
		"grok":      "XAI_API_KEY",
	}

	for provider, envVar := range envMappings {
		if key := os.Getenv(envVar); key != "" {
			cfg := m.config.Providers[provider]
			if cfg.APIKey == "" {
				cfg.APIKey = key
				m.config.Providers[provider] = cfg
			}
		}
	}
}

// save writes the config to disk.
func (m *APIKeyManager) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveUnlocked()
}

func (m *APIKeyManager) saveUnlocked() error {
	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return err
	}

	// Write with restricted permissions (only owner can read/write)
	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return err
	}

	m.log.Info("[APIKeyManager] Saved configuration to %s", m.configPath)
	return nil
}

// GetProviderConfigs returns provider configurations with masked keys.
func (m *APIKeyManager) GetProviderConfigs() []ProviderConfigResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	descriptions := map[string]string{
		"gemini":    "Google Gemini - Fast, efficient, great for analysis",
		"anthropic": "Anthropic Claude - Thoughtful, nuanced reasoning",
		"openai":    "OpenAI GPT - Versatile, broad knowledge",
		"grok":      "xAI Grok - Real-time knowledge, witty",
		"ollama":    "Ollama - Local LLM, private, no API costs",
	}

	names := map[string]string{
		"gemini":    "Gemini",
		"anthropic": "Claude",
		"openai":    "GPT",
		"grok":      "Grok",
		"ollama":    "Ollama",
	}

	var configs []ProviderConfigResponse

	// Ensure consistent ordering
	providerOrder := []string{"gemini", "anthropic", "openai", "grok", "ollama"}

	for _, id := range providerOrder {
		cfg, exists := m.config.Providers[id]
		if !exists {
			continue
		}

		hasKey := cfg.APIKey != ""
		keyPreview := ""
		if hasKey && len(cfg.APIKey) >= 4 {
			keyPreview = "..." + cfg.APIKey[len(cfg.APIKey)-4:]
		}

		// Ollama doesn't need an API key
		available := hasKey || id == "ollama"

		configs = append(configs, ProviderConfigResponse{
			ID:          id,
			Name:        names[id],
			Model:       cfg.Model,
			Endpoint:    cfg.Endpoint,
			HasKey:      hasKey,
			KeyPreview:  keyPreview,
			Enabled:     cfg.Enabled,
			Available:   available,
			Description: descriptions[id],
		})
	}

	return configs
}

// UpdateProviders updates provider configurations.
func (m *APIKeyManager) UpdateProviders(updates map[string]ProviderKeyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, update := range updates {
		existing := m.config.Providers[id]

		// Only update fields that are provided
		if update.APIKey != "" {
			existing.APIKey = update.APIKey
		}
		if update.Model != "" {
			existing.Model = update.Model
		}
		if update.Endpoint != "" {
			existing.Endpoint = update.Endpoint
		}
		existing.Enabled = update.Enabled

		m.config.Providers[id] = existing
	}

	if err := m.saveUnlocked(); err != nil {
		return err
	}

	// Trigger callback to reinitialize providers
	if m.onUpdate != nil {
		go m.onUpdate()
	}

	return nil
}

// GetAPIKey returns the API key for a provider.
func (m *APIKeyManager) GetAPIKey(provider string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cfg, ok := m.config.Providers[provider]; ok {
		return cfg.APIKey
	}
	return ""
}

// GetProviderConfig returns the full config for a provider.
func (m *APIKeyManager) GetProviderConfig(provider string) (ProviderKeyConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.config.Providers[provider]
	return cfg, ok
}

// ═══════════════════════════════════════════════════════════════════════════════
// HTTP HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// HandleGetConfig handles GET /api/config/providers.
func (m *APIKeyManager) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configs := m.GetProviderConfigs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"providers": configs,
		"configPath": m.configPath,
	})
}

// HandleUpdateConfig handles PUT /api/config/providers.
func (m *APIKeyManager) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate API keys (basic format check)
	for id, cfg := range req.Providers {
		if cfg.APIKey != "" {
			if err := validateAPIKey(id, cfg.APIKey); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error":    err.Error(),
					"provider": id,
				})
				return
			}
		}
	}

	if err := m.UpdateProviders(req.Providers); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated configs
	configs := m.GetProviderConfigs()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"providers": configs,
	})
}

// validateAPIKey performs basic validation of API key format.
func validateAPIKey(provider, key string) error {
	key = strings.TrimSpace(key)

	switch provider {
	case "gemini":
		if !strings.HasPrefix(key, "AIza") {
			return &ValidationError{Provider: provider, Message: "Gemini API key should start with 'AIza'"}
		}
	case "anthropic":
		if !strings.HasPrefix(key, "sk-ant-") {
			return &ValidationError{Provider: provider, Message: "Anthropic API key should start with 'sk-ant-'"}
		}
	case "openai":
		if !strings.HasPrefix(key, "sk-") {
			return &ValidationError{Provider: provider, Message: "OpenAI API key should start with 'sk-'"}
		}
	case "grok":
		if !strings.HasPrefix(key, "xai-") {
			return &ValidationError{Provider: provider, Message: "Grok API key should start with 'xai-'"}
		}
	}

	return nil
}

// ValidationError represents an API key validation error.
type ValidationError struct {
	Provider string
	Message  string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// RegisterConfigRoutes registers config management routes on the given mux.
func (m *APIKeyManager) RegisterConfigRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/config/providers", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			m.HandleGetConfig(w, r)
		case http.MethodPut, http.MethodPost:
			m.HandleUpdateConfig(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	m.log.Info("[APIKeyManager] Registered routes: /api/config/providers")
}
