-- Migration 017: Create routing_edges table for learned model performance
-- This table stores routing knowledge for the AutoLLM router to learn
-- which models perform best for different task types.

-- Create routing_edges table for learned model performance
CREATE TABLE IF NOT EXISTS routing_edges (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    task_type TEXT NOT NULL,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    total_latency_ms INTEGER DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, model, task_type)
);

-- Index for looking up edges by task type (common query pattern)
CREATE INDEX IF NOT EXISTS idx_routing_edges_task ON routing_edges(task_type);

-- Index for looking up edges by provider/model combination
CREATE INDEX IF NOT EXISTS idx_routing_edges_model ON routing_edges(provider, model);
