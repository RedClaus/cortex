package voice

import (
	"time"

	"github.com/normanking/cortex/internal/bus"
)

// Voice event type constants for the event bus.
const (
	// EventTypeVoiceConnected is emitted when the voice bridge connects to the orchestrator.
	EventTypeVoiceConnected = "voice.connected"

	// EventTypeVoiceDisconnected is emitted when the voice bridge disconnects.
	EventTypeVoiceDisconnected = "voice.disconnected"

	// EventTypeVoiceTranscript is emitted when a transcription is received from STT.
	EventTypeVoiceTranscript = "voice.transcript"

	// EventTypeVoiceInterrupt is emitted when a cognitive interrupt occurs.
	EventTypeVoiceInterrupt = "voice.interrupt"

	// EventTypeVoiceSynthesizing is emitted when TTS synthesis starts.
	EventTypeVoiceSynthesizing = "voice.synthesizing"

	// EventTypeVoiceComplete is emitted when TTS synthesis completes.
	EventTypeVoiceComplete = "voice.complete"

	// EventTypeVoiceError is emitted when a voice error occurs.
	EventTypeVoiceError = "voice.error"

	// EventTypeVoiceStatus is emitted for general status updates.
	EventTypeVoiceStatus = "voice.status"

	// EventTypeVoiceWakeWord is emitted when a wake word is detected (CR-015).
	EventTypeVoiceWakeWord = "voice.wake_word"

	// EventTypeVoiceEmotion is emitted when voice emotion is detected (CR-021).
	EventTypeVoiceEmotion = "voice.emotion"

	// EventTypeVoiceAudioEvent is emitted when an audio event is detected (CR-021).
	EventTypeVoiceAudioEvent = "voice.audio_event"
)

// VoiceConnectedEvent is emitted when the voice bridge successfully connects
// to the Python voice orchestrator.
type VoiceConnectedEvent struct {
	bus.BaseEvent
	SessionID       string `json:"session_id"`
	OrchestratorURL string `json:"orchestrator_url"`
}

// NewVoiceConnectedEvent creates a new voice connected event.
func NewVoiceConnectedEvent(sessionID, orchestratorURL string) *VoiceConnectedEvent {
	return &VoiceConnectedEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceConnected,
			CreatedAt: time.Now(),
		},
		SessionID:       sessionID,
		OrchestratorURL: orchestratorURL,
	}
}

// VoiceDisconnectedEvent is emitted when the voice bridge disconnects.
type VoiceDisconnectedEvent struct {
	bus.BaseEvent
	SessionID string `json:"session_id"`
	Reason    string `json:"reason,omitempty"`
}

// NewVoiceDisconnectedEvent creates a new voice disconnected event.
func NewVoiceDisconnectedEvent(sessionID, reason string) *VoiceDisconnectedEvent {
	return &VoiceDisconnectedEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceDisconnected,
			CreatedAt: time.Now(),
		},
		SessionID: sessionID,
		Reason:    reason,
	}
}

// VoiceTranscriptEvent is emitted when a transcription is received from STT.
type VoiceTranscriptEvent struct {
	bus.BaseEvent
	SessionID    string  `json:"session_id"`
	Text         string  `json:"text"`
	IsFinal      bool    `json:"is_final"`
	Confidence   float64 `json:"confidence"`
	Language     string  `json:"language,omitempty"`
	OriginalText string  `json:"original_text,omitempty"` // Raw STT output before cleanup
	WasCleaned   bool    `json:"was_cleaned,omitempty"`   // True if cleanup modified text
	HadWakeWord  bool    `json:"had_wake_word,omitempty"` // True if wake word was detected and stripped
}

// NewVoiceTranscriptEvent creates a new voice transcript event.
func NewVoiceTranscriptEvent(sessionID, text string, isFinal bool, confidence float64) *VoiceTranscriptEvent {
	return &VoiceTranscriptEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceTranscript,
			CreatedAt: time.Now(),
		},
		SessionID:  sessionID,
		Text:       text,
		IsFinal:    isFinal,
		Confidence: confidence,
	}
}

