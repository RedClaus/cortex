// Package decomposer provides task complexity analysis and decomposition.
package decomposer

import (
	"regexp"
	"strings"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// COMPLEXITY SCORER
// ═══════════════════════════════════════════════════════════════════════════════

// ComplexityLevel categorizes task complexity.
type ComplexityLevel string

const (
	ComplexitySimple  ComplexityLevel = "simple"  // Score 0-30
	ComplexityMedium  ComplexityLevel = "medium"  // Score 31-70
	ComplexityComplex ComplexityLevel = "complex" // Score 71-100
)

// Thresholds for complexity levels.
const (
	ThresholdSimple = 30
	ThresholdMedium = 70
)

// ComplexityResult contains the analysis of task complexity.
type ComplexityResult struct {
	Score       int             `json:"score"`        // 0-100
	Level       ComplexityLevel `json:"level"`        // simple/medium/complex
	Factors     []string        `json:"factors"`      // Reasons for the score
	NeedsDecomp bool            `json:"needs_decomp"` // Whether decomposition is recommended
}

// Scorer analyzes task complexity.
type Scorer struct {
	// Task type multipliers
	typeMultipliers map[cognitive.TaskType]float64
}

// NewScorer creates a new complexity scorer.
func NewScorer() *Scorer {
	return &Scorer{
		typeMultipliers: map[cognitive.TaskType]float64{
			cognitive.TaskGeneral:        1.0,
			cognitive.TaskCodeGen:        1.3, // Code generation is typically more complex
			cognitive.TaskDebug:          1.2, // Debugging requires investigation
			cognitive.TaskReview:         0.8, // Reviews are usually straightforward
			cognitive.TaskPlanning:       1.4, // Planning is inherently complex
			cognitive.TaskInfrastructure: 1.5, // Infrastructure has high risk
			cognitive.TaskExplain:        0.6, // Explanations are simpler
			cognitive.TaskRefactor:       1.5, // Refactoring is complex
		},
	}
}

// Score calculates the complexity score for a task.
func (s *Scorer) Score(input string, taskType cognitive.TaskType) *ComplexityResult {
	result := &ComplexityResult{
		Factors: make([]string, 0),
	}

	// Base score from various factors
	baseScore := 0.0

	// Factor 1: Token count (rough approximation)
	tokenCount := len(strings.Fields(input))
	tokenScore := float64(tokenCount) / 10.0 // 10 tokens = 1 point
	if tokenScore > 20 {
		tokenScore = 20
	}
	baseScore += tokenScore
	if tokenCount > 50 {
		result.Factors = append(result.Factors, "lengthy request")
	}

	// Factor 2: Step indicators
	stepIndicators := []string{
		"first", "then", "next", "after", "finally", "lastly",
		"step 1", "step 2", "step 3",
		"1.", "2.", "3.",
		"and then", "followed by",
	}
	stepCount := 0
	inputLower := strings.ToLower(input)
	for _, indicator := range stepIndicators {
		if strings.Contains(inputLower, indicator) {
			stepCount++
		}
	}
	stepScore := float64(stepCount) * 5.0
	if stepScore > 15 {
		stepScore = 15
	}
	baseScore += stepScore
	if stepCount >= 3 {
		result.Factors = append(result.Factors, "multi-step process")
	}

	// Factor 3: File references
	filePatterns := []string{
		`\b[\w/-]+\.\w{2,4}\b`,   // file.ext
		`/[\w/-]+`,               // /path/to/something
		`\b\w+/\w+/\w+`,          // dir/subdir/file
	}
	fileCount := 0
	for _, pattern := range filePatterns {
		re := regexp.MustCompile(pattern)
		fileCount += len(re.FindAllString(input, -1))
	}
	fileScore := float64(fileCount) * 3.0
	if fileScore > 15 {
		fileScore = 15
	}
	baseScore += fileScore
	if fileCount >= 3 {
		result.Factors = append(result.Factors, "multiple files involved")
	}

	// Factor 4: Conditional language
	conditionals := []string{
		"if", "else", "otherwise", "unless", "when", "depending",
		"either", "or", "both", "neither",
	}
	conditionalCount := 0
	words := strings.Fields(inputLower)
	for _, word := range words {
		for _, cond := range conditionals {
			if word == cond {
				conditionalCount++
			}
		}
	}
	conditionalScore := float64(conditionalCount) * 4.0
	if conditionalScore > 12 {
		conditionalScore = 12
	}
	baseScore += conditionalScore
	if conditionalCount >= 3 {
		result.Factors = append(result.Factors, "conditional logic")
	}

	// Factor 5: Technical complexity indicators
	technicalTerms := []string{
		"database", "api", "authentication", "authorization", "encryption",
		"migration", "deployment", "kubernetes", "docker", "terraform",
		"microservice", "distributed", "concurrent", "async", "parallel",
		"transaction", "rollback", "failover", "replication",
		"security", "vulnerability", "ssl", "certificate",
	}
	techCount := 0
	for _, term := range technicalTerms {
		if strings.Contains(inputLower, term) {
			techCount++
		}
	}
	techScore := float64(techCount) * 5.0
	if techScore > 20 {
		techScore = 20
	}
	baseScore += techScore
	if techCount >= 2 {
		result.Factors = append(result.Factors, "technical complexity")
	}

	// Factor 6: Question complexity
	questionWords := []string{"why", "how", "explain", "what causes", "what if"}
	questionScore := 0.0
	for _, q := range questionWords {
		if strings.Contains(inputLower, q) {
			questionScore += 3.0
		}
	}
	if questionScore > 10 {
		questionScore = 10
	}
	baseScore += questionScore

	// Apply task type multiplier
	multiplier := s.typeMultipliers[taskType]
	if multiplier == 0 {
		multiplier = 1.0
	}
	finalScore := baseScore * multiplier

	// Clamp to 0-100
	if finalScore < 0 {
		finalScore = 0
	}
	if finalScore > 100 {
		finalScore = 100
	}

	result.Score = int(finalScore)

	// Determine level
	switch {
	case result.Score <= ThresholdSimple:
		result.Level = ComplexitySimple
		result.NeedsDecomp = false
	case result.Score <= ThresholdMedium:
		result.Level = ComplexityMedium
		result.NeedsDecomp = false
	default:
		result.Level = ComplexityComplex
		result.NeedsDecomp = true
	}

	// Add task type factor if it contributed
	if multiplier > 1.2 {
		result.Factors = append(result.Factors, "high-complexity task type")
	}

	// Add default factor if none found
	if len(result.Factors) == 0 {
		if result.Level == ComplexitySimple {
			result.Factors = append(result.Factors, "straightforward request")
		} else {
			result.Factors = append(result.Factors, "moderate complexity")
		}
	}

	return result
}

// GetLevel returns the complexity level for a score.
func GetLevel(score int) ComplexityLevel {
	switch {
	case score <= ThresholdSimple:
		return ComplexitySimple
	case score <= ThresholdMedium:
		return ComplexityMedium
	default:
		return ComplexityComplex
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONVENIENCE FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// ScoreComplexity is a convenience function that creates a scorer and scores the input.
// Returns a score from 0-100.
func ScoreComplexity(input string) int {
	scorer := NewScorer()
	result := scorer.Score(input, cognitive.TaskGeneral)
	return result.Score
}

// ScoreComplexityWithType scores complexity with a specific task type.
func ScoreComplexityWithType(input string, taskType cognitive.TaskType) int {
	scorer := NewScorer()
	result := scorer.Score(input, taskType)
	return result.Score
}

// ShouldDecompose determines if a task should be decomposed based on complexity and template match.
// Returns true if:
// - Complexity score is >= 61 (complex threshold)
// - OR complexity is medium (31-60) AND template match is weak (< 0.70)
func ShouldDecompose(score int, templateMatch float64) bool {
	// Always decompose complex tasks
	if score > ThresholdMedium {
		return true
	}

	// Decompose medium tasks with weak template matches
	if score > ThresholdSimple && templateMatch < cognitive.ThresholdMedium {
		return true
	}

	return false
}

// ShouldDecomposeDetailed provides detailed reasoning for decomposition decision.
func ShouldDecomposeDetailed(score int, templateMatch float64) (bool, string) {
	if score > ThresholdMedium {
		return true, "high complexity score"
	}

	if score > ThresholdSimple && templateMatch < cognitive.ThresholdMedium {
		return true, "medium complexity with weak template match"
	}

	return false, "complexity manageable with available templates"
}
