package sleep

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Personality represents the complete personality configuration.
type Personality struct {
	Version         string                 `yaml:"version"`
	LastUpdated     time.Time              `yaml:"last_updated"`
	LastSleepCycle  time.Time              `yaml:"last_sleep_cycle"`
	Identity        Identity               `yaml:"identity"`
	Traits          Traits                 `yaml:"traits"`
	Preferences     map[string]string      `yaml:"preferences"`
	LearnedPatterns []LearnedPattern       `yaml:"learned_patterns"`
	Communication   CommunicationStyle     `yaml:"communication_style"`
	Boundaries      PersonalityConstraints `yaml:"boundaries"`
}

// Identity defines core identity.
type Identity struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

// Traits are adjustable personality dimensions (0.0 to 1.0 scale).
type Traits struct {
	Warmth     float64 `yaml:"warmth"`     // Friendly vs professional
	Directness float64 `yaml:"directness"` // Blunt vs diplomatic
	Verbosity  float64 `yaml:"verbosity"`  // Concise vs detailed
	Humor      float64 `yaml:"humor"`      // Serious vs playful
	Formality  float64 `yaml:"formality"`  // Casual vs formal
	Initiative float64 `yaml:"initiative"` // Reactive vs proactive
	Confidence float64 `yaml:"confidence"` // Uncertain vs assertive
	Patience   float64 `yaml:"patience"`   // Quick vs thorough
}

// LearnedPattern is a pattern learned from interactions.
type LearnedPattern struct {
	Pattern      string  `yaml:"pattern"`
	Confidence   float64 `yaml:"confidence"`
	Source       string  `yaml:"source"`
	AppliedSince string  `yaml:"applied_since"`
}

// CommunicationStyle defines how Cortex communicates.
type CommunicationStyle struct {
	Greeting    string `yaml:"greeting"`
	SignOff     string `yaml:"sign_off"`
	Uncertainty string `yaml:"uncertainty"`
}

// PersonalityConstraints define safety boundaries.
type PersonalityConstraints struct {
	MinPatience            float64  `yaml:"min_patience"`
	MaxConfidence          float64  `yaml:"max_confidence"`
	ImmutableTraits        []string `yaml:"immutable_traits"`
	MaxTraitDelta          float64  `yaml:"max_trait_delta"`
	MinAutoConfidence      float64  `yaml:"min_auto_confidence"`
	MaxRiskyProposals      int      `yaml:"max_risky_proposals"`
	MinPatternObservations int      `yaml:"min_pattern_observations"`
}

// GetConstraints returns the personality constraints with defaults applied.
func (p *Personality) GetConstraints() PersonalityConstraints {
	c := p.Boundaries

	// Apply defaults if not set
	if c.MaxTraitDelta == 0 {
		c.MaxTraitDelta = 0.1
	}
	if c.MinAutoConfidence == 0 {
		c.MinAutoConfidence = 0.85
	}
	if c.MaxRiskyProposals == 0 {
		c.MaxRiskyProposals = 2
	}
	if c.MinPatternObservations == 0 {
		c.MinPatternObservations = 5
	}
	if c.MinPatience == 0 {
		c.MinPatience = 0.3
	}
	if c.MaxConfidence == 0 {
		c.MaxConfidence = 0.95
	}
	if len(c.ImmutableTraits) == 0 {
		c.ImmutableTraits = []string{"honesty", "safety_first", "ethics"}
	}

	return c
}

// GetTraitValue returns the value of a trait by name.
func (p *Personality) GetTraitValue(name string) float64 {
	switch name {
	case "warmth":
		return p.Traits.Warmth
	case "directness":
		return p.Traits.Directness
	case "verbosity":
		return p.Traits.Verbosity
	case "humor":
		return p.Traits.Humor
	case "formality":
		return p.Traits.Formality
	case "initiative":
		return p.Traits.Initiative
	case "confidence":
		return p.Traits.Confidence
	case "patience":
		return p.Traits.Patience
	default:
		return 0
	}
}

