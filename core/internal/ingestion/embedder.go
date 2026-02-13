package ingestion

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/normanking/cortex/internal/cognitive/router"
)

// ═══════════════════════════════════════════════════════════════════════════════
// EMBEDDER WRAPPER
// ═══════════════════════════════════════════════════════════════════════════════

// Embedder wraps the router.Embedder for chunk embedding generation.
type Embedder struct {
	embedder router.Embedder
	model    string
	dim      int
}

// NewEmbedder creates a new chunk embedder.
func NewEmbedder(embedder router.Embedder) *Embedder {
	return &Embedder{
		embedder: embedder,
		model:    embedder.ModelName(),
		dim:      embedder.Dimension(),
	}
}

// EmbedChunks generates embeddings for multiple chunks in batch.
func (e *Embedder) EmbedChunks(ctx context.Context, chunks []*Chunk, batchSize int) error {
	if !e.embedder.Available() {
		return fmt.Errorf("embedder not available")
	}

	// Process in batches
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		if err := e.embedBatch(ctx, batch); err != nil {
			return fmt.Errorf("embed batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// embedBatch embeds a single batch of chunks.
func (e *Embedder) embedBatch(ctx context.Context, chunks []*Chunk) error {
	// Extract text content
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Content
	}

	// Generate embeddings
	embeddings, err := e.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}

	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(chunks))
	}

	// Assign embeddings to chunks
	for i, embedding := range embeddings {
		chunks[i].Embedding = embedding
		chunks[i].EmbeddingModel = e.model
	}

	return nil
}

// Model returns the embedding model name.
func (e *Embedder) Model() string {
	return e.model
}

// Dimension returns the embedding dimension.
func (e *Embedder) Dimension() int {
	return e.dim
}

// Available returns true if the embedder is ready to use.
func (e *Embedder) Available() bool {
	return e.embedder.Available()
}

// ═══════════════════════════════════════════════════════════════════════════════
// ENTITY EXTRACTION
// ═══════════════════════════════════════════════════════════════════════════════

// ExtractCommands extracts shell commands from chunk content.
func ExtractCommands(content string) []string {
	var commands []string

	// Pattern 1: Inline commands with $ prefix
	// Example: $ sudo sysdiagnose
	inlinePattern := `\$\s+([^\n]+)`
	matches := findAllMatches(content, inlinePattern)
	for _, match := range matches {
		if len(match) > 1 {
			commands = append(commands, match[1])
		}
	}

	// Pattern 2: Code blocks with bash/sh
	// Example: ```bash\nsudo sysdiagnose\n```
	codeBlockPattern := "```(?:bash|sh|shell|zsh)\\s+([^`]+)```"
	matches = findAllMatches(content, codeBlockPattern)
	for _, match := range matches {
		if len(match) > 1 {
			// Split multi-line commands
			lines := splitNonEmpty(match[1], "\n")
			for _, line := range lines {
				line = trimSpaces(line)
				if line != "" && !isComment(line) {
					commands = append(commands, line)
				}
			}
		}
	}

	return deduplicate(commands)
}

// ExtractKeywords extracts important keywords from content.
func ExtractKeywords(content string) []string {
	var keywords []string

	// Extract technical terms (capitalized words, acronyms)
	// Example: VLAN, macOS, sysdiagnose
	techPattern := `\b[A-Z][A-Za-z0-9]{2,}\b`
	matches := findAllMatches(content, techPattern)
	for _, match := range matches {
		if len(match) > 0 {
			keywords = append(keywords, match[0])
		}
	}

	// Extract common command names (lowercase with no spaces)
	// Example: sysdiagnose, ifconfig, grep
	cmdPattern := `\b[a-z][a-z0-9_-]{3,}\b`
	matches = findAllMatches(content, cmdPattern)
	for _, match := range matches {
		if len(match) > 0 && !isStopword(match[0]) {
			keywords = append(keywords, match[0])
		}
	}

	return deduplicate(keywords)
}

// ═══════════════════════════════════════════════════════════════════════════════
// QUALITY SCORING
// ═══════════════════════════════════════════════════════════════════════════════

// ScoreChunkQuality assigns a quality score to a chunk.
func ScoreChunkQuality(chunk *Chunk) {
	score := 1.0

	// Penalize very short chunks
	if chunk.TokenCount < 20 {
		score -= 0.3
	}

	// Penalize chunks that are mostly whitespace
	contentRatio := float64(len(trimSpaces(chunk.Content))) / float64(len(chunk.Content))
	if contentRatio < 0.5 {
		score -= 0.2
	}

	// Boost chunks with code blocks (high signal)
	if containsCodeBlock(chunk.Content) {
		score += 0.1
	}

	// Boost chunks with commands
	if len(chunk.Commands) > 0 {
		score += 0.1
	}

	// Boost chunks with multiple keywords (information density)
	if len(chunk.Keywords) >= 5 {
		score += 0.1
	}

	// Clamp to [0, 1]
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	chunk.QualityScore = score
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// findAllMatches finds all regex matches in content.
func findAllMatches(content, pattern string) [][]string {
	re := regexp.MustCompile(pattern)
	return re.FindAllStringSubmatch(content, -1)
}

// splitNonEmpty splits a string and removes empty entries.
func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, part := range parts {
		if part := trimSpaces(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

// trimSpaces trims whitespace from a string.
func trimSpaces(s string) string {
	return strings.TrimSpace(s)
}

// isComment checks if a line is a comment.
func isComment(line string) bool {
	line = trimSpaces(line)
	return strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//")
}

// isStopword checks if a word is a common stopword.
func isStopword(word string) bool {
	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"from": true, "this": true, "that": true, "which": true,
		"have": true, "has": true, "will": true, "would": true,
		"can": true, "could": true, "should": true, "may": true,
	}
	return stopwords[word]
}

// deduplicate removes duplicates from a slice.
func deduplicate(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// containsCodeBlock checks if content contains a code block.
func containsCodeBlock(content string) bool {
	return strings.Contains(content, "```")
}
