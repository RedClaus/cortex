package lobes

import (
	"context"
	"testing"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.ChatResponse{
		Content:    m.response,
		Model:      "mock-model",
		TokensUsed: 100,
	}, nil
}

type mockMemoryStore struct {
	memories []brain.Memory
}

func (m *mockMemoryStore) Search(ctx context.Context, query string, limit int) ([]brain.Memory, error) {
	return m.memories, nil
}

func (m *mockMemoryStore) Store(ctx context.Context, content string, metadata map[string]string) error {
	return nil
}

func TestEmotionLobe_ID(t *testing.T) {
	lobe := NewEmotionLobe(nil)
	assert.Equal(t, brain.LobeEmotion, lobe.ID())
}

func TestEmotionLobe_CanHandle(t *testing.T) {
	lobe := NewEmotionLobe(nil)

	tests := []struct {
		input   string
		minConf float64
		maxConf float64
	}{
		{"I feel happy today", 0.8, 1.0},
		{"This makes me frustrated", 0.8, 1.0},
		{"I'm excited about the project", 0.8, 1.0},
		{"Help me with this task", 0.4, 0.7},
		{"Calculate the sum of 2+2", 0.1, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			conf := lobe.CanHandle(tt.input)
			assert.GreaterOrEqual(t, conf, tt.minConf, "confidence should be >= %v for: %s", tt.minConf, tt.input)
			assert.LessOrEqual(t, conf, tt.maxConf, "confidence should be <= %v for: %s", tt.maxConf, tt.input)
		})
	}
}

func TestEmotionLobe_Process(t *testing.T) {
	lobe := NewEmotionLobe(&mockLLM{response: `{"sentiment": 0.8}`})
	bb := brain.NewBlackboard()

	input := brain.LobeInput{RawInput: "I'm so happy about this!"}
	result, err := lobe.Process(context.Background(), input, bb)

	require.NoError(t, err)
	assert.Equal(t, brain.LobeEmotion, result.LobeID)
	assert.NotNil(t, result.Content)
	assert.Greater(t, result.Confidence, 0.0)
	assert.NotZero(t, result.Meta.Duration)
}

func TestEmotionLobe_QuickSentiment(t *testing.T) {
	lobe := NewEmotionLobe(nil)
	bb := brain.NewBlackboard()

	positiveInput := brain.LobeInput{RawInput: "Thank you! This is amazing!"}
	result, err := lobe.Process(context.Background(), positiveInput, bb)
	require.NoError(t, err)

	emotionResult, ok := result.Content.(*EmotionResult)
	require.True(t, ok)
	assert.Greater(t, emotionResult.Sentiment, 0.0)

	negativeInput := brain.LobeInput{RawInput: "I hate this, it's terrible and frustrating"}
	result, err = lobe.Process(context.Background(), negativeInput, bb)
	require.NoError(t, err)

	emotionResult, ok = result.Content.(*EmotionResult)
	require.True(t, ok)
	assert.Less(t, emotionResult.Sentiment, 0.0)
}

func TestEmotionLobe_ResourceEstimate(t *testing.T) {
	lobe := NewEmotionLobe(nil)
	estimate := lobe.ResourceEstimate(brain.LobeInput{})

	assert.Greater(t, estimate.EstimatedTokens, 0)
	assert.Greater(t, estimate.EstimatedTime, time.Duration(0))
}

func TestTheoryOfMindLobe_ID(t *testing.T) {
	lobe := NewTheoryOfMindLobe(nil)
	assert.Equal(t, brain.LobeTheoryOfMind, lobe.ID())
}

func TestTheoryOfMindLobe_CanHandle(t *testing.T) {
	lobe := NewTheoryOfMindLobe(nil)

	tests := []struct {
		input   string
		minConf float64
	}{
		{"What do you think I meant?", 0.8},
		{"Do you understand my perspective?", 0.8},
		{"I need help with this", 0.5},
		{"Calculate 2+2", 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			conf := lobe.CanHandle(tt.input)
			assert.GreaterOrEqual(t, conf, tt.minConf)
		})
	}
}

