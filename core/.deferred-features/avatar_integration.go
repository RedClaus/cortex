// Package a2a provides avatar integration for A2A responses.
package a2a

import (
	"context"
	"fmt"
	"time"

	"github.com/a2aproject/a2a-go/a2a"
	"github.com/a2aproject/a2a-go/a2asrv"
	"github.com/a2aproject/a2a-go/a2asrv/eventqueue"
	"github.com/normanking/cortex/internal/avatar"
	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AVATAR-ENHANCED EXECUTOR
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarEnhancedExecutor wraps BrainExecutor to add avatar state to responses.
type AvatarEnhancedExecutor struct {
	base         *BrainExecutor
	avatarMgr    *avatar.StateManager
	extractor    avatar.PhonemeExtractor
	log          *logging.Logger
	baseURL      string
	enableAvatar bool
}

// NewAvatarEnhancedExecutor creates an executor with avatar support.
func NewAvatarEnhancedExecutor(
	base *BrainExecutor,
	avatarMgr *avatar.StateManager,
	baseURL string,
) *AvatarEnhancedExecutor {
	return &AvatarEnhancedExecutor{
		base:         base,
		avatarMgr:    avatarMgr,
		extractor:    avatar.NewAdvancedPhonemeExtractor(),
		log:          logging.Global(),
		baseURL:      baseURL,
		enableAvatar: avatarMgr != nil,
	}
}

// Execute processes a request with avatar state integration.
func (e *AvatarEnhancedExecutor) Execute(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	e.log.Info("[A2A+Avatar] Execute: received request taskID=%s", reqCtx.TaskID)

	// Check if client requested avatar state
	wantsAvatar := e.clientWantsAvatar(reqCtx)

	// If avatar is enabled and wanted, set thinking state
	if e.enableAvatar && wantsAvatar {
		e.avatarMgr.SetEmotion("thinking")
	}

	// Send working state
	workingEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateWorking, nil)
	if err := queue.Write(ctx, workingEvent); err != nil {
		return fmt.Errorf("failed to write state working: %w", err)
	}

	// Extract text from message
	input := extractTextFromMessage(reqCtx.Message)
	e.log.Debug("[A2A+Avatar] Execute: processing input length=%d", len(input))

	// Inject memory context if lesson store is available
	// (Uses text injection for Brain processing path)
	input = e.base.injectMemoryContextAsText(ctx, reqCtx, input)

	// Process through Brain Executive
	result, err := e.base.brain.Process(ctx, input)
	if err != nil {
		e.log.Error("[A2A+Avatar] Execute: brain processing failed: %v", err)
		if e.enableAvatar && wantsAvatar {
			e.avatarMgr.SetEmotion("neutral")
		}
		errorMsg := a2a.NewMessage(a2a.MessageRoleAgent, a2a.TextPart{Text: fmt.Sprintf("Error: %v", err)})
		failEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateFailed, errorMsg)
		failEvent.Final = true
		return queue.Write(ctx, failEvent)
	}

	// Write artifacts for lobe outputs
	if err := e.base.writeArtifacts(ctx, reqCtx, queue, result); err != nil {
		e.log.Warn("[A2A+Avatar] Execute: failed to write some artifacts: %v", err)
	}

	// Create response message with text and metadata
	responseText := contentToString(result.FinalContent)
	responseParts := []a2a.Part{a2a.TextPart{Text: responseText}}

	// Add metadata as data part
	metadata := buildMetadata(result)
	if len(metadata) > 0 {
		responseParts = append(responseParts, a2a.DataPart{Data: metadata})
	}

	// Add avatar state if enabled
	if e.enableAvatar && wantsAvatar {
		avatarData := e.buildAvatarResponseData(responseText)
		responseParts = append(responseParts, a2a.DataPart{Data: avatarData})

		// Set emotion based on response content
		e.updateAvatarEmotion(result)
	}

	responseMsg := a2a.NewMessage(a2a.MessageRoleAgent, responseParts...)

	// Complete the task
	completeEvent := a2a.NewStatusUpdateEvent(reqCtx, a2a.TaskStateCompleted, responseMsg)
	completeEvent.Final = true
	if err := queue.Write(ctx, completeEvent); err != nil {
		return fmt.Errorf("failed to write state completed: %w", err)
	}

	e.log.Info("[A2A+Avatar] Execute: completed taskID=%s totalTime=%v avatar=%v",
		reqCtx.TaskID, result.TotalTime, wantsAvatar)
	return nil
}

