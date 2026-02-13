-- ============================================================================
-- MEMCUBE STORAGE (CR-025)
-- Unified memory abstraction inspired by MemOS (arXiv:2505.22101)
-- ============================================================================

-- ============================================================================
-- MEMCUBE TABLE
-- Atomic unit of memory storage with content + metadata + provenance
-- ============================================================================

CREATE TABLE IF NOT EXISTS memcubes (
    -- Identity
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,

    -- Content Payload
    content TEXT NOT NULL,
    content_type TEXT NOT NULL CHECK (content_type IN ('text', 'skill', 'tool')),
    embedding BLOB,

    -- Provenance (where did this come from?)
    source TEXT NOT NULL,                 -- "conversation", "execution", "synthesis"
    session_id TEXT,
    parent_id TEXT,                       -- For derived/forked cubes
    created_by TEXT NOT NULL,             -- "user", "system", "lobe:reasoning"

    -- Quality signals
    confidence REAL DEFAULT 0.5,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    access_count INTEGER DEFAULT 0,

    -- Temporal
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    last_accessed_at TEXT,

    -- Access control (future multi-user support)
    scope TEXT DEFAULT 'personal',        -- "personal", "team", "global"

    -- Foreign key for parent cube (self-referencing)
    FOREIGN KEY (parent_id) REFERENCES memcubes(id) ON DELETE SET NULL
);

-- ============================================================================
-- INDEXES FOR EFFICIENT RETRIEVAL
-- ============================================================================

-- Index for type-based queries (get all skills, etc.)
CREATE INDEX IF NOT EXISTS idx_memcubes_type ON memcubes(content_type);

-- Index for session-based queries
CREATE INDEX IF NOT EXISTS idx_memcubes_session ON memcubes(session_id);

-- Index for lineage tracking (find all descendants)
CREATE INDEX IF NOT EXISTS idx_memcubes_parent ON memcubes(parent_id);

-- Index for finding high-success-rate cubes
-- Success rate = success_count / (success_count + failure_count)
CREATE INDEX IF NOT EXISTS idx_memcubes_success_rate ON memcubes(
    CAST(success_count AS REAL) / NULLIF(success_count + failure_count, 0) DESC
);

-- Index for recency-based queries
CREATE INDEX IF NOT EXISTS idx_memcubes_accessed ON memcubes(last_accessed_at DESC);

-- Index for updated time ordering
CREATE INDEX IF NOT EXISTS idx_memcubes_updated ON memcubes(updated_at DESC);

-- Index for scope-based filtering
CREATE INDEX IF NOT EXISTS idx_memcubes_scope ON memcubes(scope);

-- Composite index for reliable cubes (used by GetSuccessfulSkills)
CREATE INDEX IF NOT EXISTS idx_memcubes_reliable_skills ON memcubes(
    content_type,
    success_count,
    failure_count
) WHERE content_type = 'skill';

-- ============================================================================
-- MEMCUBE LINKS TABLE
-- Relationships between cubes: derived_from, supports, contradicts, related_to
-- ============================================================================

CREATE TABLE IF NOT EXISTS memcube_links (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    rel_type TEXT NOT NULL,               -- "derived_from", "supports", "contradicts", "related_to"
    confidence REAL DEFAULT 1.0,
    created_at TEXT DEFAULT (datetime('now')),

    PRIMARY KEY (source_id, target_id, rel_type),
    FOREIGN KEY (source_id) REFERENCES memcubes(id) ON DELETE CASCADE,
    FOREIGN KEY (target_id) REFERENCES memcubes(id) ON DELETE CASCADE
);

-- Index for finding links TO a cube (reverse lookup)
CREATE INDEX IF NOT EXISTS idx_cube_links_target ON memcube_links(target_id);

-- Index for finding links by type
CREATE INDEX IF NOT EXISTS idx_cube_links_type ON memcube_links(rel_type);

-- ============================================================================
-- VIEWS FOR COMMON QUERIES
-- ============================================================================

-- View for reliable cubes (3+ observations, 70%+ success rate)
CREATE VIEW IF NOT EXISTS v_reliable_memcubes AS
SELECT
    id,
    content,
    content_type,
    source,
    success_count,
    failure_count,
    CAST(success_count AS REAL) / (success_count + failure_count) AS success_rate,
    confidence,
    created_at,
    updated_at
FROM memcubes
WHERE (success_count + failure_count) >= 3
  AND CAST(success_count AS REAL) / (success_count + failure_count) >= 0.7
ORDER BY success_rate DESC, confidence DESC;

-- View for recently accessed cubes
CREATE VIEW IF NOT EXISTS v_recent_memcubes AS
SELECT
    id,
    content,
    content_type,
    access_count,
    last_accessed_at,
    updated_at
FROM memcubes
WHERE last_accessed_at IS NOT NULL
ORDER BY last_accessed_at DESC
LIMIT 100;

-- View for cube lineage (evolution chains)
CREATE VIEW IF NOT EXISTS v_memcube_lineage AS
SELECT
    child.id AS cube_id,
    child.content AS cube_content,
    child.version AS cube_version,
    parent.id AS parent_id,
    parent.content AS parent_content,
    parent.version AS parent_version
FROM memcubes child
JOIN memcubes parent ON child.parent_id = parent.id
ORDER BY child.created_at DESC;

-- ============================================================================
-- EMBEDDING BUCKET INDEX EXTENSION
-- Add memcube support to existing embedding_buckets table
-- ============================================================================

-- Note: The embedding_buckets table is created by 011_memory_performance.sql
-- We just need to ensure memcube type is supported in queries

-- ============================================================================
-- MIGRATION RECORD
-- ============================================================================

INSERT OR IGNORE INTO migrations (version, name)
VALUES (20, 'memcubes_cr025');