func TestTheoryOfMindLobe_Process(t *testing.T) {
	lobe := NewTheoryOfMindLobe(&mockLLM{response: `{}`})
	bb := brain.NewBlackboard()

	input := brain.LobeInput{RawInput: "How do I implement a REST API?"}
	result, err := lobe.Process(context.Background(), input, bb)

	require.NoError(t, err)
	assert.Equal(t, brain.LobeTheoryOfMind, result.LobeID)
	assert.NotNil(t, result.Content)

	userModel, ok := result.Content.(*UserModel)
	require.True(t, ok)
	assert.NotEmpty(t, userModel.InferredIntent)
}

func TestTheoryOfMindLobe_ExpertiseDetection(t *testing.T) {
	lobe := NewTheoryOfMindLobe(nil)
	bb := brain.NewBlackboard()

	beginnerInput := brain.LobeInput{RawInput: "What is a variable? I don't understand how to start"}
	result, _ := lobe.Process(context.Background(), beginnerInput, bb)
	userModel := result.Content.(*UserModel)
	assert.Equal(t, "beginner", userModel.ExpertiseLevel)

	expertInput := brain.LobeInput{RawInput: "How do I optimize the database query performance with proper indexing?"}
	result, _ = lobe.Process(context.Background(), expertInput, bb)
	userModel = result.Content.(*UserModel)
	assert.Equal(t, "expert", userModel.ExpertiseLevel)
}

func TestTheoryOfMindLobe_ResourceEstimate(t *testing.T) {
	lobe := NewTheoryOfMindLobe(nil)
	estimate := lobe.ResourceEstimate(brain.LobeInput{})

	assert.Greater(t, estimate.EstimatedTokens, 0)
	assert.Greater(t, estimate.EstimatedTime, time.Duration(0))
}

func TestRapportLobe_ID(t *testing.T) {
	lobe := NewRapportLobe(nil)
	assert.Equal(t, brain.LobeRapport, lobe.ID())
}

func TestRapportLobe_CanHandle(t *testing.T) {
	lobe := NewRapportLobe(nil)

	tests := []struct {
		input   string
		minConf float64
	}{
		{"How are you doing?", 0.8},
		{"Thank you so much!", 0.6},
		{"This is not helpful at all", 0.6},
		{"Calculate 2+2", 0.1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			conf := lobe.CanHandle(tt.input)
			assert.GreaterOrEqual(t, conf, tt.minConf)
		})
	}
}

func TestRapportLobe_Process(t *testing.T) {
	lobe := NewRapportLobe(nil)
	bb := brain.NewBlackboard()
	bb.ConversationID = "test-convo-1"

	input := brain.LobeInput{RawInput: "Thanks for the help!"}
	result, err := lobe.Process(context.Background(), input, bb)

	require.NoError(t, err)
	assert.Equal(t, brain.LobeRapport, result.LobeID)

	state, ok := result.Content.(*RelationshipState)
	require.True(t, ok)
	assert.Equal(t, 1, state.InteractionCount)
	assert.Greater(t, state.TrustLevel, 0.5)
}

func TestRapportLobe_TrustTracking(t *testing.T) {
	lobe := NewRapportLobe(nil)
	bb := brain.NewBlackboard()
	bb.ConversationID = "trust-test"

	positiveInput := brain.LobeInput{RawInput: "This is great! Thank you!"}
	result, _ := lobe.Process(context.Background(), positiveInput, bb)
	state := result.Content.(*RelationshipState)
	initialTrust := state.TrustLevel

	positiveInput2 := brain.LobeInput{RawInput: "Awesome work, I appreciate it!"}
	result, _ = lobe.Process(context.Background(), positiveInput2, bb)
	state = result.Content.(*RelationshipState)
	assert.Greater(t, state.TrustLevel, initialTrust)

	negativeInput := brain.LobeInput{RawInput: "This is wrong and terrible"}
	result, _ = lobe.Process(context.Background(), negativeInput, bb)
	state = result.Content.(*RelationshipState)
	assert.Less(t, state.TrustLevel, initialTrust+0.1)
}

func TestRapportLobe_ResourceEstimate(t *testing.T) {
	lobe := NewRapportLobe(nil)
	estimate := lobe.ResourceEstimate(brain.LobeInput{})

	assert.Greater(t, estimate.EstimatedTokens, 0)
	assert.Greater(t, estimate.EstimatedTime, time.Duration(0))
}

func TestReasoningLobe_ID(t *testing.T) {
	lobe := NewReasoningLobe(nil)
	assert.Equal(t, brain.LobeReasoning, lobe.ID())
}

