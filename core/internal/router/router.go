package router

import (
	"sync"
	"time"
)

const (
	// DefaultConfidenceThreshold is the minimum confidence for fast path.
	// Below this threshold, semantic classification is used.
	DefaultConfidenceThreshold = 0.7
)

// SmartRouter implements the Fast/Slow classification pattern.
// It tries fast regex classification first, falling back to semantic
// classification when confidence is below the threshold.
type SmartRouter struct {
	fast            *FastClassifier
	slow            *SlowClassifier
	threshold       float64
	specialists     map[TaskType]*Specialist
	riskAssessor    RiskAssessor
	enableSemanticFallback bool

	// Statistics (thread-safe)
	stats RouterStats
	mu    sync.RWMutex
}

// SmartRouterOption is a functional option for configuring SmartRouter.
type SmartRouterOption func(*SmartRouter)

// WithConfidenceThreshold sets a custom confidence threshold.
func WithConfidenceThreshold(threshold float64) SmartRouterOption {
	return func(r *SmartRouter) {
		r.threshold = threshold
	}
}

// WithSpecialists adds specialist configurations.
func WithSpecialists(specialists map[TaskType]*Specialist) SmartRouterOption {
	return func(r *SmartRouter) {
		r.specialists = specialists
	}
}

// WithSemanticFallback enables/disables semantic fallback.
func WithSemanticFallback(enabled bool) SmartRouterOption {
	return func(r *SmartRouter) {
		r.enableSemanticFallback = enabled
	}
}

// WithRiskAssessor sets a custom risk assessor.
func WithRiskAssessor(assessor RiskAssessor) SmartRouterOption {
	return func(r *SmartRouter) {
		r.riskAssessor = assessor
	}
}

// NewSmartRouter creates a new SmartRouter with the given LLM router.
// If llm is nil, semantic fallback will be disabled.
func NewSmartRouter(llm LLMRouter, opts ...SmartRouterOption) *SmartRouter {
	r := &SmartRouter{
		fast:            NewFastClassifier(),
		threshold:       DefaultConfidenceThreshold,
		specialists:     make(map[TaskType]*Specialist),
		riskAssessor:    &DefaultRiskAssessor{},
		enableSemanticFallback: true,
		stats: RouterStats{
			TaskTypeDistribution: make(map[TaskType]int64),
		},
	}

	// Only create slow classifier if LLM is provided
	if llm != nil {
		r.slow = NewSlowClassifier(llm)
	} else {
		r.enableSemanticFallback = false
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Route classifies a user request and returns a routing decision.
// This is the main entry point for the router.
func (r *SmartRouter) Route(input string, ctx *ProcessContext) *RoutingDecision {
	start := time.Now()

	// 1. Check for explicit @mention (always fast path)
	if mention, remaining := ExtractMention(input); mention != "" {
		if taskType, ok := GetTaskTypeFromMention(mention); ok {
			return r.buildDecision(taskType, remaining, 1.0, PathExplicit, start, &classificationStats{
				explicit: 1,
			})
		}
	}

	// 2. Check for platform context (always infrastructure)
	if ctx != nil && ctx.Platform != nil {
		return r.buildDecision(TaskInfrastructure, input, 0.9, PathContext, start, &classificationStats{
			context: 1,
		})
	}

	// 3. Fast Path: Regex classification with confidence
	taskType, confidence := r.fast.Classify(input)

	if confidence >= r.threshold {
		return r.buildDecision(taskType, input, confidence, PathFast, start, &classificationStats{
			fast: 1,
		})
	}

	// 4. Slow Path: Semantic classification (only when ambiguous and enabled)
	stats := &classificationStats{
		ambiguous: 1,
	}

	if r.enableSemanticFallback && r.slow != nil {
		semanticType, semanticConfidence, err := r.slow.Classify(input)
		if err == nil {
			stats.slow = 1
			return r.buildDecision(semanticType, input, semanticConfidence, PathSlow, start, stats)
		}
		// On error, fall through to use fast path result
	}

	// Fallback to fast path result (even with low confidence)
	stats.fast = 1
	return r.buildDecision(taskType, input, confidence, PathFast, start, stats)
}

// RouteSimple is a convenience method that routes without context.
func (r *SmartRouter) RouteSimple(input string) *RoutingDecision {
	return r.Route(input, nil)
}

// classificationStats holds stats from a single classification
type classificationStats struct {
	fast      int64
	slow      int64
	explicit  int64
	context   int64
	ambiguous int64
}

// buildDecision constructs a RoutingDecision with all metadata.
func (r *SmartRouter) buildDecision(
	taskType TaskType,
	input string,
	confidence float64,
	path ClassificationPath,
	start time.Time,
	stats *classificationStats,
) *RoutingDecision {
	duration := time.Since(start)

	// Update statistics under lock
	r.mu.Lock()
	r.stats.TotalRequests++
	if stats.fast > 0 {
		r.stats.FastHits++
	}
	if stats.slow > 0 {
		r.stats.SlowHits++
	}
	if stats.explicit > 0 {
		r.stats.ExplicitHits++
	}
	if stats.context > 0 {
		r.stats.ContextHits++
	}
	if stats.ambiguous > 0 {
		r.stats.AmbiguousCount++
	}
	r.stats.TaskTypeDistribution[taskType]++
	// Update running average confidence
	total := float64(r.stats.TotalRequests)
	r.stats.AverageConfidence = (r.stats.AverageConfidence*(total-1) + confidence) / total
	r.mu.Unlock()

	// Assess risk
	riskLevel := r.riskAssessor.Assess(input, taskType)

	// Find specialist
	specialist := ""
	if spec, ok := r.specialists[taskType]; ok {
		specialist = spec.Name
	}

	return &RoutingDecision{
		TaskType:               taskType,
		Input:                  input,
		Confidence:             confidence,
		Path:                   path,
		RiskLevel:              riskLevel,
		Specialist:             specialist,
		ClassifiedAt:           time.Now(),
		ClassificationDuration: duration,
	}
}

// Stats returns a copy of the current routing statistics.
func (r *SmartRouter) Stats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create a copy of task type distribution
	distCopy := make(map[TaskType]int64, len(r.stats.TaskTypeDistribution))
	for k, v := range r.stats.TaskTypeDistribution {
		distCopy[k] = v
	}

	return RouterStats{
		FastHits:             r.stats.FastHits,
		SlowHits:             r.stats.SlowHits,
		ExplicitHits:         r.stats.ExplicitHits,
		ContextHits:          r.stats.ContextHits,
		AmbiguousCount:       r.stats.AmbiguousCount,
		TotalRequests:        r.stats.TotalRequests,
		AverageConfidence:    r.stats.AverageConfidence,
		TaskTypeDistribution: distCopy,
	}
}

// ResetStats resets all routing statistics.
func (r *SmartRouter) ResetStats() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.stats.FastHits = 0
	r.stats.SlowHits = 0
	r.stats.ExplicitHits = 0
	r.stats.ContextHits = 0
	r.stats.AmbiguousCount = 0
	r.stats.TotalRequests = 0
	r.stats.AverageConfidence = 0
	r.stats.TaskTypeDistribution = make(map[TaskType]int64)
}

