-- ═══════════════════════════════════════════════════════════════════════════════
-- CORTEX CONVERSATION LOGGING AND MODEL EVALUATION SCHEMA v1.0
-- Migration: 005_conversation_eval
-- ═══════════════════════════════════════════════════════════════════════════════

-- ═══════════════════════════════════════════════════════════════════════════════
-- CONVERSATION LOGS
-- Core table for tracking all LLM interactions
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS conversation_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Request identification
    request_id TEXT NOT NULL UNIQUE,
    session_id TEXT,
    parent_request_id TEXT,

    -- Model information
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    model_tier TEXT CHECK (
        model_tier IN ('small', 'medium', 'large', 'xl', 'frontier')
    ),

    -- Request details
    prompt TEXT NOT NULL,
    system_prompt TEXT,
    context_tokens INTEGER,

    -- Response details
    response TEXT,
    completion_tokens INTEGER,
    total_tokens INTEGER,

    -- Performance metrics
    duration_ms INTEGER,
    time_to_first_token_ms INTEGER,

    -- Task classification
    task_type TEXT,
    complexity_score INTEGER,

    -- Outcome flags
    success INTEGER DEFAULT 1,
    error_code TEXT,
    error_message TEXT,

    -- Issue detection flags (computed by assessor)
    had_timeout INTEGER DEFAULT 0,
    had_repetition INTEGER DEFAULT 0,
    had_tool_failure INTEGER DEFAULT 0,
    had_truncation INTEGER DEFAULT 0,
    had_json_error INTEGER DEFAULT 0,

    -- Assessment metadata
    capability_score REAL,
    recommended_upgrade TEXT,
    assessment_reason TEXT,

    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    assessed_at DATETIME
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- MODEL METRICS
-- Aggregated per-model performance metrics (daily rollup)
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS model_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Model identification
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    model_tier TEXT,

    -- Date for daily rollup
    date TEXT NOT NULL,

    -- Request counts
    total_requests INTEGER DEFAULT 0,
    successful_requests INTEGER DEFAULT 0,
    failed_requests INTEGER DEFAULT 0,

    -- Issue counts
    timeout_count INTEGER DEFAULT 0,
    repetition_count INTEGER DEFAULT 0,
    tool_failure_count INTEGER DEFAULT 0,
    truncation_count INTEGER DEFAULT 0,
    json_error_count INTEGER DEFAULT 0,

    -- Performance aggregates
    total_duration_ms INTEGER DEFAULT 0,
    min_duration_ms INTEGER,
    max_duration_ms INTEGER,
    avg_duration_ms REAL,

    -- Token usage
    total_prompt_tokens INTEGER DEFAULT 0,
    total_completion_tokens INTEGER DEFAULT 0,
    avg_tokens_per_request REAL,

    -- Capability assessment
    avg_capability_score REAL,
    upgrade_recommendations INTEGER DEFAULT 0,

    -- Task type breakdown (JSON)
    task_type_breakdown TEXT DEFAULT '{}',

    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(provider, model, date)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- MODEL UPGRADE EVENTS
-- History of upgrade recommendations and user actions
-- ═══════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS model_upgrade_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,

    -- Reference
    conversation_log_id INTEGER REFERENCES conversation_logs(id) ON DELETE SET NULL,
    request_id TEXT,

    -- Current model
    from_provider TEXT NOT NULL,
    from_model TEXT NOT NULL,
    from_tier TEXT,

    -- Recommended model
    to_provider TEXT NOT NULL,
    to_model TEXT NOT NULL,
    to_tier TEXT,

    -- Reason
    reason TEXT NOT NULL,
    issue_type TEXT NOT NULL CHECK (
        issue_type IN ('timeout', 'repetition', 'tool_failure', 'complexity', 'truncation', 'json_error')
    ),
    capability_score REAL,

    -- User action
    user_action TEXT CHECK (
        user_action IN ('accepted', 'dismissed', 'ignored', 'pending')
    ) DEFAULT 'pending',

    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES
-- ═══════════════════════════════════════════════════════════════════════════════

-- Conversation logs
CREATE INDEX IF NOT EXISTS idx_conv_logs_request ON conversation_logs(request_id);
CREATE INDEX IF NOT EXISTS idx_conv_logs_session ON conversation_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_conv_logs_model ON conversation_logs(provider, model);
CREATE INDEX IF NOT EXISTS idx_conv_logs_created ON conversation_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conv_logs_issues ON conversation_logs(had_timeout, had_repetition, had_tool_failure);

-- Model metrics
CREATE INDEX IF NOT EXISTS idx_model_metrics_lookup ON model_metrics(provider, model, date);
CREATE INDEX IF NOT EXISTS idx_model_metrics_date ON model_metrics(date DESC);

-- Upgrade events
CREATE INDEX IF NOT EXISTS idx_upgrade_pending ON model_upgrade_events(user_action)
    WHERE user_action = 'pending';
CREATE INDEX IF NOT EXISTS idx_upgrade_created ON model_upgrade_events(created_at DESC);
