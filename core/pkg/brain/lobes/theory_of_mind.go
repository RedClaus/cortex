package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// UserModel represents the inferred model of a user.
type UserModel struct {
	ExpertiseLevel    string            `json:"expertise_level"`
	CommunicationPref string            `json:"communication_pref"`
	Goals             []string          `json:"goals"`
	Frustrations      []string          `json:"frustrations"`
	KnowledgeGaps     []string          `json:"knowledge_gaps"`
	InferredIntent    string            `json:"inferred_intent"`
	IntentConfidence  float64           `json:"intent_confidence"`
	Assumptions       map[string]string `json:"assumptions"`
	UncertainAreas    []string          `json:"uncertain_areas"`
}

// TheoryOfMindLobe models the user's mental state, intent, and perspective.
type TheoryOfMindLobe struct {
	llm LLMProvider
}

// NewTheoryOfMindLobe creates a new theory of mind processing lobe.
func NewTheoryOfMindLobe(llm LLMProvider) *TheoryOfMindLobe {
	return &TheoryOfMindLobe{
		llm: llm,
	}
}

// ID returns brain.LobeTheoryOfMind
func (l *TheoryOfMindLobe) ID() brain.LobeID {
	return brain.LobeTheoryOfMind
}

// Process infers user mental state and intent.
func (l *TheoryOfMindLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	userModel := l.quickUserModeling(input.RawInput)

	var tokensUsed int
	var modelUsed string

	if l.llm != nil {
		systemPrompt := `You are the Theory of Mind module. Model the user's mental state.

Analyze:
1. What does the user actually want? (stated vs unstated goals)
2. What do they already know? (expertise level)
3. What assumptions are they making?
4. What might they be confused about?
5. What communication style would they prefer?

Return JSON:
{
  "expertise_level": "beginner|intermediate|expert",
  "communication_pref": "detailed|concise|technical|simple",
  "goals": ["primary goal", "secondary goals..."],
  "frustrations": ["potential frustrations"],
  "knowledge_gaps": ["things they might not know"],
  "inferred_intent": "what they really want",
  "intent_confidence": 0.0-1.0,
  "assumptions": {"key": "assumed value"},
  "uncertain_areas": ["things unclear from input"]
}`

		// Build context from blackboard - read directly since Get/access is thread-safe.
		// Note: We don't clone here because we need to modify bb later with updateBlackboard.
		// Clone() marks the original as frozen (immutable) in the CoW implementation.
		var contextBuilder strings.Builder
		contextBuilder.WriteString(fmt.Sprintf("User input: %s\n", input.RawInput))

		if bb != nil && len(bb.Memories) > 0 {
			contextBuilder.WriteString("\nPrior interactions:\n")
			for _, mem := range bb.Memories {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", mem.Content))
			}
		}

		req := &llm.ChatRequest{
			Model:        "user-modeling",
			SystemPrompt: systemPrompt,
			Messages: []llm.Message{
				{Role: "user", Content: contextBuilder.String()},
			},
			Temperature: 0.4,
		}

		resp, err := l.llm.Chat(ctx, req)
		if err == nil {
			userModel = l.parseLLMResponse(resp.Content, userModel)
			tokensUsed = resp.TokensUsed
			modelUsed = resp.Model
		}
	}

	if bb != nil {
		l.updateBlackboard(bb, userModel)
	}

	result := &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    userModel,
		Confidence: userModel.IntentConfidence,
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: tokensUsed,
			ModelUsed:  modelUsed,
		},
	}

	if len(userModel.UncertainAreas) > 2 {
		result.Caveats = append(result.Caveats, "Multiple areas of uncertainty in user model")
	}

	return result, nil
}

func (l *TheoryOfMindLobe) quickUserModeling(input string) *UserModel {
	lowerInput := strings.ToLower(input)

	model := &UserModel{
		ExpertiseLevel:    "intermediate",
		CommunicationPref: "balanced",
		Goals:             []string{},
		Frustrations:      []string{},
		KnowledgeGaps:     []string{},
		InferredIntent:    "",
		IntentConfidence:  0.5,
		Assumptions:       make(map[string]string),
		UncertainAreas:    []string{},
	}

	beginnerSignals := []string{"how do i", "what is", "explain", "help me understand", "i'm new", "beginner", "don't understand"}
	expertSignals := []string{"optimize", "performance", "architecture", "implement", "refactor", "edge case", "trade-off"}

	beginnerCount := 0
	expertCount := 0

	for _, signal := range beginnerSignals {
		if strings.Contains(lowerInput, signal) {
			beginnerCount++
		}
	}
	for _, signal := range expertSignals {
		if strings.Contains(lowerInput, signal) {
			expertCount++
		}
	}

	if beginnerCount > expertCount {
		model.ExpertiseLevel = "beginner"
		model.CommunicationPref = "detailed"
	} else if expertCount > beginnerCount {
		model.ExpertiseLevel = "expert"
		model.CommunicationPref = "technical"
	}

	if strings.Contains(lowerInput, "quick") || strings.Contains(lowerInput, "fast") || strings.Contains(lowerInput, "tldr") {
		model.CommunicationPref = "concise"
	}

	questionWords := []string{"how", "what", "why", "when", "where", "can", "should", "would"}
	for _, q := range questionWords {
		if strings.HasPrefix(lowerInput, q) {
			model.InferredIntent = "seeking information"
			model.IntentConfidence = 0.7
			break
		}
	}

	actionWords := []string{"create", "make", "build", "write", "generate", "fix", "update", "change", "delete", "remove"}
	for _, a := range actionWords {
		if strings.Contains(lowerInput, a) {
			model.InferredIntent = "requesting action"
			model.IntentConfidence = 0.8
			break
		}
	}

	if model.InferredIntent == "" {
		model.InferredIntent = "general inquiry"
		model.IntentConfidence = 0.4
		model.UncertainAreas = append(model.UncertainAreas, "primary intent unclear")
	}

	return model
}

func (l *TheoryOfMindLobe) parseLLMResponse(response string, fallback *UserModel) *UserModel {
	return fallback
}

func (l *TheoryOfMindLobe) updateBlackboard(bb *brain.Blackboard, model *UserModel) {
	if bb.UserState == nil {
		bb.SetUserState(&brain.UserState{
			ExpertiseLevel: model.ExpertiseLevel,
			PreferredTone:  model.CommunicationPref,
		})
	} else {
		bb.Set("inferred_intent", model.InferredIntent)
		bb.Set("intent_confidence", model.IntentConfidence)
	}

	if len(model.Goals) > 0 {
		bb.Set("user_goals", model.Goals)
	}
}

// CanHandle returns confidence for user modeling tasks.
func (l *TheoryOfMindLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)

	explicitSignals := []string{"what do you think i", "do you understand", "perspective", "point of view", "assumption"}
	for _, signal := range explicitSignals {
		if strings.Contains(lowerInput, signal) {
			return 0.9
		}
	}

	implicitSignals := []string{"help", "need", "want", "trying to", "goal"}
	for _, signal := range implicitSignals {
		if strings.Contains(lowerInput, signal) {
			return 0.6
		}
	}

	return 0.3
}

// ResourceEstimate returns moderate resource requirements.
func (l *TheoryOfMindLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 500,
		EstimatedTime:   800 * time.Millisecond,
		RequiresGPU:     false,
	}
}
