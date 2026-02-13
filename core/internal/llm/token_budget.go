// Package llm provides Language Model provider implementations for Cortex.
// token_budget.go tracks token consumption across time periods for cost control.
package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TokenBudget tracks and enforces token consumption limits.
// Brain Alignment: Like the brain's energy budget - preventing metabolic exhaustion.
type TokenBudget struct {
	mu sync.RWMutex

	// Configuration
	config BudgetConfig

	// Current state
	state BudgetState

	// Persistence path
	statePath string

	// Alert callbacks
	alertHandlers []BudgetAlertHandler
}

// BudgetConfig defines token budget limits.
type BudgetConfig struct {
	// Daily limits
	DailyTokenLimit   int64 `yaml:"daily_token_limit" json:"daily_token_limit"`
	DailyDollarLimit  float64 `yaml:"daily_dollar_limit" json:"daily_dollar_limit"`

	// Monthly limits
	MonthlyTokenLimit  int64 `yaml:"monthly_token_limit" json:"monthly_token_limit"`
	MonthlyDollarLimit float64 `yaml:"monthly_dollar_limit" json:"monthly_dollar_limit"`

	// Per-request limits
	MaxTokensPerRequest int `yaml:"max_tokens_per_request" json:"max_tokens_per_request"`

	// Alert thresholds (0.0-1.0)
	WarnThreshold     float64 `yaml:"warn_threshold" json:"warn_threshold"`       // Alert at 80%
	CriticalThreshold float64 `yaml:"critical_threshold" json:"critical_threshold"` // Alert at 95%

	// Per-provider costs (dollars per 1M tokens)
	ProviderCosts map[string]ProviderCost `yaml:"provider_costs" json:"provider_costs"`
}

// ProviderCost defines token pricing for a provider.
type ProviderCost struct {
	InputPer1M  float64 `yaml:"input_per_1m" json:"input_per_1m"`
	OutputPer1M float64 `yaml:"output_per_1m" json:"output_per_1m"`
}

// BudgetState tracks current consumption.
type BudgetState struct {
	// Daily tracking
	DayStart      time.Time `json:"day_start"`
	DailyTokens   int64     `json:"daily_tokens"`
	DailyInputs   int64     `json:"daily_inputs"`
	DailyOutputs  int64     `json:"daily_outputs"`
	DailyCost     float64   `json:"daily_cost"`
	DailyRequests int64     `json:"daily_requests"`

	// Monthly tracking
	MonthStart      time.Time `json:"month_start"`
	MonthlyTokens   int64     `json:"monthly_tokens"`
	MonthlyInputs   int64     `json:"monthly_inputs"`
	MonthlyOutputs  int64     `json:"monthly_outputs"`
	MonthlyCost     float64   `json:"monthly_cost"`
	MonthlyRequests int64     `json:"monthly_requests"`

	// Per-provider tracking
	ProviderUsage map[string]*ProviderUsage `json:"provider_usage"`

	// Lifetime stats
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	TotalRequests int64   `json:"total_requests"`
}

// ProviderUsage tracks usage per provider.
type ProviderUsage struct {
	DailyTokens    int64   `json:"daily_tokens"`
	DailyCost      float64 `json:"daily_cost"`
	MonthlyTokens  int64   `json:"monthly_tokens"`
	MonthlyCost    float64 `json:"monthly_cost"`
	TotalTokens    int64   `json:"total_tokens"`
	TotalCost      float64 `json:"total_cost"`
	LastUsed       time.Time `json:"last_used"`
}

// BudgetAlertHandler is called when budget thresholds are crossed.
type BudgetAlertHandler func(alert BudgetAlert)

// BudgetAlert contains alert information.
type BudgetAlert struct {
	Level       AlertLevel `json:"level"`
	Message     string     `json:"message"`
	Percentage  float64    `json:"percentage"`
	Period      string     `json:"period"` // "daily" or "monthly"
	UsedTokens  int64      `json:"used_tokens"`
	LimitTokens int64      `json:"limit_tokens"`
	UsedCost    float64    `json:"used_cost"`
	LimitCost   float64    `json:"limit_cost"`
}

// AlertLevel indicates severity of budget alert.
type AlertLevel int

const (
	AlertInfo AlertLevel = iota
	AlertWarn
	AlertCritical
	AlertExceeded
)

func (l AlertLevel) String() string {
	switch l {
	case AlertInfo:
		return "INFO"
	case AlertWarn:
		return "WARN"
	case AlertCritical:
		return "CRITICAL"
	case AlertExceeded:
		return "EXCEEDED"
	default:
		return "UNKNOWN"
	}
}

