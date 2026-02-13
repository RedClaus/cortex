package introspection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/memory"
)

// LearningOutcome represents the result of verifying a learning session.
type LearningOutcome struct {
	Subject          string       `json:"subject"`
	Verified         bool         `json:"verified"`
	ItemsRetrievable int          `json:"items_retrievable"`
	SampleQueries    []string     `json:"sample_queries"`
	TestResults      []TestResult `json:"test_results"`
	Timestamp        time.Time    `json:"timestamp"`
}

// TestResult represents the result of a single verification query.
type TestResult struct {
	Query       string  `json:"query"`
	Found       bool    `json:"found"`
	ResultCount int     `json:"result_count"`
	TopScore    float64 `json:"top_score"`
}

// MetaLearningPattern represents a pattern observed about the learning process itself.
type MetaLearningPattern struct {
	Subject           string  `json:"subject"`
	AcquisitionType   string  `json:"acquisition_type"`
	Success           bool    `json:"success"`
	ItemsAcquired     int     `json:"items_acquired"`
	RetrievalAccuracy float64 `json:"retrieval_accuracy"`
	UserSatisfied     bool    `json:"user_satisfied"`
}

// StrategicMemoryCreator defines the interface for creating and searching strategic memories.
type StrategicMemoryCreator interface {
	Create(ctx context.Context, mem *memory.StrategicMemory) error
	SearchSimilar(ctx context.Context, query string, limit int) ([]memory.StrategicMemory, error)
}

// LearningConfirmation handles the verification of acquired knowledge and recording of meta-learning patterns.
type LearningConfirmation struct {
	inventory      *memory.KnowledgeInventory
	strategicStore StrategicMemoryCreator
	llmProvider    LLMProvider
}

// NewLearningConfirmation creates a new LearningConfirmation instance.
func NewLearningConfirmation(
	inventory *memory.KnowledgeInventory,
	strategic StrategicMemoryCreator,
	llm LLMProvider,
) *LearningConfirmation {
	return &LearningConfirmation{
		inventory:      inventory,
		strategicStore: strategic,
		llmProvider:    llm,
	}
}

// Verify tests if the acquired knowledge is retrievable.
func (lc *LearningConfirmation) Verify(ctx context.Context, subject string, acquisitionType string, acquisitionResult *AcquisitionResult) (*LearningOutcome, error) {
	outcome := &LearningOutcome{
		Subject:   subject,
		Timestamp: time.Now(),
	}

	outcome.SampleQueries = lc.generateTestQueries(subject)

	successCount := 0
	totalRelevance := 0.0

	for _, query := range outcome.SampleQueries {
		// Guard against nil inventory (CR-018 fix)
		if lc.inventory == nil {
			continue
		}
		result, err := lc.inventory.Query(ctx, query, nil)
		if err != nil {
			fmt.Printf("Error querying inventory for verification: %v\n", err)
			continue
		}

		testResult := TestResult{
			Query:       query,
			ResultCount: result.TotalMatches,
			Found:       result.TotalMatches > 0,
		}

		if result.TotalMatches > 0 && len(result.TopResults) > 0 {
			testResult.TopScore = result.TopResults[0].Relevance
			totalRelevance += testResult.TopScore
			successCount++
		}

		outcome.TestResults = append(outcome.TestResults, testResult)
	}

	if len(outcome.SampleQueries) > 0 {
		successRate := float64(successCount) / float64(len(outcome.SampleQueries))
		outcome.Verified = successRate >= 0.5

		if outcome.Verified && acquisitionResult != nil && acquisitionResult.Success {
			pattern := &MetaLearningPattern{
				Subject:           subject,
				AcquisitionType:   acquisitionType,
				Success:           true,
				ItemsAcquired:     acquisitionResult.ItemsIngested,
				RetrievalAccuracy: successRate,
				UserSatisfied:     true,
			}

			// Guard against nil strategicStore in goroutine (CR-018 fix)
			if lc.strategicStore != nil {
				go func() {
					if err := lc.RecordMetaLearning(context.Background(), pattern); err != nil {
						fmt.Printf("Error recording meta-learning: %v\n", err)
					}
				}()
			}
		}
	}

	maxItems := 0
	for _, tr := range outcome.TestResults {
		if tr.ResultCount > maxItems {
			maxItems = tr.ResultCount
		}
	}
	outcome.ItemsRetrievable = maxItems

	return outcome, nil
}

// generateTestQueries creates a set of queries to test retrieval of the subject.
func (lc *LearningConfirmation) generateTestQueries(subject string) []string {
	queries := []string{
		subject,
		"what is " + subject,
		subject + " example",
	}

	isCommandLike := !strings.Contains(subject, " ") ||
		strings.Contains(strings.ToLower(subject), "cli") ||
		strings.Contains(strings.ToLower(subject), "command")

	if isCommandLike {
		queries = append(queries, "how to use "+subject)
		queries = append(queries, subject+" options")
		queries = append(queries, subject+" syntax")
	}

	return queries
}

// RecordMetaLearning stores successful learning patterns in strategic memory.
func (lc *LearningConfirmation) RecordMetaLearning(ctx context.Context, pattern *MetaLearningPattern) error {
	if !pattern.Success {
		return nil
	}

	principle := fmt.Sprintf("For learning about '%s', %s is effective (acquired %d items, %.0f%% retrieval accuracy)",
		pattern.Subject,
		pattern.AcquisitionType,
		pattern.ItemsAcquired,
		pattern.RetrievalAccuracy*100,
	)

	triggerPattern := lc.extractTriggerPattern(pattern.Subject)

	mem := &memory.StrategicMemory{
		Principle:      principle,
		Category:       "meta_learning",
		TriggerPattern: triggerPattern,
		SuccessCount:   1,
		Confidence:     pattern.RetrievalAccuracy,
	}

	return lc.strategicStore.Create(ctx, mem)
}

// GetLearningRecommendation recommends an acquisition strategy based on past success.
func (lc *LearningConfirmation) GetLearningRecommendation(ctx context.Context, subject string) (string, float64, error) {
	query := "meta_learning " + subject

	memories, err := lc.strategicStore.SearchSimilar(ctx, query, 5)
	if err != nil {
		return "", 0, err
	}

	var bestType string
	var bestScore float64 = 0

	for _, mem := range memories {
		if mem.Category != "meta_learning" {
			continue
		}

		parts := strings.Split(mem.Principle, ", ")
		if len(parts) < 2 {
			continue
		}

		rest := parts[1]
		typeEnd := strings.Index(rest, " is effective")
		if typeEnd == -1 {
			continue
		}

		acqType := rest[:typeEnd]
		score := mem.Confidence

		if score > bestScore {
			bestScore = score
			bestType = acqType
		}
	}

	if bestType != "" {
		return bestType, bestScore, nil
	}

	return "", 0, fmt.Errorf("no recommendation found")
}

// extractTriggerPattern creates a simplified trigger pattern from the subject.
func (lc *LearningConfirmation) extractTriggerPattern(subject string) string {
	stopwords := map[string]bool{
		"how": true, "to": true, "use": true, "what": true, "is": true,
		"a": true, "an": true, "the": true, "for": true, "in": true,
		"on": true, "about": true, "learn": true,
	}

	words := strings.Fields(strings.ToLower(subject))
	var keyTerms []string

	for _, w := range words {
		if !stopwords[w] {
			keyTerms = append(keyTerms, w)
		}
	}

	if len(keyTerms) == 0 {
		return subject
	}

	return strings.Join(keyTerms, " ")
}
