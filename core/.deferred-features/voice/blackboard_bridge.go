// Package voice provides voice processing capabilities for Cortex.
// blackboard_bridge.go bridges voice events to the cognitive Blackboard.
package voice

import (
	"sync"
	"time"

	"github.com/normanking/cortex/internal/bus"
	"github.com/normanking/cortex/pkg/brain"
	"github.com/rs/zerolog/log"
)

// BlackboardBridge connects voice emotion events to the cognitive Blackboard.
// Brain Alignment: This bridge enables multimodal emotion processing by feeding
// voice-detected emotions to the Emotion Lobe via the shared Blackboard.
type BlackboardBridge struct {
	eventBus    *bus.EventBus
	mu          sync.RWMutex
	subscribers []bus.Subscription

	// Current voice emotion state (refreshed on each event)
	currentEmotion *VoiceEmotionState

	// Configuration
	config BlackboardBridgeConfig
}

// BlackboardBridgeConfig configures the bridge behavior.
type BlackboardBridgeConfig struct {
	// EmotionDecayTime is how long voice emotion remains valid
	EmotionDecayTime time.Duration

	// MinConfidence filters out low-confidence detections
	MinConfidence float64

	// EnableAudioEvents enables audio event bridging
	EnableAudioEvents bool
}

// DefaultBlackboardBridgeConfig returns sensible defaults.
func DefaultBlackboardBridgeConfig() BlackboardBridgeConfig {
	return BlackboardBridgeConfig{
		EmotionDecayTime:  5 * time.Second, // Voice emotion expires after 5s
		MinConfidence:     0.3,              // Filter out < 30% confidence
		EnableAudioEvents: true,
	}
}

// VoiceEmotionState represents the current voice emotion state.
// This is what gets written to the Blackboard.
type VoiceEmotionState struct {
	// Primary detected emotion
	Primary string `json:"primary"`

	// Confidence in the detection
	Confidence float64 `json:"confidence"`

	// All emotion scores
	All map[string]float64 `json:"all,omitempty"`

	// When this was detected
	DetectedAt time.Time `json:"detected_at"`

	// Source backend
	Backend string `json:"backend"`

	// Associated transcript (if any)
	Transcript string `json:"transcript,omitempty"`
}

// IsValid checks if the emotion state is still valid (not expired).
func (s *VoiceEmotionState) IsValid(decayTime time.Duration) bool {
	if s == nil {
		return false
	}
	return time.Since(s.DetectedAt) < decayTime
}

// NewBlackboardBridge creates a new bridge with the given event bus.
func NewBlackboardBridge(eventBus *bus.EventBus, config BlackboardBridgeConfig) *BlackboardBridge {
	return &BlackboardBridge{
		eventBus:    eventBus,
		subscribers: make([]bus.Subscription, 0),
		config:      config,
	}
}

// Start begins listening for voice emotion events.
func (b *BlackboardBridge) Start() error {
	if b.eventBus == nil {
		return nil // No event bus, nothing to subscribe to
	}

	// Subscribe to voice emotion events
	sub := b.eventBus.Subscribe(EventTypeVoiceEmotion, b.handleVoiceEmotionEvent)
	b.subscribers = append(b.subscribers, sub)

	// Subscribe to audio events if enabled
	if b.config.EnableAudioEvents {
		sub := b.eventBus.Subscribe(EventTypeVoiceAudioEvent, b.handleVoiceAudioEvent)
		b.subscribers = append(b.subscribers, sub)
	}

	log.Info().Msg("blackboard bridge started - listening for voice emotion events")
	return nil
}

// Stop unsubscribes from all events.
func (b *BlackboardBridge) Stop() {
	for _, sub := range b.subscribers {
		sub.Unsubscribe()
	}
	b.subscribers = nil
	log.Info().Msg("blackboard bridge stopped")
}

// handleVoiceEmotionEvent processes voice emotion events.
func (b *BlackboardBridge) handleVoiceEmotionEvent(event bus.Event) {
	emotionEvent, ok := event.(*VoiceEmotionEvent)
	if !ok {
		return
	}

	// Filter low-confidence detections
	if emotionEvent.Confidence < b.config.MinConfidence {
		log.Debug().
			Float64("confidence", emotionEvent.Confidence).
			Float64("min_confidence", b.config.MinConfidence).
			Msg("voice emotion filtered (low confidence)")
		return
	}

	b.mu.Lock()
	b.currentEmotion = &VoiceEmotionState{
		Primary:    emotionEvent.PrimaryEmotion,
		Confidence: emotionEvent.Confidence,
		All:        emotionEvent.AllEmotions,
		DetectedAt: time.Now(),
		Backend:    emotionEvent.Backend,
		Transcript: emotionEvent.TranscriptText,
	}
	b.mu.Unlock()

	log.Debug().
		Str("emotion", emotionEvent.PrimaryEmotion).
		Float64("confidence", emotionEvent.Confidence).
		Str("backend", emotionEvent.Backend).
		Msg("voice emotion captured for blackboard")
}