// DefaultBudgetConfig returns sensible defaults for token budgeting.
func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		DailyTokenLimit:     1000000,  // 1M tokens/day
		DailyDollarLimit:    10.0,     // $10/day
		MonthlyTokenLimit:   20000000, // 20M tokens/month
		MonthlyDollarLimit:  100.0,    // $100/month
		MaxTokensPerRequest: 100000,   // 100K per request
		WarnThreshold:       0.80,     // Warn at 80%
		CriticalThreshold:   0.95,     // Critical at 95%
		ProviderCosts: map[string]ProviderCost{
			"openai": {
				InputPer1M:  0.15, // gpt-4o-mini input
				OutputPer1M: 0.60, // gpt-4o-mini output
			},
			"anthropic": {
				InputPer1M:  3.00,  // claude-3-5-sonnet input
				OutputPer1M: 15.00, // claude-3-5-sonnet output
			},
			"gemini": {
				InputPer1M:  0.075, // gemini-1.5-flash input
				OutputPer1M: 0.30,  // gemini-1.5-flash output
			},
			"grok": {
				InputPer1M:  5.00,
				OutputPer1M: 15.00,
			},
			"groq": {
				InputPer1M:  0.05, // Very cheap
				OutputPer1M: 0.10,
			},
			"ollama": {
				InputPer1M:  0.0, // Local, no cost
				OutputPer1M: 0.0,
			},
		},
	}
}

// NewTokenBudget creates a new token budget tracker.
func NewTokenBudget(config BudgetConfig, statePath string) *TokenBudget {
	tb := &TokenBudget{
		config:    config,
		statePath: statePath,
		state: BudgetState{
			DayStart:      startOfDay(time.Now()),
			MonthStart:    startOfMonth(time.Now()),
			ProviderUsage: make(map[string]*ProviderUsage),
		},
		alertHandlers: make([]BudgetAlertHandler, 0),
	}

	// Load persisted state if exists
	tb.loadState()

	return tb
}

// OnAlert registers a callback for budget alerts.
func (tb *TokenBudget) OnAlert(handler BudgetAlertHandler) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.alertHandlers = append(tb.alertHandlers, handler)
}

// CanSpend checks if spending the given tokens would exceed limits.
func (tb *TokenBudget) CanSpend(provider string, inputTokens, outputTokens int) (bool, string) {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	tb.maybeResetPeriods()

	totalTokens := int64(inputTokens + outputTokens)
	cost := tb.calculateCost(provider, inputTokens, outputTokens)

	// Check per-request limit
	if tb.config.MaxTokensPerRequest > 0 && int(totalTokens) > tb.config.MaxTokensPerRequest {
		return false, fmt.Sprintf("request exceeds max tokens per request (%d > %d)",
			totalTokens, tb.config.MaxTokensPerRequest)
	}

	// Check daily token limit
	if tb.config.DailyTokenLimit > 0 {
		if tb.state.DailyTokens+totalTokens > tb.config.DailyTokenLimit {
			return false, fmt.Sprintf("would exceed daily token limit (%d + %d > %d)",
				tb.state.DailyTokens, totalTokens, tb.config.DailyTokenLimit)
		}
	}

	// Check daily dollar limit
	if tb.config.DailyDollarLimit > 0 {
		if tb.state.DailyCost+cost > tb.config.DailyDollarLimit {
			return false, fmt.Sprintf("would exceed daily cost limit ($%.2f + $%.4f > $%.2f)",
				tb.state.DailyCost, cost, tb.config.DailyDollarLimit)
		}
	}

	// Check monthly token limit
	if tb.config.MonthlyTokenLimit > 0 {
		if tb.state.MonthlyTokens+totalTokens > tb.config.MonthlyTokenLimit {
			return false, fmt.Sprintf("would exceed monthly token limit (%d + %d > %d)",
				tb.state.MonthlyTokens, totalTokens, tb.config.MonthlyTokenLimit)
		}
	}

	// Check monthly dollar limit
	if tb.config.MonthlyDollarLimit > 0 {
		if tb.state.MonthlyCost+cost > tb.config.MonthlyDollarLimit {
			return false, fmt.Sprintf("would exceed monthly cost limit ($%.2f + $%.4f > $%.2f)",
				tb.state.MonthlyCost, cost, tb.config.MonthlyDollarLimit)
		}
	}

	return true, ""
}

