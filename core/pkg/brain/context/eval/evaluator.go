package eval

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	ctxpkg "github.com/normanking/cortex/pkg/brain/context"
)

// LLMJudge is the interface for an LLM that evaluates responses.
// This allows the evaluator to work with different LLM backends.
type LLMJudge interface {
	// Judge evaluates a response and returns a score with reasoning.
	Judge(ctx context.Context, request JudgeRequest) (*JudgeResponse, error)
}

// JudgeRequest contains the data for an LLM judge evaluation.
type JudgeRequest struct {
	// UserInput is the original query.
	UserInput string

	// ContextUsed describes what context was provided.
	ContextUsed string

	// Response is the AI's response to evaluate.
	Response string

	// GoldenOutput is an optional reference response.
	GoldenOutput string

	// Dimensions to score.
	Dimensions []string

	// DetailedReasoning requests explanation.
	DetailedReasoning bool
}

// JudgeResponse contains the LLM's evaluation.
type JudgeResponse struct {
	// OverallScore is 0-100.
	OverallScore int

	// DimensionScores for each dimension.
	DimensionScores map[string]int

	// Reasoning explains the score.
	Reasoning string

	// Issues identified.
	Issues []string

	// Suggestions for improvement.
	Suggestions []string
}

// Evaluator evaluates context strategies using LLM-as-Judge.
type Evaluator struct {
	config EvaluatorConfig
	judge  LLMJudge
	mu     sync.Mutex
}

// NewEvaluator creates a new strategy evaluator.
func NewEvaluator(judge LLMJudge) *Evaluator {
	return &Evaluator{
		config: DefaultEvaluatorConfig(),
		judge:  judge,
	}
}

// NewEvaluatorWithConfig creates an evaluator with custom config.
func NewEvaluatorWithConfig(judge LLMJudge, config EvaluatorConfig) *Evaluator {
	return &Evaluator{
		config: config,
		judge:  judge,
	}
}

// EvaluateStrategy runs all test cases against a single strategy.
func (e *Evaluator) EvaluateStrategy(
	ctx context.Context,
	strategy *StrategyConfig,
	testCases []*TestCase,
	responseGenerator func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error),
) (*StrategyReport, error) {
	start := time.Now()
	results := make([]*EvaluationResult, 0, len(testCases))

	// Create blackboard for this strategy
	bb := ctxpkg.NewAttentionBlackboardWithConfig(strategy.ZoneConfig)

	// Evaluate each test case
	for _, tc := range testCases {
		result, err := e.evaluateSingle(ctx, strategy, tc, bb, responseGenerator)
		if err != nil {
			// Log error but continue with other tests
			result = &EvaluationResult{
				TestCaseID: tc.ID,
				StrategyID: strategy.ID,
				Score:      0,
				Issues:     []string{fmt.Sprintf("Evaluation error: %v", err)},
				Timestamp:  time.Now(),
			}
		}
		results = append(results, result)
	}

	return e.generateReport(strategy, results, time.Since(start)), nil
}

// evaluateSingle evaluates a single test case.
func (e *Evaluator) evaluateSingle(
	ctx context.Context,
	strategy *StrategyConfig,
	tc *TestCase,
	bb *ctxpkg.AttentionBlackboard,
	responseGenerator func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error),
) (*EvaluationResult, error) {
	start := time.Now()

	// Generate response
	response, err := responseGenerator(ctx, tc.Input, bb)
	if err != nil {
		return nil, fmt.Errorf("response generation failed: %w", err)
	}

	latency := time.Since(start)
	stats := bb.Stats()

	// Build judge request
	judgeReq := JudgeRequest{
		UserInput:         tc.Input,
		ContextUsed:       fmt.Sprintf("Tokens: %d/%d (%.1f%% utilization)", stats.TotalTokens, stats.BudgetLimit, stats.Utilization*100),
		Response:          response,
		GoldenOutput:      tc.GoldenOutput,
		Dimensions:        AllDimensions(),
		DetailedReasoning: e.config.EnableDetailedReasoning,
	}

	// Get LLM judgment
	judgeResp, err := e.judge.Judge(ctx, judgeReq)
	if err != nil {
		return nil, fmt.Errorf("judge evaluation failed: %w", err)
	}

	return &EvaluationResult{
		TestCaseID:  tc.ID,
		StrategyID:  strategy.ID,
		Score:       judgeResp.OverallScore,
		LatencyMs:   latency.Milliseconds(),
		TokensUsed:  stats.TotalTokens,
		TokenBudget: stats.BudgetLimit,
		Utilization: stats.Utilization,
		Response:    response,
		Reasoning:   judgeResp.Reasoning,
		Dimensions:  judgeResp.DimensionScores,
		Issues:      judgeResp.Issues,
		Suggestions: judgeResp.Suggestions,
		Timestamp:   time.Now(),
	}, nil
}

