// Package tts provides Piper neural TTS provider using local ONNX models.
// Piper is a fast, local text-to-speech system with high quality voices.
// https://github.com/rhasspy/piper
package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// PiperProvider implements TTS using local Piper neural TTS
type PiperProvider struct {
	logger     zerolog.Logger
	config     *PiperConfig
	binaryPath string
}

// PiperConfig holds Piper TTS configuration
type PiperConfig struct {
	BinaryPath   string `json:"binary_path"`   // Path to piper binary
	ModelsDir    string `json:"models_dir"`    // Directory containing .onnx models
	DefaultVoice string `json:"default_voice"` // Default voice model (e.g., "en_US-amy-medium")
}

// DefaultPiperConfig returns sensible defaults for Piper TTS
func DefaultPiperConfig() *PiperConfig {
	homeDir, _ := os.UserHomeDir()
	return &PiperConfig{
		BinaryPath:   filepath.Join(homeDir, "Library/Python/3.11/bin/piper"),
		ModelsDir:    filepath.Join(homeDir, ".cortex/piper-voices"),
		DefaultVoice: "en_US-amy-medium",
	}
}

// NewPiperProvider creates a new Piper TTS provider
func NewPiperProvider(logger zerolog.Logger, config *PiperConfig) *PiperProvider {
	if config == nil {
		config = DefaultPiperConfig()
	}

	// Try to find piper binary
	binaryPath := config.BinaryPath
	if binaryPath == "" {
		// Try common locations
		homeDir, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(homeDir, "Library/Python/3.11/bin/piper"),
			filepath.Join(homeDir, ".local/bin/piper"),
			"/usr/local/bin/piper",
			"/opt/homebrew/bin/piper",
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				binaryPath = path
				break
			}
		}
	}

	return &PiperProvider{
		logger:     logger.With().Str("provider", "piper-tts").Logger(),
		config:     config,
		binaryPath: binaryPath,
	}
}

// Name returns the provider identifier
func (p *PiperProvider) Name() string {
	return "piper"
}

// IsAvailable checks if Piper is installed and models are available
func (p *PiperProvider) IsAvailable() bool {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return false
	}

	// Check if binary exists
	if p.binaryPath == "" {
		p.logger.Debug().Msg("Piper binary not found")
		return false
	}

	if _, err := os.Stat(p.binaryPath); err != nil {
		p.logger.Debug().Str("path", p.binaryPath).Msg("Piper binary not accessible")
		return false
	}

	// Check if default voice model exists
	modelPath := p.getModelPath(p.config.DefaultVoice)
	if _, err := os.Stat(modelPath); err != nil {
		p.logger.Debug().Str("model", modelPath).Msg("Piper model not found")
		return false
	}

	return true
}

// Voice mapping from OpenAI/standard voice IDs to Piper models
var piperVoiceMap = map[string]string{
	// Female voices (mapped to amy)
	"nova":      "en_US-amy-medium",
	"shimmer":   "en_US-amy-medium",
	"af_bella":  "en_US-amy-medium",
	"af_sarah":  "en_US-amy-medium",
	"Samantha":  "en_US-amy-medium",

	// Male voices (mapped to lessac)
	"onyx":       "en_US-lessac-medium",
	"echo":       "en_US-lessac-medium",
	"am_adam":    "en_US-lessac-medium",
	"am_michael": "en_US-lessac-medium",
	"Daniel":     "en_US-lessac-medium",

	// Neutral/other
	"alloy": "en_US-amy-medium",
	"fable": "en_US-lessac-medium",
}

// Synthesize converts text to audio using Piper TTS
func (p *PiperProvider) Synthesize(ctx context.Context, req *SynthesizeRequest) (*SynthesizeResponse, error) {
	if !p.IsAvailable() {
		return nil, fmt.Errorf("Piper TTS not available")
	}

	startTime := time.Now()

	// Sanitize and limit text to prevent Piper errors
	text := sanitizeTextForPiper(req.Text)
	if len(text) > 500 {
		// Piper can struggle with very long text, truncate to first 500 chars
		text = text[:500] + "..."
		p.logger.Debug().Int("original", len(req.Text)).Int("truncated", len(text)).Msg("Truncated text for Piper")
	}

	if len(text) == 0 {
		return nil, fmt.Errorf("empty text after sanitization")
	}

	// Map voice ID to Piper model
	modelName := p.mapVoice(req.VoiceID)
	modelPath := p.getModelPath(modelName)

	// Check if model exists
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("Piper model not found: %s", modelPath)
	}

	// Create temp file for output
	tmpFile, err := os.CreateTemp("", "piper-*.wav")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	p.logger.Debug().
		Str("model", modelName).
		Str("modelPath", modelPath).
		Int("textLen", len(text)).
		Msg("Synthesizing with Piper TTS")

	// Build piper command
	// echo "text" | piper --model model.onnx -f output.wav
	cmd := exec.CommandContext(ctx, p.binaryPath,
		"--model", modelPath,
		"-f", tmpPath,
	)

	// Pass sanitized text via stdin
	cmd.Stdin = bytes.NewBufferString(text)

	// Capture stderr for debugging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		p.logger.Error().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("Piper TTS failed")
		return nil, fmt.Errorf("piper command failed: %w", err)
	}

	// Read the generated audio file
	audioData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read audio file: %w", err)
	}

	processingTime := time.Since(startTime)

	p.logger.Info().
		Str("model", modelName).
		Int("audioBytes", len(audioData)).
		Dur("processingTime", processingTime).
		Msg("Piper TTS synthesis complete")

	return &SynthesizeResponse{
		Audio:          audioData,
		Format:         "wav",
		SampleRate:     22050, // Piper default
		ProcessingTime: processingTime,
		VoiceID:        req.VoiceID,
		Provider:       p.Name(),
	}, nil
}

