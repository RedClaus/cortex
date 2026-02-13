package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CAPABILITY ASSESSOR
// ═══════════════════════════════════════════════════════════════════════════════

// CapabilityAssessor evaluates LLM response quality and detects issues.
type CapabilityAssessor struct {
	// Configuration
	timeoutThresholdMs int     // Response time threshold (default: 60000ms)
	repetitionMinCount int     // Minimum repetitions to flag (default: 3)
	trigramThreshold   float64 // Trigram repetition rate threshold (default: 0.3)
}

// NewCapabilityAssessor creates a new assessor with default thresholds.
func NewCapabilityAssessor() *CapabilityAssessor {
	return &CapabilityAssessor{
		// CR-009: Increased from 30s to 60s to reduce false positives for
		// complex tasks that naturally take longer to process
		timeoutThresholdMs: 60000, // 60 seconds
		repetitionMinCount: 3,
		trigramThreshold:   0.30, // 30% repetition
	}
}

// WithTimeoutThreshold sets a custom timeout threshold in milliseconds.
func (a *CapabilityAssessor) WithTimeoutThreshold(ms int) *CapabilityAssessor {
	a.timeoutThresholdMs = ms
	return a
}

// WithRepetitionMinCount sets the minimum count for repetition detection.
func (a *CapabilityAssessor) WithRepetitionMinCount(count int) *CapabilityAssessor {
	a.repetitionMinCount = count
	return a
}

// ═══════════════════════════════════════════════════════════════════════════════
// ASSESSMENT
// ═══════════════════════════════════════════════════════════════════════════════

// Assess evaluates a conversation log and returns capability assessment.
func (a *CapabilityAssessor) Assess(ctx context.Context, log *ConversationLog) *Assessment {
	assessment := &Assessment{
		CapabilityScore: 100.0, // Start at perfect score
		Issues:          []Issue{},
		Confidence:      0.8, // Default confidence
	}

	// Run all detectors
	a.detectTimeout(log, assessment)
	a.detectRepetition(log, assessment)
	a.detectToolFailure(log, assessment)
	a.detectJSONError(log, assessment)
	a.detectTruncation(log, assessment)

	// Calculate final score based on issues
	assessment.CapabilityScore = a.calculateScore(assessment.Issues)

	// Determine confidence based on response length and complexity
	assessment.Confidence = a.calculateConfidence(log, assessment.Issues)

	return assessment
}

// AssessAndUpdate evaluates a log and updates it with the assessment results.
func (a *CapabilityAssessor) AssessAndUpdate(ctx context.Context, log *ConversationLog) *Assessment {
	assessment := a.Assess(ctx, log)

	// Update log with issue flags
	for _, issue := range assessment.Issues {
		switch issue.Type {
		case IssueTimeout:
			log.HadTimeout = true
		case IssueRepetition:
			log.HadRepetition = true
		case IssueToolFailure:
			log.HadToolFailure = true
		case IssueTruncation:
			log.HadTruncation = true
		case IssueJSONError:
			log.HadJSONError = true
		}
	}

	// Update capability score
	log.CapabilityScore = assessment.CapabilityScore

	return assessment
}

// ═══════════════════════════════════════════════════════════════════════════════
// TIMEOUT DETECTION
// ═══════════════════════════════════════════════════════════════════════════════

// detectTimeout checks if response time exceeded threshold.
func (a *CapabilityAssessor) detectTimeout(log *ConversationLog, assessment *Assessment) {
	if log.DurationMs > a.timeoutThresholdMs {
		severity := SeverityMedium
		if log.DurationMs > a.timeoutThresholdMs*2 {
			severity = SeverityHigh
		}

		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueTimeout,
			Severity:    severity,
			Description: "Response time exceeded threshold",
			Evidence:    formatDuration(log.DurationMs),
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// REPETITION DETECTION
// ═══════════════════════════════════════════════════════════════════════════════

// detectRepetition checks for repetitive patterns in the response.
func (a *CapabilityAssessor) detectRepetition(log *ConversationLog, assessment *Assessment) {
	if log.Response == "" {
		return
	}

	// Check for repeated sentences
	if a.hasRepeatedSentences(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueRepetition,
			Severity:    SeverityHigh,
			Description: "Response contains repeated sentences",
			Evidence:    "Same sentence appears 3+ times",
		})
		return
	}

	// Check for n-gram repetition
	if rate := a.calculateTrigramRepetition(log.Response); rate > a.trigramThreshold {
		severity := SeverityMedium
		if rate > 0.5 {
			severity = SeverityHigh
		}

		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueRepetition,
			Severity:    severity,
			Description: "High repetition rate detected in response",
			Evidence:    formatPercent(rate),
		})
		return
	}

	// Check for stuck patterns
	if a.hasStuckPattern(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueRepetition,
			Severity:    SeverityHigh,
			Description: "Response appears stuck in a loop",
			Evidence:    "Repetitive phrase pattern detected",
		})
	}
}