// generateReport creates a summary report from results.
func (e *Evaluator) generateReport(
	strategy *StrategyConfig,
	results []*EvaluationResult,
	duration time.Duration,
) *StrategyReport {
	report := &StrategyReport{
		Strategy:    strategy,
		Results:     results,
		Duration:    duration,
		ByCategory:  make(map[string]float64),
		ByDimension: make(map[string]float64),
	}

	if len(results) == 0 {
		return report
	}

	// Calculate statistics
	var totalScore, totalLatency, totalUtil float64
	scores := make([]int, 0, len(results))
	dimTotals := make(map[string]float64)
	dimCounts := make(map[string]int)

	for _, r := range results {
		totalScore += float64(r.Score)
		totalLatency += float64(r.LatencyMs)
		totalUtil += r.Utilization
		scores = append(scores, r.Score)

		// Track dimension scores
		for dim, score := range r.Dimensions {
			dimTotals[dim] += float64(score)
			dimCounts[dim]++
		}
	}

	n := float64(len(results))
	report.OverallScore = totalScore / n
	report.AverageLatencyMs = int64(totalLatency / n)
	report.AverageUtilization = totalUtil / n

	// Calculate median
	sort.Ints(scores)
	if len(scores)%2 == 0 {
		report.MedianScore = float64(scores[len(scores)/2-1]+scores[len(scores)/2]) / 2
	} else {
		report.MedianScore = float64(scores[len(scores)/2])
	}

	report.MinScore = scores[0]
	report.MaxScore = scores[len(scores)-1]

	// Calculate dimension averages
	for dim, total := range dimTotals {
		report.ByDimension[dim] = total / float64(dimCounts[dim])
	}

	// Collect common issues
	issueCounts := make(map[string]int)
	for _, r := range results {
		for _, issue := range r.Issues {
			issueCounts[issue]++
		}
	}

	// Sort issues by frequency
	type issueCount struct {
		issue string
		count int
	}
	var sortedIssues []issueCount
	for issue, count := range issueCounts {
		if count > 1 { // Only include issues that occur more than once
			sortedIssues = append(sortedIssues, issueCount{issue, count})
		}
	}
	sort.Slice(sortedIssues, func(i, j int) bool {
		return sortedIssues[i].count > sortedIssues[j].count
	})

	for _, ic := range sortedIssues {
		report.CommonIssues = append(report.CommonIssues, ic.issue)
	}

	// Generate recommendations
	report.Recommendations = e.generateRecommendations(report)

	return report
}

// generateRecommendations creates actionable recommendations.
func (e *Evaluator) generateRecommendations(report *StrategyReport) []string {
	recs := make([]string, 0)

	if report.OverallScore < 70 {
		recs = append(recs, "Overall score is below 70; consider increasing context budget or improving context selection")
	}

	if report.AverageUtilization > 0.9 {
		recs = append(recs, "High utilization (>90%); enable compaction or increase token budget")
	} else if report.AverageUtilization < 0.3 {
		recs = append(recs, "Low utilization (<30%); consider adding more context or reducing budget")
	}

	// Dimension-specific recommendations
	for dim, score := range report.ByDimension {
		if score < 60 {
			switch dim {
			case DimensionRelevance:
				recs = append(recs, "Relevance is low; improve context filtering with lobe masks")
			case DimensionCompleteness:
				recs = append(recs, "Completeness is low; ensure all necessary context is included")
			case DimensionAccuracy:
				recs = append(recs, "Accuracy is low; verify source quality and recency")
			case DimensionClarity:
				recs = append(recs, "Clarity is low; structure context better with zone organization")
			case DimensionHelpfulness:
				recs = append(recs, "Helpfulness is low; focus on actionable context in the Actionable zone")
			}
		}
	}

	return recs
}

