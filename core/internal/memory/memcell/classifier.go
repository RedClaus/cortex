package memcell

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

// ══════════════════════════════════════════════════════════════════════════════
// CLASSIFIER - 3-TIER MEMORY TYPE CLASSIFICATION
// Tier 1: Regex patterns (instant, $0)
// Tier 2: Embedding similarity (cached, ~$0)
// Tier 3: LLM classification (fallback, $$$)
// ══════════════════════════════════════════════════════════════════════════════

// Classifier implements 3-tier memory type classification.
type Classifier struct {
	llm        LLMProvider
	embedder   EmbedderFunc
	cache      *classifierCache
	patterns   map[MemoryType][]*regexp.Regexp
	thresholds ClassifierThresholds
}

// ClassifierThresholds configures classification confidence thresholds.
type ClassifierThresholds struct {
	PatternConfidence   float64 // Minimum confidence for pattern match
	EmbeddingConfidence float64 // Minimum confidence for embedding match
	LLMFallbackBelow    float64 // Use LLM if confidence below this
}

// DefaultClassifierThresholds returns sensible defaults.
func DefaultClassifierThresholds() ClassifierThresholds {
	return ClassifierThresholds{
		PatternConfidence:   0.7,
		EmbeddingConfidence: 0.6,
		LLMFallbackBelow:    0.5,
	}
}

// NewClassifier creates a new 3-tier classifier.
func NewClassifier(llm LLMProvider, embedder EmbedderFunc) *Classifier {
	c := &Classifier{
		llm:        llm,
		embedder:   embedder,
		cache:      newClassifierCache(),
		thresholds: DefaultClassifierThresholds(),
	}
	c.initPatterns()
	return c
}

