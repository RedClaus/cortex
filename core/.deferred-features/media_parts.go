// Package a2a contains A2A protocol extensions for audio/video streaming.
//
// This file defines custom message part types for multimedia content:
// - AudioPart: Audio data (TTS output, waveforms)
// - VideoPart: Video frames (vision stream)
// - AvatarStatePart: Real-time avatar animation state
package a2a

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/normanking/cortex/internal/avatar"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AUDIO PART
// ═══════════════════════════════════════════════════════════════════════════════

// AudioPart represents audio content in A2A messages.
// This extends the standard A2A parts to support voice/TTS output.
type AudioPart struct {
	// MimeType specifies the audio format (audio/wav, audio/mp3, audio/ogg)
	MimeType string `json:"mimeType"`

	// Data contains base64-encoded audio data
	Data string `json:"data"`

	// URL is an optional URL to fetch the audio (alternative to inline data)
	URL string `json:"url,omitempty"`

	// DurationMs is the audio duration in milliseconds
	DurationMs int64 `json:"durationMs"`

	// SampleRate is the audio sample rate (e.g., 44100, 22050)
	SampleRate int `json:"sampleRate,omitempty"`

	// Channels is the number of audio channels (1=mono, 2=stereo)
	Channels int `json:"channels,omitempty"`

	// Transcript is the text that was synthesized (for TTS audio)
	Transcript string `json:"transcript,omitempty"`

	// VoiceID identifies the voice used for TTS
	VoiceID string `json:"voiceId,omitempty"`

	// Phonemes contains timed phoneme data for lip sync
	Phonemes []PhonemeData `json:"phonemes,omitempty"`
}

// PhonemeData represents a single phoneme with timing for lip sync.
type PhonemeData struct {
	Phoneme   string  `json:"phoneme"`   // ARPAbet phoneme (e.g., "AA", "B", "CH")
	StartMs   int64   `json:"startMs"`   // Start time in milliseconds
	EndMs     int64   `json:"endMs"`     // End time in milliseconds
	Viseme    int     `json:"viseme"`    // Viseme group (0-14) for animation
	Intensity float64 `json:"intensity"` // Intensity 0-1
}

// PartType returns the A2A part type identifier.
func (p AudioPart) PartType() string {
	return "audio"
}

// NewAudioPart creates a new AudioPart from raw audio data.
func NewAudioPart(audioData []byte, mimeType string, durationMs int64) *AudioPart {
	return &AudioPart{
		MimeType:   mimeType,
		Data:       base64.StdEncoding.EncodeToString(audioData),
		DurationMs: durationMs,
	}
}

// NewAudioPartFromURL creates an AudioPart that references external audio.
func NewAudioPartFromURL(url string, mimeType string, durationMs int64) *AudioPart {
	return &AudioPart{
		MimeType:   mimeType,
		URL:        url,
		DurationMs: durationMs,
	}
}

// GetAudioData decodes and returns the raw audio bytes.
func (p *AudioPart) GetAudioData() ([]byte, error) {
	return base64.StdEncoding.DecodeString(p.Data)
}

// ═══════════════════════════════════════════════════════════════════════════════
// VIDEO PART
// ═══════════════════════════════════════════════════════════════════════════════

// VideoPart represents video content in A2A messages.
// This supports both single frames and video streams.
type VideoPart struct {
	// MimeType specifies the video/image format (video/webm, video/mp4, image/jpeg)
	MimeType string `json:"mimeType"`

	// Data contains base64-encoded video/frame data
	Data string `json:"data,omitempty"`

	// URL is an optional URL to fetch the video (alternative to inline data)
	URL string `json:"url,omitempty"`

	// StreamURL is a WebSocket URL for real-time video streaming
	StreamURL string `json:"streamUrl,omitempty"`

	// Width in pixels
	Width int `json:"width,omitempty"`

	// Height in pixels
	Height int `json:"height,omitempty"`

	// FrameRate for video content (frames per second)
	FrameRate float64 `json:"frameRate,omitempty"`

	// DurationMs for video clips
	DurationMs int64 `json:"durationMs,omitempty"`

	// Timestamp for this frame (for streams)
	Timestamp time.Time `json:"timestamp,omitempty"`

	// FrameNumber for sequencing
	FrameNumber int64 `json:"frameNumber,omitempty"`

	// Analysis contains vision analysis results
	Analysis *VisionAnalysis `json:"analysis,omitempty"`
}

