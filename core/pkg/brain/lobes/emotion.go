package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// EmotionResult represents the output of emotional analysis.
type EmotionResult struct {
	// Detected sentiment (-1.0 to 1.0, negative to positive)
	Sentiment float64 `json:"sentiment"`

	// Primary detected emotion
	PrimaryEmotion string `json:"primary_emotion"`

	// Secondary emotions with confidence scores
	SecondaryEmotions map[string]float64 `json:"secondary_emotions"`

	// Emotional intensity (0.0 to 1.0)
	Intensity float64 `json:"intensity"`

	// Suggested response tone
	SuggestedTone string `json:"suggested_tone"`

	// Whether emotional support seems needed
	NeedsSupport bool `json:"needs_support"`

	// CR-021: Multimodal emotion data
	// VoiceEmotion contains voice-based emotion if detected
	VoiceEmotion *VoiceEmotionInput `json:"voice_emotion,omitempty"`

	// EmotionSource indicates where the emotion was detected
	EmotionSource string `json:"emotion_source"` // "text", "voice", "fused"

	// FusionConfidence indicates confidence in multimodal fusion
	FusionConfidence float64 `json:"fusion_confidence,omitempty"`
}

// VoiceEmotionInput represents voice emotion data from the Blackboard.
// CR-021: This enables multimodal emotion processing.
type VoiceEmotionInput struct {
	Primary    string             `json:"primary"`
	Confidence float64            `json:"confidence"`
	All        map[string]float64 `json:"all,omitempty"`
	Backend    string             `json:"backend,omitempty"`
}

// EmotionLobe handles emotional state detection and response calibration.
// It analyzes user input for sentiment, emotion, and suggests appropriate
// response tones for empathetic interaction.
type EmotionLobe struct {
	llm LLMProvider
}

// NewEmotionLobe creates a new emotion processing lobe.
func NewEmotionLobe(llm LLMProvider) *EmotionLobe {
	return &EmotionLobe{
		llm: llm,
	}
}

// ID returns brain.LobeEmotion
func (l *EmotionLobe) ID() brain.LobeID {
	return brain.LobeEmotion
}

// Process analyzes emotional content in the input.
func (l *EmotionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	// Step 1: Quick text-based sentiment analysis
	quickResult := l.quickSentimentAnalysis(input.RawInput)
	quickResult.EmotionSource = "text"

	var emotionResult *EmotionResult
	var tokensUsed int
	var modelUsed string

	// Step 2: LLM-based emotion analysis (if available)
	if l.llm != nil {
		systemPrompt := `You are the Emotion Lobe of a cognitive system. Analyze the emotional content of the input.

Return a JSON object with:
{
  "sentiment": <float -1.0 to 1.0>,
  "primary_emotion": "<emotion name>",
  "secondary_emotions": {"<emotion>": <confidence 0-1>, ...},
  "intensity": <float 0.0 to 1.0>,
  "suggested_tone": "<empathetic|neutral|enthusiastic|calming|supportive>",
  "needs_support": <true|false>
}

Emotions to detect: joy, sadness, anger, fear, surprise, disgust, anticipation, trust, frustration, confusion, excitement, anxiety, contentment, disappointment`

		req := &llm.ChatRequest{
			Model:        "", // Use provider's default model
			SystemPrompt: systemPrompt,
			Messages: []llm.Message{
				{Role: "user", Content: fmt.Sprintf("Analyze the emotional content of: %s", input.RawInput)},
			},
			Temperature: 0.3,
		}

		resp, err := l.llm.Chat(ctx, req)
		if err != nil {
			emotionResult = quickResult
		} else {
			emotionResult = l.parseLLMResponse(resp.Content, quickResult)
			tokensUsed = resp.TokensUsed
			modelUsed = resp.Model
		}
	} else {
		emotionResult = quickResult
	}

	// Step 3: CR-021 - Extract voice emotion from Blackboard and fuse
	if bb != nil {
		voiceEmotion := l.extractVoiceEmotion(bb)
		if voiceEmotion != nil {
			emotionResult = l.fuseEmotions(emotionResult, voiceEmotion)
		}
	}

	// Update Blackboard with emotion state
	if bb != nil && bb.UserState == nil {
		bb.SetUserState(&brain.UserState{
			EstimatedMood: emotionResult.PrimaryEmotion,
			PreferredTone: emotionResult.SuggestedTone,
		})
	} else if bb != nil && bb.UserState != nil {
		bb.Set("detected_emotion", emotionResult.PrimaryEmotion)
		bb.Set("emotional_intensity", emotionResult.Intensity)
		bb.Set("emotion_source", emotionResult.EmotionSource)
	}

	result := &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    emotionResult,
		Confidence: l.calculateConfidence(emotionResult),
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: tokensUsed,
			ModelUsed:  modelUsed,
		},
	}

	if emotionResult.NeedsSupport && emotionResult.Intensity > 0.7 {
		result.RequestReplan = true
		result.ReplanReason = "High emotional distress detected - suggest supportive response path"
		result.SuggestLobes = []brain.LobeID{brain.LobeRapport, brain.LobeTheoryOfMind}
	}

	return result, nil
}

