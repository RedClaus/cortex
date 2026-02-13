package memcell

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════════════════════════════════════════════════════════════
// LLM-BASED EXTRACTOR
// ══════════════════════════════════════════════════════════════════════════════

// LLMProvider is the interface for LLM calls (matches internal/llm.Provider).
type LLMProvider interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// EmbedderFunc generates embeddings for content.
type EmbedderFunc func(ctx context.Context, content string) ([]float32, error)

// LLMExtractor implements Extractor using LLM for extraction tasks.
type LLMExtractor struct {
	llm            LLMProvider
	embedder       EmbedderFunc
	classifier     *Classifier
	boundaryConfig BoundaryConfig
}

// NewLLMExtractor creates a new LLM-based extractor.
func NewLLMExtractor(llm LLMProvider, embedder EmbedderFunc, classifier *Classifier) *LLMExtractor {
	return &LLMExtractor{
		llm:            llm,
		embedder:       embedder,
		classifier:     classifier,
		boundaryConfig: DefaultBoundaryConfig(),
	}
}

// Extract creates MemCells from conversation turns.
func (e *LLMExtractor) Extract(ctx context.Context, turns []ConversationTurn) ([]MemCell, error) {
	if len(turns) == 0 {
		return nil, nil
	}

	var cells []MemCell
	var currentEpisodeID string
	var prevContent string

	for i, turn := range turns {
		// Detect event boundary
		isBoundary := false
		if i > 0 {
			isBoundary, _ = e.DetectBoundary(ctx, prevContent, turn.Content)
		}

		// Start new episode if boundary detected
		if isBoundary || currentEpisodeID == "" {
			currentEpisodeID = uuid.New().String()
		}

		// Classify the content
		memType, confidence, err := e.Classify(ctx, turn.Content)
		if err != nil {
			memType = MemTypeInteraction
			confidence = 0.5
		}

		// Extract entities
		entities, _ := e.ExtractEntities(ctx, turn.Content)

		// Create the MemCell
		cell := MemCell{
			ID:             uuid.New().String(),
			SourceID:       turn.ID,
			Version:        1,
			CreatedAt:      turn.Timestamp,
			UpdatedAt:      time.Now(),
			RawContent:     turn.Content,
			Entities:       entities,
			MemoryType:     memType,
			Confidence:     confidence,
			Scope:          ScopePersonal,
			EpisodeID:      currentEpisodeID,
			EventBoundary:  isBoundary,
			ConversationID: turn.ConversationID,
			TurnNumber:     turn.TurnNumber,
		}

		// Compute importance
		importance, _ := e.ComputeImportance(ctx, &cell)
		cell.Importance = importance

		// Generate summary for long content
		if len(turn.Content) > 500 {
			summary, _ := e.GenerateSummary(ctx, turn.Content)
			cell.Summary = summary
		}

		// Generate embedding if embedder available
		if e.embedder != nil {
			embedding, _ := e.embedder(ctx, turn.Content)
			cell.Embedding = embedding
		}

		// Extract key phrases
		cell.KeyPhrases = extractKeyPhrases(turn.Content)

		// Capture context
		if i > 0 {
			cell.PrecedingCtx = truncate(turns[i-1].Content, 200)
		}
		if i < len(turns)-1 {
			cell.FollowingCtx = truncate(turns[i+1].Content, 200)
		}

		cells = append(cells, cell)
		prevContent = turn.Content
	}

	return cells, nil
}

// Classify determines the memory type of content.
func (e *LLMExtractor) Classify(ctx context.Context, content string) (MemoryType, float64, error) {
	if e.classifier != nil {
		return e.classifier.Classify(ctx, content)
	}

	// Fallback to simple heuristic classification
	memType, confidence := classifyByPatterns(content)
	return memType, confidence, nil
}

