package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RelationshipPrompt is the LLM prompt for classifying relationships between memories.
const RelationshipPrompt = `Compare these two pieces of information and determine their relationship:

MEMORY 1 (%s):
%s

MEMORY 2 (%s):
%s

What is the relationship between Memory 1 and Memory 2?
Respond with ONLY one of these options:
- CONTRADICTS (Memory 1 contradicts/invalidates Memory 2)
- SUPPORTS (Memory 1 provides evidence for Memory 2)
- EVOLVED_FROM (Memory 1 is an updated version of Memory 2)
- RELATED_TO (General topical relationship)
- NONE (No meaningful relationship)

Also provide a confidence score (0.0-1.0).

Format: [RELATIONSHIP] [CONFIDENCE]
Example: SUPPORTS 0.85`

// MemoryLink represents an explicit connection between two memories.
type MemoryLink struct {
	SourceID   string            `json:"source_id"`
	TargetID   string            `json:"target_id"`
	SourceType MemoryType        `json:"source_type"`
	TargetType MemoryType        `json:"target_type"`
	RelType    LinkType          `json:"rel_type"`
	Confidence float64           `json:"confidence"`
	Metadata   map[string]string `json:"metadata"`
	CreatedAt  time.Time         `json:"created_at"`
	CreatedBy  string            `json:"created_by"` // "user", "system", "llm"
}

// MemoryWithContext represents a memory along with its relationships.
type MemoryWithContext struct {
	Memory         GenericMemory `json:"memory"`
	Contradictions []MemoryLink  `json:"contradictions"`
	Supports       []MemoryLink  `json:"supports"`
	RelatedTo      []MemoryLink  `json:"related_to"`
	HasUpdates     bool          `json:"has_updates"`
}

// RoutingEdge represents a learned model-to-task performance relationship.
type RoutingEdge struct {
	ID           string    `json:"id"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	TaskType     string    `json:"task_type"`
	SuccessRate  float64   `json:"success_rate"`
	SampleCount  int       `json:"sample_count"`
	AvgLatencyMs int       `json:"avg_latency_ms"`
	LastUpdated  time.Time `json:"last_updated"`
	CreatedAt    time.Time `json:"created_at"`
}

// RoutingKnowledge aggregates routing performance data for a task type.
type RoutingKnowledge struct {
	TaskType        string        `json:"task_type"`
	BestModel       string        `json:"best_model"`
	BestProvider    string        `json:"best_provider"`
	BestSuccessRate float64       `json:"best_success_rate"`
	ModelRankings   []RoutingEdge `json:"model_rankings"`
	TotalSamples    int           `json:"total_samples"`
}

// LinkStore manages memory links in SQLite.
type LinkStore struct {
	db                *sql.DB
	embedder          Embedder
	llm               LLMProvider
	neighborhoodStore *NeighborhoodStore
	vectorIndex       *VectorIndex
}

// NewLinkStore creates a new LinkStore.
func NewLinkStore(db *sql.DB, embedder Embedder, llm LLMProvider) *LinkStore {
	return &LinkStore{
		db:       db,
		embedder: embedder,
		llm:      llm,
	}
}

func (s *LinkStore) SetNeighborhoodStore(ns *NeighborhoodStore) {
	s.neighborhoodStore = ns
}

func (s *LinkStore) SetVectorIndex(vi *VectorIndex) {
	s.vectorIndex = vi
}

// CreateLink inserts or replaces a memory link.
func (s *LinkStore) CreateLink(ctx context.Context, link *MemoryLink) error {
	if link == nil {
		return fmt.Errorf("link cannot be nil")
	}

	if link.SourceID == "" || link.TargetID == "" {
		return fmt.Errorf("source_id and target_id are required")
	}

	if link.RelType == "" {
		return fmt.Errorf("rel_type is required")
	}

	// Encode metadata as JSON
	var metadataJSON string
	if link.Metadata != nil {
		data, err := json.Marshal(link.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = string(data)
	}

	// Set default values
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}
	if link.CreatedBy == "" {
		link.CreatedBy = "system"
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_links (source_id, target_id, source_type, target_type, rel_type, confidence, metadata, created_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, target_id, rel_type) DO UPDATE SET
			source_type = excluded.source_type,
			target_type = excluded.target_type,
			confidence = excluded.confidence,
			metadata = excluded.metadata,
			created_at = excluded.created_at,
			created_by = excluded.created_by
	`, link.SourceID, link.TargetID, link.SourceType, link.TargetType,
		link.RelType, link.Confidence, metadataJSON, link.CreatedAt.Format(time.RFC3339), link.CreatedBy)

	if err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}

	log.Debug().
		Str("source_id", link.SourceID).
		Str("target_id", link.TargetID).
		Str("rel_type", string(link.RelType)).
		Float64("confidence", link.Confidence).
		Msg("memory link created")

	return nil
}

