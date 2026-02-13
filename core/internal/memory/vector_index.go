package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

const (
	DefaultNumBuckets       = 16
	DefaultBucketDimensions = 8
)

type VectorIndex struct {
	db         *sql.DB
	numBuckets int
	bucketDims int
}

func NewVectorIndex(db *sql.DB) *VectorIndex {
	return &VectorIndex{
		db:         db,
		numBuckets: DefaultNumBuckets,
		bucketDims: DefaultBucketDimensions,
	}
}

func (vi *VectorIndex) SetNumBuckets(n int) {
	if n > 0 {
		vi.numBuckets = n
	}
}

func (vi *VectorIndex) IndexMemory(ctx context.Context, memoryID string, memType MemoryType, embedding []float32) error {
	if len(embedding) == 0 {
		return nil
	}

	bucketID := vi.computeBucketID(embedding)

	_, err := vi.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO embedding_buckets (bucket_id, memory_id, memory_type)
		VALUES (?, ?, ?)
	`, bucketID, memoryID, memType)

	return err
}

func (vi *VectorIndex) RemoveMemory(ctx context.Context, memoryID string) error {
	_, err := vi.db.ExecContext(ctx, `DELETE FROM embedding_buckets WHERE memory_id = ?`, memoryID)
	return err
}

func (vi *VectorIndex) SearchSimilar(ctx context.Context, queryEmb []float32, limit int, threshold float64) ([]ScoredItem[GenericMemory], error) {
	if len(queryEmb) == 0 {
		return nil, nil
	}

	primaryBucket := vi.computeBucketID(queryEmb)
	adjacentBuckets := vi.getAdjacentBuckets(primaryBucket)

	allBuckets := append([]string{primaryBucket}, adjacentBuckets...)

	var candidates []ScoredItem[GenericMemory]

	for _, bucketID := range allBuckets {
		rows, err := vi.db.QueryContext(ctx, `
			SELECT eb.memory_id, eb.memory_type
			FROM embedding_buckets eb
			WHERE eb.bucket_id = ?
		`, bucketID)
		if err != nil {
			continue
		}

		for rows.Next() {
			var memID, memType string
			if err := rows.Scan(&memID, &memType); err != nil {
				continue
			}

			emb, content, err := vi.getMemoryEmbedding(ctx, memID, MemoryType(memType))
			if err != nil || emb == nil {
				continue
			}

			sim := CosineSimilarity(queryEmb, emb)
			if sim >= threshold {
				candidates = append(candidates, ScoredItem[GenericMemory]{
					Item: GenericMemory{
						ID:        memID,
						Type:      MemoryType(memType),
						Content:   content,
						Embedding: emb,
					},
					Score: sim,
				})
			}
		}
		rows.Close()
	}

	// Use min-heap based top-K selection: O(n log k) vs O(n log n) for sort
	candidates = TopKWithScores(candidates, limit)

	return candidates, nil
}

func (vi *VectorIndex) SearchWithNeighborFallback(
	ctx context.Context,
	ns *NeighborhoodStore,
	queryEmb []float32,
	limit int,
	threshold float64,
) ([]ScoredItem[GenericMemory], error) {
	indexed, err := vi.SearchSimilar(ctx, queryEmb, limit, threshold)
	if err == nil && len(indexed) >= limit {
		return indexed, nil
	}

	if ns != nil && len(indexed) < limit {
		if len(indexed) > 0 {
			neighbors, err := ns.GetNeighbors(ctx, indexed[0].Item.ID)
			if err == nil {
				for _, n := range neighbors {
					if len(indexed) >= limit {
						break
					}
					found := false
					for _, existing := range indexed {
						if existing.Item.ID == n.MemoryID {
							found = true
							break
						}
					}
					if !found {
						indexed = append(indexed, ScoredItem[GenericMemory]{
							Item: GenericMemory{
								ID:      n.MemoryID,
								Type:    n.MemoryType,
								Content: n.Content,
							},
							Score: n.Similarity,
						})
					}
				}
			}
		}
	}

	return indexed, nil
}

func (vi *VectorIndex) computeBucketID(embedding []float32) string {
	if len(embedding) == 0 {
		return "0"
	}

	step := len(embedding) / vi.bucketDims
	if step == 0 {
		step = 1
	}

	var bucketBits uint64
	for i := 0; i < vi.bucketDims && i*step < len(embedding); i++ {
		sum := float32(0)
		count := 0
		for j := i * step; j < (i+1)*step && j < len(embedding); j++ {
			sum += embedding[j]
			count++
		}
		if count > 0 && sum/float32(count) > 0 {
			bucketBits |= 1 << i
		}
	}

	return fmt.Sprintf("%x", bucketBits)
}

func (vi *VectorIndex) getAdjacentBuckets(bucketID string) []string {
	var original uint64
	fmt.Sscanf(bucketID, "%x", &original)

	adjacent := make([]string, 0, vi.bucketDims)
	for i := 0; i < vi.bucketDims; i++ {
		flipped := original ^ (1 << i)
		adjacent = append(adjacent, fmt.Sprintf("%x", flipped))
	}

	return adjacent
}

func (vi *VectorIndex) getMemoryEmbedding(ctx context.Context, memoryID string, memType MemoryType) ([]float32, string, error) {
	var embBlob []byte
	var content string

	switch memType {
	case MemoryTypeStrategic:
		err := vi.db.QueryRowContext(ctx, `
			SELECT embedding, principle FROM strategic_memory WHERE id = ?
		`, memoryID).Scan(&embBlob, &content)
		if err != nil {
			return nil, "", err
		}
	case MemoryTypeEpisodic:
		err := vi.db.QueryRowContext(ctx, `
			SELECT embedding, content FROM memories WHERE id = ?
		`, memoryID).Scan(&embBlob, &content)
		if err != nil {
			return nil, "", err
		}
	default:
		return nil, "", fmt.Errorf("unsupported memory type: %s", memType)
	}

	return BytesToFloat32Slice(embBlob), content, nil
}

func (vi *VectorIndex) RebuildIndex(ctx context.Context) error {
	_, err := vi.db.ExecContext(ctx, `DELETE FROM embedding_buckets`)
	if err != nil {
		return fmt.Errorf("clear buckets: %w", err)
	}

	rows, err := vi.db.QueryContext(ctx, `
		SELECT id, embedding FROM strategic_memory WHERE embedding IS NOT NULL
	`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var id string
		var embBlob []byte
		if err := rows.Scan(&id, &embBlob); err != nil {
			continue
		}
		emb := BytesToFloat32Slice(embBlob)
		if emb != nil {
			vi.IndexMemory(ctx, id, MemoryTypeStrategic, emb)
		}
	}
	rows.Close()

	episodicRows, err := vi.db.QueryContext(ctx, `
		SELECT id, embedding FROM memories WHERE embedding IS NOT NULL
	`)
	if err == nil {
		for episodicRows.Next() {
			var id string
			var embBlob []byte
			if err := episodicRows.Scan(&id, &embBlob); err != nil {
				continue
			}
			emb := BytesToFloat32Slice(embBlob)
			if emb != nil {
				vi.IndexMemory(ctx, id, MemoryTypeEpisodic, emb)
			}
		}
		episodicRows.Close()
	}

	return nil
}

func (vi *VectorIndex) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalIndexed int
	vi.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embedding_buckets`).Scan(&totalIndexed)
	stats["total_indexed"] = totalIndexed

	var uniqueBuckets int
	vi.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT bucket_id) FROM embedding_buckets`).Scan(&uniqueBuckets)
	stats["unique_buckets"] = uniqueBuckets

	if uniqueBuckets > 0 {
		stats["avg_memories_per_bucket"] = float64(totalIndexed) / float64(uniqueBuckets)
	}

	var strategicCount, episodicCount int
	vi.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embedding_buckets WHERE memory_type = 'strategic'`).Scan(&strategicCount)
	vi.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embedding_buckets WHERE memory_type = 'episodic'`).Scan(&episodicCount)
	stats["strategic_indexed"] = strategicCount
	stats["episodic_indexed"] = episodicCount

	return stats, nil
}

func (vi *VectorIndex) EstimateSearchReduction(ctx context.Context) (float64, error) {
	var totalMemories int
	vi.db.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM strategic_memory WHERE embedding IS NOT NULL) +
		       (SELECT COUNT(*) FROM memories WHERE embedding IS NOT NULL)
	`).Scan(&totalMemories)

	var avgBucketSize float64
	vi.db.QueryRowContext(ctx, `
		SELECT AVG(cnt) FROM (
			SELECT COUNT(*) as cnt FROM embedding_buckets GROUP BY bucket_id
		)
	`).Scan(&avgBucketSize)

	if totalMemories == 0 || avgBucketSize == 0 {
		return 1.0, nil
	}

	searchScope := avgBucketSize * float64(vi.bucketDims+1)
	reduction := searchScope / float64(totalMemories)

	return math.Min(reduction, 1.0), nil
}
