package bridge

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/normanking/cortexavatar/internal/a2a"
	"github.com/normanking/cortexavatar/internal/audio"
	"github.com/normanking/cortexavatar/internal/avatar"
	"github.com/normanking/cortexavatar/internal/bus"
	"github.com/normanking/cortexavatar/internal/config"
	"github.com/normanking/cortexavatar/internal/stt"
	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/normanking/cortexavatar/internal/vision"
	"github.com/rs/zerolog"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AudioBridge exposes audio methods to the frontend
type AudioBridge struct {
	ctx        context.Context
	a2aClient  *a2a.Client
	controller *avatar.Controller
	eventBus   *bus.EventBus
	logger     zerolog.Logger
	cfg        *config.Config

	// Managers
	audioManager  *audio.Manager
	visionManager *vision.Manager
	sttProvider   stt.Provider
	ttsProvider   tts.Provider
	piperTTS      *tts.PiperProvider      // High-quality local fallback
	macOSTTS      *tts.MacOSTTSProvider   // Fallback for quota errors
	cartesiaTTS   *tts.CartesiaProvider   // Ultra-low-latency with lip-sync
	elevenLabsTTS *tts.ElevenLabsProvider // High-quality cloud TTS (free tier)

	// CR-001: STT filtering and fragment accumulation
	sttFilter      *stt.STTFilter      // Removes filler words from transcripts
	fragmentBuffer *stt.FragmentBuffer // Accumulates short utterances until pause

	mu             sync.RWMutex
	micEnabled     bool
	speakerEnabled bool
	cameraEnabled  bool
	screenEnabled  bool
	isListening    bool

	// Speech buffer for STT
	speechBuffer []byte
	speechStart  time.Time

	// TTS queue management - prevent overlapping speech
	ttsMu       sync.Mutex
	ttsActive   bool
	ttsCancelCh chan struct{}
	ttsEndTime  time.Time // When TTS ended, for echo cooldown
}

// NewAudioBridge creates the audio bridge
func NewAudioBridge(
	a2aClient *a2a.Client,
	controller *avatar.Controller,
	eventBus *bus.EventBus,
	cfg *config.Config,
	logger zerolog.Logger,
) *AudioBridge {
	bridge := &AudioBridge{
		a2aClient:      a2aClient,
		controller:     controller,
		eventBus:       eventBus,
		cfg:            cfg,
		logger:         logger.With().Str("component", "audio-bridge").Logger(),
		micEnabled:     true,
		speakerEnabled: true,
		speechBuffer:   make([]byte, 0, 16000*2*30), // 30 seconds at 16kHz mono
	}

	// Initialize audio manager
	audioConfig := audio.DefaultAudioConfig()
	audioConfig.SampleRate = cfg.Audio.SampleRate
	audioConfig.VADThreshold = cfg.Audio.VADThreshold
	bridge.audioManager = audio.NewManager(audioConfig, eventBus, logger)

	// Initialize vision manager
	visionConfig := vision.DefaultConfig()
	bridge.visionManager = vision.NewManager(visionConfig, eventBus, logger)

	// Initialize STT provider - prefer Groq Whisper (free tier), fallback to A2A
	groqProvider := stt.NewGroqWhisperProvider(logger, nil)
	if groqProvider.IsAvailable() {
		bridge.sttProvider = groqProvider
		logger.Info().Msg("Using Groq Whisper for STT (free tier)")
	} else {
		bridge.sttProvider = stt.NewA2AProvider(a2aClient, logger, nil)
		logger.Warn().Msg("Groq API key not set - STT may not work. Get a free key at https://console.groq.com")
	}

	// Initialize TTS providers
	cartesiaTTS := tts.NewCartesiaProvider(logger, &tts.CartesiaConfig{
		DefaultVoice: "a0e99841-438c-4a64-b679-ae501e7d6091",
		Model:        "sonic-3",
		Language:     "en",
		SampleRate:   22050,
	})
	openaiTTS := tts.NewOpenAIProvider(logger, &tts.OpenAIConfig{
		DefaultVoice: cfg.TTS.VoiceID,
		Model:        "tts-1",
		Speed:        cfg.TTS.Speed,
	})
	elevenLabsTTS := tts.NewElevenLabsProvider(logger, nil)
	piperTTS := tts.NewPiperProvider(logger, nil)
	macosTTS := tts.NewMacOSTTSProvider(logger, &tts.MacOSConfig{
		DefaultVoice: "Samantha",
		Rate:         175,
	})

	// Select TTS provider by priority: Cartesia > ElevenLabs > OpenAI > Piper > macOS
	if cartesiaTTS.IsAvailable() {
		bridge.ttsProvider = cartesiaTTS
		bridge.cartesiaTTS = cartesiaTTS
		logger.Info().Msg("Using Cartesia TTS for ultra-low-latency streaming with lip-sync")
	} else if elevenLabsTTS.IsAvailable() {
		bridge.ttsProvider = elevenLabsTTS
		logger.Info().Msg("Using ElevenLabs TTS for high-quality cloud speech")
	} else if openaiTTS.IsAvailable() {
		bridge.ttsProvider = openaiTTS
		logger.Info().Str("voice", cfg.TTS.VoiceID).Msg("Using OpenAI TTS for natural speech")
	} else if piperTTS.IsAvailable() {
		bridge.ttsProvider = piperTTS
		logger.Info().Msg("Using Piper TTS for high-quality local speech")
	} else if macosTTS.IsAvailable() {
		bridge.ttsProvider = macosTTS
		logger.Info().Msg("Using macOS native TTS")
	} else {
		ttsConfig := tts.DefaultA2AConfig()
		ttsConfig.DefaultVoice = cfg.TTS.VoiceID
		bridge.ttsProvider = tts.NewA2AProvider(a2aClient, logger, ttsConfig)
		logger.Warn().Msg("Using A2A TTS as last resort")
	}

	// Store fallback providers
	bridge.piperTTS = piperTTS
	bridge.macOSTTS = macosTTS
	bridge.elevenLabsTTS = elevenLabsTTS
	if cartesiaTTS.IsAvailable() {
		bridge.cartesiaTTS = cartesiaTTS
	}

	// CR-001: Initialize STT filter and fragment buffer for voice UX
	bridge.sttFilter = stt.NewSTTFilter(nil) // Uses DefaultFillerWords
	bridge.fragmentBuffer = stt.NewFragmentBuffer(&stt.FragmentBufferConfig{
		TimeoutMs:    500, // 500ms pause detection
		MinWordCount: 2,   // At least 2 words before sending
	})
	logger.Info().Msg("CR-001: STT filter and fragment buffer initialized")

	return bridge
}

