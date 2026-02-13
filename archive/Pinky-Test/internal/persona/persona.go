// Package persona manages Pinky's personality system, providing
// built-in persona templates and support for custom user-defined personas.
package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Persona defines a complete personality configuration
type Persona struct {
	ID           string `yaml:"id"`
	Name         string `yaml:"name"`
	Description  string `yaml:"description,omitempty"`
	SystemPrompt string `yaml:"system_prompt"`
	Traits       Traits `yaml:"traits"`
}

// Traits configures personality attributes that affect response style
type Traits struct {
	Formality  Formality  `yaml:"formality"`
	Verbosity  Verbosity  `yaml:"verbosity"`
	EmojiUsage EmojiUsage `yaml:"emoji_usage"`
	Humor      Humor      `yaml:"humor"`
}

// Formality levels
type Formality string

const (
	FormalityLow    Formality = "low"
	FormalityMedium Formality = "medium"
	FormalityHigh   Formality = "high"
)

// Verbosity levels
type Verbosity string

const (
	VerbosityMinimal Verbosity = "minimal"
	VerbosityNormal  Verbosity = "normal"
	VerbosityVerbose Verbosity = "verbose"
)

// EmojiUsage levels
type EmojiUsage string

const (
	EmojiNone       EmojiUsage = "none"
	EmojiOccasional EmojiUsage = "occasional"
	EmojiFrequent   EmojiUsage = "frequent"
)

// Humor levels
type Humor string

const (
	HumorNone     Humor = "none"
	HumorMinimal  Humor = "minimal"
	HumorModerate Humor = "moderate"
	HumorHigh     Humor = "high"
)

// Manager handles persona loading, caching, and selection
type Manager struct {
	builtIn  map[string]*Persona
	custom   map[string]*Persona
	basePath string
	current  string
}

// DefaultPersona is the persona used when none is specified
const DefaultPersona = "professional"

// NewManager creates a persona manager with built-in templates
func NewManager(customPath string) *Manager {
	m := &Manager{
		builtIn:  make(map[string]*Persona),
		custom:   make(map[string]*Persona),
		basePath: customPath,
		current:  DefaultPersona,
	}
	m.registerBuiltIn()
	return m
}

// Current returns the currently active persona
func (m *Manager) Current() (*Persona, error) {
	return m.Get(m.current)
}

// CurrentID returns the ID of the currently active persona
func (m *Manager) CurrentID() string {
	return m.current
}

// Select changes the active persona
func (m *Manager) Select(id string) error {
	id = strings.ToLower(id)

	// Validate persona exists
	if _, err := m.Get(id); err != nil {
		return err
	}

	m.current = id
	return nil
}

// Get returns a persona by ID, checking custom personas first
func (m *Manager) Get(id string) (*Persona, error) {
	id = strings.ToLower(id)

	// Custom personas take precedence
	if p, ok := m.custom[id]; ok {
		return p, nil
	}

	// Fall back to built-in
	if p, ok := m.builtIn[id]; ok {
		return p, nil
	}

	return nil, fmt.Errorf("persona not found: %s", id)
}

// List returns all available personas (built-in and custom)
func (m *Manager) List() []*Persona {
	seen := make(map[string]bool)
	var result []*Persona

	// Custom personas first (they override built-in)
	for id, p := range m.custom {
		result = append(result, p)
		seen[id] = true
	}

	// Built-in personas (skip if overridden)
	for id, p := range m.builtIn {
		if !seen[id] {
			result = append(result, p)
		}
	}

	return result
}

// ListBuiltIn returns only built-in personas
func (m *Manager) ListBuiltIn() []*Persona {
	result := make([]*Persona, 0, len(m.builtIn))
	for _, p := range m.builtIn {
		result = append(result, p)
	}
	return result
}

// ListCustom returns only custom personas
func (m *Manager) ListCustom() []*Persona {
	result := make([]*Persona, 0, len(m.custom))
	for _, p := range m.custom {
		result = append(result, p)
	}
	return result
}

// LoadCustom loads custom personas from the configured path
func (m *Manager) LoadCustom() error {
	if m.basePath == "" {
		return nil
	}

	// Check if directory exists
	info, err := os.Stat(m.basePath)
	if os.IsNotExist(err) {
		return nil // No custom personas directory, that's fine
	}
	if err != nil {
		return fmt.Errorf("checking personas directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("personas path is not a directory: %s", m.basePath)
	}

	// Load all YAML files
	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return fmt.Errorf("reading personas directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(m.basePath, name)
		if err := m.loadPersonaFile(path); err != nil {
			return fmt.Errorf("loading %s: %w", name, err)
		}
	}

	return nil
}

// loadPersonaFile loads a single persona from a YAML file
func (m *Manager) loadPersonaFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var p Persona
	if err := yaml.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	if p.ID == "" {
		// Use filename without extension as ID
		base := filepath.Base(path)
		p.ID = strings.TrimSuffix(strings.TrimSuffix(base, ".yaml"), ".yml")
	}

	p.ID = strings.ToLower(p.ID)
	m.custom[p.ID] = &p
	return nil
}

// Register adds a custom persona programmatically
func (m *Manager) Register(p *Persona) error {
	if p.ID == "" {
		return fmt.Errorf("persona ID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("persona name is required")
	}

	id := strings.ToLower(p.ID)
	m.custom[id] = p
	return nil
}

// IsBuiltIn returns true if the persona ID is a built-in
func (m *Manager) IsBuiltIn(id string) bool {
	_, ok := m.builtIn[strings.ToLower(id)]
	return ok
}
