-- ============================================================================
-- CR-025 PHASE 3: SKILL LIBRARY (Voyager-Style)
-- Stores successful execution patterns as reusable skills
-- ============================================================================

-- Skills table for storing learned execution patterns
CREATE TABLE IF NOT EXISTS skills (
    id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,

    -- Skill content as JSON (Name, Description, Pattern, InputSchema, Examples, Tags)
    skill_json TEXT NOT NULL,

    -- Provenance
    source TEXT NOT NULL DEFAULT 'execution',  -- 'execution', 'manual', 'evolution', 'synthesis'
    session_id TEXT,                           -- Source session ID
    parent_id TEXT,                            -- For evolved skills (links to parent skill)

    -- Quality signals
    confidence REAL DEFAULT 0.5,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,

    -- Vector embedding for semantic search
    embedding BLOB,

    -- Temporal tracking
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    last_accessed_at TEXT,
    access_count INTEGER DEFAULT 0,

    -- Self-referencing foreign key for evolution chain
    FOREIGN KEY (parent_id) REFERENCES skills(id) ON DELETE SET NULL
);

-- Index for finding high-success-rate skills
CREATE INDEX IF NOT EXISTS idx_skills_success_rate ON skills(
    (CAST(success_count AS REAL) + 1.0) / (success_count + failure_count + 2.0) DESC
);

-- Index for finding frequently accessed skills
CREATE INDEX IF NOT EXISTS idx_skills_access ON skills(access_count DESC, last_accessed_at DESC);

-- Index for finding skills by source
CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source);

-- Index for finding child skills (evolution chain)
CREATE INDEX IF NOT EXISTS idx_skills_parent ON skills(parent_id);

-- Index for session-based queries
CREATE INDEX IF NOT EXISTS idx_skills_session ON skills(session_id);

-- FTS5 table for full-text search on skill content
CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
    id,
    skill_json,
    content='skills',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS skills_fts_insert AFTER INSERT ON skills BEGIN
    INSERT INTO skills_fts(rowid, id, skill_json)
    VALUES (NEW.rowid, NEW.id, NEW.skill_json);
END;

CREATE TRIGGER IF NOT EXISTS skills_fts_delete AFTER DELETE ON skills BEGIN
    INSERT INTO skills_fts(skills_fts, rowid, id, skill_json)
    VALUES ('delete', OLD.rowid, OLD.id, OLD.skill_json);
END;

CREATE TRIGGER IF NOT EXISTS skills_fts_update AFTER UPDATE ON skills BEGIN
    INSERT INTO skills_fts(skills_fts, rowid, id, skill_json)
    VALUES ('delete', OLD.rowid, OLD.id, OLD.skill_json);
    INSERT INTO skills_fts(rowid, id, skill_json)
    VALUES (NEW.rowid, NEW.id, NEW.skill_json);
END;
