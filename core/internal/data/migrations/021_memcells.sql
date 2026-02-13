-- ============================================================================
-- MEMCELL ATOMIC MEMORY STRUCTURE (CR-027)
-- Inspired by EverMemOS MemCell Architecture
-- Hippocampal-aligned structured memory with relational links
-- ============================================================================

-- ============================================================================
-- MEMCELLS TABLE
-- Atomic unit of memory with 5 layers: Identity, Content, Classification,
-- Relational, Context
-- ============================================================================

CREATE TABLE IF NOT EXISTS memcells (
    -- Identity Layer
    id TEXT PRIMARY KEY,
    source_id TEXT,                           -- Original message/document ID
    version INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    last_access_at TEXT,
    access_count INTEGER DEFAULT 0,

    -- Content Layer
    raw_content TEXT NOT NULL,
    summary TEXT,
    entities TEXT,                            -- JSON array of extracted entities
    key_phrases TEXT,                         -- JSON array of key phrases
    sentiment REAL DEFAULT 0,                 -- -1 to 1

    -- Classification Layer
    memory_type TEXT NOT NULL CHECK (memory_type IN (
        'episode', 'event', 'interaction',    -- Episodic
        'fact', 'knowledge', 'procedure',     -- Semantic
        'preference', 'profile', 'relationship', -- Personal
        'principle', 'lesson', 'goal',        -- Strategic
        'context', 'project', 'mood'          -- Contextual
    )),
    confidence REAL DEFAULT 0.5,              -- 0 to 1
    importance REAL DEFAULT 0.5,              -- 0 to 1
    topics TEXT,                              -- JSON array of topic tags
    scope TEXT DEFAULT 'personal' CHECK (scope IN ('personal', 'team', 'global')),

    -- Relational Layer (direct foreign keys)
    parent_id TEXT REFERENCES memcells(id) ON DELETE SET NULL,
    supersedes_id TEXT REFERENCES memcells(id) ON DELETE SET NULL,
    episode_id TEXT,

    -- Context Layer
    event_boundary INTEGER DEFAULT 0,         -- Boolean: marks episode boundary
    preceding_ctx TEXT,                       -- Context before this memory
    following_ctx TEXT,                       -- Context after this memory
    conversation_id TEXT,
    turn_number INTEGER,
    user_state TEXT                           -- JSON snapshot of user state
);

-- ============================================================================
-- MEMCELL RELATIONS TABLE
-- Many-to-many relationships: related, contradicts, supports, child
-- ============================================================================

