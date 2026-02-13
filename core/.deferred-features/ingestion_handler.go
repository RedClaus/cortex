package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/normanking/cortex/internal/ingestion"
)

// ═══════════════════════════════════════════════════════════════════════════════
// INGESTION API REQUEST/RESPONSE TYPES
// ═══════════════════════════════════════════════════════════════════════════════

// IngestionIngestRequest is the request body for POST /api/v1/knowledge/ingest.
type IngestionIngestRequest struct {
	// Path to file or directory to ingest (mutually exclusive with Content)
	Path string `json:"path,omitempty"`

	// Raw content to ingest (mutually exclusive with Path)
	Content string `json:"content,omitempty"`

	// Format of the content (auto-detected if not provided)
	Format string `json:"format,omitempty"`

	// Optional: Name for the source (defaults to filename or required if Content is provided)
	Name string `json:"name,omitempty"`

	// Optional: Description of the source
	Description string `json:"description,omitempty"`

	// Optional: Tags for categorization
	Tags []string `json:"tags,omitempty"`

	// Optional: Category (e.g., "documentation", "guidelines")
	Category string `json:"category,omitempty"`

	// Optional: Platform (e.g., "macos", "linux", "windows", "all")
	Platform string `json:"platform,omitempty"`
}

// IngestionIngestResponse is the response for POST /api/v1/knowledge/ingest.
type IngestionIngestResponse struct {
	SourceID       string   `json:"source_id"`
	ChunksCreated  int      `json:"chunks_created"`
	EmbeddingModel string   `json:"embedding_model"`
	DurationMs     int64    `json:"duration_ms"`
	Warnings       []string `json:"warnings,omitempty"`
}

// IngestionSearchRequest is the request body for POST /api/v1/knowledge/search.
type IngestionSearchRequest struct {
	Query    string  `json:"query"`
	Limit    int     `json:"limit,omitempty"`     // Default: 5
	MinScore float64 `json:"min_score,omitempty"` // Default: 0.5
}

// IngestionSearchResponse is the response for POST /api/v1/knowledge/search.
type IngestionSearchResponse struct {
	Results     []IngestionSearchResult `json:"results"`
	Query       string                  `json:"query"`
	TotalTokens int                     `json:"total_tokens"`
	LatencyMs   int64                   `json:"latency_ms"`
}

