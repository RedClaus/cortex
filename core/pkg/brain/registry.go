package brain

import "sync"

// Registry implements LobeRegistry with thread-safe lobe management.
type Registry struct {
	mu    sync.RWMutex
	lobes map[LobeID]Lobe
}

// NewRegistry creates an empty lobe registry.
func NewRegistry() *Registry {
	return &Registry{lobes: make(map[LobeID]Lobe)}
}

// Register adds a lobe to the registry.
func (r *Registry) Register(lobe Lobe) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lobes[lobe.ID()] = lobe
}

// Get retrieves a lobe by ID.
func (r *Registry) Get(id LobeID) (Lobe, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lobe, ok := r.lobes[id]
	return lobe, ok
}

// GetAll retrieves multiple lobes by ID.
func (r *Registry) GetAll(ids []LobeID) []Lobe {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Lobe, 0, len(ids))
	for _, id := range ids {
		if lobe, ok := r.lobes[id]; ok {
			result = append(result, lobe)
		}
	}
	return result
}

// All returns all registered lobes.
func (r *Registry) All() []Lobe {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Lobe, 0, len(r.lobes))
	for _, lobe := range r.lobes {
		result = append(result, lobe)
	}
	return result
}

// Count returns the number of registered lobes.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.lobes)
}
