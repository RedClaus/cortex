// Package tts provides macOS native TTS provider using the 'say' command.
// This is a zero-dependency fallback that uses high-quality system voices.
package tts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

// MacOSTTSProvider implements TTS using macOS native 'say' command
type MacOSTTSProvider struct {
	logger zerolog.Logger
	config *MacOSConfig
}

// MacOSConfig holds macOS TTS configuration
type MacOSConfig struct {
	DefaultVoice string `json:"default_voice"` // Samantha, Daniel, etc.
	Rate         int    `json:"rate"`          // Words per minute (default 175)
	OutputFormat string `json:"output_format"` // aiff, mp3, etc.
}

// DefaultMacOSConfig returns sensible defaults for macOS TTS
func DefaultMacOSConfig() *MacOSConfig {
	return &MacOSConfig{
		DefaultVoice: "Samantha", // High quality female voice
		Rate:         175,        // Natural speaking rate
		OutputFormat: "mp3",
	}
}

// NewMacOSTTSProvider creates a new macOS TTS provider
func NewMacOSTTSProvider(logger zerolog.Logger, config *MacOSConfig) *MacOSTTSProvider {
	if config == nil {
		config = DefaultMacOSConfig()
	}

	return &MacOSTTSProvider{
		logger: logger.With().Str("provider", "macos-tts").Logger(),
		config: config,
	}
}

// Name returns the provider identifier
func (p *MacOSTTSProvider) Name() string {
	return "macos"
}

// IsAvailable checks if this is macOS and 'say' command exists
func (p *MacOSTTSProvider) IsAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("say")
	return err == nil
}

// macOS voice mapping to our voice IDs
var macOSVoiceMap = map[string]string{
	// Female voices (OpenAI equivalents)
	"nova":     "Samantha",
	"shimmer":  "Samantha",
	"af_bella": "Samantha",
	"af_sarah": "Karen",

	// Male voices (OpenAI equivalents)
	"onyx":       "Daniel",
	"echo":       "Daniel",
	"am_adam":    "Alex",
	"am_michael": "Daniel",

	// Neutral
	"alloy": "Samantha",
	"fable": "Daniel",
}

// Synthesize converts text to audio using macOS 'say' command
func (p *MacOSTTSProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsAvailable() {
		return nil, fmt.Errorf("macOS TTS not available")
	}

	startTime := time.Now()

	voiceID := req.VoiceID
	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}
	macOSVoice := p.mapVoice(voiceID)

	tmpFile, err := os.CreateTemp("", "tts-*.m4a")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"-v", macOSVoice,
		"-o", tmpPath,
		"--data-format=aac",
	}

	if p.config.Rate != 175 {
		args = append(args, "-r", fmt.Sprintf("%d", p.config.Rate))
	}

	args = append(args, req.Text)

	p.logger.Debug().
		Str("voice", macOSVoice).
		Int("textLen", len(req.Text)).
		Msg("Synthesizing with macOS TTS")

	cmd := exec.CommandContext(ctx, "say", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		p.logger.Error().
			Err(err).
			Str("output", string(output)).
			Msg("macOS TTS failed")
		return nil, fmt.Errorf("say command failed: %w", err)
	}

	audioData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read audio file: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Str("voice", macOSVoice).
		Int("audioBytes", len(audioData)).
		Dur("processingTime", processingTime).
		Msg("macOS TTS synthesis complete")

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         "m4a",
		SampleRate:     22050,
		ProcessingTime: processingTime,
		VoiceID:        voiceID,
		Provider:       p.Name(),
	}, nil
}

