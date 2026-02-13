-- ═══════════════════════════════════════════════════════════════════════════════
-- ROUTING OUTCOMES FOR ROAMPAL LEARNING
-- Migration: 015_routing_outcomes
-- Purpose: Add routing decision tracking to conversation_logs for adaptive learning
-- ═══════════════════════════════════════════════════════════════════════════════

-- ═══════════════════════════════════════════════════════════════════════════════
-- ADD ROUTING COLUMNS TO CONVERSATION_LOGS
-- These columns capture the AutoLLM routing decision for each request
-- ═══════════════════════════════════════════════════════════════════════════════

-- Lane used: "fast" (local) or "smart" (frontier)
ALTER TABLE conversation_logs ADD COLUMN routing_lane TEXT CHECK (
    routing_lane IS NULL OR routing_lane IN ('fast', 'smart')
);

-- Reason for lane selection (e.g., "complexity_low", "tool_required", "vision_needed")
ALTER TABLE conversation_logs ADD COLUMN routing_reason TEXT;

-- Whether the routing was forced by a constraint (not based on heuristics)
ALTER TABLE conversation_logs ADD COLUMN routing_forced INTEGER DEFAULT 0;

-- The constraint that forced routing (e.g., "vision", "context_overflow", "user_override")
ALTER TABLE conversation_logs ADD COLUMN routing_constraint TEXT;

-- Quality score for this response outcome (0-1, for learning signal)
ALTER TABLE conversation_logs ADD COLUMN outcome_score REAL CHECK (
    outcome_score IS NULL OR (outcome_score >= 0 AND outcome_score <= 1)
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES FOR ROUTING ANALYSIS
-- ═══════════════════════════════════════════════════════════════════════════════

-- Index for querying by lane (fast vs smart distribution)
CREATE INDEX IF NOT EXISTS idx_conv_logs_routing_lane ON conversation_logs(routing_lane)
    WHERE routing_lane IS NOT NULL;

-- Index for forced routing analysis
CREATE INDEX IF NOT EXISTS idx_conv_logs_routing_forced ON conversation_logs(routing_forced)
    WHERE routing_forced = 1;

-- Composite index for learning queries: lane + success + outcome_score
CREATE INDEX IF NOT EXISTS idx_conv_logs_routing_learning ON conversation_logs(
    routing_lane, success, outcome_score
) WHERE routing_lane IS NOT NULL;