// Bind sets the Wails runtime context
func (b *AudioBridge) Bind(ctx context.Context) {
	b.ctx = ctx

	// Subscribe to audio events
	b.eventBus.Subscribe(bus.EventTypeListeningStarted, func(e bus.Event) {
		runtime.EventsEmit(b.ctx, "audio:listening", true)
	})

	b.eventBus.Subscribe(bus.EventTypeListeningStopped, func(e bus.Event) {
		runtime.EventsEmit(b.ctx, "audio:listening", false)
	})

	b.eventBus.Subscribe(bus.EventTypeSpeakingStarted, func(e bus.Event) {
		runtime.EventsEmit(b.ctx, "audio:speaking", true)
	})

	b.eventBus.Subscribe(bus.EventTypeSpeakingStopped, func(e bus.Event) {
		runtime.EventsEmit(b.ctx, "audio:speaking", false)
	})

	b.eventBus.Subscribe(bus.EventTypeTranscript, func(e bus.Event) {
		if text, ok := e.Data["text"].(string); ok {
			runtime.EventsEmit(b.ctx, "audio:transcript", text)
		}
	})
}

// StartListening begins audio capture and STT
func (b *AudioBridge) StartListening() error {
	b.mu.Lock()
	if b.isListening {
		b.mu.Unlock()
		return nil
	}
	b.isListening = true
	b.mu.Unlock()

	b.controller.StartListening()
	b.eventBus.Publish(bus.Event{
		Type: bus.EventTypeListeningStarted,
		Data: map[string]any{},
	})

	b.logger.Info().Msg("Started listening")
	return nil
}

// StopListening stops audio capture
func (b *AudioBridge) StopListening() error {
	b.mu.Lock()
	if !b.isListening {
		b.mu.Unlock()
		return nil
	}
	b.isListening = false
	b.mu.Unlock()

	b.controller.StopListening()
	b.eventBus.Publish(bus.Event{
		Type: bus.EventTypeListeningStopped,
		Data: map[string]any{},
	})

	b.logger.Info().Msg("Stopped listening")
	return nil
}

// ToggleMic toggles microphone on/off
func (b *AudioBridge) ToggleMic() bool {
	b.mu.Lock()
	b.micEnabled = !b.micEnabled
	enabled := b.micEnabled
	b.mu.Unlock()

	if enabled {
		b.StartListening()
	} else {
		b.StopListening()
	}

	return enabled
}

// ToggleSpeaker toggles speaker on/off
func (b *AudioBridge) ToggleSpeaker() bool {
	b.mu.Lock()
	b.speakerEnabled = !b.speakerEnabled
	enabled := b.speakerEnabled
	b.mu.Unlock()

	return enabled
}

// ToggleCamera toggles camera on/off
func (b *AudioBridge) ToggleCamera() bool {
	b.mu.Lock()
	b.cameraEnabled = !b.cameraEnabled
	enabled := b.cameraEnabled
	b.mu.Unlock()

	if enabled {
		b.eventBus.Publish(bus.Event{Type: bus.EventTypeCameraEnabled})
	} else {
		b.eventBus.Publish(bus.Event{Type: bus.EventTypeCameraDisabled})
	}

	return enabled
}

// ToggleScreenShare toggles screen sharing on/off
func (b *AudioBridge) ToggleScreenShare() bool {
	b.mu.Lock()
	b.screenEnabled = !b.screenEnabled
	enabled := b.screenEnabled
	b.mu.Unlock()

	if enabled {
		b.eventBus.Publish(bus.Event{Type: bus.EventTypeScreenShareEnabled})
	} else {
		b.eventBus.Publish(bus.Event{Type: bus.EventTypeScreenShareDisabled})
	}

	return enabled
}

// SendMessage sends a text message to CortexBrain
func (b *AudioBridge) SendMessage(text string) (string, error) {
	b.logger.Info().Str("text", text).Msg(">>> SendMessage called from frontend")
	b.controller.StartThinking()

	// Emit thinking audio cue to bridge the cognitive processing silence
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:thinking_start", nil)
	}

	response, err := b.a2aClient.SendMessage(context.Background(), text)

	// Stop thinking audio cue
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:thinking_stop", nil)
	}

	if err != nil {
		b.logger.Error().Err(err).Str("text", text).Msg("<<< SendMessage FAILED")
		b.controller.StopThinking()
		b.controller.SetEmotion(avatar.EmotionConfused)
		return "", err
	}

	b.controller.StopThinking()
	b.controller.SetEmotion(avatar.EmotionHappy)

	responseText := response.ExtractText()
	b.logger.Info().
		Int("responseLen", len(responseText)).
		Str("preview", truncateString(responseText, 100)).
		Msg("<<< SendMessage SUCCESS")

	// Trigger TTS for the response (speakText handles filtering)
	if b.speakerEnabled && responseText != "" {
		spokenText := b.extractSpokenPortion(responseText)
		go b.speakText(spokenText)
	}

	return responseText, nil
}

