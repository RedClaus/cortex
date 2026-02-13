package router

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultSemanticTimeout is the maximum time allowed for semantic classification.
	DefaultSemanticTimeout = 200 * time.Millisecond

	// ClassificationPrompt is the system prompt for the classifier LLM.
	ClassificationPrompt = `You are a request classifier. Classify the user's request into exactly ONE category.

Categories:
- general: General questions, chat, or unclear requests
- code_generation: Writing new code, creating files, implementing features
- debug: Fixing errors, bugs, crashes, or unexpected behavior
- review: Reviewing code, providing feedback, auditing
- planning: Designing architecture, discussing approaches, strategy
- infrastructure: DevOps, servers, networking, cloud, deployment
- explain: Explaining concepts, code, or documentation
- refactor: Improving existing code structure without changing behavior

Respond with ONLY the category name, nothing else.`
)

// SlowClassifier implements LLM-based semantic classification.
// It's used when the fast classifier is uncertain (confidence < threshold).
type SlowClassifier struct {
	llm     LLMRouter
	timeout time.Duration
}

// NewSlowClassifier creates a new semantic classifier.
func NewSlowClassifier(llm LLMRouter) *SlowClassifier {
	return &SlowClassifier{
		llm:     llm,
		timeout: DefaultSemanticTimeout,
	}
}

// NewSlowClassifierWithTimeout creates a semantic classifier with custom timeout.
func NewSlowClassifierWithTimeout(llm LLMRouter, timeout time.Duration) *SlowClassifier {
	return &SlowClassifier{
		llm:     llm,
		timeout: timeout,
	}
}

// Classify uses an LLM to semantically classify the input.
// Returns TaskType, confidence (always 0.85 for LLM classification), and error.
func (c *SlowClassifier) Classify(input string) (TaskType, float64, error) {
	if c.llm == nil {
		return TaskGeneral, 0.5, fmt.Errorf("LLM router not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Use the LLM interface
	taskType, confidence, err := c.classifyWithContext(ctx, input)
	if err != nil {
		return TaskGeneral, 0.5, err
	}

	return taskType, confidence, nil
}

// classifyWithContext performs the actual LLM classification.
func (c *SlowClassifier) classifyWithContext(ctx context.Context, input string) (TaskType, float64, error) {
	// Call the LLM router
	taskType, confidence, err := c.llm.Classify(input)
	if err != nil {
		return TaskGeneral, 0.5, err
	}

	return taskType, confidence, nil
}

// ParseLLMResponse converts an LLM text response to a TaskType.
// This is useful when implementing the LLMRouter interface.
func ParseLLMResponse(response string) TaskType {
	// Clean and normalize the response
	category := strings.TrimSpace(strings.ToLower(response))

	// Remove any punctuation or extra text
	category = strings.TrimSuffix(category, ".")
	category = strings.TrimSuffix(category, ",")

	// Map to TaskType
	switch category {
	case "code_generation", "codegen", "code generation", "write", "create":
		return TaskCodeGen
	case "debug", "debugging", "fix", "error":
		return TaskDebug
	case "review", "code review", "audit":
		return TaskReview
	case "planning", "plan", "design", "architecture":
		return TaskPlanning
	case "infrastructure", "infra", "devops", "ops":
		return TaskInfrastructure
	case "explain", "explanation", "describe":
		return TaskExplain
	case "refactor", "refactoring", "cleanup":
		return TaskRefactor
	default:
		return TaskGeneral
	}
}

// MockLLMRouter is a mock implementation for testing.
type MockLLMRouter struct {
	responses map[string]TaskType
	delay     time.Duration
}

// NewMockLLMRouter creates a mock LLM router for testing.
func NewMockLLMRouter() *MockLLMRouter {
	return &MockLLMRouter{
		responses: make(map[string]TaskType),
		delay:     10 * time.Millisecond,
	}
}

// WithResponse adds a response mapping for testing.
func (m *MockLLMRouter) WithResponse(input string, taskType TaskType) *MockLLMRouter {
	m.responses[strings.ToLower(input)] = taskType
	return m
}

// WithDelay sets a simulated delay for testing timeout behavior.
func (m *MockLLMRouter) WithDelay(delay time.Duration) *MockLLMRouter {
	m.delay = delay
	return m
}

// Classify implements LLMRouter for testing.
func (m *MockLLMRouter) Classify(input string) (TaskType, float64, error) {
	// Simulate LLM delay
	time.Sleep(m.delay)

	// Check for exact match
	if taskType, ok := m.responses[strings.ToLower(input)]; ok {
		return taskType, 0.85, nil
	}

	// Check for partial match
	lower := strings.ToLower(input)
	for key, taskType := range m.responses {
		if strings.Contains(lower, key) {
			return taskType, 0.85, nil
		}
	}

	// Default classification based on keywords
	return m.defaultClassify(input), 0.75, nil
}

// defaultClassify provides basic keyword-based fallback for the mock.
func (m *MockLLMRouter) defaultClassify(input string) TaskType {
	lower := strings.ToLower(input)

	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "bug") || strings.Contains(lower, "fix"):
		return TaskDebug
	case strings.Contains(lower, "write") || strings.Contains(lower, "create") || strings.Contains(lower, "generate"):
		return TaskCodeGen
	case strings.Contains(lower, "review") || strings.Contains(lower, "check"):
		return TaskReview
	case strings.Contains(lower, "plan") || strings.Contains(lower, "design"):
		return TaskPlanning
	case strings.Contains(lower, "deploy") || strings.Contains(lower, "server") || strings.Contains(lower, "docker"):
		return TaskInfrastructure
	case strings.Contains(lower, "explain") || strings.Contains(lower, "what is"):
		return TaskExplain
	case strings.Contains(lower, "refactor") || strings.Contains(lower, "improve"):
		return TaskRefactor
	default:
		return TaskGeneral
	}
}

// OllamaLLMRouter implements LLMRouter using Ollama for local LLM inference.
// This is a placeholder - actual implementation would use the Ollama API.
type OllamaLLMRouter struct {
	model    string
	endpoint string
	timeout  time.Duration
}

// NewOllamaLLMRouter creates a new Ollama-based LLM router.
func NewOllamaLLMRouter(model string, endpoint string) *OllamaLLMRouter {
	if model == "" {
		model = "llama3.2:1b" // Small, fast model for classification
	}
	if endpoint == "" {
		endpoint = "http://127.0.0.1:11434"
	}
	return &OllamaLLMRouter{
		model:    model,
		endpoint: endpoint,
		timeout:  DefaultSemanticTimeout,
	}
}

// Classify implements LLMRouter using Ollama.
// Note: This is a stub - real implementation would make HTTP request to Ollama.
func (o *OllamaLLMRouter) Classify(input string) (TaskType, float64, error) {
	// TODO: Implement actual Ollama API call
	// For now, return a basic classification
	//
	// Real implementation would:
	// 1. POST to o.endpoint/api/generate
	// 2. Send ClassificationPrompt + input
	// 3. Parse response and call ParseLLMResponse

	// Placeholder: use mock classification
	mock := NewMockLLMRouter()
	return mock.Classify(input)
}
