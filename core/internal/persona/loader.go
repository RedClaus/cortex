package persona

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads a PersonaCore from a YAML file.
// If the file doesn't exist, returns the default persona.
func LoadFromFile(path string) (*PersonaCore, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default persona if file doesn't exist
		return NewPersonaCore(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read persona file: %w", err)
	}

	return LoadFromYAML(data)
}

// LoadFromYAML parses YAML data into a PersonaCore.
func LoadFromYAML(data []byte) (*PersonaCore, error) {
	// Start with default values
	persona := NewPersonaCore()

	// Unmarshal YAML into the struct
	if err := yaml.Unmarshal(data, persona); err != nil {
		return nil, fmt.Errorf("failed to parse persona YAML: %w", err)
	}

	// Validate the loaded persona
	if err := persona.Validate(); err != nil {
		return nil, fmt.Errorf("invalid persona configuration: %w", err)
	}

	return persona, nil
}

// Validate checks that the persona configuration is valid.
func (p *PersonaCore) Validate() error {
	// Identity validation
	if p.Identity.Name == "" {
		return fmt.Errorf("identity.name is required")
	}
	if p.Identity.Role == "" {
		return fmt.Errorf("identity.role is required")
	}

	// Validate tone
	validTones := map[Tone]bool{
		ToneProfessional: true,
		ToneCasual:       true,
		ToneTechnical:    true,
		ToneFriendly:     true,
	}
	if !validTones[p.Communication.Tone] {
		return fmt.Errorf("invalid tone: %s (valid: professional, casual, technical, friendly)", p.Communication.Tone)
	}

	// Validate detail level
	validDetails := map[DetailLevel]bool{
		DetailConcise:    true,
		DetailBalanced:   true,
		DetailDetailed:   true,
		DetailExhaustive: true,
	}
	if !validDetails[p.Communication.DetailLevel] {
		return fmt.Errorf("invalid detail_level: %s (valid: concise, balanced, detailed, exhaustive)", p.Communication.DetailLevel)
	}

	// Validate formatting
	validFormats := map[Formatting]bool{
		FormatMarkdown: true,
		FormatPlain:    true,
		FormatRich:     true,
	}
	if !validFormats[p.Communication.Formatting] {
		return fmt.Errorf("invalid formatting: %s (valid: markdown, plain, rich)", p.Communication.Formatting)
	}

	return nil
}

// SaveToFile writes the persona configuration to a YAML file.
func (p *PersonaCore) SaveToFile(path string) error {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal persona: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write persona file: %w", err)
	}

	return nil
}

// ToYAML returns the persona as a YAML string.
func (p *PersonaCore) ToYAML() (string, error) {
	data, err := yaml.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("failed to marshal persona: %w", err)
	}
	return string(data), nil
}

// DefaultPersonaPath returns the default path for the persona configuration file.
func DefaultPersonaPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.cortex/persona.yaml"
	}
	return filepath.Join(home, ".cortex", "persona.yaml")
}

// CreateDefaultFile creates a default persona configuration file if it doesn't exist.
func CreateDefaultFile() error {
	path := DefaultPersonaPath()

	// Check if already exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, don't overwrite
	}

	persona := NewPersonaCore()
	return persona.SaveToFile(path)
}

// WithIdentity returns a modified persona with updated identity.
func (p *PersonaCore) WithIdentity(name, version, role string) *PersonaCore {
	clone := p.Clone()
	clone.Identity.Name = name
	clone.Identity.Version = version
	clone.Identity.Role = role
	return clone
}

// WithTone returns a modified persona with updated tone.
func (p *PersonaCore) WithTone(tone Tone) *PersonaCore {
	clone := p.Clone()
	clone.Communication.Tone = tone
	return clone
}

// WithDetailLevel returns a modified persona with updated detail level.
func (p *PersonaCore) WithDetailLevel(level DetailLevel) *PersonaCore {
	clone := p.Clone()
	clone.Communication.DetailLevel = level
	return clone
}

// AddExpertise adds a primary skill to the persona.
func (p *PersonaCore) AddExpertise(skill string, isPrimary bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if isPrimary {
		p.Expertise.Primary = append(p.Expertise.Primary, skill)
	} else {
		p.Expertise.Secondary = append(p.Expertise.Secondary, skill)
	}
}

// SetDomainProficiency sets the proficiency level for a domain.
func (p *PersonaCore) SetDomainProficiency(domain, level string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Expertise.Domains == nil {
		p.Expertise.Domains = make(map[string]string)
	}
	p.Expertise.Domains[domain] = level
}
