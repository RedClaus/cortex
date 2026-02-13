package models

import (
	"context"
	"fmt"
	"sync"
)

// ModelInfo represents information about an LLM model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ContextSize int    `json:"context_size,omitempty"`
}

// Provider is the interface for model providers
type Provider interface {
	Engine() string
	ListModels(ctx context.Context) ([]ModelInfo, error)
	ValidateModel(model string) bool
}

// Registry manages multiple providers
type Registry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewRegistry creates a new registry instance
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Engine()] = p
}

// Get retrieves a provider by engine name
func (r *Registry) Get(engine string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[engine]
	return p, ok
}

// ListModels returns models for a specific engine
func (r *Registry) ListModels(ctx context.Context, engine string) ([]ModelInfo, error) {
	r.mu.RLock()
	p, ok := r.providers[engine]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown engine: %s", engine)
	}

	return p.ListModels(ctx)
}

// ValidateModel checks if a model is valid for a given engine
func (r *Registry) ValidateModel(engine, model string) bool {
	r.mu.RLock()
	p, ok := r.providers[engine]
	r.mu.RUnlock()

	if !ok {
		return false
	}

	return p.ValidateModel(model)
}

// Engines returns a list of registered engine names
func (r *Registry) Engines() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engines := make([]string, 0, len(r.providers))
	for engine := range r.providers {
		engines = append(engines, engine)
	}
	return engines
}
