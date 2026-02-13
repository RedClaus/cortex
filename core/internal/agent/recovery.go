// Package agent provides agentic capabilities for Cortex.
// This file implements intelligent timeout recovery with model fallback.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/logging"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TIMEOUT RECOVERY SYSTEM
// ═══════════════════════════════════════════════════════════════════════════════

// TimeoutError represents a timeout during LLM execution with diagnostic info.
type TimeoutError struct {
	OriginalError   error
	Provider        string
	Model           string
	Endpoint        string
	TaskComplexity  string // "simple", "moderate", "complex"
	ElapsedTime     time.Duration
	HealthCheckDone bool
	HealthStatus    *HealthStatus
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout after %v on %s/%s: %v", e.ElapsedTime, e.Provider, e.Model, e.OriginalError)
}

// HealthStatus represents the health of an LLM endpoint.
type HealthStatus struct {
	Available     bool          `json:"available"`
	ResponseTime  time.Duration `json:"response_time_ms"`
	ModelsLoaded  []string      `json:"models_loaded"`
	Error         string        `json:"error,omitempty"`
	CheckedAt     time.Time     `json:"checked_at"`
	ServerVersion string        `json:"server_version,omitempty"`
}

// RecoveryAction represents what action to take after a timeout.
type RecoveryAction string

const (
	ActionRetry          RecoveryAction = "retry"           // Retry with same model
	ActionFallback       RecoveryAction = "fallback"        // Use frontier model
	ActionSimplify       RecoveryAction = "simplify"        // Break down the task
	ActionAbort          RecoveryAction = "abort"           // Cannot recover
	ActionWaitAndRetry   RecoveryAction = "wait_and_retry"  // Server overloaded, wait
)

// RecoveryDecision contains the recovery strategy and reasoning.
type RecoveryDecision struct {
	Action          RecoveryAction `json:"action"`
	Reason          string         `json:"reason"`
	FallbackModel   string         `json:"fallback_model,omitempty"`
	FallbackProvider string        `json:"fallback_provider,omitempty"`
	WaitDuration    time.Duration  `json:"wait_duration,omitempty"`
	ShouldLearn     bool           `json:"should_learn"`
	LearningNote    string         `json:"learning_note,omitempty"`
}

// RecoveryConfig configures the timeout recovery behavior.
type RecoveryConfig struct {
	// Primary endpoint (e.g., Ollama)
	PrimaryEndpoint string
	PrimaryProvider string
	PrimaryModel    string

	// Fallback providers (ordered by preference)
	FallbackProviders []FallbackProvider

	// Health check settings
	HealthCheckTimeout time.Duration

	// Retry settings
	MaxRetries     int
	RetryDelay     time.Duration
	MaxWaitTime    time.Duration
}

// FallbackProvider represents an alternative LLM provider.
type FallbackProvider struct {
	Name     string // "anthropic", "openai", "gemini"
	Model    string // e.g., "claude-sonnet-4-20250514", "gpt-4o"
	APIKey   string
	Priority int    // Lower = higher priority
}

// DefaultRecoveryConfig returns sensible defaults.
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		HealthCheckTimeout: 5 * time.Second,
		MaxRetries:         2,
		RetryDelay:         5 * time.Second,
		MaxWaitTime:        30 * time.Second,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// HEALTH CHECKER
// ═══════════════════════════════════════════════════════════════════════════════

// HealthChecker checks the health of LLM endpoints.
type HealthChecker struct {
	log    *logging.Logger
	client *http.Client
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		log: logging.Global(),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckOllama checks the health of an Ollama endpoint.
func (h *HealthChecker) CheckOllama(ctx context.Context, endpoint string) *HealthStatus {
	start := time.Now()
	status := &HealthStatus{
		CheckedAt: time.Now(),
	}

	// Check if server is responding
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/tags", nil)
	if err != nil {
		status.Error = fmt.Sprintf("failed to create request: %v", err)
		return status
	}

	resp, err := h.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			status.Error = "server not responding (timeout)"
		} else if strings.Contains(err.Error(), "connection refused") {
			status.Error = "server not running (connection refused)"
		} else {
			status.Error = fmt.Sprintf("connection error: %v", err)
		}
		return status
	}
	defer resp.Body.Close()

	status.ResponseTime = time.Since(start)
	status.Available = resp.StatusCode == http.StatusOK

	if !status.Available {
		status.Error = fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
		return status
	}

	// Parse models list
	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err == nil {
		for _, m := range tagsResp.Models {
			status.ModelsLoaded = append(status.ModelsLoaded, m.Name)
		}
	}

	// Check running models (ps endpoint)
	psReq, _ := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/ps", nil)
	if psResp, err := h.client.Do(psReq); err == nil {
		defer psResp.Body.Close()
		var psData struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(psResp.Body).Decode(&psData) == nil {
			h.log.Debug("[HealthCheck] Running models: %v", psData.Models)
		}
	}

	h.log.Info("[HealthCheck] Ollama at %s: available=%v, response_time=%v, models=%d",
		endpoint, status.Available, status.ResponseTime, len(status.ModelsLoaded))

	return status
}

