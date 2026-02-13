package persona

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager("")

	// Should have 4 built-in personas
	if len(m.builtIn) != 4 {
		t.Errorf("expected 4 built-in personas, got %d", len(m.builtIn))
	}

	// Default should be professional
	if m.CurrentID() != "professional" {
		t.Errorf("expected default persona 'professional', got '%s'", m.CurrentID())
	}
}

func TestGetBuiltInPersonas(t *testing.T) {
	m := NewManager("")

	tests := []struct {
		id   string
		name string
	}{
		{"professional", "Professional"},
		{"casual", "Casual"},
		{"mentor", "Mentor"},
		{"minimalist", "Minimalist"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			p, err := m.Get(tt.id)
			if err != nil {
				t.Fatalf("Get(%q) error: %v", tt.id, err)
			}
			if p.Name != tt.name {
				t.Errorf("expected name %q, got %q", tt.name, p.Name)
			}
			if p.SystemPrompt == "" {
				t.Error("expected non-empty system prompt")
			}
		})
	}
}

func TestGetCaseInsensitive(t *testing.T) {
	m := NewManager("")

	// Should work with different cases
	cases := []string{"Professional", "PROFESSIONAL", "pRoFeSSioNaL"}
	for _, id := range cases {
		p, err := m.Get(id)
		if err != nil {
			t.Errorf("Get(%q) should work case-insensitively: %v", id, err)
		}
		if p.ID != "professional" {
			t.Errorf("expected ID 'professional', got '%s'", p.ID)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	m := NewManager("")

	_, err := m.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent persona")
	}
}

func TestSelect(t *testing.T) {
	m := NewManager("")

	// Select casual
	if err := m.Select("casual"); err != nil {
		t.Fatalf("Select(casual) error: %v", err)
	}

	if m.CurrentID() != "casual" {
		t.Errorf("expected current 'casual', got '%s'", m.CurrentID())
	}

	p, err := m.Current()
	if err != nil {
		t.Fatalf("Current() error: %v", err)
	}
	if p.ID != "casual" {
		t.Errorf("expected persona ID 'casual', got '%s'", p.ID)
	}
}

func TestSelectInvalid(t *testing.T) {
	m := NewManager("")

	err := m.Select("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent persona")
	}

	// Current should remain unchanged
	if m.CurrentID() != "professional" {
		t.Errorf("current should remain 'professional', got '%s'", m.CurrentID())
	}
}

func TestList(t *testing.T) {
	m := NewManager("")

	all := m.List()
	if len(all) != 4 {
		t.Errorf("expected 4 personas, got %d", len(all))
	}

	builtIn := m.ListBuiltIn()
	if len(builtIn) != 4 {
		t.Errorf("expected 4 built-in personas, got %d", len(builtIn))
	}

	custom := m.ListCustom()
	if len(custom) != 0 {
		t.Errorf("expected 0 custom personas, got %d", len(custom))
	}
}

func TestIsBuiltIn(t *testing.T) {
	m := NewManager("")

	if !m.IsBuiltIn("professional") {
		t.Error("professional should be built-in")
	}
	if !m.IsBuiltIn("CASUAL") { // case insensitive
		t.Error("CASUAL should be built-in (case insensitive)")
	}
	if m.IsBuiltIn("custom") {
		t.Error("custom should not be built-in")
	}
}

func TestRegister(t *testing.T) {
	m := NewManager("")

	custom := &Persona{
		ID:           "pirate",
		Name:         "Pirate",
		Description:  "Speaks like a pirate",
		SystemPrompt: "Arr, ye be a pirate assistant!",
		Traits: Traits{
			Formality:  FormalityLow,
			Verbosity:  VerbosityNormal,
			EmojiUsage: EmojiFrequent,
			Humor:      HumorHigh,
		},
	}

	if err := m.Register(custom); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	// Should be retrievable
	p, err := m.Get("pirate")
	if err != nil {
		t.Fatalf("Get(pirate) error: %v", err)
	}
	if p.Name != "Pirate" {
		t.Errorf("expected name 'Pirate', got '%s'", p.Name)
	}

	// Should appear in custom list
	if len(m.ListCustom()) != 1 {
		t.Errorf("expected 1 custom persona, got %d", len(m.ListCustom()))
	}

	// Total should be 5
	if len(m.List()) != 5 {
		t.Errorf("expected 5 total personas, got %d", len(m.List()))
	}
}

