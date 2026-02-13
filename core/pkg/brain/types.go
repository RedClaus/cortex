package brain

// LobeID represents cognitive modules - 20 total.
// It is the unique identifier for a functional unit in the brain.
type LobeID string

const (
	// Perception Layer (3)

	// LobeVision handles visual input processing.
	LobeVision LobeID = "vision"
	// LobeAudition handles auditory input processing.
	LobeAudition LobeID = "audition"
	// LobeTextParsing handles text input parsing and structuring.
	LobeTextParsing LobeID = "text_parsing"

	// Cognitive Layer (4)

	// LobeMemory handles information storage and retrieval.
	LobeMemory LobeID = "memory"
	// LobePlanning handles task decomposition and planning.
	LobePlanning LobeID = "planning"
	// LobeCreativity handles idea generation and creative thinking.
	LobeCreativity LobeID = "creativity"
	// LobeReasoning handles logical deduction and reasoning.
	LobeReasoning LobeID = "reasoning"

	// Social-Emotional Layer (3)

	// LobeEmotion handles emotional state processing.
	LobeEmotion LobeID = "emotion"
	// LobeTheoryOfMind handles understanding others' perspectives.
	LobeTheoryOfMind LobeID = "theory_of_mind"
	// LobeRapport handles social bonding and interaction.
	LobeRapport LobeID = "rapport"

	// Specialized Reasoning (5)

	// LobeCoding handles software development tasks.
	LobeCoding LobeID = "coding"
	// LobeLogic handles formal logic and math.
	LobeLogic LobeID = "logic"
	// LobeTemporal handles time-based reasoning.
	LobeTemporal LobeID = "temporal"
	// LobeSpatial handles spatial reasoning.
	LobeSpatial LobeID = "spatial"
	// LobeCausal handles cause-and-effect analysis.
	LobeCausal LobeID = "causal"

	// Executive Functions (4)

	// LobeAttention handles focus and prioritization.
	LobeAttention LobeID = "attention"
	// LobeMetacognition handles thinking about thinking.
	LobeMetacognition LobeID = "metacognition"
	// LobeInhibition handles impulse control.
	LobeInhibition LobeID = "inhibition"
	// LobeSelfKnowledge handles self-awareness and capability knowledge.
	LobeSelfKnowledge LobeID = "self_knowledge"

	// Safety & Regulation (1)

	// LobeSafety handles safety checks and harm prevention.
	LobeSafety LobeID = "safety"
)

// RiskLevel categorizes potential harm.
type RiskLevel string

const (
	// RiskLow represents minimal risk.
	RiskLow RiskLevel = "low"
	// RiskMedium represents moderate risk.
	RiskMedium RiskLevel = "medium"
	// RiskHigh represents significant risk.
	RiskHigh RiskLevel = "high"
	// RiskCritical represents extreme risk.
	RiskCritical RiskLevel = "critical"
)

// ComputeTier controls resource allocation.
type ComputeTier string

const (
	// ComputeFast uses local small model.
	ComputeFast ComputeTier = "fast"
	// ComputeDeep uses local large model.
	ComputeDeep ComputeTier = "deep"
	// ComputeMax uses Cloud API.
	ComputeMax ComputeTier = "max"
	// ComputeHybrid uses Mix based on subtask.
	ComputeHybrid ComputeTier = "hybrid"
)

// AllLobes returns a slice of all defined LobeIDs.
func AllLobes() []LobeID {
	return []LobeID{
		LobeVision, LobeAudition, LobeTextParsing,
		LobeMemory, LobePlanning, LobeCreativity, LobeReasoning,
		LobeEmotion, LobeTheoryOfMind, LobeRapport,
		LobeCoding, LobeLogic, LobeTemporal, LobeSpatial, LobeCausal,
		LobeAttention, LobeMetacognition, LobeInhibition, LobeSelfKnowledge,
		LobeSafety,
	}
}

// String returns the string representation of the LobeID.
func (l LobeID) String() string {
	return string(l)
}

// Valid returns true if the LobeID is a known valid lobe.
func (l LobeID) Valid() bool {
	switch l {
	case LobeVision, LobeAudition, LobeTextParsing,
		LobeMemory, LobePlanning, LobeCreativity, LobeReasoning,
		LobeEmotion, LobeTheoryOfMind, LobeRapport,
		LobeCoding, LobeLogic, LobeTemporal, LobeSpatial, LobeCausal,
		LobeAttention, LobeMetacognition, LobeInhibition, LobeSelfKnowledge,
		LobeSafety:
		return true
	default:
		return false
	}
}

// Valid returns true if the RiskLevel is a known valid level.
func (r RiskLevel) Valid() bool {
	switch r {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return true
	default:
		return false
	}
}

// Valid returns true if the ComputeTier is a known valid tier.
func (c ComputeTier) Valid() bool {
	switch c {
	case ComputeFast, ComputeDeep, ComputeMax, ComputeHybrid:
		return true
	default:
		return false
	}
}