// DetectBoundary checks if content marks an event boundary.
func (e *LLMExtractor) DetectBoundary(ctx context.Context, prev, current string) (bool, error) {
	// Check for explicit transition patterns
	lowerCurrent := strings.ToLower(current)
	for _, pattern := range e.boundaryConfig.TransitionPatterns {
		if strings.HasPrefix(lowerCurrent, pattern) || strings.Contains(lowerCurrent, " "+pattern+" ") {
			return true, nil
		}
	}

	// Check for completion signals in previous content
	completionSignals := []string{
		"that fixed it", "thanks", "thank you", "got it", "perfect",
		"that works", "solved", "done", "complete", "finished",
	}
	lowerPrev := strings.ToLower(prev)
	for _, signal := range completionSignals {
		if strings.Contains(lowerPrev, signal) {
			return true, nil
		}
	}

	// If embedder available, check semantic distance
	if e.embedder != nil {
		prevEmb, err1 := e.embedder(ctx, prev)
		currEmb, err2 := e.embedder(ctx, current)
		if err1 == nil && err2 == nil {
			distance := cosineSimilarity(prevEmb, currEmb)
			if distance < 1.0-e.boundaryConfig.EmbeddingThreshold {
				return true, nil
			}
		}
	}

	return false, nil
}

// ExtractEntities extracts named entities from content.
func (e *LLMExtractor) ExtractEntities(ctx context.Context, content string) ([]string, error) {
	// Simple pattern-based entity extraction
	entities := make(map[string]bool)

	// Extract code identifiers (CamelCase, snake_case)
	codePatterns := regexp.MustCompile(`\b([A-Z][a-z]+(?:[A-Z][a-z]+)+|[a-z]+_[a-z_]+)\b`)
	for _, match := range codePatterns.FindAllString(content, -1) {
		entities[match] = true
	}

	// Extract file paths
	pathPattern := regexp.MustCompile(`(?:\./|/|~/)[a-zA-Z0-9_/\-\.]+\.[a-zA-Z0-9]+`)
	for _, match := range pathPattern.FindAllString(content, -1) {
		entities[match] = true
	}

	// Extract URLs
	urlPattern := regexp.MustCompile(`https?://[^\s]+`)
	for _, match := range urlPattern.FindAllString(content, -1) {
		entities[match] = true
	}

	// Extract quoted strings (potential names/terms)
	quotedPattern := regexp.MustCompile(`"([^"]+)"`)
	for _, match := range quotedPattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && len(match[1]) < 50 {
			entities[match[1]] = true
		}
	}

	result := make([]string, 0, len(entities))
	for entity := range entities {
		result = append(result, entity)
	}
	return result, nil
}

// ComputeImportance calculates importance score.
func (e *LLMExtractor) ComputeImportance(ctx context.Context, cell *MemCell) (float64, error) {
	importance := 0.5 // Base importance

	// Type-based importance boost
	switch cell.MemoryType {
	case MemTypePrinciple, MemTypeLesson, MemTypeGoal:
		importance += 0.3 // Strategic memories are highly important
	case MemTypePreference, MemTypeProfile:
		importance += 0.2 // Personal memories are important
	case MemTypeFact, MemTypeKnowledge:
		importance += 0.15 // Semantic memories moderately important
	case MemTypeProcedure:
		importance += 0.1 // Procedures useful but not critical
	}

	// Entity richness boost
	if len(cell.Entities) > 3 {
		importance += 0.1
	}

	// Length consideration (very short or very long = less important)
	contentLen := len(cell.RawContent)
	if contentLen > 100 && contentLen < 2000 {
		importance += 0.05
	}

	// Event boundary = more important (marks topic change)
	if cell.EventBoundary {
		importance += 0.1
	}

	// Cap at 1.0
	if importance > 1.0 {
		importance = 1.0
	}

	return importance, nil
}

// GenerateSummary creates a compressed summary.
func (e *LLMExtractor) GenerateSummary(ctx context.Context, content string) (string, error) {
	if e.llm == nil {
		// Fallback: first 200 chars + ellipsis
		return truncate(content, 200), nil
	}

	prompt := `Summarize the following text in 1-2 concise sentences. Focus on the key information and actionable points.

Text:
` + content + `

Summary:`

	summary, err := e.llm.Complete(ctx, prompt)
	if err != nil {
		return truncate(content, 200), nil
	}

	return strings.TrimSpace(summary), nil
}

// ══════════════════════════════════════════════════════════════════════════════
// SIMPLE EXTRACTOR (NO LLM REQUIRED)
// ══════════════════════════════════════════════════════════════════════════════

// SimpleExtractor provides basic extraction without LLM dependencies.
type SimpleExtractor struct {
	embedder       EmbedderFunc
	boundaryConfig BoundaryConfig
}

