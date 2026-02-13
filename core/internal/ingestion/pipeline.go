package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/normanking/cortex/internal/ingestion/parsers"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PIPELINE
// ═══════════════════════════════════════════════════════════════════════════════

// Pipeline handles knowledge ingestion from documents.
type Pipeline struct {
	embedder *Embedder
	chunker  *Chunker
	config   *PipelineConfig
}

// NewPipeline creates a new ingestion pipeline.
func NewPipeline(embedder *Embedder, config *PipelineConfig) *Pipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}

	return &Pipeline{
		embedder: embedder,
		chunker: NewChunker(
			config.MaxChunkSize,
			config.ChunkOverlap,
			config.MinChunkSize,
		),
		config: config,
	}
}

// Ingest processes a knowledge source and returns chunks ready for storage.
func (p *Pipeline) Ingest(ctx context.Context, req *IngestionRequest) (*IngestionResult, error) {
	startTime := time.Now()
	result := &IngestionResult{
		EmbeddingModel: p.config.EmbeddingModel,
	}

	// 1. Load content
	content, format, err := p.loadContent(req)
	if err != nil {
		return nil, fmt.Errorf("load content: %w", err)
	}

	// 2. Generate content hash for source-level deduplication
	_ = hashSourceContent(content) // TODO: Use for deduplication in store

	// 3. Parse content into structured document
	doc, err := p.parseContent(content, format, req.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("parse content: %w", err)
	}

	// 4. Generate source ID
	sourceID := generateSourceID(req.Name)
	result.SourceID = sourceID

	// 5. Chunk the document
	chunks, err := p.chunker.Chunk(doc, sourceID)
	if err != nil {
		return nil, fmt.Errorf("chunk document: %w", err)
	}

	// 6. Extract entities (commands, keywords)
	if p.config.ExtractCommands || p.config.ExtractKeywords {
		p.extractEntities(chunks)
	}

	// 7. Generate embeddings for chunks (if embedder is available)
	// Embedding failures are non-fatal - we can still use keyword search
	if p.embedder != nil && p.embedder.Available() {
		if err := p.embedder.EmbedChunks(ctx, chunks, p.config.BatchSize); err != nil {
			// Log warning but continue - keyword search will still work
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Embedding failed (using keyword search only): %v", err))
		}
	} else if p.embedder == nil {
		result.Warnings = append(result.Warnings, "No embedder configured - using keyword search only")
	} else {
		result.Warnings = append(result.Warnings, "Embedder not available - using keyword search only")
	}

	// 8. Score chunk quality
	p.scoreChunkQuality(chunks)

	// 9. Filter low-quality chunks
	qualityChunks := filterByQuality(chunks, p.config.MinQualityScore)
	skipped := len(chunks) - len(qualityChunks)
	if skipped > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Filtered %d low-quality chunks (< %.2f quality score)", skipped, p.config.MinQualityScore))
	}

	result.ChunksCreated = len(qualityChunks)
	result.Duration = time.Since(startTime)
	result.QualityScore = averageQualityScore(qualityChunks)

	return result, nil
}

// IngestFile ingests from a file path.
func (p *Pipeline) IngestFile(ctx context.Context, filePath string, opts *IngestionOptions) (*IngestionResult, []* Chunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read file: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = filepath.Base(filePath)
	}

	req := &IngestionRequest{
		Name:        name,
		Description: opts.Description,
		SourceType:  "file",
		SourcePath:  filePath,
		Content:     string(content),
		Category:    opts.Category,
		Tags:        opts.Tags,
		Platform:    opts.Platform,
	}

	result, err := p.Ingest(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Re-process to get chunks (this is a simplified version - in production you'd store chunks)
	format := detectFormatFromPath(filePath)
	doc, _ := p.parseContent(string(content), format, filePath)
	chunks, _ := p.chunker.Chunk(doc, result.SourceID)

	// Extract entities and embed
	if p.config.ExtractCommands || p.config.ExtractKeywords {
		p.extractEntities(chunks)
	}
	if p.embedder != nil && p.embedder.Available() {
		_ = p.embedder.EmbedChunks(ctx, chunks, p.config.BatchSize)
	}
	p.scoreChunkQuality(chunks)
	qualityChunks := filterByQuality(chunks, p.config.MinQualityScore)

	return result, qualityChunks, nil
}

// IngestDirectory ingests all supported files in a directory.
func (p *Pipeline) IngestDirectory(ctx context.Context, dirPath string, opts *IngestionOptions) ([]*IngestionResult, error) {
	var results []*IngestionResult

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if supported format
		if !isSupportedFormat(filepath.Ext(path)) {
			return nil
		}

		fileOpts := *opts
		if fileOpts.Name == "" {
			fileOpts.Name = info.Name()
		}

		result, _, err := p.IngestFile(ctx, path, &fileOpts)
		if err != nil {
			// Log warning but continue
			results = append(results, &IngestionResult{
				Warnings: []string{fmt.Sprintf("Failed to ingest %s: %v", path, err)},
			})
			return nil
		}

		results = append(results, result)
		return nil
	})

	return results, err
}

