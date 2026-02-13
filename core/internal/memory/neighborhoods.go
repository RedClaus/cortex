package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const (
	DefaultNeighborLimit    = 10
	DefaultSimilarityThresh = 0.7
	NeighborhoodStaleHours  = 24
)

type Neighborhood struct {
	MemoryID      string             `json:"memory_id"`
	MemoryType    MemoryType         `json:"memory_type"`
	Neighbors     map[string]float64 `json:"neighbors"`
	NeighborCount int                `json:"neighbor_count"`
	ComputedAt    time.Time          `json:"computed_at"`
	IsStale       bool               `json:"is_stale"`
	EmbeddingHash string             `json:"embedding_hash"`
}

type NeighborResult struct {
	MemoryID   string     `json:"memory_id"`
	MemoryType MemoryType `json:"memory_type"`
	Content    string     `json:"content"`
	Similarity float64    `json:"similarity"`
}

type NeighborhoodStore struct {
	db               *sql.DB
	embedder         Embedder
	neighborLimit    int
	similarityThresh float64
}

func NewNeighborhoodStore(db *sql.DB, embedder Embedder) *NeighborhoodStore {
	return &NeighborhoodStore{
		db:               db,
		embedder:         embedder,
		neighborLimit:    DefaultNeighborLimit,
		similarityThresh: DefaultSimilarityThresh,
	}
}

func (ns *NeighborhoodStore) SetNeighborLimit(limit int) {
	if limit > 0 {
		ns.neighborLimit = limit
	}
}

func (ns *NeighborhoodStore) SetSimilarityThreshold(thresh float64) {
	if thresh > 0 && thresh <= 1 {
		ns.similarityThresh = thresh
	}
}

func (ns *NeighborhoodStore) GetNeighbors(ctx context.Context, memoryID string) ([]NeighborResult, error) {
	var neighborsJSON string
	var isStale int

	err := ns.db.QueryRowContext(ctx, `
		SELECT neighbors, is_stale FROM memory_neighborhoods WHERE memory_id = ?
	`, memoryID).Scan(&neighborsJSON, &isStale)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get neighbors: %w", err)
	}

	if isStale == 1 {
		return nil, nil
	}

	var neighbors map[string]float64
	if err := json.Unmarshal([]byte(neighborsJSON), &neighbors); err != nil {
		return nil, fmt.Errorf("unmarshal neighbors: %w", err)
	}

	return ns.hydrateNeighbors(ctx, neighbors)
}

