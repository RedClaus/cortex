package brain

import (
	"sync"
	"time"
)

// ExecutionRecord captures a complete execution for learning.
type ExecutionRecord struct {
	ID             string               `json:"id"`
	Input          string               `json:"input"`
	Classification ClassificationResult `json:"classification"`
	Strategy       ThinkingStrategy     `json:"strategy"`
	Result         *ExecutionResult     `json:"result"`
	Outcome        Outcome              `json:"outcome"`
	Feedback       *UserFeedback        `json:"feedback,omitempty"`
	SystemMetrics  SystemMetrics        `json:"system_metrics"`
	CreatedAt      time.Time            `json:"created_at"`
}

// Outcome represents the result quality assessment.
type Outcome struct {
	Success         bool    `json:"success"`
	ConfidenceScore float64 `json:"confidence_score"`
	LatencyMS       int64   `json:"latency_ms"`
	TokensUsed      int     `json:"tokens_used"`
	ReplanCount     int     `json:"replan_count"`
	ErrorMessage    string  `json:"error_message,omitempty"`
}

// UserFeedback captures explicit user feedback on a response.
type UserFeedback struct {
	Rating    int       `json:"rating"` // 1-5
	Comment   string    `json:"comment,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// OutcomeLogger records execution outcomes for analysis.
type OutcomeLogger struct {
	mu      sync.RWMutex
	records []ExecutionRecord
	maxSize int
	store   RecordStore
}

// RecordStore persists execution records.
type RecordStore interface {
	Save(record ExecutionRecord) error
	Query(filter RecordFilter) ([]ExecutionRecord, error)
}

// RecordFilter specifies criteria for querying records.
type RecordFilter struct {
	Since       time.Time
	Until       time.Time
	LobeID      LobeID
	MinRating   int
	SuccessOnly bool
	Limit       int
}

// NewOutcomeLogger creates a logger with optional persistence.
func NewOutcomeLogger(store RecordStore, maxInMemory int) *OutcomeLogger {
	if maxInMemory <= 0 {
		maxInMemory = 1000
	}
	return &OutcomeLogger{
		records: make([]ExecutionRecord, 0, maxInMemory),
		maxSize: maxInMemory,
		store:   store,
	}
}

// Log records an execution outcome.
func (l *OutcomeLogger) Log(record ExecutionRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	record.CreatedAt = time.Now()

	if len(l.records) >= l.maxSize {
		l.records = l.records[1:]
	}
	l.records = append(l.records, record)

	if l.store != nil {
		return l.store.Save(record)
	}
	return nil
}

// GetRecent returns the most recent records.
func (l *OutcomeLogger) GetRecent(n int) []ExecutionRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n <= 0 || n > len(l.records) {
		n = len(l.records)
	}

	start := len(l.records) - n
	result := make([]ExecutionRecord, n)
	copy(result, l.records[start:])
	return result
}

// GetStats returns aggregate statistics.
func (l *OutcomeLogger) GetStats() LearningStats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats := LearningStats{
		TotalExecutions: len(l.records),
		LobeUsage:       make(map[LobeID]int),
		StrategyUsage:   make(map[string]int),
	}

	var totalLatency int64
	var successCount int
	var totalTokens int

	for _, r := range l.records {
		stats.LobeUsage[r.Classification.PrimaryLobe]++
		stats.StrategyUsage[r.Strategy.Name]++
		totalLatency += r.Outcome.LatencyMS
		totalTokens += r.Outcome.TokensUsed

		if r.Outcome.Success {
			successCount++
		}

		if r.Feedback != nil {
			stats.TotalFeedback++
			stats.AvgRating += float64(r.Feedback.Rating)
		}
	}

	if len(l.records) > 0 {
		stats.AvgLatencyMS = float64(totalLatency) / float64(len(l.records))
		stats.SuccessRate = float64(successCount) / float64(len(l.records))
		stats.AvgTokensUsed = float64(totalTokens) / float64(len(l.records))
	}

	if stats.TotalFeedback > 0 {
		stats.AvgRating /= float64(stats.TotalFeedback)
	}

	return stats
}

// LearningStats provides aggregate metrics.
type LearningStats struct {
	TotalExecutions int            `json:"total_executions"`
	SuccessRate     float64        `json:"success_rate"`
	AvgLatencyMS    float64        `json:"avg_latency_ms"`
	AvgTokensUsed   float64        `json:"avg_tokens_used"`
	AvgRating       float64        `json:"avg_rating"`
	TotalFeedback   int            `json:"total_feedback"`
	LobeUsage       map[LobeID]int `json:"lobe_usage"`
	StrategyUsage   map[string]int `json:"strategy_usage"`
}

// AddFeedback associates user feedback with the most recent execution.
func (l *OutcomeLogger) AddFeedback(rating int, comment string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.records) == 0 {
		return false
	}

	idx := len(l.records) - 1
	l.records[idx].Feedback = &UserFeedback{
		Rating:    rating,
		Comment:   comment,
		Timestamp: time.Now(),
	}
	return true
}
