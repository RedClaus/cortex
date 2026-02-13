-- ═══════════════════════════════════════════════════════════════════════════════
-- CORTEX KNOWLEDGE INGESTION SCHEMA v1.0
-- Multi-format document ingestion with semantic chunking and embedding indexing
-- ═══════════════════════════════════════════════════════════════════════════════
--
-- This migration adds the ingestion pipeline that enables:
-- 1. Multi-format document parsing (Markdown, Code, PDF, Text)
-- 2. Semantic chunking with content-addressable storage
-- 3. Embedding-based chunk indexing for similarity search
-- 4. Document metadata and relationship tracking
--
-- ═══════════════════════════════════════════════════════════════════════════════

-- ═══════════════════════════════════════════════════════════════════════════════
-- KNOWLEDGE SOURCES (Ingested Documents)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS knowledge_sources (
    -- Primary key
    id TEXT PRIMARY KEY,

    -- Identity
    name TEXT NOT NULL,                    -- Human-readable name
    description TEXT,                       -- What this document contains

    -- Source information
    source_type TEXT NOT NULL CHECK (source_type IN ('file', 'url', 'api', 'manual')),
    source_path TEXT,                       -- Original file path or URL
    format TEXT NOT NULL CHECK (format IN ('markdown', 'text', 'code', 'pdf', 'json', 'yaml')),

    -- Categorization
    category TEXT,                          -- 'terminal', 'networking', 'security', etc.
    tags TEXT DEFAULT '[]',                 -- JSON array of tags
    platform TEXT,                          -- 'macos', 'linux', 'windows', 'all'

    -- Versioning
    version TEXT,
    content_hash TEXT NOT NULL,             -- SHA256 of source content (deduplication)

    -- Status
    status TEXT NOT NULL DEFAULT 'active' CHECK (
        status IN ('active', 'archived', 'processing', 'error')
    ),
    chunk_count INTEGER DEFAULT 0 CHECK (chunk_count >= 0),

    -- Quality
    quality_score REAL DEFAULT 1.0 CHECK (quality_score >= 0 AND quality_score <= 1),

    -- Timestamps
    ingested_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at DATETIME
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- KNOWLEDGE CHUNKS (Individual Retrievable Units)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    -- Primary key
    id TEXT PRIMARY KEY,

    -- Source reference
    source_id TEXT NOT NULL REFERENCES knowledge_sources(id) ON DELETE CASCADE,

    -- Content
    content TEXT NOT NULL,                  -- The actual text content
    content_type TEXT NOT NULL CHECK (
        content_type IN ('text', 'code', 'command', 'example', 'definition', 'mixed')
    ),

    -- Content-Addressable Storage (CAS) - KEY FOR DEDUPLICATION
    content_hash TEXT NOT NULL,             -- SHA256 of chunk content (enables skip on re-ingest)

    -- Hierarchy
    parent_chunk_id TEXT REFERENCES knowledge_chunks(id),
    position INTEGER NOT NULL DEFAULT 0 CHECK (position >= 0),
    depth INTEGER NOT NULL DEFAULT 0 CHECK (depth >= 0),

    -- Metadata from source
    title TEXT,                             -- Section/heading title
    section_path TEXT,                      -- e.g., "21. SYSTEM DIAGNOSTICS > sysdiagnose"

    -- Embedding (with model tracking for drift detection)
    embedding BLOB,                         -- float32 array serialized (768 dims for nomic-embed-text), NULL if not available
    embedding_model TEXT NOT NULL DEFAULT 'nomic-embed-text',
    embedding_dim INTEGER NOT NULL DEFAULT 768,

    -- Extracted entities (for enhanced retrieval)
    commands TEXT DEFAULT '[]',             -- JSON array of commands mentioned
    keywords TEXT DEFAULT '[]',             -- JSON array of extracted keywords

    -- Offset tracking (for reconstruction)
    start_offset INTEGER DEFAULT 0,
    end_offset INTEGER DEFAULT 0,

    -- Usage tracking
    retrieval_count INTEGER DEFAULT 0 CHECK (retrieval_count >= 0),
    last_retrieved_at DATETIME,
    avg_relevance_score REAL,              -- Average similarity when retrieved

    -- Quality
    token_count INTEGER DEFAULT 0 CHECK (token_count >= 0),
    quality_score REAL DEFAULT 1.0 CHECK (quality_score >= 0 AND quality_score <= 1),

    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES FOR CHUNKS
-- ═══════════════════════════════════════════════════════════════════════════════

-- CRITICAL: CAS index for fast deduplication lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_content_hash ON knowledge_chunks(content_hash);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_chunks_source ON knowledge_chunks(source_id);
CREATE INDEX IF NOT EXISTS idx_chunks_type ON knowledge_chunks(content_type);
CREATE INDEX IF NOT EXISTS idx_chunks_title ON knowledge_chunks(title);
CREATE INDEX IF NOT EXISTS idx_chunks_embedding_model ON knowledge_chunks(embedding_model);
CREATE INDEX IF NOT EXISTS idx_chunks_quality ON knowledge_chunks(quality_score DESC);
CREATE INDEX IF NOT EXISTS idx_chunks_retrieval ON knowledge_chunks(retrieval_count DESC);

-- ═══════════════════════════════════════════════════════════════════════════════
-- FULL-TEXT SEARCH INDEX FOR CHUNKS
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_chunks_fts USING fts5(
    content,
    title,
    section_path,
    commands,
    keywords,
    content='knowledge_chunks',
    content_rowid='rowid',
    tokenize='porter unicode61'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS knowledge_chunks_fts_insert
AFTER INSERT ON knowledge_chunks BEGIN
    INSERT INTO knowledge_chunks_fts(rowid, content, title, section_path, commands, keywords)
    VALUES (new.rowid, new.content, new.title, new.section_path, new.commands, new.keywords);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_chunks_fts_delete
AFTER DELETE ON knowledge_chunks BEGIN
    INSERT INTO knowledge_chunks_fts(knowledge_chunks_fts, rowid, content, title, section_path, commands, keywords)
    VALUES ('delete', old.rowid, old.content, old.title, old.section_path, old.commands, old.keywords);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_chunks_fts_update
AFTER UPDATE ON knowledge_chunks BEGIN
    INSERT INTO knowledge_chunks_fts(knowledge_chunks_fts, rowid, content, title, section_path, commands, keywords)
    VALUES ('delete', old.rowid, old.content, old.title, old.section_path, old.commands, old.keywords);
    INSERT INTO knowledge_chunks_fts(rowid, content, title, section_path, commands, keywords)
    VALUES (new.rowid, new.content, new.title, new.section_path, new.commands, new.keywords);
END;

-- ═══════════════════════════════════════════════════════════════════════════════
-- KNOWLEDGE RETRIEVAL LOG (for analytics and learning)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS knowledge_retrieval_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Query details
    query TEXT NOT NULL,
    query_embedding BLOB,

    -- Retrieval results
    chunks_retrieved TEXT NOT NULL,         -- JSON array of chunk IDs
    top_similarity REAL,                    -- Highest similarity score
    retrieval_method TEXT CHECK (
        retrieval_method IN ('vector', 'keyword', 'hybrid')
    ),

    -- Performance
    latency_ms INTEGER,

    -- Context
    session_id TEXT,

    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_retrieval_log_created ON knowledge_retrieval_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_retrieval_log_session ON knowledge_retrieval_log(session_id);

-- ═══════════════════════════════════════════════════════════════════════════════
-- EMBEDDING MODELS REGISTRY (for drift detection)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS embedding_models (
    id TEXT PRIMARY KEY,                    -- e.g., "nomic-embed-text:v1.5"
    dimension INTEGER NOT NULL,             -- e.g., 768
    is_current BOOLEAN DEFAULT FALSE,       -- Currently active model
    chunks_using INTEGER DEFAULT 0,         -- Count of chunks using this model
    registered_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES FOR SOURCES
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE INDEX IF NOT EXISTS idx_sources_category ON knowledge_sources(category);
CREATE INDEX IF NOT EXISTS idx_sources_platform ON knowledge_sources(platform);
CREATE INDEX IF NOT EXISTS idx_sources_status ON knowledge_sources(status);
CREATE INDEX IF NOT EXISTS idx_sources_format ON knowledge_sources(format);
CREATE INDEX IF NOT EXISTS idx_sources_content_hash ON knowledge_sources(content_hash);

-- ═══════════════════════════════════════════════════════════════════════════════
-- VIEWS FOR MONITORING AND ANALYTICS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Knowledge health overview
CREATE VIEW IF NOT EXISTS v_knowledge_health AS
SELECT
    ks.id as source_id,
    ks.name,
    ks.category,
    ks.format,
    ks.chunk_count,
    ks.status,
    COUNT(kc.id) as actual_chunks,
    SUM(kc.retrieval_count) as total_retrievals,
    AVG(kc.avg_relevance_score) as avg_relevance,
    AVG(kc.quality_score) as avg_quality,
    MAX(kc.last_retrieved_at) as last_used
FROM knowledge_sources ks
LEFT JOIN knowledge_chunks kc ON ks.id = kc.source_id
GROUP BY ks.id;

-- Embedding drift detection
CREATE VIEW IF NOT EXISTS v_embedding_drift_status AS
SELECT
    embedding_model,
    COUNT(*) as chunk_count,
    AVG(quality_score) as avg_quality,
    CASE
        WHEN embedding_model != (SELECT id FROM embedding_models WHERE is_current = 1)
        THEN 'needs_reindex'
        ELSE 'current'
    END as status
FROM knowledge_chunks
GROUP BY embedding_model;

-- Top performing chunks (high retrieval, high relevance)
CREATE VIEW IF NOT EXISTS v_top_chunks AS
SELECT
    kc.*,
    ks.name as source_name,
    ks.category
FROM knowledge_chunks kc
JOIN knowledge_sources ks ON kc.source_id = ks.id
WHERE kc.retrieval_count > 0
ORDER BY
    kc.retrieval_count DESC,
    kc.avg_relevance_score DESC
LIMIT 100;

-- Low quality chunks (candidates for cleanup)
CREATE VIEW IF NOT EXISTS v_low_quality_chunks AS
SELECT
    kc.*,
    ks.name as source_name
FROM knowledge_chunks kc
JOIN knowledge_sources ks ON kc.source_id = ks.id
WHERE kc.quality_score < 0.5
ORDER BY kc.quality_score ASC;

-- ═══════════════════════════════════════════════════════════════════════════════
-- MIGRATION RECORD
-- ═══════════════════════════════════════════════════════════════════════════════

INSERT OR IGNORE INTO migrations (version, name) VALUES (7, 'knowledge_ingestion_v1');

-- ═══════════════════════════════════════════════════════════════════════════════
-- INITIAL DATA
-- ═══════════════════════════════════════════════════════════════════════════════

-- Register default embedding model
INSERT OR IGNORE INTO embedding_models (id, dimension, is_current, chunks_using)
VALUES ('nomic-embed-text', 768, 1, 0);