func TestReasoningLobe_CanHandle(t *testing.T) {
	lobe := NewReasoningLobe(nil)

	highConf := lobe.CanHandle("Why does this happen?")
	assert.Greater(t, highConf, 0.5)

	lowConf := lobe.CanHandle("Create a file")
	assert.Less(t, lowConf, 0.5)
}

func TestReasoningLobe_Process(t *testing.T) {
	lobe := NewReasoningLobe(&mockLLM{response: "Analysis complete"})
	bb := brain.NewBlackboard()

	input := brain.LobeInput{RawInput: "Why does water boil at 100 degrees?"}
	result, err := lobe.Process(context.Background(), input, bb)

	require.NoError(t, err)
	assert.Equal(t, brain.LobeReasoning, result.LobeID)
	assert.Equal(t, "Analysis complete", result.Content)
}

func TestCodingLobe_ID(t *testing.T) {
	lobe := NewCodingLobe(nil)
	assert.Equal(t, brain.LobeCoding, lobe.ID())
}

func TestCodingLobe_CanHandle(t *testing.T) {
	lobe := NewCodingLobe(nil)

	highConf := lobe.CanHandle("Write a function to sort an array")
	assert.Greater(t, highConf, 0.5)

	lowConf := lobe.CanHandle("What is the weather?")
	assert.Less(t, lowConf, 0.5)
}

func TestSafetyLobe_ID(t *testing.T) {
	lobe := NewSafetyLobe()
	assert.Equal(t, brain.LobeSafety, lobe.ID())
}

func TestSafetyLobe_CanHandle(t *testing.T) {
	lobe := NewSafetyLobe()

	highConf := lobe.CanHandle("rm -rf /")
	assert.Greater(t, highConf, 0.5)
}

func TestMemoryLobe_ID(t *testing.T) {
	lobe := NewMemoryLobe(&mockMemoryStore{})
	assert.Equal(t, brain.LobeMemory, lobe.ID())
}

func TestMemoryLobe_CanHandle(t *testing.T) {
	lobe := NewMemoryLobe(&mockMemoryStore{})

	highConf := lobe.CanHandle("Remember what I said yesterday")
	assert.Greater(t, highConf, 0.5)
}

func TestPlanningLobe_ID(t *testing.T) {
	lobe := NewPlanningLobe(nil)
	assert.Equal(t, brain.LobePlanning, lobe.ID())
}

func TestPlanningLobe_CanHandle(t *testing.T) {
	lobe := NewPlanningLobe(nil)

	highConf := lobe.CanHandle("Plan the steps for this project")
	assert.Greater(t, highConf, 0.5)
}

func TestCreativityLobe_ID(t *testing.T) {
	lobe := NewCreativityLobe(nil)
	assert.Equal(t, brain.LobeCreativity, lobe.ID())
}

func TestCreativityLobe_CanHandle(t *testing.T) {
	lobe := NewCreativityLobe(nil)

	highConf := lobe.CanHandle("Brainstorm ideas for a new app")
	assert.Greater(t, highConf, 0.5)
}

func TestLogicLobe_ID(t *testing.T) {
	lobe := NewLogicLobe(nil)
	assert.Equal(t, brain.LobeLogic, lobe.ID())
}

