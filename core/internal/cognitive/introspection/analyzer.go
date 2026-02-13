package introspection

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/normanking/cortex/internal/memory"
)

// GapAnalyzer analyzes knowledge gaps and recommends actions.
type GapAnalyzer struct {
	llmProvider LLMProvider
}

// NewGapAnalyzer creates a new gap analyzer.
func NewGapAnalyzer(llm LLMProvider) *GapAnalyzer {
	return &GapAnalyzer{
		llmProvider: llm,
	}
}

// Analyze performs gap analysis on inventory results.
func (ga *GapAnalyzer) Analyze(ctx context.Context, query *IntrospectionQuery, inventory *memory.InventoryResult) (*GapAnalysis, error) {
	analysis := &GapAnalysis{
		Subject: query.Subject,
	}

	// Determine if we have stored knowledge
	if inventory != nil {
		analysis.HasStoredKnowledge = inventory.TotalMatches > 0
		analysis.StoredKnowledgeCount = inventory.TotalMatches
	}

	// Assess LLM capability for this subject
	if ga.llmProvider != nil {
		llmAssessment, err := ga.assessLLMCapability(ctx, query.Subject)
		if err != nil {
			// Default to assuming LLM can help somewhat
			analysis.LLMCanAnswer = true
			analysis.LLMConfidence = 0.5
		} else {
			analysis.LLMCanAnswer = llmAssessment.CanAnswer
			analysis.LLMConfidence = llmAssessment.Confidence
		}
	} else {
		// No LLM provider, assume moderate capability
		analysis.LLMCanAnswer = true
		analysis.LLMConfidence = 0.5
	}

	// Determine gap severity
	analysis.GapSeverity = ga.determineGapSeverity(analysis)

	// Generate acquisition options
	analysis.AcquisitionOptions = ga.generateAcquisitionOptions(query.Subject)

	// Determine recommended action
	analysis.RecommendedAction = ga.determineRecommendedAction(analysis)

	return analysis, nil
}

// LLMAssessment represents LLM's self-assessment of capability.
type LLMAssessment struct {
	CanAnswer   bool    `json:"can_answer"`
	Confidence  float64 `json:"confidence"`
	Explanation string  `json:"explanation"`
}

// assessLLMCapability asks the LLM about its own knowledge on a subject.
func (ga *GapAnalyzer) assessLLMCapability(ctx context.Context, subject string) (*LLMAssessment, error) {
	prompt := `You are being asked to assess your own knowledge about a topic.

Topic: "` + subject + `"

Respond in JSON format only:
{
  "can_answer": true/false,
  "confidence": 0.0-1.0,
  "explanation": "brief explanation of your knowledge level"
}

Be honest about your limitations. Consider:
- Do you have training data on this topic?
- Can you provide accurate, helpful information?
- Are there caveats or limitations to your knowledge?

Common confidence levels:
- 0.9+: Core knowledge area (programming, general concepts)
- 0.7-0.9: Good knowledge with some gaps
- 0.5-0.7: Partial knowledge, may need verification
- 0.3-0.5: Limited knowledge, user should verify
- <0.3: Very limited or no knowledge`

	response, err := ga.llmProvider.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return parseLLMAssessment(response)
}

// parseLLMAssessment parses the LLM's self-assessment response.
func parseLLMAssessment(response string) (*LLMAssessment, error) {
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	var result LLMAssessment
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Default to moderate confidence
		return &LLMAssessment{
			CanAnswer:   true,
			Confidence:  0.5,
			Explanation: "Unable to assess",
		}, nil
	}

	return &result, nil
}

// determineGapSeverity calculates overall gap severity.
func (ga *GapAnalyzer) determineGapSeverity(analysis *GapAnalysis) GapSeverity {
	// Good stored knowledge = no gap
	if analysis.StoredKnowledgeCount >= 10 {
		return GapSeverityNone
	}

	// Some stored knowledge = minimal gap
	if analysis.StoredKnowledgeCount > 0 {
		return GapSeverityMinimal
	}

	// No stored knowledge - severity depends on LLM capability
	if analysis.LLMCanAnswer && analysis.LLMConfidence > 0.7 {
		return GapSeverityModerate
	}

	if analysis.LLMCanAnswer && analysis.LLMConfidence > 0.4 {
		return GapSeverityModerate
	}

	return GapSeveritySevere
}

// generateAcquisitionOptions creates options for filling the gap.
func (ga *GapAnalyzer) generateAcquisitionOptions(subject string) []AcquisitionOption {
	options := []AcquisitionOption{
		{
			Type:        AcquisitionTypeFile,
			Description: "Ingest a file containing " + subject + " information",
			Confidence:  0.9, // File ingest is reliable if content is good
			Effort:      "low",
		},
		{
			Type:        AcquisitionTypeWebSearch,
			Description: "Search the internet for " + subject + " and learn from results",
			Confidence:  0.7, // Web search quality varies
			Effort:      "medium",
		},
	}

	// Add documentation crawl for programming topics
	if ga.isProgrammingTopic(subject) {
		options = append(options, AcquisitionOption{
			Type:        AcquisitionTypeDocCrawl,
			Description: "Crawl official documentation for " + subject,
			Confidence:  0.85,
			Effort:      "high",
		})
	}

	return options
}

// determineRecommendedAction picks the best action.
func (ga *GapAnalyzer) determineRecommendedAction(analysis *GapAnalysis) string {
	if analysis.GapSeverity == GapSeverityNone {
		return "use_stored_knowledge"
	}

	if analysis.GapSeverity == GapSeverityMinimal {
		return "supplement_with_llm"
	}

	if analysis.GapSeverity == GapSeverityModerate && analysis.LLMCanAnswer {
		return "offer_llm_and_acquisition"
	}

	return "recommend_acquisition"
}

// isProgrammingTopic checks if subject is programming-related.
func (ga *GapAnalyzer) isProgrammingTopic(subject string) bool {
	markers := []string{
		"python", "javascript", "go", "golang", "rust", "java", "c++",
		"programming", "coding", "development", "api", "library",
		"framework", "sdk", "typescript", "ruby", "swift", "kotlin",
		"react", "vue", "angular", "node", "npm", "docker", "kubernetes",
	}

	subjectLower := strings.ToLower(subject)
	for _, marker := range markers {
		if strings.Contains(subjectLower, marker) {
			return true
		}
	}
	return false
}
