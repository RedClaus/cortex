package voice

import "time"

// InterruptType represents the type of cognitive interrupt.
type InterruptType int

const (
	// InterruptTypeUserSpeech indicates the user started speaking during TTS playback.
	InterruptTypeUserSpeech InterruptType = iota

	// InterruptTypeManual indicates a manual cancellation (e.g., button press).
	InterruptTypeManual

	// InterruptTypeTimeout indicates a timeout or session expiry.
	InterruptTypeTimeout

	// InterruptTypeError indicates an error condition triggered the interrupt.
	InterruptTypeError
)

// String returns the string representation of the interrupt type.
func (t InterruptType) String() string {
	switch t {
	case InterruptTypeUserSpeech:
		return "user_speech"
	case InterruptTypeManual:
		return "manual"
	case InterruptTypeTimeout:
		return "timeout"
	case InterruptTypeError:
		return "error"
	default:
		return "unknown"
	}
}

// InterruptSignal represents a cognitive interrupt that should stop current processing.
// Interrupts are raised when:
// - User starts speaking while AI is responding (most common)
// - Manual cancellation via UI
// - Timeout or error conditions
type InterruptSignal struct {
	// Type is the interrupt type (user_speech, manual, timeout, error)
	Type InterruptType

	// Reason is a human-readable explanation
	Reason string

	// Timestamp is when the interrupt occurred
	Timestamp time.Time

	// SessionID is the voice session that triggered this interrupt
	SessionID string

	// Metadata contains additional context-specific information
	Metadata map[string]interface{}
}

// NewInterruptSignal creates a new interrupt signal with the given parameters.
func NewInterruptSignal(typ InterruptType, reason, sessionID string) *InterruptSignal {
	return &InterruptSignal{
		Type:      typ,
		Reason:    reason,
		Timestamp: time.Now(),
		SessionID: sessionID,
		Metadata:  make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to the interrupt signal and returns the signal for chaining.
func (i *InterruptSignal) WithMetadata(key string, value interface{}) *InterruptSignal {
	i.Metadata[key] = value
	return i
}
