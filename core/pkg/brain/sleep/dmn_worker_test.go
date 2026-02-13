package sleep

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger implements Logger for testing.
type mockLogger struct {
	debugCalls []string
	infoCalls  []string
	warnCalls  []string
	errorCalls []string
}

func (m *mockLogger) Debug(format string, args ...interface{}) {
	m.debugCalls = append(m.debugCalls, format)
}

func (m *mockLogger) Info(format string, args ...interface{}) {
	m.infoCalls = append(m.infoCalls, format)
}

func (m *mockLogger) Warn(format string, args ...interface{}) {
	m.warnCalls = append(m.warnCalls, format)
}

func (m *mockLogger) Error(format string, args ...interface{}) {
	m.errorCalls = append(m.errorCalls, format)
}

func TestDefaultDMNConfig(t *testing.T) {
	config := DefaultDMNConfig()

	assert.True(t, config.EnableOutcomeAggregation, "outcome aggregation should be enabled by default")
	assert.True(t, config.EnableTierPromotion, "tier promotion should be enabled by default")
	assert.Equal(t, 50, config.OutcomeAggregationBatchSize)
	assert.Equal(t, 100, config.TierPromotionBatchSize)
	assert.Equal(t, 5, config.MinSamplesForAggregation)

	// Check tier promotion thresholds
	thresholds := config.TierPromotionThresholds
	assert.Equal(t, 3, thresholds.MinApplyCountForCandidate)
	assert.Equal(t, 10, thresholds.MinApplyCountForProven)
	assert.Equal(t, 0.80, thresholds.MinSuccessRateForProven)
	assert.Equal(t, 25, thresholds.MinApplyCountForIdentity)
	assert.Equal(t, 0.90, thresholds.MinSuccessRateForIdentity)
	assert.Equal(t, 5, thresholds.MinUniqueSessionsForIdentity)
	assert.Equal(t, 30*24*time.Hour, thresholds.MinAgeForIdentity)
}

func TestNewDMNWorker(t *testing.T) {
	config := DefaultDMNConfig()
	logger := &mockLogger{}

	worker := NewDMNWorker(config, nil, logger)

	assert.NotNil(t, worker)
	assert.Equal(t, config, worker.config)
	assert.Equal(t, logger, worker.log)
}

func TestNewDMNWorker_NilLogger(t *testing.T) {
	config := DefaultDMNConfig()

	worker := NewDMNWorker(config, nil, nil)

	assert.NotNil(t, worker)
	assert.NotNil(t, worker.log, "should have noop logger when nil provided")
}

func TestDMNWorker_Run_NoStores(t *testing.T) {
	// Test that DMN Worker runs without errors when no stores are configured
	config := DefaultDMNConfig()
	logger := &mockLogger{}

	worker := NewDMNWorker(config, nil, logger)

	ctx := context.Background()
	result, err := worker.Run(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.TaskTypesProcessed)
	assert.Equal(t, 0, result.MemoriesChecked)
	assert.Equal(t, 0, result.MemoriesPromoted)
	assert.True(t, result.Duration > 0)
}

func TestDMNWorker_Run_Disabled(t *testing.T) {
	config := DefaultDMNConfig()
	config.EnableOutcomeAggregation = false
	config.EnableTierPromotion = false

	logger := &mockLogger{}
	worker := NewDMNWorker(config, nil, logger)

	ctx := context.Background()
	result, err := worker.Run(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.TaskTypesProcessed)
	assert.Equal(t, 0, result.MemoriesChecked)
}

func TestDMNWorker_Run_ContextCancellation(t *testing.T) {
	config := DefaultDMNConfig()
	logger := &mockLogger{}

	worker := NewDMNWorker(config, nil, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := worker.Run(ctx)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Nil(t, result)
}

func TestDMNWorker_CalculateEligibleTier(t *testing.T) {
	config := DefaultDMNConfig()
	worker := NewDMNWorker(config, nil, nil)

	tests := []struct {
		name           string
		applyCount     int
		successRate    float64
		uniqueSessions int
		age            time.Duration
		expectedTier   string
	}{
		{
			name:         "tentative - low apply count",
			applyCount:   1,
			successRate:  0.5,
			expectedTier: "tentative",
		},
		{
			name:         "candidate - meets apply count threshold",
			applyCount:   3,
			successRate:  0.5,
			expectedTier: "candidate",
		},
		{
			name:         "proven - high apply count and success rate",
			applyCount:   10,
			successRate:  0.85,
			expectedTier: "proven",
		},
		{
			name:         "candidate - high apply count but low success rate",
			applyCount:   15,
			successRate:  0.60,
			expectedTier: "candidate",
		},
		{
			name:           "identity - all criteria met",
			applyCount:     30,
			successRate:    0.95,
			uniqueSessions: 10,
			age:            60 * 24 * time.Hour, // 60 days
			expectedTier:   "identity",
		},
		{
			name:           "proven - identity criteria partially met (insufficient age)",
			applyCount:     30,
			successRate:    0.95,
			uniqueSessions: 10,
			age:            10 * 24 * time.Hour, // 10 days - not enough
			expectedTier:   "proven",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdAt := time.Now().Add(-tt.age)
			tier := worker.calculateEligibleTier(tt.applyCount, tt.successRate, tt.uniqueSessions, createdAt)
			assert.Equal(t, tt.expectedTier, tier)
		})
	}
}

func TestCountJSONArrayElements(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"[]", 0},
		{"null", 0},
		{`["a"]`, 1},
		{`["a","b"]`, 2},
		{`["a","b","c"]`, 3},
		{`["session1","session2","session3","session4","session5"]`, 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := countJSONArrayElements(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDMNResult_EmptyInitialization(t *testing.T) {
	result := &DMNResult{}

	assert.Equal(t, 0, result.TaskTypesProcessed)
	assert.Equal(t, 0, result.MemoriesChecked)
	assert.Equal(t, 0, result.MemoriesPromoted)
	assert.Nil(t, result.ModelRankings)
	assert.Nil(t, result.Promotions)
	assert.Nil(t, result.AggregationErrors)
	assert.Nil(t, result.PromotionErrors)
}

func TestModelRanking(t *testing.T) {
	ranking := ModelRanking{
		TaskType:     "coding",
		Rank:         1,
		Provider:     "openai",
		Model:        "gpt-4o",
		SuccessRate:  0.95,
		SampleCount:  100,
		AvgLatencyMs: 500,
	}

	assert.Equal(t, "coding", ranking.TaskType)
	assert.Equal(t, 1, ranking.Rank)
	assert.Equal(t, "openai", ranking.Provider)
	assert.Equal(t, "gpt-4o", ranking.Model)
	assert.Equal(t, 0.95, ranking.SuccessRate)
	assert.Equal(t, 100, ranking.SampleCount)
	assert.Equal(t, 500, ranking.AvgLatencyMs)
}

func TestPromotionRecord(t *testing.T) {
	record := PromotionRecord{
		MemoryID: "strat_123",
		OldTier:  "tentative",
		NewTier:  "candidate",
		Reason:   "Met promotion criteria for candidate tier",
	}

	assert.Equal(t, "strat_123", record.MemoryID)
	assert.Equal(t, "tentative", record.OldTier)
	assert.Equal(t, "candidate", record.NewTier)
	assert.Contains(t, record.Reason, "candidate")
}