// ApplyChange applies a single change to the personality.
func (p *Personality) ApplyChange(change PersonalityChange) error {
	switch change.Path {
	case "traits.warmth":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Warmth = v
	case "traits.directness":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Directness = v
	case "traits.verbosity":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Verbosity = v
	case "traits.humor":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Humor = v
	case "traits.formality":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Formality = v
	case "traits.initiative":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Initiative = v
	case "traits.confidence":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Confidence = v
	case "traits.patience":
		v, ok := change.NewValue.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64 for %s", ErrInvalidPath, change.Path)
		}
		p.Traits.Patience = v
	case "learned_patterns":
		if pattern, ok := change.NewValue.(LearnedPattern); ok {
			p.LearnedPatterns = append(p.LearnedPatterns, pattern)
		} else {
			return fmt.Errorf("%w: expected LearnedPattern for %s", ErrInvalidPath, change.Path)
		}
	default:
		return fmt.Errorf("%w: %s", ErrInvalidPath, change.Path)
	}
	return nil
}

// PersonalityStore handles loading/saving personality files.
type PersonalityStore struct {
	basePath    string
	historyPath string
}

// NewPersonalityStore creates a new personality store.
func NewPersonalityStore(basePath string) *PersonalityStore {
	return &PersonalityStore{
		basePath:    basePath,
		historyPath: filepath.Join(basePath, "history"),
	}
}

// Load loads the current personality from disk.
func (ps *PersonalityStore) Load() (*Personality, error) {
	path := filepath.Join(ps.basePath, "cortex.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ps.CreateDefault()
		}
		return nil, fmt.Errorf("failed to read personality file: %w", err)
	}

	var p Personality
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse personality file: %w", err)
	}
	return &p, nil
}

// Save saves the personality to disk.
func (ps *PersonalityStore) Save(p *Personality) error {
	// Ensure directory exists
	if err := os.MkdirAll(ps.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create personality directory: %w", err)
	}

	data, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to marshal personality: %w", err)
	}

	path := filepath.Join(ps.basePath, "cortex.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write personality file: %w", err)
	}

	return nil
}

// Backup creates a backup of the current personality.
func (ps *PersonalityStore) Backup() error {
	// Ensure history directory exists
	if err := os.MkdirAll(ps.historyPath, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	srcPath := filepath.Join(ps.basePath, "cortex.yaml")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to backup
		}
		return fmt.Errorf("failed to read personality for backup: %w", err)
	}

	backupName := fmt.Sprintf("%s.yaml", time.Now().Format("2006-01-02T150405"))
	backupPath := filepath.Join(ps.historyPath, backupName)

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	return nil
}

// CreateDefault creates and saves a default personality file.
func (ps *PersonalityStore) CreateDefault() (*Personality, error) {
	p := &Personality{
		Version:        "1.0",
		LastUpdated:    time.Now(),
		LastSleepCycle: time.Now(),
		Identity: Identity{
			Name: "Cortex",
			Role: "AI companion and collaborator",
		},
		Traits: Traits{
			Warmth:     0.7,
			Directness: 0.8,
			Verbosity:  0.5,
			Humor:      0.3,
			Formality:  0.4,
			Initiative: 0.6,
			Confidence: 0.7,
			Patience:   0.8,
		},
		Preferences: map[string]string{
			"code_style":        "clean, well-documented, pragmatic",
			"explanation_depth": "adapt to user's apparent expertise",
			"error_handling":    "honest about mistakes, quick to correct",
		},
		LearnedPatterns: []LearnedPattern{},
		Communication: CommunicationStyle{
			Greeting:    "casual acknowledgment, dive into task",
			SignOff:     "open-ended, inviting follow-up",
			Uncertainty: "express clearly, offer to investigate",
		},
		Boundaries: PersonalityConstraints{
			MinPatience:            0.5,
			MaxConfidence:          0.9,
			ImmutableTraits:        []string{"honesty", "safety_first", "ethics"},
			MaxTraitDelta:          0.1,
			MinAutoConfidence:      0.85,
			MaxRiskyProposals:      2,
			MinPatternObservations: 5,
		},
	}

	if err := ps.Save(p); err != nil {
		return nil, err
	}

	return p, nil
}

// GetChangeHistory returns a list of backup files (personality history).
func (ps *PersonalityStore) GetChangeHistory() ([]string, error) {
	entries, err := os.ReadDir(ps.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			files = append(files, e.Name())
		}
	}

	return files, nil
}

// LoadFromHistory loads a personality from a backup file.
func (ps *PersonalityStore) LoadFromHistory(filename string) (*Personality, error) {
	path := filepath.Join(ps.historyPath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}

	var p Personality
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse history file: %w", err)
	}

	return &p, nil
}
