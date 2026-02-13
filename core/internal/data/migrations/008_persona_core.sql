-- ═══════════════════════════════════════════════════════════════════════════════
-- Migration 008: Persona Core System
-- ═══════════════════════════════════════════════════════════════════════════════
-- This migration creates the personas table for storing structured AI personas
-- with expertise domains, communication styles, and behavioral modes.
--
-- Each persona compiles into a system prompt at load time (zero runtime cost).
-- Built-in personas are protected from deletion.
-- ═══════════════════════════════════════════════════════════════════════════════

-- Personas table: Core persona definitions with structured metadata
CREATE TABLE IF NOT EXISTS personas (
    -- Primary key
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL DEFAULT '1.0',

    -- === IDENTITY ===
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    background TEXT,
    traits TEXT NOT NULL,           -- JSON array: ["methodical", "patient"]
    "values" TEXT NOT NULL,          -- JSON array: ["reliability", "clarity"] (quoted: reserved keyword)

    -- === EXPERTISE ===
    expertise TEXT NOT NULL,         -- JSON array of ExpertiseDomain objects

    -- === COMMUNICATION ===
    style TEXT NOT NULL,             -- JSON object: CommunicationStyle

    -- === BEHAVIORAL MODES ===
    modes TEXT NOT NULL,             -- JSON array of BehavioralMode objects
    default_mode TEXT,               -- ID of the default mode

    -- === KNOWLEDGE LINKS ===
    knowledge_source_ids TEXT,       -- JSON array of knowledge source IDs

    -- === COMPILED OUTPUT ===
    system_prompt TEXT NOT NULL,     -- Generated from structured fields, cached

    -- === METADATA ===
    is_built_in INTEGER NOT NULL DEFAULT 0,  -- Boolean: 1 = built-in, 0 = custom
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_personas_name ON personas(name);
CREATE INDEX IF NOT EXISTS idx_personas_is_built_in ON personas(is_built_in);
CREATE INDEX IF NOT EXISTS idx_personas_created_at ON personas(created_at);

-- ═══════════════════════════════════════════════════════════════════════════════
-- NOTES
-- ═══════════════════════════════════════════════════════════════════════════════
-- JSON Field Schemas:
--
-- traits: ["trait1", "trait2", ...]
--
-- values: ["value1", "value2", ...]
--
-- expertise: [
--   {
--     "domain": "kubernetes",
--     "depth": "expert",
--     "specialties": ["helm", "operators"],
--     "boundaries": ["application bugs"]
--   }
-- ]
--
-- style: {
--   "tone": "professional",
--   "verbosity": "adaptive",
--   "formatting": "markdown",
--   "patterns": ["pattern1", "pattern2"],
--   "avoids": ["avoid1", "avoid2"]
-- }
--
-- modes: [
--   {
--     "id": "normal",
--     "name": "Standard Assistance",
--     "description": "General help",
--     "prompt_augment": "...",
--     "entry_keywords": ["error", "debug"],
--     "exit_keywords": ["thanks", "solved"],
--     "manual_trigger": "debug mode",
--     "force_verbose": false,
--     "force_concise": false,
--     "sort_order": 0
--   }
-- ]
--
-- knowledge_source_ids: ["source-id-1", "source-id-2"]
-- ═══════════════════════════════════════════════════════════════════════════════