// SendMessageWithVision sends a message with image to CortexBrain
func (b *AudioBridge) SendMessageWithVision(text, imageBase64, mimeType string) (string, error) {
	b.controller.StartThinking()

	response, err := b.a2aClient.SendMessageWithVision(context.Background(), text, imageBase64, mimeType)
	if err != nil {
		b.controller.StopThinking()
		b.controller.SetEmotion(avatar.EmotionConfused)
		return "", err
	}

	b.controller.StopThinking()
	b.controller.SetEmotion(avatar.EmotionHappy)

	return response.ExtractText(), nil
}

// IsMicEnabled returns microphone status
func (b *AudioBridge) IsMicEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.micEnabled
}

// IsSpeakerEnabled returns speaker status
func (b *AudioBridge) IsSpeakerEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.speakerEnabled
}

// IsCameraEnabled returns camera status
func (b *AudioBridge) IsCameraEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.cameraEnabled
}

// IsScreenShareEnabled returns screen share status
func (b *AudioBridge) IsScreenShareEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.screenEnabled
}

// chunkCount tracks received audio chunks for periodic logging
var chunkCount int64

// ProcessAudioChunk handles audio data from the frontend
// audioBase64 is base64-encoded PCM audio (16kHz, 16-bit, mono)
// isSpeech indicates VAD detection from frontend
func (b *AudioBridge) ProcessAudioChunk(audioBase64 string, isSpeech bool, rms float64) {
	chunkCount++
	// Log every 50th chunk to show audio is flowing
	if chunkCount%50 == 0 {
		b.logger.Debug().Int64("chunks", chunkCount).Float64("rms", rms).Bool("speech", isSpeech).Msg("Audio chunks received")
	}

	// ECHO COOLDOWN: Skip speech detection for 500ms after TTS ends to avoid self-triggering
	// This prevents the tail of TTS audio from being captured as user input
	const echoCooldown = 500 * time.Millisecond
	if !b.ttsEndTime.IsZero() && time.Since(b.ttsEndTime) < echoCooldown {
		// Still in cooldown period after TTS ended
		return
	}

	// BARGE-IN: If speech detected while TTS is playing, cancel TTS immediately
	// This enables natural conversation flow where user can interrupt the assistant
	// We use a higher RMS threshold during TTS to avoid self-triggering from speaker echo
	bargeInThreshold := 0.02 // Higher threshold during TTS to reduce echo triggers
	if isSpeech && b.ttsActive && rms > bargeInThreshold {
		b.logger.Info().Float64("rms", rms).Float64("threshold", bargeInThreshold).Msg("Barge-in: User speech detected during TTS playback")
		b.cancelOngoingSpeech()
	}

	// Debug: log when speech detected
	if isSpeech {
		b.logger.Debug().Float64("rms", rms).Bool("speech", isSpeech).Int("dataLen", len(audioBase64)).Msg("Speech detected in audio")
	}
	b.audioManager.ProcessAudioChunk(audioBase64, isSpeech, rms)

	// Accumulate audio for STT during speech
	if isSpeech {
		audioData, err := base64.StdEncoding.DecodeString(audioBase64)
		if err == nil {
			b.mu.Lock()
			if len(b.speechBuffer) == 0 {
				b.speechStart = time.Now()
			}
			b.speechBuffer = append(b.speechBuffer, audioData...)
			b.mu.Unlock()
		}
	} else {
		// Speech ended, process STT if we have audio
		b.mu.Lock()
		audioLen := len(b.speechBuffer)
		if audioLen > 16000 { // At least 0.5 second of audio (16000 samples = 0.5s at 16kHz mono)
			audio := make([]byte, audioLen)
			copy(audio, b.speechBuffer)
			b.speechBuffer = b.speechBuffer[:0]
			b.mu.Unlock()

			// Process STT asynchronously
			go b.processSpeechToText(audio)
		} else {
			b.speechBuffer = b.speechBuffer[:0]
			b.mu.Unlock()
		}
	}
}