// Classify determines the memory type of content using 3-tier classification.
func (c *Classifier) Classify(ctx context.Context, content string) (MemoryType, float64, error) {
	// Check cache first
	if cached, ok := c.cache.get(content); ok {
		return cached.memType, cached.confidence, nil
	}

	// Tier 1: Pattern matching (instant, free)
	memType, confidence := c.classifyByPatterns(content)
	if confidence >= c.thresholds.PatternConfidence {
		c.cache.set(content, memType, confidence)
		return memType, confidence, nil
	}

	// Tier 2: Embedding similarity (if embedder available)
	if c.embedder != nil {
		embType, embConf := c.classifyByEmbedding(ctx, content)
		if embConf > confidence {
			memType, confidence = embType, embConf
		}
		if confidence >= c.thresholds.EmbeddingConfidence {
			c.cache.set(content, memType, confidence)
			return memType, confidence, nil
		}
	}

	// Tier 3: LLM classification (if available and confidence still low)
	if c.llm != nil && confidence < c.thresholds.LLMFallbackBelow {
		llmType, llmConf, err := c.classifyByLLM(ctx, content)
		if err == nil && llmConf > confidence {
			memType, confidence = llmType, llmConf
		}
	}

	c.cache.set(content, memType, confidence)
	return memType, confidence, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// TIER 1: PATTERN-BASED CLASSIFICATION
// ══════════════════════════════════════════════════════════════════════════════

func (c *Classifier) initPatterns() {
	c.patterns = make(map[MemoryType][]*regexp.Regexp)

	// Strategic: Principle patterns
	c.patterns[MemTypePrinciple] = compilePatterns([]string{
		`(?i)\b(always|never)\s+(should|must|need to)`,
		`(?i)\b(important|critical|essential)\s+to\s+`,
		`(?i)\brule:\s*`,
		`(?i)\bprinciple:\s*`,
		`(?i)\bbest practice[s]?:\s*`,
		`(?i)\bkey insight:\s*`,
		`(?i)\bremember:\s+always`,
		`(?i)\bfundamental(ly)?`,
	})

	// Strategic: Lesson patterns
	c.patterns[MemTypeLesson] = compilePatterns([]string{
		`(?i)\blearned\s+(that|the)`,
		`(?i)\brealized\s+(that|the)`,
		`(?i)\bmistake\s+(was|is)`,
		`(?i)\blesson:\s*`,
		`(?i)\bfrom\s+now\s+on`,
		`(?i)\bnext\s+time`,
		`(?i)\bwon'?t\s+forget`,
		`(?i)\bturns?\s+out`,
		`(?i)\bnote\s+to\s+self`,
	})

	// Strategic: Goal patterns
	c.patterns[MemTypeGoal] = compilePatterns([]string{
		`(?i)\bgoal:\s*`,
		`(?i)\bobjective:\s*`,
		`(?i)\bwant\s+to\s+`,
		`(?i)\bneed\s+to\s+`,
		`(?i)\bplanning\s+to\s+`,
		`(?i)\baim\s+to\s+`,
		`(?i)\bintend\s+to\s+`,
		`(?i)\btodo:\s*`,
		`(?i)\bby\s+end\s+of`,
	})

	// Personal: Preference patterns
	c.patterns[MemTypePreference] = compilePatterns([]string{
		`(?i)\bi\s+prefer`,
		`(?i)\bi\s+(like|love)\s+`,
		`(?i)\bi\s+(don'?t|do not)\s+(like|want)`,
		`(?i)\bi\s+hate`,
		`(?i)\bmy\s+favorite`,
		`(?i)\bi\s+usually`,
		`(?i)\bi\s+always\s+use`,
		`(?i)\bwhen\s+i\s+`,
		`(?i)\bfor\s+me,?\s+`,
	})

	// Personal: Profile patterns
	c.patterns[MemTypeProfile] = compilePatterns([]string{
		`(?i)\bi\s+am\s+a`,
		`(?i)\bi\s+work\s+(at|on|with)`,
		`(?i)\bmy\s+(name|job|role|team)`,
		`(?i)\bi'?m\s+(based|located)`,
		`(?i)\bi\s+(live|work)\s+in`,
		`(?i)\bmy\s+background`,
		`(?i)\bi\s+specialize`,
	})

	// Personal: Relationship patterns
	c.patterns[MemTypeRelationship] = compilePatterns([]string{
		`(?i)\bmy\s+(colleague|coworker|friend|boss|manager)`,
		`(?i)\b(he|she|they)\s+(is|are)\s+my`,
		`(?i)\bwork(ing|s)?\s+with\s+\w+`,
		`(?i)\breporting\s+to`,
		`(?i)\bteam\s+member`,
	})

	// Semantic: Fact patterns
	c.patterns[MemTypeFact] = compilePatterns([]string{
		`(?i)\bis\s+a\s+`,
		`(?i)\bare\s+the\s+`,
		`(?i)\bmeans\s+that`,
		`(?i)\bdefined\s+as`,
		`(?i)\bequals?\s+`,
		`(?i)\bconsists?\s+of`,
		`(?i)\baccording\s+to`,
	})

	// Semantic: Knowledge patterns
	c.patterns[MemTypeKnowledge] = compilePatterns([]string{
		`(?i)\bbecause\s+`,
		`(?i)\bsince\s+`,
		`(?i)\btherefore`,
		`(?i)\bthis\s+means`,
		`(?i)\bthe\s+reason`,
		`(?i)\bin\s+order\s+to`,
		`(?i)\bso\s+that`,
	})

	// Semantic: Procedure patterns
	c.patterns[MemTypeProcedure] = compilePatterns([]string{
		`(?i)\bhow\s+to\s+`,
		`(?i)\bstep\s+\d+`,
		`(?i)\bfirst,?\s+`,
		`(?i)\bthen,?\s+`,
		`(?i)\bfinally,?\s+`,
		`(?i)\bto\s+do\s+this`,
		`(?i)\bhere'?s?\s+how`,
		`(?i)\brun\s+the\s+command`,
		"```",                           // Code block
		`(?i)\bfunction\s+\w+`,          // Code definition
		`(?i)\bdef\s+\w+`,               // Python function
		`(?i)\bfunc\s+\w+`,              // Go function
		`(?i)\bclass\s+\w+`,             // Class definition
	})

	// Contextual: Project patterns
	c.patterns[MemTypeProject] = compilePatterns([]string{
		`(?i)\bproject:\s*`,
		`(?i)\bworking\s+on\s+`,
		`(?i)\brepository`,
		`(?i)\bcodebase`,
		`(?i)\bthe\s+\w+\s+project`,
		`(?i)\bin\s+this\s+repo`,
	})

	// Contextual: Mood patterns
	c.patterns[MemTypeMood] = compilePatterns([]string{
		`(?i)\bi\s+(feel|am feeling)`,
		`(?i)\bi'?m\s+(happy|sad|frustrated|excited|worried|anxious)`,
		`(?i)\bfrustrat(ed|ing)`,
		`(?i)\bexcit(ed|ing)`,
		`(?i)\bworri(ed|some)`,
	})
}

func (c *Classifier) classifyByPatterns(content string) (MemoryType, float64) {
	// Priority order: strategic > personal > semantic > contextual > episodic
	priorityOrder := []MemoryType{
		// Strategic (highest priority)
		MemTypePrinciple, MemTypeLesson, MemTypeGoal,
		// Personal
		MemTypePreference, MemTypeProfile, MemTypeRelationship,
		// Semantic
		MemTypeProcedure, MemTypeFact, MemTypeKnowledge,
		// Contextual
		MemTypeProject, MemTypeMood, MemTypeContext,
	}

	for _, memType := range priorityOrder {
		patterns, ok := c.patterns[memType]
		if !ok {
			continue
		}
		for _, pattern := range patterns {
			if pattern.MatchString(content) {
				// Higher confidence for longer/more specific matches
				matchLen := len(pattern.FindString(content))
				confidence := 0.6 + float64(matchLen)/200.0
				if confidence > 0.9 {
					confidence = 0.9
				}
				return memType, confidence
			}
		}
	}

	// Default to interaction
	return MemTypeInteraction, 0.4
}

// ══════════════════════════════════════════════════════════════════════════════
// TIER 2: EMBEDDING-BASED CLASSIFICATION
// ══════════════════════════════════════════════════════════════════════════════

// Exemplar texts for each memory type (used for embedding similarity).
var typeExemplars = map[MemoryType][]string{
	MemTypePrinciple: {
		"Always validate user input before processing",
		"Never commit secrets to version control",
		"Important: always use context for cancellation",
	},
	MemTypeLesson: {
		"I learned that caching is essential for performance",
		"Mistake was not testing edge cases",
		"Realized the bug was in the authentication flow",
	},
	MemTypeGoal: {
		"Goal: implement the new feature by Friday",
		"Want to refactor the database layer",
		"Need to improve test coverage to 80%",
	},
	MemTypePreference: {
		"I prefer using Go for backend services",
		"I like having type safety in my code",
		"I usually start with writing tests first",
	},
	MemTypeProcedure: {
		"How to deploy: first build the binary, then copy to server",
		"Step 1: Install dependencies with npm install",
		"Run the following command to start the server",
	},
	MemTypeKnowledge: {
		"The HTTP status code 404 means not found",
		"Go uses goroutines for concurrent execution",
		"REST APIs use HTTP methods for CRUD operations",
	},
}

func (c *Classifier) classifyByEmbedding(ctx context.Context, content string) (MemoryType, float64) {
	if c.embedder == nil {
		return MemTypeInteraction, 0
	}

	contentEmb, err := c.embedder(ctx, content)
	if err != nil {
		return MemTypeInteraction, 0
	}

	bestType := MemTypeInteraction
	bestSim := 0.0

	for memType, exemplars := range typeExemplars {
		for _, exemplar := range exemplars {
			// Check cache for exemplar embedding
			exemplarEmb, ok := c.cache.getEmbedding(exemplar)
			if !ok {
				exemplarEmb, err = c.embedder(ctx, exemplar)
				if err != nil {
					continue
				}
				c.cache.setEmbedding(exemplar, exemplarEmb)
			}

			sim := cosineSimilarity(contentEmb, exemplarEmb)
			if sim > bestSim {
				bestSim = sim
				bestType = memType
			}
		}
	}

	// Convert similarity to confidence (0.5-1.0 range maps to 0.5-0.9)
	confidence := 0.5 + (bestSim-0.5)*0.8
	if confidence < 0.5 {
		confidence = 0.5
	}
	if confidence > 0.9 {
		confidence = 0.9
	}

	return bestType, confidence
}

// ══════════════════════════════════════════════════════════════════════════════
// TIER 3: LLM-BASED CLASSIFICATION
// ══════════════════════════════════════════════════════════════════════════════

func (c *Classifier) classifyByLLM(ctx context.Context, content string) (MemoryType, float64, error) {
	if c.llm == nil {
		return MemTypeInteraction, 0, nil
	}

	prompt := `Classify the following text into exactly one memory type. Return JSON with "type" and "confidence" (0-1).

Memory types:
- episode: Conversation segment or narrative
- event: Discrete occurrence
- interaction: User message/response
- fact: Verified information
- knowledge: Domain knowledge
- procedure: How-to or step-by-step
- preference: User preference/like/dislike
- profile: User characteristic
- relationship: About people/connections
- principle: Guiding rule or best practice
- lesson: Insight from experience/mistake
- goal: Objective or intention
- context: Situational information
- project: Project-specific info
- mood: Emotional state

Text:
` + truncate(content, 500) + `

JSON response:`

	response, err := c.llm.Complete(ctx, prompt)
	if err != nil {
		return MemTypeInteraction, 0, err
	}

	// Parse JSON response
	var result struct {
		Type       string  `json:"type"`
		Confidence float64 `json:"confidence"`
	}

	// Try to extract JSON from response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			memType := MemoryType(result.Type)
			if memType.IsValid() {
				return memType, result.Confidence, nil
			}
		}
	}

	return MemTypeInteraction, 0.5, nil
}