// GetLink retrieves a specific memory link by source, target, and relationship type.
func (s *LinkStore) GetLink(ctx context.Context, sourceID, targetID string, relType LinkType) (*MemoryLink, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT source_id, target_id, source_type, target_type, rel_type, confidence, metadata, created_at, created_by
		FROM memory_links
		WHERE source_id = ? AND target_id = ? AND rel_type = ?
	`, sourceID, targetID, relType)

	return s.scanMemoryLink(row)
}

// DeleteLink removes a specific memory link.
func (s *LinkStore) DeleteLink(ctx context.Context, sourceID, targetID string, relType LinkType) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM memory_links
		WHERE source_id = ? AND target_id = ? AND rel_type = ?
	`, sourceID, targetID, relType)

	if err != nil {
		return fmt.Errorf("failed to delete link: %w", err)
	}

	rows, _ := result.RowsAffected()
	log.Debug().
		Str("source_id", sourceID).
		Str("target_id", targetID).
		Str("rel_type", string(relType)).
		Int64("rows_affected", rows).
		Msg("memory link deleted")

	return nil
}

// GetLinkedMemories retrieves all links for a memory (in both directions).
// If relTypes is empty, all link types are returned.
func (s *LinkStore) GetLinkedMemories(ctx context.Context, memoryID string, relTypes ...LinkType) ([]MemoryLink, error) {
	var rows *sql.Rows
	var err error

	if len(relTypes) == 0 {
		// Get all link types in both directions
		rows, err = s.db.QueryContext(ctx, `
			SELECT source_id, target_id, source_type, target_type, rel_type, confidence, metadata, created_at, created_by
			FROM memory_links
			WHERE source_id = ? OR target_id = ?
		`, memoryID, memoryID)
	} else {
		// Build placeholders for rel_types
		placeholders := make([]string, len(relTypes))
		args := make([]any, 0, len(relTypes)+2)
		args = append(args, memoryID, memoryID)

		for i, rt := range relTypes {
			placeholders[i] = "?"
			args = append(args, rt)
		}

		query := fmt.Sprintf(`
			SELECT source_id, target_id, source_type, target_type, rel_type, confidence, metadata, created_at, created_by
			FROM memory_links
			WHERE (source_id = ? OR target_id = ?) AND rel_type IN (%s)
		`, strings.Join(placeholders, ","))

		rows, err = s.db.QueryContext(ctx, query, args...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query linked memories: %w", err)
	}
	defer rows.Close()

	return s.scanMemoryLinks(rows)
}

// GetContradictions is a shorthand for getting all contradiction links for a memory.
func (s *LinkStore) GetContradictions(ctx context.Context, memoryID string) ([]MemoryLink, error) {
	return s.GetLinkedMemories(ctx, memoryID, LinkContradicts)
}

// GetSupports is a shorthand for getting all support links for a memory.
func (s *LinkStore) GetSupports(ctx context.Context, memoryID string) ([]MemoryLink, error) {
	return s.GetLinkedMemories(ctx, memoryID, LinkSupports)
}

// TraverseLinks performs a BFS traversal of memory links up to maxDepth.
// Returns a map from memory ID to all its links.
func (s *LinkStore) TraverseLinks(ctx context.Context, memoryID string, maxDepth int) (map[string][]MemoryLink, error) {
	if maxDepth <= 0 {
		maxDepth = 3 // Default max depth
	}

	result := make(map[string][]MemoryLink)
	visited := make(map[string]bool)
	queue := []string{memoryID}
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		nextQueue := []string{}

		for i := 0; i < levelSize; i++ {
			currentID := queue[i]
			if visited[currentID] {
				continue
			}
			visited[currentID] = true

			links, err := s.GetLinkedMemories(ctx, currentID)
			if err != nil {
				log.Warn().Err(err).Str("memory_id", currentID).Msg("failed to get links during traversal")
				continue
			}

			if len(links) > 0 {
				result[currentID] = links
			}

			// Add connected memories to next level queue
			for _, link := range links {
				nextID := link.TargetID
				if link.TargetID == currentID {
					nextID = link.SourceID
				}
				if !visited[nextID] {
					nextQueue = append(nextQueue, nextID)
				}
			}
		}

		queue = nextQueue
		depth++
	}

	log.Debug().
		Str("start_memory_id", memoryID).
		Int("max_depth", maxDepth).
		Int("memories_found", len(result)).
		Msg("link traversal complete")

	return result, nil
}

