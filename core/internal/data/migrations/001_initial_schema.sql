-- ═══════════════════════════════════════════════════════════════════════════════
-- CORTEX KNOWLEDGE DATABASE SCHEMA v3
-- Team-Ready, Local-First, Conflict-Resilient
-- ═══════════════════════════════════════════════════════════════════════════════
--
-- Location: ~/.cortex/knowledge.db
-- ⚠️  CRITICAL: Must be on LOCAL DISK, not network drive (SQLite + network = corruption)
--
-- Note: PRAGMAs are handled programmatically in db.go initPragmas()
-- ═══════════════════════════════════════════════════════════════════════════════

-- ═══════════════════════════════════════════════════════════════════════════════
-- CONFIGURATION
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Default configuration
INSERT OR IGNORE INTO config (key, value) VALUES 
    ('schema_version', '3'),
    ('user_id', ''),
    ('team_id', ''),
    ('user_name', '');

-- ═══════════════════════════════════════════════════════════════════════════════
-- KNOWLEDGE ITEMS (Core Table)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS knowledge_items (
    -- Primary key
    id TEXT PRIMARY KEY,
    
    -- Type classification
    type TEXT NOT NULL CHECK (type IN ('sop', 'lesson', 'pattern', 'session', 'document')),
    
    -- Content
    content TEXT NOT NULL,
    title TEXT,                      -- Optional human-readable title
    tags TEXT DEFAULT '[]',          -- JSON array: ["cisco", "vlan", "layer2"]
    
    -- ═══ TEAM ATTRIBUTION ═══
    scope TEXT NOT NULL DEFAULT 'personal' CHECK (scope IN ('global', 'team', 'personal')),
    team_id TEXT,                    -- NULL for personal, team UUID for team/global scope
    author_id TEXT NOT NULL,         -- Who created this
    author_name TEXT,                -- Human-readable name (denormalized for display)
    
    -- ═══ QUALITY SIGNALS ═══
    confidence REAL DEFAULT 0.5 CHECK (confidence >= 0 AND confidence <= 1),
    trust_score REAL DEFAULT 0.5 CHECK (trust_score >= 0 AND trust_score <= 1),
    success_count INTEGER DEFAULT 0 CHECK (success_count >= 0),
    failure_count INTEGER DEFAULT 0 CHECK (failure_count >= 0),
    feedback_positive INTEGER DEFAULT 0 CHECK (feedback_positive >= 0),
    feedback_negative INTEGER DEFAULT 0 CHECK (feedback_negative >= 0),
    access_count INTEGER DEFAULT 0 CHECK (access_count >= 0),
    
    -- ═══ SYNC METADATA ═══
    version INTEGER DEFAULT 1,                    -- Incremented on each local update
    remote_id TEXT,                               -- ID in Acontext (NULL if not synced yet)
    remote_version INTEGER,                       -- Last known remote version
    sync_status TEXT DEFAULT 'pending' CHECK (sync_status IN ('pending', 'synced', 'conflict', 'local_only')),
    last_synced_at DATETIME,
    
    -- ═══ TEMPORAL ═══
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at DATETIME,
    deleted_at DATETIME,                          -- Soft delete for sync
    
    -- ═══ TYPE-SPECIFIC METADATA (JSON) ═══
    metadata TEXT DEFAULT '{}'                    -- Flexible JSON for type-specific data
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- FULL-TEXT SEARCH INDEX
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
    id,
    title,
    content,
    tags,
    content=knowledge_items,
    content_rowid=rowid,
    tokenize='porter unicode61'       -- Porter stemming for better search
);

-- Triggers to keep FTS index synchronized
CREATE TRIGGER IF NOT EXISTS knowledge_fts_insert AFTER INSERT ON knowledge_items BEGIN
    INSERT INTO knowledge_fts(rowid, id, title, content, tags) 
    VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_fts_delete AFTER DELETE ON knowledge_items BEGIN
    INSERT INTO knowledge_fts(knowledge_fts, rowid, id, title, content, tags) 
    VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS knowledge_fts_update AFTER UPDATE ON knowledge_items BEGIN
    INSERT INTO knowledge_fts(knowledge_fts, rowid, id, title, content, tags) 
    VALUES ('delete', old.rowid, old.id, old.title, old.content, old.tags);
    INSERT INTO knowledge_fts(rowid, id, title, content, tags) 
    VALUES (new.rowid, new.id, new.title, new.content, new.tags);
