package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// RelationshipState tracks the relationship quality with a user.
type RelationshipState struct {
	TrustLevel        float64           `json:"trust_level"`
	FamiliarityLevel  float64           `json:"familiarity_level"`
	InteractionCount  int               `json:"interaction_count"`
	PositiveHistory   int               `json:"positive_history"`
	NegativeHistory   int               `json:"negative_history"`
	SharedContext     []string          `json:"shared_context"`
	UserPreferences   map[string]string `json:"user_preferences"`
	LastInteractionAt time.Time         `json:"last_interaction_at"`
	RelationshipTone  string            `json:"relationship_tone"`
	SuggestedApproach string            `json:"suggested_approach"`
}

// RapportLobe manages relationship state and social interaction quality.
type RapportLobe struct {
	llm          LLMProvider
	stateCache   map[string]*RelationshipState
	defaultState *RelationshipState
}

// NewRapportLobe creates a new rapport management lobe.
func NewRapportLobe(llm LLMProvider) *RapportLobe {
	return &RapportLobe{
		llm:        llm,
		stateCache: make(map[string]*RelationshipState),
		defaultState: &RelationshipState{
			TrustLevel:        0.5,
			FamiliarityLevel:  0.0,
			InteractionCount:  0,
			PositiveHistory:   0,
			NegativeHistory:   0,
			SharedContext:     []string{},
			UserPreferences:   make(map[string]string),
			RelationshipTone:  "neutral",
			SuggestedApproach: "friendly-professional",
		},
	}
}

// ID returns brain.LobeRapport
func (l *RapportLobe) ID() brain.LobeID {
	return brain.LobeRapport
}

// Process evaluates and updates relationship state.
func (l *RapportLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	conversationID := ""
	if bb != nil {
		conversationID = bb.ConversationID
	}

	state := l.getOrCreateState(conversationID)
	state.InteractionCount++
	state.LastInteractionAt = time.Now()

	l.analyzeInteraction(input.RawInput, state, bb)

	var tokensUsed int
	var modelUsed string

	if l.llm != nil && state.InteractionCount%5 == 0 {
		l.deepRelationshipAnalysis(ctx, input, bb, state, &tokensUsed, &modelUsed)
	}

	l.determineSuggestedApproach(state)

	if bb != nil {
		bb.Set("relationship_trust", state.TrustLevel)
		bb.Set("suggested_approach", state.SuggestedApproach)
		bb.Set("relationship_tone", state.RelationshipTone)
	}

	result := &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    state,
		Confidence: l.calculateConfidence(state),
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: tokensUsed,
			ModelUsed:  modelUsed,
		},
	}

	if state.TrustLevel < 0.3 {
		result.Caveats = append(result.Caveats, "Low trust level detected - extra care recommended")
	}

	return result, nil
}

func (l *RapportLobe) getOrCreateState(conversationID string) *RelationshipState {
	if conversationID == "" {
		return l.copyDefaultState()
	}

	if state, exists := l.stateCache[conversationID]; exists {
		return state
	}

	state := l.copyDefaultState()
	l.stateCache[conversationID] = state
	return state
}

func (l *RapportLobe) copyDefaultState() *RelationshipState {
	return &RelationshipState{
		TrustLevel:        l.defaultState.TrustLevel,
		FamiliarityLevel:  l.defaultState.FamiliarityLevel,
		InteractionCount:  l.defaultState.InteractionCount,
		PositiveHistory:   l.defaultState.PositiveHistory,
		NegativeHistory:   l.defaultState.NegativeHistory,
		SharedContext:     make([]string, 0),
		UserPreferences:   make(map[string]string),
		RelationshipTone:  l.defaultState.RelationshipTone,
		SuggestedApproach: l.defaultState.SuggestedApproach,
	}
}