// CompareStrategies evaluates multiple strategies and compares them.
func (e *Evaluator) CompareStrategies(
	ctx context.Context,
	strategies []*StrategyConfig,
	testCases []*TestCase,
	responseGenerator func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error),
) (*ComparisonReport, error) {
	reports := make([]*StrategyReport, 0, len(strategies))

	for _, strategy := range strategies {
		report, err := e.EvaluateStrategy(ctx, strategy, testCases, responseGenerator)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate strategy %s: %w", strategy.ID, err)
		}
		reports = append(reports, report)
	}

	return e.generateComparison(strategies, reports), nil
}

// generateComparison creates a comparison report.
func (e *Evaluator) generateComparison(
	strategies []*StrategyConfig,
	reports []*StrategyReport,
) *ComparisonReport {
	comparison := &ComparisonReport{
		Strategies:      strategies,
		Reports:         reports,
		BestByCategory:  make(map[string]string),
		BestByDimension: make(map[string]string),
		Timestamp:       time.Now(),
	}

	if len(reports) == 0 {
		return comparison
	}

	// Rank by overall score
	type ranked struct {
		id    string
		score float64
	}
	rankings := make([]ranked, 0, len(reports))
	for _, report := range reports {
		rankings = append(rankings, ranked{report.Strategy.ID, report.OverallScore})
	}
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].score > rankings[j].score
	})

	for _, r := range rankings {
		comparison.Ranking = append(comparison.Ranking, r.id)
	}
	comparison.BestStrategy = comparison.Ranking[0]

	// Find best by dimension
	for _, dim := range AllDimensions() {
		bestScore := -1.0
		bestID := ""
		for _, report := range reports {
			if score, ok := report.ByDimension[dim]; ok && score > bestScore {
				bestScore = score
				bestID = report.Strategy.ID
			}
		}
		if bestID != "" {
			comparison.BestByDimension[dim] = bestID
		}
	}

	// Generate insights
	comparison.Insights = e.generateInsights(comparison)

	return comparison
}

// generateInsights creates key findings from comparison.
func (e *Evaluator) generateInsights(comparison *ComparisonReport) []string {
	insights := make([]string, 0)

	if len(comparison.Reports) < 2 {
		return insights
	}

	// Score spread
	best := comparison.Reports[0]
	for _, r := range comparison.Reports {
		if r.Strategy.ID == comparison.BestStrategy {
			best = r
			break
		}
	}

	insights = append(insights, fmt.Sprintf("Best strategy: %s with score %.1f/100", best.Strategy.Name, best.OverallScore))

	// Find the worst
	worst := comparison.Reports[len(comparison.Reports)-1]
	if worst.Strategy.ID != best.Strategy.ID {
		spread := best.OverallScore - worst.OverallScore
		if spread > 20 {
			insights = append(insights, fmt.Sprintf("Significant score spread: %.1f points between best and worst", spread))
		}
	}

	// Dimension leaders
	for dim, strategyID := range comparison.BestByDimension {
		if strategyID != comparison.BestStrategy {
			insights = append(insights, fmt.Sprintf("%s leads in %s dimension", strategyID, dim))
		}
	}

	return insights
}

// MockJudge provides a simple judge for testing (no LLM required).
type MockJudge struct {
	// BaseScore is the default score to return.
	BaseScore int

	// ScoreVariance adds randomness to scores.
	ScoreVariance int
}

// Judge implements LLMJudge for testing.
func (m *MockJudge) Judge(ctx context.Context, req JudgeRequest) (*JudgeResponse, error) {
	// Simple heuristic-based scoring
	score := m.BaseScore

	// Adjust based on response length
	if len(req.Response) < 50 {
		score -= 10 // Too short
	} else if len(req.Response) > 500 {
		score += 5 // Detailed
	}

	// Check for golden output match
	if req.GoldenOutput != "" {
		if strings.Contains(strings.ToLower(req.Response), strings.ToLower(req.GoldenOutput)) {
			score += 10
		}
	}

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	// Generate dimension scores
	dimScores := make(map[string]int)
	for _, dim := range req.Dimensions {
		dimScores[dim] = score + (m.ScoreVariance - m.ScoreVariance/2) // Slight variation
	}

	return &JudgeResponse{
		OverallScore:    score,
		DimensionScores: dimScores,
		Reasoning:       "Mock evaluation based on response heuristics",
		Issues:          []string{},
		Suggestions:     []string{},
	}, nil
}