// hasRepeatedSentences checks if any sentence appears 3+ times.
func (a *CapabilityAssessor) hasRepeatedSentences(text string) bool {
	sentences := splitSentences(text)
	counts := make(map[string]int)

	for _, s := range sentences {
		normalized := strings.TrimSpace(strings.ToLower(s))
		if len(normalized) > 10 { // Ignore very short sentences
			counts[normalized]++
			if counts[normalized] >= a.repetitionMinCount {
				return true
			}
		}
	}

	return false
}

// calculateTrigramRepetition calculates the repetition rate of trigrams.
func (a *CapabilityAssessor) calculateTrigramRepetition(text string) float64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) < 4 {
		return 0
	}

	trigrams := make(map[string]int)
	for i := 0; i < len(words)-2; i++ {
		trigram := words[i] + " " + words[i+1] + " " + words[i+2]
		trigrams[trigram]++
	}

	// Count repeated trigrams
	repeated := 0
	total := len(words) - 2
	for _, count := range trigrams {
		if count > 1 {
			repeated += count - 1
		}
	}

	if total == 0 {
		return 0
	}
	return float64(repeated) / float64(total)
}

// stuckPhraseStarters are common phrases that indicate a model getting stuck
var stuckPhraseStarters = []string{
	"i think",
	"let me",
	"here's",
	"i'll",
	"i will",
	"i can",
	"i would",
	"first,",
	"to do this",
}

// hasStuckPattern detects common stuck patterns like "I think... I think..."
// Uses a programmatic approach since Go's RE2 doesn't support backreferences.
func (a *CapabilityAssessor) hasStuckPattern(text string) bool {
	lowerText := strings.ToLower(text)

	// Check for repeated phrase starters (3+ occurrences within 500 chars)
	for _, phrase := range stuckPhraseStarters {
		count := 0
		searchText := lowerText
		for {
			idx := strings.Index(searchText, phrase)
			if idx == -1 {
				break
			}
			count++
			// Only count if occurrences are close together (within 500 chars)
			if count >= 3 {
				// Check if they're clustered
				firstIdx := strings.Index(lowerText, phrase)
				lastIdx := strings.LastIndex(lowerText, phrase)
				if lastIdx-firstIdx < 500 {
					return true
				}
			}
			searchText = searchText[idx+len(phrase):]
		}
	}

	// Check for repeated sentences by splitting on periods
	sentences := strings.Split(text, ".")
	sentenceCounts := make(map[string]int)
	for _, s := range sentences {
		trimmed := strings.TrimSpace(strings.ToLower(s))
		if len(trimmed) > 10 { // Ignore very short fragments
			sentenceCounts[trimmed]++
			if sentenceCounts[trimmed] >= 3 {
				return true
			}
		}
	}

	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// TOOL FAILURE DETECTION
// ═══════════════════════════════════════════════════════════════════════════════

// detectToolFailure checks for signs of failed tool execution.
func (a *CapabilityAssessor) detectToolFailure(log *ConversationLog, assessment *Assessment) {
	if log.Response == "" {
		return
	}

	// Check for error patterns in output
	if a.hasErrorPatterns(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueToolFailure,
			Severity:    SeverityMedium,
			Description: "Response indicates tool execution failure",
			Evidence:    "Error pattern detected in output",
		})
		return
	}

	// Check for malformed tool calls
	if a.hasMalformedToolCall(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueToolFailure,
			Severity:    SeverityHigh,
			Description: "Response contains malformed tool call",
			Evidence:    "Invalid tool call syntax detected",
		})
	}
}

// Error patterns indicating tool failure
var errorPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)error:\s+`),
	regexp.MustCompile(`(?i)failed to\s+`),
	regexp.MustCompile(`(?i)unable to parse`),
	regexp.MustCompile(`(?i)invalid (json|syntax|format)`),
	regexp.MustCompile(`(?i)command not found`),
	regexp.MustCompile(`(?i)permission denied`),
	regexp.MustCompile(`(?i)no such file or directory`),
	regexp.MustCompile(`(?i)tool execution failed`),
}

func (a *CapabilityAssessor) hasErrorPatterns(text string) bool {
	for _, pattern := range errorPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// Tool call patterns (JSON-like structures in responses)
var toolCallPattern = regexp.MustCompile(`\{[^}]*"(name|tool|function)"[^}]*\}`)

func (a *CapabilityAssessor) hasMalformedToolCall(text string) bool {
	// Look for incomplete JSON-like tool calls
	matches := toolCallPattern.FindAllString(text, -1)
	for _, match := range matches {
		// Check if it's valid JSON
		if !isValidJSON(match) {
			return true
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// JSON ERROR DETECTION
// ═══════════════════════════════════════════════════════════════════════════════

// detectJSONError checks for JSON parsing issues in responses.
func (a *CapabilityAssessor) detectJSONError(log *ConversationLog, assessment *Assessment) {
	if log.Response == "" {
		return
	}

	trimmed := strings.TrimSpace(log.Response)

	// Check if response looks like it should be JSON but isn't valid
	if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) {
		if !isValidJSON(trimmed) {
			assessment.Issues = append(assessment.Issues, Issue{
				Type:        IssueJSONError,
				Severity:    SeverityMedium,
				Description: "Response appears to be malformed JSON",
				Evidence:    "Starts with JSON delimiter but fails to parse",
			})
		}
	}

	// Check code blocks for malformed JSON
	if a.hasCodeBlockJSONError(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueJSONError,
			Severity:    SeverityLow,
			Description: "Code block contains invalid JSON",
			Evidence:    "JSON in code block fails to parse",
		})
	}
}

var codeBlockPattern = regexp.MustCompile("```(?:json)?\\s*\\n([^`]+)\\n```")

func (a *CapabilityAssessor) hasCodeBlockJSONError(text string) bool {
	matches := codeBlockPattern.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 1 {
			content := strings.TrimSpace(match[1])
			// Only check if it looks like JSON
			if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
				if !isValidJSON(content) {
					return true
				}
			}
		}
	}
	return false
}

