package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	EpisodeTypeConversation = "conversation"
	EpisodeTypeTask         = "task"
	EpisodeTypeSession      = "session"
	EpisodeTypeReflection   = "reflection"

	DefaultEpisodeGapMinutes = 30
	DefaultMaxEpisodeMembers = 50
)

type Episode struct {
	ID               string            `json:"id"`
	EpisodeType      string            `json:"episode_type"`
	StartedAt        time.Time         `json:"started_at"`
	EndedAt          *time.Time        `json:"ended_at"`
	Title            string            `json:"title"`
	Summary          string            `json:"summary"`
	SummaryEmbedding []float32         `json:"summary_embedding,omitempty"`
	MemoryCount      int               `json:"memory_count"`
	TokenEstimate    int               `json:"token_estimate"`
	SummaryTokens    int               `json:"summary_tokens"`
	CompressionRatio float64           `json:"compression_ratio"`
	Metadata         map[string]string `json:"metadata"`
	CreatedAt        time.Time         `json:"created_at"`
	IsActive         bool              `json:"is_active"`
}

type EpisodeMember struct {
	EpisodeID   string     `json:"episode_id"`
	MemoryID    string     `json:"memory_id"`
	MemoryType  MemoryType `json:"memory_type"`
	SequenceNum int        `json:"sequence_num"`
	AddedAt     time.Time  `json:"added_at"`
}

type EpisodeStore struct {
	db                *sql.DB
	embedder          Embedder
	gapMinutes        int
	maxEpisodeMembers int
}

func NewEpisodeStore(db *sql.DB, embedder Embedder) *EpisodeStore {
	return &EpisodeStore{
		db:                db,
		embedder:          embedder,
		gapMinutes:        DefaultEpisodeGapMinutes,
		maxEpisodeMembers: DefaultMaxEpisodeMembers,
	}
}

func (es *EpisodeStore) SetGapMinutes(minutes int) {
	if minutes > 0 {
		es.gapMinutes = minutes
	}
}

func (es *EpisodeStore) CreateEpisode(ctx context.Context, episodeType, title string) (*Episode, error) {
	ep := &Episode{
		ID:          "ep_" + uuid.New().String(),
		EpisodeType: episodeType,
		Title:       title,
		StartedAt:   time.Now(),
		CreatedAt:   time.Now(),
		IsActive:    true,
		Metadata:    make(map[string]string),
	}

	metaJSON, _ := json.Marshal(ep.Metadata)

	_, err := es.db.ExecContext(ctx, `
		INSERT INTO memory_episodes (id, episode_type, started_at, title, metadata, is_active)
		VALUES (?, ?, ?, ?, ?, 1)
	`, ep.ID, ep.EpisodeType, ep.StartedAt.Format(time.RFC3339), ep.Title, string(metaJSON))

	if err != nil {
		return nil, fmt.Errorf("create episode: %w", err)
	}

	return ep, nil
}

func (es *EpisodeStore) GetActiveEpisode(ctx context.Context, episodeType string) (*Episode, error) {
	ep := &Episode{}
	var endedAt, metaJSON sql.NullString
	var startedAt, createdAt string

	err := es.db.QueryRowContext(ctx, `
		SELECT id, episode_type, started_at, ended_at, title, summary, memory_count, 
		       token_estimate, summary_tokens, compression_ratio, metadata, created_at, is_active
		FROM memory_episodes
		WHERE episode_type = ? AND is_active = 1
		ORDER BY started_at DESC LIMIT 1
	`, episodeType).Scan(
		&ep.ID, &ep.EpisodeType, &startedAt, &endedAt, &ep.Title, &ep.Summary,
		&ep.MemoryCount, &ep.TokenEstimate, &ep.SummaryTokens, &ep.CompressionRatio,
		&metaJSON, &createdAt, &ep.IsActive,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get active episode: %w", err)
	}

	ep.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	ep.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339, endedAt.String)
		ep.EndedAt = &t
	}
	if metaJSON.Valid {
		json.Unmarshal([]byte(metaJSON.String), &ep.Metadata)
	}

	return ep, nil
}