// RiskAssessor determines the risk level of a request.
type RiskAssessor interface {
	Assess(input string, taskType TaskType) RiskLevel
}

// DefaultRiskAssessor implements basic risk assessment.
type DefaultRiskAssessor struct{}

// Assess determines risk based on input content and task type.
func (a *DefaultRiskAssessor) Assess(input string, taskType TaskType) RiskLevel {
	// Check for dangerous patterns
	dangerousPatterns := []string{
		"rm -rf",
		"drop table",
		"delete from",
		"format",
		"fdisk",
		"mkfs",
		"dd if=",
		"> /dev/",
		"chmod 777",
		":(){ :|:& };:",
	}

	inputLower := stringToLower(input)
	for _, pattern := range dangerousPatterns {
		if containsIgnoreCase(inputLower, pattern) {
			return RiskCritical
		}
	}

	// Risk by task type
	switch taskType {
	case TaskInfrastructure:
		// Infrastructure tasks are inherently riskier
		if containsIgnoreCase(inputLower, "production") ||
			containsIgnoreCase(inputLower, "prod") ||
			containsIgnoreCase(inputLower, "delete") ||
			containsIgnoreCase(inputLower, "destroy") {
			return RiskHigh
		}
		return RiskMedium

	case TaskCodeGen:
		// Writing code has medium risk (modifies files)
		return RiskMedium

	case TaskDebug:
		// Debugging might involve running code
		return RiskMedium

	case TaskReview, TaskExplain, TaskPlanning:
		// Read-only or informational tasks
		return RiskLow

	case TaskRefactor:
		// Modifies existing code
		return RiskMedium

	default:
		return RiskLow
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(stringToLower(s) == stringToLower(substr) ||
			indexIgnoreCase(s, substr) >= 0)
}

// stringToLower converts string to lowercase.
func stringToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// indexIgnoreCase returns the index of substr in s (case-insensitive).
func indexIgnoreCase(s, substr string) int {
	sLower := stringToLower(s)
	substrLower := stringToLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return i
		}
	}
	return -1
}
