-- ============================================================================
-- CR-025-LITE: Strategic Memory Evolution Enhancement
-- Adds versioning, attribution, activation logs, and promotion narratives
-- ============================================================================

-- Add evolution fields to strategic_memory (if not exists)
-- SQLite doesn't support ADD COLUMN IF NOT EXISTS, so we use a workaround

-- Check and add version column
SELECT CASE
    WHEN COUNT(*) = 0 THEN 'ALTER TABLE strategic_memory ADD COLUMN version INTEGER DEFAULT 1'
    ELSE 'SELECT 1'
END
FROM pragma_table_info('strategic_memory') WHERE name = 'version';

-- Actually perform the alter (will fail silently if column exists)
ALTER TABLE strategic_memory ADD COLUMN version INTEGER DEFAULT 1;

-- Check and add parent_id column
ALTER TABLE strategic_memory ADD COLUMN parent_id TEXT;

-- Check and add evolution_chain column
ALTER TABLE strategic_memory ADD COLUMN evolution_chain TEXT DEFAULT '[]';

-- Index for parent lookups
CREATE INDEX IF NOT EXISTS idx_strategic_memory_parent ON strategic_memory(parent_id);

-- Index for version queries
CREATE INDEX IF NOT EXISTS idx_strategic_memory_version ON strategic_memory(version);

-- ============================================================================
-- Outcome Attribution Table
-- Links memories to query outcomes for impact analysis
-- ============================================================================

CREATE TABLE IF NOT EXISTS memory_attributions (
    id TEXT PRIMARY KEY,
    memory_id TEXT NOT NULL,
    query_id TEXT NOT NULL,
    query_text TEXT,
    outcome TEXT CHECK (outcome IN ('success', 'failure', 'partial')),
    contribution REAL DEFAULT 0.5,
    created_at TEXT DEFAULT (datetime('now')),
    session_id TEXT,
    FOREIGN KEY (memory_id) REFERENCES strategic_memory(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_attributions_memory ON memory_attributions(memory_id);
CREATE INDEX IF NOT EXISTS idx_attributions_query ON memory_attributions(query_id);
CREATE INDEX IF NOT EXISTS idx_attributions_session ON memory_attributions(session_id);
CREATE INDEX IF NOT EXISTS idx_attributions_outcome ON memory_attributions(outcome);
CREATE INDEX IF NOT EXISTS idx_attributions_time ON memory_attributions(created_at DESC);

-- ============================================================================
-- Activation Log Table
-- Records memory retrieval events for audit and analysis
-- ============================================================================

CREATE TABLE IF NOT EXISTS activation_logs (
    id TEXT PRIMARY KEY,
    query_id TEXT NOT NULL,
    query_text TEXT,
    memories_found TEXT,  -- JSON array of memory IDs
    retrieval_type TEXT,  -- similarity, fts, category, tier
    latency_ms INTEGER,
    tokens_used INTEGER,
    lane TEXT,  -- fast, smart
    created_at TEXT DEFAULT (datetime('now')),
    session_id TEXT
);

CREATE INDEX IF NOT EXISTS idx_activation_logs_query ON activation_logs(query_id);
CREATE INDEX IF NOT EXISTS idx_activation_logs_session ON activation_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_activation_logs_time ON activation_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activation_logs_latency ON activation_logs(latency_ms DESC);
CREATE INDEX IF NOT EXISTS idx_activation_logs_lane ON activation_logs(lane);

-- ============================================================================
-- Promotion Narratives Table
-- Explains tier changes with human-readable context
-- ============================================================================

CREATE TABLE IF NOT EXISTS promotion_narratives (
    id TEXT PRIMARY KEY,
    memory_id TEXT NOT NULL,
    from_tier TEXT,
    to_tier TEXT NOT NULL,
    reason TEXT NOT NULL,
    metrics TEXT,  -- JSON of triggering metrics
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (memory_id) REFERENCES strategic_memory(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_promotion_narratives_memory ON promotion_narratives(memory_id);
CREATE INDEX IF NOT EXISTS idx_promotion_narratives_to_tier ON promotion_narratives(to_tier);
CREATE INDEX IF NOT EXISTS idx_promotion_narratives_time ON promotion_narratives(created_at DESC);

-- ============================================================================
-- Helpful Views for Analysis
-- ============================================================================

-- View: Memory impact summary
CREATE VIEW IF NOT EXISTS memory_impact_summary AS
SELECT
    ma.memory_id,
    sm.principle,
    sm.tier,
    COUNT(*) as total_uses,
    SUM(CASE WHEN ma.outcome = 'success' THEN 1 ELSE 0 END) as successes,
    SUM(CASE WHEN ma.outcome = 'failure' THEN 1 ELSE 0 END) as failures,
    AVG(ma.contribution) as avg_contribution,
    CAST(SUM(CASE WHEN ma.outcome = 'success' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) as impact_rate
FROM memory_attributions ma
JOIN strategic_memory sm ON ma.memory_id = sm.id
GROUP BY ma.memory_id;

-- View: Recent promotions with memory details
CREATE VIEW IF NOT EXISTS recent_promotions AS
SELECT
    pn.id,
    pn.memory_id,
    sm.principle,
    pn.from_tier,
    pn.to_tier,
    pn.reason,
    pn.created_at
FROM promotion_narratives pn
JOIN strategic_memory sm ON pn.memory_id = sm.id
ORDER BY pn.created_at DESC;

-- View: Activation statistics by lane
CREATE VIEW IF NOT EXISTS activation_stats_by_lane AS
SELECT
    lane,
    COUNT(*) as total_queries,
    AVG(latency_ms) as avg_latency_ms,
    SUM(tokens_used) as total_tokens,
    retrieval_type,
    COUNT(DISTINCT session_id) as unique_sessions
FROM activation_logs
GROUP BY lane, retrieval_type;

-- Migration record
INSERT OR IGNORE INTO migrations (version, name, applied_at)
VALUES (18, 'strategic_evolution_cr025lite', datetime('now'));