func (ns *NeighborhoodStore) hydrateNeighbors(ctx context.Context, neighbors map[string]float64) ([]NeighborResult, error) {
	if len(neighbors) == 0 {
		return nil, nil
	}

	results := make([]NeighborResult, 0, len(neighbors))

	for memID, similarity := range neighbors {
		content, memType, err := ns.getMemoryContent(ctx, memID)
		if err != nil {
			continue
		}
		results = append(results, NeighborResult{
			MemoryID:   memID,
			MemoryType: memType,
			Content:    content,
			Similarity: similarity,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	return results, nil
}

func (ns *NeighborhoodStore) getMemoryContent(ctx context.Context, memoryID string) (string, MemoryType, error) {
	var content string
	err := ns.db.QueryRowContext(ctx, `
		SELECT principle FROM strategic_memory WHERE id = ?
	`, memoryID).Scan(&content)
	if err == nil {
		return content, MemoryTypeStrategic, nil
	}

	err = ns.db.QueryRowContext(ctx, `
		SELECT content FROM memories WHERE id = ?
	`, memoryID).Scan(&content)
	if err == nil {
		return content, MemoryTypeEpisodic, nil
	}

	return "", "", fmt.Errorf("memory not found: %s", memoryID)
}

func (ns *NeighborhoodStore) ComputeNeighborhood(ctx context.Context, memoryID string, embedding []float32) error {
	if embedding == nil || len(embedding) == 0 {
		return fmt.Errorf("compute neighborhood: embedding required")
	}

	embHash := hashEmbedding(embedding)
	memType := ns.detectMemoryType(ctx, memoryID)

	candidates, err := ns.findCandidates(ctx, memoryID, embedding)
	if err != nil {
		return fmt.Errorf("find candidates: %w", err)
	}

	neighbors := make(map[string]float64)
	for _, c := range candidates {
		if len(neighbors) >= ns.neighborLimit {
			break
		}
		neighbors[c.MemoryID] = c.Similarity
	}

	neighborsJSON, err := json.Marshal(neighbors)
	if err != nil {
		return fmt.Errorf("marshal neighbors: %w", err)
	}

	_, err = ns.db.ExecContext(ctx, `
		INSERT INTO memory_neighborhoods (memory_id, memory_type, neighbors, neighbor_count, computed_at, is_stale, embedding_hash)
		VALUES (?, ?, ?, ?, datetime('now'), 0, ?)
		ON CONFLICT(memory_id) DO UPDATE SET
			neighbors = excluded.neighbors,
			neighbor_count = excluded.neighbor_count,
			computed_at = excluded.computed_at,
			is_stale = 0,
			embedding_hash = excluded.embedding_hash
	`, memoryID, memType, string(neighborsJSON), len(neighbors), embHash)

	if err != nil {
		return fmt.Errorf("save neighborhood: %w", err)
	}

	return nil
}

func (ns *NeighborhoodStore) findCandidates(ctx context.Context, excludeID string, queryEmb []float32) ([]NeighborResult, error) {
	var candidates []NeighborResult

	rows, err := ns.db.QueryContext(ctx, `
		SELECT id, principle, embedding FROM strategic_memory WHERE embedding IS NOT NULL AND id != ?
	`, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, content string
		var embBlob []byte
		if err := rows.Scan(&id, &content, &embBlob); err != nil {
			continue
		}
		emb := BytesToFloat32Slice(embBlob)
		if emb == nil {
			continue
		}
		sim := CosineSimilarity(queryEmb, emb)
		if sim >= ns.similarityThresh {
			candidates = append(candidates, NeighborResult{
				MemoryID:   id,
				MemoryType: MemoryTypeStrategic,
				Content:    content,
				Similarity: sim,
			})
		}
	}

	episodicRows, err := ns.db.QueryContext(ctx, `
		SELECT id, content, embedding FROM memories WHERE embedding IS NOT NULL AND id != ?
	`, excludeID)
	if err == nil {
		defer episodicRows.Close()
		for episodicRows.Next() {
			var id, content string
			var embBlob []byte
			if err := episodicRows.Scan(&id, &content, &embBlob); err != nil {
				continue
			}
			emb := BytesToFloat32Slice(embBlob)
			if emb == nil {
				continue
			}
			sim := CosineSimilarity(queryEmb, emb)
			if sim >= ns.similarityThresh {
				candidates = append(candidates, NeighborResult{
					MemoryID:   id,
					MemoryType: MemoryTypeEpisodic,
					Content:    content,
					Similarity: sim,
				})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Similarity > candidates[j].Similarity
	})

	return candidates, nil
}

func (ns *NeighborhoodStore) MarkStale(ctx context.Context, memoryID string) error {
	_, err := ns.db.ExecContext(ctx, `
		UPDATE memory_neighborhoods SET is_stale = 1 WHERE memory_id = ?
	`, memoryID)
	return err
}

func (ns *NeighborhoodStore) MarkAllStale(ctx context.Context) error {
	_, err := ns.db.ExecContext(ctx, `UPDATE memory_neighborhoods SET is_stale = 1`)
	return err
}

func (ns *NeighborhoodStore) GetStaleCount(ctx context.Context) (int, error) {
	var count int
	err := ns.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_neighborhoods WHERE is_stale = 1`).Scan(&count)
	return count, err
}

func (ns *NeighborhoodStore) RefreshStaleNeighborhoods(ctx context.Context, batchSize int) (int, error) {
	rows, err := ns.db.QueryContext(ctx, `
		SELECT memory_id, memory_type FROM memory_neighborhoods WHERE is_stale = 1 LIMIT ?
	`, batchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var refreshed int
	for rows.Next() {
		var memID, memType string
		if err := rows.Scan(&memID, &memType); err != nil {
			continue
		}

		emb, err := ns.getEmbedding(ctx, memID, MemoryType(memType))
		if err != nil || emb == nil {
			continue
		}

		if err := ns.ComputeNeighborhood(ctx, memID, emb); err == nil {
			refreshed++
		}
	}

	return refreshed, nil
}

func (ns *NeighborhoodStore) getEmbedding(ctx context.Context, memoryID string, memType MemoryType) ([]float32, error) {
	var embBlob []byte
	var err error

	switch memType {
	case MemoryTypeStrategic:
		err = ns.db.QueryRowContext(ctx, `SELECT embedding FROM strategic_memory WHERE id = ?`, memoryID).Scan(&embBlob)
	case MemoryTypeEpisodic:
		err = ns.db.QueryRowContext(ctx, `SELECT embedding FROM memories WHERE id = ?`, memoryID).Scan(&embBlob)
	default:
		return nil, fmt.Errorf("unknown memory type: %s", memType)
	}

	if err != nil {
		return nil, err
	}

	return BytesToFloat32Slice(embBlob), nil
}

func (ns *NeighborhoodStore) detectMemoryType(ctx context.Context, memoryID string) MemoryType {
	var exists int
	ns.db.QueryRowContext(ctx, `SELECT 1 FROM strategic_memory WHERE id = ?`, memoryID).Scan(&exists)
	if exists == 1 {
		return MemoryTypeStrategic
	}
	return MemoryTypeEpisodic
}

func (ns *NeighborhoodStore) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var total, fresh, stale int
	ns.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_neighborhoods`).Scan(&total)
	ns.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_neighborhoods WHERE is_stale = 0`).Scan(&fresh)
	ns.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_neighborhoods WHERE is_stale = 1`).Scan(&stale)

	stats["total"] = total
	stats["fresh"] = fresh
	stats["stale"] = stale

	var avgNeighbors float64
	ns.db.QueryRowContext(ctx, `SELECT AVG(neighbor_count) FROM memory_neighborhoods WHERE is_stale = 0`).Scan(&avgNeighbors)
	stats["avg_neighbors"] = avgNeighbors

	return stats, nil
}

func hashEmbedding(embedding []float32) string {
	data := Float32SliceToBytes(embedding)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}
