// Package eval provides context strategy evaluation using LLM-as-Judge.
//
// The evaluator helps compare different context configurations by:
// 1. Running test cases through different strategies
// 2. Using an LLM to judge response quality
// 3. Scoring and ranking strategies
//
// This enables data-driven decisions about context engineering without
// requiring manual evaluation of each configuration.
package eval

import (
	"time"

	"github.com/normanking/cortex/pkg/brain/context"
)

// TestCase represents a single test for context strategy evaluation.
type TestCase struct {
	// ID is a unique identifier for this test case.
	ID string

	// Name is a human-readable name.
	Name string

	// Input is the user input to process.
	Input string

	// ExpectedCapabilities are the lobes/skills needed.
	ExpectedCapabilities []string

	// GoldenOutput is an optional expected response for comparison.
	GoldenOutput string

	// Category groups test cases (e.g., "coding", "memory", "reasoning").
	Category string

	// Priority indicates importance (1-10, higher = more important).
	Priority int
}

// StrategyConfig defines a context configuration to evaluate.
type StrategyConfig struct {
	// ID identifies this strategy.
	ID string

	// Name is a human-readable name.
	Name string

	// Description explains what this strategy does.
	Description string

	// ZoneConfig defines token budgets per zone.
	ZoneConfig context.ZoneConfig

	// EnableMasks enables per-lobe context filtering.
	EnableMasks bool

	// EnableCompaction enables automatic pruning.
	EnableCompaction bool

	// CompactionThreshold is the utilization threshold for compaction.
	CompactionThreshold float64

	// CustomParameters for strategy-specific settings.
	CustomParameters map[string]any
}

// EvaluationResult contains results from evaluating a single test case.
type EvaluationResult struct {
	// TestCaseID identifies which test was evaluated.
	TestCaseID string

	// StrategyID identifies which strategy was used.
	StrategyID string

	// Score is the quality score (0-100).
	Score int

	// LatencyMs is how long the response took.
	LatencyMs int64

	// TokensUsed is the total tokens in context.
	TokensUsed int

	// TokenBudget is the maximum available tokens.
	TokenBudget int

	// Utilization is TokensUsed/TokenBudget.
	Utilization float64

	// Response is the actual LLM response.
	Response string

	// Reasoning explains why this score was given.
	Reasoning string

	// Dimensions are sub-scores for different quality aspects.
	Dimensions map[string]int

	// Issues are specific problems identified.
	Issues []string

	// Suggestions are improvement recommendations.
	Suggestions []string

	// Timestamp when evaluation occurred.
	Timestamp time.Time
}

// StrategyReport summarizes evaluation of a single strategy.
type StrategyReport struct {
	// Strategy that was evaluated.
	Strategy *StrategyConfig

	// Results for each test case.
	Results []*EvaluationResult

	// OverallScore is the average score across all tests.
	OverallScore float64

	// MedianScore is the median score.
	MedianScore float64

	// MinScore is the lowest score.
	MinScore int

	// MaxScore is the highest score.
	MaxScore int

	// AverageLatencyMs across all tests.
	AverageLatencyMs int64

	// AverageUtilization across all tests.
	AverageUtilization float64

	// ByCategory groups scores by test category.
	ByCategory map[string]float64

	// ByDimension groups scores by quality dimension.
	ByDimension map[string]float64

	// CommonIssues are frequently occurring problems.
	CommonIssues []string

	// Recommendations for improving this strategy.
	Recommendations []string

	// Duration is total evaluation time.
	Duration time.Duration
}

// ComparisonReport compares multiple strategies.
type ComparisonReport struct {
	// Strategies that were compared.
	Strategies []*StrategyConfig

	// Reports for each strategy.
	Reports []*StrategyReport

	// Ranking orders strategies by overall score.
	Ranking []string // Strategy IDs in order

	// BestStrategy is the highest-scoring strategy.
	BestStrategy string

	// BestByCategory maps category to best strategy for that category.
	BestByCategory map[string]string

	// BestByDimension maps dimension to best strategy for that dimension.
	BestByDimension map[string]string

	// Insights are key findings from the comparison.
	Insights []string

	// Timestamp when comparison was generated.
	Timestamp time.Time
}

// Quality dimensions for scoring
const (
	DimensionRelevance   = "relevance"   // How relevant is the response?
	DimensionCompleteness = "completeness" // Does it fully address the query?
	DimensionAccuracy    = "accuracy"    // Is the information correct?
	DimensionClarity     = "clarity"     // Is it clear and well-structured?
	DimensionHelpfulness = "helpfulness" // Is it actually helpful?
)

// AllDimensions returns all quality dimensions.
func AllDimensions() []string {
	return []string{
		DimensionRelevance,
		DimensionCompleteness,
		DimensionAccuracy,
		DimensionClarity,
		DimensionHelpfulness,
	}
}

// EvaluatorConfig configures the strategy evaluator.
type EvaluatorConfig struct {
	// JudgeModel is the LLM to use for judging (e.g., "gpt-4o").
	JudgeModel string

	// ConcurrentEvals is max parallel evaluations.
	ConcurrentEvals int

	// TimeoutSeconds per evaluation.
	TimeoutSeconds int

	// EnableDetailedReasoning includes reasoning in results.
	EnableDetailedReasoning bool

	// EnableDimensionScoring scores each dimension separately.
	EnableDimensionScoring bool

	// CustomPrompt overrides the default judge prompt.
	CustomPrompt string
}

// DefaultEvaluatorConfig returns sensible defaults.
func DefaultEvaluatorConfig() EvaluatorConfig {
	return EvaluatorConfig{
		JudgeModel:              "gpt-4o-mini", // Fast and capable
		ConcurrentEvals:         3,
		TimeoutSeconds:          30,
		EnableDetailedReasoning: true,
		EnableDimensionScoring:  true,
	}
}
