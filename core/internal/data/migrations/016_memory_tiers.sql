-- Migration 016: Add memory tier to strategic_memory table
-- Part of Phase 3 RoamPal integration - Memory Tier Types

-- Add memory tier column with default value of 'tentative'
ALTER TABLE strategic_memory ADD COLUMN tier TEXT DEFAULT 'tentative';

-- Create index for efficient tier-based queries
CREATE INDEX idx_strategic_memory_tier ON strategic_memory(tier);