func (es *EpisodeStore) GetOrCreateEpisode(ctx context.Context, episodeType, title string) (*Episode, error) {
	ep, err := es.GetActiveEpisode(ctx, episodeType)
	if err != nil {
		return nil, err
	}

	if ep != nil {
		gapThreshold := time.Now().Add(-time.Duration(es.gapMinutes) * time.Minute)

		var lastActivity string
		es.db.QueryRowContext(ctx, `
			SELECT MAX(added_at) FROM episode_members WHERE episode_id = ?
		`, ep.ID).Scan(&lastActivity)

		if lastActivity != "" {
			lastTime, _ := time.Parse(time.RFC3339, lastActivity)
			if lastTime.Before(gapThreshold) {
				if err := es.CloseEpisode(ctx, ep.ID); err != nil {
					return nil, err
				}
				return es.CreateEpisode(ctx, episodeType, title)
			}
		}

		if ep.MemoryCount >= es.maxEpisodeMembers {
			if err := es.CloseEpisode(ctx, ep.ID); err != nil {
				return nil, err
			}
			return es.CreateEpisode(ctx, episodeType, title)
		}

		return ep, nil
	}

	return es.CreateEpisode(ctx, episodeType, title)
}

func (es *EpisodeStore) AddMemory(ctx context.Context, episodeID, memoryID string, memType MemoryType, tokenCount int) error {
	var maxSeq sql.NullInt64
	es.db.QueryRowContext(ctx, `
		SELECT MAX(sequence_num) FROM episode_members WHERE episode_id = ?
	`, episodeID).Scan(&maxSeq)

	nextSeq := 1
	if maxSeq.Valid {
		nextSeq = int(maxSeq.Int64) + 1
	}

	_, err := es.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO episode_members (episode_id, memory_id, memory_type, sequence_num)
		VALUES (?, ?, ?, ?)
	`, episodeID, memoryID, memType, nextSeq)
	if err != nil {
		return fmt.Errorf("add memory to episode: %w", err)
	}

	_, err = es.db.ExecContext(ctx, `
		UPDATE memory_episodes SET token_estimate = token_estimate + ? WHERE id = ?
	`, tokenCount, episodeID)

	return err
}

func (es *EpisodeStore) CloseEpisode(ctx context.Context, episodeID string) error {
	_, err := es.db.ExecContext(ctx, `
		UPDATE memory_episodes 
		SET is_active = 0, ended_at = datetime('now')
		WHERE id = ?
	`, episodeID)
	return err
}

func (es *EpisodeStore) SetSummary(ctx context.Context, episodeID, summary string) error {
	summaryTokens := len(summary) / 4

	var embeddingBlob []byte
	if es.embedder != nil {
		emb, err := es.embedder.Embed(ctx, summary)
		if err == nil {
			embeddingBlob = Float32SliceToBytes(emb)
		}
	}

	_, err := es.db.ExecContext(ctx, `
		UPDATE memory_episodes 
		SET summary = ?, summary_tokens = ?, summary_embedding = ?
		WHERE id = ?
	`, summary, summaryTokens, embeddingBlob, episodeID)

	return err
}

func (es *EpisodeStore) GetEpisodeMembers(ctx context.Context, episodeID string) ([]EpisodeMember, error) {
	rows, err := es.db.QueryContext(ctx, `
		SELECT episode_id, memory_id, memory_type, sequence_num, added_at
		FROM episode_members
		WHERE episode_id = ?
		ORDER BY sequence_num
	`, episodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []EpisodeMember
	for rows.Next() {
		var m EpisodeMember
		var addedAt string
		if err := rows.Scan(&m.EpisodeID, &m.MemoryID, &m.MemoryType, &m.SequenceNum, &addedAt); err != nil {
			continue
		}
		m.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		members = append(members, m)
	}

	return members, nil
}

func (es *EpisodeStore) GetRecentSummaries(ctx context.Context, episodeType string, limit int) ([]Episode, error) {
	rows, err := es.db.QueryContext(ctx, `
		SELECT id, episode_type, started_at, ended_at, title, summary, memory_count,
		       compression_ratio, is_active
		FROM memory_episodes
		WHERE episode_type = ? AND summary IS NOT NULL AND summary != ''
		ORDER BY started_at DESC
		LIMIT ?
	`, episodeType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var ep Episode
		var startedAt string
		var endedAt sql.NullString
		if err := rows.Scan(&ep.ID, &ep.EpisodeType, &startedAt, &endedAt, &ep.Title,
			&ep.Summary, &ep.MemoryCount, &ep.CompressionRatio, &ep.IsActive); err != nil {
			continue
		}
		ep.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		if endedAt.Valid {
			t, _ := time.Parse(time.RFC3339, endedAt.String)
			ep.EndedAt = &t
		}
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

func (es *EpisodeStore) SearchSimilarEpisodes(ctx context.Context, queryEmb []float32, limit int) ([]Episode, error) {
	rows, err := es.db.QueryContext(ctx, `
		SELECT id, episode_type, started_at, title, summary, summary_embedding, memory_count, compression_ratio
		FROM memory_episodes
		WHERE summary_embedding IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		ep    Episode
		score float64
	}
	var candidates []scored

	for rows.Next() {
		var ep Episode
		var startedAt string
		var embBlob []byte
		if err := rows.Scan(&ep.ID, &ep.EpisodeType, &startedAt, &ep.Title, &ep.Summary,
			&embBlob, &ep.MemoryCount, &ep.CompressionRatio); err != nil {
			continue
		}
		ep.StartedAt, _ = time.Parse(time.RFC3339, startedAt)

		emb := BytesToFloat32Slice(embBlob)
		if emb == nil {
			continue
		}

		sim := CosineSimilarity(queryEmb, emb)
		if sim >= 0.5 {
			candidates = append(candidates, scored{ep: ep, score: sim})
		}
	}

	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	result := make([]Episode, 0, limit)
	for i := 0; i < len(candidates) && i < limit; i++ {
		result = append(result, candidates[i].ep)
	}

	return result, nil
}

