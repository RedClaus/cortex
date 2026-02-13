package memory

import (
	"regexp"
	"strings"
	"time"
)

// SessionContext holds extracted facts from the current conversation session.
// This is ephemeral (not persisted) but cached for the session duration.
type SessionContext struct {
	Facts       []SessionFact `json:"facts"`
	LastUpdated time.Time     `json:"last_updated"`
	TokenCount  int           `json:"token_count"`
}

// SessionFact is a fact extracted from conversation history.
type SessionFact struct {
	Fact        string    `json:"fact"`
	Source      string    `json:"source"` // "pattern" or "llm"
	ExtractedAt time.Time `json:"extracted_at"`
	Priority    int       `json:"priority"` // Higher = more important
}

// SessionContextExtractor extracts facts from conversation using pattern matching.
// This runs in the hot path and must be < 10ms.
type SessionContextExtractor struct {
	patterns []*regexp.Regexp
}

// NewSessionContextExtractor creates a pattern-based fact extractor.
func NewSessionContextExtractor() *SessionContextExtractor {
	patterns := []string{
		// Travel patterns
		`(?i)(?:going|traveling|flying|heading) to ([A-Z][a-zA-Z\s]+)`,
		`(?i)(?:I'm |I am |we're |we are )(?:in|at) ([A-Z][a-zA-Z\s,]+)`,
		`(?i)trip to ([A-Z][a-zA-Z\s]+)`,
		`(?i)vacation (?:in|to|at) ([A-Z][a-zA-Z\s]+)`,
		`(?i)visiting ([A-Z][a-zA-Z\s]+)`,
		// Location patterns
		`(?i)(?:I'm |I am )(?:currently )?(?:in|at) ([A-Z][a-zA-Z\s,]+)`,
		// Date patterns
		`(?i)(?:on|this|next) (monday|tuesday|wednesday|thursday|friday|saturday|sunday)`,
		`(?i)today I (?:am |'m )(.+)`,
		// Family/relationship patterns
		`(?i)my (wife|husband|partner|spouse)(?:'s name is | is )([A-Z][a-z]+)`,
		`(?i)my (son|daughter|child)(?:'s name is | is | named )([A-Z][a-z]+)`,
		// Project patterns
		`(?i)working on (?:a project called |)([A-Z][a-zA-Z0-9_-]+)`,
		`(?i)my project (?:is called |named |is )([A-Z][a-zA-Z0-9_-]+)`,
		// Preference patterns
		`(?i)I (?:prefer|like|use|want) (.+)`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			compiled = append(compiled, re)
		}
	}

	return &SessionContextExtractor{patterns: compiled}
}

// ExtractFacts extracts facts from a message using pattern matching.
// This is designed to be fast (<10ms) for hot path use.
func (e *SessionContextExtractor) ExtractFacts(message string) []SessionFact {
	var facts []SessionFact

	for _, re := range e.patterns {
		matches := re.FindAllStringSubmatch(message, -1)
		for _, match := range matches {
			if len(match) > 1 {
				// Clean up the extracted fact
				factText := strings.TrimSpace(match[0])
				if len(factText) > 0 && len(factText) < 200 {
					fact := SessionFact{
						Fact:        factText,
						Source:      "pattern",
						ExtractedAt: time.Now(),
						Priority:    1,
					}
					facts = append(facts, fact)
				}
			}
		}
	}

	return facts
}

// Merge merges new facts into the session context, avoiding duplicates.
func (sc *SessionContext) Merge(newFacts []SessionFact) {
	seen := make(map[string]bool)
	for _, f := range sc.Facts {
		seen[strings.ToLower(f.Fact)] = true
	}

	for _, f := range newFacts {
		key := strings.ToLower(f.Fact)
		if !seen[key] {
			sc.Facts = append(sc.Facts, f)
			seen[key] = true
		}
	}
	sc.LastUpdated = time.Now()
}

// ToContextString converts session facts to a string for prompt injection.
func (sc *SessionContext) ToContextString() string {
	if len(sc.Facts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Session context (mentioned in this conversation):\n")
	for _, f := range sc.Facts {
		sb.WriteString("- ")
		sb.WriteString(f.Fact)
		sb.WriteString("\n")
	}
	return sb.String()
}

// Clear resets the session context.
func (sc *SessionContext) Clear() {
	sc.Facts = nil
	sc.LastUpdated = time.Now()
	sc.TokenCount = 0
}