// ═══════════════════════════════════════════════════════════════════════════════
// HELPER METHODS
// ═══════════════════════════════════════════════════════════════════════════════

// loadContent loads content from request.
func (p *Pipeline) loadContent(req *IngestionRequest) (string, string, error) {
	if req.Content != "" {
		// Direct content provided
		format := req.Format
		if format == "" {
			format = detectFormat(req.Content, req.SourcePath)
		}
		return req.Content, format, nil
	}

	if req.SourcePath != "" {
		// Load from file
		content, err := os.ReadFile(req.SourcePath)
		if err != nil {
			return "", "", err
		}
		format := req.Format
		if format == "" {
			format = detectFormatFromPath(req.SourcePath)
		}
		return string(content), format, nil
	}

	return "", "", fmt.Errorf("no content or source path provided")
}

// parseContent parses content based on format.
func (p *Pipeline) parseContent(content, format, path string) (*parsers.ParsedDocument, error) {
	var parser interface{ Parse(string) (*parsers.ParsedDocument, error) }

	switch format {
	case "markdown":
		parser = parsers.NewMarkdownParser()
	case "text":
		parser = parsers.NewTextParser()
	case "code":
		codeParser := parsers.NewCodeParser()
		if path != "" {
			return codeParser.ParseWithPath(content, path)
		}
		parser = codeParser
	default:
		// Default to text parser
		parser = parsers.NewTextParser()
	}

	return parser.Parse(content)
}

// extractEntities extracts commands and keywords from chunks.
func (p *Pipeline) extractEntities(chunks []*Chunk) {
	for _, chunk := range chunks {
		if p.config.ExtractCommands {
			chunk.Commands = ExtractCommands(chunk.Content)
		}
		if p.config.ExtractKeywords {
			chunk.Keywords = ExtractKeywords(chunk.Content)
		}
	}
}

// scoreChunkQuality scores all chunks.
func (p *Pipeline) scoreChunkQuality(chunks []*Chunk) {
	for _, chunk := range chunks {
		ScoreChunkQuality(chunk)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// UTILITY FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════════

// hashSourceContent generates a SHA256 hash of source content.
func hashSourceContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// generateSourceID generates a unique source ID.
func generateSourceID(name string) string {
	return fmt.Sprintf("src_%s", uuid.New().String()[:8])
}

// detectFormat detects format from content and path.
func detectFormat(content, path string) string {
	if path != "" {
		if format := detectFormatFromPath(path); format != "" {
			return format
		}
	}

	// Detect from content
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "#") || strings.Contains(content, "```") {
		return "markdown"
	}
	if strings.HasPrefix(content, "{") || strings.HasPrefix(content, "[") {
		return "json"
	}
	if strings.Contains(content, "---") && strings.Contains(content, ":") {
		return "yaml"
	}

	return "text"
}

// detectFormatFromPath detects format from file extension.
func detectFormatFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "text"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb":
		return "code"
	case ".sh", ".bash", ".zsh", ".fish":
		return "code"
	default:
		return "text"
	}
}

// isSupportedFormat checks if file extension is supported.
func isSupportedFormat(ext string) bool {
	ext = strings.ToLower(ext)
	supported := map[string]bool{
		".md": true, ".markdown": true,
		".txt": true,
		".json": true,
		".yaml": true, ".yml": true,
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".c": true, ".cpp": true, ".rs": true,
		".rb": true, ".sh": true, ".bash": true,
	}
	return supported[ext]
}

// filterByQuality filters chunks by minimum quality score.
func filterByQuality(chunks []*Chunk, minScore float64) []*Chunk {
	var filtered []*Chunk
	for _, chunk := range chunks {
		if chunk.QualityScore >= minScore {
			filtered = append(filtered, chunk)
		}
	}
	return filtered
}

// averageQualityScore calculates average quality score.
func averageQualityScore(chunks []*Chunk) float64 {
	if len(chunks) == 0 {
		return 0
	}
	var sum float64
	for _, chunk := range chunks {
		sum += chunk.QualityScore
	}
	return sum / float64(len(chunks))
}