// CheckDnet checks the health of a dnet endpoint (OpenAI-compatible API).
func (h *HealthChecker) CheckDnet(ctx context.Context, endpoint string) *HealthStatus {
	start := time.Now()
	status := &HealthStatus{
		CheckedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// dnet uses OpenAI-compatible /v1/models endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/v1/models", nil)
	if err != nil {
		status.Error = fmt.Sprintf("failed to create request: %v", err)
		return status
	}

	resp, err := h.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			status.Error = "server not responding (timeout)"
		} else if strings.Contains(err.Error(), "connection refused") {
			status.Error = "server not running (connection refused)"
		} else {
			status.Error = fmt.Sprintf("connection error: %v", err)
		}
		return status
	}
	defer resp.Body.Close()

	status.ResponseTime = time.Since(start)
	status.Available = resp.StatusCode == http.StatusOK

	if !status.Available {
		status.Error = fmt.Sprintf("unexpected status code: %d", resp.StatusCode)
		return status
	}

	// Parse models list (OpenAI format)
	var modelsResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err == nil {
		for _, m := range modelsResp.Data {
			status.ModelsLoaded = append(status.ModelsLoaded, m.ID)
		}
	}

	h.log.Info("[HealthCheck] dnet at %s: available=%v, response_time=%v, models=%d",
		endpoint, status.Available, status.ResponseTime, len(status.ModelsLoaded))

	return status
}

// CheckEndpoint checks health based on provider type.
func (h *HealthChecker) CheckEndpoint(ctx context.Context, endpoint, provider string) *HealthStatus {
	switch provider {
	case "dnet":
		return h.CheckDnet(ctx, endpoint)
	case "ollama":
		return h.CheckOllama(ctx, endpoint)
	default:
		// Fallback to generic HTTP check
		return h.checkGeneric(ctx, endpoint)
	}
}

// checkGeneric performs a simple HTTP GET to check if endpoint is responding.
func (h *HealthChecker) checkGeneric(ctx context.Context, endpoint string) *HealthStatus {
	start := time.Now()
	status := &HealthStatus{
		CheckedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		status.Error = fmt.Sprintf("failed to create request: %v", err)
		return status
	}

	resp, err := h.client.Do(req)
	if err != nil {
		status.Error = fmt.Sprintf("connection error: %v", err)
		return status
	}
	defer resp.Body.Close()

	status.ResponseTime = time.Since(start)
	status.Available = resp.StatusCode >= 200 && resp.StatusCode < 400

	return status
}

// ═══════════════════════════════════════════════════════════════════════════════
// RECOVERY ANALYZER
// ═══════════════════════════════════════════════════════════════════════════════

// RecoveryAnalyzer analyzes timeouts and decides recovery strategy.
type RecoveryAnalyzer struct {
	config  *RecoveryConfig
	checker *HealthChecker
	log     *logging.Logger
}

// NewRecoveryAnalyzer creates a new recovery analyzer.
func NewRecoveryAnalyzer(config *RecoveryConfig) *RecoveryAnalyzer {
	if config == nil {
		config = DefaultRecoveryConfig()
	}
	return &RecoveryAnalyzer{
		config:  config,
		checker: NewHealthChecker(),
		log:     logging.Global(),
	}
}

// AnalyzeTimeout analyzes a timeout error and returns a recovery decision.
func (r *RecoveryAnalyzer) AnalyzeTimeout(ctx context.Context, err error, taskContext *TaskContext) *RecoveryDecision {
	r.log.Info("[Recovery] Analyzing timeout: %v", err)

	// Step 1: Check primary endpoint health (use provider-aware check)
	health := r.checker.CheckEndpoint(ctx, r.config.PrimaryEndpoint, r.config.PrimaryProvider)

	// Step 2: Assess task complexity
	complexity := r.assessComplexity(taskContext)

	// Step 3: Determine recovery action
	decision := r.decide(health, complexity, taskContext)

	r.log.Info("[Recovery] Decision: action=%s, reason=%s", decision.Action, decision.Reason)

	return decision
}

// TaskContext provides context about the task that timed out.
type TaskContext struct {
	Task             string        // Original user task
	StepsCompleted   int           // Steps completed before timeout
	ToolsUsed        []string      // Tools used so far
	ElapsedTime      time.Duration // Total time elapsed
	ConversationSize int           // Number of messages in context
	LastToolOutput   string        // Output of last tool (may indicate complexity)
}

