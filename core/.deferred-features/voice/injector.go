// Package voice provides voice processing capabilities for Cortex.
package voice

import (
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PROMPT INJECTOR - Combines base prompts with voice-specific instructions
// FR-002: Voice-optimized system prompt injection when voice mode is active
// ═══════════════════════════════════════════════════════════════════════════════

// PromptMessage represents a conversation message for the LLM.
type PromptMessage struct {
	Role    string `json:"role"`    // "user", "assistant", "system"
	Content string `json:"content"` // Message content
}

// PromptInjector handles the injection of voice-optimized prompts.
// It combines the base system prompt with voice-specific instructions
// when voice mode is active.
type PromptInjector struct {
	modeDetector *ModeDetector
	basePrompt   string
	includeCode  bool // Whether to include code-related voice guidance
	includeError bool // Whether to include error-related voice guidance
}

// InjectorOption configures the PromptInjector.
type InjectorOption func(*PromptInjector)

// WithCodeGuidance includes code-related voice guidance in the prompt.
func WithCodeGuidance() InjectorOption {
	return func(p *PromptInjector) {
		p.includeCode = true
	}
}

// WithErrorGuidance includes error-related voice guidance in the prompt.
func WithErrorGuidance() InjectorOption {
	return func(p *PromptInjector) {
		p.includeError = true
	}
}

// NewPromptInjector creates a new PromptInjector with the given mode detector and base prompt.
func NewPromptInjector(modeDetector *ModeDetector, basePrompt string, opts ...InjectorOption) *PromptInjector {
	p := &PromptInjector{
		modeDetector: modeDetector,
		basePrompt:   basePrompt,
		includeCode:  true, // Default to including code guidance
		includeError: true, // Default to including error guidance
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// BuildSystemPrompt combines the base prompt with voice-specific instructions
// if voice mode is currently active.
func (p *PromptInjector) BuildSystemPrompt() string {
	// Check if we're in voice mode
	if p.modeDetector == nil || !p.modeDetector.IsVoiceMode() {
		return p.basePrompt
	}

	// Build the voice-enhanced prompt
	var sb strings.Builder

	// Start with voice system prompt
	sb.WriteString(VoiceSystemPrompt)
	sb.WriteString("\n\n")

	// Add code guidance if enabled
	if p.includeCode {
		sb.WriteString(VoiceCodePrompt)
		sb.WriteString("\n\n")
	}

	// Add error guidance if enabled
	if p.includeError {
		sb.WriteString(VoiceErrorPrompt)
		sb.WriteString("\n\n")
	}

	// Add separator and base prompt
	sb.WriteString("## Additional Context\n")
	sb.WriteString(p.basePrompt)

	return sb.String()
}

// BuildFewShotMessages returns few-shot examples as messages if voice mode is active.
// These examples demonstrate the expected response style for voice output.
// Returns nil if voice mode is not active or mode detector is nil.
func (p *PromptInjector) BuildFewShotMessages() []PromptMessage {
	// Only include few-shot examples in voice mode
	if p.modeDetector == nil || !p.modeDetector.IsVoiceMode() {
		return nil
	}

	fewShot := GetVoiceFewShotMessages()
	messages := make([]PromptMessage, len(fewShot))
	for i, m := range fewShot {
		messages[i] = PromptMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	return messages
}

// IsVoiceMode returns whether the injector's mode detector indicates voice mode.
func (p *PromptInjector) IsVoiceMode() bool {
	if p.modeDetector == nil {
		return false
	}
	return p.modeDetector.IsVoiceMode()
}

// SetBasePrompt updates the base prompt.
func (p *PromptInjector) SetBasePrompt(prompt string) {
	p.basePrompt = prompt
}

// GetBasePrompt returns the current base prompt.
func (p *PromptInjector) GetBasePrompt() string {
	return p.basePrompt
}

// ModeDetector returns the underlying mode detector.
func (p *PromptInjector) ModeDetector() *ModeDetector {
	return p.modeDetector
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONVENIENCE FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// InjectVoicePrompt is a convenience function that injects voice-specific
// instructions into a base prompt if voice mode is active.
func InjectVoicePrompt(basePrompt string, isVoiceMode bool) string {
	if !isVoiceMode {
		return basePrompt
	}

	var sb strings.Builder
	sb.WriteString(VoiceSystemPrompt)
	sb.WriteString("\n\n")
	sb.WriteString(VoiceCodePrompt)
	sb.WriteString("\n\n")
	sb.WriteString(VoiceErrorPrompt)
	sb.WriteString("\n\n## Additional Context\n")
	sb.WriteString(basePrompt)

	return sb.String()
}

// GetVoicePromptOnly returns just the voice-specific system prompt
// without any base prompt. Useful for testing or standalone voice interactions.
func GetVoicePromptOnly() string {
	var sb strings.Builder
	sb.WriteString(VoiceSystemPrompt)
	sb.WriteString("\n\n")
	sb.WriteString(VoiceCodePrompt)
	sb.WriteString("\n\n")
	sb.WriteString(VoiceErrorPrompt)
	return sb.String()
}