// ═══════════════════════════════════════════════════════════════════════════════
// TRUNCATION DETECTION
// ═══════════════════════════════════════════════════════════════════════════════

// detectTruncation checks if response appears to be cut off.
func (a *CapabilityAssessor) detectTruncation(log *ConversationLog, assessment *Assessment) {
	if log.Response == "" {
		return
	}

	// Check for unclosed code blocks (odd number of ```)
	if a.hasUnclosedCodeBlock(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueTruncation,
			Severity:    SeverityMedium,
			Description: "Response has unclosed code block",
			Evidence:    "Odd number of code block delimiters",
		})
		return
	}

	// Check for mid-sentence ending
	if a.endsMidSentence(log.Response) {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueTruncation,
			Severity:    SeverityMedium,
			Description: "Response appears to end mid-sentence",
			Evidence:    "Missing sentence termination",
		})
		return
	}

	// Check for suspiciously short response for high complexity
	if log.ComplexityScore > 60 && len(log.Response) < 100 {
		assessment.Issues = append(assessment.Issues, Issue{
			Type:        IssueTruncation,
			Severity:    SeverityLow,
			Description: "Response unusually short for task complexity",
			Evidence:    "High complexity task with minimal response",
		})
	}
}

func (a *CapabilityAssessor) hasUnclosedCodeBlock(text string) bool {
	count := strings.Count(text, "```")
	return count%2 != 0
}

func (a *CapabilityAssessor) endsMidSentence(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return false
	}

	// Get last character
	lastChar := trimmed[len(trimmed)-1]

	// Check if ends with sentence terminator or code block
	validEndings := []byte{'.', '!', '?', '`', ')', ']', '}', '"', '\'', ':'}
	for _, v := range validEndings {
		if lastChar == v {
			return false
		}
	}

	// Also check if ends with markdown list/header (newline before content end)
	if strings.HasSuffix(trimmed, "\n") {
		return false
	}

	return true
}

// ═══════════════════════════════════════════════════════════════════════════════
// SCORING
// ═══════════════════════════════════════════════════════════════════════════════

// calculateScore computes capability score based on detected issues.
func (a *CapabilityAssessor) calculateScore(issues []Issue) float64 {
	score := 100.0

	// Deduct points based on issue severity
	for _, issue := range issues {
		switch issue.Severity {
		case SeverityHigh:
			score -= 30
		case SeverityMedium:
			score -= 15
		case SeverityLow:
			score -= 5
		}
	}

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	return score
}

// calculateConfidence determines assessment confidence level.
func (a *CapabilityAssessor) calculateConfidence(log *ConversationLog, issues []Issue) float64 {
	confidence := 0.8 // Base confidence

	// Higher confidence with more response data
	if len(log.Response) > 500 {
		confidence += 0.1
	}

	// Lower confidence for very short responses
	if len(log.Response) < 50 {
		confidence -= 0.2
	}

	// Higher confidence when clear issues are detected
	for _, issue := range issues {
		if issue.Severity == SeverityHigh {
			confidence += 0.05
		}
	}

	// Clamp to 0-1
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	return confidence
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════════

// splitSentences splits text into sentences.
func splitSentences(text string) []string {
	// Simple sentence splitting by common terminators
	pattern := regexp.MustCompile(`[.!?]+\s+`)
	return pattern.Split(text, -1)
}

// isValidJSON checks if a string is valid JSON.
func isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return json.Valid([]byte(s))
}

// formatDuration formats milliseconds into a human-readable string.
func formatDuration(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000.0
	if seconds < 60 {
		return fmt.Sprintf("%.1fs", seconds)
	}
	minutes := seconds / 60.0
	return fmt.Sprintf("%.1fm", minutes)
}

// formatPercent formats a float as a percentage string.
func formatPercent(rate float64) string {
	return fmt.Sprintf("%.0f%%", rate*100)
}