func TestRegisterOverridesBuiltIn(t *testing.T) {
	m := NewManager("")

	// Override professional
	custom := &Persona{
		ID:           "professional",
		Name:         "Custom Professional",
		SystemPrompt: "Custom override",
	}

	if err := m.Register(custom); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	// Should get custom version
	p, err := m.Get("professional")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if p.Name != "Custom Professional" {
		t.Errorf("expected custom override, got '%s'", p.Name)
	}

	// Built-in should still exist
	if !m.IsBuiltIn("professional") {
		t.Error("professional should still be marked as built-in")
	}

	// Total count should still be 4 (override, not add)
	if len(m.List()) != 4 {
		t.Errorf("expected 4 total personas (override), got %d", len(m.List()))
	}
}

func TestRegisterValidation(t *testing.T) {
	m := NewManager("")

	// Missing ID
	err := m.Register(&Persona{Name: "Test"})
	if err == nil {
		t.Error("expected error for missing ID")
	}

	// Missing Name
	err = m.Register(&Persona{ID: "test"})
	if err == nil {
		t.Error("expected error for missing Name")
	}
}

func TestLoadCustomFromDirectory(t *testing.T) {
	// Create temp directory with custom personas
	tmpDir, err := os.MkdirTemp("", "pinky-personas-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a custom persona file
	personaYAML := `id: pirate
name: Pirate
description: Speaks like a pirate
system_prompt: |
  Arr, ye be a pirate assistant!
  Speak in pirate vernacular.
traits:
  formality: low
  verbosity: normal
  emoji_usage: frequent
  humor: high
`
	personaPath := filepath.Join(tmpDir, "pirate.yaml")
	if err := os.WriteFile(personaPath, []byte(personaYAML), 0644); err != nil {
		t.Fatalf("failed to write persona file: %v", err)
	}

	// Create manager and load customs
	m := NewManager(tmpDir)
	if err := m.LoadCustom(); err != nil {
		t.Fatalf("LoadCustom error: %v", err)
	}

	// Should have the custom persona
	p, err := m.Get("pirate")
	if err != nil {
		t.Fatalf("Get(pirate) error: %v", err)
	}
	if p.Name != "Pirate" {
		t.Errorf("expected name 'Pirate', got '%s'", p.Name)
	}
	if p.Traits.Humor != HumorHigh {
		t.Errorf("expected humor 'high', got '%s'", p.Traits.Humor)
	}
}

func TestLoadCustomNoDirectory(t *testing.T) {
	m := NewManager("/nonexistent/path")

	// Should not error, just return nil
	if err := m.LoadCustom(); err != nil {
		t.Errorf("LoadCustom should not error for missing dir: %v", err)
	}
}

func TestLoadCustomEmptyPath(t *testing.T) {
	m := NewManager("")

	// Should not error
	if err := m.LoadCustom(); err != nil {
		t.Errorf("LoadCustom should not error for empty path: %v", err)
	}
}

func TestLoadCustomInfersIDFromFilename(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pinky-personas-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create persona without explicit ID
	personaYAML := `name: Robot
system_prompt: Beep boop.
`
	if err := os.WriteFile(filepath.Join(tmpDir, "robot.yaml"), []byte(personaYAML), 0644); err != nil {
		t.Fatalf("failed to write persona file: %v", err)
	}

	m := NewManager(tmpDir)
	if err := m.LoadCustom(); err != nil {
		t.Fatalf("LoadCustom error: %v", err)
	}

	// Should use filename as ID
	p, err := m.Get("robot")
	if err != nil {
		t.Fatalf("Get(robot) error: %v", err)
	}
	if p.ID != "robot" {
		t.Errorf("expected ID 'robot', got '%s'", p.ID)
	}
}

func TestBuiltInPersonaTraits(t *testing.T) {
	tests := []struct {
		id         string
		formality  Formality
		verbosity  Verbosity
		emojiUsage EmojiUsage
		humor      Humor
	}{
		{"professional", FormalityHigh, VerbosityNormal, EmojiNone, HumorNone},
		{"casual", FormalityLow, VerbosityNormal, EmojiOccasional, HumorMinimal},
		{"mentor", FormalityMedium, VerbosityVerbose, EmojiNone, HumorMinimal},
		{"minimalist", FormalityMedium, VerbosityMinimal, EmojiNone, HumorNone},
	}

	m := NewManager("")

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			p, _ := m.Get(tt.id)
			if p.Traits.Formality != tt.formality {
				t.Errorf("formality: expected %s, got %s", tt.formality, p.Traits.Formality)
			}
			if p.Traits.Verbosity != tt.verbosity {
				t.Errorf("verbosity: expected %s, got %s", tt.verbosity, p.Traits.Verbosity)
			}
			if p.Traits.EmojiUsage != tt.emojiUsage {
				t.Errorf("emoji_usage: expected %s, got %s", tt.emojiUsage, p.Traits.EmojiUsage)
			}
			if p.Traits.Humor != tt.humor {
				t.Errorf("humor: expected %s, got %s", tt.humor, p.Traits.Humor)
			}
		})
	}
}
