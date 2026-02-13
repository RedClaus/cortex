package audio

import (
	"math"
	"sync"
	"time"
)

// VAD implements Voice Activity Detection using RMS energy analysis.
// For browser-based VAD (more accurate), see frontend implementation.
type VAD struct {
	config      *VADConfig
	mu          sync.RWMutex

	// State
	isActive    bool
	lastActive  time.Time

	// Smoothing
	energyHistory []float64
	historyIndex  int
}

// VADConfig holds VAD configuration
type VADConfig struct {
	Threshold       float64 `json:"threshold"`        // Energy threshold (0-1), default 0.01
	SmoothingFrames int     `json:"smoothing_frames"` // Number of frames to smooth, default 5
	MinSpeechMs     int     `json:"min_speech_ms"`    // Minimum speech duration, default 250
	MaxSilenceMs    int     `json:"max_silence_ms"`   // Max silence before end, default 500
	PaddingMs       int     `json:"padding_ms"`       // Padding before/after speech, default 300
}

// DefaultVADConfig returns sensible defaults
func DefaultVADConfig() *VADConfig {
	return &VADConfig{
		Threshold:       0.01,  // RMS threshold
		SmoothingFrames: 5,
		MinSpeechMs:     250,
		MaxSilenceMs:    500,
		PaddingMs:       300,
	}
}

// NewVAD creates a new VAD instance
func NewVAD(config *VADConfig) *VAD {
	if config == nil {
		config = DefaultVADConfig()
	}

	return &VAD{
		config:        config,
		energyHistory: make([]float64, config.SmoothingFrames),
	}
}

// Process analyzes an audio chunk and returns VAD result
func (v *VAD) Process(audioData []byte, bitDepth int) *VADResult {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Calculate RMS energy
	rms := v.calculateRMS(audioData, bitDepth)

	// Update smoothing history
	v.energyHistory[v.historyIndex] = rms
	v.historyIndex = (v.historyIndex + 1) % len(v.energyHistory)

	// Calculate smoothed energy
	smoothedRMS := v.calculateSmoothedRMS()

	// Determine speech activity
	isSpeech := smoothedRMS >= v.config.Threshold

	if isSpeech {
		v.isActive = true
		v.lastActive = time.Now()
	} else if v.isActive {
		// Check if silence has exceeded max duration
		silenceDuration := time.Since(v.lastActive)
		if silenceDuration > time.Duration(v.config.MaxSilenceMs)*time.Millisecond {
			v.isActive = false
		} else {
			// Still in speech segment (within silence tolerance)
			isSpeech = true
		}
	}

	// Calculate confidence based on how far above/below threshold
	confidence := 0.5
	if isSpeech {
		confidence = math.Min(1.0, 0.5 + (smoothedRMS-v.config.Threshold)*10)
	} else {
		confidence = math.Max(0.0, 0.5 - (v.config.Threshold-smoothedRMS)*10)
	}

	return &VADResult{
		IsSpeech:   isSpeech,
		Confidence: confidence,
		RMS:        smoothedRMS,
	}
}

// calculateRMS computes Root Mean Square energy
func (v *VAD) calculateRMS(audioData []byte, bitDepth int) float64 {
	if len(audioData) == 0 {
		return 0
	}

	var sum float64
	var count int

	switch bitDepth {
	case 16:
		// 16-bit signed PCM
		for i := 0; i+1 < len(audioData); i += 2 {
			sample := int16(audioData[i]) | int16(audioData[i+1])<<8
			normalized := float64(sample) / 32768.0
			sum += normalized * normalized
			count++
		}
	case 32:
		// 32-bit float PCM
		for i := 0; i+3 < len(audioData); i += 4 {
			bits := uint32(audioData[i]) | uint32(audioData[i+1])<<8 | uint32(audioData[i+2])<<16 | uint32(audioData[i+3])<<24
			sample := math.Float32frombits(bits)
			sum += float64(sample * sample)
			count++
		}
	default:
		// Assume 8-bit unsigned PCM
		for _, b := range audioData {
			normalized := (float64(b) - 128.0) / 128.0
			sum += normalized * normalized
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return math.Sqrt(sum / float64(count))
}

// calculateSmoothedRMS returns the average RMS over the history window
func (v *VAD) calculateSmoothedRMS() float64 {
	var sum float64
	for _, e := range v.energyHistory {
		sum += e
	}
	return sum / float64(len(v.energyHistory))
}

// IsActive returns whether speech is currently detected
func (v *VAD) IsActive() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isActive
}

// Reset clears VAD state
func (v *VAD) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.isActive = false
	v.historyIndex = 0
	for i := range v.energyHistory {
		v.energyHistory[i] = 0
	}
}

// UpdateConfig updates VAD configuration
func (v *VAD) UpdateConfig(config *VADConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.config = config

	// Resize history if needed
	if len(v.energyHistory) != config.SmoothingFrames {
		v.energyHistory = make([]float64, config.SmoothingFrames)
		v.historyIndex = 0
	}
}
