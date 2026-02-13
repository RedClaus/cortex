package introspection

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

// Embedder defines the interface for generating embeddings (optional).
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Classifier detects and categorizes introspection queries.
// It uses a two-phase approach:
// 1. Fast regex pattern matching for common phrases
// 2. LLM classification for ambiguous cases
type Classifier struct {
	patterns    map[QueryType][]*regexp.Regexp
	llmProvider LLMProvider
	embedder    Embedder // Optional, for future semantic similarity
}

// NewClassifier creates a new introspection classifier.
// The embedder parameter is optional and can be nil.
func NewClassifier(llm LLMProvider, embedder Embedder) *Classifier {
	c := &Classifier{
		patterns:    make(map[QueryType][]*regexp.Regexp),
		llmProvider: llm,
		embedder:    embedder,
	}
	c.compilePatterns()
	return c
}

// compilePatterns initializes regex patterns for each query type.
func (c *Classifier) compilePatterns() {
	// Knowledge check patterns - "do you know X?"
	c.patterns[QueryTypeKnowledgeCheck] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)do you (know|have|understand|remember)\s+(.+)`),
		regexp.MustCompile(`(?i)(do|have) you (learned|stored|memorized)\s+(.+)`),
		regexp.MustCompile(`(?i)is\s+(.+)\s+in your (memory|knowledge|database)`),
		regexp.MustCompile(`(?i)have you been (taught|trained on)\s+(.+)`),
		regexp.MustCompile(`(?i)do you have\s+(.+)\s+(stored|saved|in memory)`),
		regexp.MustCompile(`(?i)what do you know about\s+(.+)`),
		regexp.MustCompile(`(?i)do you have any (knowledge|information) (about|on)\s+(.+)`),
	}

	// Capability check patterns - "can you help with X?"
	c.patterns[QueryTypeCapabilityCheck] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)can you (help with|assist with|do)\s+(.+)`),
		regexp.MustCompile(`(?i)are you (able|capable) (to|of)\s+(.+)`),
		regexp.MustCompile(`(?i)do you (support|handle)\s+(.+)`),
		regexp.MustCompile(`(?i)can you help me (with|on)\s+(.+)`),
	}

	// Memory list patterns - "what do you know?"
	c.patterns[QueryTypeMemoryList] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)what (do you know|have you learned|is in your memory)\??\s*$`),
		regexp.MustCompile(`(?i)list (your|all) (knowledge|memories|topics)`),
		regexp.MustCompile(`(?i)show me what you know`),
		regexp.MustCompile(`(?i)what topics do you know`),
		regexp.MustCompile(`(?i)what have you learned`),
	}

	// Skill assessment patterns - "how good are you at X?"
	c.patterns[QueryTypeSkillAssessment] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)how (good|well) (are you|do you) (at|know)\s+(.+)`),
		regexp.MustCompile(`(?i)rate your (knowledge|expertise|skill) (in|on|at)\s+(.+)`),
		regexp.MustCompile(`(?i)what'?s your (level|expertise) (in|on|at)\s+(.+)`),
	}
}

// Classify analyzes input and returns an IntrospectionQuery.
func (c *Classifier) Classify(ctx context.Context, input string) (*IntrospectionQuery, error) {
	input = strings.TrimSpace(input)

	// Phase 1: Fast regex pattern matching
	for queryType, patterns := range c.patterns {
		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(input); matches != nil {
				subject := extractSubject(matches)
				return &IntrospectionQuery{
					Type:          queryType,
					Subject:       subject,
					SearchTerms:   expandSearchTerms(subject),
					OriginalQuery: input,
					Confidence:    0.9,
				}, nil
			}
		}
	}

	// Phase 2: LLM classification for ambiguous cases
	if c.llmProvider != nil && seemsPotentiallyIntrospective(input) {
		return c.classifyWithLLM(ctx, input)
	}

	// Not an introspection query
	return &IntrospectionQuery{
		Type:          QueryTypeNotIntrospective,
		OriginalQuery: input,
		Confidence:    0.95,
	}, nil
}