// Spend records token consumption.
func (tb *TokenBudget) Spend(provider string, inputTokens, outputTokens int) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.maybeResetPeriods()

	totalTokens := int64(inputTokens + outputTokens)
	cost := tb.calculateCost(provider, inputTokens, outputTokens)

	// Update daily stats
	tb.state.DailyTokens += totalTokens
	tb.state.DailyInputs += int64(inputTokens)
	tb.state.DailyOutputs += int64(outputTokens)
	tb.state.DailyCost += cost
	tb.state.DailyRequests++

	// Update monthly stats
	tb.state.MonthlyTokens += totalTokens
	tb.state.MonthlyInputs += int64(inputTokens)
	tb.state.MonthlyOutputs += int64(outputTokens)
	tb.state.MonthlyCost += cost
	tb.state.MonthlyRequests++

	// Update lifetime stats
	tb.state.TotalTokens += totalTokens
	tb.state.TotalCost += cost
	tb.state.TotalRequests++

	// Update provider-specific stats
	pu, exists := tb.state.ProviderUsage[provider]
	if !exists {
		pu = &ProviderUsage{}
		tb.state.ProviderUsage[provider] = pu
	}
	pu.DailyTokens += totalTokens
	pu.DailyCost += cost
	pu.MonthlyTokens += totalTokens
	pu.MonthlyCost += cost
	pu.TotalTokens += totalTokens
	pu.TotalCost += cost
	pu.LastUsed = time.Now()

	// Check thresholds and send alerts
	tb.checkAlerts()

	// Persist state
	tb.saveState()
}

// GetState returns a copy of the current budget state.
func (tb *TokenBudget) GetState() BudgetState {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	tb.maybeResetPeriods()

	// Deep copy
	state := tb.state
	state.ProviderUsage = make(map[string]*ProviderUsage)
	for k, v := range tb.state.ProviderUsage {
		copy := *v
		state.ProviderUsage[k] = &copy
	}
	return state
}

// Remaining returns remaining tokens and cost for the day/month.
func (tb *TokenBudget) Remaining() (dailyTokens, monthlyTokens int64, dailyCost, monthlyCost float64) {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	tb.maybeResetPeriods()

	dailyTokens = tb.config.DailyTokenLimit - tb.state.DailyTokens
	if dailyTokens < 0 {
		dailyTokens = 0
	}

	monthlyTokens = tb.config.MonthlyTokenLimit - tb.state.MonthlyTokens
	if monthlyTokens < 0 {
		monthlyTokens = 0
	}

	dailyCost = tb.config.DailyDollarLimit - tb.state.DailyCost
	if dailyCost < 0 {
		dailyCost = 0
	}

	monthlyCost = tb.config.MonthlyDollarLimit - tb.state.MonthlyCost
	if monthlyCost < 0 {
		monthlyCost = 0
	}

	return
}

// UsagePercentage returns current usage as percentage of limits.
func (tb *TokenBudget) UsagePercentage() (dailyToken, monthlyToken, dailyCost, monthlyCost float64) {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	tb.maybeResetPeriods()

	if tb.config.DailyTokenLimit > 0 {
		dailyToken = float64(tb.state.DailyTokens) / float64(tb.config.DailyTokenLimit)
	}
	if tb.config.MonthlyTokenLimit > 0 {
		monthlyToken = float64(tb.state.MonthlyTokens) / float64(tb.config.MonthlyTokenLimit)
	}
	if tb.config.DailyDollarLimit > 0 {
		dailyCost = tb.state.DailyCost / tb.config.DailyDollarLimit
	}
	if tb.config.MonthlyDollarLimit > 0 {
		monthlyCost = tb.state.MonthlyCost / tb.config.MonthlyDollarLimit
	}

	return
}

// calculateCost computes the cost for given tokens.
func (tb *TokenBudget) calculateCost(provider string, inputTokens, outputTokens int) float64 {
	costs, exists := tb.config.ProviderCosts[provider]
	if !exists {
		// Default cost estimation
		costs = ProviderCost{InputPer1M: 1.0, OutputPer1M: 2.0}
	}

	inputCost := float64(inputTokens) / 1000000.0 * costs.InputPer1M
	outputCost := float64(outputTokens) / 1000000.0 * costs.OutputPer1M

	return inputCost + outputCost
}

