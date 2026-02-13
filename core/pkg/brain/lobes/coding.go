package lobes

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/llm"
	"github.com/normanking/cortex/pkg/brain"
)

// CodingLobe handles software development tasks.
type CodingLobe struct {
	llm LLMProvider
}

// NewCodingLobe creates a new coding lobe.
func NewCodingLobe(llm LLMProvider) *CodingLobe {
	return &CodingLobe{
		llm: llm,
	}
}

// ID returns brain.LobeCoding
func (l *CodingLobe) ID() brain.LobeID {
	return brain.LobeCoding
}

// Process generates or analyzes code based on input.
func (l *CodingLobe) Process(ctx context.Context, input brain.LobeInput, bb *brain.Blackboard) (*brain.LobeResult, error) {
	startTime := time.Now()

	// Construct system prompt
	systemPrompt := `You are an expert software engineer and architect. 
Your goal is to analyze, write, or refactor code with high precision and adherence to best practices.
Follow these guidelines:
1. Write clean, idiomatic, and efficient code.
2. Include comments explaining complex logic, but avoid obvious comments.
3. If fixing a bug, explain the root cause and the solution.
4. If refactoring, explain the benefits of the changes.
5. Consider edge cases and error handling.
6. Use the context provided to understand the project structure and conventions.`

	// Build context string from blackboard
	var contextBuilder strings.Builder
	if bb != nil {
		if len(bb.Memories) > 0 {
			contextBuilder.WriteString("\nRelevant Memories:\n")
			for _, mem := range bb.Memories {
				contextBuilder.WriteString(fmt.Sprintf("- %s\n", mem.Content))
			}
		}
		// We could add more context from entities or data if needed
	}

	userMessage := input.RawInput
	if contextBuilder.Len() > 0 {
		userMessage = fmt.Sprintf("Context:\n%s\n\nTask:\n%s", contextBuilder.String(), input.RawInput)
	}

	req := &llm.ChatRequest{
		Model:        "", // Use provider's default model
		SystemPrompt: systemPrompt,
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: userMessage,
			},
		},
		Temperature: 0.2, // Lower temperature for code generation
	}

	resp, err := l.llm.Chat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("coding lobe llm error: %w", err)
	}

	result := &brain.LobeResult{
		LobeID:     l.ID(),
		Content:    resp.Content,
		Confidence: 1.0, // Coding tasks usually expect high confidence if successful
		Meta: brain.LobeMeta{
			StartedAt:  startTime,
			Duration:   time.Since(startTime),
			TokensUsed: resp.TokensUsed,
			ModelUsed:  resp.Model,
		},
	}

	return result, nil
}

// CanHandle returns high confidence for code-related queries.
func (l *CodingLobe) CanHandle(input string) float64 {
	lowerInput := strings.ToLower(input)

	keywords := []string{"code", "implement", "function", "class", "bug", "fix", "refactor", "struct", "interface", "api"}
	extensions := []string{".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs"}

	for _, kw := range keywords {
		if strings.Contains(lowerInput, kw) {
			return 0.9
		}
	}

	for _, ext := range extensions {
		if strings.Contains(lowerInput, ext) {
			return 0.95
		}
	}

	return 0.1
}

// ResourceEstimate returns higher resource requirements for code generation.
func (l *CodingLobe) ResourceEstimate(input brain.LobeInput) brain.ResourceEstimate {
	return brain.ResourceEstimate{
		EstimatedTokens: 2000,
		EstimatedTime:   10 * time.Second, // Rough estimate
		RequiresGPU:     true,
	}
}