// Cancel implements a2asrv.AgentExecutor.
func (e *AvatarEnhancedExecutor) Cancel(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	// Reset avatar to neutral if enabled
	if e.enableAvatar {
		e.avatarMgr.SetEmotion("neutral")
	}
	return e.base.Cancel(ctx, reqCtx, queue)
}

// clientWantsAvatar checks if the client requested avatar state.
func (e *AvatarEnhancedExecutor) clientWantsAvatar(reqCtx *a2asrv.RequestContext) bool {
	if reqCtx.Message == nil || reqCtx.Message.Metadata == nil {
		return false
	}

	// Check for explicit avatar request
	if includeAvatar, ok := reqCtx.Message.Metadata["includeAvatarState"].(bool); ok {
		return includeAvatar
	}

	// Check output modes
	if modes, ok := reqCtx.Message.Metadata["outputModes"].([]interface{}); ok {
		for _, mode := range modes {
			if modeStr, ok := mode.(string); ok {
				if modeStr == "avatar" || modeStr == "avatar_state" || modeStr == "application/x-avatar-state" {
					return true
				}
			}
		}
	}

	return false
}

// buildAvatarResponseData creates the avatar state data for the response.
func (e *AvatarEnhancedExecutor) buildAvatarResponseData(responseText string) map[string]any {
	// Get current avatar state
	currentState := e.avatarMgr.GetCurrentState()

	// Estimate speech duration (roughly 125 words per minute)
	wordCount := len(responseText) / 5 // rough estimate
	durationMs := int64((float64(wordCount) / 125.0) * 60000)
	if durationMs < 1000 {
		durationMs = 1000
	}

	// Extract phonemes for lip sync
	phonemes := e.extractor.ExtractPhonemes(responseText, durationMs)
	phonemeData := ConvertPhonemes(phonemes, e.extractor)

	// Build avatar endpoints
	endpoints := NewAvatarEndpoints(e.baseURL)

	return map[string]any{
		"_type": "avatar_state",
		"endpoints": map[string]string{
			"stateSSE":     endpoints.StateSSE,
			"currentState": endpoints.CurrentState,
			"health":       endpoints.Health,
		},
		"currentState": map[string]any{
			"phoneme":    string(currentState.Phoneme),
			"emotion":    currentState.Emotion.Primary,
			"valence":    currentState.Emotion.Valence,
			"arousal":    currentState.Emotion.Arousal,
			"isSpeaking": currentState.IsSpeaking,
			"isThinking": currentState.IsThinking,
			"gaze": map[string]float64{
				"x":         currentState.Gaze.X,
				"y":         currentState.Gaze.Y,
				"blinkRate": currentState.Gaze.BlinkRate,
			},
		},
		"lipSync": map[string]any{
			"text":       responseText,
			"durationMs": durationMs,
			"phonemes":   phonemeData,
		},
	}
}

