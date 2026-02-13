package persona_test

import (
	"strings"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/persona"
)

func TestNewPersonaCore(t *testing.T) {
	p := persona.NewPersonaCore()

	// Verify identity defaults
	if p.Identity.Name != "Cortex" {
		t.Errorf("expected name 'Cortex', got %q", p.Identity.Name)
	}
	if p.Identity.Version != "3.0" {
		t.Errorf("expected version '3.0', got %q", p.Identity.Version)
	}
	if p.Identity.Role != "AI Terminal Assistant" {
		t.Errorf("expected role 'AI Terminal Assistant', got %q", p.Identity.Role)
	}

	// Verify expertise defaults
	if len(p.Expertise.Primary) == 0 {
		t.Error("expected primary expertise to be populated")
	}
	hasGo := false
	for _, skill := range p.Expertise.Primary {
		if skill == "Go" {
			hasGo = true
			break
		}
	}
	if !hasGo {
		t.Error("expected 'Go' in primary expertise")
	}

	// Verify communication defaults
	if p.Communication.Tone != persona.ToneProfessional {
		t.Errorf("expected tone 'professional', got %q", p.Communication.Tone)
	}
	if p.Communication.Formatting != persona.FormatMarkdown {
		t.Errorf("expected formatting 'markdown', got %q", p.Communication.Formatting)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	p := persona.NewPersonaCore()

	// Build prompt without context
	prompt := p.BuildSystemPrompt(nil)

	// Verify essential parts are present
	if !strings.Contains(prompt, "Cortex") {
		t.Error("prompt should contain persona name")
	}
	if !strings.Contains(prompt, "AI Terminal Assistant") {
		t.Error("prompt should contain role")
	}
	if !strings.Contains(prompt, "Go") {
		t.Error("prompt should contain primary expertise")
	}
	if !strings.Contains(prompt, "professional") {
		t.Error("prompt should contain tone instructions")
	}

	// Build prompt with context
	ctx := &persona.SessionContext{
		CWD:        "/home/user/project",
		Language:   "Go",
		Framework:  "Gin",
		GitBranch:  "feature/test",
	}
	promptWithContext := p.BuildSystemPrompt(ctx)

	if !strings.Contains(promptWithContext, "/home/user/project") {
		t.Error("prompt should contain working directory")
	}
	if !strings.Contains(promptWithContext, "Go") {
		t.Error("prompt should contain language")
	}
	if !strings.Contains(promptWithContext, "feature/test") {
		t.Error("prompt should contain git branch")
	}
}

func TestBuildSystemPromptWithMode(t *testing.T) {
	p := persona.NewPersonaCore()

	// Set debugging mode
	p.SetMode(&persona.BehavioralMode{
		Type: persona.ModeDebugging,
		Adjustments: persona.ModeAdjustments{
			Verbosity:     0.7,
			ThinkingDepth: 0.8,
		},
		EnteredAt: time.Now(),
		Trigger:   "test",
	})

	prompt := p.BuildSystemPrompt(nil)

	if !strings.Contains(prompt, "debugging") {
		t.Error("prompt should contain mode name")
	}
	if !strings.Contains(prompt, "DEBUGGING") {
		t.Error("prompt should contain mode instructions")
	}
}

func TestClone(t *testing.T) {
	p := persona.NewPersonaCore()
	p.SetMode(&persona.BehavioralMode{
		Type:      persona.ModeTeaching,
		EnteredAt: time.Now(),
	})

	clone := p.Clone()

	// Verify clone is independent
	if clone.Identity.Name != p.Identity.Name {
		t.Error("clone should have same name")
	}

	// Modify clone and verify original is unchanged
	clone.Identity.Name = "Modified"
	if p.Identity.Name == "Modified" {
		t.Error("modifying clone should not affect original")
	}

	// Verify mode is cloned
	mode := clone.GetMode()
	if mode.Type != persona.ModeTeaching {
		t.Errorf("clone should have same mode, got %s", mode.Type)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*persona.PersonaCore)
		wantErr bool
	}{
		{
			name:    "valid default",
			modify:  func(p *persona.PersonaCore) {},
			wantErr: false,
		},
		{
			name: "empty name",
			modify: func(p *persona.PersonaCore) {
				p.Identity.Name = ""
			},
			wantErr: true,
		},
		{
			name: "empty role",
			modify: func(p *persona.PersonaCore) {
				p.Identity.Role = ""
			},
			wantErr: true,
		},
		{
			name: "invalid tone",
			modify: func(p *persona.PersonaCore) {
				p.Communication.Tone = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid detail level",
			modify: func(p *persona.PersonaCore) {
				p.Communication.DetailLevel = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid formatting",
			modify: func(p *persona.PersonaCore) {
				p.Communication.Formatting = "invalid"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := persona.NewPersonaCore()
			tt.modify(p)
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromYAML(t *testing.T) {
	yaml := `
identity:
  name: TestBot
  version: "1.0"
  role: Test Assistant
  personality:
    - helpful
    - precise
expertise:
  primary:
    - Python
    - Go
  secondary:
    - Rust
  domains:
    testing: expert
communication:
  tone: technical
  detail_level: detailed
  formatting: markdown
  use_emoji: true
`

	p, err := persona.LoadFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFromYAML failed: %v", err)
	}

	if p.Identity.Name != "TestBot" {
		t.Errorf("expected name 'TestBot', got %q", p.Identity.Name)
	}
	if p.Identity.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", p.Identity.Version)
	}
	if p.Communication.Tone != persona.ToneTechnical {
		t.Errorf("expected tone 'technical', got %q", p.Communication.Tone)
	}
	if !p.Communication.UseEmoji {
		t.Error("expected use_emoji to be true")
	}
}

func TestLoadFromYAMLInvalid(t *testing.T) {
	yaml := `
identity:
  name: ""
  role: Test
`
	_, err := persona.LoadFromYAML([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid YAML (empty name)")
	}
}

func TestWithModifiers(t *testing.T) {
	p := persona.NewPersonaCore()

	// Test WithIdentity
	modified := p.WithIdentity("NewName", "2.0", "New Role")
	if modified.Identity.Name != "NewName" {
		t.Errorf("WithIdentity: expected name 'NewName', got %q", modified.Identity.Name)
	}
	if p.Identity.Name != "Cortex" {
		t.Error("WithIdentity should not modify original")
	}

	// Test WithTone
	modified = p.WithTone(persona.ToneCasual)
	if modified.Communication.Tone != persona.ToneCasual {
		t.Errorf("WithTone: expected 'casual', got %q", modified.Communication.Tone)
	}
	if p.Communication.Tone != persona.ToneProfessional {
		t.Error("WithTone should not modify original")
	}

	// Test WithDetailLevel
	modified = p.WithDetailLevel(persona.DetailExhaustive)
	if modified.Communication.DetailLevel != persona.DetailExhaustive {
		t.Errorf("WithDetailLevel: expected 'exhaustive', got %q", modified.Communication.DetailLevel)
	}
}

func TestAddExpertise(t *testing.T) {
	p := persona.NewPersonaCore()
	initialPrimary := len(p.Expertise.Primary)
	initialSecondary := len(p.Expertise.Secondary)

	p.AddExpertise("NewSkill", true)
	if len(p.Expertise.Primary) != initialPrimary+1 {
		t.Error("AddExpertise should add to primary")
	}

	p.AddExpertise("SecondarySkill", false)
	if len(p.Expertise.Secondary) != initialSecondary+1 {
		t.Error("AddExpertise should add to secondary")
	}
}

func TestSetDomainProficiency(t *testing.T) {
	p := persona.NewPersonaCore()

	p.SetDomainProficiency("machine_learning", "familiar")

	if p.Expertise.Domains["machine_learning"] != "familiar" {
		t.Errorf("expected proficiency 'familiar', got %q", p.Expertise.Domains["machine_learning"])
	}
}

func TestToYAML(t *testing.T) {
	p := persona.NewPersonaCore()

	yaml, err := p.ToYAML()
	if err != nil {
		t.Fatalf("ToYAML failed: %v", err)
	}

	if !strings.Contains(yaml, "Cortex") {
		t.Error("YAML should contain name")
	}
	if !strings.Contains(yaml, "professional") {
		t.Error("YAML should contain tone")
	}
}

func TestConcurrentAccess(t *testing.T) {
	p := persona.NewPersonaCore()

	// Test concurrent mode access
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				p.SetMode(&persona.BehavioralMode{
					Type:      persona.ModeDebugging,
					EnteredAt: time.Now(),
				})
				_ = p.GetMode()
				_ = p.BuildSystemPrompt(nil)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
