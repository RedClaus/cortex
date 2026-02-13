package voice

// VoicePersona defines a pre-configured voice personality with associated TTS settings.
// Personas combine voice selection with behavioral traits to create consistent
// character experiences across interactions.
type VoicePersona struct {
	// Name is the persona identifier (e.g., "henry", "ada", "nexus")
	Name string `json:"name"`

	// Description is a human-readable description of the persona
	Description string `json:"description"`

	// VoiceID is the Kokoro voice identifier to use for TTS
	VoiceID string `json:"voice_id"`

	// Speed is the playback speed multiplier (0.5-2.0, default 1.0)
	Speed float64 `json:"speed"`

	// Traits describe the persona's characteristics for prompt engineering
	Traits []string `json:"traits"`

	// Language is the primary language code (e.g., "en")
	Language string `json:"language"`

	// Gender is the voice gender
	Gender Gender `json:"gender"`
}

// BuiltInPersonas contains the default voice personas available in the system.
// These personas are designed to cover common use cases:
// - Henry: Authoritative technical assistant for complex explanations
// - Ada: Warm, educational guide for learning and tutorials
// - Nexus: Fast, efficient assistant for quick interactions
var BuiltInPersonas = map[string]VoicePersona{
	"henry": {
		Name:        "henry",
		Description: "Deep American male voice with an authoritative, confident tone. Ideal for technical explanations and complex topics.",
		VoiceID:     "am_adam",
		Speed:       1.0,
		Traits: []string{
			"authoritative",
			"confident",
			"technical",
			"patient",
			"thorough",
		},
		Language: "en",
		Gender:   GenderMale,
	},
	"ada": {
		Name:        "ada",
		Description: "Warm American female voice with an educational, approachable tone. Perfect for tutorials and learning-focused interactions.",
		VoiceID:     "af_sarah",
		Speed:       0.95,
		Traits: []string{
			"warm",
			"educational",
			"approachable",
			"encouraging",
			"clear",
		},
		Language: "en",
		Gender:   GenderFemale,
	},
	"nexus": {
		Name:        "nexus",
		Description: "Fast, neutral voice optimized for efficiency. Minimal personality, maximum speed for quick command confirmations and brief responses.",
		VoiceID:     "am_michael",
		Speed:       1.15,
		Traits: []string{
			"efficient",
			"neutral",
			"concise",
			"direct",
			"fast",
		},
		Language: "en",
		Gender:   GenderMale,
	},
	"emma": {
		Name:        "emma",
		Description: "Refined British female voice with a professional, articulate tone. Suited for formal communications and presentations.",
		VoiceID:     "bf_emma",
		Speed:       1.0,
		Traits: []string{
			"refined",
			"professional",
			"articulate",
			"polished",
			"formal",
		},
		Language: "en",
		Gender:   GenderFemale,
	},
	"george": {
		Name:        "george",
		Description: "Distinguished British male voice with gravitas. Excellent for narration and authoritative statements.",
		VoiceID:     "bm_george",
		Speed:       0.95,
		Traits: []string{
			"distinguished",
			"authoritative",
			"narrative",
			"measured",
			"sophisticated",
		},
		Language: "en",
		Gender:   GenderMale,
	},
}

// DefaultPersona is the persona used when none is specified.
const DefaultPersona = "henry"

// GetPersona retrieves a persona by name, returning the default if not found.
func GetPersona(name string) VoicePersona {
	if persona, ok := BuiltInPersonas[name]; ok {
		return persona
	}
	return BuiltInPersonas[DefaultPersona]
}

// GetPersonaOrNil retrieves a persona by name, returning nil if not found.
func GetPersonaOrNil(name string) *VoicePersona {
	if persona, ok := BuiltInPersonas[name]; ok {
		return &persona
	}
	return nil
}

// ListPersonas returns all available persona names.
func ListPersonas() []string {
	names := make([]string, 0, len(BuiltInPersonas))
	for name := range BuiltInPersonas {
		names = append(names, name)
	}
	return names
}

// ListPersonaDetails returns all available personas with their full details.
func ListPersonaDetails() []VoicePersona {
	personas := make([]VoicePersona, 0, len(BuiltInPersonas))
	for _, persona := range BuiltInPersonas {
		personas = append(personas, persona)
	}
	return personas
}

// ValidatePersona checks if a persona name exists.
func ValidatePersona(name string) bool {
	_, ok := BuiltInPersonas[name]
	return ok
}

// PersonaForVoice finds a persona that uses the given voice ID.
// Returns nil if no persona uses that voice.
func PersonaForVoice(voiceID string) *VoicePersona {
	for _, persona := range BuiltInPersonas {
		if persona.VoiceID == voiceID {
			return &persona
		}
	}
	return nil
}

// GetTraitsPrompt generates a prompt snippet describing the persona's traits.
// Useful for injecting persona characteristics into LLM prompts.
func (p *VoicePersona) GetTraitsPrompt() string {
	if len(p.Traits) == 0 {
		return ""
	}

	prompt := "Communicate in a "
	for i, trait := range p.Traits {
		if i > 0 {
			if i == len(p.Traits)-1 {
				prompt += " and "
			} else {
				prompt += ", "
			}
		}
		prompt += trait
	}
	prompt += " manner."
	return prompt
}
