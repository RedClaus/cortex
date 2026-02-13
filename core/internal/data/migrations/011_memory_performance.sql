-- ============================================================================
-- CORTEX MEMORY PERFORMANCE OPTIMIZATION
-- Migration 011: Vector Indexing, Precomputed Neighborhoods, Episode Segmentation
-- Implements patterns from agi-memory for O(1) similarity lookups
-- ============================================================================

-- ============================================================================
-- PRECOMPUTED NEIGHBORHOODS (Component 1)
-- Store precomputed nearest neighbors to avoid O(n) scans at query time
-- Inspired by agi-memory's hot-path optimization (~10-50ms vs ~500ms)
-- ============================================================================

CREATE TABLE IF NOT EXISTS memory_neighborhoods (
    memory_id TEXT PRIMARY KEY,
    memory_type TEXT NOT NULL,              -- 'strategic', 'episodic', etc.
    neighbors TEXT NOT NULL DEFAULT '{}',   -- JSON: {"memory_id": weight, ...}
    neighbor_count INTEGER DEFAULT 0,
    computed_at TEXT DEFAULT (datetime('now')),
    is_stale INTEGER DEFAULT 1,             -- 1=needs recompute, 0=fresh
    embedding_hash TEXT                     -- Hash of embedding for cache invalidation
);

-- Index for stale neighborhood refresh
CREATE INDEX IF NOT EXISTS idx_neighborhoods_stale ON memory_neighborhoods(is_stale, computed_at);
CREATE INDEX IF NOT EXISTS idx_neighborhoods_type ON memory_neighborhoods(memory_type);

-- ============================================================================
-- EMBEDDING BUCKETS (Component 2)
-- Locality-sensitive hashing for approximate nearest neighbor search
-- Reduces O(n) full scan to O(n/k) bucket scan where k = number of buckets
-- ============================================================================

CREATE TABLE IF NOT EXISTS embedding_buckets (
    bucket_id TEXT NOT NULL,                -- Hash-based bucket identifier
    memory_id TEXT NOT NULL,
    memory_type TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (bucket_id, memory_id)
);

-- Index for bucket lookups
CREATE INDEX IF NOT EXISTS idx_buckets_memory ON embedding_buckets(memory_id);

-- ============================================================================
-- EPISODES (Component 3)
-- Temporal grouping of related memories to reduce context token usage
-- Instead of retrieving 20 individual memories, retrieve 3 episode summaries
-- ============================================================================

CREATE TABLE IF NOT EXISTS memory_episodes (
    id TEXT PRIMARY KEY,
    episode_type TEXT NOT NULL DEFAULT 'conversation',  -- 'conversation', 'task', 'session'
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    ended_at TEXT,                          -- NULL = ongoing episode
    title TEXT,                             -- Auto-generated or user-provided
    summary TEXT,                           -- LLM-generated summary of episode
    summary_embedding BLOB,                 -- For semantic episode search
    memory_count INTEGER DEFAULT 0,
    token_estimate INTEGER DEFAULT 0,       -- Estimated tokens if all memories included
    summary_tokens INTEGER DEFAULT 0,       -- Tokens in summary (much smaller)
    compression_ratio REAL GENERATED ALWAYS AS (
        CASE WHEN token_estimate > 0 AND summary_tokens > 0
        THEN CAST(summary_tokens AS REAL) / token_estimate
        ELSE 1.0 END
    ) STORED,
    metadata TEXT DEFAULT '{}',             -- JSON for additional context
    created_at TEXT DEFAULT (datetime('now')),
    is_active INTEGER DEFAULT 1             -- 1=ongoing, 0=completed
);

-- Indexes for episode retrieval
CREATE INDEX IF NOT EXISTS idx_episodes_active ON memory_episodes(is_active, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_episodes_type ON memory_episodes(episode_type, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_episodes_ended ON memory_episodes(ended_at);

-- Episode membership - which memories belong to which episode
CREATE TABLE IF NOT EXISTS episode_members (
    episode_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    memory_type TEXT NOT NULL,              -- 'strategic', 'episodic', 'user', etc.
    sequence_num INTEGER,                   -- Order within episode
    added_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (episode_id, memory_id),
    FOREIGN KEY (episode_id) REFERENCES memory_episodes(id) ON DELETE CASCADE
);

-- Indexes for episode membership
CREATE INDEX IF NOT EXISTS idx_episode_members_memory ON episode_members(memory_id);
CREATE INDEX IF NOT EXISTS idx_episode_members_seq ON episode_members(episode_id, sequence_num);

-- Trigger to maintain memory_count
CREATE TRIGGER IF NOT EXISTS episode_member_insert AFTER INSERT ON episode_members BEGIN
    UPDATE memory_episodes SET memory_count = memory_count + 1 WHERE id = new.episode_id;
END;

CREATE TRIGGER IF NOT EXISTS episode_member_delete AFTER DELETE ON episode_members BEGIN
    UPDATE memory_episodes SET memory_count = memory_count - 1 WHERE id = old.episode_id;
END;

-- ============================================================================
-- CONTENT EMBEDDING CACHE (Component 4)
-- Cache embeddings by content hash to avoid redundant embedding calls
-- Note: Named 'content_embedding_cache' to avoid conflict with 004's embedding_cache
-- ============================================================================

CREATE TABLE IF NOT EXISTS content_embedding_cache (
    content_hash TEXT PRIMARY KEY,          -- SHA256 of content
    embedding BLOB NOT NULL,
    dimension INTEGER NOT NULL,             -- Embedding dimension for validation
    model_id TEXT,                          -- Which model generated this
    created_at TEXT DEFAULT (datetime('now')),
    last_used_at TEXT DEFAULT (datetime('now')),
    use_count INTEGER DEFAULT 1
);

-- Index for LRU eviction
CREATE INDEX IF NOT EXISTS idx_content_embedding_cache_lru ON content_embedding_cache(last_used_at);

-- ============================================================================
-- VIEWS
-- ============================================================================

-- Episode summaries for context injection (token-efficient)
CREATE VIEW IF NOT EXISTS v_episode_summaries AS
SELECT 
    id,
    episode_type,
    title,
    summary,
    memory_count,
    compression_ratio,
    started_at,
    ended_at,
    CASE 
        WHEN ended_at IS NULL THEN 'ongoing'
        ELSE 'completed'
    END as status
FROM memory_episodes
WHERE summary IS NOT NULL
ORDER BY started_at DESC;

-- Stale neighborhoods needing refresh
CREATE VIEW IF NOT EXISTS v_stale_neighborhoods AS
SELECT 
    memory_id,
    memory_type,
    computed_at,
    neighbor_count
FROM memory_neighborhoods
WHERE is_stale = 1
ORDER BY computed_at ASC;

-- Neighborhood stats for monitoring
CREATE VIEW IF NOT EXISTS v_neighborhood_stats AS
SELECT 
    memory_type,
    COUNT(*) as total_memories,
    SUM(CASE WHEN is_stale = 0 THEN 1 ELSE 0 END) as fresh_count,
    SUM(CASE WHEN is_stale = 1 THEN 1 ELSE 0 END) as stale_count,
    AVG(neighbor_count) as avg_neighbors
FROM memory_neighborhoods
GROUP BY memory_type;

-- ============================================================================
-- MIGRATION RECORD
-- ============================================================================

INSERT OR IGNORE INTO migrations (version, name) 
VALUES (11, 'memory_performance_optimization');