// VisionAnalysis contains results from vision processing.
type VisionAnalysis struct {
	Description string           `json:"description,omitempty"`
	Objects     []DetectedObject `json:"objects,omitempty"`
	Text        string           `json:"text,omitempty"`
	Confidence  float64          `json:"confidence"`
	AnalyzedAt  time.Time        `json:"analyzedAt"`
}

// DetectedObject represents an object found in the video/image.
type DetectedObject struct {
	Label       string    `json:"label"`
	Confidence  float64   `json:"confidence"`
	BoundingBox [4]int    `json:"boundingBox"` // [x, y, width, height]
	Attributes  []string  `json:"attributes,omitempty"`
}

// PartType returns the A2A part type identifier.
func (p VideoPart) PartType() string {
	return "video"
}

// NewVideoPart creates a new VideoPart from frame data.
func NewVideoPart(frameData []byte, mimeType string, width, height int) *VideoPart {
	return &VideoPart{
		MimeType:  mimeType,
		Data:      base64.StdEncoding.EncodeToString(frameData),
		Width:     width,
		Height:    height,
		Timestamp: time.Now(),
	}
}

// NewVideoPartFromURL creates a VideoPart that references external video.
func NewVideoPartFromURL(url string, mimeType string) *VideoPart {
	return &VideoPart{
		MimeType: mimeType,
		URL:      url,
	}
}

// NewVideoStreamPart creates a VideoPart for WebSocket streaming.
func NewVideoStreamPart(streamURL string, width, height int, frameRate float64) *VideoPart {
	return &VideoPart{
		MimeType:  "video/stream",
		StreamURL: streamURL,
		Width:     width,
		Height:    height,
		FrameRate: frameRate,
	}
}

