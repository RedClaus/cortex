// Package persona provides AI identity, expertise, and communication management.
// This implements CR-002's PersonaCore specification for the Cortex AI assistant.
package persona

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// PersonaCore represents the complete AI persona configuration.
// It combines identity, expertise, and communication style to generate
// context-aware system prompts.
type PersonaCore struct {
	Identity      Identity           `yaml:"identity" json:"identity"`
	Expertise     Expertise          `yaml:"expertise" json:"expertise"`
	Communication CommunicationStyle `yaml:"communication" json:"communication"`

	// Runtime state (not persisted)
	mode *BehavioralMode
	mu   sync.RWMutex
}

// Identity defines who the AI is - its name, role, and personality traits.
type Identity struct {
	Name        string   `yaml:"name" json:"name"`               // Display name (e.g., "Cortex")
	Version     string   `yaml:"version" json:"version"`         // Version identifier
	Role        string   `yaml:"role" json:"role"`               // Primary role description
	Personality []string `yaml:"personality" json:"personality"` // Personality traits
}

// Expertise defines what the AI specializes in.
type Expertise struct {
	Primary   []string          `yaml:"primary" json:"primary"`     // Primary languages/tools
	Secondary []string          `yaml:"secondary" json:"secondary"` // Secondary skills
	Domains   map[string]string `yaml:"domains" json:"domains"`     // Domain -> proficiency level
}

// CommunicationStyle defines how the AI communicates with users.
type CommunicationStyle struct {
	Tone        Tone        `yaml:"tone" json:"tone"`                 // Communication tone
	DetailLevel DetailLevel `yaml:"detail_level" json:"detail_level"` // Response detail level
	Formatting  Formatting  `yaml:"formatting" json:"formatting"`     // Output format preference
	UseEmoji    bool        `yaml:"use_emoji" json:"use_emoji"`       // Whether to use emoji
}

// Tone represents the communication tone
type Tone string

const (
	ToneProfessional Tone = "professional"
	ToneCasual       Tone = "casual"
	ToneTechnical    Tone = "technical"
	ToneFriendly     Tone = "friendly"
)

// DetailLevel represents how verbose responses should be
type DetailLevel string

const (
	DetailConcise    DetailLevel = "concise"
	DetailBalanced   DetailLevel = "balanced"
	DetailDetailed   DetailLevel = "detailed"
	DetailExhaustive DetailLevel = "exhaustive"
)

// Formatting represents output format preference
type Formatting string

const (
	FormatMarkdown Formatting = "markdown"
	FormatPlain    Formatting = "plain"
	FormatRich     Formatting = "rich"
)

// NewPersonaCore creates a new PersonaCore with the default Cortex configuration.
func NewPersonaCore() *PersonaCore {
	return &PersonaCore{
		Identity: Identity{
			Name:    "Cortex",
			Version: "3.0",
			Role:    "AI Terminal Assistant",
			Personality: []string{
				"helpful",
				"precise",
				"security-conscious",
				"efficiency-focused",
			},
		},
		Expertise: Expertise{
			Primary: []string{
				"Go",
				"Python",
				"Shell/Bash",
				"DevOps",
			},
			Secondary: []string{
				"Rust",
				"TypeScript",
				"Docker",
				"Kubernetes",
			},
			Domains: map[string]string{
				"cli":        "expert",
				"databases":  "proficient",
				"web":        "familiar",
				"security":   "proficient",
				"networking": "familiar",
			},
		},
		Communication: CommunicationStyle{
			Tone:        ToneProfessional,
			DetailLevel: DetailBalanced,
			Formatting:  FormatMarkdown,
			UseEmoji:    false,
		},
		mode: nil,
	}
}

// SetMode sets the current behavioral mode (thread-safe).
func (p *PersonaCore) SetMode(mode *BehavioralMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// GetMode returns the current behavioral mode (thread-safe).
func (p *PersonaCore) GetMode() *BehavioralMode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.mode == nil {
		return DefaultMode()
	}
	return p.mode
}

// SessionContext provides context for prompt generation.
type SessionContext struct {
	CWD             string            `json:"cwd"`
	ProjectType     string            `json:"project_type,omitempty"`
	Language        string            `json:"language,omitempty"`
	Framework       string            `json:"framework,omitempty"`
	GitBranch       string            `json:"git_branch,omitempty"`
	RecentCommands  []string          `json:"recent_commands,omitempty"`
	RecentErrors    []string          `json:"recent_errors,omitempty"`
	EnvironmentVars map[string]string `json:"environment_vars,omitempty"`
}