// IngestionSearchResult represents a single search result.
type IngestionSearchResult struct {
	Content     string                 `json:"content"`
	Score       float64                `json:"score"`
	Source      string                 `json:"source"`
	SourceID    string                 `json:"source_id"`
	ContentType string                 `json:"content_type"`
	Title       string                 `json:"title,omitempty"`
	MatchType   string                 `json:"match_type"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// IngestionSourcesResponse is the response for GET /api/v1/knowledge/sources.
type IngestionSourcesResponse struct {
	Sources []IngestionSourceHealth `json:"sources"`
	Total   int                     `json:"total"`
}

// IngestionSourceHealth represents health metrics for a knowledge source.
type IngestionSourceHealth struct {
	SourceID        string    `json:"source_id"`
	Name            string    `json:"name"`
	Category        string    `json:"category,omitempty"`
	Format          string    `json:"format"`
	ChunkCount      int       `json:"chunk_count"`
	Status          string    `json:"status"`
	TotalRetrievals int       `json:"total_retrievals"`
	AvgRelevance    float64   `json:"avg_relevance"`
	AvgQuality      float64   `json:"avg_quality"`
	LastUsed        time.Time `json:"last_used,omitempty"`
}

// IngestionStatsResponse is the response for GET /api/v1/knowledge/stats.
type IngestionStatsResponse struct {
	TotalSources    int            `json:"total_sources"`
	TotalChunks     int            `json:"total_chunks"`
	AvgQualityScore float64        `json:"avg_quality_score"`
	SourcesByFormat map[string]int `json:"sources_by_format"`
	ChunksByType    map[string]int `json:"chunks_by_type"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// INGESTION API HANDLERS
// ═══════════════════════════════════════════════════════════════════════════════

// handleIngestionIngest handles POST /api/v1/knowledge/ingest - Ingest a document.
func (p *Prism) handleIngestionIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if ingestion components are initialized
	if p.ingestionPipeline == nil || p.ingestionStore == nil {
		p.writeError(w, http.StatusServiceUnavailable, "ingestion pipeline not available")
		return
	}

	// Parse request
	var req IngestionIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Path == "" && req.Content == "" {
		p.writeError(w, http.StatusBadRequest, "either path or content must be provided")
		return
	}

	if req.Path != "" && req.Content != "" {
		p.writeError(w, http.StatusBadRequest, "path and content are mutually exclusive")
		return
	}

	// If content is provided, name is required
	if req.Content != "" && req.Name == "" {
		p.writeError(w, http.StatusBadRequest, "name is required when providing raw content")
		return
	}

	// Create ingestion request
	ingestionReq := &ingestion.IngestionRequest{
		Name:        req.Name,
		Description: req.Description,
		SourcePath:  req.Path,
		Content:     req.Content,
		Format:      req.Format,
		Category:    req.Category,
		Tags:        req.Tags,
		Platform:    req.Platform,
	}

	// Set source type
	if req.Path != "" {
		ingestionReq.SourceType = "file"
	} else {
		ingestionReq.SourceType = "manual"
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	startTime := time.Now()

	var result *ingestion.IngestionResult
	var chunks []*ingestion.Chunk
	var err error

	// Ingest based on request type
	if req.Content != "" {
		// Direct content ingestion
		result, err = p.ingestionPipeline.Ingest(ctx, ingestionReq)
		if err != nil {
			p.log.Warn("[Prism] Ingestion failed: %v", err)
			p.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Note: chunks would need to be extracted separately, but for now we'll save the source only
		chunks = []*ingestion.Chunk{}
	} else {
		// File-based ingestion
		result, chunks, err = p.ingestionPipeline.IngestFile(ctx, req.Path, &ingestion.IngestionOptions{
			Name:            req.Name,
			Description:     req.Description,
			Category:        req.Category,
			Tags:            req.Tags,
			Platform:        req.Platform,
			ExtractCommands: true,
			ExtractKeywords: true,
			MinQualityScore: 0.5,
		})
		if err != nil {
			p.log.Warn("[Prism] Ingestion failed: %v", err)
			p.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Save source to database
	if err := p.ingestionStore.SaveSource(ctx, result, ingestionReq); err != nil {
		p.log.Warn("[Prism] Failed to save source: %v", err)
		p.writeError(w, http.StatusInternalServerError, "failed to save source metadata")
		return
	}

	// Save chunks to database
	if len(chunks) > 0 {
		if err := p.ingestionStore.SaveChunks(ctx, chunks); err != nil {
			p.log.Warn("[Prism] Failed to save chunks: %v", err)
			// Continue - source is saved, chunks can be re-generated
		}
	}

	p.log.Info("[Prism] Ingested source: %s (%d chunks, %dms)",
		result.SourceID, result.ChunksCreated, time.Since(startTime).Milliseconds())

	// Build response
	response := IngestionIngestResponse{
		SourceID:       result.SourceID,
		ChunksCreated:  result.ChunksCreated,
		EmbeddingModel: result.EmbeddingModel,
		DurationMs:     result.Duration.Milliseconds(),
		Warnings:       result.Warnings,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleIngestionSearch handles POST /api/v1/knowledge/search - Search knowledge base.
func (p *Prism) handleIngestionSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if retriever is initialized
	if p.ingestionRetriever == nil {
		p.writeError(w, http.StatusServiceUnavailable, "knowledge search not available")
		return
	}

	// Parse request
	var req IngestionSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate
	if req.Query == "" {
		p.writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	// Set defaults
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.MinScore <= 0 {
		req.MinScore = 0.5
	}

	// Create context
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Search
	retrievalResult, err := p.ingestionRetriever.Search(ctx, req.Query)
	if err != nil {
		p.log.Warn("[Prism] Search failed: %v", err)
		p.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	var results []IngestionSearchResult
	for _, chunkResult := range retrievalResult.Chunks {
		// Filter by min score
		if chunkResult.Similarity < req.MinScore {
			continue
		}

		// Limit results
		if len(results) >= req.Limit {
			break
		}

		result := IngestionSearchResult{
			Content:     chunkResult.Chunk.Content,
			Score:       chunkResult.Similarity,
			Source:      chunkResult.Chunk.Title,
			SourceID:    chunkResult.Chunk.SourceID,
			ContentType: chunkResult.Chunk.ContentType,
			Title:       chunkResult.Chunk.Title,
			MatchType:   chunkResult.MatchType,
			Metadata:    make(map[string]interface{}),
		}

		// Add metadata
		if chunkResult.Chunk.SectionPath != "" {
			result.Metadata["section_path"] = chunkResult.Chunk.SectionPath
		}
		if len(chunkResult.Chunk.Commands) > 0 {
			result.Metadata["commands"] = chunkResult.Chunk.Commands
		}
		if len(chunkResult.Chunk.Keywords) > 0 {
			result.Metadata["keywords"] = chunkResult.Chunk.Keywords
		}

		results = append(results, result)
	}

	response := IngestionSearchResponse{
		Results:     results,
		Query:       req.Query,
		TotalTokens: retrievalResult.TotalTokens,
		LatencyMs:   retrievalResult.LatencyMs,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleIngestionSources handles GET /api/v1/knowledge/sources - List all sources.
func (p *Prism) handleIngestionSources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if store is initialized
	if p.ingestionStore == nil {
		p.writeError(w, http.StatusServiceUnavailable, "knowledge store not available")
		return
	}

	// Create context
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get sources
	sources, err := p.ingestionStore.ListSources(ctx)
	if err != nil {
		p.log.Warn("[Prism] Failed to list sources: %v", err)
		p.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to response format
	var healthSources []IngestionSourceHealth
	for _, source := range sources {
		healthSources = append(healthSources, IngestionSourceHealth{
			SourceID:        source.SourceID,
			Name:            source.Name,
			Category:        source.Category,
			Format:          source.Format,
			ChunkCount:      source.ChunkCount,
			Status:          source.Status,
			TotalRetrievals: source.TotalRetrievals,
			AvgRelevance:    source.AvgRelevance,
			AvgQuality:      source.AvgQuality,
			LastUsed:        source.LastUsed,
		})
	}

	response := IngestionSourcesResponse{
		Sources: healthSources,
		Total:   len(healthSources),
	}

	p.writeJSON(w, http.StatusOK, response)
}

// handleIngestionDeleteSource handles DELETE /api/v1/knowledge/source/:id - Delete a source.
func (p *Prism) handleIngestionDeleteSource(w http.ResponseWriter, r *http.Request, sourceID string) {
	if r.Method != http.MethodDelete {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if store is initialized
	if p.ingestionStore == nil {
		p.writeError(w, http.StatusServiceUnavailable, "knowledge store not available")
		return
	}

	// Validate source ID
	if sourceID == "" {
		p.writeError(w, http.StatusBadRequest, "source ID is required")
		return
	}

	// Create context
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Delete source
	if err := p.ingestionStore.DeleteSource(ctx, sourceID); err != nil {
		p.log.Warn("[Prism] Failed to delete source %s: %v", sourceID, err)
		p.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	p.log.Info("[Prism] Deleted source: %s", sourceID)

	w.WriteHeader(http.StatusNoContent)
}

// handleIngestionStats handles GET /api/v1/knowledge/stats - Get knowledge base statistics.
func (p *Prism) handleIngestionStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if store is initialized
	if p.ingestionStore == nil {
		p.writeError(w, http.StatusServiceUnavailable, "knowledge store not available")
		return
	}

	// Create context
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get stats
	stats, err := p.ingestionStore.GetStats(ctx)
	if err != nil {
		p.log.Warn("[Prism] Failed to get stats: %v", err)
		p.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build response
	response := IngestionStatsResponse{
		TotalSources:    stats.TotalSources,
		TotalChunks:     stats.TotalChunks,
		AvgQualityScore: stats.AvgQualityScore,
		SourcesByFormat: stats.SourcesByFormat,
		ChunksByType:    stats.ChunksByType,
	}

	p.writeJSON(w, http.StatusOK, response)
}

// ═══════════════════════════════════════════════════════════════════════════════
// ROUTING HELPER
// ═══════════════════════════════════════════════════════════════════════════════

// handleIngestionSourceByID routes DELETE /api/v1/knowledge/source/:id
func (p *Prism) handleIngestionSourceByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge/source/")
	if path == "" {
		p.writeError(w, http.StatusBadRequest, "source ID is required")
		return
	}

	switch r.Method {
	case http.MethodDelete:
		p.handleIngestionDeleteSource(w, r, path)
	default:
		p.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
