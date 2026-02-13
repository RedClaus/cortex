package memory

import (
	"testing"
	"time"
)

func TestParseTemporalContext_RelativeDay(t *testing.T) {
	// Use a fixed reference time for reproducible tests
	refTime := time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name          string
		query         string
		wantHasRef    bool
		wantRelative  string
		wantDayOffset int // Expected day offset from refTime
	}{
		{
			name:          "yesterday",
			query:         "What did I do yesterday?",
			wantHasRef:    true,
			wantRelative:  "yesterday",
			wantDayOffset: -1,
		},
		{
			name:          "today",
			query:         "Show me today's tasks",
			wantHasRef:    true,
			wantRelative:  "today",
			wantDayOffset: 0,
		},
		{
			name:          "tomorrow",
			query:         "Schedule for tomorrow",
			wantHasRef:    true,
			wantRelative:  "tomorrow",
			wantDayOffset: 1,
		},
		{
			name:       "no temporal reference",
			query:      "Tell me about the project",
			wantHasRef: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if ctx.HasTimeReference != tt.wantHasRef {
				t.Errorf("HasTimeReference = %v, want %v", ctx.HasTimeReference, tt.wantHasRef)
			}

			if tt.wantHasRef {
				if ctx.RelativeTime != tt.wantRelative {
					t.Errorf("RelativeTime = %q, want %q", ctx.RelativeTime, tt.wantRelative)
				}

				expectedDay := refTime.AddDate(0, 0, tt.wantDayOffset)
				if ctx.AbsoluteTime.Year() != expectedDay.Year() ||
					ctx.AbsoluteTime.Month() != expectedDay.Month() ||
					ctx.AbsoluteTime.Day() != expectedDay.Day() {
					t.Errorf("AbsoluteTime = %v, want day to be %v",
						ctx.AbsoluteTime.Format("2006-01-02"),
						expectedDay.Format("2006-01-02"))
				}
			}
		})
	}
}

func TestParseTemporalContext_DurationAgo(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name         string
		query        string
		wantDuration time.Duration
	}{
		{
			name:         "2 hours ago",
			query:        "What happened 2 hours ago?",
			wantDuration: 2 * time.Hour,
		},
		{
			name:         "3 days ago",
			query:        "Show me the changes from 3 days ago",
			wantDuration: 3 * 24 * time.Hour,
		},
		{
			name:         "1 week ago",
			query:        "The meeting 1 week ago",
			wantDuration: 7 * 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if !ctx.HasTimeReference {
				t.Fatal("Expected HasTimeReference to be true")
			}

			expectedTime := refTime.Add(-tt.wantDuration)
			diff := ctx.AbsoluteTime.Sub(expectedTime).Abs()

			// Allow for some tolerance in the comparison
			if diff > time.Minute {
				t.Errorf("AbsoluteTime = %v, want approximately %v (diff: %v)",
					ctx.AbsoluteTime, expectedTime, diff)
			}
		})
	}
}

func TestParseTemporalContext_WeekRelative(t *testing.T) {
	// Use a Wednesday for testing
	refTime := time.Date(2026, 2, 4, 14, 0, 0, 0, time.UTC) // Wednesday

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "last week",
			query: "The deployment from last week",
		},
		{
			name:  "this week",
			query: "Meetings this week",
		},
		{
			name:  "next week",
			query: "Plan for next week",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if !ctx.HasTimeReference {
				t.Errorf("Expected HasTimeReference to be true for %q", tt.query)
			}
		})
	}
}

func TestParseTemporalContext_DayOfWeek(t *testing.T) {
	// Use a Wednesday (Feb 4, 2026 is Wednesday)
	refTime := time.Date(2026, 2, 4, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		query      string
		wantHasRef bool
	}{
		{
			name:       "last monday",
			query:      "Meeting last Monday",
			wantHasRef: true,
		},
		{
			name:       "next friday",
			query:      "Schedule for next Friday",
			wantHasRef: true,
		},
		{
			name:       "this tuesday",
			query:      "This Tuesday's agenda",
			wantHasRef: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if ctx.HasTimeReference != tt.wantHasRef {
				t.Errorf("HasTimeReference = %v, want %v", ctx.HasTimeReference, tt.wantHasRef)
			}
		})
	}
}