// processSpeechToText transcribes audio and sends to CortexBrain
func (b *AudioBridge) processSpeechToText(audioData []byte) {
	b.logger.Info().Int("bytes", len(audioData)).Msg("Processing speech to text")

	// Transcribe
	resp, err := b.sttProvider.Transcribe(context.Background(), &stt.TranscribeRequest{
		Audio:      audioData,
		Format:     "pcm",
		SampleRate: 16000,
		Channels:   1,
	})
	if err != nil {
		b.logger.Error().Err(err).Msg("STT failed")
		return
	}

	if resp.Text == "" {
		b.logger.Debug().Msg("Empty transcription")
		return
	}

	// CR-001: Apply STT filter to remove filler words
	rawText := resp.Text
	cleanedText, hasMeaningful := b.sttFilter.Clean(rawText)
	if !hasMeaningful {
		b.logger.Debug().Str("raw", rawText).Msg("Transcript contained only filler words, skipping")
		return
	}

	b.logger.Info().
		Str("raw", rawText).
		Str("cleaned", cleanedText).
		Msg("Transcribed speech (CR-001 filtered)")

	// Emit cleaned transcript event to frontend
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:transcript", cleanedText)
	}

	// CR-001: Add to fragment buffer for accumulation
	b.fragmentBuffer.Add(cleanedText)

	// Check if we should send to brain (enough words or pause detected)
	if !b.fragmentBuffer.ShouldSend() {
		b.logger.Debug().
			Int("words", b.fragmentBuffer.WordCount()).
			Str("buffered", b.fragmentBuffer.Peek()).
			Msg("Buffering fragment, waiting for more input or pause")
		return
	}

	// Flush buffer and send accumulated text
	textToSend := b.fragmentBuffer.Flush()
	if textToSend == "" {
		return
	}

	// Send to CortexBrain with voice mode (CR-093)
	b.logger.Info().Str("userInput", textToSend).Msg(">>> Sending to CortexBrain (voice mode)")
	b.controller.StartThinking()

	// Emit thinking audio cue to bridge the cognitive processing silence
	// This provides natural feedback that the system is working
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:thinking_start", nil)
	}

	// CR-093: Use SendMessageWithOptions with voice mode to trigger Voice Executive
	response, err := b.a2aClient.SendMessageWithOptions(context.Background(), textToSend, a2a.SendMessageOptions{
		Mode: "voice", // Triggers Voice Executive for <2s latency responses
	})

	// Stop thinking audio cue
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:thinking_stop", nil)
	}

	if err != nil {
		b.logger.Error().Err(err).Str("input", textToSend).Msg("<<< CortexBrain FAILED")
		b.controller.StopThinking()
		return
	}

	b.controller.StopThinking()
	responseText := response.ExtractText()

	// Log the raw response for debugging
	b.logger.Info().
		Int("responseLen", len(responseText)).
		Str("responsePreview", truncateString(responseText, 200)).
		Msg("<<< CortexBrain response received")

	// Check for missing model errors and notify user
	b.checkForModelErrors(responseText)

	// Emit response to frontend
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "cortex:response", responseText)
	}

	// Check for empty response
	if responseText == "" {
		b.logger.Warn().Str("input", textToSend).Msg("<<< CortexBrain returned EMPTY response!")
		return
	}

	// Synthesize speech for response (skip system/background messages)
	if b.speakerEnabled {
		// Extract spoken portion - for long responses, we only speak the first part
		spokenText := b.extractSpokenPortion(responseText)
		if b.shouldSkipTTS(spokenText) {
			b.logger.Debug().
				Int("textLen", len(spokenText)).
				Str("preview", truncateString(spokenText, 50)).
				Msg("Skipping TTS for system/background message")
		} else {
			go b.speakText(spokenText)
		}
	}
}

// speakText synthesizes and plays text with cancellation support for barge-in
func (b *AudioBridge) speakText(text string) {
	// Check if we should skip this TTS
	if b.shouldSkipTTS(text) {
		b.logger.Debug().
			Int("textLen", len(text)).
			Str("preview", truncateString(text, 80)).
			Msg("BLOCKED TTS - system/internal message detected")
		return
	}

	// Cancel any ongoing speech and acquire TTS lock
	b.cancelOngoingSpeech()
	b.ttsMu.Lock()
	defer b.ttsMu.Unlock()

	// Mark TTS as active and create cancel channel
	b.ttsActive = true
	b.ttsCancelCh = make(chan struct{})
	defer func() {
		b.ttsActive = false
		b.ttsEndTime = time.Now() // Mark TTS end time for echo cooldown
		// Don't close here - cancelOngoingSpeech will close it if needed
		select {
		case <-b.ttsCancelCh:
			// Already closed by cancel
		default:
			close(b.ttsCancelCh)
		}
	}()

	// Create cancelable context that listens to ttsCancelCh for barge-in
	ttsCtx, ttsCancel := context.WithCancel(context.Background())
	defer ttsCancel()

	// Monitor ttsCancelCh in background and cancel context when signaled
	go func() {
		select {
		case <-b.ttsCancelCh:
			b.logger.Debug().Msg("Barge-in detected - canceling TTS synthesis")
			ttsCancel()
		case <-ttsCtx.Done():
			// Context already canceled or completed
		}
	}()

	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:stop_playback", nil)
		runtime.EventsEmit(b.ctx, "audio:speaking", true)
	}

	b.controller.StartSpeaking(nil)

	// Track if we successfully sent audio to frontend for playback
	// If so, frontend manages isSpeaking state - we don't send audio:speaking=false
	audioSentToFrontend := false
	defer func() {
		b.controller.StopSpeaking()
		// Only emit audio:speaking=false if we didn't send audio to frontend
		// Frontend's playAudio() handles isSpeaking state based on actual playback
		if b.ctx != nil && !audioSentToFrontend {
			runtime.EventsEmit(b.ctx, "audio:speaking", false)
		}
	}()

	// Use configured voice from settings
	voiceID := b.cfg.TTS.VoiceID
	if voiceID == "" {
		voiceID = "nova" // Default to OpenAI Nova
	}

	b.logger.Info().
		Str("voice", voiceID).
		Str("provider", b.ttsProvider.Name()).
		Int("textLen", len(text)).
		Msg("Starting TTS synthesis")

	// Request phonemes for lip-sync when using Cartesia
	withPhonemes := b.cartesiaTTS != nil && b.ttsProvider.Name() == "cartesia"

	// Use cancelable context for TTS - enables fast barge-in
	resp, err := b.ttsProvider.Synthesize(ttsCtx, &tts.SynthesizeRequest{
		Text:         text,
		VoiceID:      voiceID,
		WithPhonemes: withPhonemes,
	})

	// Check if canceled (barge-in)
	if ttsCtx.Err() == context.Canceled {
		b.logger.Info().Msg("TTS canceled due to barge-in")
		return
	}

	// WKWebView doesn't support Web Speech API properly - use native macOS TTS
	if err != nil || resp == nil || len(resp.Audio) == 0 {
		if b.macOSTTS != nil && b.macOSTTS.IsAvailable() {
			b.logger.Info().Str("voice", voiceID).Msg("Using native macOS TTS with direct audio output")
			if b.ctx != nil {
				b.logger.Debug().Msg("Emitting audio:speaking=true to frontend")
				runtime.EventsEmit(b.ctx, "audio:speaking", true)
			} else {
				b.logger.Warn().Msg("Cannot emit audio:speaking - context is nil")
			}
			speakErr := b.macOSTTS.SpeakDirect(ttsCtx, text, voiceID)
			if b.ctx != nil {
				b.logger.Debug().Msg("Emitting audio:speaking=false to frontend")
				runtime.EventsEmit(b.ctx, "audio:speaking", false)
			}
			if speakErr != nil {
				b.logger.Error().Err(speakErr).Msg("macOS direct TTS failed")
			}
			audioSentToFrontend = true
		}
		return
	}

	audioBase64 := base64.StdEncoding.EncodeToString(resp.Audio)
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:playback", map[string]any{
			"audio":  audioBase64,
			"format": resp.Format,
		})
		audioSentToFrontend = true

		// Emit viseme timeline for lip-sync if available
		if len(resp.Phonemes) > 0 {
			visemeTimeline := tts.GenerateVisemeTimeline(resp.Phonemes)
			runtime.EventsEmit(b.ctx, "viseme:timeline", visemeTimeline.ConvertToFrontendFormat())
			b.logger.Debug().Int("visemes", len(visemeTimeline.Events)).Msg("Emitted viseme timeline for lip-sync")
		} else if withPhonemes {
			// Fallback: generate approximate visemes from text
			estimatedDuration := time.Duration(len(text)) * 60 * time.Millisecond
			visemeTimeline := tts.GenerateVisemeTimelineFromText(text, estimatedDuration)
			runtime.EventsEmit(b.ctx, "viseme:timeline", visemeTimeline.ConvertToFrontendFormat())
			b.logger.Debug().Int("visemes", len(visemeTimeline.Events)).Msg("Emitted text-based viseme timeline")
		}
	}

	b.logger.Info().Int("bytes", len(resp.Audio)).Msg("TTS audio sent to frontend")
}

