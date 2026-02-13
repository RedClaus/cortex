// Package memory implements Pinky's memory system with temporal awareness.
package memory

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TemporalContext represents parsed time references from a user query.
type TemporalContext struct {
	HasTimeReference bool
	RelativeTime     string     // e.g., "yesterday", "last week", "2 hours ago"
	AbsoluteTime     time.Time  // Parsed absolute time
	TimeRange        *TimeRange // e.g., "between Monday and Friday"
	Recurrence       string     // e.g., "every morning", "on Fridays"
}

// TimeRange represents a time range for temporal queries.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// TemporalTag stores time-related metadata for a memory.
type TemporalTag struct {
	Type  TemporalTagType // "relative", "absolute", "recurring"
	Value string          // The original expression
	Time  time.Time       // The computed time
}

// TemporalTagType categorizes temporal references.
type TemporalTagType string

const (
	TagRelative  TemporalTagType = "relative"
	TagAbsolute  TemporalTagType = "absolute"
	TagRecurring TemporalTagType = "recurring"
)

// Precompiled patterns for temporal detection
var (
	// Relative time patterns
	relativePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(yesterday|today|tomorrow)\b`),
		regexp.MustCompile(`(?i)\b(last|this|next)\s+(week|month|year|monday|tuesday|wednesday|thursday|friday|saturday|sunday)\b`),
		regexp.MustCompile(`(?i)\b(\d+)\s+(second|minute|hour|day|week|month|year)s?\s+ago\b`),
		regexp.MustCompile(`(?i)\b(earlier|later)\s+(today|this\s+week|this\s+month)\b`),
		regexp.MustCompile(`(?i)\b(this|last|earlier this)\s+(morning|afternoon|evening|night)\b`),
	}

	// Time of day patterns
	timeOfDayPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(morning|afternoon|evening|night|midnight|noon)\b`),
	}

	// Recurring patterns
	recurringPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bevery\s+(day|week|month|monday|tuesday|wednesday|thursday|friday|saturday|sunday|morning|evening)\b`),
		regexp.MustCompile(`(?i)\b(on|every)\s+(mondays|tuesdays|wednesdays|thursdays|fridays|saturdays|sundays)\b`),
		regexp.MustCompile(`(?i)\b(weekly|daily|monthly|yearly)\b`),
	}

	// Duration patterns for parsing "N units ago"
	durationPattern = regexp.MustCompile(`(?i)(\d+)\s+(second|minute|hour|day|week|month|year)s?\s+ago`)

	// Day of week pattern
	dayOfWeekPattern = regexp.MustCompile(`(?i)(last|this|next)\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)`)

	// Relative day pattern
	relativeDayPattern = regexp.MustCompile(`(?i)\b(yesterday|today|tomorrow)\b`)
)

// ParseTemporalContext extracts time references from a query string.
// Returns a TemporalContext with parsed information about any time references.
func ParseTemporalContext(query string, referenceTime time.Time) *TemporalContext {
	ctx := &TemporalContext{}
	queryLower := strings.ToLower(query)

	// Check relative patterns
	for _, pattern := range relativePatterns {
		if match := pattern.FindString(queryLower); match != "" {
			ctx.HasTimeReference = true
			ctx.RelativeTime = match
			ctx.AbsoluteTime = resolveRelativeTime(match, referenceTime)
			return ctx
		}
	}

	// Check recurring patterns
	for _, pattern := range recurringPatterns {
		if match := pattern.FindString(queryLower); match != "" {
			ctx.HasTimeReference = true
			ctx.Recurrence = match
			return ctx
		}
	}

	// Check time of day patterns (these provide context but not specific times)
	for _, pattern := range timeOfDayPatterns {
		if match := pattern.FindString(queryLower); match != "" {
			ctx.HasTimeReference = true
			ctx.RelativeTime = match
			ctx.AbsoluteTime = resolveTimeOfDay(match, referenceTime)
			return ctx
		}
	}

	return ctx
}

// resolveRelativeTime converts a relative time expression to an absolute time.
func resolveRelativeTime(expr string, now time.Time) time.Time {
	exprLower := strings.ToLower(strings.TrimSpace(expr))

	// Handle "yesterday", "today", "tomorrow"
	if match := relativeDayPattern.FindString(exprLower); match != "" {
		switch strings.ToLower(match) {
		case "yesterday":
			return now.AddDate(0, 0, -1)
		case "today":
			return now
		case "tomorrow":
			return now.AddDate(0, 0, 1)
		}
	}

	// Handle "N units ago"
	if matches := durationPattern.FindStringSubmatch(exprLower); len(matches) == 3 {
		amount, _ := strconv.Atoi(matches[1])
		unit := strings.ToLower(matches[2])
		return subtractDuration(now, amount, unit)
	}

	// Handle "last/this/next [day of week]"
	if matches := dayOfWeekPattern.FindStringSubmatch(exprLower); len(matches) == 3 {
		modifier := strings.ToLower(matches[1])
		dayName := strings.ToLower(matches[2])
		return resolveWeekday(now, modifier, dayName)
	}

	// Handle "last/this/next week/month/year"
	if strings.Contains(exprLower, "last week") {
		return now.AddDate(0, 0, -7)
	}
	if strings.Contains(exprLower, "this week") {
		return now
	}
	if strings.Contains(exprLower, "next week") {
		return now.AddDate(0, 0, 7)
	}
	if strings.Contains(exprLower, "last month") {
		return now.AddDate(0, -1, 0)
	}
	if strings.Contains(exprLower, "this month") {
		return now
	}
	if strings.Contains(exprLower, "next month") {
		return now.AddDate(0, 1, 0)
	}
	if strings.Contains(exprLower, "last year") {
		return now.AddDate(-1, 0, 0)
	}
	if strings.Contains(exprLower, "this year") {
		return now
	}
	if strings.Contains(exprLower, "next year") {
		return now.AddDate(1, 0, 0)
	}

	// Handle time of day modifiers
	if strings.Contains(exprLower, "this morning") || strings.Contains(exprLower, "earlier this morning") {
		return time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
	}
	if strings.Contains(exprLower, "this afternoon") {
		return time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, now.Location())
	}
	if strings.Contains(exprLower, "this evening") || strings.Contains(exprLower, "last evening") {
		return time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	}
	if strings.Contains(exprLower, "last night") {
		yesterday := now.AddDate(0, 0, -1)
		return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 21, 0, 0, 0, now.Location())
	}

	return now
}

// resolveTimeOfDay converts a time-of-day expression to an absolute time.
func resolveTimeOfDay(expr string, now time.Time) time.Time {
	exprLower := strings.ToLower(strings.TrimSpace(expr))

	switch exprLower {
	case "morning":
		return time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
	case "noon":
		return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	case "afternoon":
		return time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, now.Location())
	case "evening":
		return time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	case "night":
		return time.Date(now.Year(), now.Month(), now.Day(), 21, 0, 0, 0, now.Location())
	case "midnight":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}

	return now
}

// subtractDuration subtracts an amount of time units from the given time.
func subtractDuration(t time.Time, amount int, unit string) time.Time {
	switch unit {
	case "second":
		return t.Add(-time.Duration(amount) * time.Second)
	case "minute":
		return t.Add(-time.Duration(amount) * time.Minute)
	case "hour":
		return t.Add(-time.Duration(amount) * time.Hour)
	case "day":
		return t.AddDate(0, 0, -amount)
	case "week":
		return t.AddDate(0, 0, -amount*7)
	case "month":
		return t.AddDate(0, -amount, 0)
	case "year":
		return t.AddDate(-amount, 0, 0)
	}
	return t
}

// resolveWeekday resolves "last/this/next [weekday]" to an absolute time.
func resolveWeekday(now time.Time, modifier, dayName string) time.Time {
	targetDay := parseWeekday(dayName)
	if targetDay == -1 {
		return now
	}

	currentDay := int(now.Weekday())
	diff := targetDay - currentDay

	switch modifier {
	case "last":
		if diff >= 0 {
			diff -= 7
		}
	case "this":
		// "this" means within the current week
		// If the day has passed, it refers to that past day
		// If it's upcoming, it refers to that future day
	case "next":
		if diff <= 0 {
			diff += 7
		}
	}

	return now.AddDate(0, 0, diff)
}

// parseWeekday converts a day name to time.Weekday value.
func parseWeekday(name string) int {
	switch strings.ToLower(name) {
	case "sunday":
		return int(time.Sunday)
	case "monday":
		return int(time.Monday)
	case "tuesday":
		return int(time.Tuesday)
	case "wednesday":
		return int(time.Wednesday)
	case "thursday":
		return int(time.Thursday)
	case "friday":
		return int(time.Friday)
	case "saturday":
		return int(time.Saturday)
	}
	return -1
}

// CreateTemporalTag creates a TemporalTag from an expression.
func CreateTemporalTag(expr string, referenceTime time.Time) *TemporalTag {
	ctx := ParseTemporalContext(expr, referenceTime)
	if !ctx.HasTimeReference {
		return nil
	}

	tagType := TagRelative
	if ctx.Recurrence != "" {
		tagType = TagRecurring
	}

	return &TemporalTag{
		Type:  tagType,
		Value: ctx.RelativeTime,
		Time:  ctx.AbsoluteTime,
	}
}

// TemporalDistance calculates the time distance between a memory and a query context.
// Returns a score from 0 to 1, where 1 is an exact match.
func TemporalDistance(memoryTime time.Time, queryContext *TemporalContext) float64 {
	if !queryContext.HasTimeReference {
		return 0.5 // Neutral score when no temporal reference
	}

	// If we have a time range, check if memory falls within
	if queryContext.TimeRange != nil {
		if memoryTime.After(queryContext.TimeRange.Start) && memoryTime.Before(queryContext.TimeRange.End) {
			return 1.0
		}
		return 0.0
	}

	// Calculate distance from the target time
	targetTime := queryContext.AbsoluteTime
	distance := memoryTime.Sub(targetTime).Abs()

	// Score based on proximity
	// Same day: 1.0
	// Same week: 0.7
	// Same month: 0.4
	// Older: decay to 0.1
	switch {
	case distance < 24*time.Hour:
		return 1.0
	case distance < 7*24*time.Hour:
		return 0.7
	case distance < 30*24*time.Hour:
		return 0.4
	case distance < 365*24*time.Hour:
		return 0.2
	default:
		return 0.1
	}
}