// maybeResetPeriods resets counters if day/month has changed.
func (tb *TokenBudget) maybeResetPeriods() {
	now := time.Now()
	todayStart := startOfDay(now)
	monthStart := startOfMonth(now)

	// Reset daily if new day
	if todayStart.After(tb.state.DayStart) {
		tb.state.DayStart = todayStart
		tb.state.DailyTokens = 0
		tb.state.DailyInputs = 0
		tb.state.DailyOutputs = 0
		tb.state.DailyCost = 0
		tb.state.DailyRequests = 0

		for _, pu := range tb.state.ProviderUsage {
			pu.DailyTokens = 0
			pu.DailyCost = 0
		}
	}

	// Reset monthly if new month
	if monthStart.After(tb.state.MonthStart) {
		tb.state.MonthStart = monthStart
		tb.state.MonthlyTokens = 0
		tb.state.MonthlyInputs = 0
		tb.state.MonthlyOutputs = 0
		tb.state.MonthlyCost = 0
		tb.state.MonthlyRequests = 0

		for _, pu := range tb.state.ProviderUsage {
			pu.MonthlyTokens = 0
			pu.MonthlyCost = 0
		}
	}
}

// checkAlerts checks thresholds and fires alerts.
func (tb *TokenBudget) checkAlerts() {
	if len(tb.alertHandlers) == 0 {
		return
	}

	// Check daily token usage
	if tb.config.DailyTokenLimit > 0 {
		pct := float64(tb.state.DailyTokens) / float64(tb.config.DailyTokenLimit)
		tb.maybeAlert(pct, "daily", "tokens", tb.state.DailyTokens, tb.config.DailyTokenLimit, 0, 0)
	}

	// Check daily cost
	if tb.config.DailyDollarLimit > 0 {
		pct := tb.state.DailyCost / tb.config.DailyDollarLimit
		tb.maybeAlert(pct, "daily", "cost", 0, 0, tb.state.DailyCost, tb.config.DailyDollarLimit)
	}

	// Check monthly token usage
	if tb.config.MonthlyTokenLimit > 0 {
		pct := float64(tb.state.MonthlyTokens) / float64(tb.config.MonthlyTokenLimit)
		tb.maybeAlert(pct, "monthly", "tokens", tb.state.MonthlyTokens, tb.config.MonthlyTokenLimit, 0, 0)
	}

	// Check monthly cost
	if tb.config.MonthlyDollarLimit > 0 {
		pct := tb.state.MonthlyCost / tb.config.MonthlyDollarLimit
		tb.maybeAlert(pct, "monthly", "cost", 0, 0, tb.state.MonthlyCost, tb.config.MonthlyDollarLimit)
	}
}

// maybeAlert fires an alert if threshold is crossed.
func (tb *TokenBudget) maybeAlert(pct float64, period, metric string, usedTokens, limitTokens int64, usedCost, limitCost float64) {
	var level AlertLevel
	if pct >= 1.0 {
		level = AlertExceeded
	} else if pct >= tb.config.CriticalThreshold {
		level = AlertCritical
	} else if pct >= tb.config.WarnThreshold {
		level = AlertWarn
	} else {
		return // No alert needed
	}

	alert := BudgetAlert{
		Level:       level,
		Percentage:  pct * 100,
		Period:      period,
		UsedTokens:  usedTokens,
		LimitTokens: limitTokens,
		UsedCost:    usedCost,
		LimitCost:   limitCost,
	}

	if metric == "tokens" {
		alert.Message = fmt.Sprintf("%s %s token budget at %.1f%% (%d/%d)",
			level.String(), period, pct*100, usedTokens, limitTokens)
	} else {
		alert.Message = fmt.Sprintf("%s %s cost budget at %.1f%% ($%.2f/$%.2f)",
			level.String(), period, pct*100, usedCost, limitCost)
	}

	for _, handler := range tb.alertHandlers {
		go handler(alert)
	}
}

// saveState persists budget state to disk.
func (tb *TokenBudget) saveState() {
	if tb.statePath == "" {
		return
	}

	data, err := json.MarshalIndent(tb.state, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(tb.statePath)
	os.MkdirAll(dir, 0755)
	os.WriteFile(tb.statePath, data, 0644)
}

// loadState loads budget state from disk.
func (tb *TokenBudget) loadState() {
	if tb.statePath == "" {
		return
	}

	data, err := os.ReadFile(tb.statePath)
	if err != nil {
		return
	}

	var state BudgetState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	// Ensure map is initialized
	if state.ProviderUsage == nil {
		state.ProviderUsage = make(map[string]*ProviderUsage)
	}

	tb.state = state
}

// ─────────────────────────────────────────────────────────────────────────────
// UTILITY FUNCTIONS
// ─────────────────────────────────────────────────────────────────────────────

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}