// ProcessCameraFrame handles camera frame from frontend
func (b *AudioBridge) ProcessCameraFrame(imageBase64 string, width, height int) {
	b.visionManager.ProcessCameraFrame(imageBase64, width, height)
}

// ProcessScreenFrame handles screen capture from frontend
func (b *AudioBridge) ProcessScreenFrame(imageBase64 string, width, height int) {
	b.visionManager.ProcessScreenFrame(imageBase64, width, height)
}

// GetPersonas returns available personas
func (b *AudioBridge) GetPersonas() []config.Persona {
	return config.AvailablePersonas()
}

// GetCurrentPersona returns the current persona
func (b *AudioBridge) GetCurrentPersona() *config.Persona {
	return config.GetPersona(b.cfg.Avatar.Persona)
}

// SetPersona changes the current persona
func (b *AudioBridge) SetPersona(personaID string) error {
	persona := config.GetPersona(personaID)
	if persona == nil {
		return nil
	}

	b.cfg.Avatar.Persona = personaID
	b.cfg.User.PersonaID = personaID // Update for A2A message metadata
	b.cfg.TTS.VoiceID = persona.VoiceID

	// Update A2A client's persona ID for future messages
	b.a2aClient.SetPersonaID(personaID)

	// Update TTS provider voice
	if provider, ok := b.ttsProvider.(*tts.A2AProvider); ok {
		_ = provider // Update default voice
	}

	// Notify frontend
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "persona:changed", persona)
	}

	b.logger.Info().Str("persona", personaID).Msg("Persona changed")
	return nil
}

// SpeakText synthesizes and plays the given text
func (b *AudioBridge) SpeakText(text string) {
	go b.speakText(text)
}

// StopSpeaking interrupts current speech
func (b *AudioBridge) StopSpeaking() {
	b.cancelOngoingSpeech()
	b.controller.StopSpeaking()
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:stop_playback", nil)
	}
}

// cancelOngoingSpeech cancels any ongoing TTS synthesis and tells frontend to stop playback
// This enables fast barge-in by signaling the TTS context to cancel
func (b *AudioBridge) cancelOngoingSpeech() {
	// Signal TTS goroutine to cancel synthesis (for barge-in)
	if b.ttsActive && b.ttsCancelCh != nil {
		select {
		case <-b.ttsCancelCh:
			// Already closed
		default:
			close(b.ttsCancelCh)
			b.logger.Debug().Msg("Signaled TTS cancellation for barge-in")
		}
	}

	// Tell frontend to stop playback immediately
	if b.ctx != nil {
		runtime.EventsEmit(b.ctx, "audio:stop_playback", nil)
	}
	b.controller.StopSpeaking()
	b.logger.Debug().Msg("Cancelled ongoing speech")
}