// classifyWithLLM uses the LLM for ambiguous classification.
func (c *Classifier) classifyWithLLM(ctx context.Context, input string) (*IntrospectionQuery, error) {
	prompt := `Analyze this user query and determine if they are asking about the AI assistant's own knowledge, memory, or capabilities.

Query: "` + input + `"

Respond in JSON format only:
{
  "is_introspective": true/false,
  "query_type": "knowledge_check|capability_check|memory_list|skill_assessment|not_introspective",
  "subject": "the topic they're asking about (if applicable, else empty string)",
  "confidence": 0.0-1.0
}

Guidelines:
- "knowledge_check": User asks if you know/have/remember something specific
- "capability_check": User asks if you can do/help with something
- "memory_list": User asks what you know in general
- "skill_assessment": User asks about your skill level
- "not_introspective": Regular question not about your own knowledge

Only classify as introspective if the user is genuinely asking about what the AI knows/has stored, not just asking a regular question.`

	response, err := c.llmProvider.Complete(ctx, prompt)
	if err != nil {
		// Fall back to not introspective on LLM error
		return &IntrospectionQuery{
			Type:          QueryTypeNotIntrospective,
			OriginalQuery: input,
			Confidence:    0.5,
		}, nil
	}

	return parseClassificationResponse(response, input)
}

// parseClassificationResponse parses the LLM JSON response.
func parseClassificationResponse(response, input string) (*IntrospectionQuery, error) {
	// Try to extract JSON from response
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	codeBlockMarker := "```"
	if strings.HasPrefix(response, codeBlockMarker) {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, codeBlockMarker) {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	var result struct {
		IsIntrospective bool    `json:"is_introspective"`
		QueryType       string  `json:"query_type"`
		Subject         string  `json:"subject"`
		Confidence      float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// Parse failed, default to not introspective
		return &IntrospectionQuery{
			Type:          QueryTypeNotIntrospective,
			OriginalQuery: input,
			Confidence:    0.5,
		}, nil
	}

	if !result.IsIntrospective {
		return &IntrospectionQuery{
			Type:          QueryTypeNotIntrospective,
			OriginalQuery: input,
			Confidence:    result.Confidence,
		}, nil
	}

	queryType := QueryType(result.QueryType)
	// Validate query type
	switch queryType {
	case QueryTypeKnowledgeCheck, QueryTypeCapabilityCheck,
		QueryTypeMemoryList, QueryTypeSkillAssessment:
		// Valid
	default:
		queryType = QueryTypeNotIntrospective
	}

	return &IntrospectionQuery{
		Type:          queryType,
		Subject:       result.Subject,
		SearchTerms:   expandSearchTerms(result.Subject),
		OriginalQuery: input,
		Confidence:    result.Confidence,
	}, nil
}

// extractSubject extracts the subject from regex matches.
func extractSubject(matches []string) string {
	// Return the last non-empty capture group
	for i := len(matches) - 1; i > 0; i-- {
		if matches[i] != "" {
			return cleanSubject(matches[i])
		}
	}
	return ""
}

// cleanSubject removes common suffixes and cleans up the subject.
func cleanSubject(s string) string {
	s = strings.TrimSpace(s)

	// Remove trailing punctuation
	s = strings.TrimRight(s, "?!.")

	// Remove common filler words at the end
	suffixes := []string{
		" or not",
		" at all",
		" already",
	}
	for _, suffix := range suffixes {
		s = strings.TrimSuffix(s, suffix)
	}

	return strings.TrimSpace(s)
}

// expandSearchTerms generates additional search terms from the subject.
func expandSearchTerms(subject string) []string {
	if subject == "" {
		return nil
	}

	terms := []string{subject}

	// Add common variations
	synonyms := map[string][]string{
		"linux":       {"bash", "shell", "unix", "terminal"},
		"commands":    {"command", "cmd", "cli"},
		"docker":      {"container", "containerization"},
		"git":         {"version control", "vcs"},
		"python":      {"py", "python3"},
		"javascript":  {"js", "node", "nodejs"},
		"go":          {"golang"},
		"kubernetes":  {"k8s", "kube"},
		"postgres":    {"postgresql", "psql"},
		"redis":       {"cache", "caching"},
		"api":         {"rest", "http", "endpoint"},
		"database":    {"db", "sql"},
		"programming": {"coding", "development"},
	}

	subjectLower := strings.ToLower(subject)
	for key, syns := range synonyms {
		if strings.Contains(subjectLower, key) {
			terms = append(terms, syns...)
		}
	}

	return terms
}

// seemsPotentiallyIntrospective checks if input might be introspective.
func seemsPotentiallyIntrospective(input string) bool {
	markers := []string{
		"you know",
		"your memory",
		"your knowledge",
		"have you",
		"do you",
		"can you",
		"are you",
		"what do you",
		"what have you",
		"do you have",
	}

	inputLower := strings.ToLower(input)
	for _, marker := range markers {
		if strings.Contains(inputLower, marker) {
			return true
		}
	}
	return false
}
