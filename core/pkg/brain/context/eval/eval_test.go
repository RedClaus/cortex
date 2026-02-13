package eval

import (
	"context"
	"testing"

	ctxpkg "github.com/normanking/cortex/pkg/brain/context"
)

func TestNewEvaluator(t *testing.T) {
	judge := &MockJudge{BaseScore: 75}
	e := NewEvaluator(judge)

	if e == nil {
		t.Fatal("NewEvaluator returned nil")
	}
	if e.judge == nil {
		t.Error("Judge should be set")
	}
}

func TestDefaultEvaluatorConfig(t *testing.T) {
	config := DefaultEvaluatorConfig()

	if config.JudgeModel == "" {
		t.Error("JudgeModel should be set")
	}
	if config.ConcurrentEvals <= 0 {
		t.Error("ConcurrentEvals should be positive")
	}
	if config.TimeoutSeconds <= 0 {
		t.Error("TimeoutSeconds should be positive")
	}
}

func TestAllDimensions(t *testing.T) {
	dims := AllDimensions()

	if len(dims) != 5 {
		t.Errorf("Expected 5 dimensions, got %d", len(dims))
	}

	expected := []string{
		DimensionRelevance,
		DimensionCompleteness,
		DimensionAccuracy,
		DimensionClarity,
		DimensionHelpfulness,
	}

	for _, exp := range expected {
		found := false
		for _, dim := range dims {
			if dim == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing dimension: %s", exp)
		}
	}
}

func TestMockJudge(t *testing.T) {
	judge := &MockJudge{BaseScore: 75}

	req := JudgeRequest{
		UserInput:  "What is Go?",
		Response:   "Go is a programming language developed by Google. It is known for its simplicity, efficiency, and built-in concurrency support.",
		Dimensions: AllDimensions(),
	}

	resp, err := judge.Judge(context.Background(), req)
	if err != nil {
		t.Fatalf("Judge failed: %v", err)
	}

	if resp.OverallScore < 70 || resp.OverallScore > 85 {
		t.Errorf("Expected score around 75-80, got %d", resp.OverallScore)
	}

	if len(resp.DimensionScores) != 5 {
		t.Errorf("Expected 5 dimension scores, got %d", len(resp.DimensionScores))
	}
}

func TestMockJudge_ShortResponse(t *testing.T) {
	judge := &MockJudge{BaseScore: 75}

	req := JudgeRequest{
		UserInput:  "What is Go?",
		Response:   "A language.", // Too short
		Dimensions: AllDimensions(),
	}

	resp, err := judge.Judge(context.Background(), req)
	if err != nil {
		t.Fatalf("Judge failed: %v", err)
	}

	// Short response should be penalized
	if resp.OverallScore >= 75 {
		t.Errorf("Short response should have lower score, got %d", resp.OverallScore)
	}
}

func TestMockJudge_GoldenMatch(t *testing.T) {
	judge := &MockJudge{BaseScore: 75}

	req := JudgeRequest{
		UserInput:    "What is Go?",
		Response:     "Go is a programming language created by Google with great concurrency support.",
		GoldenOutput: "programming language",
		Dimensions:   AllDimensions(),
	}

	resp, err := judge.Judge(context.Background(), req)
	if err != nil {
		t.Fatalf("Judge failed: %v", err)
	}

	// Matching golden should boost score
	if resp.OverallScore <= 75 {
		t.Errorf("Golden match should boost score, got %d", resp.OverallScore)
	}
}

func TestEvaluator_EvaluateStrategy(t *testing.T) {
	judge := &MockJudge{BaseScore: 80}
	e := NewEvaluator(judge)

	strategy := &StrategyConfig{
		ID:   "test-strategy",
		Name: "Test Strategy",
		ZoneConfig: ctxpkg.ZoneConfig{
			Critical:   100,
			Supporting: 200,
			Actionable: 100,
		},
	}

	testCases := []*TestCase{
		{
			ID:       "tc-1",
			Name:     "Basic query",
			Input:    "What is Go?",
			Category: "general",
		},
		{
			ID:       "tc-2",
			Name:     "Code query",
			Input:    "Write a hello world in Go",
			Category: "coding",
		},
	}

	// Simple response generator
	responseGen := func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error) {
		return "This is a test response that provides helpful information about the query. " +
			"It includes relevant details and explains concepts clearly.", nil
	}

	report, err := e.EvaluateStrategy(context.Background(), strategy, testCases, responseGen)
	if err != nil {
		t.Fatalf("EvaluateStrategy failed: %v", err)
	}

	if report.Strategy.ID != strategy.ID {
		t.Errorf("Report strategy mismatch")
	}

	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(report.Results))
	}

	if report.OverallScore < 70 || report.OverallScore > 90 {
		t.Errorf("Expected overall score 70-90, got %.1f", report.OverallScore)
	}
}