// checkForModelErrors detects missing Ollama model errors and notifies the user
func (b *AudioBridge) checkForModelErrors(text string) {
	// Look for patterns like: model 'xyz' not found
	if strings.Contains(text, "model") && strings.Contains(text, "not found") {
		// Extract model name using regex
		re := regexp.MustCompile(`model '([^']+)' not found`)
		matches := re.FindStringSubmatch(text)
		if len(matches) > 1 {
			modelName := matches[1]
			b.logger.Warn().Str("model", modelName).Msg("CortexBrain reports missing Ollama model")

			// Emit event to frontend
			if b.ctx != nil {
				runtime.EventsEmit(b.ctx, "cortex:model_missing", map[string]any{
					"model": modelName,
				})
			}
		}
	}
}

// shouldSkipTTS checks if the text should NOT be spoken
// This filters out error messages, system messages, and background task output
// The filtering level is controlled by cfg.TTS.ReasoningFilter (0-100)
func (b *AudioBridge) shouldSkipTTS(text string) bool {
	// Normalize for checking
	lower := strings.ToLower(text)
	filterLevel := b.cfg.TTS.ReasoningFilter // 0=hear all, 100=max filter

	// LEVEL 0 (Always skip) - System/technical content that should never be spoken
	// Code blocks - ALWAYS skip
	if strings.Contains(text, "```") {
		b.logger.Debug().Msg("TTS SKIP: contains code blocks")
		return true
	}

	// JSON-like content
	if strings.Contains(text, "\":") || strings.Contains(text, "\": ") {
		b.logger.Debug().Msg("TTS SKIP: JSON-like content")
		return true
	}

	// Very long responses (likely technical/code)
	if len(text) > 600 {
		b.logger.Debug().Int("len", len(text)).Msg("TTS SKIP: text too long (>600 chars)")
		return true
	}

	// Tool execution markers
	if strings.Contains(lower, "tool_use") ||
		strings.Contains(lower, "tool_result") ||
		strings.Contains(lower, "<tool>") ||
		strings.Contains(lower, "</tool>") {
		b.logger.Debug().Msg("TTS SKIP: tool markers")
		return true
	}

	// LEVEL 20+ - Technical brackets, status messages
	if filterLevel >= 20 {
		bracketCount := strings.Count(text, "(") + strings.Count(text, "{") + strings.Count(text, "[")
		if bracketCount >= 3 {
			b.logger.Debug().Int("brackets", bracketCount).Msg("TTS SKIP: too many brackets")
			return true
		}

		// Status prefixes
		if strings.HasPrefix(text, "[") && strings.Contains(text, "]") {
			b.logger.Debug().Msg("TTS SKIP: status prefix [...]")
			return true
		}
		if strings.HasPrefix(text, "Status:") ||
			strings.HasPrefix(text, "Progress:") ||
			strings.HasPrefix(text, "Debug:") ||
			strings.HasPrefix(text, "Error:") {
			b.logger.Debug().Msg("TTS SKIP: status prefix")
			return true
		}

		// File paths and URLs
		if strings.HasPrefix(text, "/") || strings.Contains(text, ":/") {
			b.logger.Debug().Msg("TTS SKIP: file path")
			return true
		}
		if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") {
			b.logger.Debug().Msg("TTS SKIP: URL")
			return true
		}
	}

	// LEVEL 40+ - Internal processing messages
	if filterLevel >= 40 {
		internalPhrases := []string{
			"running task",
			"background task",
			"executing command",
			"executing tool",
			"tool execution",
			"processing request",
			"querying database",
			"searching files",
			"reading file",
			"writing file",
			"api call",
			"api request",
		}
		for _, phrase := range internalPhrases {
			if strings.Contains(lower, phrase) {
				b.logger.Debug().Str("phrase", phrase).Int("level", filterLevel).Msg("TTS SKIP: internal phrase")
				return true
			}
		}
	}

	// LEVEL 60+ - System/internal markers and confused responses
	if filterLevel >= 60 {
		systemPhrases := []string{
			"<thinking>",
			"</thinking>",
			"<reflection>",
			"</reflection>",
			"<inner>",
			"</inner>",
			"[thinking]",
			"[reflection]",
			"[internal]",
			"i notice that your input",
			"appears to be incomplete",
			"you've provided a structure",
			"your input appears",
			"memory context",
			"previous learning history",
			"cognitive lobe",
			"lobe activating",
			"your message",
			"your input",
			"you seem to",
			"i received",
			"i got your",
			"you sent",
			"you typed",
		}
		for _, phrase := range systemPhrases {
			if strings.Contains(lower, phrase) {
				b.logger.Debug().Str("phrase", phrase).Int("level", filterLevel).Msg("TTS SKIP: system phrase")
				return true
			}
		}
	}

	// LEVEL 80+ - Reasoning/analysis patterns (Hannah's thoughtful style)
	if filterLevel >= 80 {
		reasoningPhrases := []string{
			"reasoning process",
			"logical analysis",
			"input assessment",
			"context evaluation",
			"let me think",
			"i'm thinking",
			"let me consider",
			"i'm processing",
			"analyzing your",
			"analyzing the input",
			"analyzing the context",
			"i can deduce",
			"understanding your",
			"i'm reflecting",
			"let me reflect",
			"i'm considering",
			"working through",
			"let me work through",
			"evaluating",
			"assessing",
			"the input states",
			"the input is",
			"given the context",
		}
		for _, phrase := range reasoningPhrases {
			if strings.Contains(lower, phrase) {
				b.logger.Debug().Str("phrase", phrase).Int("level", filterLevel).Msg("TTS SKIP: reasoning phrase")
				return true
			}
		}
	}

	// LEVEL 100 - Maximum filter: also skip gentle preambles
	if filterLevel >= 100 {
		preamblePhrases := []string{
			"i want to make sure",
			"before we dive in",
			"taking a moment",
			"give me a moment",
		}
		for _, phrase := range preamblePhrases {
			if strings.Contains(lower, phrase) {
				b.logger.Debug().Str("phrase", phrase).Int("level", filterLevel).Msg("TTS SKIP: preamble phrase")
				return true
			}
		}
	}

	// Log that we're allowing TTS
	b.logger.Debug().Int("len", len(text)).Int("filterLevel", filterLevel).Str("preview", truncateString(text, 50)).Msg("TTS ALLOWED")
	return false
}