// AutoLinkMemory automatically discovers and creates links for a memory.
// It finds similar memories and uses LLM to classify relationships.
func (s *LinkStore) AutoLinkMemory(ctx context.Context, memory GenericMemory) ([]MemoryLink, error) {
	if memory.ID == "" {
		return nil, fmt.Errorf("memory ID is required")
	}

	// Get embedding for the memory if not present
	var embedding []float32
	if len(memory.Embedding) > 0 {
		embedding = memory.Embedding
	} else if s.embedder != nil && memory.Content != "" {
		var err error
		// Use EmbedFast for responsiveness - auto-linking is not critical path
		embedding, err = s.embedder.EmbedFast(ctx, memory.Content)
		if err != nil {
			log.Warn().Err(err).Msg("failed to embed memory for auto-linking")
			return nil, nil // Non-fatal, return empty
		}
	} else {
		return nil, nil // No embedding available, skip auto-linking
	}

	// Find similar memories
	similarMemories, err := s.findSimilarMemories(ctx, embedding, memory.Type, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar memories: %w", err)
	}

	var createdLinks []MemoryLink

	for _, similar := range similarMemories {
		// Skip self
		if similar.ID == memory.ID {
			continue
		}

		// Skip if link already exists (in either direction)
		existing, err := s.GetLinkedMemories(ctx, memory.ID)
		if err == nil {
			linkExists := false
			for _, link := range existing {
				if link.SourceID == similar.ID || link.TargetID == similar.ID {
					linkExists = true
					break
				}
			}
			if linkExists {
				continue
			}
		}

		// Determine relationship using LLM
		relType, confidence, err := s.determineRelationship(ctx, memory, similar)
		if err != nil {
			log.Debug().Err(err).
				Str("source", memory.ID).
				Str("target", similar.ID).
				Msg("failed to determine relationship")
			continue
		}

		// Skip if no meaningful relationship or low confidence
		if relType == "" || confidence < MinConfidenceForLink {
			continue
		}

		// Create the link
		link := &MemoryLink{
			SourceID:   memory.ID,
			TargetID:   similar.ID,
			SourceType: memory.Type,
			TargetType: similar.Type,
			RelType:    relType,
			Confidence: confidence,
			Metadata: map[string]string{
				"auto_linked": "true",
				"similarity":  fmt.Sprintf("%.3f", CosineSimilarity(embedding, similar.Embedding)),
			},
			CreatedAt: time.Now(),
			CreatedBy: "llm",
		}

		if err := s.CreateLink(ctx, link); err != nil {
			log.Warn().Err(err).Msg("failed to create auto-link")
			continue
		}

		createdLinks = append(createdLinks, *link)
	}

	log.Info().
		Str("memory_id", memory.ID).
		Int("links_created", len(createdLinks)).
		Msg("auto-linking complete")

	return createdLinks, nil
}

