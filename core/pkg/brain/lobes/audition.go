package lobes

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type AuditionLobe struct {
	audioProcessor AudioProcessor
}

type AudioProcessor interface {
	Transcribe(ctx context.Context, audioData []byte) (*AudioTranscription, error)
	AnalyzeTone(ctx context.Context, audioData []byte) (*ToneAnalysis, error)
}

type AudioTranscription struct {
	Text       string  `json:"text"`
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

type ToneAnalysis struct {
	Sentiment  string  `json:"sentiment"`
	Emotion    string  `json:"emotion"`
	Urgency    float64 `json:"urgency"`
	Confidence float64 `json:"confidence"`
}

func NewAuditionLobe(processor AudioProcessor) *AuditionLobe {
	return &AuditionLobe{audioProcessor: processor}
}

func (l *AuditionLobe) ID() brain.LobeID {
	return brain.LobeAudition
}

func (l *AuditionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	result := AuditionResult{
		HasAudioInput: false,
		Transcription: nil,
		ToneAnalysis:  nil,
	}

	audioData, ok := bb.Get("audio_data")
	if ok {
		if data, valid := audioData.([]byte); valid && l.audioProcessor != nil {
			transcription, err := l.audioProcessor.Transcribe(ctx, data)
			if err == nil {
				result.HasAudioInput = true
				result.Transcription = transcription
			}

			tone, err := l.audioProcessor.AnalyzeTone(ctx, data)
			if err == nil {
				result.ToneAnalysis = tone
			}
		}
	}

	return &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    result,
		Confidence: 0.8,
		Meta: brain.LobeMeta{
			StartedAt: startTime,
			Duration:  time.Since(startTime),
		},
	}, nil
}

func (l *AuditionLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"audio", "sound", "voice", "listen", "hear", "speech", "recording"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}
	return 0.1
}

func (l *AuditionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 300,
		EstimatedTime:   2 * time.Second,
		RequiresGPU:     true,
	}
}

type AuditionResult struct {
	HasAudioInput bool                `json:"has_audio_input"`
	Transcription *AudioTranscription `json:"transcription,omitempty"`
	ToneAnalysis  *ToneAnalysis       `json:"tone_analysis,omitempty"`
}