func TestEvaluator_CompareStrategies(t *testing.T) {
	judge := &MockJudge{BaseScore: 75}
	e := NewEvaluator(judge)

	strategies := []*StrategyConfig{
		{
			ID:   "strategy-a",
			Name: "Strategy A",
			ZoneConfig: ctxpkg.ZoneConfig{
				Critical:   100,
				Supporting: 200,
				Actionable: 100,
			},
		},
		{
			ID:   "strategy-b",
			Name: "Strategy B",
			ZoneConfig: ctxpkg.ZoneConfig{
				Critical:   200,
				Supporting: 400,
				Actionable: 200,
			},
		},
	}

	testCases := []*TestCase{
		{ID: "tc-1", Name: "Test 1", Input: "Query 1"},
	}

	responseGen := func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error) {
		return "A comprehensive response that addresses the query thoroughly.", nil
	}

	comparison, err := e.CompareStrategies(context.Background(), strategies, testCases, responseGen)
	if err != nil {
		t.Fatalf("CompareStrategies failed: %v", err)
	}

	if len(comparison.Reports) != 2 {
		t.Errorf("Expected 2 reports, got %d", len(comparison.Reports))
	}

	if len(comparison.Ranking) != 2 {
		t.Errorf("Expected 2 rankings, got %d", len(comparison.Ranking))
	}

	if comparison.BestStrategy == "" {
		t.Error("BestStrategy should be set")
	}
}

func TestParseJudgeResponse(t *testing.T) {
	response := `OVERALL_SCORE: 85
RELEVANCE: 90
COMPLETENESS: 80
ACCURACY: 85
CLARITY: 88
HELPFULNESS: 82
REASONING: The response is well-structured and addresses the main points.
ISSUES: Minor formatting issue; Could include more examples
SUGGESTIONS: Add code snippets; Improve structure`

	result, err := parseJudgeResponse(response)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if result.OverallScore != 85 {
		t.Errorf("Expected overall score 85, got %d", result.OverallScore)
	}

	if result.DimensionScores["relevance"] != 90 {
		t.Errorf("Expected relevance 90, got %d", result.DimensionScores["relevance"])
	}

	if result.Reasoning == "" {
		t.Error("Reasoning should be parsed")
	}

	if len(result.Issues) == 0 {
		t.Error("Issues should be parsed")
	}

	if len(result.Suggestions) == 0 {
		t.Error("Suggestions should be parsed")
	}
}

func TestStrategyReport_Statistics(t *testing.T) {
	judge := &MockJudge{BaseScore: 70, ScoreVariance: 10}
	e := NewEvaluator(judge)

	strategy := &StrategyConfig{
		ID: "test",
		ZoneConfig: ctxpkg.ZoneConfig{
			Critical:   100,
			Supporting: 100,
			Actionable: 100,
		},
	}

	testCases := make([]*TestCase, 5)
	for i := 0; i < 5; i++ {
		testCases[i] = &TestCase{
			ID:    string(rune('a' + i)),
			Input: "Test input",
		}
	}

	responseGen := func(ctx context.Context, input string, bb *ctxpkg.AttentionBlackboard) (string, error) {
		return "A sufficiently long response that should meet minimum length requirements.", nil
	}

	report, err := e.EvaluateStrategy(context.Background(), strategy, testCases, responseGen)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	// Check statistics are calculated
	if report.MinScore > report.MaxScore {
		t.Error("MinScore should be <= MaxScore")
	}

	if report.OverallScore < float64(report.MinScore) || report.OverallScore > float64(report.MaxScore) {
		t.Error("OverallScore should be between min and max")
	}

	if report.MedianScore == 0 {
		t.Error("MedianScore should be calculated")
	}
}

func TestTestCase_Fields(t *testing.T) {
	tc := &TestCase{
		ID:                   "test-1",
		Name:                 "Test Case 1",
		Input:                "What is Go?",
		ExpectedCapabilities: []string{"reasoning", "memory"},
		GoldenOutput:         "Go is a programming language",
		Category:             "general",
		Priority:             5,
	}

	if tc.ID == "" {
		t.Error("ID should be set")
	}
	if tc.Name == "" {
		t.Error("Name should be set")
	}
	if len(tc.ExpectedCapabilities) != 2 {
		t.Error("ExpectedCapabilities should be set")
	}
}

func TestStrategyConfig_Fields(t *testing.T) {
	config := &StrategyConfig{
		ID:          "strategy-1",
		Name:        "Test Strategy",
		Description: "A test strategy",
		ZoneConfig: ctxpkg.ZoneConfig{
			Critical:   100,
			Supporting: 200,
			Actionable: 100,
		},
		EnableMasks:         true,
		EnableCompaction:    true,
		CompactionThreshold: 0.85,
		CustomParameters:    map[string]any{"custom": "value"},
	}

	if config.ID == "" {
		t.Error("ID should be set")
	}
	if config.ZoneConfig.Total() != 400 {
		t.Errorf("ZoneConfig total should be 400, got %d", config.ZoneConfig.Total())
	}
	if !config.EnableMasks {
		t.Error("EnableMasks should be true")
	}
}