// NewSimpleExtractor creates a simple pattern-based extractor.
func NewSimpleExtractor(embedder EmbedderFunc) *SimpleExtractor {
	return &SimpleExtractor{
		embedder:       embedder,
		boundaryConfig: DefaultBoundaryConfig(),
	}
}

// Extract creates MemCells from conversation turns using pattern-based analysis.
func (e *SimpleExtractor) Extract(ctx context.Context, turns []ConversationTurn) ([]MemCell, error) {
	if len(turns) == 0 {
		return nil, nil
	}

	var cells []MemCell
	var currentEpisodeID string
	var prevContent string

	for i, turn := range turns {
		// Detect boundary
		isBoundary := false
		if i > 0 {
			isBoundary, _ = e.DetectBoundary(ctx, prevContent, turn.Content)
		}

		if isBoundary || currentEpisodeID == "" {
			currentEpisodeID = uuid.New().String()
		}

		// Pattern-based classification
		memType, confidence := classifyByPatterns(turn.Content)

		// Extract entities
		entities, _ := e.ExtractEntities(ctx, turn.Content)

		cell := MemCell{
			ID:             uuid.New().String(),
			SourceID:       turn.ID,
			Version:        1,
			CreatedAt:      turn.Timestamp,
			UpdatedAt:      time.Now(),
			RawContent:     turn.Content,
			Entities:       entities,
			KeyPhrases:     extractKeyPhrases(turn.Content),
			MemoryType:     memType,
			Confidence:     confidence,
			Scope:          ScopePersonal,
			EpisodeID:      currentEpisodeID,
			EventBoundary:  isBoundary,
			ConversationID: turn.ConversationID,
			TurnNumber:     turn.TurnNumber,
		}

		// Simple importance calculation
		cell.Importance = calculateSimpleImportance(&cell)

		// Summary for long content
		if len(turn.Content) > 500 {
			cell.Summary = truncate(turn.Content, 200)
		}

		// Embedding
		if e.embedder != nil {
			embedding, _ := e.embedder(ctx, turn.Content)
			cell.Embedding = embedding
		}

		// Context capture
		if i > 0 {
			cell.PrecedingCtx = truncate(turns[i-1].Content, 200)
		}
		if i < len(turns)-1 {
			cell.FollowingCtx = truncate(turns[i+1].Content, 200)
		}

		cells = append(cells, cell)
		prevContent = turn.Content
	}

	return cells, nil
}

// Classify determines memory type using patterns.
func (e *SimpleExtractor) Classify(ctx context.Context, content string) (MemoryType, float64, error) {
	memType, confidence := classifyByPatterns(content)
	return memType, confidence, nil
}

// DetectBoundary checks for event boundaries.
func (e *SimpleExtractor) DetectBoundary(ctx context.Context, prev, current string) (bool, error) {
	// Check transition patterns
	lowerCurrent := strings.ToLower(current)
	for _, pattern := range e.boundaryConfig.TransitionPatterns {
		if strings.HasPrefix(lowerCurrent, pattern) {
			return true, nil
		}
	}

	// Check completion signals
	lowerPrev := strings.ToLower(prev)
	completionSignals := []string{"thanks", "thank you", "that works", "perfect", "done"}
	for _, signal := range completionSignals {
		if strings.Contains(lowerPrev, signal) {
			return true, nil
		}
	}

	return false, nil
}

// ExtractEntities extracts entities using patterns.
func (e *SimpleExtractor) ExtractEntities(ctx context.Context, content string) ([]string, error) {
	entities := make(map[string]bool)

	// Code identifiers
	codePatterns := regexp.MustCompile(`\b([A-Z][a-z]+(?:[A-Z][a-z]+)+|[a-z]+_[a-z_]+)\b`)
	for _, match := range codePatterns.FindAllString(content, -1) {
		entities[match] = true
	}

	// File paths
	pathPattern := regexp.MustCompile(`(?:\./|/|~/)[a-zA-Z0-9_/\-\.]+`)
	for _, match := range pathPattern.FindAllString(content, -1) {
		entities[match] = true
	}

	result := make([]string, 0, len(entities))
	for entity := range entities {
		result = append(result, entity)
	}
	return result, nil
}