// isErrorMessage is deprecated, use shouldSkipTTS instead
func (b *AudioBridge) isErrorMessage(text string) bool {
	return b.shouldSkipTTS(text)
}

// truncateString truncates a string to maxLen chars with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// extractSpokenPortion extracts the conversational portion of a response for TTS
// It removes markdown, code, thinking/reflection sections, and extracts just the key spoken content
func (b *AudioBridge) extractSpokenPortion(text string) string {
	// First, strip any explicit thinking/reflection blocks
	text = b.stripThinkingBlocks(text)

	// Already short enough - return as is (after cleaning)
	if len(text) <= 300 {
		return b.cleanForTTS(text)
	}

	// Split into paragraphs
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) == 0 {
		return b.cleanForTTS(text)
	}

	// Build spoken content from conversational paragraphs only
	var spoken strings.Builder
	for _, para := range paragraphs {
		// Skip empty paragraphs
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Skip code blocks
		if strings.HasPrefix(para, "```") || strings.HasPrefix(para, "    ") {
			continue
		}

		// Skip ALL markdown headers (they're structural, not conversational)
		if strings.HasPrefix(para, "#") {
			continue
		}

		// Skip ALL bullet point lists (they're not natural speech)
		if strings.HasPrefix(para, "-") || strings.HasPrefix(para, "•") {
			continue
		}

		// Skip if starts with asterisk AND has newlines (likely a multi-line list)
		// But allow single-line asterisk text (like *emphasis* or *action*)
		if strings.HasPrefix(para, "*") && strings.Contains(para, "\n") {
			continue
		}

		// Skip multi-line list items
		if strings.Count(para, "\n") > 2 {
			continue
		}

		// Skip meta/thinking paragraphs (Hannah's inner voice)
		lowerPara := strings.ToLower(para)
		if b.isThinkingParagraph(lowerPara) {
			continue
		}

		// Clean the paragraph for TTS
		cleaned := b.cleanForTTS(para)
		if cleaned == "" {
			continue
		}

		// Add to spoken content
		if spoken.Len() > 0 {
			spoken.WriteString(" ")
		}
		spoken.WriteString(cleaned)

		// Stop after first substantial conversational paragraph
		// For voice, one paragraph is usually enough
		if spoken.Len() >= 150 {
			break
		}
	}

	result := spoken.String()
	if result == "" {
		// Fallback: just clean the first 300 chars
		return b.cleanForTTS(truncateString(text, 300))
	}

	// Truncate if still too long
	if len(result) > 400 {
		// Find a natural break point
		breakPoints := []string{". ", "! ", "? ", ", "}
		for _, bp := range breakPoints {
			if idx := strings.LastIndex(result[:400], bp); idx > 150 {
				result = result[:idx+1]
				break
			}
		}
		if len(result) > 400 {
			result = result[:397] + "..."
		}
	}

	b.logger.Debug().
		Int("originalLen", len(text)).
		Int("spokenLen", len(result)).
		Str("preview", truncateString(result, 100)).
		Msg("Extracted spoken portion")

	return result
}

// stripThinkingBlocks removes explicit thinking/reflection blocks from text
func (b *AudioBridge) stripThinkingBlocks(text string) string {
	// Remove <thinking>...</thinking> blocks
	thinkingRegex := regexp.MustCompile(`(?is)<thinking>.*?</thinking>`)
	text = thinkingRegex.ReplaceAllString(text, "")

	// Remove <reflection>...</reflection> blocks
	reflectionRegex := regexp.MustCompile(`(?is)<reflection>.*?</reflection>`)
	text = reflectionRegex.ReplaceAllString(text, "")

	// Remove [thinking]...[/thinking] blocks
	thinkingBracket := regexp.MustCompile(`(?is)\[thinking\].*?\[/thinking\]`)
	text = thinkingBracket.ReplaceAllString(text, "")

	// Remove *internal thoughts* style (italicized internal monologue)
	// Only if it looks like internal thought (starts and ends with asterisk, multiple words)
	internalThought := regexp.MustCompile(`(?m)^\s*\*[^*]{20,}\*\s*$`)
	text = internalThought.ReplaceAllString(text, "")

	return strings.TrimSpace(text)
}