END;

-- ═══════════════════════════════════════════════════════════════════════════════
-- SOP-SPECIFIC TABLES
-- ═══════════════════════════════════════════════════════════════════════════════

-- SOP Steps (normalized for complex SOPs)
CREATE TABLE IF NOT EXISTS sop_steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sop_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL,
    description TEXT NOT NULL,
    command TEXT,                    -- Optional command to execute
    expected_output TEXT,            -- What success looks like
    rollback_command TEXT,           -- How to undo this step
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(sop_id, step_order)
);

-- Link lessons to SOPs (for inline warnings)
CREATE TABLE IF NOT EXISTS sop_lesson_links (
    sop_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    lesson_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    relevance REAL DEFAULT 1.0 CHECK (relevance >= 0 AND relevance <= 1),
    link_type TEXT DEFAULT 'warning' CHECK (link_type IN ('warning', 'prerequisite', 'followup')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (sop_id, lesson_id)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- TRUST PROFILES
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS trust_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    domain TEXT NOT NULL,            -- "cisco", "linux", "python", "aws", etc.
    
    -- Trust metrics
    score REAL DEFAULT 0.5 CHECK (score >= 0 AND score <= 1),
    success_count INTEGER DEFAULT 0 CHECK (success_count >= 0),
    failure_count INTEGER DEFAULT 0 CHECK (failure_count >= 0),
    
    -- Verification thresholds (can be customized per domain)
    auto_approve_threshold REAL DEFAULT 0.7,
    
    -- Temporal
    last_activity DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, domain)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- SESSIONS (Conversation Memory)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    
    -- Session context
    title TEXT,                      -- Auto-generated or user-provided
    platform_vendor TEXT,            -- "cisco", "linux", etc.
    platform_name TEXT,              -- "ios-xe", "ubuntu", etc.
    platform_version TEXT,
    working_directory TEXT,
    
    -- Acontext integration
    remote_session_id TEXT,          -- ID in Acontext
    space_id TEXT,                   -- Acontext Space ID
    
    -- State
    status TEXT DEFAULT 'active' CHECK (status IN ('active', 'completed', 'abandoned')),
    
    -- Temporal
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at DATETIME,
    last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Session messages
CREATE TABLE IF NOT EXISTS session_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content TEXT NOT NULL,
    
    -- Tool execution details
    tool_name TEXT,
    tool_input TEXT,                 -- JSON
    tool_output TEXT,
    tool_success INTEGER,            -- 0 or 1
    
    -- Temporal
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- SYNC MANAGEMENT
-- ═══════════════════════════════════════════════════════════════════════════════

-- Pending sync operations (queue)
CREATE TABLE IF NOT EXISTS sync_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id TEXT NOT NULL,
    item_type TEXT NOT NULL,         -- 'knowledge', 'session', etc.
    operation TEXT NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    
    -- Retry management
    queued_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 5,
    last_attempt_at DATETIME,
    last_error TEXT,
    
    -- Priority (lower = higher priority)
    priority INTEGER DEFAULT 5,
    
    UNIQUE(item_id, operation)
);

-- Sync state tracking
CREATE TABLE IF NOT EXISTS sync_state (
    scope TEXT PRIMARY KEY,          -- 'personal', 'team', 'global'
    team_id TEXT,
    last_sync_at DATETIME,
    last_sync_cursor TEXT,           -- Pagination cursor from Acontext
    items_synced INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0
);