// handleVoiceAudioEvent processes audio event events.
func (b *BlackboardBridge) handleVoiceAudioEvent(event bus.Event) {
	audioEvent, ok := event.(*VoiceAudioEventEvent)
	if !ok {
		return
	}

	// Filter low-confidence detections
	if audioEvent.Confidence < b.config.MinConfidence {
		return
	}

	log.Debug().
		Str("event_type", audioEvent.EventType).
		Float64("confidence", audioEvent.Confidence).
		Str("backend", audioEvent.Backend).
		Msg("audio event detected")
}

// GetCurrentEmotion returns the current voice emotion state.
func (b *BlackboardBridge) GetCurrentEmotion() *VoiceEmotionState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.currentEmotion == nil {
		return nil
	}

	// Check if still valid
	if !b.currentEmotion.IsValid(b.config.EmotionDecayTime) {
		return nil
	}

	// Return a copy
	copy := *b.currentEmotion
	return &copy
}

// WriteToBlackboard writes the current voice emotion to a Blackboard.
// Call this before processing a request through the cognitive pipeline.
func (b *BlackboardBridge) WriteToBlackboard(bb *brain.Blackboard) {
	emotion := b.GetCurrentEmotion()
	if emotion == nil {
		return
	}

	// Write to blackboard with standard key
	bb.Set("voice_emotion", emotion)
	bb.Set("voice_emotion_primary", emotion.Primary)
	bb.Set("voice_emotion_confidence", emotion.Confidence)

	log.Debug().
		Str("emotion", emotion.Primary).
		Float64("confidence", emotion.Confidence).
		Msg("voice emotion written to blackboard")
}

// ClearCurrentEmotion clears the current voice emotion state.
func (b *BlackboardBridge) ClearCurrentEmotion() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentEmotion = nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Blackboard Keys for Voice Emotion
// ─────────────────────────────────────────────────────────────────────────────

// Blackboard key constants for voice emotion data.
const (
	// BlackboardKeyVoiceEmotion is the key for the full VoiceEmotionState
	BlackboardKeyVoiceEmotion = "voice_emotion"

	// BlackboardKeyVoiceEmotionPrimary is the key for just the primary emotion string
	BlackboardKeyVoiceEmotionPrimary = "voice_emotion_primary"

	// BlackboardKeyVoiceEmotionConfidence is the key for the emotion confidence
	BlackboardKeyVoiceEmotionConfidence = "voice_emotion_confidence"
)

// GetVoiceEmotionFromBlackboard extracts voice emotion from a Blackboard.
// Returns nil if no voice emotion is present.
func GetVoiceEmotionFromBlackboard(bb *brain.Blackboard) *VoiceEmotionState {
	if bb == nil {
		return nil
	}

	val, ok := bb.Get(BlackboardKeyVoiceEmotion)
	if !ok {
		return nil
	}

	emotion, ok := val.(*VoiceEmotionState)
	if !ok {
		return nil
	}

	return emotion
}

// ─────────────────────────────────────────────────────────────────────────────
// STT Result to Event Converter
// ─────────────────────────────────────────────────────────────────────────────

// EmitEventsFromSTTResult publishes voice emotion and audio events from an STT result.
// Call this after receiving a transcription result with emotion data.
func EmitEventsFromSTTResult(eventBus *bus.EventBus, sessionID string, result *STTResult) {
	if eventBus == nil || result == nil {
		return
	}

	// Emit voice emotion event
	if result.Emotion != nil {
		emotionEvent := NewVoiceEmotionEvent(
			sessionID,
			result.Emotion.Primary,
			result.Emotion.Confidence,
			result.Backend,
		).WithAllEmotions(result.Emotion.All).
			WithTranscript(result.Text)

		eventBus.Publish(emotionEvent)

		log.Debug().
			Str("emotion", result.Emotion.Primary).
			Float64("confidence", result.Emotion.Confidence).
			Msg("voice emotion event emitted")
	}

	// Emit audio event events
	for _, audioEvent := range result.AudioEvents {
		event := NewVoiceAudioEventEvent(
			sessionID,
			audioEvent.Type,
			audioEvent.Confidence,
			result.Backend,
		).WithTimeRange(audioEvent.StartTime, audioEvent.EndTime)

		eventBus.Publish(event)

		log.Debug().
			Str("event_type", audioEvent.Type).
			Float64("confidence", audioEvent.Confidence).
			Msg("audio event emitted")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Global Bridge Singleton
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalBlackboardBridge     *BlackboardBridge
	globalBlackboardBridgeOnce sync.Once
)

// GetBlackboardBridge returns the global bridge instance.
// Note: Requires SetGlobalEventBus to be called first.
func GetBlackboardBridge() *BlackboardBridge {
	globalBlackboardBridgeOnce.Do(func() {
		// Create with nil event bus - will be set later
		globalBlackboardBridge = NewBlackboardBridge(nil, DefaultBlackboardBridgeConfig())
	})
	return globalBlackboardBridge
}

// InitBlackboardBridge initializes the global bridge with an event bus.
func InitBlackboardBridge(eventBus *bus.EventBus) *BlackboardBridge {
	bridge := GetBlackboardBridge()
	bridge.eventBus = eventBus
	if err := bridge.Start(); err != nil {
		log.Error().Err(err).Msg("failed to start blackboard bridge")
	}
	return bridge
}