// ComputeImportance calculates importance score.
func (e *SimpleExtractor) ComputeImportance(ctx context.Context, cell *MemCell) (float64, error) {
	return calculateSimpleImportance(cell), nil
}

// GenerateSummary creates a truncated summary.
func (e *SimpleExtractor) GenerateSummary(ctx context.Context, content string) (string, error) {
	return truncate(content, 200), nil
}

// ══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ══════════════════════════════════════════════════════════════════════════════

// classifyByPatterns provides pattern-based classification.
func classifyByPatterns(content string) (MemoryType, float64) {
	lower := strings.ToLower(content)

	// Strategic patterns
	principlePatterns := []string{
		"always", "never", "important to", "rule:", "principle:",
		"best practice", "should always", "must always",
	}
	for _, p := range principlePatterns {
		if strings.Contains(lower, p) {
			return MemTypePrinciple, 0.7
		}
	}

	// Lesson patterns
	lessonPatterns := []string{
		"learned that", "realized", "mistake was", "lesson:",
		"from now on", "next time", "won't forget",
	}
	for _, p := range lessonPatterns {
		if strings.Contains(lower, p) {
			return MemTypeLesson, 0.7
		}
	}

	// Goal patterns
	goalPatterns := []string{
		"want to", "need to", "goal:", "objective:",
		"planning to", "aim to", "intend to",
	}
	for _, p := range goalPatterns {
		if strings.Contains(lower, p) {
			return MemTypeGoal, 0.6
		}
	}

	// Preference patterns
	preferencePatterns := []string{
		"i prefer", "i like", "i don't like", "i hate",
		"my favorite", "i usually", "i always use",
	}
	for _, p := range preferencePatterns {
		if strings.Contains(lower, p) {
			return MemTypePreference, 0.7
		}
	}

	// Procedure patterns (code-related)
	procedurePatterns := []string{
		"how to", "step 1", "first,", "then,", "finally,",
		"to do this", "here's how", "the way to",
	}
	for _, p := range procedurePatterns {
		if strings.Contains(lower, p) {
			return MemTypeProcedure, 0.6
		}
	}

	// Knowledge/fact patterns
	factPatterns := []string{
		"is a", "are the", "means that", "defined as",
		"because", "since", "therefore",
	}
	for _, p := range factPatterns {
		if strings.Contains(lower, p) {
			return MemTypeKnowledge, 0.5
		}
	}

	// Code detection
	codeIndicators := []string{
		"```", "func ", "function ", "class ", "def ",
		"import ", "const ", "var ", "let ",
	}
	for _, p := range codeIndicators {
		if strings.Contains(content, p) {
			return MemTypeProcedure, 0.6
		}
	}

	// Default to interaction
	return MemTypeInteraction, 0.4
}

// extractKeyPhrases extracts important phrases from content.
func extractKeyPhrases(content string) []string {
	phrases := make(map[string]bool)

	// Extract capitalized phrases (potential proper nouns/terms)
	capPattern := regexp.MustCompile(`\b([A-Z][a-z]+(?: [A-Z][a-z]+)+)\b`)
	for _, match := range capPattern.FindAllString(content, 10) {
		phrases[match] = true
	}

	// Extract code-like terms
	codePattern := regexp.MustCompile(`\b(func|function|class|struct|interface|type)\s+(\w+)`)
	for _, match := range codePattern.FindAllStringSubmatch(content, 10) {
		if len(match) > 2 {
			phrases[match[2]] = true
		}
	}

	result := make([]string, 0, len(phrases))
	for phrase := range phrases {
		result = append(result, phrase)
	}
	return result
}

// calculateSimpleImportance computes importance without LLM.
func calculateSimpleImportance(cell *MemCell) float64 {
	importance := 0.5

	switch cell.MemoryType {
	case MemTypePrinciple, MemTypeLesson:
		importance += 0.3
	case MemTypeGoal, MemTypePreference:
		importance += 0.2
	case MemTypeProcedure, MemTypeKnowledge:
		importance += 0.1
	}

	if len(cell.Entities) > 3 {
		importance += 0.1
	}

	if cell.EventBoundary {
		importance += 0.05
	}

	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}

// truncate shortens content to maxLen characters.
func truncate(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt approximation for float64.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