// mapVoice maps standard voice IDs to Piper model names
func (p *PiperProvider) mapVoice(voiceID string) string {
	if voiceID == "" {
		return p.config.DefaultVoice
	}
	if mapped, ok := piperVoiceMap[voiceID]; ok {
		return mapped
	}
	// Check if it's already a Piper model name
	modelPath := p.getModelPath(voiceID)
	if _, err := os.Stat(modelPath); err == nil {
		return voiceID
	}
	return p.config.DefaultVoice
}

// getModelPath returns the full path to a Piper model
func (p *PiperProvider) getModelPath(modelName string) string {
	return filepath.Join(p.config.ModelsDir, modelName+".onnx")
}

// SynthesizeStream handles streaming synthesis (not supported, returns single chunk)
func (p *PiperProvider) SynthesizeStream(ctx context.Context, req *SynthesizeRequest) (<-chan *AudioChunk, error) {
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

// ListVoices returns available Piper voices
func (p *PiperProvider) ListVoices(ctx context.Context) ([]Voice, error) {
	var voices []Voice

	// List available models in the models directory
	files, err := os.ReadDir(p.config.ModelsDir)
	if err != nil {
		return nil, fmt.Errorf("read models dir: %w", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".onnx" {
			modelName := file.Name()[:len(file.Name())-5] // Remove .onnx extension

			// Parse model name for display (e.g., "en_US-amy-medium" -> "Amy (Female, American)")
			voice := Voice{
				ID:       modelName,
				Name:     formatVoiceName(modelName),
				Language: "en-US",
				Gender:   guessGender(modelName),
			}
			voices = append(voices, voice)
		}
	}

	return voices, nil
}

// formatVoiceName formats a Piper model name for display
func formatVoiceName(modelName string) string {
	switch modelName {
	case "en_US-amy-medium":
		return "Amy (Female, American)"
	case "en_US-lessac-medium":
		return "Lessac (Male, American)"
	default:
		return modelName
	}
}

// guessGender guesses gender from model name
func guessGender(modelName string) string {
	// Known female voices
	femaleVoices := []string{"amy", "jenny", "kathleen", "libritts"}
	for _, name := range femaleVoices {
		if bytes.Contains([]byte(modelName), []byte(name)) {
			return "female"
		}
	}
	return "male"
}

// Health checks if Piper TTS is available
func (p *PiperProvider) Health(ctx context.Context) error {
	if !p.IsAvailable() {
		return ErrProviderUnavailable
	}
	return nil
}

// Capabilities returns Piper TTS capabilities
func (p *PiperProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsStreaming:  false,
		SupportsCloning:    false,
		SupportsPhonemes:   false,
		SupportedLanguages: []string{"en"},
		MaxTextLength:      500, // Limited to prevent errors
		AvgLatencyMs:       100, // Very fast local inference
		RequiresGPU:        false,
		IsLocal:            true,
	}
}

// sanitizeTextForPiper cleans text to prevent Piper errors
func sanitizeTextForPiper(text string) string {
	// Remove markdown formatting
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1") // Bold
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")     // Italic
	text = regexp.MustCompile("`[^`]+`").ReplaceAllString(text, "")          // Code
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1") // Links

	// Remove code blocks
	text = regexp.MustCompile("(?s)```[^`]*```").ReplaceAllString(text, "")

	// Remove bullet points and numbering
	text = regexp.MustCompile(`(?m)^[\s]*[-*•]\s*`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?m)^[\s]*\d+\.\s*`).ReplaceAllString(text, "")

	// Replace multiple newlines with single space
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	// Remove special characters that might cause issues
	text = strings.ReplaceAll(text, "\"", "'")
	text = strings.ReplaceAll(text, "\\", "")
	text = strings.ReplaceAll(text, "\t", " ")

	// Trim whitespace
	text = strings.TrimSpace(text)

	return text
}
