-- ═══════════════════════════════════════════════════════════════════════════════
-- CORTEX COGNITIVE ARCHITECTURE SCHEMA v1.0
-- Runtime Distillation, Template Registry, Embedding Router
-- ═══════════════════════════════════════════════════════════════════════════════
--
-- This migration adds the cognitive layer that enables:
-- 1. Template-based response generation (BAU tasks)
-- 2. Embedding-based semantic routing
-- 3. Runtime distillation from frontier models
-- 4. Gradual template promotion through probationary grading
--
-- ═══════════════════════════════════════════════════════════════════════════════

-- ═══════════════════════════════════════════════════════════════════════════════
-- TEMPLATES (Core Cognitive Table)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS templates (
    -- Primary key
    id TEXT PRIMARY KEY,

    -- Identity
    name TEXT NOT NULL,                    -- Human-readable name
    description TEXT,                       -- What this template does

    -- Intent matching
    intent TEXT NOT NULL,                   -- Natural language intent (e.g., "configure VLAN")
    intent_embedding BLOB,                  -- float32[] serialized (768 dims for nomic-embed-text)
    intent_keywords TEXT DEFAULT '[]',      -- JSON array of fallback keywords

    -- Template body
    template_body TEXT NOT NULL,            -- Go text/template syntax
    example_output TEXT,                    -- Example of rendered template

    -- Variable schema (flat JSON Schema - no nested objects)
    variable_schema TEXT NOT NULL DEFAULT '{}',  -- JSON Schema for variables

    -- GBNF Grammar (pre-computed for Ollama)
    gbnf_grammar TEXT,                      -- Generated GBNF grammar string

    -- Classification
    task_type TEXT NOT NULL DEFAULT 'general' CHECK (
        task_type IN ('general', 'code_gen', 'debug', 'review', 'planning',
                      'infrastructure', 'explain', 'refactor')
    ),
    domain TEXT,                            -- Optional domain (e.g., "cisco", "kubernetes")

    -- ═══ LIFECYCLE STATUS ═══
    status TEXT NOT NULL DEFAULT 'probation' CHECK (
        status IN ('probation', 'validated', 'promoted', 'deprecated')
    ),
    -- probation: Just created by distillation, needs grading
    -- validated: Passed initial grading period
    -- promoted: High confidence, used in production
    -- deprecated: Failed too many times, not used

    -- ═══ QUALITY METRICS ═══
    confidence_score REAL DEFAULT 0.5 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    complexity_score INTEGER DEFAULT 50 CHECK (complexity_score >= 0 AND complexity_score <= 100),

    -- Usage tracking
    use_count INTEGER DEFAULT 0 CHECK (use_count >= 0),
    success_count INTEGER DEFAULT 0 CHECK (success_count >= 0),
    failure_count INTEGER DEFAULT 0 CHECK (failure_count >= 0),

    -- Source tracking
    source_type TEXT DEFAULT 'distillation' CHECK (
        source_type IN ('distillation', 'manual', 'imported')
    ),
    source_model TEXT,                      -- Model that created this (e.g., "claude-sonnet")
    source_request_id TEXT,                 -- Original request that triggered distillation

    -- ═══ TEMPORAL ═══
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    promoted_at DATETIME,                   -- When moved to promoted status
    deprecated_at DATETIME                  -- When deprecated
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- TEMPLATE USAGE LOG
-- Tracks every use of a template for analytics and grading
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS template_usage_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    template_id TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    session_id TEXT,                        -- Optional link to session
    request_id TEXT,                        -- Request that triggered this use

    -- Input/Output
    user_input TEXT NOT NULL,               -- Original user request
    extracted_variables TEXT,               -- JSON of extracted variables
    rendered_output TEXT,                   -- Final rendered template

    -- Matching details
    similarity_score REAL,                  -- Embedding similarity (0-1)
    match_method TEXT CHECK (               -- How the template was matched
        match_method IN ('embedding', 'keyword', 'exact')
    ),

    -- Outcome
    success INTEGER,                        -- 0 or 1 (NULL if not yet known)
    user_feedback TEXT CHECK (              -- Explicit user feedback
        user_feedback IN ('positive', 'negative', 'neutral') OR user_feedback IS NULL
    ),

    -- Timing
    extraction_ms INTEGER,                  -- Time to extract variables
    rendering_ms INTEGER,                   -- Time to render template
    total_ms INTEGER,                       -- Total processing time

    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- TEMPLATE GRADING LOG
-- Records grading outcomes for probationary templates
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS template_grading_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    template_id TEXT NOT NULL REFERENCES templates(id) ON DELETE CASCADE,
    usage_log_id INTEGER REFERENCES template_usage_log(id) ON DELETE SET NULL,

    -- Grading details
    grader_model TEXT NOT NULL,             -- Model that performed grading (e.g., "claude-sonnet")
    grade TEXT NOT NULL CHECK (grade IN ('pass', 'fail', 'partial')),
    grade_reason TEXT,                      -- Why this grade was given

    -- Detailed scores
    correctness_score REAL CHECK (correctness_score >= 0 AND correctness_score <= 1),
    completeness_score REAL CHECK (completeness_score >= 0 AND completeness_score <= 1),

    -- Confidence adjustment
    confidence_delta REAL,                  -- How much confidence changed (+0.1, -0.1, etc.)

    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- EMBEDDING INDEX CACHE
-- Pre-computed embeddings for efficient similarity search
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS embedding_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Source reference
    source_type TEXT NOT NULL CHECK (source_type IN ('template', 'knowledge', 'query')),
    source_id TEXT NOT NULL,

    -- Embedding data
    text_hash TEXT NOT NULL,                -- SHA256 of original text
    embedding BLOB NOT NULL,                -- float32[] serialized
    embedding_model TEXT NOT NULL,          -- Model used (e.g., "nomic-embed-text")
    embedding_dim INTEGER NOT NULL,         -- Dimension (768 for nomic)

    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(source_type, source_id)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- DISTILLATION REQUESTS
-- Tracks requests that triggered frontier model distillation
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS distillation_requests (
    id TEXT PRIMARY KEY,

    -- Request details
    user_input TEXT NOT NULL,               -- Original user request
    task_type TEXT,                         -- Classified task type

    -- Routing decision
    similarity_score REAL,                  -- Highest similarity to existing template
    route_reason TEXT,                      -- Why this was routed to frontier

    -- Frontier response
    frontier_model TEXT NOT NULL,           -- Model used (e.g., "claude-sonnet-4")
    solution TEXT,                          -- The solution provided to user

    -- Distillation result
    template_created INTEGER DEFAULT 0,     -- 1 if template was extracted
    template_id TEXT REFERENCES templates(id) ON DELETE SET NULL,
    extraction_error TEXT,                  -- Error if extraction failed

    -- Safety valve outcomes
    compilation_passed INTEGER,             -- Template compiled successfully
    schema_valid INTEGER,                   -- Schema is flat (no nesting)
    grammar_generated INTEGER,              -- GBNF generated successfully

    -- Timing
    frontier_ms INTEGER,                    -- Time for frontier response
    extraction_ms INTEGER,                  -- Time for template extraction

    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- COGNITIVE SYSTEM METRICS
-- Daily rollup of cognitive system performance
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS cognitive_metrics (
    date TEXT NOT NULL,                     -- YYYY-MM-DD
    metric TEXT NOT NULL,                   -- Metric name
    value REAL DEFAULT 0,

    PRIMARY KEY (date, metric)
);

-- Pre-define key metrics
INSERT OR IGNORE INTO cognitive_metrics (date, metric, value) VALUES
    (date('now'), 'template_hits', 0),
    (date('now'), 'template_misses', 0),
    (date('now'), 'distillation_requests', 0),
    (date('now'), 'distillation_successes', 0),
    (date('now'), 'grading_passes', 0),
    (date('now'), 'grading_fails', 0),
    (date('now'), 'promotions', 0),
    (date('now'), 'deprecations', 0);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES
-- ═══════════════════════════════════════════════════════════════════════════════

-- Templates
CREATE INDEX IF NOT EXISTS idx_templates_status ON templates(status);
CREATE INDEX IF NOT EXISTS idx_templates_task_type ON templates(task_type);
CREATE INDEX IF NOT EXISTS idx_templates_domain ON templates(domain);
CREATE INDEX IF NOT EXISTS idx_templates_confidence ON templates(confidence_score DESC);
CREATE INDEX IF NOT EXISTS idx_templates_use_count ON templates(use_count DESC);
CREATE INDEX IF NOT EXISTS idx_templates_promoted ON templates(status, confidence_score DESC)
    WHERE status = 'promoted';

-- Usage log
CREATE INDEX IF NOT EXISTS idx_usage_template ON template_usage_log(template_id);
CREATE INDEX IF NOT EXISTS idx_usage_session ON template_usage_log(session_id);
CREATE INDEX IF NOT EXISTS idx_usage_created ON template_usage_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_pending_grade ON template_usage_log(success)
    WHERE success IS NULL;

-- Grading log
CREATE INDEX IF NOT EXISTS idx_grading_template ON template_grading_log(template_id);
CREATE INDEX IF NOT EXISTS idx_grading_created ON template_grading_log(created_at DESC);

-- Embedding cache
CREATE INDEX IF NOT EXISTS idx_embedding_source ON embedding_cache(source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_embedding_hash ON embedding_cache(text_hash);

-- Distillation requests
CREATE INDEX IF NOT EXISTS idx_distillation_created ON distillation_requests(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_distillation_template ON distillation_requests(template_id);

-- ═══════════════════════════════════════════════════════════════════════════════
-- VIEWS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Active templates (promoted + validated) for routing
CREATE VIEW IF NOT EXISTS v_active_templates AS
SELECT * FROM templates
WHERE status IN ('promoted', 'validated')
ORDER BY confidence_score DESC, use_count DESC;

-- Templates pending grading
CREATE VIEW IF NOT EXISTS v_probation_templates AS
SELECT
    t.*,
    COUNT(g.id) as grading_count,
    SUM(CASE WHEN g.grade = 'pass' THEN 1 ELSE 0 END) as pass_count,
    SUM(CASE WHEN g.grade = 'fail' THEN 1 ELSE 0 END) as fail_count
FROM templates t
LEFT JOIN template_grading_log g ON t.id = g.template_id
WHERE t.status = 'probation'
GROUP BY t.id;

-- Templates ready for promotion
CREATE VIEW IF NOT EXISTS v_promotion_candidates AS
SELECT * FROM v_probation_templates
WHERE grading_count >= 3
  AND pass_count >= 3
  AND (pass_count * 1.0 / grading_count) >= 0.9;

-- Templates at risk of deprecation
CREATE VIEW IF NOT EXISTS v_deprecation_candidates AS
SELECT * FROM v_probation_templates
WHERE grading_count >= 3
  AND (fail_count * 1.0 / grading_count) >= 0.5;

-- Daily cognitive stats
CREATE VIEW IF NOT EXISTS v_cognitive_daily_stats AS
SELECT
    date,
    MAX(CASE WHEN metric = 'template_hits' THEN value END) as template_hits,
    MAX(CASE WHEN metric = 'template_misses' THEN value END) as template_misses,
    MAX(CASE WHEN metric = 'distillation_requests' THEN value END) as distillation_requests,
    MAX(CASE WHEN metric = 'distillation_successes' THEN value END) as distillation_successes,
    MAX(CASE WHEN metric = 'grading_passes' THEN value END) as grading_passes,
    MAX(CASE WHEN metric = 'grading_fails' THEN value END) as grading_fails,
    MAX(CASE WHEN metric = 'promotions' THEN value END) as promotions,
    MAX(CASE WHEN metric = 'deprecations' THEN value END) as deprecations
FROM cognitive_metrics
GROUP BY date
ORDER BY date DESC;

-- ═══════════════════════════════════════════════════════════════════════════════
-- FTS FOR TEMPLATE KEYWORD SEARCH (Fallback when embeddings unavailable)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE VIRTUAL TABLE IF NOT EXISTS templates_fts USING fts5(
    id,
    name,
    description,
    intent,
    intent_keywords,
    content=templates,
    content_rowid=rowid,
    tokenize='porter unicode61'
);

-- Triggers to keep FTS index synchronized
CREATE TRIGGER IF NOT EXISTS templates_fts_insert AFTER INSERT ON templates BEGIN
    INSERT INTO templates_fts(rowid, id, name, description, intent, intent_keywords)
    VALUES (new.rowid, new.id, new.name, new.description, new.intent, new.intent_keywords);
END;

CREATE TRIGGER IF NOT EXISTS templates_fts_delete AFTER DELETE ON templates BEGIN
    INSERT INTO templates_fts(templates_fts, rowid, id, name, description, intent, intent_keywords)
    VALUES ('delete', old.rowid, old.id, old.name, old.description, old.intent, old.intent_keywords);
END;

CREATE TRIGGER IF NOT EXISTS templates_fts_update AFTER UPDATE ON templates BEGIN
    INSERT INTO templates_fts(templates_fts, rowid, id, name, description, intent, intent_keywords)
    VALUES ('delete', old.rowid, old.id, old.name, old.description, old.intent, old.intent_keywords);
    INSERT INTO templates_fts(rowid, id, name, description, intent, intent_keywords)
    VALUES (new.rowid, new.id, new.name, new.description, new.intent, new.intent_keywords);
END;

-- ═══════════════════════════════════════════════════════════════════════════════
-- MIGRATION RECORD
-- ═══════════════════════════════════════════════════════════════════════════════

INSERT OR IGNORE INTO migrations (version, name) VALUES (4, 'cognitive_architecture_v1');