// RetrieveWithContext retrieves a memory along with all its relationship context.
func (s *LinkStore) RetrieveWithContext(ctx context.Context, memoryID string, memoryType MemoryType) (*MemoryWithContext, error) {
	// Load the memory
	memory, err := s.loadGenericMemory(ctx, memoryID, memoryType)
	if err != nil {
		return nil, fmt.Errorf("failed to load memory: %w", err)
	}

	// Get all links for this memory
	allLinks, err := s.GetLinkedMemories(ctx, memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get links: %w", err)
	}

	result := &MemoryWithContext{
		Memory:         *memory,
		Contradictions: []MemoryLink{},
		Supports:       []MemoryLink{},
		RelatedTo:      []MemoryLink{},
		HasUpdates:     false,
	}

	// Categorize links
	for _, link := range allLinks {
		switch link.RelType {
		case LinkContradicts:
			result.Contradictions = append(result.Contradictions, link)
		case LinkSupports:
			result.Supports = append(result.Supports, link)
		case LinkRelatedTo:
			result.RelatedTo = append(result.RelatedTo, link)
		case LinkEvolvedFrom:
			result.HasUpdates = true
			result.RelatedTo = append(result.RelatedTo, link)
		case LinkCausedBy, LinkLeadsTo:
			result.RelatedTo = append(result.RelatedTo, link)
		}
	}

	return result, nil
}

