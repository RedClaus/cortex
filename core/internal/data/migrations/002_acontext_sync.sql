-- Migration 002: Acontext Cloud Sync Support
-- Adds sync_cursors table for tracking incremental sync progress

CREATE TABLE IF NOT EXISTS sync_cursors (
    id TEXT PRIMARY KEY,                 -- 'default' for single-user, can support multiple profiles later
    last_pull_at DATETIME,               -- Last successful pull timestamp
    last_push_at DATETIME,               -- Last successful push timestamp
    server_cursor TEXT,                  -- Pagination cursor from Acontext API
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Insert default cursor
INSERT OR IGNORE INTO sync_cursors (id, updated_at) VALUES ('default', CURRENT_TIMESTAMP);