// isThinkingParagraph checks if a paragraph is internal thinking/meta-dialogue
// Uses the reasoning filter level from config
func (b *AudioBridge) isThinkingParagraph(lowerPara string) bool {
	filterLevel := b.cfg.TTS.ReasoningFilter

	// At 0% filter, don't skip any thinking paragraphs
	if filterLevel < 40 {
		return false
	}

	// LEVEL 40+ - Skip obvious meta/system paragraphs
	level40Starters := []string{
		"i notice",
		"i see that",
		"looking at",
		"from what i can see",
		"based on your",
	}
	if filterLevel >= 40 {
		for _, starter := range level40Starters {
			if strings.HasPrefix(lowerPara, starter) {
				return true
			}
		}
	}

	// LEVEL 60+ - Skip processing/analyzing paragraphs
	level60Starters := []string{
		"analyzing",
		"analysis:",
		"i'm processing",
		"let me understand",
		"before i respond",
		"i want to make sure i understand",
		"the input",
		"given the context",
		"i can deduce",
		"based on the",
		"1.",
		"2.",
		"3.",
	}
	if filterLevel >= 60 {
		for _, starter := range level60Starters {
			if strings.HasPrefix(lowerPara, starter) {
				return true
			}
		}
	}

	// LEVEL 80+ - Skip thinking/reflection starters
	level80Starters := []string{
		"let me think",
		"i'm thinking",
		"hmm,",
		"let me consider",
		"i'm reflecting",
		"let me reflect",
		"working through",
		"i'm considering",
	}
	if filterLevel >= 80 {
		for _, starter := range level80Starters {
			if strings.HasPrefix(lowerPara, starter) {
				return true
			}
		}
	}

	return false
}

// cleanForTTS removes markdown and other formatting not suitable for TTS
func (b *AudioBridge) cleanForTTS(text string) string {
	// Remove markdown bold/italic
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "__", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", " ")

	// Remove markdown headers
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Remove header markers
		for strings.HasPrefix(line, "#") {
			line = strings.TrimPrefix(line, "#")
			line = strings.TrimSpace(line)
		}
		// Skip empty lines
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	text = strings.Join(cleaned, " ")

	// Remove markdown links [text](url) -> text
	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	text = linkRegex.ReplaceAllString(text, "$1")

	// Remove inline code
	text = regexp.MustCompile("`[^`]+`").ReplaceAllString(text, "")

	// Remove emojis (optional - some are fine but complex ones break TTS)
	// Keep common simple ones like ✓ ✗ → but remove complex multi-byte ones
	// For now, let's keep emojis as Piper/macOS TTS handles them okay

	// Remove bullet markers
	text = strings.ReplaceAll(text, "• ", "")
	text = regexp.MustCompile(`^- `).ReplaceAllString(text, "")

	// Collapse multiple spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// GetSTTStatus returns the current STT provider status
func (b *AudioBridge) GetSTTStatus() map[string]any {
	status := map[string]any{
		"provider":  b.sttProvider.Name(),
		"available": false,
		"message":   "",
	}

	if err := b.sttProvider.Health(context.Background()); err != nil {
		status["message"] = err.Error()
	} else {
		status["available"] = true
		status["message"] = "STT ready"
	}

	return status
}

// SetGroqAPIKey sets the Groq API key for STT
func (b *AudioBridge) SetGroqAPIKey(apiKey string) error {
	if apiKey == "" {
		return nil
	}

	// Create new Groq provider with the key
	config := stt.DefaultGroqWhisperConfig()
	config.APIKey = apiKey
	b.sttProvider = stt.NewGroqWhisperProvider(b.logger, config)

	b.logger.Info().Msg("Groq API key set, STT now available")
	return nil
}

// TestTTS triggers a TTS test and returns true if successful
func (b *AudioBridge) TestTTS(text string) bool {
	b.logger.Info().Str("text", text).Msg("TestTTS called")
	if text == "" {
		text = "Hello, this is a test of the text to speech system."
	}
	go b.speakText(text)
	return true
}

// GetTTSProvider returns info about the current TTS provider
func (b *AudioBridge) GetTTSProvider() map[string]any {
	return map[string]any{
		"name":  b.ttsProvider.Name(),
		"voice": b.cfg.TTS.VoiceID,
	}
}

// TranscribeAudio transcribes audio sent from the frontend
// audioBase64 is base64-encoded PCM audio (16kHz, 16-bit, mono)
func (b *AudioBridge) TranscribeAudio(audioBase64 string) (string, error) {
	b.logger.Info().
		Int("base64Len", len(audioBase64)).
		Str("sttProvider", b.sttProvider.Name()).
		Msg("========== TranscribeAudio called ==========")

	audioData, err := base64.StdEncoding.DecodeString(audioBase64)
	if err != nil {
		b.logger.Error().Err(err).Int("base64Len", len(audioBase64)).Msg("FAILED to decode audio base64")
		return "", err
	}

	// Calculate duration
	durationMs := float64(len(audioData)) / 16000 / 2 * 1000 // 16-bit = 2 bytes per sample
	b.logger.Info().
		Int("audioBytes", len(audioData)).
		Float64("durationMs", durationMs).
		Msg("Audio decoded successfully")

	if len(audioData) < 16000 { // Less than 0.5 seconds
		b.logger.Warn().Int("bytes", len(audioData)).Float64("durationMs", durationMs).Msg("Audio too short (<500ms), skipping")
		return "", nil
	}

	b.logger.Info().Msg("Calling STT provider...")
	startTime := time.Now()

	resp, err := b.sttProvider.Transcribe(context.Background(), &stt.TranscribeRequest{
		Audio:      audioData,
		Format:     "pcm",
		SampleRate: 16000,
		Channels:   1,
	})

	transcribeTime := time.Since(startTime)

	if err != nil {
		b.logger.Error().
			Err(err).
			Dur("elapsed", transcribeTime).
			Msg("STT transcription FAILED")
		return "", err
	}

	b.logger.Info().
		Str("transcript", resp.Text).
		Float64("confidence", resp.Confidence).
		Dur("elapsed", transcribeTime).
		Msg("STT transcription SUCCESS")

	return resp.Text, nil
}