// VoiceInterruptEvent is emitted when a cognitive interrupt occurs.
type VoiceInterruptEvent struct {
	bus.BaseEvent
	SessionID     string                 `json:"session_id"`
	InterruptType string                 `json:"interrupt_type"`
	Reason        string                 `json:"reason"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewVoiceInterruptEvent creates a new voice interrupt event.
func NewVoiceInterruptEvent(sessionID, interruptType, reason string) *VoiceInterruptEvent {
	return &VoiceInterruptEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceInterrupt,
			CreatedAt: time.Now(),
		},
		SessionID:     sessionID,
		InterruptType: interruptType,
		Reason:        reason,
		Metadata:      make(map[string]interface{}),
	}
}

// VoiceSynthesizingEvent is emitted when TTS synthesis starts.
type VoiceSynthesizingEvent struct {
	bus.BaseEvent
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	VoiceID   string `json:"voice_id,omitempty"`
	Provider  string `json:"provider,omitempty"`
}

// NewVoiceSynthesizingEvent creates a new voice synthesizing event.
func NewVoiceSynthesizingEvent(sessionID, text, voiceID string) *VoiceSynthesizingEvent {
	return &VoiceSynthesizingEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceSynthesizing,
			CreatedAt: time.Now(),
		},
		SessionID: sessionID,
		Text:      text,
		VoiceID:   voiceID,
	}
}

// VoiceCompleteEvent is emitted when TTS synthesis completes.
type VoiceCompleteEvent struct {
	bus.BaseEvent
	SessionID   string        `json:"session_id"`
	Text        string        `json:"text"`
	Duration    time.Duration `json:"duration_ns"`
	DurationMs  int64         `json:"duration_ms"`
	AudioLength float64       `json:"audio_length"` // in seconds
	Provider    string        `json:"provider,omitempty"`
	VoiceID     string        `json:"voice_id,omitempty"`
}

// NewVoiceCompleteEvent creates a new voice complete event.
func NewVoiceCompleteEvent(sessionID, text string, duration time.Duration) *VoiceCompleteEvent {
	return &VoiceCompleteEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceComplete,
			CreatedAt: time.Now(),
		},
		SessionID:  sessionID,
		Text:       text,
		Duration:   duration,
		DurationMs: duration.Milliseconds(),
	}
}

// VoiceErrorEvent is emitted when a voice error occurs.
type VoiceErrorEvent struct {
	bus.BaseEvent
	SessionID   string `json:"session_id"`
	Error       string `json:"error"`
	Component   string `json:"component,omitempty"` // "stt", "tts", "bridge"
	Recoverable bool   `json:"recoverable"`
}

// NewVoiceErrorEvent creates a new voice error event.
func NewVoiceErrorEvent(sessionID, error, component string, recoverable bool) *VoiceErrorEvent {
	return &VoiceErrorEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceError,
			CreatedAt: time.Now(),
		},
		SessionID:   sessionID,
		Error:       error,
		Component:   component,
		Recoverable: recoverable,
	}
}

// VoiceStatusEvent is emitted for general status updates.
type VoiceStatusEvent struct {
	bus.BaseEvent
	SessionID string                 `json:"session_id"`
	State     string                 `json:"state"` // "idle", "listening", "processing", "speaking"
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewVoiceStatusEvent creates a new voice status event.
func NewVoiceStatusEvent(sessionID, state string) *VoiceStatusEvent {
	return &VoiceStatusEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceStatus,
			CreatedAt: time.Now(),
		},
		SessionID: sessionID,
		State:     state,
		Metadata:  make(map[string]interface{}),
	}
}

// VoiceWakeWordEvent is emitted when a wake word is detected (CR-015).
// This is triggered by the pre-STT hotword detection before full transcription.
type VoiceWakeWordEvent struct {
	bus.BaseEvent
	SessionID   string  `json:"session_id"`
	WakeWord    string  `json:"wake_word"`    // Detected wake word (e.g., "hey_cortex", "hey_henry")
	Confidence  float64 `json:"confidence"`   // Detection confidence (0.0-1.0)
	AudioBase64 string  `json:"audio_base64"` // Pre-detection audio buffer (base64)
}

// NewVoiceWakeWordEvent creates a new voice wake word event.
func NewVoiceWakeWordEvent(sessionID, wakeWord string, confidence float64) *VoiceWakeWordEvent {
	return &VoiceWakeWordEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceWakeWord,
			CreatedAt: time.Now(),
		},
		SessionID:  sessionID,
		WakeWord:   wakeWord,
		Confidence: confidence,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Voice Emotion Events (CR-021)
// ─────────────────────────────────────────────────────────────────────────────

// VoiceEmotionEvent is emitted when voice-based emotion is detected (CR-021).
// Brain Alignment: This event feeds the Emotion Lobe with multimodal emotion signals,
// enabling fusion of voice emotion with text-based emotion analysis.
type VoiceEmotionEvent struct {
	bus.BaseEvent
	SessionID string `json:"session_id"`

	// Primary detected emotion (happy, sad, angry, surprised, fearful, disgusted, neutral)
	PrimaryEmotion string `json:"primary_emotion"`

	// Confidence in the primary emotion (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// All detected emotions with confidence scores
	AllEmotions map[string]float64 `json:"all_emotions,omitempty"`

	// Source transcript that triggered this emotion detection
	TranscriptText string `json:"transcript_text,omitempty"`

	// Backend that detected the emotion (e.g., "sensevoice")
	Backend string `json:"backend"`
}

// NewVoiceEmotionEvent creates a new voice emotion event.
func NewVoiceEmotionEvent(sessionID, primaryEmotion string, confidence float64, backend string) *VoiceEmotionEvent {
	return &VoiceEmotionEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceEmotion,
			CreatedAt: time.Now(),
		},
		SessionID:      sessionID,
		PrimaryEmotion: primaryEmotion,
		Confidence:     confidence,
		AllEmotions:    make(map[string]float64),
		Backend:        backend,
	}
}

// WithAllEmotions sets all emotion scores.
func (e *VoiceEmotionEvent) WithAllEmotions(emotions map[string]float64) *VoiceEmotionEvent {
	e.AllEmotions = emotions
	return e
}

// WithTranscript sets the source transcript.
func (e *VoiceEmotionEvent) WithTranscript(text string) *VoiceEmotionEvent {
	e.TranscriptText = text
	return e
}

// VoiceAudioEventEvent is emitted when an audio event is detected (CR-021).
// Examples: speech, music, laughter, applause, crying, coughing.
type VoiceAudioEventEvent struct {
	bus.BaseEvent
	SessionID string `json:"session_id"`

	// Type of audio event (speech, music, laughter, applause, etc.)
	EventType string `json:"event_type"`

	// Confidence in the event detection (0.0-1.0)
	Confidence float64 `json:"confidence"`

	// StartTime is the event start timestamp in seconds (relative to audio)
	StartTime float64 `json:"start_time,omitempty"`

	// EndTime is the event end timestamp in seconds (relative to audio)
	EndTime float64 `json:"end_time,omitempty"`

	// Backend that detected the event (e.g., "sensevoice")
	Backend string `json:"backend"`
}

// NewVoiceAudioEventEvent creates a new voice audio event event.
func NewVoiceAudioEventEvent(sessionID, eventType string, confidence float64, backend string) *VoiceAudioEventEvent {
	return &VoiceAudioEventEvent{
		BaseEvent: bus.BaseEvent{
			EventType: EventTypeVoiceAudioEvent,
			CreatedAt: time.Now(),
		},
		SessionID:  sessionID,
		EventType:  eventType,
		Confidence: confidence,
		Backend:    backend,
	}
}

// WithTimeRange sets the time range for the audio event.
func (e *VoiceAudioEventEvent) WithTimeRange(start, end float64) *VoiceAudioEventEvent {
	e.StartTime = start
	e.EndTime = end
	return e
}
