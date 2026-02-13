-- ============================================================================
-- CORTEX ENHANCED MEMORY SCHEMA
-- Migration 010: Strategic Memory, Topic Clustering, Identity, Memory Links
-- CR-015: Cortex Memory Enhancement (AGI-Memory Integration)
-- ============================================================================

-- ============================================================================
-- STRATEGIC MEMORY (Component 1)
-- High-level principles and heuristics derived from patterns of success/failure
-- ============================================================================

CREATE TABLE IF NOT EXISTS strategic_memory (
    id TEXT PRIMARY KEY,
    principle TEXT NOT NULL,              -- The rule/heuristic
    category TEXT,                        -- "debugging", "docker", "git", etc.
    trigger_pattern TEXT,                 -- When to apply this principle
    success_count INTEGER DEFAULT 0,      -- Times following this worked
    failure_count INTEGER DEFAULT 0,      -- Times ignoring this failed
    success_rate REAL GENERATED ALWAYS AS (
        CASE WHEN (success_count + failure_count) > 0 
        THEN CAST(success_count AS REAL) / (success_count + failure_count)
        ELSE 0.5 END
    ) STORED,
    confidence REAL DEFAULT 0.5,          -- How confident we are (0-1)
    source_sessions TEXT,                 -- JSON array of session IDs that formed this
    embedding BLOB,                       -- Vector for similarity search
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    last_applied_at TEXT,
    apply_count INTEGER DEFAULT 0
);

-- Indexes for fast retrieval
CREATE INDEX IF NOT EXISTS idx_strategic_category ON strategic_memory(category);
CREATE INDEX IF NOT EXISTS idx_strategic_success_rate ON strategic_memory(success_rate DESC);
CREATE INDEX IF NOT EXISTS idx_strategic_confidence ON strategic_memory(confidence DESC);
CREATE INDEX IF NOT EXISTS idx_strategic_updated ON strategic_memory(updated_at DESC);

-- FTS5 for text search on strategic memory
CREATE VIRTUAL TABLE IF NOT EXISTS strategic_memory_fts USING fts5(
    id,
    principle,
    category,
    trigger_pattern,
    content=strategic_memory,
    content_rowid=rowid,
    tokenize='porter unicode61'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS strategic_memory_fts_ai AFTER INSERT ON strategic_memory BEGIN
    INSERT INTO strategic_memory_fts(rowid, id, principle, category, trigger_pattern)
    VALUES (new.rowid, new.id, new.principle, new.category, new.trigger_pattern);
END;

CREATE TRIGGER IF NOT EXISTS strategic_memory_fts_ad AFTER DELETE ON strategic_memory BEGIN
    INSERT INTO strategic_memory_fts(strategic_memory_fts, rowid, id, principle, category, trigger_pattern)
    VALUES ('delete', old.rowid, old.id, old.principle, old.category, old.trigger_pattern);
END;

CREATE TRIGGER IF NOT EXISTS strategic_memory_fts_au AFTER UPDATE ON strategic_memory BEGIN
    INSERT INTO strategic_memory_fts(strategic_memory_fts, rowid, id, principle, category, trigger_pattern)
    VALUES ('delete', old.rowid, old.id, old.principle, old.category, old.trigger_pattern);
    INSERT INTO strategic_memory_fts(rowid, id, principle, category, trigger_pattern)
    VALUES (new.rowid, new.id, new.principle, new.category, new.trigger_pattern);
END;

-- ============================================================================
-- TOPIC CLUSTERING (Component 2)
-- Automatic grouping of related memories into topics
-- ============================================================================

CREATE TABLE IF NOT EXISTS memory_topics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,                   -- "Project Phoenix", "Docker Debugging"
    description TEXT,
    keywords TEXT,                        -- JSON array of keywords
    centroid_embedding BLOB,              -- Average vector of all members
    member_count INTEGER DEFAULT 0,
    last_active_at TEXT DEFAULT (datetime('now')),
    created_at TEXT DEFAULT (datetime('now')),
    is_active INTEGER DEFAULT 1           -- Currently relevant topic (1=active, 0=inactive)
);

-- Index for fast topic retrieval
CREATE INDEX IF NOT EXISTS idx_topic_active ON memory_topics(is_active, last_active_at DESC);
CREATE INDEX IF NOT EXISTS idx_topic_name ON memory_topics(name);

-- Topic Membership Table
CREATE TABLE IF NOT EXISTS memory_topic_members (
    topic_id TEXT NOT NULL,
    memory_id TEXT NOT NULL,
    memory_type TEXT NOT NULL,            -- "episodic", "procedural", "strategic"
    added_at TEXT DEFAULT (datetime('now')),
    relevance_score REAL DEFAULT 1.0,
    PRIMARY KEY (topic_id, memory_id),
    FOREIGN KEY (topic_id) REFERENCES memory_topics(id) ON DELETE CASCADE
);