// assessComplexity estimates task complexity.
func (r *RecoveryAnalyzer) assessComplexity(tc *TaskContext) string {
	if tc == nil {
		return "unknown"
	}

	// Heuristics for complexity assessment
	score := 0

	// More steps = more complex
	if tc.StepsCompleted > 5 {
		score += 2
	} else if tc.StepsCompleted > 2 {
		score += 1
	}

	// Large conversation context = complex
	if tc.ConversationSize > 20 {
		score += 2
	} else if tc.ConversationSize > 10 {
		score += 1
	}

	// Multiple tool types = complex
	uniqueTools := make(map[string]bool)
	for _, t := range tc.ToolsUsed {
		uniqueTools[t] = true
	}
	if len(uniqueTools) > 3 {
		score += 2
	} else if len(uniqueTools) > 1 {
		score += 1
	}

	// Long task description = likely complex
	if len(tc.Task) > 200 {
		score += 1
	}

	// Classify
	switch {
	case score >= 5:
		return "complex"
	case score >= 2:
		return "moderate"
	default:
		return "simple"
	}
}

// decide makes the recovery decision based on health and complexity.
func (r *RecoveryAnalyzer) decide(health *HealthStatus, complexity string, tc *TaskContext) *RecoveryDecision {
	decision := &RecoveryDecision{
		ShouldLearn: true, // Default: learn from this
	}

	// Case 1: Server is down
	if !health.Available {
		if strings.Contains(health.Error, "connection refused") {
			decision.Action = ActionFallback
			decision.Reason = "Ollama server is not running"
			decision.LearningNote = "Primary LLM server unavailable - used frontier fallback"
		} else if strings.Contains(health.Error, "timeout") {
			decision.Action = ActionFallback
			decision.Reason = "Ollama server not responding (network issue or overloaded)"
			decision.LearningNote = "Network timeout to primary LLM - consider server health"
		} else {
			decision.Action = ActionFallback
			decision.Reason = fmt.Sprintf("Primary LLM health check failed: %s", health.Error)
			decision.LearningNote = "Primary LLM failed health check"
		}

		// Set fallback to frontier model
		r.setFallbackProvider(decision)
		return decision
	}

	// Case 2: Server is healthy but timed out - likely task complexity
	switch complexity {
	case "complex":
		decision.Action = ActionFallback
		decision.Reason = "Complex task exceeded local model capabilities"
		decision.LearningNote = fmt.Sprintf("Task '%s' too complex for local model - needed frontier", truncate(tc.Task, 50))
		r.setFallbackProvider(decision)

	case "moderate":
		// Server healthy + moderate complexity = could be model loading or temporary issue
		if health.ResponseTime > 2*time.Second {
			// Server is slow, might be loading model
			decision.Action = ActionWaitAndRetry
			decision.Reason = "Server responding slowly, may be loading model"
			decision.WaitDuration = 10 * time.Second
			decision.ShouldLearn = false // Don't learn from temporary issues
		} else {
			// Server fast but task timed out - use fallback
			decision.Action = ActionFallback
			decision.Reason = "Task complexity exceeded timeout threshold"
			decision.LearningNote = "Moderate complexity task timed out - frontier model needed"
			r.setFallbackProvider(decision)
		}

	case "simple":
		// Simple task + healthy server + timeout = something wrong with request
		decision.Action = ActionRetry
		decision.Reason = "Simple task timed out unexpectedly, retrying"
		decision.ShouldLearn = false

	default:
		decision.Action = ActionFallback
		decision.Reason = "Unable to assess complexity, using frontier model"
		r.setFallbackProvider(decision)
	}

	return decision
}

// setFallbackProvider sets the frontier model for fallback.
func (r *RecoveryAnalyzer) setFallbackProvider(decision *RecoveryDecision) {
	// Priority: Anthropic > OpenAI > Gemini (for agentic tasks)
	for _, fb := range r.config.FallbackProviders {
		if fb.APIKey != "" {
			decision.FallbackProvider = fb.Name
			decision.FallbackModel = fb.Model
			return
		}
	}

	// Default fallback if no providers configured
	decision.FallbackProvider = "anthropic"
	decision.FallbackModel = "claude-sonnet-4-20250514"
}

// ═══════════════════════════════════════════════════════════════════════════════
// LEARNING RECORDER
// ═══════════════════════════════════════════════════════════════════════════════

// TimeoutLearning records timeout events for learning.
type TimeoutLearning struct {
	Timestamp       time.Time      `json:"timestamp"`
	Task            string         `json:"task"`
	PrimaryModel    string         `json:"primary_model"`
	Complexity      string         `json:"complexity"`
	StepsCompleted  int            `json:"steps_completed"`
	TimeoutAfter    time.Duration  `json:"timeout_after"`
	RecoveryAction  RecoveryAction `json:"recovery_action"`
	FallbackUsed    string         `json:"fallback_used,omitempty"`
	FallbackSuccess bool           `json:"fallback_success,omitempty"`
	LearningNote    string         `json:"learning_note"`
}

// LearningCallback is called to record timeout learning.
type LearningCallback func(learning *TimeoutLearning)