// GetFrameData decodes and returns the raw frame bytes.
func (p *VideoPart) GetFrameData() ([]byte, error) {
	return base64.StdEncoding.DecodeString(p.Data)
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVATAR STATE PART
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarStatePart represents real-time avatar animation state.
// This is streamed alongside audio for synchronized lip sync and expressions.
type AvatarStatePart struct {
	// SessionID links this state to a session
	SessionID string `json:"sessionId,omitempty"`

	// Timestamp when this state was captured
	Timestamp time.Time `json:"timestamp"`

	// Phoneme is the current mouth shape
	Phoneme string `json:"phoneme"`

	// Viseme is the viseme group (0-14) for 3D animation
	Viseme int `json:"viseme"`

	// PhonemeIntensity is the current phoneme intensity (0-1)
	PhonemeIntensity float64 `json:"phonemeIntensity"`

	// Emotion is the current emotional state
	Emotion *EmotionStateData `json:"emotion,omitempty"`

	// Gaze is the current gaze direction
	Gaze *GazeData `json:"gaze,omitempty"`

	// BlendShapes contains 3D blend shape weights for facial animation
	BlendShapes map[string]float64 `json:"blendShapes,omitempty"`

	// IsSpeaking indicates if the avatar is currently speaking
	IsSpeaking bool `json:"isSpeaking"`

	// IsThinking indicates if the avatar is processing/thinking
	IsThinking bool `json:"isThinking"`

	// SSE endpoint URL for real-time state streaming
	StreamURL string `json:"streamUrl,omitempty"`
}

// EmotionStateData represents emotional state for A2A transport.
type EmotionStateData struct {
	Primary       string             `json:"primary"`       // Primary emotion
	Secondary     map[string]float64 `json:"secondary,omitempty"`
	Valence       float64            `json:"valence"`       // -1 to 1
	Arousal       float64            `json:"arousal"`       // 0 to 1
	SuggestedTone string             `json:"suggestedTone,omitempty"`
}

// GazeData represents gaze direction for A2A transport.
type GazeData struct {
	X         float64 `json:"x"`         // -1 (left) to 1 (right)
	Y         float64 `json:"y"`         // -1 (down) to 1 (up)
	BlinkRate float64 `json:"blinkRate"` // Blinks per minute
}

// PartType returns the A2A part type identifier.
func (p AvatarStatePart) PartType() string {
	return "avatar_state"
}

// NewAvatarStatePart creates an AvatarStatePart from internal avatar state.
func NewAvatarStatePart(state *avatar.AvatarState) *AvatarStatePart {
	if state == nil {
		return &AvatarStatePart{
			Timestamp: time.Now(),
			Phoneme:   "rest",
			Viseme:    0,
		}
	}

	part := &AvatarStatePart{
		Timestamp:        state.Timestamp,
		Phoneme:          string(state.Phoneme),
		PhonemeIntensity: state.Intensity,
		IsSpeaking:       state.IsSpeaking,
		IsThinking:       state.IsThinking,
		SessionID:        state.SessionID,
	}

	// Convert emotion
	part.Emotion = &EmotionStateData{
		Primary:       state.Emotion.Primary,
		Valence:       state.Emotion.Valence,
		Arousal:       state.Emotion.Arousal,
		SuggestedTone: state.Emotion.SuggestedTone,
	}
	if state.Emotion.Secondary != nil {
		part.Emotion.Secondary = state.Emotion.Secondary
	}

	// Convert gaze
	part.Gaze = &GazeData{
		X:         state.Gaze.X,
		Y:         state.Gaze.Y,
		BlinkRate: state.Gaze.BlinkRate,
	}

	return part
}

// NewAvatarStateStreamPart creates an AvatarStatePart with SSE endpoint.
func NewAvatarStateStreamPart(streamURL string) *AvatarStatePart {
	return &AvatarStatePart{
		Timestamp: time.Now(),
		StreamURL: streamURL,
		Phoneme:   "rest",
		Viseme:    0,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// COMBINED MEDIA MESSAGE
// ═══════════════════════════════════════════════════════════════════════════════

// MediaMessage combines text, audio, video, and avatar state for rich responses.
// This is the primary structure for multimodal A2A communication.
type MediaMessage struct {
	// Text is the main text content
	Text string `json:"text,omitempty"`

	// Audio contains TTS audio output
	Audio *AudioPart `json:"audio,omitempty"`

	// Video contains video/vision content
	Video *VideoPart `json:"video,omitempty"`

	// AvatarState contains real-time animation state
	AvatarState *AvatarStatePart `json:"avatarState,omitempty"`

	// Metadata contains additional context
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ToJSON serializes the MediaMessage to JSON.
func (m *MediaMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON deserializes a MediaMessage from JSON.
func (m *MediaMessage) FromJSON(data []byte) error {
	return json.Unmarshal(data, m)
}

// HasAudio returns true if the message contains audio.
func (m *MediaMessage) HasAudio() bool {
	return m.Audio != nil && (m.Audio.Data != "" || m.Audio.URL != "")
}

// HasVideo returns true if the message contains video.
func (m *MediaMessage) HasVideo() bool {
	return m.Video != nil && (m.Video.Data != "" || m.Video.URL != "" || m.Video.StreamURL != "")
}

// HasAvatarState returns true if the message contains avatar state.
func (m *MediaMessage) HasAvatarState() bool {
	return m.AvatarState != nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS FOR A2A INTEGRATION
// ═══════════════════════════════════════════════════════════════════════════════

// ConvertPhonemes converts internal phonemes to A2A PhonemeData format.
func ConvertPhonemes(phonemes []avatar.TimedPhoneme, extractor avatar.PhonemeExtractor) []PhonemeData {
	result := make([]PhonemeData, len(phonemes))
	for i, p := range phonemes {
		viseme := 0
		if extractor != nil {
			viseme = extractor.PhonemeToViseme(p.Phoneme)
		}
		result[i] = PhonemeData{
			Phoneme:   string(p.Phoneme),
			StartMs:   p.StartMs,
			EndMs:     p.EndMs,
			Viseme:    viseme,
			Intensity: p.Intensity,
		}
	}
	return result
}

// BuildAvatarStateEndpoints returns the SSE and current state endpoints.
type AvatarEndpoints struct {
	StateSSE     string `json:"stateSSE"`     // SSE endpoint for real-time state
	CurrentState string `json:"currentState"` // GET endpoint for current state
	Health       string `json:"health"`       // Health check endpoint
}

// NewAvatarEndpoints creates endpoint URLs based on base URL.
func NewAvatarEndpoints(baseURL string) *AvatarEndpoints {
	return &AvatarEndpoints{
		StateSSE:     baseURL + "/api/v1/avatar/state",
		CurrentState: baseURL + "/api/v1/avatar/current",
		Health:       baseURL + "/api/v1/avatar/health",
	}
}
