package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/ingestion/parsers"
	"github.com/normanking/cortex/pkg/types"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CHUNKER
// ═══════════════════════════════════════════════════════════════════════════════

// Chunker splits documents into retrievable chunks based on semantic boundaries.
type Chunker struct {
	maxTokens int
	overlap   int
	minTokens int
}

// NewChunker creates a new semantic chunker.
func NewChunker(maxTokens, overlap, minTokens int) *Chunker {
	return &Chunker{
		maxTokens: maxTokens,
		overlap:   overlap,
		minTokens: minTokens,
	}
}

// Chunk splits a parsed document into chunks.
func (c *Chunker) Chunk(doc *parsers.ParsedDocument, sourceID string) ([]*Chunk, error) {
	var chunks []*Chunk
	position := 0

	for _, section := range doc.Sections {
		sectionChunks := c.chunkSection(section, doc.Title, sourceID, nil, 0, &position)
		chunks = append(chunks, sectionChunks...)
	}

	return chunks, nil
}

// chunkSection recursively chunks a section and its subsections.
func (c *Chunker) chunkSection(
	section *parsers.Section,
	docTitle string,
	sourceID string,
	parentID *string,
	depth int,
	position *int,
) []*Chunk {
	var chunks []*Chunk

	// Build section path
	sectionPath := docTitle
	if section.Title != "" {
		sectionPath += " > " + section.Title
	}

	// Process section content
	if section.Content != "" {
		contentChunks := c.splitContent(
			section.Content,
			section.Title,
			sectionPath,
			sourceID,
			parentID,
			depth,
			position,
		)
		chunks = append(chunks, contentChunks...)
	}

	// Process code blocks separately (don't split them)
	for _, codeBlock := range section.CodeBlocks {
		chunk := &Chunk{
			ID:          generateChunkID(),
			SourceID:    sourceID,
			Content:     codeBlock.Code,
			ContentType: "code",
			ContentHash: hashContent(codeBlock.Code),
			Title:       section.Title,
			SectionPath: sectionPath,
			ParentChunkID: parentID,
			Position:    *position,
			Depth:       depth,
			TokenCount:  types.EstimateTokens(codeBlock.Code),
			QualityScore: 1.0, // Code blocks are high quality
			CreatedAt:   time.Now(),
		}
		chunks = append(chunks, chunk)
		*position++
	}

	// Process subsections recursively
	for _, subsection := range section.Subsections {
		var subParentID *string
		if len(chunks) > 0 {
			subParentID = &chunks[0].ID
		}
		subChunks := c.chunkSection(subsection, docTitle, sourceID, subParentID, depth+1, position)
		chunks = append(chunks, subChunks...)
	}

	return chunks
}

// splitContent splits text content into chunks at paragraph boundaries.
func (c *Chunker) splitContent(
	content string,
	title string,
	sectionPath string,
	sourceID string,
	parentID *string,
	depth int,
	position *int,
) []*Chunk {
	var chunks []*Chunk

	// Split by natural boundaries (paragraphs)
	paragraphs := splitIntoParagraphs(content)

	var currentChunk strings.Builder
	currentTokens := 0

	for _, para := range paragraphs {
		paraTokens := types.EstimateTokens(para)

		// If single paragraph exceeds max, split it further
		if paraTokens > c.maxTokens {
			// Flush current chunk first
			if currentChunk.Len() > 0 {
				chunks = append(chunks, c.createChunk(
					currentChunk.String(),
					title,
					sectionPath,
					sourceID,
					parentID,
					depth,
					position,
				))
				currentChunk.Reset()
				currentTokens = 0
			}

			// Split long paragraph
			subChunks := c.splitLongText(para, title, sectionPath, sourceID, parentID, depth, position)
			chunks = append(chunks, subChunks...)
			continue
		}

		// If adding this paragraph exceeds max, start new chunk
		if currentTokens+paraTokens > c.maxTokens {
			chunks = append(chunks, c.createChunk(
				currentChunk.String(),
				title,
				sectionPath,
				sourceID,
				parentID,
				depth,
				position,
			))

			// Start new chunk with overlap
			currentChunk.Reset()
			currentTokens = 0

			// Add overlap from previous content
			if c.overlap > 0 && len(chunks) > 0 {
				overlapText := getOverlapText(chunks[len(chunks)-1].Content, c.overlap)
				currentChunk.WriteString(overlapText)
				currentChunk.WriteString("\n\n")
				currentTokens = types.EstimateTokens(overlapText)
			}
		}

		currentChunk.WriteString(para)
		currentChunk.WriteString("\n\n")
		currentTokens += paraTokens
	}

	// Flush remaining content
	if currentChunk.Len() > 0 && currentTokens >= c.minTokens {
		chunks = append(chunks, c.createChunk(
			currentChunk.String(),
			title,
			sectionPath,
			sourceID,
			parentID,
			depth,
			position,
		))
	}

	return chunks
}

