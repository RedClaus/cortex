// Package orchestrator provides the RAPID framework configuration.
// CR-026: Reduce AI Prompt Iteration Depth
//
// The RAPID framework reduces iteration depth by:
// 1. Gathering context automatically (fingerprint stage)
// 2. Classifying intent with confidence (routing stage)
// 3. Gating execution on confidence (rapid_gate stage)
// 4. Asking compound questions when confidence is low
// 5. Proceeding with stated assumptions when appropriate
package orchestrator

// RAPIDConfig configures the RAPID confidence gating behavior.
type RAPIDConfig struct {
	// Enabled controls whether RAPID gating is active.
	Enabled bool `yaml:"enabled"`

	// MinConfidence is the threshold for proceeding without clarification.
	// Requests with routing confidence below this will trigger clarification.
	// Default: 0.7 (70% confidence required)
	MinConfidence float64 `yaml:"min_confidence"`

	// SkipForSimpleCommands bypasses RAPID for shell commands like ls, cd, pwd.
	// Default: true
	SkipForSimpleCommands bool `yaml:"skip_for_simple_commands"`

	// MaxClarifications limits how many times we can ask for clarification
	// before proceeding anyway. Prevents infinite clarification loops.
	// Default: 2
	MaxClarifications int `yaml:"max_clarifications"`

	// SkipInVoiceMode disables clarification questions in voice mode
	// since they're awkward in spoken conversation.
	// Default: true
	SkipInVoiceMode bool `yaml:"skip_in_voice_mode"`
}

// DefaultRAPIDConfig returns sensible defaults for RAPID gating.
func DefaultRAPIDConfig() *RAPIDConfig {
	return &RAPIDConfig{
		Enabled:               true,
		MinConfidence:         0.4, // Low threshold - only ask for truly ambiguous queries
		SkipForSimpleCommands: true,
		MaxClarifications:     2,
		SkipInVoiceMode:       true,
	}
}

// WithRAPIDConfig sets the RAPID configuration.
// CR-026: Reduce AI Prompt Iteration Depth
func WithRAPIDConfig(cfg *RAPIDConfig) Option {
	return func(o *Orchestrator) {
		o.rapidConfig = cfg
	}
}

// EnableRAPID enables the RAPID framework with default settings.
func EnableRAPID() Option {
	return func(o *Orchestrator) {
		o.rapidConfig = DefaultRAPIDConfig()
	}
}

// DisableRAPID disables the RAPID framework.
func DisableRAPID() Option {
	return func(o *Orchestrator) {
		o.rapidConfig = &RAPIDConfig{Enabled: false}
	}
}