// ══════════════════════════════════════════════════════════════════════════════
// CACHE
// ══════════════════════════════════════════════════════════════════════════════

type classifierCache struct {
	mu             sync.RWMutex
	classifications map[string]classificationResult
	embeddings      map[string][]float32
	maxSize         int
}

type classificationResult struct {
	memType    MemoryType
	confidence float64
}

func newClassifierCache() *classifierCache {
	return &classifierCache{
		classifications: make(map[string]classificationResult),
		embeddings:      make(map[string][]float32),
		maxSize:         1000,
	}
}

func (c *classifierCache) get(content string) (classificationResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result, ok := c.classifications[hashContent(content)]
	return result, ok
}

func (c *classifierCache) set(content string, memType MemoryType, confidence float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple eviction: clear if too large
	if len(c.classifications) >= c.maxSize {
		c.classifications = make(map[string]classificationResult)
	}

	c.classifications[hashContent(content)] = classificationResult{
		memType:    memType,
		confidence: confidence,
	}
}

func (c *classifierCache) getEmbedding(content string) ([]float32, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	emb, ok := c.embeddings[hashContent(content)]
	return emb, ok
}

func (c *classifierCache) setEmbedding(content string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.embeddings) >= c.maxSize {
		c.embeddings = make(map[string][]float32)
	}

	c.embeddings[hashContent(content)] = embedding
}

func hashContent(content string) string {
	// Simple hash: first 100 chars + length
	if len(content) > 100 {
		return content[:100] + string(rune(len(content)))
	}
	return content
}

// ══════════════════════════════════════════════════════════════════════════════
// HELPERS
// ══════════════════════════════════════════════════════════════════════════════

func compilePatterns(patterns []string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			result = append(result, re)
		}
	}
	return result
}