// findSimilarMemories finds memories similar to the given embedding.
// Uses precomputed neighborhoods and vector index for O(1) lookups when available,
// falling back to O(n) scan only when necessary.
func (s *LinkStore) findSimilarMemories(ctx context.Context, embedding []float32, memoryType MemoryType, limit int) ([]GenericMemory, error) {
	var memories []GenericMemory

	if s.vectorIndex != nil {
		results, err := s.vectorIndex.SearchWithNeighborFallback(ctx, s.neighborhoodStore, embedding, limit, SimilarityThreshold)
		if err == nil && len(results) > 0 {
			for _, r := range results {
				memories = append(memories, r.Item)
			}
			return memories, nil
		}
	}

	// Limit to top 100 memories to prevent slow O(n) scans
	// ORDER BY confidence/updated prioritizes high-quality and recent memories
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, 'strategic' as type, principle as content, embedding
		FROM strategic_memory
		WHERE embedding IS NOT NULL
		ORDER BY confidence DESC, updated_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query strategic memories: %w", err)
	}
	defer rows.Close()

	var candidates []ScoredItem[GenericMemory]

	for rows.Next() {
		var id, contentType, content string
		var embeddingBlob []byte

		if err := rows.Scan(&id, &contentType, &content, &embeddingBlob); err != nil {
			log.Warn().Err(err).Msg("failed to scan strategic memory row")
			continue
		}

		storedEmbedding := BytesToFloat32Slice(embeddingBlob)
		if storedEmbedding == nil {
			continue
		}

		similarity := CosineSimilarity(embedding, storedEmbedding)
		if similarity >= SimilarityThreshold {
			candidates = append(candidates, ScoredItem[GenericMemory]{
				Item: GenericMemory{
					ID:        id,
					Type:      MemoryType(contentType),
					Content:   content,
					Embedding: storedEmbedding,
				},
				Score: similarity,
			})
		}
	}

	// Also limit episodic memories scan
	episodicRows, err := s.db.QueryContext(ctx, `
		SELECT id, 'episodic' as type, content, embedding
		FROM memories
		WHERE embedding IS NOT NULL
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err == nil {
		defer episodicRows.Close()
		for episodicRows.Next() {
			var id, contentType, content string
			var embeddingBlob []byte

			if err := episodicRows.Scan(&id, &contentType, &content, &embeddingBlob); err != nil {
				continue
			}

			storedEmbedding := BytesToFloat32Slice(embeddingBlob)
			if storedEmbedding == nil {
				continue
			}

			similarity := CosineSimilarity(embedding, storedEmbedding)
			if similarity >= SimilarityThreshold {
				candidates = append(candidates, ScoredItem[GenericMemory]{
					Item: GenericMemory{
						ID:        id,
						Type:      MemoryType(contentType),
						Content:   content,
						Embedding: storedEmbedding,
					},
					Score: similarity,
				})
			}
		}
	}

	SortByScoreDesc(candidates)
	for i := 0; i < len(candidates) && i < limit; i++ {
		memories = append(memories, candidates[i].Item)
	}

	return memories, nil
}

// determineRelationship uses LLM to classify the relationship between two memories.
func (s *LinkStore) determineRelationship(ctx context.Context, mem1, mem2 GenericMemory) (LinkType, float64, error) {
	if s.llm == nil {
		// Fallback: use semantic similarity to infer relationship
		similarity := CosineSimilarity(mem1.Embedding, mem2.Embedding)
		if similarity >= DeduplicationThreshold {
			return LinkEvolvedFrom, similarity, nil
		} else if similarity >= SimilarityThreshold {
			return LinkRelatedTo, similarity, nil
		}
		return "", 0, nil
	}

	prompt := fmt.Sprintf(RelationshipPrompt,
		string(mem1.Type), mem1.Content,
		string(mem2.Type), mem2.Content)

	response, err := s.llm.Complete(ctx, prompt)
	if err != nil {
		return "", 0, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response (format: "RELATIONSHIP CONFIDENCE")
	relType, confidence := parseRelationshipResponse(response)

	return relType, confidence, nil
}

// parseRelationshipResponse parses the LLM response for relationship classification.
func parseRelationshipResponse(response string) (LinkType, float64) {
	response = strings.TrimSpace(strings.ToUpper(response))
	parts := strings.Fields(response)

	if len(parts) < 2 {
		return "", 0
	}

	relationship := parts[0]
	confidence, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		confidence = 0.5 // Default confidence
	}

	// Clamp confidence to valid range
	if confidence < 0 {
		confidence = 0
	} else if confidence > 1 {
		confidence = 1
	}

	switch relationship {
	case "CONTRADICTS":
		return LinkContradicts, confidence
	case "SUPPORTS":
		return LinkSupports, confidence
	case "EVOLVED_FROM":
		return LinkEvolvedFrom, confidence
	case "RELATED_TO":
		return LinkRelatedTo, confidence
	case "NONE":
		return "", 0
	default:
		return "", 0
	}
}

// loadGenericMemory loads a memory from the appropriate table based on type.
func (s *LinkStore) loadGenericMemory(ctx context.Context, memoryID string, memoryType MemoryType) (*GenericMemory, error) {
	var memory GenericMemory
	memory.ID = memoryID
	memory.Type = memoryType

	switch memoryType {
	case MemoryTypeStrategic:
		row := s.db.QueryRowContext(ctx, `
			SELECT principle, embedding FROM strategic_memory WHERE id = ?
		`, memoryID)

		var content string
		var embeddingBlob []byte
		if err := row.Scan(&content, &embeddingBlob); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("strategic memory not found: %s", memoryID)
			}
			return nil, err
		}
		memory.Content = content
		memory.Embedding = BytesToFloat32Slice(embeddingBlob)

	case MemoryTypeEpisodic:
		row := s.db.QueryRowContext(ctx, `
			SELECT content, embedding FROM memories WHERE id = ?
		`, memoryID)

		var content string
		var embeddingBlob []byte
		if err := row.Scan(&content, &embeddingBlob); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("episodic memory not found: %s", memoryID)
			}
			return nil, err
		}
		memory.Content = content
		memory.Embedding = BytesToFloat32Slice(embeddingBlob)

	default:
		// For other types, try to find in memories table
		row := s.db.QueryRowContext(ctx, `
			SELECT content, embedding FROM memories WHERE id = ?
		`, memoryID)

		var content string
		var embeddingBlob []byte
		if err := row.Scan(&content, &embeddingBlob); err != nil {
			if err == sql.ErrNoRows {
				return nil, fmt.Errorf("memory not found: %s", memoryID)
			}
			return nil, err
		}
		memory.Content = content
		memory.Embedding = BytesToFloat32Slice(embeddingBlob)
	}

	return &memory, nil
}

// rawLinkRow holds scanned values before processing.
type rawLinkRow struct {
	sourceID, targetID     string
	sourceType, targetType string
	relType                string
	confidence             float64
	metadataJSON           sql.NullString
	createdAtStr           string
	createdBy              sql.NullString
}

// toMemoryLink converts raw scanned values to a MemoryLink struct.
func (r *rawLinkRow) toMemoryLink() *MemoryLink {
	link := &MemoryLink{
		SourceID:   r.sourceID,
		TargetID:   r.targetID,
		SourceType: MemoryType(r.sourceType),
		TargetType: MemoryType(r.targetType),
		RelType:    LinkType(r.relType),
		Confidence: r.confidence,
	}

	// Parse created_at
	link.CreatedAt, _ = time.Parse(time.RFC3339, r.createdAtStr)
	if link.CreatedAt.IsZero() {
		link.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", r.createdAtStr)
	}

	// Parse created_by
	if r.createdBy.Valid {
		link.CreatedBy = r.createdBy.String
	}

	// Parse metadata
	if r.metadataJSON.Valid && r.metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(r.metadataJSON.String), &link.Metadata); err != nil {
			link.Metadata = make(map[string]string)
		}
	} else {
		link.Metadata = make(map[string]string)
	}

	return link
}

// scanMemoryLink scans a single row into a MemoryLink.
func (s *LinkStore) scanMemoryLink(row *sql.Row) (*MemoryLink, error) {
	var r rawLinkRow

	err := row.Scan(
		&r.sourceID, &r.targetID, &r.sourceType, &r.targetType,
		&r.relType, &r.confidence, &r.metadataJSON, &r.createdAtStr, &r.createdBy,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan memory link: %w", err)
	}

	return r.toMemoryLink(), nil
}

// scanMemoryLinks scans multiple rows into a slice of MemoryLinks.
func (s *LinkStore) scanMemoryLinks(rows *sql.Rows) ([]MemoryLink, error) {
	var links []MemoryLink

	for rows.Next() {
		var r rawLinkRow

		err := rows.Scan(
			&r.sourceID, &r.targetID, &r.sourceType, &r.targetType,
			&r.relType, &r.confidence, &r.metadataJSON, &r.createdAtStr, &r.createdBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan memory link row: %w", err)
		}

		links = append(links, *r.toMemoryLink())
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating memory link rows: %w", err)
	}

	return links, nil
}

// =============================================================================
// Routing Edge Methods
// =============================================================================

// CreateRoutingEdge creates or updates a routing performance edge.
// Uses ON CONFLICT to upsert based on the (provider, model, task_type) unique constraint.
func (s *LinkStore) CreateRoutingEdge(ctx context.Context, edge *RoutingEdge) error {
	if edge == nil {
		return fmt.Errorf("edge cannot be nil")
	}

	if edge.Provider == "" || edge.Model == "" || edge.TaskType == "" {
		return fmt.Errorf("provider, model, and task_type are required")
	}

	// Generate ID if not provided
	if edge.ID == "" {
		edge.ID = "re_" + uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = now
	}
	edge.LastUpdated = now

	// Convert success_rate and sample_count to success_count/failure_count
	successCount := int(float64(edge.SampleCount) * edge.SuccessRate)
	failureCount := edge.SampleCount - successCount
	totalLatencyMs := edge.AvgLatencyMs * edge.SampleCount

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO routing_edges (id, provider, model, task_type, success_count, failure_count, total_latency_ms, last_updated, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model, task_type) DO UPDATE SET
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			total_latency_ms = excluded.total_latency_ms,
			last_updated = excluded.last_updated
	`, edge.ID, edge.Provider, edge.Model, edge.TaskType, successCount, failureCount, totalLatencyMs,
		edge.LastUpdated.Format(time.RFC3339), edge.CreatedAt.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("failed to create routing edge: %w", err)
	}

	log.Debug().
		Str("provider", edge.Provider).
		Str("model", edge.Model).
		Str("task_type", edge.TaskType).
		Float64("success_rate", edge.SuccessRate).
		Int("sample_count", edge.SampleCount).
		Msg("routing edge created/updated")

	return nil
}