-- Conflict log (for debugging and manual resolution)
CREATE TABLE IF NOT EXISTS sync_conflicts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id TEXT NOT NULL,
    local_version INTEGER,
    remote_version INTEGER,
    local_content TEXT,              -- JSON snapshot
    remote_content TEXT,             -- JSON snapshot
    resolution TEXT CHECK (resolution IN ('pending', 'local_wins', 'remote_wins', 'manual')),
    resolved_at DATETIME,
    resolved_by TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- METRICS AND ANALYTICS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Daily metrics rollup
CREATE TABLE IF NOT EXISTS daily_metrics (
    date TEXT NOT NULL,              -- YYYY-MM-DD
    metric TEXT NOT NULL,            -- 'lessons_created', 'commands_executed', etc.
    value REAL DEFAULT 0,
    
    PRIMARY KEY (date, metric)
);

-- Router statistics
CREATE TABLE IF NOT EXISTS router_stats (
    date TEXT PRIMARY KEY,           -- YYYY-MM-DD
    fast_hits INTEGER DEFAULT 0,
    slow_hits INTEGER DEFAULT 0,
    ambiguous_hits INTEGER DEFAULT 0,
    avg_classification_ms REAL
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES
-- ═══════════════════════════════════════════════════════════════════════════════

-- Knowledge items
CREATE INDEX IF NOT EXISTS idx_knowledge_type ON knowledge_items(type);
CREATE INDEX IF NOT EXISTS idx_knowledge_scope ON knowledge_items(scope);
CREATE INDEX IF NOT EXISTS idx_knowledge_team ON knowledge_items(team_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_author ON knowledge_items(author_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_sync_status ON knowledge_items(sync_status);
CREATE INDEX IF NOT EXISTS idx_knowledge_deleted ON knowledge_items(deleted_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_updated ON knowledge_items(updated_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_confidence ON knowledge_items(confidence DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_trust ON knowledge_items(trust_score DESC);

-- Trust profiles
CREATE INDEX IF NOT EXISTS idx_trust_user ON trust_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_trust_domain ON trust_profiles(domain);
CREATE INDEX IF NOT EXISTS idx_trust_score ON trust_profiles(score DESC);

-- Sessions
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_platform ON sessions(platform_vendor);

-- Session messages
CREATE INDEX IF NOT EXISTS idx_messages_session ON session_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_role ON session_messages(role);

-- Sync queue
CREATE INDEX IF NOT EXISTS idx_sync_queue_pending ON sync_queue(queued_at) 
    WHERE attempts < max_attempts;
CREATE INDEX IF NOT EXISTS idx_sync_queue_priority ON sync_queue(priority, queued_at);

-- ═══════════════════════════════════════════════════════════════════════════════
-- VIEWS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Active (non-deleted) knowledge items
CREATE VIEW IF NOT EXISTS v_active_knowledge AS
SELECT * FROM knowledge_items WHERE deleted_at IS NULL;

-- SOPs with warning counts
CREATE VIEW IF NOT EXISTS v_sops_with_warnings AS
SELECT 
    k.*,
    COUNT(sll.lesson_id) as warning_count
FROM knowledge_items k
LEFT JOIN sop_lesson_links sll ON k.id = sll.sop_id
WHERE k.type = 'sop' AND k.deleted_at IS NULL
GROUP BY k.id;

-- Items pending sync
CREATE VIEW IF NOT EXISTS v_pending_sync AS
SELECT 
    k.*, 
    q.operation, 
    q.attempts,
    q.last_error
FROM knowledge_items k
JOIN sync_queue q ON k.id = q.item_id
WHERE q.attempts < q.max_attempts
ORDER BY q.priority, q.queued_at;

-- Trust leaderboard
CREATE VIEW IF NOT EXISTS v_trust_leaderboard AS
SELECT 
    domain, 
    user_id, 
    score,
    success_count,
    failure_count,
    CASE 
        WHEN (success_count + failure_count) > 0 
        THEN ROUND(100.0 * success_count / (success_count + failure_count), 1)
        ELSE 50.0 
    END as success_rate_pct
FROM trust_profiles
ORDER BY domain, score DESC;

-- Recent activity
CREATE VIEW IF NOT EXISTS v_recent_activity AS
SELECT 
    id,
    type,
    title,
    scope,
    author_name,
    updated_at,
    'knowledge' as source
FROM knowledge_items
WHERE deleted_at IS NULL
UNION ALL
SELECT 
    id,
    'session' as type,
    title,
    'personal' as scope,
    NULL as author_name,
    last_activity_at as updated_at,
    'session' as source
FROM sessions
WHERE status = 'active'
ORDER BY updated_at DESC
LIMIT 50;

-- ═══════════════════════════════════════════════════════════════════════════════
-- TRIGGERS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Note: updated_at and version are handled in Go code to avoid recursive triggers.
-- SQLite doesn't support BEFORE UPDATE triggers that modify NEW values.

-- Auto-queue for sync on insert
CREATE TRIGGER IF NOT EXISTS knowledge_sync_on_insert
AFTER INSERT ON knowledge_items
WHEN new.scope != 'global'  -- Don't sync global (read-only from cloud)
BEGIN
    INSERT OR REPLACE INTO sync_queue (item_id, item_type, operation, priority)
    VALUES (new.id, 'knowledge', 'create', 
            CASE new.scope WHEN 'team' THEN 3 ELSE 5 END);
END;

-- Auto-queue for sync on update
CREATE TRIGGER IF NOT EXISTS knowledge_sync_on_update
AFTER UPDATE ON knowledge_items
WHEN new.scope != 'global' AND old.content != new.content
BEGIN
    INSERT OR REPLACE INTO sync_queue (item_id, item_type, operation, priority)
    VALUES (new.id, 'knowledge', 'update',
            CASE new.scope WHEN 'team' THEN 3 ELSE 5 END);
END;

-- Auto-queue for sync on delete
CREATE TRIGGER IF NOT EXISTS knowledge_sync_on_delete
AFTER UPDATE ON knowledge_items
WHEN new.deleted_at IS NOT NULL AND old.deleted_at IS NULL AND new.scope != 'global'
BEGIN
    INSERT OR REPLACE INTO sync_queue (item_id, item_type, operation, priority)
    VALUES (new.id, 'knowledge', 'delete', 1);
END;

-- Note: trust_profiles updated_at is handled in Go code to avoid recursive triggers.

-- ═══════════════════════════════════════════════════════════════════════════════
-- SAMPLE DATA (for testing)
-- ═══════════════════════════════════════════════════════════════════════════════

-- Uncomment to seed with test data
/*
INSERT INTO knowledge_items (id, type, title, content, tags, scope, author_id, author_name, confidence)
VALUES 
    ('sop-001', 'sop', 'Cisco VLAN Configuration', 
     'Standard procedure for configuring VLANs on Cisco switches...', 
     '["cisco", "vlan", "layer2", "network"]', 
     'team', 'user-001', 'Alice', 0.9),
    
    ('lesson-001', 'lesson', 'Check trunk mode before VLAN config',
     'When: Configuring VLANs on Cisco switches\nDo: Run "show interfaces trunk" first\nAvoid: Applying VLAN to access port\nBecause: VLAN config on access port will fail silently',
     '["cisco", "vlan", "troubleshooting"]',
     'team', 'user-002', 'Bob', 0.85);

INSERT INTO sop_lesson_links (sop_id, lesson_id, relevance, link_type)
VALUES ('sop-001', 'lesson-001', 0.95, 'warning');

INSERT INTO trust_profiles (user_id, domain, score, success_count, failure_count)
VALUES 
    ('user-001', 'cisco', 0.82, 45, 10),
    ('user-001', 'linux', 0.91, 120, 12),
    ('user-002', 'cisco', 0.75, 30, 10);
*/

-- ═══════════════════════════════════════════════════════════════════════════════
-- MIGRATION SUPPORT
-- ═══════════════════════════════════════════════════════════════════════════════

-- Store migration history
CREATE TABLE IF NOT EXISTS migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO migrations (version, name) VALUES (3, 'initial_v3_team_sync');