// BuildSystemPrompt generates a complete system prompt based on persona and context.
// The prompt adapts based on the current behavioral mode.
func (p *PersonaCore) BuildSystemPrompt(ctx *SessionContext) string {
	var sb strings.Builder

	// Identity section
	sb.WriteString(fmt.Sprintf("You are %s v%s, a %s.\n\n",
		p.Identity.Name,
		p.Identity.Version,
		p.Identity.Role,
	))

	// Personality traits
	if len(p.Identity.Personality) > 0 {
		sb.WriteString("## Personality\n")
		sb.WriteString("You are ")
		sb.WriteString(strings.Join(p.Identity.Personality, ", "))
		sb.WriteString(".\n\n")
	}

	// Expertise section
	sb.WriteString("## Expertise\n")
	if len(p.Expertise.Primary) > 0 {
		sb.WriteString("Primary skills: ")
		sb.WriteString(strings.Join(p.Expertise.Primary, ", "))
		sb.WriteString("\n")
	}
	if len(p.Expertise.Secondary) > 0 {
		sb.WriteString("Secondary skills: ")
		sb.WriteString(strings.Join(p.Expertise.Secondary, ", "))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Communication style
	sb.WriteString("## Communication Style\n")
	sb.WriteString(p.buildCommunicationInstructions())
	sb.WriteString("\n")

	// Behavioral mode adjustments
	mode := p.GetMode()
	if mode != nil && mode.Type != ModeNormal {
		sb.WriteString("## Current Mode: ")
		sb.WriteString(string(mode.Type))
		sb.WriteString("\n")
		sb.WriteString(mode.GetInstructions())
		sb.WriteString("\n")
	}

	// Context section
	if ctx != nil {
		sb.WriteString("## Current Context\n")
		if ctx.CWD != "" {
			sb.WriteString(fmt.Sprintf("Working directory: %s\n", ctx.CWD))
		}
		if ctx.ProjectType != "" {
			sb.WriteString(fmt.Sprintf("Project type: %s\n", ctx.ProjectType))
		}
		if ctx.Language != "" {
			sb.WriteString(fmt.Sprintf("Language: %s\n", ctx.Language))
		}
		if ctx.Framework != "" {
			sb.WriteString(fmt.Sprintf("Framework: %s\n", ctx.Framework))
		}
		if ctx.GitBranch != "" {
			sb.WriteString(fmt.Sprintf("Git branch: %s\n", ctx.GitBranch))
		}
		if len(ctx.RecentErrors) > 0 {
			sb.WriteString("Recent errors:\n")
			for _, err := range ctx.RecentErrors {
				sb.WriteString(fmt.Sprintf("  - %s\n", err))
			}
		}
	}

	return sb.String()
}

// buildCommunicationInstructions generates instructions based on communication style.
func (p *PersonaCore) buildCommunicationInstructions() string {
	var parts []string

	// Tone instructions
	switch p.Communication.Tone {
	case ToneProfessional:
		parts = append(parts, "Maintain a professional and respectful tone.")
	case ToneCasual:
		parts = append(parts, "Use a casual, conversational tone.")
	case ToneTechnical:
		parts = append(parts, "Be precise and use technical terminology appropriately.")
	case ToneFriendly:
		parts = append(parts, "Be warm and approachable while remaining helpful.")
	}

	// Detail level instructions
	switch p.Communication.DetailLevel {
	case DetailConcise:
		parts = append(parts, "Keep responses brief and to the point.")
	case DetailBalanced:
		parts = append(parts, "Provide balanced explanations - not too brief, not too verbose.")
	case DetailDetailed:
		parts = append(parts, "Provide thorough explanations with context.")
	case DetailExhaustive:
		parts = append(parts, "Be comprehensive and cover all relevant aspects.")
	}

	// Formatting instructions
	switch p.Communication.Formatting {
	case FormatMarkdown:
		parts = append(parts, "Use Markdown formatting for code blocks, lists, and headers.")
	case FormatPlain:
		parts = append(parts, "Use plain text without special formatting.")
	case FormatRich:
		parts = append(parts, "Use rich formatting including code highlighting and structured output.")
	}

	// Emoji preference
	if !p.Communication.UseEmoji {
		parts = append(parts, "Do not use emoji in responses.")
	}

	return strings.Join(parts, " ")
}

// Clone creates a deep copy of the PersonaCore.
func (p *PersonaCore) Clone() *PersonaCore {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Copy expertise domains
	domains := make(map[string]string)
	for k, v := range p.Expertise.Domains {
		domains[k] = v
	}

	// Copy personality traits
	personality := make([]string, len(p.Identity.Personality))
	copy(personality, p.Identity.Personality)

	// Copy primary/secondary skills
	primary := make([]string, len(p.Expertise.Primary))
	copy(primary, p.Expertise.Primary)
	secondary := make([]string, len(p.Expertise.Secondary))
	copy(secondary, p.Expertise.Secondary)

	clone := &PersonaCore{
		Identity: Identity{
			Name:        p.Identity.Name,
			Version:     p.Identity.Version,
			Role:        p.Identity.Role,
			Personality: personality,
		},
		Expertise: Expertise{
			Primary:   primary,
			Secondary: secondary,
			Domains:   domains,
		},
		Communication: p.Communication,
	}

	if p.mode != nil {
		modeCopy := *p.mode
		clone.mode = &modeCopy
	}

	return clone
}

// BehavioralMode represents the current behavioral state of the AI.
// Modes affect verbosity, thinking depth, and response style.
type BehavioralMode struct {
	Type        ModeType        `json:"type"`
	Adjustments ModeAdjustments `json:"adjustments"`
	EnteredAt   time.Time       `json:"entered_at"`
	Trigger     string          `json:"trigger"` // What caused the transition
}

// ModeType represents the type of behavioral mode.
type ModeType string

const (
	ModeNormal    ModeType = "normal"
	ModeDebugging ModeType = "debugging"
	ModeTeaching  ModeType = "teaching"
	ModePair      ModeType = "pair_programming"
	ModeReview    ModeType = "code_review"
)

// ModeAdjustments define how behavior changes in each mode.
type ModeAdjustments struct {
	Verbosity      float64 `json:"verbosity"`       // 0.0-1.0, higher = more verbose
	ThinkingDepth  float64 `json:"thinking_depth"`  // 0.0-1.0, higher = more deliberation
	CodeVsExplain  float64 `json:"code_vs_explain"` // 0.0=explain, 1.0=just code
	CheckpointFreq int     `json:"checkpoint_freq"` // How often to ask for confirmation
}

// DefaultMode returns the default Normal mode.
func DefaultMode() *BehavioralMode {
	return &BehavioralMode{
		Type: ModeNormal,
		Adjustments: ModeAdjustments{
			Verbosity:      0.5,
			ThinkingDepth:  0.5,
			CodeVsExplain:  0.5,
			CheckpointFreq: 0,
		},
		EnteredAt: time.Now(),
		Trigger:   "default",
	}
}

// GetInstructions returns mode-specific instructions for the system prompt.
func (m *BehavioralMode) GetInstructions() string {
	switch m.Type {
	case ModeDebugging:
		return `You are in DEBUGGING mode:
- Ask clarifying questions about error context
- Request stack traces or logs if not provided
- Explain the root cause before suggesting fixes
- Verify fixes with the user before proceeding`

	case ModeTeaching:
		return `You are in TEACHING mode:
- Explain concepts thoroughly, starting from fundamentals if needed
- Use analogies and examples to illustrate points
- Break down complex topics into digestible steps
- Check understanding before moving forward`

	case ModePair:
		return `You are in PAIR PROGRAMMING mode:
- Work collaboratively, suggesting next steps
- Explain your reasoning as you go
- Be ready to adjust based on user preferences
- Keep a steady pace without rushing`

	case ModeReview:
		return `You are in CODE REVIEW mode:
- Focus on code quality, correctness, and maintainability
- Point out potential bugs, security issues, and improvements
- Be constructive and explain the "why" behind suggestions
- Prioritize critical issues over style preferences`

	default:
		return ""
	}
}

// String returns a string representation of the mode.
func (m *BehavioralMode) String() string {
	return fmt.Sprintf("Mode{type=%s, verbosity=%.1f, depth=%.1f, entered=%s}",
		m.Type, m.Adjustments.Verbosity, m.Adjustments.ThinkingDepth,
		m.EnteredAt.Format(time.RFC3339))
}