// splitLongText splits very long text at sentence boundaries.
func (c *Chunker) splitLongText(
	text string,
	title string,
	sectionPath string,
	sourceID string,
	parentID *string,
	depth int,
	position *int,
) []*Chunk {
	var chunks []*Chunk

	sentences := splitIntoSentences(text)
	var currentChunk strings.Builder
	currentTokens := 0

	for _, sentence := range sentences {
		sentenceTokens := types.EstimateTokens(sentence)

		if currentTokens+sentenceTokens > c.maxTokens && currentChunk.Len() > 0 {
			chunks = append(chunks, c.createChunk(
				currentChunk.String(),
				title,
				sectionPath,
				sourceID,
				parentID,
				depth,
				position,
			))
			currentChunk.Reset()
			currentTokens = 0
		}

		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")
		currentTokens += sentenceTokens
	}

	if currentChunk.Len() > 0 && currentTokens >= c.minTokens {
		chunks = append(chunks, c.createChunk(
			currentChunk.String(),
			title,
			sectionPath,
			sourceID,
			parentID,
			depth,
			position,
		))
	}

	return chunks
}

// createChunk creates a chunk from accumulated content.
func (c *Chunker) createChunk(
	content string,
	title string,
	sectionPath string,
	sourceID string,
	parentID *string,
	depth int,
	position *int,
) *Chunk {
	content = strings.TrimSpace(content)
	chunk := &Chunk{
		ID:            generateChunkID(),
		SourceID:      sourceID,
		Content:       content,
		ContentType:   detectContentType(content),
		ContentHash:   hashContent(content),
		Title:         title,
		SectionPath:   sectionPath,
		ParentChunkID: parentID,
		Position:      *position,
		Depth:         depth,
		TokenCount:    types.EstimateTokens(content),
		QualityScore:  1.0, // Will be adjusted later
		CreatedAt:     time.Now(),
	}
	*position++
	return chunk
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// generateChunkID generates a unique chunk ID.
func generateChunkID() string {
	return fmt.Sprintf("chunk_%s", uuid.New().String()[:8])
}

// hashContent generates a SHA256 hash of content for CAS.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}


// splitIntoParagraphs splits text on double newlines.
func splitIntoParagraphs(text string) []string {
	// Split on double newlines
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)

	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitIntoSentences splits text at sentence boundaries.
func splitIntoSentences(text string) []string {
	// Simple sentence splitting (can be improved with NLP)
	re := regexp.MustCompile(`[.!?]+\s+`)
	sentences := re.Split(text, -1)

	var result []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// detectContentType detects the type of content.
func detectContentType(content string) string {
	// Check if it's primarily commands
	if strings.Count(content, "```") >= 2 || strings.HasPrefix(strings.TrimSpace(content), "$") {
		return "command"
	}

	// Check if it's a definition/explanation
	if strings.Contains(content, "Purpose:") || strings.Contains(content, "Definition:") {
		return "definition"
	}

	// Check if it's an example
	if strings.Contains(strings.ToLower(content), "example") || strings.Contains(content, "e.g.") {
		return "example"
	}

	// Check if mixed (has both text and code)
	if strings.Contains(content, "```") {
		return "mixed"
	}

	return "text"
}

// getOverlapText gets the last N tokens worth of text for overlap.
func getOverlapText(content string, targetTokens int) string {
	words := strings.Fields(content)
	targetWords := targetTokens // Rough: 1 word ≈ 1 token

	if len(words) <= targetWords {
		return content
	}

	return strings.Join(words[len(words)-targetWords:], " ")
}
