// Package voice provides voice-related types and utilities for Cortex.
// mode_config.go provides voice mode configuration and detection (CR-012-A).
package voice

// VoiceModeConfig holds voice-specific prompt configuration.
type VoiceModeConfig struct {
	// Enable voice-optimized responses
	Enabled bool

	// Persona to use (default: "henry")
	Persona string

	// Response length preference: "terse", "normal", "verbose"
	Verbosity string

	// Include filler words for natural speech
	NaturalSpeech bool

	// Max response length in tokens (voice should be shorter)
	MaxTokens int

	// Voice ID to use for TTS
	VoiceID string
}

// DefaultVoiceModeConfig returns production defaults.
func DefaultVoiceModeConfig() VoiceModeConfig {
	return VoiceModeConfig{
		Enabled:       true,
		Persona:       "henry",
		Verbosity:     "normal",
		NaturalSpeech: true,
		MaxTokens:     150,       // Keep voice responses concise
		VoiceID:       "am_adam", // Henry's voice
	}
}

// GetVoiceSystemPrompt returns the appropriate system prompt for voice mode.
func GetVoiceSystemPrompt(config VoiceModeConfig) string {
	if !config.Enabled {
		return "" // Use default text prompt
	}

	switch config.Persona {
	case "henry":
		return GetHenryPrompt(config.Verbosity)
	default:
		return GetHenryPrompt(config.Verbosity)
	}
}

// VoiceResponseGuidelines returns additional guidelines to append based on verbosity.
func VoiceResponseGuidelines(verbosity string) string {
	switch verbosity {
	case "terse":
		return HenryTerseMode
	case "verbose":
		return HenryVerboseMode
	default:
		return ""
	}
}

// VoiceSettings contains all voice-related settings.
type VoiceSettings struct {
	// Mode configuration
	Config VoiceModeConfig

	// Response formatting
	Formatter *VoiceResponseFormatter

	// Conversation context
	Context *VoiceConversationContext
}

// NewVoiceSettings creates voice settings with defaults.
func NewVoiceSettings() *VoiceSettings {
	return &VoiceSettings{
		Config:    DefaultVoiceModeConfig(),
		Formatter: DefaultVoiceResponseFormatter(),
		Context:   NewVoiceConversationContext(),
	}
}

// PrepareVoiceResponse prepares a response for TTS output.
func (s *VoiceSettings) PrepareVoiceResponse(response string) string {
	// Apply formatting
	formatted := s.Formatter.Format(response)

	// Add follow-up suggestion if appropriate
	if suggestion := s.Context.SuggestFollowUp(); suggestion != "" {
		formatted += " " + suggestion
	}

	return formatted
}

// GetSystemPrompt returns the voice system prompt with context injection.
func (s *VoiceSettings) GetSystemPrompt() string {
	prompt := GetVoiceSystemPrompt(s.Config)

	// Add context injection if available
	if s.Context != nil {
		contextInjection := s.Context.BuildContextInjection()
		if contextInjection != "" {
			prompt += "\n\n## Current Context\n" + contextInjection
		}
	}

	return prompt
}
