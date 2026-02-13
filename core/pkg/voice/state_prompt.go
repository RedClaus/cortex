// Package voice provides voice-related types and utilities for Cortex.
// state_prompt.go provides state-aware prompt enhancement for natural conversation flow (CR-012-C).
package voice

import (
	"fmt"
	"strings"
)

// StateAwarePromptConfig holds configuration for state-aware prompts.
type StateAwarePromptConfig struct {
	// State is the current conversation state.
	State ConversationState
	// Formality is the current formality level.
	Formality string
	// TurnCount is the number of conversation turns.
	TurnCount int
	// IsFirstTurn indicates if this is the first interaction.
	IsFirstTurn bool
	// PreviousInput is the user's previous input (for context).
	PreviousInput string
	// Verbosity preference (terse, normal, verbose).
	Verbosity string
}

// GetStateAwarePrompt appends state guidance to the base prompt.
// This helps the LLM adapt its responses based on conversation context.
func GetStateAwarePrompt(basePrompt string, config StateAwarePromptConfig) string {
	var stateGuidance strings.Builder

	stateGuidance.WriteString("\n\n## Current Conversation State\n")

	switch config.Formality {
	case "formal":
		stateGuidance.WriteString(`**Mode: Formal**
This is a new conversation or returning after a break.
- Use complete sentences
- Be welcoming but professional
- Don't assume context from previous interactions
- Introduce yourself briefly if asked
`)

	case "casual":
		stateGuidance.WriteString(`**Mode: Casual**
We're in an ongoing conversation (turns: ` + fmt.Sprintf("%d", config.TurnCount) + `).
- Be brief and direct
- Skip pleasantries and greetings
- Reference previous context naturally
- It's okay to use contractions and informal language
`)

	case "familiar":
		stateGuidance.WriteString(`**Mode: Familiar**
We've been talking for a while (turns: ` + fmt.Sprintf("%d", config.TurnCount) + `).
- Be very brief and natural
- Use shorthand references
- Anticipate follow-up questions
- Skip explanations for things we've discussed
`)

	case "engaged":
		stateGuidance.WriteString(`**Mode: Engaged**
User is actively speaking or mid-thought.
- Listen actively and respond minimally
- Don't interrupt unnecessarily
- Brief acknowledgments are fine
- Wait for user to complete their thought
`)
	}

	// Add verbosity guidance
	switch config.Verbosity {
	case "terse":
		stateGuidance.WriteString(`
**Verbosity: Terse**
- Maximum 1-2 sentences per response
- Skip explanations unless critical
- Use single-word acknowledgments when possible
`)
	case "verbose":
		stateGuidance.WriteString(`
**Verbosity: Verbose**
- Include brief explanations of what you're doing
- Offer related suggestions proactively
- Acceptable to use 3-4 sentences when helpful
`)
	}

	// Add first turn guidance
	if config.IsFirstTurn {
		stateGuidance.WriteString(`
**First Turn**
- This is the first interaction in this session
- A brief greeting is appropriate
- Establish helpful tone
`)
	}

	// Add previous context if available
	if config.PreviousInput != "" {
		stateGuidance.WriteString(fmt.Sprintf(`
**Previous Context**
User previously said: "%s"
- Reference this naturally if relevant
- Build on previous answers rather than restating
`, truncateForPrompt(config.PreviousInput, 100)))
	}

	return basePrompt + stateGuidance.String()
}

// GetStateAwarePromptFromContext builds state-aware prompt from HenryBrain context.
func GetStateAwarePromptFromContext(basePrompt string, ctx map[string]interface{}) string {
	config := StateAwarePromptConfig{
		Formality: "neutral",
		Verbosity: "normal",
	}

	if state, ok := ctx["state"].(string); ok {
		config.State = ConversationState(state)
	}
	if formality, ok := ctx["formality"].(string); ok {
		config.Formality = formality
	}
	if turnCount, ok := ctx["turn_count"].(int); ok {
		config.TurnCount = turnCount
	}
	if isFirst, ok := ctx["is_first_turn"].(bool); ok {
		config.IsFirstTurn = isFirst
	}
	if verbosity, ok := ctx["verbosity"].(string); ok {
		config.Verbosity = verbosity
	}
	if prevInput, ok := ctx["previous_input"].(string); ok {
		config.PreviousInput = prevInput
	}

	return GetStateAwarePrompt(basePrompt, config)
}

// GetFormalityGuidance returns just the formality-specific guidance.
func GetFormalityGuidance(formality string) string {
	switch formality {
	case "formal":
		return "Respond formally with complete sentences. Be welcoming but professional."
	case "casual":
		return "Respond casually and briefly. Skip pleasantries."
	case "familiar":
		return "Respond very briefly. Use shorthand and anticipate follow-ups."
	case "engaged":
		return "Respond minimally. User is mid-thought."
	default:
		return "Respond naturally and helpfully."
	}
}

// GetVerbosityGuidance returns just the verbosity-specific guidance.
func GetVerbosityGuidance(verbosity string) string {
	switch verbosity {
	case "terse":
		return "Keep responses to 1-2 sentences maximum."
	case "verbose":
		return "Include helpful explanations and suggestions."
	default:
		return "Use natural response length."
	}
}

// BuildConversationalPrompt builds a complete conversational prompt.
func BuildConversationalPrompt(persona VoicePersona, verbosity string, stateCtx map[string]interface{}) string {
	// Get base persona prompt
	basePrompt := GetVoicePersonaPrompt(persona, verbosity)

	// Add state awareness
	return GetStateAwarePromptFromContext(basePrompt, stateCtx)
}

// truncateForPrompt truncates a string for inclusion in prompt.
func truncateForPrompt(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ResponseStyleHints returns style hints based on current state.
type ResponseStyleHints struct {
	MaxSentences   int
	UseGreeting    bool
	UseFarewell    bool
	IncludeContext bool
	Tone           string
}

// GetResponseStyleHints returns hints for response generation based on state.
func GetResponseStyleHints(state ConversationState, turnCount int, verbosity string) ResponseStyleHints {
	hints := ResponseStyleHints{
		MaxSentences:   3,
		UseGreeting:    false,
		UseFarewell:    false,
		IncludeContext: true,
		Tone:           "professional",
	}

	switch state {
	case StateCold:
		hints.UseGreeting = turnCount == 0
		hints.Tone = "welcoming"
		hints.MaxSentences = 3
	case StateWarm:
		hints.Tone = "casual"
		hints.MaxSentences = 2
	case StateActive:
		hints.Tone = "brief"
		hints.MaxSentences = 1
		hints.IncludeContext = false
	}

	// Adjust for verbosity
	switch verbosity {
	case "terse":
		hints.MaxSentences = 1
		hints.IncludeContext = false
	case "verbose":
		hints.MaxSentences += 2
		hints.IncludeContext = true
	}

	return hints
}
