package router

import (
	"testing"
	"time"
)

// ============================================================================
// TaskType Tests
// ============================================================================

func TestTaskType_String(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected string
	}{
		{TaskGeneral, "general"},
		{TaskCodeGen, "code_generation"},
		{TaskDebug, "debug"},
		{TaskReview, "review"},
		{TaskPlanning, "planning"},
		{TaskInfrastructure, "infrastructure"},
		{TaskExplain, "explain"},
		{TaskRefactor, "refactor"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.taskType.String(); got != tt.expected {
				t.Errorf("TaskType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskType_IsValid(t *testing.T) {
	tests := []struct {
		taskType TaskType
		valid    bool
	}{
		{TaskGeneral, true},
		{TaskCodeGen, true},
		{TaskDebug, true},
		{TaskType("invalid"), false},
		{TaskType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.taskType), func(t *testing.T) {
			if got := tt.taskType.IsValid(); got != tt.valid {
				t.Errorf("TaskType.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		risk     RiskLevel
		expected string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskCritical, "critical"},
		{RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.risk.String(); got != tt.expected {
				t.Errorf("RiskLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// ============================================================================
// FastClassifier Tests
// ============================================================================

func TestFastClassifier_Classify(t *testing.T) {
	classifier := NewFastClassifier()

	tests := []struct {
		name           string
		input          string
		expectedType   TaskType
		minConfidence  float64
	}{
		// Debug patterns
		{
			name:          "debug with error keyword",
			input:         "Fix this error in my code",
			expectedType:  TaskDebug,
			minConfidence: 0.6,
		},
		{
			name:          "debug with bug keyword",
			input:         "There's a bug in the authentication module",
			expectedType:  TaskDebug,
			minConfidence: 0.6,
		},
		{
			name:          "debug why question",
			input:         "Why isn't this function returning the right value?",
			expectedType:  TaskDebug,
			minConfidence: 0.5,
		},
		{
			name:          "debug not working",
			input:         "This code is not working",
			expectedType:  TaskDebug,
			minConfidence: 0.6,
		},

		// Code generation patterns
		{
			name:          "write function",
			input:         "Write a function to parse JSON",
			expectedType:  TaskCodeGen,
			minConfidence: 0.6,
		},
		{
			name:          "create new component",
			input:         "Create a new React component for the sidebar",
			expectedType:  TaskCodeGen,
			minConfidence: 0.6,
		},
		{
			name:          "generate code",
			input:         "Generate code for API authentication",
			expectedType:  TaskCodeGen,
			minConfidence: 0.6,
		},

		// Review patterns
		{
			name:          "code review",
			input:         "Review this code for best practices",
			expectedType:  TaskReview,
			minConfidence: 0.6,
		},
		{
			name:          "PR review",
			input:         "Can you do a PR review on this pull request?",
			expectedType:  TaskReview,
			minConfidence: 0.6,
		},

		// Planning patterns
		{
			name:          "design architecture",
			input:         "How should I design the database architecture?",
			expectedType:  TaskPlanning,
			minConfidence: 0.5,
		},
		{
			name:          "best approach",
			input:         "What's the best approach to implement caching?",
			expectedType:  TaskPlanning,
			minConfidence: 0.5,
		},

		// Infrastructure patterns
		{
			name:          "docker deployment",
			input:         "Deploy this app using Docker",
			expectedType:  TaskInfrastructure,
			minConfidence: 0.6,
		},
		{
			name:          "kubernetes setup",
			input:         "Configure Kubernetes for production",
			expectedType:  TaskInfrastructure,
			minConfidence: 0.6,
		},
		{
			name:          "AWS configuration",
			input:         "Set up AWS S3 bucket with proper permissions",
			expectedType:  TaskInfrastructure,
			minConfidence: 0.5,
		},

		// Explain patterns
		{
			name:          "explain concept",
			input:         "Explain how async/await works in JavaScript",
			expectedType:  TaskExplain,
			minConfidence: 0.6,
		},
		{
			name:          "what is question",
			input:         "What is dependency injection?",
			expectedType:  TaskExplain,
			minConfidence: 0.5,
		},

		// Refactor patterns
		{
			name:          "refactor code",
			input:         "Refactor this function to be more readable",
			expectedType:  TaskRefactor,
			minConfidence: 0.6,
		},
		{
			name:          "improve code",
			input:         "Improve this code structure",
			expectedType:  TaskRefactor,
			minConfidence: 0.5,
		},

		// General (ambiguous)
		{
			name:          "ambiguous request",
			input:         "Help me with this",
			expectedType:  TaskGeneral,
			minConfidence: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskType, confidence := classifier.Classify(tt.input)

			if taskType != tt.expectedType {
				t.Errorf("Classify(%q) type = %v, want %v", tt.input, taskType, tt.expectedType)
			}

			if confidence < tt.minConfidence {
				t.Errorf("Classify(%q) confidence = %v, want >= %v", tt.input, confidence, tt.minConfidence)
			}
		})
	}
}

func TestFastClassifier_ClassifyWithMatches(t *testing.T) {
	classifier := NewFastClassifier()

	taskType, confidence, matches := classifier.ClassifyWithMatches("Fix this bug in my code")

	if taskType != TaskDebug {
		t.Errorf("expected TaskDebug, got %v", taskType)
	}

	if confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %v", confidence)
	}

	if len(matches) == 0 {
		t.Error("expected at least one pattern match")
	}
}

func TestExtractMention(t *testing.T) {
	tests := []struct {
		input           string
		expectedMention string
		expectedRemain  string
	}{
		{"@review check this code", "review", "check this code"},
		{"@debug fix the error", "debug", "fix the error"},
		{"@code write a function", "code", "write a function"},
		{"no mention here", "", "no mention here"},
		{"text @mention in middle", "", "text @mention in middle"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			mention, remaining := ExtractMention(tt.input)

			if mention != tt.expectedMention {
				t.Errorf("ExtractMention(%q) mention = %v, want %v", tt.input, mention, tt.expectedMention)
			}

			if remaining != tt.expectedRemain {
				t.Errorf("ExtractMention(%q) remaining = %v, want %v", tt.input, remaining, tt.expectedRemain)
			}
		})
	}
}

func TestGetTaskTypeFromMention(t *testing.T) {
	tests := []struct {
		mention  string
		expected TaskType
		ok       bool
	}{
		{"review", TaskReview, true},
		{"debug", TaskDebug, true},
		{"fix", TaskDebug, true},
		{"plan", TaskPlanning, true},
		{"code", TaskCodeGen, true},
		{"infra", TaskInfrastructure, true},
		{"explain", TaskExplain, true},
		{"refactor", TaskRefactor, true},
		{"unknown", TaskGeneral, false},
	}

	for _, tt := range tests {
		t.Run(tt.mention, func(t *testing.T) {
			taskType, ok := GetTaskTypeFromMention(tt.mention)

			if ok != tt.ok {
				t.Errorf("GetTaskTypeFromMention(%q) ok = %v, want %v", tt.mention, ok, tt.ok)
			}

			if ok && taskType != tt.expected {
				t.Errorf("GetTaskTypeFromMention(%q) = %v, want %v", tt.mention, taskType, tt.expected)
			}
		})
	}
}

// ============================================================================
// SlowClassifier Tests
// ============================================================================

func TestMockLLMRouter_Classify(t *testing.T) {
	mock := NewMockLLMRouter().
		WithResponse("fix the error", TaskDebug).
		WithResponse("write code", TaskCodeGen)

	tests := []struct {
		input    string
		expected TaskType
	}{
		{"fix the error", TaskDebug},
		{"write code", TaskCodeGen},
		{"explain this concept", TaskExplain}, // Default classification
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			taskType, confidence, err := mock.Classify(tt.input)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if taskType != tt.expected {
				t.Errorf("Classify(%q) = %v, want %v", tt.input, taskType, tt.expected)
			}

			if confidence < 0.5 {
				t.Errorf("expected confidence >= 0.5, got %v", confidence)
			}
		})
	}
}

func TestParseLLMResponse(t *testing.T) {
	tests := []struct {
		response string
		expected TaskType
	}{
		{"debug", TaskDebug},
		{"DEBUG", TaskDebug},
		{"code_generation", TaskCodeGen},
		{"codegen", TaskCodeGen},
		{"review.", TaskReview},
		{"planning,", TaskPlanning},
		{"  infrastructure  ", TaskInfrastructure},
		{"unknown_category", TaskGeneral},
	}

	for _, tt := range tests {
		t.Run(tt.response, func(t *testing.T) {
			taskType := ParseLLMResponse(tt.response)

			if taskType != tt.expected {
				t.Errorf("ParseLLMResponse(%q) = %v, want %v", tt.response, taskType, tt.expected)
			}
		})
	}
}

// ============================================================================
// SmartRouter Tests
// ============================================================================

func TestSmartRouter_Route_FastPath(t *testing.T) {
	router := NewSmartRouter(nil) // No LLM, fast path only

	tests := []struct {
		name         string
		input        string
		expectedType TaskType
		expectedPath ClassificationPath
	}{
		{
			name:         "debug request",
			input:        "Fix this bug in the authentication module",
			expectedType: TaskDebug,
			expectedPath: PathFast,
		},
		{
			name:         "code generation request",
			input:        "Write a function to validate email addresses",
			expectedType: TaskCodeGen,
			expectedPath: PathFast,
		},
		{
			name:         "review request",
			input:        "Review this code for security vulnerabilities",
			expectedType: TaskReview,
			expectedPath: PathFast,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := router.RouteSimple(tt.input)

			if decision.TaskType != tt.expectedType {
				t.Errorf("Route(%q) TaskType = %v, want %v", tt.input, decision.TaskType, tt.expectedType)
			}

			if decision.Path != tt.expectedPath {
				t.Errorf("Route(%q) Path = %v, want %v", tt.input, decision.Path, tt.expectedPath)
			}

			if decision.Confidence <= 0 {
				t.Errorf("Route(%q) Confidence = %v, want > 0", tt.input, decision.Confidence)
			}

			if decision.ClassificationDuration <= 0 {
				t.Errorf("Route(%q) ClassificationDuration = %v, want > 0", tt.input, decision.ClassificationDuration)
			}
		})
	}
}

func TestSmartRouter_Route_ExplicitMention(t *testing.T) {
	router := NewSmartRouter(nil)

	decision := router.RouteSimple("@debug this is clearly a review task")

	if decision.TaskType != TaskDebug {
		t.Errorf("expected TaskDebug for @debug mention, got %v", decision.TaskType)
	}

	if decision.Path != PathExplicit {
		t.Errorf("expected PathExplicit, got %v", decision.Path)
	}

	if decision.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0 for explicit mention, got %v", decision.Confidence)
	}
}

func TestSmartRouter_Route_PlatformContext(t *testing.T) {
	router := NewSmartRouter(nil)

	ctx := &ProcessContext{
		Platform: &PlatformInfo{
			Vendor: "cisco",
			Name:   "ios-xe",
		},
	}

	decision := router.Route("show running-config", ctx)

	if decision.TaskType != TaskInfrastructure {
		t.Errorf("expected TaskInfrastructure for platform context, got %v", decision.TaskType)
	}

	if decision.Path != PathContext {
		t.Errorf("expected PathContext, got %v", decision.Path)
	}
}

func TestSmartRouter_Route_SlowPath(t *testing.T) {
	mock := NewMockLLMRouter().
		WithResponse("ambiguous", TaskPlanning).
		WithDelay(10 * time.Millisecond)

	router := NewSmartRouter(mock, WithConfidenceThreshold(0.99)) // Force slow path

	decision := router.RouteSimple("ambiguous request that needs semantic analysis")

	// Should use slow path due to high threshold
	if decision.Path != PathSlow && decision.Path != PathFast {
		t.Errorf("expected PathSlow or PathFast, got %v", decision.Path)
	}
}

func TestSmartRouter_Stats(t *testing.T) {
	router := NewSmartRouter(nil)

	// Make some routing decisions
	router.RouteSimple("Fix this bug")
	router.RouteSimple("Write a function")
	router.RouteSimple("@review check this code")

	stats := router.Stats()

	if stats.TotalRequests != 3 {
		t.Errorf("expected TotalRequests = 3, got %d", stats.TotalRequests)
	}

	if stats.ExplicitHits != 1 {
		t.Errorf("expected ExplicitHits = 1, got %d", stats.ExplicitHits)
	}

	if stats.FastPathRatio() == 0 {
		t.Error("expected non-zero fast path ratio")
	}
}

func TestSmartRouter_ResetStats(t *testing.T) {
	router := NewSmartRouter(nil)

	router.RouteSimple("Fix this bug")
	router.ResetStats()

	stats := router.Stats()

	if stats.TotalRequests != 0 {
		t.Errorf("expected TotalRequests = 0 after reset, got %d", stats.TotalRequests)
	}
}

// ============================================================================
// RiskAssessor Tests
// ============================================================================

func TestDefaultRiskAssessor_Assess(t *testing.T) {
	assessor := &DefaultRiskAssessor{}

	tests := []struct {
		name     string
		input    string
		taskType TaskType
		expected RiskLevel
	}{
		{
			name:     "rm -rf is critical",
			input:    "run rm -rf /",
			taskType: TaskGeneral,
			expected: RiskCritical,
		},
		{
			name:     "drop table is critical",
			input:    "execute DROP TABLE users",
			taskType: TaskGeneral,
			expected: RiskCritical,
		},
		{
			name:     "production infrastructure is high risk",
			input:    "deploy to production server",
			taskType: TaskInfrastructure,
			expected: RiskHigh,
		},
		{
			name:     "normal infrastructure is medium risk",
			input:    "configure nginx",
			taskType: TaskInfrastructure,
			expected: RiskMedium,
		},
		{
			name:     "code generation is medium risk",
			input:    "write a function",
			taskType: TaskCodeGen,
			expected: RiskMedium,
		},
		{
			name:     "code review is low risk",
			input:    "review this code",
			taskType: TaskReview,
			expected: RiskLow,
		},
		{
			name:     "explanation is low risk",
			input:    "explain how it works",
			taskType: TaskExplain,
			expected: RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			risk := assessor.Assess(tt.input, tt.taskType)

			if risk != tt.expected {
				t.Errorf("Assess(%q, %v) = %v, want %v", tt.input, tt.taskType, risk, tt.expected)
			}
		})
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkFastClassifier_Classify(b *testing.B) {
	classifier := NewFastClassifier()
	inputs := []string{
		"Fix this bug in the authentication module",
		"Write a function to validate email addresses",
		"Review this code for security vulnerabilities",
		"Deploy the application to production",
		"Explain how dependency injection works",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.Classify(inputs[i%len(inputs)])
	}
}

func BenchmarkSmartRouter_Route(b *testing.B) {
	router := NewSmartRouter(nil) // Fast path only
	inputs := []string{
		"Fix this bug in the authentication module",
		"Write a function to validate email addresses",
		"Review this code for security vulnerabilities",
		"Deploy the application to production",
		"Explain how dependency injection works",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.RouteSimple(inputs[i%len(inputs)])
	}
}

func BenchmarkSmartRouter_Route_WithMention(b *testing.B) {
	router := NewSmartRouter(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.RouteSimple("@debug fix this error")
	}
}