// SpeakDirect speaks text directly through system audio (blocking until complete)
func (p *MacOSTTSProvider) SpeakDirect(ctx context.Context, text string, voiceID string) error {
	if !p.IsAvailable() {
		return fmt.Errorf("macOS TTS not available")
	}

	if voiceID == "" {
		voiceID = p.config.DefaultVoice
	}
	macOSVoice := p.mapVoice(voiceID)

	args := []string{"-v", macOSVoice}
	if p.config.Rate != 175 {
		args = append(args, "-r", fmt.Sprintf("%d", p.config.Rate))
	}
	args = append(args, text)

	p.logger.Debug().
		Str("voice", macOSVoice).
		Int("textLen", len(text)).
		Msg("Speaking directly with macOS TTS")

	cmd := exec.CommandContext(ctx, "say", args...)
	return cmd.Run()
}

// mapVoice maps our voice IDs to macOS system voices
func (p *MacOSTTSProvider) mapVoice(voiceID string) string {
	if mapped, ok := macOSVoiceMap[voiceID]; ok {
		return mapped
	}
	// Check if it's already a macOS voice name
	if p.isValidMacOSVoice(voiceID) {
		return voiceID
	}
	return p.config.DefaultVoice
}

// isValidMacOSVoice checks if a voice name is a valid macOS voice
func (p *MacOSTTSProvider) isValidMacOSVoice(name string) bool {
	validVoices := []string{
		"Samantha", "Daniel", "Alex", "Karen", "Victoria", "Zoe",
		"Serena", "Fiona", "Oliver", "Tom", "Fred", "Rishi",
	}
	for _, v := range validVoices {
		if v == name {
			return true
		}
	}
	return false
}

// SynthesizeStream handles streaming synthesis
func (p *MacOSTTSProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
	chunks := make(chan *AudioChunk, 1)

	go func() {
		defer close(chunks)

		resp, err := p.Synthesize(ctx, req)
		if err != nil {
			p.logger.Error().Err(err).Msg("Stream synthesis failed")
			return
		}

		chunks <- &AudioChunk{
			Data:    resp.Audio,
			Index:   0,
			IsFinal: true,
		}
	}()

	return chunks, nil
}

// ListVoices returns available macOS voices
func (p *MacOSTTSProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	// These are the high-quality voices typically available on macOS
	return []Voice{
		{ID: "Samantha", Name: "Samantha (Female, American)", Language: "en-US", Gender: "female"},
		{ID: "Daniel", Name: "Daniel (Male, British)", Language: "en-GB", Gender: "male"},
		{ID: "Alex", Name: "Alex (Male, American)", Language: "en-US", Gender: "male"},
		{ID: "Karen", Name: "Karen (Female, Australian)", Language: "en-AU", Gender: "female"},
		{ID: "Victoria", Name: "Victoria (Female, American)", Language: "en-US", Gender: "female"},
		{ID: "Serena", Name: "Serena (Female, British)", Language: "en-GB", Gender: "female"},
		{ID: "Oliver", Name: "Oliver (Male, British)", Language: "en-GB", Gender: "male"},
	}, nil
}

// Health checks if macOS TTS is available
func (p *MacOSTTSProvider) Health(ctx context.Context) error {
	if !p.IsAvailable() {
		return ErrProviderUnavailable
	}
	return nil
}

// Capabilities returns macOS TTS capabilities
func (p *MacOSTTSProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false,
		SupportsCloning:    false,
		SupportsPhonemes:   false,
		SupportedLanguages: []string{"en"},
		MaxTextLength:      10000,
		AvgLatencyMs:       200,
		RequiresGPU:        false,
		IsLocal:            true,
	}
}

// GetSystemVoices returns all available macOS system voices
func GetSystemVoices() ([]string, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("not macOS")
	}

	// Run: say -v '?'
	cmd := exec.Command("say", "-v", "?")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse output - each line starts with voice name
	var voices []string
	lines := filepath.SplitList(string(output))
	for _, line := range lines {
		if len(line) > 0 {
			// Voice name is first word
			parts := filepath.SplitList(line)
			if len(parts) > 0 {
				voices = append(voices, parts[0])
			}
		}
	}

	return voices, nil
}