func (es *EpisodeStore) GetUnsummarizedEpisodes(ctx context.Context, limit int) ([]Episode, error) {
	rows, err := es.db.QueryContext(ctx, `
		SELECT id, episode_type, started_at, title, memory_count
		FROM memory_episodes
		WHERE is_active = 0 AND (summary IS NULL OR summary = '')
		ORDER BY ended_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []Episode
	for rows.Next() {
		var ep Episode
		var startedAt string
		if err := rows.Scan(&ep.ID, &ep.EpisodeType, &startedAt, &ep.Title, &ep.MemoryCount); err != nil {
			continue
		}
		ep.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
		episodes = append(episodes, ep)
	}

	return episodes, nil
}

func (es *EpisodeStore) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var total, active, summarized int
	es.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_episodes`).Scan(&total)
	es.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_episodes WHERE is_active = 1`).Scan(&active)
	es.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_episodes WHERE summary IS NOT NULL AND summary != ''`).Scan(&summarized)

	stats["total_episodes"] = total
	stats["active_episodes"] = active
	stats["summarized_episodes"] = summarized

	var avgCompression float64
	es.db.QueryRowContext(ctx, `
		SELECT AVG(compression_ratio) FROM memory_episodes 
		WHERE compression_ratio > 0 AND compression_ratio < 1
	`).Scan(&avgCompression)
	stats["avg_compression_ratio"] = avgCompression

	var totalMembers int
	es.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM episode_members`).Scan(&totalMembers)
	stats["total_episode_members"] = totalMembers

	return stats, nil
}