// updateAvatarEmotion sets avatar emotion based on brain result.
func (e *AvatarEnhancedExecutor) updateAvatarEmotion(result interface{}) {
	// Try to extract emotion from result
	// For now, just set to neutral after processing
	e.avatarMgr.SetEmotion("neutral")
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVATAR SSE STREAMING FOR A2A
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarStateStreamer streams avatar state over SSE to A2A clients.
type AvatarStateStreamer struct {
	avatarMgr *avatar.StateManager
	log       *logging.Logger
}

// NewAvatarStateStreamer creates a new avatar state streamer.
func NewAvatarStateStreamer(avatarMgr *avatar.StateManager) *AvatarStateStreamer {
	return &AvatarStateStreamer{
		avatarMgr: avatarMgr,
		log:       logging.Global(),
	}
}

// StreamToQueue streams avatar state updates to an event queue.
// This allows A2A clients to receive real-time avatar state.
func (s *AvatarStateStreamer) StreamToQueue(ctx context.Context, reqCtx *a2asrv.RequestContext, queue eventqueue.Queue) error {
	if s.avatarMgr == nil {
		return fmt.Errorf("avatar manager not configured")
	}

	// Subscribe to avatar state updates
	stateCh, cleanup := s.avatarMgr.Subscribe()
	defer cleanup()

	s.log.Info("[A2A+Avatar] Starting avatar state stream for task %s", reqCtx.TaskID)

	for {
		select {
		case <-ctx.Done():
			s.log.Debug("[A2A+Avatar] Context cancelled, stopping avatar stream")
			return ctx.Err()

		case state, ok := <-stateCh:
			if !ok {
				s.log.Debug("[A2A+Avatar] Avatar state channel closed")
				return nil
			}

			// Convert to A2A-compatible format
			avatarPart := NewAvatarStatePart(state)

			// Create artifact event for avatar state
			avatarEvent := a2a.NewArtifactEvent(reqCtx, a2a.DataPart{Data: map[string]any{
				"_type":            "avatar_state_update",
				"timestamp":        avatarPart.Timestamp.Format(time.RFC3339Nano),
				"phoneme":          avatarPart.Phoneme,
				"viseme":           avatarPart.Viseme,
				"phonemeIntensity": avatarPart.PhonemeIntensity,
				"isSpeaking":       avatarPart.IsSpeaking,
				"isThinking":       avatarPart.IsThinking,
				"emotion":          avatarPart.Emotion,
				"gaze":             avatarPart.Gaze,
			}})
			avatarEvent.Artifact.Name = "avatar-state"
			avatarEvent.Artifact.Description = "Real-time avatar animation state"

			if err := queue.Write(ctx, avatarEvent); err != nil {
				s.log.Warn("[A2A+Avatar] Failed to write avatar state: %v", err)
				// Continue streaming, don't fail on single write error
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// EXTENDED AGENT CARD WITH AVATAR CAPABILITIES
// ═══════════════════════════════════════════════════════════════════════════════

// AvatarCapabilities describes avatar-specific capabilities.
type AvatarCapabilities struct {
	// LipSync indicates support for phoneme-based lip sync
	LipSync bool `json:"lipSync"`

	// EmotionMapping indicates support for emotional expression
	EmotionMapping bool `json:"emotionMapping"`

	// GazeTracking indicates support for gaze direction
	GazeTracking bool `json:"gazeTracking"`

	// BlendShapes indicates support for 3D blend shape weights
	BlendShapes bool `json:"blendShapes"`

	// RealtimeSSE indicates support for SSE streaming of state
	RealtimeSSE bool `json:"realtimeSSE"`

	// VisemeGroups is the number of viseme groups supported (14)
	VisemeGroups int `json:"visemeGroups"`

	// UpdateRate is the state update rate in Hz
	UpdateRate int `json:"updateRate"`
}

// VisionCapabilities describes vision-specific capabilities.
type VisionCapabilities struct {
	// FrameIngestion indicates support for video frame analysis
	FrameIngestion bool `json:"frameIngestion"`

	// WebSocketStream indicates support for WebSocket video streaming
	WebSocketStream bool `json:"webSocketStream"`

	// ObjectDetection indicates support for object detection
	ObjectDetection bool `json:"objectDetection"`

	// TextRecognition indicates support for OCR
	TextRecognition bool `json:"textRecognition"`

	// MaxFPS is the maximum frames per second for analysis
	MaxFPS float64 `json:"maxFPS"`

	// SupportedFormats lists supported image formats
	SupportedFormats []string `json:"supportedFormats"`
}

// AudioCapabilities describes audio-specific capabilities.
type AudioCapabilities struct {
	// TTS indicates support for text-to-speech
	TTS bool `json:"tts"`

	// STT indicates support for speech-to-text
	STT bool `json:"stt"`

	// PhonemeExtraction indicates support for phoneme timing
	PhonemeExtraction bool `json:"phonemeExtraction"`

	// Voices lists available TTS voices
	Voices []string `json:"voices,omitempty"`

	// SupportedFormats lists supported audio formats
	SupportedFormats []string `json:"supportedFormats"`
}

// ExtendedCapabilities extends standard A2A capabilities with media support.
type ExtendedCapabilities struct {
	Avatar *AvatarCapabilities `json:"avatar,omitempty"`
	Vision *VisionCapabilities `json:"vision,omitempty"`
	Audio  *AudioCapabilities  `json:"audio,omitempty"`
}

// GetDefaultAvatarCapabilities returns the default avatar capabilities.
func GetDefaultAvatarCapabilities() *AvatarCapabilities {
	return &AvatarCapabilities{
		LipSync:        true,
		EmotionMapping: true,
		GazeTracking:   true,
		BlendShapes:    true,
		RealtimeSSE:    true,
		VisemeGroups:   14,
		UpdateRate:     60,
	}
}

// GetDefaultVisionCapabilities returns the default vision capabilities.
func GetDefaultVisionCapabilities() *VisionCapabilities {
	return &VisionCapabilities{
		FrameIngestion:  true,
		WebSocketStream: true,
		ObjectDetection: true,
		TextRecognition: true,
		MaxFPS:          5,
		SupportedFormats: []string{
			"image/jpeg",
			"image/png",
			"image/webp",
		},
	}
}

// GetDefaultAudioCapabilities returns the default audio capabilities.
func GetDefaultAudioCapabilities() *AudioCapabilities {
	return &AudioCapabilities{
		TTS:               true,
		STT:               true,
		PhonemeExtraction: true,
		SupportedFormats: []string{
			"audio/wav",
			"audio/mp3",
			"audio/ogg",
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// AVATAR SKILLS FOR AGENT CARD
// ═══════════════════════════════════════════════════════════════════════════════

// GetAvatarSkill returns the avatar animation skill for the agent card.
func GetAvatarSkill() a2a.AgentSkill {
	return a2a.AgentSkill{
		ID:          "avatar",
		Name:        "Avatar Animation",
		Description: "Real-time avatar animation with lip sync, emotional expressions, and gaze tracking. Supports SSE streaming for 60 FPS state updates.",
		Tags:        []string{"avatar", "animation", "lip-sync", "emotion", "gaze", "3d"},
		Examples: []string{
			"Speak this text with matching lip sync",
			"Show a happy expression",
			"Look at the user while speaking",
		},
		InputModes:  []string{"text", "audio"},
		OutputModes: []string{"application/x-avatar-state", "text/event-stream"},
	}
}

// GetVisionSkill returns the vision processing skill for the agent card.
func GetVisionSkill() a2a.AgentSkill {
	return a2a.AgentSkill{
		ID:          "vision",
		Name:        "Visual Understanding",
		Description: "Analyze images and video streams for object detection, scene understanding, and text recognition. Supports real-time video streaming via WebSocket.",
		Tags:        []string{"vision", "image", "video", "ocr", "detection", "analysis"},
		Examples: []string{
			"What do you see in this image?",
			"Describe the video stream",
			"Read the text in this screenshot",
		},
		InputModes:  []string{"image/*", "video/*", "application/x-video-stream"},
		OutputModes: []string{"text", "application/json"},
	}
}

// GetVoiceSkill returns the voice interaction skill for the agent card.
func GetVoiceSkill() a2a.AgentSkill {
	return a2a.AgentSkill{
		ID:          "voice",
		Name:        "Voice Interaction",
		Description: "Text-to-speech synthesis with phoneme timing for lip sync, and speech-to-text recognition for voice input.",
		Tags:        []string{"voice", "tts", "stt", "speech", "audio"},
		Examples: []string{
			"Speak this response aloud",
			"Listen to voice input",
			"Convert audio to text",
		},
		InputModes:  []string{"text", "audio/*"},
		OutputModes: []string{"audio/*", "text", "application/x-phoneme-timeline"},
	}
}