// PromptJudge uses a prompt template to create LLM judge requests.
// This is used when you have a real LLM backend.
type PromptJudge struct {
	// Generate is the function that calls the LLM.
	Generate func(ctx context.Context, prompt string) (string, error)
}

// Judge implements LLMJudge using a prompt template.
func (p *PromptJudge) Judge(ctx context.Context, req JudgeRequest) (*JudgeResponse, error) {
	prompt := fmt.Sprintf(`You are an expert AI response evaluator. Analyze the following AI response and provide a quality assessment.

USER INPUT: %s

CONTEXT USED: %s

AI RESPONSE:
%s

%s

Please evaluate the response on these dimensions (0-100 each):
- Relevance: How well does it address the user's query?
- Completeness: Does it fully address all aspects?
- Accuracy: Is the information correct?
- Clarity: Is it well-structured and easy to understand?
- Helpfulness: Is it actually useful to the user?

Provide your evaluation in this exact format:
OVERALL_SCORE: [0-100]
RELEVANCE: [0-100]
COMPLETENESS: [0-100]
ACCURACY: [0-100]
CLARITY: [0-100]
HELPFULNESS: [0-100]
REASONING: [Your explanation]
ISSUES: [Issue 1]; [Issue 2]; ...
SUGGESTIONS: [Suggestion 1]; [Suggestion 2]; ...`,
		req.UserInput,
		req.ContextUsed,
		req.Response,
		func() string {
			if req.GoldenOutput != "" {
				return fmt.Sprintf("REFERENCE RESPONSE (for comparison): %s", req.GoldenOutput)
			}
			return ""
		}(),
	)

	response, err := p.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return parseJudgeResponse(response)
}

// parseJudgeResponse parses the structured LLM response.
func parseJudgeResponse(response string) (*JudgeResponse, error) {
	result := &JudgeResponse{
		DimensionScores: make(map[string]int),
	}

	// Parse scores using regex
	patterns := map[string]*regexp.Regexp{
		"overall":      regexp.MustCompile(`OVERALL_SCORE:\s*(\d+)`),
		"relevance":    regexp.MustCompile(`RELEVANCE:\s*(\d+)`),
		"completeness": regexp.MustCompile(`COMPLETENESS:\s*(\d+)`),
		"accuracy":     regexp.MustCompile(`ACCURACY:\s*(\d+)`),
		"clarity":      regexp.MustCompile(`CLARITY:\s*(\d+)`),
		"helpfulness":  regexp.MustCompile(`HELPFULNESS:\s*(\d+)`),
	}

	for key, pattern := range patterns {
		if match := pattern.FindStringSubmatch(response); len(match) > 1 {
			score, _ := strconv.Atoi(match[1])
			if key == "overall" {
				result.OverallScore = score
			} else {
				result.DimensionScores[key] = score
			}
		}
	}

	// Parse reasoning (allow multiline with (?s) flag)
	reasoningPattern := regexp.MustCompile(`(?s)REASONING:\s*(.+?)(?:ISSUES:|$)`)
	if match := reasoningPattern.FindStringSubmatch(response); len(match) > 1 {
		result.Reasoning = strings.TrimSpace(match[1])
	}

	// Parse issues (allow multiline)
	issuesPattern := regexp.MustCompile(`(?s)ISSUES:\s*(.+?)(?:SUGGESTIONS:|$)`)
	if match := issuesPattern.FindStringSubmatch(response); len(match) > 1 {
		parts := strings.Split(match[1], ";")
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result.Issues = append(result.Issues, trimmed)
			}
		}
	}

	// Parse suggestions (allow multiline)
	suggestionsPattern := regexp.MustCompile(`(?s)SUGGESTIONS:\s*(.+)$`)
	if match := suggestionsPattern.FindStringSubmatch(response); len(match) > 1 {
		parts := strings.Split(match[1], ";")
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				result.Suggestions = append(result.Suggestions, trimmed)
			}
		}
	}

	// Default overall score if not parsed
	if result.OverallScore == 0 && len(result.DimensionScores) > 0 {
		total := 0
		for _, score := range result.DimensionScores {
			total += score
		}
		result.OverallScore = total / len(result.DimensionScores)
	}

	return result, nil
}
