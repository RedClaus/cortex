// Package discovery provides brain discovery and selection functionality.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Brain represents a discovered CortexBrain instance
type Brain struct {
	ID          string    `json:"id"`          // Unique identifier (url-based)
	Name        string    `json:"name"`        // Agent name from card
	Version     string    `json:"version"`     // Agent version
	URL         string    `json:"url"`         // Base URL (e.g., http://localhost:8080)
	Protocol    string    `json:"protocol"`    // Protocol version
	Description string    `json:"description"` // Description from agent card
	Provider    string    `json:"provider"`    // LLM provider (ollama, openai, etc.)
	Model       string    `json:"model"`       // Current model
	Status      string    `json:"status"`      // "online", "offline", "error"
	Latency     int64     `json:"latency"`     // Response time in ms
	LastSeen    time.Time `json:"lastSeen"`    // Last successful contact
	RequiresAuth bool     `json:"requiresAuth"` // Whether auth is required
	Skills      []string  `json:"skills"`      // Available skills
}

// AgentCard is the A2A agent card structure
type AgentCard struct {
	Name            string `json:"name"`
	Version         string `json:"version"`
	Description     string `json:"description"`
	URL             string `json:"url"`
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Streaming bool `json:"streaming"`
	} `json:"capabilities"`
	Skills []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"skills"`
}

// Config holds discovery configuration
type Config struct {
	// Ports to scan for brains
	Ports []int
	// Custom URLs to check (in addition to port scanning)
	CustomURLs []string
	// Scan timeout per endpoint
	Timeout time.Duration
	// How often to refresh discovery
	RefreshInterval time.Duration
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Ports: []int{
			8080, // Default CortexBrain port
			8081, // Alternative port
			8082,
			8083,
			9080, // dnet port (if it has agent card)
		},
		CustomURLs:      []string{},
		Timeout:         2 * time.Second,
		RefreshInterval: 30 * time.Second,
	}
}

// Service discovers and tracks available brains
type Service struct {
	cfg        *Config
	httpClient *http.Client

	mu          sync.RWMutex
	brains      map[string]*Brain
	selectedID  string

	onUpdate    func([]*Brain) // Callback when brains list changes
	onSelect    func(*Brain)   // Callback when brain is selected

	stopCh      chan struct{}
	running     bool
}

// NewService creates a new discovery service
func NewService(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return &Service{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		brains: make(map[string]*Brain),
		stopCh: make(chan struct{}),
	}
}

// SetOnUpdate sets callback for when brains list changes
func (s *Service) SetOnUpdate(fn func([]*Brain)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onUpdate = fn
}

// SetOnSelect sets callback for when brain is selected
func (s *Service) SetOnSelect(fn func(*Brain)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSelect = fn
}

// Start begins background discovery
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	// Initial scan
	go s.Scan()

	// Periodic refresh
	go func() {
		ticker := time.NewTicker(s.cfg.RefreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.Scan()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// Stop stops background discovery
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// Scan performs a discovery scan
func (s *Service) Scan() []*Brain {
	var wg sync.WaitGroup
	results := make(chan *Brain, len(s.cfg.Ports)+len(s.cfg.CustomURLs))

	// Scan ports on localhost
	for _, port := range s.cfg.Ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			url := fmt.Sprintf("http://localhost:%d", p)
			if brain := s.probe(url); brain != nil {
				results <- brain
			}
		}(port)
	}

	// Scan custom URLs
	for _, url := range s.cfg.CustomURLs {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if brain := s.probe(u); brain != nil {
				results <- brain
			}
		}(url)
	}

	// Wait and collect
	go func() {
		wg.Wait()
		close(results)
	}()

	// Update brains map
	s.mu.Lock()

	// Mark all as potentially offline
	for _, b := range s.brains {
		b.Status = "offline"
	}

	// Update with scan results
	for brain := range results {
		s.brains[brain.ID] = brain
	}

	// Build list
	list := make([]*Brain, 0, len(s.brains))
	for _, b := range s.brains {
		list = append(list, b)
	}

	callback := s.onUpdate
	s.mu.Unlock()

	// Notify
	if callback != nil {
		callback(list)
	}

	return list
}

// probe checks a URL for a brain
func (s *Service) probe(baseURL string) *Brain {
	start := time.Now()

	cardURL := baseURL + "/.well-known/agent-card.json"
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", cardURL, nil)
	if err != nil {
		return nil
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil
	}

	latency := time.Since(start).Milliseconds()

	// Extract provider and model from description
	provider, model := parseProviderModel(card.Description)

	// Extract skills
	skills := make([]string, 0, len(card.Skills))
	for _, sk := range card.Skills {
		skills = append(skills, sk.ID)
	}

	// Check if auth is required (look for 401 on a protected endpoint)
	requiresAuth := s.checkAuthRequired(baseURL)

	return &Brain{
		ID:           baseURL,
		Name:         card.Name,
		Version:      card.Version,
		URL:          baseURL,
		Protocol:     card.ProtocolVersion,
		Description:  card.Description,
		Provider:     provider,
		Model:        model,
		Status:       "online",
		Latency:      latency,
		LastSeen:     time.Now(),
		RequiresAuth: requiresAuth,
		Skills:       skills,
	}
}

// parseProviderModel extracts provider and model from description
// e.g., "AI-Powered Assistant with Brain Executive (Provider: ollama, Model: moondream:latest)"
func parseProviderModel(desc string) (provider, model string) {
	provider = "unknown"
	model = "unknown"

	// Simple parsing - look for "Provider: X, Model: Y"
	// This could be made more robust
	if len(desc) == 0 {
		return
	}

	// Find Provider:
	providerIdx := findSubstring(desc, "Provider: ")
	if providerIdx >= 0 {
		start := providerIdx + len("Provider: ")
		end := start
		for end < len(desc) && desc[end] != ',' && desc[end] != ')' {
			end++
		}
		if end > start {
			provider = desc[start:end]
		}
	}

	// Find Model:
	modelIdx := findSubstring(desc, "Model: ")
	if modelIdx >= 0 {
		start := modelIdx + len("Model: ")
		end := start
		for end < len(desc) && desc[end] != ',' && desc[end] != ')' {
			end++
		}
		if end > start {
			model = desc[start:end]
		}
	}

	return
}

func findSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// checkAuthRequired checks if the brain requires authentication
func (s *Service) checkAuthRequired(baseURL string) bool {
	// Try to access a protected endpoint
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/user", nil)
	if err != nil {
		return false
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 401 means auth is required
	return resp.StatusCode == http.StatusUnauthorized
}

// GetBrains returns all discovered brains
func (s *Service) GetBrains() []*Brain {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*Brain, 0, len(s.brains))
	for _, b := range s.brains {
		list = append(list, b)
	}
	return list
}

// GetBrain returns a specific brain by ID
func (s *Service) GetBrain(id string) *Brain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.brains[id]
}

// GetSelected returns the currently selected brain
func (s *Service) GetSelected() *Brain {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.selectedID == "" {
		return nil
	}
	return s.brains[s.selectedID]
}

// Select sets the active brain
func (s *Service) Select(id string) error {
	s.mu.Lock()

	brain, exists := s.brains[id]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("brain not found: %s", id)
	}

	s.selectedID = id
	callback := s.onSelect
	s.mu.Unlock()

	if callback != nil {
		callback(brain)
	}

	return nil
}

// AddCustomURL adds a custom URL to scan
func (s *Service) AddCustomURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	for _, u := range s.cfg.CustomURLs {
		if u == url {
			return
		}
	}

	s.cfg.CustomURLs = append(s.cfg.CustomURLs, url)
}

// RemoveCustomURL removes a custom URL
func (s *Service) RemoveCustomURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, u := range s.cfg.CustomURLs {
		if u == url {
			s.cfg.CustomURLs = append(s.cfg.CustomURLs[:i], s.cfg.CustomURLs[i+1:]...)
			return
		}
	}
}
