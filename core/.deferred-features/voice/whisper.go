package voice

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WhisperService handles speech-to-text transcription using whisper.cpp.
type WhisperService struct {
	config WhisperConfig
}

// NewWhisperService creates a new Whisper service instance.
func NewWhisperService(config WhisperConfig) (*WhisperService, error) {
	// Set defaults
	if config.DefaultModelSize == "" {
		config.DefaultModelSize = "base"
	}
	if config.MaxAudioSize == 0 {
		config.MaxAudioSize = 25 * 1024 * 1024 // 25MB
	}
	if config.TempDir == "" {
		config.TempDir = os.TempDir()
	}
	if config.NumThreads == 0 {
		config.NumThreads = 4
	}

	// Validate model path exists
	if config.ModelPath != "" {
		if _, err := os.Stat(config.ModelPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("model path does not exist: %s", config.ModelPath)
		}
	}

	// Validate executable exists
	if config.ExecutablePath != "" {
		if _, err := exec.LookPath(config.ExecutablePath); err != nil {
			return nil, fmt.Errorf("whisper executable not found: %s", config.ExecutablePath)
		}
	} else {
		// Try to find whisper in PATH
		if _, err := exec.LookPath("whisper"); err != nil {
			return nil, fmt.Errorf("whisper.cpp executable not found in PATH (install from https://github.com/ggerganov/whisper.cpp)")
		}
		config.ExecutablePath = "whisper"
	}

	return &WhisperService{config: config}, nil
}

// Transcribe converts audio to text using whisper.cpp.
func (s *WhisperService) Transcribe(req TranscriptionRequest) (*TranscriptionResponse, error) {
	startTime := time.Now()

	// Validate audio data
	if len(req.AudioData) == 0 {
		return &TranscriptionResponse{Error: "empty audio data"}, fmt.Errorf("empty audio data")
	}
	if int64(len(req.AudioData)) > s.config.MaxAudioSize {
		return &TranscriptionResponse{Error: "audio file too large"}, fmt.Errorf("audio exceeds max size of %d bytes", s.config.MaxAudioSize)
	}

	// Determine model size
	modelSize := req.ModelSize
	if modelSize == "" {
		modelSize = s.config.DefaultModelSize
	}

	// Create temporary audio file
	tempFile, err := s.createTempAudioFile(req.AudioData, req.Format)
	if err != nil {
		return &TranscriptionResponse{Error: err.Error()}, err
	}
	defer os.Remove(tempFile)

	// Build whisper.cpp command
	args := s.buildWhisperCommand(tempFile, modelSize, req.Language)

	// Execute whisper.cpp
	cmd := exec.Command(s.config.ExecutablePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("whisper execution failed: %v - %s", err, string(output))
		return &TranscriptionResponse{Error: errMsg}, fmt.Errorf("whisper execution failed: %w - %s", err, string(output))
	}

	// Parse whisper output
	response, err := s.parseWhisperOutput(string(output), req.Language)
	if err != nil {
		return &TranscriptionResponse{Error: err.Error()}, err
	}

	response.ProcessingTime = time.Since(startTime)
	return response, nil
}

// createTempAudioFile writes audio data to a temporary file.
func (s *WhisperService) createTempAudioFile(audioData []byte, format string) (string, error) {
	ext := format
	if ext == "" {
		ext = "wav" // Default to WAV
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	tempFile := filepath.Join(s.config.TempDir, fmt.Sprintf("whisper_%d%s", time.Now().UnixNano(), ext))
	if err := os.WriteFile(tempFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("failed to write temp audio file: %v", err)
	}

	return tempFile, nil
}

// buildWhisperCommand constructs the whisper.cpp command arguments.
func (s *WhisperService) buildWhisperCommand(audioFile, modelSize, language string) []string {
	args := []string{
		"-f", audioFile,
		"-m", s.getModelPath(modelSize),
		"-t", fmt.Sprintf("%d", s.config.NumThreads),
		"-oj", // Output JSON
	}

	// Add language if specified
	if language != "" {
		args = append(args, "-l", language)
	}

	// Add GPU flag if enabled
	if s.config.EnableGPU {
		args = append(args, "-ng")
	}

	return args
}

// getModelPath returns the full path to the whisper model file.
func (s *WhisperService) getModelPath(modelSize string) string {
	if s.config.ModelPath == "" {
		// Default: assume models are in ~/.whisper/
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, ".whisper", fmt.Sprintf("ggml-%s.bin", modelSize))
	}
	return filepath.Join(s.config.ModelPath, fmt.Sprintf("ggml-%s.bin", modelSize))
}

// parseWhisperOutput parses the JSON output from whisper.cpp.
func (s *WhisperService) parseWhisperOutput(output, language string) (*TranscriptionResponse, error) {
	// whisper.cpp with -oj flag outputs JSON
	// Expected format:
	// {
	//   "transcription": [
	//     {"timestamps": {"from": "00:00:00,000", "to": "00:00:02,500"}, "text": "Hello world"}
	//   ]
	// }

	var whisperOutput struct {
		Transcription []struct {
			Timestamps struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"timestamps"`
			Text string `json:"text"`
		} `json:"transcription"`
	}

	// Try to find JSON in output (whisper.cpp may have additional text)
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		// No JSON found, treat entire output as text
		return &TranscriptionResponse{
			Text:       strings.TrimSpace(output),
			Confidence: 0.8, // Assume moderate confidence
			Language:   language,
		}, nil
	}

	jsonOutput := output[jsonStart:]
	if err := json.Unmarshal([]byte(jsonOutput), &whisperOutput); err != nil {
		// Fallback: treat output as plain text
		return &TranscriptionResponse{
			Text:       strings.TrimSpace(output),
			Confidence: 0.7,
			Language:   language,
		}, nil
	}

	// Build response
	var fullText strings.Builder
	var segments []TranscriptionSegment
	for i, segment := range whisperOutput.Transcription {
		fullText.WriteString(segment.Text)
		fullText.WriteString(" ")

		segments = append(segments, TranscriptionSegment{
			ID:         i,
			Start:      parseTimestamp(segment.Timestamps.From),
			End:        parseTimestamp(segment.Timestamps.To),
			Text:       segment.Text,
			Confidence: 0.85, // whisper.cpp doesn't provide per-segment confidence
		})
	}

	return &TranscriptionResponse{
		Text:       strings.TrimSpace(fullText.String()),
		Confidence: 0.85,
		Language:   language,
		Segments:   segments,
	}, nil
}

// parseTimestamp converts "HH:MM:SS,mmm" to seconds.
func parseTimestamp(ts string) float64 {
	parts := strings.Split(ts, ":")
	if len(parts) != 3 {
		return 0
	}

	var hours, minutes float64
	var secondsParts []string

	fmt.Sscanf(parts[0], "%f", &hours)
	fmt.Sscanf(parts[1], "%f", &minutes)
	secondsParts = strings.Split(parts[2], ",")

	var seconds, millis float64
	if len(secondsParts) >= 1 {
		fmt.Sscanf(secondsParts[0], "%f", &seconds)
	}
	if len(secondsParts) >= 2 {
		fmt.Sscanf(secondsParts[1], "%f", &millis)
	}

	return hours*3600 + minutes*60 + seconds + millis/1000
}