// quickSentimentAnalysis provides fast keyword-based emotion detection.
func (l *EmotionLobe) quickSentimentAnalysis(input string) *EmotionResult {
	lowerInput := strings.ToLower(input)

	result := &EmotionResult{
		Sentiment:         0.0,
		PrimaryEmotion:    "neutral",
		SecondaryEmotions: make(map[string]float64),
		Intensity:         0.3,
		SuggestedTone:     "neutral",
		NeedsSupport:      false,
	}

	// Positive indicators
	positiveWords := []string{"happy", "great", "awesome", "love", "excited", "wonderful", "amazing", "thanks", "thank you", "appreciate", "good", "excellent"}
	positiveCount := 0
	for _, word := range positiveWords {
		if strings.Contains(lowerInput, word) {
			positiveCount++
		}
	}

	// Negative indicators
	negativeWords := []string{"sad", "angry", "frustrated", "upset", "hate", "annoying", "terrible", "awful", "worried", "anxious", "scared", "confused", "stuck", "help"}
	negativeCount := 0
	for _, word := range negativeWords {
		if strings.Contains(lowerInput, word) {
			negativeCount++
		}
	}

	// Distress indicators
	distressWords := []string{"help", "please", "urgent", "emergency", "desperate", "can't", "don't know", "stuck", "lost"}
	for _, word := range distressWords {
		if strings.Contains(lowerInput, word) {
			result.NeedsSupport = true
			break
		}
	}

	// Calculate sentiment
	if positiveCount > negativeCount {
		result.Sentiment = float64(positiveCount) * 0.2
		if result.Sentiment > 1.0 {
			result.Sentiment = 1.0
		}
		result.PrimaryEmotion = "positive"
		result.SuggestedTone = "enthusiastic"
	} else if negativeCount > positiveCount {
		result.Sentiment = float64(negativeCount) * -0.2
		if result.Sentiment < -1.0 {
			result.Sentiment = -1.0
		}
		result.PrimaryEmotion = "negative"
		result.SuggestedTone = "supportive"
	}

	// Intensity based on exclamation marks and caps
	exclamations := strings.Count(input, "!")
	capsRatio := float64(len(strings.TrimFunc(input, func(r rune) bool { return r < 'A' || r > 'Z' }))) / float64(len(input)+1)
	result.Intensity = 0.3 + float64(exclamations)*0.1 + capsRatio*0.3
	if result.Intensity > 1.0 {
		result.Intensity = 1.0
	}

	return result
}

// parseLLMResponse parses the LLM's JSON response into EmotionResult.
func (l *EmotionLobe) parseLLMResponse(response string, fallback *EmotionResult) *EmotionResult {
	// In production, use proper JSON parsing
	// For now, return fallback with enhanced data
	return fallback
}

// calculateConfidence returns confidence based on analysis completeness.
func (l *EmotionLobe) calculateConfidence(result *EmotionResult) float64 {
	confidence := 0.5 // Base confidence

	if result.PrimaryEmotion != "neutral" {
		confidence += 0.2
	}
	if len(result.SecondaryEmotions) > 0 {
		confidence += 0.1
	}
	if result.Intensity > 0.5 {
		confidence += 0.1
	}
	if result.SuggestedTone != "" {
		confidence += 0.1
	}

	return confidence
}

