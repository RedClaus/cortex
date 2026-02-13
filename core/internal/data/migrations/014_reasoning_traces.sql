-- Migration 014: Reasoning Traces for System 3 Meta-Cognition (CR-024)
-- Stores successful reasoning paths for reuse, enabling 80% step reduction
-- on recurring problems (per Sophia paper arXiv:2512.18202).

-- Reasoning traces table
CREATE TABLE IF NOT EXISTS reasoning_traces (
    id TEXT PRIMARY KEY,
    query TEXT NOT NULL,                    -- Original user query
    query_embedding BLOB,                   -- Vector embedding for similarity search
    approach TEXT,                          -- High-level strategy description
    steps_json BLOB,                        -- Compressed JSON of reasoning steps
    outcome TEXT DEFAULT 'success',         -- success, partial, failed, interrupted
    success_score REAL DEFAULT 0.0,         -- 0-1 quality score
    reused_count INTEGER DEFAULT 0,         -- Number of times trace was reused
    tools_used TEXT DEFAULT '[]',           -- JSON array of tool names
    lobes_activated TEXT DEFAULT '[]',      -- JSON array of brain lobes
    total_duration_ms INTEGER DEFAULT 0,    -- Total execution time in ms
    tokens_used INTEGER DEFAULT 0,          -- Estimated token consumption
    created_at TEXT NOT NULL,
    last_used_at TEXT NOT NULL,
    metadata TEXT DEFAULT '{}'              -- Additional metadata JSON
);

-- Index for similarity search (embedding buckets)
CREATE INDEX IF NOT EXISTS idx_traces_embedding ON reasoning_traces(query_embedding);

-- Index for finding successful traces
CREATE INDEX IF NOT EXISTS idx_traces_success ON reasoning_traces(success_score DESC);

-- Index for finding reusable traces
CREATE INDEX IF NOT EXISTS idx_traces_reused ON reasoning_traces(reused_count DESC);

-- Index for recency-based queries
CREATE INDEX IF NOT EXISTS idx_traces_created ON reasoning_traces(created_at DESC);

-- Index for outcome filtering
CREATE INDEX IF NOT EXISTS idx_traces_outcome ON reasoning_traces(outcome);

-- Full-text search on query text
CREATE VIRTUAL TABLE IF NOT EXISTS reasoning_traces_fts USING fts5(
    query,
    approach,
    content='reasoning_traces',
    content_rowid='rowid'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS reasoning_traces_ai AFTER INSERT ON reasoning_traces BEGIN
    INSERT INTO reasoning_traces_fts(rowid, query, approach)
    VALUES (NEW.rowid, NEW.query, NEW.approach);
END;

CREATE TRIGGER IF NOT EXISTS reasoning_traces_ad AFTER DELETE ON reasoning_traces BEGIN
    INSERT INTO reasoning_traces_fts(reasoning_traces_fts, rowid, query, approach)
    VALUES('delete', OLD.rowid, OLD.query, OLD.approach);
END;

CREATE TRIGGER IF NOT EXISTS reasoning_traces_au AFTER UPDATE ON reasoning_traces BEGIN
    INSERT INTO reasoning_traces_fts(reasoning_traces_fts, rowid, query, approach)
    VALUES('delete', OLD.rowid, OLD.query, OLD.approach);
    INSERT INTO reasoning_traces_fts(rowid, query, approach)
    VALUES (NEW.rowid, NEW.query, NEW.approach);
END;

-- View for trace statistics
CREATE VIEW IF NOT EXISTS v_trace_stats AS
SELECT
    COUNT(*) as total_traces,
    SUM(CASE WHEN outcome = 'success' THEN 1 ELSE 0 END) as successful_traces,
    SUM(CASE WHEN reused_count > 0 THEN 1 ELSE 0 END) as reused_traces,
    AVG(success_score) as avg_success_score,
    SUM(reused_count) as total_reuses,
    AVG(tokens_used) as avg_tokens_per_trace,
    AVG(total_duration_ms) as avg_duration_ms
FROM reasoning_traces;

-- View for recently reused traces (high-value traces)
CREATE VIEW IF NOT EXISTS v_valuable_traces AS
SELECT
    id, query, approach, outcome, success_score, reused_count,
    total_duration_ms, created_at, last_used_at
FROM reasoning_traces
WHERE success_score >= 0.7 OR reused_count > 0
ORDER BY
    CASE WHEN reused_count > 0 THEN reused_count * 10 ELSE 0 END + success_score * 5 DESC
LIMIT 100;