// GetRoutingEdge retrieves a specific routing edge by provider, model, and task type.
func (s *LinkStore) GetRoutingEdge(ctx context.Context, provider, model, taskType string) (*RoutingEdge, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider, model, task_type, success_count, failure_count, total_latency_ms, last_updated, created_at
		FROM routing_edges
		WHERE provider = ? AND model = ? AND task_type = ?
	`, provider, model, taskType)

	return s.scanRoutingEdge(row)
}

// UpdateRoutingEdge updates stats for an existing edge with incremental update.
// Increments sample_count, recalculates success_rate, and updates avg_latency_ms as running average.
func (s *LinkStore) UpdateRoutingEdge(ctx context.Context, provider, model, taskType string, success bool, latencyMs int) error {
	if provider == "" || model == "" || taskType == "" {
		return fmt.Errorf("provider, model, and task_type are required")
	}

	now := time.Now()

	// Use a single atomic update with CASE expression
	var result sql.Result
	var err error

	if success {
		result, err = s.db.ExecContext(ctx, `
			UPDATE routing_edges
			SET success_count = success_count + 1,
				total_latency_ms = total_latency_ms + ?,
				last_updated = ?
			WHERE provider = ? AND model = ? AND task_type = ?
		`, latencyMs, now.Format(time.RFC3339), provider, model, taskType)
	} else {
		result, err = s.db.ExecContext(ctx, `
			UPDATE routing_edges
			SET failure_count = failure_count + 1,
				total_latency_ms = total_latency_ms + ?,
				last_updated = ?
			WHERE provider = ? AND model = ? AND task_type = ?
		`, latencyMs, now.Format(time.RFC3339), provider, model, taskType)
	}

	if err != nil {
		return fmt.Errorf("failed to update routing edge: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Edge doesn't exist, create it
		edge := &RoutingEdge{
			Provider:     provider,
			Model:        model,
			TaskType:     taskType,
			SampleCount:  1,
			AvgLatencyMs: latencyMs,
		}
		if success {
			edge.SuccessRate = 1.0
		} else {
			edge.SuccessRate = 0.0
		}
		return s.CreateRoutingEdge(ctx, edge)
	}

	log.Debug().
		Str("provider", provider).
		Str("model", model).
		Str("task_type", taskType).
		Bool("success", success).
		Int("latency_ms", latencyMs).
		Msg("routing edge updated")

	return nil
}

// GetModelRelationships returns all edges for a model (all task types).
func (s *LinkStore) GetModelRelationships(ctx context.Context, provider, model string) ([]RoutingEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, model, task_type, success_count, failure_count, total_latency_ms, last_updated, created_at
		FROM routing_edges
		WHERE provider = ? AND model = ?
		ORDER BY task_type
	`, provider, model)
	if err != nil {
		return nil, fmt.Errorf("failed to query model relationships: %w", err)
	}
	defer rows.Close()

	return s.scanRoutingEdges(rows)
}

