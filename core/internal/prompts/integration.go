package prompts

import (
	"sort"
	"sync"
)

// TemplateProvider wraps Store for use with cognitive templates
type TemplateProvider struct {
	store *Store
	mu    sync.RWMutex

	// customPrompts allows user-defined prompts to override defaults
	customPrompts map[string]map[string]string // task -> tier -> prompt
}

// NewTemplateProvider creates a provider from the store
func NewTemplateProvider(store *Store) *TemplateProvider {
	if store == nil {
		store = Load()
	}
	return &TemplateProvider{
		store:         store,
		customPrompts: make(map[string]map[string]string),
	}
}

// GetSystemPrompt returns the optimized system prompt for a task
// based on the model's parameter count. Uses tier selection logic:
// - < 14B params: small tier (concise prompts)
// - >= 14B params: large tier (detailed prompts)
func (p *TemplateProvider) GetSystemPrompt(task string, modelParams int64) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check custom prompts first
	if taskPrompts, ok := p.customPrompts[task]; ok {
		tier := "large"
		if modelParams < 14_000_000_000 {
			tier = "small"
		}
		if prompt, ok := taskPrompts[tier]; ok {
			return prompt
		}
	}

	// Fallback to store
	return p.store.Get(task, modelParams)
}

// GetPromptTemplate returns a template string for a specific tier
// This can be used with the cognitive template engine
func (p *TemplateProvider) GetPromptTemplate(task string, tier string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Check custom prompts first
	if taskPrompts, ok := p.customPrompts[task]; ok {
		if prompt, ok := taskPrompts[tier]; ok {
			return prompt
		}
	}

	// Fallback to store
	return p.store.GetTier(task, tier)
}

// ListTasks returns all available task types (sorted alphabetically)
func (p *TemplateProvider) ListTasks() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Combine tasks from store and custom prompts
	taskSet := make(map[string]bool)

	// Add store tasks
	for task := range p.store.prompts {
		taskSet[task] = true
	}

	// Add custom tasks
	for task := range p.customPrompts {
		taskSet[task] = true
	}

	// Convert to sorted slice
	tasks := make([]string, 0, len(taskSet))
	for task := range taskSet {
		tasks = append(tasks, task)
	}
	sort.Strings(tasks)

	return tasks
}

// RegisterCustomPrompt adds a user-defined prompt
// Custom prompts take precedence over default prompts from the store
func (p *TemplateProvider) RegisterCustomPrompt(task, tier, prompt string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.customPrompts[task]; !ok {
		p.customPrompts[task] = make(map[string]string)
	}
	p.customPrompts[task][tier] = prompt
}

// HasTask checks if a task exists in either the store or custom prompts
func (p *TemplateProvider) HasTask(task string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, ok := p.customPrompts[task]; ok {
		return true
	}
	return p.store.Has(task)
}

// GetTiers returns available tiers for a task
// Returns empty slice if task doesn't exist
func (p *TemplateProvider) GetTiers(task string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tierSet := make(map[string]bool)

	// Check custom prompts
	if taskPrompts, ok := p.customPrompts[task]; ok {
		for tier := range taskPrompts {
			tierSet[tier] = true
		}
	}

	// Check store
	if taskPrompts, ok := p.store.prompts[task]; ok {
		for tier := range taskPrompts {
			tierSet[tier] = true
		}
	}

	// Convert to sorted slice
	tiers := make([]string, 0, len(tierSet))
	for tier := range tierSet {
		tiers = append(tiers, tier)
	}
	sort.Strings(tiers)

	return tiers
}

// RemoveCustomPrompt removes a custom prompt for a task/tier
func (p *TemplateProvider) RemoveCustomPrompt(task, tier string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if taskPrompts, ok := p.customPrompts[task]; ok {
		delete(taskPrompts, tier)
		if len(taskPrompts) == 0 {
			delete(p.customPrompts, task)
		}
	}
}

// ClearCustomPrompts removes all custom prompts
func (p *TemplateProvider) ClearCustomPrompts() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.customPrompts = make(map[string]map[string]string)
}