func TestParseTemporalContext_TimeOfDay(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		query    string
		wantHour int
	}{
		{
			name:     "morning",
			query:    "This morning's email",
			wantHour: 9,
		},
		{
			name:     "afternoon",
			query:    "This afternoon",
			wantHour: 14,
		},
		{
			name:     "evening",
			query:    "This evening",
			wantHour: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if !ctx.HasTimeReference {
				t.Fatal("Expected HasTimeReference to be true")
			}

			if ctx.AbsoluteTime.Hour() != tt.wantHour {
				t.Errorf("Hour = %d, want %d", ctx.AbsoluteTime.Hour(), tt.wantHour)
			}
		})
	}
}

func TestParseTemporalContext_Recurring(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name           string
		query          string
		wantRecurrence string
	}{
		{
			name:           "every monday",
			query:          "Every Monday meeting",
			wantRecurrence: "every monday",
		},
		{
			name:           "daily",
			query:          "Daily standup",
			wantRecurrence: "daily",
		},
		{
			name:           "weekly",
			query:          "Weekly sync",
			wantRecurrence: "weekly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ParseTemporalContext(tt.query, refTime)

			if !ctx.HasTimeReference {
				t.Fatal("Expected HasTimeReference to be true")
			}

			if ctx.Recurrence != tt.wantRecurrence {
				t.Errorf("Recurrence = %q, want %q", ctx.Recurrence, tt.wantRecurrence)
			}
		})
	}
}

func TestTemporalDistance(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		memoryTime    time.Time
		queryTime     time.Time
		wantScoreMin  float64
		wantScoreMax  float64
	}{
		{
			name:         "same day - high score",
			memoryTime:   refTime.Add(-2 * time.Hour),
			queryTime:    refTime,
			wantScoreMin: 0.9,
			wantScoreMax: 1.0,
		},
		{
			name:         "same week - medium score",
			memoryTime:   refTime.AddDate(0, 0, -3),
			queryTime:    refTime,
			wantScoreMin: 0.6,
			wantScoreMax: 0.8,
		},
		{
			name:         "same month - lower score",
			memoryTime:   refTime.AddDate(0, 0, -20),
			queryTime:    refTime,
			wantScoreMin: 0.3,
			wantScoreMax: 0.5,
		},
		{
			name:         "old memory - low score",
			memoryTime:   refTime.AddDate(-1, 0, 0),
			queryTime:    refTime,
			wantScoreMin: 0.0,
			wantScoreMax: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &TemporalContext{
				HasTimeReference: true,
				AbsoluteTime:     tt.queryTime,
			}

			score := TemporalDistance(tt.memoryTime, ctx)

			if score < tt.wantScoreMin || score > tt.wantScoreMax {
				t.Errorf("score = %f, want between %f and %f",
					score, tt.wantScoreMin, tt.wantScoreMax)
			}
		})
	}
}

func TestTemporalDistance_NoContext(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 0, 0, 0, time.UTC)
	memoryTime := refTime.AddDate(0, 0, -5)

	// With no temporal reference, should return neutral score
	ctx := &TemporalContext{
		HasTimeReference: false,
	}

	score := TemporalDistance(memoryTime, ctx)

	if score != 0.5 {
		t.Errorf("score = %f, want 0.5 (neutral)", score)
	}
}

func TestCreateTemporalTag(t *testing.T) {
	refTime := time.Date(2026, 2, 7, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		expr     string
		wantNil  bool
		wantType TemporalTagType
	}{
		{
			name:     "yesterday creates tag",
			expr:     "yesterday",
			wantNil:  false,
			wantType: TagRelative,
		},
		{
			name:     "every monday creates recurring tag",
			expr:     "every monday",
			wantNil:  false,
			wantType: TagRecurring,
		},
		{
			name:    "no temporal reference returns nil",
			expr:    "hello world",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag := CreateTemporalTag(tt.expr, refTime)

			if tt.wantNil {
				if tag != nil {
					t.Errorf("Expected nil tag, got %+v", tag)
				}
				return
			}

			if tag == nil {
				t.Fatal("Expected non-nil tag")
			}

			if tag.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", tag.Type, tt.wantType)
			}
		})
	}
}