func TestLogicLobe_CanHandle(t *testing.T) {
	lobe := NewLogicLobe(nil)
	conf := lobe.CanHandle("Calculate something")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestTemporalLobe_ID(t *testing.T) {
	lobe := NewTemporalLobe(nil)
	assert.Equal(t, brain.LobeTemporal, lobe.ID())
}

func TestTemporalLobe_CanHandle(t *testing.T) {
	lobe := NewTemporalLobe(nil)
	conf := lobe.CanHandle("What time is it?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestSpatialLobe_ID(t *testing.T) {
	lobe := NewSpatialLobe(nil)
	assert.Equal(t, brain.LobeSpatial, lobe.ID())
}

func TestSpatialLobe_CanHandle(t *testing.T) {
	lobe := NewSpatialLobe(nil)
	conf := lobe.CanHandle("Position something")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestCausalLobe_ID(t *testing.T) {
	lobe := NewCausalLobe(nil)
	assert.Equal(t, brain.LobeCausal, lobe.ID())
}

func TestCausalLobe_CanHandle(t *testing.T) {
	lobe := NewCausalLobe(nil)
	conf := lobe.CanHandle("What caused this?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestAttentionLobe_ID(t *testing.T) {
	lobe := NewAttentionLobe()
	assert.Equal(t, brain.LobeAttention, lobe.ID())
}

func TestAttentionLobe_CanHandle(t *testing.T) {
	lobe := NewAttentionLobe()
	conf := lobe.CanHandle("Focus on this")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestMetacognitionLobe_ID(t *testing.T) {
	lobe := NewMetacognitionLobe()
	assert.Equal(t, brain.LobeMetacognition, lobe.ID())
}

func TestMetacognitionLobe_CanHandle(t *testing.T) {
	lobe := NewMetacognitionLobe()
	conf := lobe.CanHandle("Are you confident?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestInhibitionLobe_ID(t *testing.T) {
	lobe := NewInhibitionLobe()
	assert.Equal(t, brain.LobeInhibition, lobe.ID())
}

func TestInhibitionLobe_CanHandle(t *testing.T) {
	lobe := NewInhibitionLobe()
	conf := lobe.CanHandle("Wait before responding")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestSelfKnowledgeLobe_ID(t *testing.T) {
	lobe := NewSelfKnowledgeLobe()
	assert.Equal(t, brain.LobeSelfKnowledge, lobe.ID())
}

func TestSelfKnowledgeLobe_CanHandle(t *testing.T) {
	lobe := NewSelfKnowledgeLobe()
	conf := lobe.CanHandle("What can you do?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestVisionLobe_ID(t *testing.T) {
	lobe := NewVisionLobe(nil)
	assert.Equal(t, brain.LobeVision, lobe.ID())
}

func TestVisionLobe_CanHandle(t *testing.T) {
	lobe := NewVisionLobe(nil)
	conf := lobe.CanHandle("What's in this image?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestAuditionLobe_ID(t *testing.T) {
	lobe := NewAuditionLobe(nil)
	assert.Equal(t, brain.LobeAudition, lobe.ID())
}

func TestAuditionLobe_CanHandle(t *testing.T) {
	lobe := NewAuditionLobe(nil)
	conf := lobe.CanHandle("What's in this audio?")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestTextParsingLobe_ID(t *testing.T) {
	lobe := NewTextParsingLobe()
	assert.Equal(t, brain.LobeTextParsing, lobe.ID())
}

func TestTextParsingLobe_CanHandle(t *testing.T) {
	lobe := NewTextParsingLobe()
	conf := lobe.CanHandle("Extract entities")
	assert.GreaterOrEqual(t, conf, 0.0)
	assert.LessOrEqual(t, conf, 1.0)
}

func TestAllLobesImplementInterface(t *testing.T) {
	lobes := []brain.Lobe{
		NewEmotionLobe(nil),
		NewTheoryOfMindLobe(nil),
		NewRapportLobe(nil),
		NewReasoningLobe(nil),
		NewCodingLobe(nil),
		NewSafetyLobe(),
		NewMemoryLobe(&mockMemoryStore{}),
		NewPlanningLobe(nil),
		NewCreativityLobe(nil),
		NewLogicLobe(nil),
		NewTemporalLobe(nil),
		NewSpatialLobe(nil),
		NewCausalLobe(nil),
		NewAttentionLobe(),
		NewMetacognitionLobe(),
		NewInhibitionLobe(),
		NewSelfKnowledgeLobe(),
		NewVisionLobe(nil),
		NewAuditionLobe(nil),
		NewTextParsingLobe(),
	}

	for _, lobe := range lobes {
		t.Run(string(lobe.ID()), func(t *testing.T) {
			assert.NotEmpty(t, lobe.ID())
			assert.True(t, lobe.ID().Valid())

			conf := lobe.CanHandle("test input")
			assert.GreaterOrEqual(t, conf, 0.0)
			assert.LessOrEqual(t, conf, 1.0)

			estimate := lobe.ResourceEstimate(brain.LobeInput{})
			assert.GreaterOrEqual(t, estimate.EstimatedTokens, 0)
			assert.GreaterOrEqual(t, estimate.EstimatedTime, time.Duration(0))
		})
	}
}

func TestLobeCountMatchesAllLobes(t *testing.T) {
	allLobeIDs := brain.AllLobes()
	assert.Len(t, allLobeIDs, 20, "Should have exactly 20 lobe IDs defined")
}
