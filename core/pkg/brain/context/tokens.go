package context

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// TokenEstimator provides fast token count estimation.
// Uses heuristics rather than actual tokenization for speed.
//
// The goal is O(1) or O(n) estimation that's "good enough" for budget
// enforcement without the overhead of real tokenization.
type TokenEstimator struct {
	// CharsPerToken is the average characters per token.
	// Default: 4 (conservative estimate for English text)
	// Code typically has ~3.5 chars/token, prose ~4.5
	CharsPerToken float64
}

// DefaultTokenEstimator returns an estimator with default settings.
func DefaultTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		CharsPerToken: 4.0,
	}
}

// Estimate returns an estimated token count for the given content.
// This is a fast heuristic, not an exact count.
func (e *TokenEstimator) Estimate(content interface{}) int {
	text := stringify(content)
	if text == "" {
		return 0
	}

	// Count characters (runes for Unicode support)
	charCount := utf8.RuneCountInString(text)

	// Base estimate
	tokens := float64(charCount) / e.CharsPerToken

	// Adjustments for common patterns
	tokens += e.adjustForWhitespace(text)
	tokens += e.adjustForSpecialChars(text)

	// Minimum 1 token for non-empty content
	if tokens < 1 {
		tokens = 1
	}

	return int(tokens)
}

// EstimateString is a convenience method for string content.
func (e *TokenEstimator) EstimateString(s string) int {
	return e.Estimate(s)
}

// adjustForWhitespace adds tokens for whitespace patterns.
// Newlines and multiple spaces often become separate tokens.
func (e *TokenEstimator) adjustForWhitespace(text string) float64 {
	newlines := strings.Count(text, "\n")
	return float64(newlines) * 0.5
}

// adjustForSpecialChars adds tokens for punctuation and symbols.
// These often tokenize separately.
func (e *TokenEstimator) adjustForSpecialChars(text string) float64 {
	specialCount := 0
	for _, r := range text {
		switch r {
		case '{', '}', '[', ']', '(', ')', '<', '>', ':', ';', ',', '.', '!', '?', '"', '\'', '`':
			specialCount++
		}
	}
	return float64(specialCount) * 0.3
}

// stringify converts any content to a string for token estimation.
func stringify(content interface{}) string {
	if content == nil {
		return ""
	}

	switch v := content.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case fmt.Stringer:
		return v.String()
	default:
		// For structs/maps, use fmt which gives a reasonable approximation
		return fmt.Sprintf("%v", v)
	}
}

// EstimateItems returns total token count for a slice of context items.
func (e *TokenEstimator) EstimateItems(items []*ContextItem) int {
	total := 0
	for _, item := range items {
		if item.TokenCount > 0 {
			total += item.TokenCount
		} else {
			total += e.Estimate(item.Content)
		}
	}
	return total
}

// QuickEstimate is a static function for one-off estimates without creating an estimator.
func QuickEstimate(content interface{}) int {
	return DefaultTokenEstimator().Estimate(content)
}

// TokenBudget tracks token usage against a limit.
type TokenBudget struct {
	Limit int
	Used  int
}

// NewTokenBudget creates a budget with the given limit.
func NewTokenBudget(limit int) *TokenBudget {
	return &TokenBudget{
		Limit: limit,
		Used:  0,
	}
}

// Available returns remaining tokens.
func (b *TokenBudget) Available() int {
	return b.Limit - b.Used
}

// CanFit returns true if the given token count fits in the budget.
func (b *TokenBudget) CanFit(tokens int) bool {
	return b.Used+tokens <= b.Limit
}

// Use consumes tokens from the budget. Returns false if over limit.
func (b *TokenBudget) Use(tokens int) bool {
	if !b.CanFit(tokens) {
		return false
	}
	b.Used += tokens
	return true
}

// Release returns tokens to the budget.
func (b *TokenBudget) Release(tokens int) {
	b.Used -= tokens
	if b.Used < 0 {
		b.Used = 0
	}
}

// Utilization returns the percentage of budget used (0.0-1.0).
func (b *TokenBudget) Utilization() float64 {
	if b.Limit == 0 {
		return 0
	}
	return float64(b.Used) / float64(b.Limit)
}

// IsOverBudget returns true if usage exceeds limit.
func (b *TokenBudget) IsOverBudget() bool {
	return b.Used > b.Limit
}