// GetTaskRelationships returns all edges for a task type (model rankings).
// Results are ordered by success rate descending for ranking purposes.
func (s *LinkStore) GetTaskRelationships(ctx context.Context, taskType string) ([]RoutingEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, provider, model, task_type, success_count, failure_count, total_latency_ms, last_updated, created_at
		FROM routing_edges
		WHERE task_type = ?
	`, taskType)
	if err != nil {
		return nil, fmt.Errorf("failed to query task relationships: %w", err)
	}
	defer rows.Close()

	edges, err := s.scanRoutingEdges(rows)
	if err != nil {
		return nil, err
	}

	// Sort by success rate descending (best performing models first)
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].SuccessRate > edges[j].SuccessRate
	})

	return edges, nil
}

// GetRoutingKnowledge aggregates routing knowledge for a task type.
// Returns the best model, rankings, and total samples for informed routing decisions.
func (s *LinkStore) GetRoutingKnowledge(ctx context.Context, taskType string) (*RoutingKnowledge, error) {
	edges, err := s.GetTaskRelationships(ctx, taskType)
	if err != nil {
		return nil, fmt.Errorf("failed to get task relationships: %w", err)
	}

	knowledge := &RoutingKnowledge{
		TaskType:      taskType,
		ModelRankings: edges, // Already sorted by success rate
		TotalSamples:  0,
	}

	// Calculate total samples and find best model
	for _, edge := range edges {
		knowledge.TotalSamples += edge.SampleCount
	}

	// First edge (if any) is the best due to sorting
	if len(edges) > 0 {
		knowledge.BestModel = edges[0].Model
		knowledge.BestProvider = edges[0].Provider
		knowledge.BestSuccessRate = edges[0].SuccessRate
	}

	log.Debug().
		Str("task_type", taskType).
		Str("best_model", knowledge.BestModel).
		Str("best_provider", knowledge.BestProvider).
		Float64("best_success_rate", knowledge.BestSuccessRate).
		Int("total_samples", knowledge.TotalSamples).
		Int("model_count", len(edges)).
		Msg("routing knowledge aggregated")

	return knowledge, nil
}

// scanRoutingEdge scans a single row into a RoutingEdge.
func (s *LinkStore) scanRoutingEdge(row *sql.Row) (*RoutingEdge, error) {
	var edge RoutingEdge
	var successCount, failureCount, totalLatencyMs int
	var lastUpdatedStr, createdAtStr string

	err := row.Scan(
		&edge.ID, &edge.Provider, &edge.Model, &edge.TaskType,
		&successCount, &failureCount, &totalLatencyMs,
		&lastUpdatedStr, &createdAtStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan routing edge: %w", err)
	}

	// Calculate derived fields
	edge.SampleCount = successCount + failureCount
	if edge.SampleCount > 0 {
		edge.SuccessRate = float64(successCount) / float64(edge.SampleCount)
		edge.AvgLatencyMs = totalLatencyMs / edge.SampleCount
	}

	// Parse timestamps
	edge.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdatedStr)
	if edge.LastUpdated.IsZero() {
		edge.LastUpdated, _ = time.Parse("2006-01-02 15:04:05", lastUpdatedStr)
	}
	edge.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
	}

	return &edge, nil
}

// scanRoutingEdges scans multiple rows into a slice of RoutingEdges.
func (s *LinkStore) scanRoutingEdges(rows *sql.Rows) ([]RoutingEdge, error) {
	edges := []RoutingEdge{} // Return empty slice, not nil

	for rows.Next() {
		var edge RoutingEdge
		var successCount, failureCount, totalLatencyMs int
		var lastUpdatedStr, createdAtStr string

		err := rows.Scan(
			&edge.ID, &edge.Provider, &edge.Model, &edge.TaskType,
			&successCount, &failureCount, &totalLatencyMs,
			&lastUpdatedStr, &createdAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan routing edge row: %w", err)
		}

		// Calculate derived fields
		edge.SampleCount = successCount + failureCount
		if edge.SampleCount > 0 {
			edge.SuccessRate = float64(successCount) / float64(edge.SampleCount)
			edge.AvgLatencyMs = totalLatencyMs / edge.SampleCount
		}

		// Parse timestamps
		edge.LastUpdated, _ = time.Parse(time.RFC3339, lastUpdatedStr)
		if edge.LastUpdated.IsZero() {
			edge.LastUpdated, _ = time.Parse("2006-01-02 15:04:05", lastUpdatedStr)
		}
		edge.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if edge.CreatedAt.IsZero() {
			edge.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAtStr)
		}

		edges = append(edges, edge)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating routing edge rows: %w", err)
	}

	return edges, nil
}

// GetDistinctTaskTypes returns all unique task types from routing edges.
// Used by the DMN Worker for outcome aggregation.
func (s *LinkStore) GetDistinctTaskTypes(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT task_type
		FROM routing_edges
		WHERE task_type IS NOT NULL AND task_type != ''
		ORDER BY task_type
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query distinct task types: %w", err)
	}
	defer rows.Close()

	var taskTypes []string
	for rows.Next() {
		var taskType string
		if err := rows.Scan(&taskType); err != nil {
			log.Warn().Err(err).Msg("failed to scan task type")
			continue
		}
		taskTypes = append(taskTypes, taskType)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task types: %w", err)
	}

	return taskTypes, nil
}