func (l *RapportLobe) analyzeInteraction(input string, state *RelationshipState, bb *brain.Blackboard) {
	lowerInput := strings.ToLower(input)

	positiveSignals := []string{"thanks", "thank you", "great", "perfect", "awesome", "helpful", "appreciate", "love it", "excellent"}
	negativeSignals := []string{"wrong", "bad", "terrible", "useless", "stupid", "hate", "annoying", "frustrated", "disappointed"}

	for _, signal := range positiveSignals {
		if strings.Contains(lowerInput, signal) {
			state.PositiveHistory++
			state.TrustLevel = min(1.0, state.TrustLevel+0.05)
			break
		}
	}

	for _, signal := range negativeSignals {
		if strings.Contains(lowerInput, signal) {
			state.NegativeHistory++
			state.TrustLevel = max(0.0, state.TrustLevel-0.1)
			break
		}
	}

	state.FamiliarityLevel = min(1.0, float64(state.InteractionCount)*0.05)

	if bb != nil {
		if emotion, ok := bb.Get("detected_emotion"); ok {
			if emotionStr, ok := emotion.(string); ok {
				if emotionStr == "positive" {
					state.TrustLevel = min(1.0, state.TrustLevel+0.02)
				}
			}
		}
	}

	l.updateRelationshipTone(state)
}

func (l *RapportLobe) updateRelationshipTone(state *RelationshipState) {
	ratio := float64(state.PositiveHistory+1) / float64(state.NegativeHistory+1)

	switch {
	case ratio > 3.0 && state.TrustLevel > 0.7:
		state.RelationshipTone = "warm"
	case ratio > 1.5 && state.TrustLevel > 0.5:
		state.RelationshipTone = "friendly"
	case ratio < 0.5 || state.TrustLevel < 0.3:
		state.RelationshipTone = "cautious"
	default:
		state.RelationshipTone = "neutral"
	}
}

func (l *RapportLobe) deepRelationshipAnalysis(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard, state *RelationshipState, tokensUsed *int, modelUsed *string) {
	systemPrompt := `Analyze the relationship quality and suggest communication approach.

Consider:
- Trust indicators in user messages
- Frustration or satisfaction signals
- Communication style preferences
- Relationship trajectory (improving/declining)

Return JSON:
{
  "trust_adjustment": -0.2 to 0.2,
  "tone_recommendation": "warm|friendly|neutral|professional|cautious",
  "approach": "specific approach recommendation"
}`

	var contextBuilder strings.Builder
	contextBuilder.WriteString(fmt.Sprintf("Current input: %s\n", input.RawInput))
	contextBuilder.WriteString(fmt.Sprintf("Interaction count: %d\n", state.InteractionCount))
	contextBuilder.WriteString(fmt.Sprintf("Positive history: %d, Negative: %d\n", state.PositiveHistory, state.NegativeHistory))
	contextBuilder.WriteString(fmt.Sprintf("Current trust: %.2f\n", state.TrustLevel))

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{Role: "user", Content: contextBuilder.String()},
		},
		Temperature: 0.3,
	}

	resp, err := l.llm.Chat(ctx, req)
	if err == nil {
		*tokensUsed = resp.TokensUsed
		*modelUsed = resp.Model
	}
}

func (l *RapportLobe) determineSuggestedApproach(state *RelationshipState) {
	switch state.RelationshipTone {
	case "warm":
		state.SuggestedApproach = "casual-friendly"
	case "friendly":
		state.SuggestedApproach = "friendly-professional"
	case "cautious":
		state.SuggestedApproach = "careful-supportive"
	default:
		state.SuggestedApproach = "balanced-helpful"
	}

	if state.FamiliarityLevel > 0.7 {
		state.SuggestedApproach += " with callbacks to shared history"
	}
}

func (l *RapportLobe) calculateConfidence(state *RelationshipState) float64 {
	baseConfidence := 0.5

	if state.InteractionCount > 10 {
		baseConfidence += 0.2
	} else if state.InteractionCount > 3 {
		baseConfidence += 0.1
	}

	if state.PositiveHistory > 0 || state.NegativeHistory > 0 {
		baseConfidence += 0.1
	}

	return min(1.0, baseConfidence)
}

// CanHandle returns confidence for relationship/social tasks.
func (l *RapportLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)

	socialSignals := []string{"how are you", "nice to", "pleasure", "relationship", "trust", "rapport", "remember when"}
	for _, signal := range socialSignals {
		if strings.Contains(lowerInput, signal) {
			return 0.85
		}
	}

	feedbackSignals := []string{"thanks", "thank you", "appreciate", "great job", "well done", "not helpful", "wrong"}
	for _, signal := range feedbackSignals {
		if strings.Contains(lowerInput, signal) {
			return 0.7
		}
	}

	return 0.2
}

// ResourceEstimate returns low resource requirements.
func (l *RapportLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 200,
		EstimatedTime:   300 * time.Millisecond,
		RequiresGPU:     false,
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