-- Index for finding topics for a memory
CREATE INDEX IF NOT EXISTS idx_topic_members_memory ON memory_topic_members(memory_id);
CREATE INDEX IF NOT EXISTS idx_topic_members_type ON memory_topic_members(memory_type);

-- Triggers to maintain member_count
CREATE TRIGGER IF NOT EXISTS topic_member_insert AFTER INSERT ON memory_topic_members BEGIN
    UPDATE memory_topics SET member_count = member_count + 1 WHERE id = new.topic_id;
END;

CREATE TRIGGER IF NOT EXISTS topic_member_delete AFTER DELETE ON memory_topic_members BEGIN
    UPDATE memory_topics SET member_count = member_count - 1 WHERE id = old.topic_id;
END;

-- ============================================================================
-- AGENT IDENTITY (Component 3)
-- Persistent agent identity and state
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_identity (
    id TEXT PRIMARY KEY DEFAULT 'henry',
    name TEXT NOT NULL DEFAULT 'Henry',
    core_values TEXT,                     -- JSON array: ["Privacy first", "Be concise"]
    current_goal TEXT,                    -- What are we working on?
    mood TEXT DEFAULT 'neutral',          -- Derived from recent sessions
    persona_prompt TEXT,                  -- Custom persona instructions
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Initialize default identity
INSERT OR IGNORE INTO agent_identity (id, name, core_values)
VALUES ('henry', 'Henry', '["Be helpful", "Be concise", "Respect privacy"]');

-- ============================================================================
-- MEMORY LINKS (Component 4)
-- Explicit connections between memories
-- Note: Drop and recreate to upgrade from legacy schema
-- ============================================================================

DROP TABLE IF EXISTS memory_links;

CREATE TABLE memory_links (
    source_id TEXT NOT NULL,
    target_id TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'strategic',  -- "episodic", "procedural", "strategic"
    target_type TEXT NOT NULL DEFAULT 'strategic',
    rel_type TEXT NOT NULL,               -- "contradicts", "supports", "evolved_from", "related_to"
    confidence REAL DEFAULT 1.0,
    metadata TEXT,                        -- JSON for additional context
    created_at TEXT DEFAULT (datetime('now')),
    created_by TEXT,                      -- "user", "system", "llm"
    PRIMARY KEY (source_id, target_id, rel_type)
);

-- Indexes for efficient traversal
CREATE INDEX IF NOT EXISTS idx_link_source ON memory_links(source_id);
CREATE INDEX IF NOT EXISTS idx_link_target ON memory_links(target_id);
CREATE INDEX IF NOT EXISTS idx_link_type ON memory_links(rel_type);
CREATE INDEX IF NOT EXISTS idx_link_created ON memory_links(created_at DESC);

-- ============================================================================
-- VIEWS
-- ============================================================================

-- Active strategic principles (ordered by reliability)
CREATE VIEW IF NOT EXISTS v_reliable_principles AS
SELECT 
    id,
    principle,
    category,
    trigger_pattern,
    success_count,
    failure_count,
    success_rate,
    confidence,
    apply_count,
    last_applied_at
FROM strategic_memory
WHERE (success_count + failure_count) >= 3  -- Minimum evidence threshold
ORDER BY success_rate DESC, confidence DESC;

-- Active topics with member counts
CREATE VIEW IF NOT EXISTS v_active_topics AS
SELECT 
    id,
    name,
    description,
    keywords,
    member_count,
    last_active_at,
    created_at
FROM memory_topics
WHERE is_active = 1
ORDER BY last_active_at DESC;

-- Memory links with both directions (for graph traversal)
-- Drop existing view first in case schema changed
DROP VIEW IF EXISTS v_memory_graph;
CREATE VIEW v_memory_graph AS
SELECT
    source_id,
    target_id,
    source_type,
    target_type,
    rel_type,
    confidence,
    created_at
FROM memory_links
UNION ALL
SELECT
    target_id as source_id,
    source_id as target_id,
    target_type as source_type,
    source_type as target_type,
    CASE rel_type
        WHEN 'contradicts' THEN 'contradicts'
        WHEN 'supports' THEN 'supported_by'
        WHEN 'evolved_from' THEN 'evolved_to'
        WHEN 'related_to' THEN 'related_to'
        WHEN 'caused_by' THEN 'causes'
        WHEN 'leads_to' THEN 'follows_from'
        ELSE rel_type
    END as rel_type,
    confidence,
    created_at
FROM memory_links;

-- ============================================================================
-- MIGRATION RECORD
-- ============================================================================

INSERT OR IGNORE INTO migrations (version, name) 
VALUES (10, 'memory_enhanced_cr015');