// CanHandle returns confidence for emotion-related tasks.
func (l *EmotionLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)

	// High confidence for explicit emotional content
	emotionKeywords := []string{
		"feel", "feeling", "emotion", "mood", "happy", "sad", "angry",
		"frustrated", "excited", "anxious", "worried", "stressed",
		"love", "hate", "upset", "disappointed", "grateful", "thankful",
	}

	for _, keyword := range emotionKeywords {
		if strings.Contains(lowerInput, keyword) {
			return 0.85
		}
	}

	// Medium confidence for support-seeking
	supportKeywords := []string{"help", "please", "need", "want", "hope"}
	for _, keyword := range supportKeywords {
		if strings.Contains(lowerInput, keyword) {
			return 0.5
		}
	}

	// Low baseline - emotion can inform any response
	return 0.2
}

// ResourceEstimate returns low resource requirements.
func (l *EmotionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 300,
		EstimatedTime:   500 * time.Millisecond,
		RequiresGPU:     false,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CR-021: Voice Emotion Fusion
// ─────────────────────────────────────────────────────────────────────────────

// extractVoiceEmotion retrieves voice emotion data from the Blackboard.
func (l *EmotionLobe) extractVoiceEmotion(bb *brain.Blackboard) *VoiceEmotionInput {
	if bb == nil {
		return nil
	}

	// Try to get the full voice emotion state
	val, ok := bb.Get("voice_emotion")
	if !ok {
		return nil
	}

	// Handle different possible types from the Blackboard
	switch v := val.(type) {
	case *VoiceEmotionInput:
		return v
	case map[string]interface{}:
		// Parse from map (in case it was stored differently)
		return l.parseVoiceEmotionMap(v)
	default:
		return nil
	}
}

// parseVoiceEmotionMap parses a map into VoiceEmotionInput.
func (l *EmotionLobe) parseVoiceEmotionMap(m map[string]interface{}) *VoiceEmotionInput {
	result := &VoiceEmotionInput{}

	if primary, ok := m["primary"].(string); ok {
		result.Primary = primary
	} else if primary, ok := m["Primary"].(string); ok {
		result.Primary = primary
	}

	if conf, ok := m["confidence"].(float64); ok {
		result.Confidence = conf
	} else if conf, ok := m["Confidence"].(float64); ok {
		result.Confidence = conf
	}

	if all, ok := m["all"].(map[string]float64); ok {
		result.All = all
	} else if all, ok := m["All"].(map[string]interface{}); ok {
		result.All = make(map[string]float64)
		for k, v := range all {
			if f, ok := v.(float64); ok {
				result.All[k] = f
			}
		}
	}

	if backend, ok := m["backend"].(string); ok {
		result.Backend = backend
	}

	// Only return if we have valid data
	if result.Primary == "" || result.Confidence == 0 {
		return nil
	}

	return result
}

// fuseEmotions combines text and voice emotion signals.
// Brain Alignment: Multimodal emotion fusion mirrors how the brain integrates
// auditory and semantic emotional cues for more accurate assessment.
func (l *EmotionLobe) fuseEmotions(textResult *EmotionResult, voiceEmotion *VoiceEmotionInput) *EmotionResult {
	if voiceEmotion == nil {
		return textResult
	}

	result := &EmotionResult{
		Sentiment:         textResult.Sentiment,
		PrimaryEmotion:    textResult.PrimaryEmotion,
		SecondaryEmotions: textResult.SecondaryEmotions,
		Intensity:         textResult.Intensity,
		SuggestedTone:     textResult.SuggestedTone,
		NeedsSupport:      textResult.NeedsSupport,
		VoiceEmotion:      voiceEmotion,
		EmotionSource:     "fused",
	}

	// Fusion weights: voice emotion is often more reliable for certain emotions
	voiceWeight := l.calculateVoiceWeight(voiceEmotion)
	textWeight := 1.0 - voiceWeight

	// Map voice emotions to our emotion taxonomy
	mappedVoiceEmotion := l.mapVoiceEmotion(voiceEmotion.Primary)

	// Decide primary emotion based on confidence-weighted fusion
	if voiceEmotion.Confidence > 0.7 && voiceWeight > 0.5 {
		// High-confidence voice emotion takes precedence
		result.PrimaryEmotion = mappedVoiceEmotion
		result.EmotionSource = "voice"
	} else if textResult.Intensity > 0.7 && textWeight > voiceWeight {
		// High-intensity text emotion takes precedence
		result.EmotionSource = "text"
	} else {
		// True fusion - check for agreement or conflict
		if l.emotionsAgree(textResult.PrimaryEmotion, mappedVoiceEmotion) {
			// Agreement boosts confidence
			result.Intensity = min(1.0, textResult.Intensity+0.2)
			result.FusionConfidence = 0.9
		} else {
			// Conflict - use higher confidence signal
			if voiceEmotion.Confidence > 0.6 {
				result.PrimaryEmotion = mappedVoiceEmotion
			}
			result.FusionConfidence = 0.5
		}
	}

	// Fuse sentiment based on voice valence
	voiceSentiment := l.emotionToSentiment(voiceEmotion.Primary)
	result.Sentiment = textResult.Sentiment*textWeight + voiceSentiment*voiceWeight

	// Update suggested tone based on fused result
	result.SuggestedTone = l.suggestTone(result)

	// Merge secondary emotions with voice emotion scores
	if voiceEmotion.All != nil {
		if result.SecondaryEmotions == nil {
			result.SecondaryEmotions = make(map[string]float64)
		}
		for emotion, score := range voiceEmotion.All {
			mapped := l.mapVoiceEmotion(emotion)
			existing := result.SecondaryEmotions[mapped]
			result.SecondaryEmotions[mapped] = existing*textWeight + score*voiceWeight
		}
	}

	return result
}

// calculateVoiceWeight determines how much to weight voice emotion.
// Higher weight for high-confidence voice emotions.
func (l *EmotionLobe) calculateVoiceWeight(voice *VoiceEmotionInput) float64 {
	if voice == nil {
		return 0.0
	}

	// Base weight on confidence
	weight := voice.Confidence * 0.6 // Max 60% from voice

	// Boost for strong emotions that are easier to detect in voice
	strongVoiceEmotions := map[string]float64{
		"angry":     0.15,
		"happy":     0.10,
		"sad":       0.10,
		"fearful":   0.10,
		"surprised": 0.05,
	}

	if boost, ok := strongVoiceEmotions[voice.Primary]; ok {
		weight += boost
	}

	return min(0.7, weight) // Cap at 70% voice weight
}

// mapVoiceEmotion maps SenseVoice emotions to our taxonomy.
func (l *EmotionLobe) mapVoiceEmotion(voiceEmotion string) string {
	mapping := map[string]string{
		"happy":     "joy",
		"sad":       "sadness",
		"angry":     "anger",
		"surprised": "surprise",
		"fearful":   "fear",
		"disgusted": "disgust",
		"neutral":   "neutral",
	}

	if mapped, ok := mapping[voiceEmotion]; ok {
		return mapped
	}
	return voiceEmotion
}

// emotionsAgree checks if two emotions are semantically similar.
func (l *EmotionLobe) emotionsAgree(emotion1, emotion2 string) bool {
	// Same emotion
	if emotion1 == emotion2 {
		return true
	}

	// Positive cluster
	positive := map[string]bool{
		"joy": true, "happy": true, "excitement": true, "contentment": true,
		"anticipation": true, "trust": true,
	}
	if positive[emotion1] && positive[emotion2] {
		return true
	}

	// Negative cluster
	negative := map[string]bool{
		"sadness": true, "sad": true, "anger": true, "angry": true,
		"fear": true, "fearful": true, "disgust": true, "disgusted": true,
		"frustration": true, "anxiety": true, "disappointment": true,
	}
	if negative[emotion1] && negative[emotion2] {
		return true
	}

	return false
}

// emotionToSentiment converts an emotion to a sentiment value.
func (l *EmotionLobe) emotionToSentiment(emotion string) float64 {
	sentiments := map[string]float64{
		"happy":     0.8,
		"joy":       0.8,
		"sad":       -0.6,
		"sadness":   -0.6,
		"angry":     -0.7,
		"anger":     -0.7,
		"surprised": 0.2,
		"surprise":  0.2,
		"fearful":   -0.5,
		"fear":      -0.5,
		"disgusted": -0.6,
		"disgust":   -0.6,
		"neutral":   0.0,
	}

	if s, ok := sentiments[emotion]; ok {
		return s
	}
	return 0.0
}

// suggestTone suggests a response tone based on the fused emotion result.
func (l *EmotionLobe) suggestTone(result *EmotionResult) string {
	switch {
	case result.NeedsSupport:
		return "supportive"
	case result.Sentiment > 0.5:
		return "enthusiastic"
	case result.Sentiment < -0.5:
		if result.PrimaryEmotion == "anger" || result.PrimaryEmotion == "angry" {
			return "calming"
		}
		return "empathetic"
	case result.Intensity > 0.7:
		if result.Sentiment > 0 {
			return "enthusiastic"
		}
		return "empathetic"
	default:
		return "neutral"
	}
}