CREATE TABLE IF NOT EXISTS memcell_relations (
    from_id TEXT NOT NULL REFERENCES memcells(id) ON DELETE CASCADE,
    to_id TEXT NOT NULL REFERENCES memcells(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL CHECK (relation_type IN (
        'related',      -- Semantic similarity
        'contradicts',  -- Conflicting information
        'supports',     -- Supporting evidence
        'child',        -- Hierarchical relationship
        'causes',       -- Causal relationship
        'precedes',     -- Temporal ordering
        'elaborates'    -- More detail on same topic
    )),
    strength REAL DEFAULT 0.5,                -- 0 to 1
    created_at TEXT DEFAULT (datetime('now')),

    PRIMARY KEY (from_id, to_id, relation_type)
);

-- ============================================================================
-- INDEXES FOR EFFICIENT RETRIEVAL
-- ============================================================================

-- Classification indexes
CREATE INDEX IF NOT EXISTS idx_memcells_type ON memcells(memory_type);
CREATE INDEX IF NOT EXISTS idx_memcells_scope ON memcells(scope);
CREATE INDEX IF NOT EXISTS idx_memcells_importance ON memcells(importance DESC);
CREATE INDEX IF NOT EXISTS idx_memcells_confidence ON memcells(confidence DESC);

-- Temporal indexes
CREATE INDEX IF NOT EXISTS idx_memcells_created ON memcells(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_memcells_accessed ON memcells(last_access_at DESC);

-- Relational indexes
CREATE INDEX IF NOT EXISTS idx_memcells_episode ON memcells(episode_id);
CREATE INDEX IF NOT EXISTS idx_memcells_conversation ON memcells(conversation_id);
CREATE INDEX IF NOT EXISTS idx_memcells_parent ON memcells(parent_id);
CREATE INDEX IF NOT EXISTS idx_memcells_supersedes ON memcells(supersedes_id);

-- Event boundary index for episode detection
CREATE INDEX IF NOT EXISTS idx_memcells_boundary ON memcells(event_boundary)
    WHERE event_boundary = 1;

-- Relations indexes
CREATE INDEX IF NOT EXISTS idx_relations_from ON memcell_relations(from_id);
CREATE INDEX IF NOT EXISTS idx_relations_to ON memcell_relations(to_id);
CREATE INDEX IF NOT EXISTS idx_relations_type ON memcell_relations(relation_type);
CREATE INDEX IF NOT EXISTS idx_relations_strength ON memcell_relations(strength DESC);

-- Composite index for type + importance (common query pattern)
CREATE INDEX IF NOT EXISTS idx_memcells_type_importance ON memcells(memory_type, importance DESC);

-- ============================================================================
-- FULL-TEXT SEARCH
-- ============================================================================

CREATE VIRTUAL TABLE IF NOT EXISTS memcells_fts USING fts5(
    raw_content,
    summary,
    entities,
    key_phrases,
    content='memcells',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS memcells_ai AFTER INSERT ON memcells BEGIN
    INSERT INTO memcells_fts(rowid, raw_content, summary, entities, key_phrases)
    VALUES (new.rowid, new.raw_content, new.summary, new.entities, new.key_phrases);
END;

CREATE TRIGGER IF NOT EXISTS memcells_ad AFTER DELETE ON memcells BEGIN
    INSERT INTO memcells_fts(memcells_fts, rowid, raw_content, summary, entities, key_phrases)
    VALUES ('delete', old.rowid, old.raw_content, old.summary, old.entities, old.key_phrases);
END;

CREATE TRIGGER IF NOT EXISTS memcells_au AFTER UPDATE ON memcells BEGIN
    INSERT INTO memcells_fts(memcells_fts, rowid, raw_content, summary, entities, key_phrases)
    VALUES ('delete', old.rowid, old.raw_content, old.summary, old.entities, old.key_phrases);
    INSERT INTO memcells_fts(rowid, raw_content, summary, entities, key_phrases)
    VALUES (new.rowid, new.raw_content, new.summary, new.entities, new.key_phrases);
END;

-- ============================================================================
-- VIEWS FOR COMMON QUERIES
-- ============================================================================

-- View for high-importance memories
CREATE VIEW IF NOT EXISTS v_important_memcells AS
SELECT
    id,
    raw_content,
    summary,
    memory_type,
    importance,
    confidence,
    created_at
FROM memcells
WHERE importance >= 0.7
ORDER BY importance DESC, created_at DESC;

-- View for event boundaries (episode starts)
CREATE VIEW IF NOT EXISTS v_episode_boundaries AS
SELECT
    id,
    raw_content,
    memory_type,
    conversation_id,
    turn_number,
    created_at
FROM memcells
WHERE event_boundary = 1
ORDER BY created_at DESC;

-- View for strategic memories (principles, lessons, goals)
CREATE VIEW IF NOT EXISTS v_strategic_memcells AS
SELECT
    id,
    raw_content,
    summary,
    memory_type,
    importance,
    confidence,
    topics,
    created_at
FROM memcells
WHERE memory_type IN ('principle', 'lesson', 'goal')
ORDER BY importance DESC, confidence DESC;

-- View for personal memories (user model)
CREATE VIEW IF NOT EXISTS v_personal_memcells AS
SELECT
    id,
    raw_content,
    summary,
    memory_type,
    importance,
    created_at
FROM memcells
WHERE memory_type IN ('preference', 'profile', 'relationship')
ORDER BY importance DESC, created_at DESC;

-- ============================================================================
-- MIGRATION RECORD
-- ============================================================================

INSERT OR IGNORE INTO migrations (version, name)
VALUES (21, 'memcells_cr027');
