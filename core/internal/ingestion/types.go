// Package ingestion provides multi-format document ingestion capabilities for
// the Cortex knowledge base. It supports parsing, chunking, embedding, and
// storing documents from various sources.
package ingestion

import (
	"time"

	"github.com/normanking/cortex/internal/cognitive"
)

// ═══════════════════════════════════════════════════════════════════════════════
// CORE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// Document represents an ingested source document.
type Document struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	SourceType  string            `json:"source_type"` // file, url, api, manual
	SourcePath  string            `json:"source_path,omitempty"`
	Format      string            `json:"format"` // markdown, text, code, pdf, json, yaml
	Content     string            `json:"content"`
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Platform    string            `json:"platform,omitempty"` // macos, linux, windows, all
	Version     string            `json:"version,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Chunk represents a single retrievable unit of knowledge.
type Chunk struct {
	ID             string              `json:"id"`
	SourceID       string              `json:"source_id"`
	Content        string              `json:"content"`
	ContentType    string              `json:"content_type"` // text, code, command, example, definition, mixed
	ContentHash    string              `json:"content_hash"` // SHA256 for CAS deduplication
	ParentChunkID  *string             `json:"parent_chunk_id,omitempty"`
	Position       int                 `json:"position"`
	Depth          int                 `json:"depth"`
	Title          string              `json:"title,omitempty"`
	SectionPath    string              `json:"section_path,omitempty"`
	Embedding      cognitive.Embedding `json:"embedding,omitempty"`
	EmbeddingModel string              `json:"embedding_model"`
	Commands       []string            `json:"commands,omitempty"`
	Keywords       []string            `json:"keywords,omitempty"`
	StartOffset    int                 `json:"start_offset"`
	EndOffset      int                 `json:"end_offset"`
	TokenCount     int                 `json:"token_count"`
	QualityScore   float64             `json:"quality_score"`
	CreatedAt      time.Time           `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// REQUEST/RESPONSE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// IngestionRequest represents a request to ingest knowledge.
type IngestionRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	SourceType  string            `json:"source_type"` // file, url, api, manual
	SourcePath  string            `json:"source_path,omitempty"`
	Content     string            `json:"content,omitempty"` // direct content for manual ingestion
	Format      string            `json:"format,omitempty"`  // auto-detected if not provided
	Category    string            `json:"category,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Platform    string            `json:"platform,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// IngestionResult represents the result of ingestion.
type IngestionResult struct {
	SourceID       string        `json:"source_id"`
	ChunksCreated  int           `json:"chunks_created"`
	ChunksSkipped  int           `json:"chunks_skipped"` // CAS hits
	Warnings       []string      `json:"warnings,omitempty"`
	Duration       time.Duration `json:"duration"`
	QualityScore   float64       `json:"quality_score"`
	EmbeddingModel string        `json:"embedding_model"`
}

// IngestionOptions provides fine-grained control over ingestion.
type IngestionOptions struct {
	Name            string
	Description     string
	Category        string
	Tags            []string
	Platform        string
	MaxChunkSize    int     // Max tokens per chunk (default: 512)
	ChunkOverlap    int     // Overlap between chunks (default: 50)
	MinChunkSize    int     // Min tokens per chunk (default: 50)
	ExtractCommands bool    // Extract command entities (default: true)
	ExtractKeywords bool    // Extract keywords for FTS (default: true)
	MinQualityScore float64 // Minimum quality score to accept chunk (default: 0.5)
}

// Note: ParsedDocument, Section, CodeBlock, and Metadata types are defined
// in the parsers package to avoid import cycles.

// ═══════════════════════════════════════════════════════════════════════════════
// RETRIEVAL TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// RetrievalResult represents knowledge retrieval matches.
type RetrievalResult struct {
	Chunks      []*ChunkResult `json:"chunks"`
	TotalTokens int            `json:"total_tokens"`
	LatencyMs   int64          `json:"latency_ms"`
}

// ChunkResult wraps a chunk with its relevance score.
type ChunkResult struct {
	Chunk      *Chunk   `json:"chunk"`
	Similarity float64  `json:"similarity"` // 0.0 - 1.0
	MatchType  string   `json:"match_type"` // vector, keyword, hybrid
	Highlights []string `json:"highlights,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// CONFIGURATION TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// PipelineConfig configures the ingestion pipeline.
type PipelineConfig struct {
	MaxChunkSize     int     // Max tokens per chunk (default: 512)
	ChunkOverlap     int     // Overlap between chunks (default: 50)
	MinChunkSize     int     // Min tokens per chunk (default: 50)
	ExtractCommands  bool    // Extract command entities
	ExtractKeywords  bool    // Extract keywords for FTS
	BatchSize        int     // Embedding batch size
	EmbeddingModel   string  // Current embedding model ID
	MinQualityScore  float64 // Minimum quality score to accept chunk (0-1)
	EnableCleaning   bool    // Use Frontier model to clean noisy docs
	CleaningModel    string  // Which model to use for cleaning
}

// DefaultPipelineConfig returns default configuration.
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		MaxChunkSize:     512,
		ChunkOverlap:     50,
		MinChunkSize:     50,
		ExtractCommands:  true,
		ExtractKeywords:  true,
		BatchSize:        32,
		EmbeddingModel:   "nomic-embed-text",
		MinQualityScore:  0.5,
		EnableCleaning:   false, // Disabled by default (costs API calls)
		CleaningModel:    "claude-sonnet-4-20250514",
	}
}

// RetrieverConfig configures the knowledge retriever.
type RetrieverConfig struct {
	MaxResults      int     // Maximum results to return
	MinSimilarity   float64 // Minimum similarity threshold
	VectorWeight    float64 // Weight for vector search (0-1)
	KeywordWeight   float64 // Weight for keyword search (0-1)
	MaxTokens       int     // Max total tokens to return
	DiversityFactor float64 // Factor to promote diverse results (0-1)
}

// DefaultRetrieverConfig returns default configuration.
func DefaultRetrieverConfig() *RetrieverConfig {
	return &RetrieverConfig{
		MaxResults:      5,
		MinSimilarity:   0.5,
		VectorWeight:    0.7,
		KeywordWeight:   0.3,
		MaxTokens:       2048,
		DiversityFactor: 0.2,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// STATISTICS AND HEALTH
// ═══════════════════════════════════════════════════════════════════════════════

// IngestionStats provides statistics about the knowledge base.
type IngestionStats struct {
	TotalSources     int     `json:"total_sources"`
	TotalChunks      int     `json:"total_chunks"`
	TotalRetrievals  int     `json:"total_retrievals"`
	AvgQualityScore  float64 `json:"avg_quality_score"`
	SourcesByFormat  map[string]int
	ChunksByType     map[string]int
	EmbeddingModels  []EmbeddingModelInfo
}

// EmbeddingModelInfo provides information about embedding models in use.
type EmbeddingModelInfo struct {
	Model       string `json:"model"`
	ChunkCount  int    `json:"chunk_count"`
	IsCurrent   bool   `json:"is_current"`
	NeedsReindex bool   `json:"needs_reindex"`
}

// SourceHealth provides health metrics for a knowledge source.
type SourceHealth struct {
	SourceID       string    `json:"source_id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Format         string    `json:"format"`
	ChunkCount     int       `json:"chunk_count"`
	Status         string    `json:"status"`
	TotalRetrievals int      `json:"total_retrievals"`
	AvgRelevance   float64   `json:"avg_relevance"`
	AvgQuality     float64   `json:"avg_quality"`
	LastUsed       time.Time `json:"last_used,omitempty"`
}
