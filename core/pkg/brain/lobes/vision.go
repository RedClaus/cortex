package lobes

import (
	"context"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

type VisionLobe struct {
	imageProcessor ImageProcessor
}

type ImageProcessor interface {
	Analyze(ctx context.Context, imageData []byte) (*ImageAnalysis, error)
}

type ImageAnalysis struct {
	Description string           `json:"description"`
	Objects     []DetectedObject `json:"objects"`
	Text        string           `json:"text"`
	Confidence  float64          `json:"confidence"`
}

type DetectedObject struct {
	Label       string  `json:"label"`
	Confidence  float64 `json:"confidence"`
	BoundingBox [4]int  `json:"bounding_box"`
}

func NewVisionLobe(processor ImageProcessor) *VisionLobe {
	return &VisionLobe{imageProcessor: processor}
}

func (l *VisionLobe) ID() brain.LobeID {
	return brain.LobeVision
}

func (l *VisionLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	result := VisionResult{
		HasVisualInput: false,
		Analysis:       nil,
	}

	imageData, ok := bb.Get("image_data")
	if ok {
		if data, valid := imageData.([]byte); valid && l.imageProcessor != nil {
			analysis, err := l.imageProcessor.Analyze(ctx, data)
			if err == nil {
				result.HasVisualInput = true
				result.Analysis = analysis
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

func (l *VisionLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)
	keywords := []string{"image", "picture", "photo", "see", "look at", "visual", "screenshot"}
	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}
	return 0.1
}

func (l *VisionLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 500,
		EstimatedTime:   3 * time.Second,
		RequiresGPU:     true,
	}
}

type VisionResult struct {
	HasVisualInput bool           `json:"has_visual_input"`
	Analysis       *ImageAnalysis `json:"analysis,omitempty"`
}
