package lobes

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/normanking/cortex/pkg/brain"
)

// TextParsingLobe handles text input parsing and entity extraction.
type TextParsingLobe struct {
	entityPatterns map[string]*regexp.Regexp
}

// NewTextParsingLobe creates a text parsing lobe with default patterns.
func NewTextParsingLobe() *TextParsingLobe {
	return &TextParsingLobe{
		entityPatterns: map[string]*regexp.Regexp{
			"url":        regexp.MustCompile(`https?://[^\s]+`),
			"email":      regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			"filepath":   regexp.MustCompile(`(?:/?[\w\.-]+)+/[\w\.-]+`),
			"code_block": regexp.MustCompile(`(?s)\x60{3}.*?\x60{3}`),
			"number":     regexp.MustCompile(`\b\d+(\.\d+)?\b`),
			"date":       regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`),
		},
	}
}

// ID returns brain.LobeTextParsing
func (l *TextParsingLobe) ID() brain.LobeID {
	return brain.LobeTextParsing
}

// Process extracts entities and structures from input text.
func (l *TextParsingLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	start := time.Now()
	text := input.RawInput

	// Extract entities based on patterns
	for entityType, pattern := range l.entityPatterns {
		matches := pattern.FindAllStringIndex(text, -1)
		for _, match := range matches {
			val := text[match[0]:match[1]]
			bb.AddEntity(brain.Entity{
				Type:  entityType,
				Value: val,
				Start: match[0],
				End:   match[1],
			})
		}
	}

	// Extract intent hints
	lowerText := strings.ToLower(strings.TrimSpace(text))
	questionStarters := []string{"who", "what", "when", "where", "why", "how"}
	for _, starter := range questionStarters {
		if strings.HasPrefix(lowerText, starter) {
			bb.AddEntity(brain.Entity{
				Type:  "intent_hint",
				Value: "question_" + starter,
				Start: 0,
				End:   len(starter),
			})
			break
		}
	}

	return &brain.LobeResult{
		LobeID:  l.ID(),
		Content: "extracted_entities", // Simple indicator
		Meta: brain.LobeMeta{
			StartedAt:  start,
			Duration:   time.Since(start),
			TokensUsed: 0,
			ModelUsed:  "regex_engine",
			CacheHit:   false,
		},
		Confidence: 1.0, // Parsing is deterministic
	}, nil
}

// CanHandle returns high confidence for all text (always runs as preprocessing).
func (l *TextParsingLobe) CanHandle(input string) float64 {
	return 0.8
}

// ResourceEstimate returns minimal requirements (regex only).
func (l *TextParsingLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 0,
		EstimatedTime:   time.Millisecond,
		RequiresGPU:     false,
	}
}
