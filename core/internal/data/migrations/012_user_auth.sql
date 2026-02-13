-- ═══════════════════════════════════════════════════════════════════════════════
-- Migration 012: User Authentication & Persona Assignment
-- ═══════════════════════════════════════════════════════════════════════════════
-- This migration creates tables for user authentication and user-persona
-- assignments. Enables multi-user support with persona selection.
--
-- Security Notes:
-- - Passwords are stored as bcrypt hashes (cost 12)
-- - Refresh tokens are stored as SHA-256 hashes
-- - Access tokens are JWTs (not stored in DB)
-- ═══════════════════════════════════════════════════════════════════════════════

-- Users table: Core user accounts
CREATE TABLE IF NOT EXISTS users (
    -- Primary key (UUID)
    id TEXT PRIMARY KEY,

    -- === CREDENTIALS ===
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,    -- bcrypt hash
    email TEXT,

    -- === METADATA ===
    display_name TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,  -- Boolean: 1 = active, 0 = disabled
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User-Persona assignments: Links users to their assigned personas
CREATE TABLE IF NOT EXISTS user_personas (
    -- Composite primary key
    user_id TEXT NOT NULL,
    persona_id TEXT NOT NULL,

    -- === ASSIGNMENT METADATA ===
    is_default INTEGER NOT NULL DEFAULT 0,  -- Boolean: 1 = default persona for user
    assigned_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- === CONSTRAINTS ===
    PRIMARY KEY (user_id, persona_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE CASCADE
);

-- User sessions: Stores refresh token hashes for session management
CREATE TABLE IF NOT EXISTS user_sessions (
    -- Primary key (UUID)
    id TEXT PRIMARY KEY,

    -- === SESSION DATA ===
    user_id TEXT NOT NULL,
    refresh_token_hash TEXT NOT NULL,  -- SHA-256 hash of refresh token

    -- === EXPIRATION ===
    expires_at DATETIME NOT NULL,

    -- === METADATA ===
    user_agent TEXT,         -- Browser/client info
    ip_address TEXT,         -- Last known IP
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- === CONSTRAINTS ===
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- ═══════════════════════════════════════════════════════════════════════════════
-- INDEXES
-- ═══════════════════════════════════════════════════════════════════════════════

-- Users indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);

-- User-Persona indexes
CREATE INDEX IF NOT EXISTS idx_user_personas_user ON user_personas(user_id);
CREATE INDEX IF NOT EXISTS idx_user_personas_persona ON user_personas(persona_id);
CREATE INDEX IF NOT EXISTS idx_user_personas_default ON user_personas(user_id, is_default);

-- User sessions indexes
CREATE INDEX IF NOT EXISTS idx_user_sessions_user ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(refresh_token_hash);

-- ═══════════════════════════════════════════════════════════════════════════════
-- TRIGGER: Update updated_at on users table
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TRIGGER IF NOT EXISTS users_updated_at
    AFTER UPDATE ON users
    FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- ═══════════════════════════════════════════════════════════════════════════════
-- TRIGGER: Ensure only one default persona per user
-- ═══════════════════════════════════════════════════════════════════════════════
CREATE TRIGGER IF NOT EXISTS user_personas_single_default
    BEFORE INSERT ON user_personas
    WHEN NEW.is_default = 1
BEGIN
    UPDATE user_personas SET is_default = 0 WHERE user_id = NEW.user_id AND is_default = 1;
END;

CREATE TRIGGER IF NOT EXISTS user_personas_single_default_update
    BEFORE UPDATE ON user_personas
    WHEN NEW.is_default = 1 AND OLD.is_default = 0
BEGIN
    UPDATE user_personas SET is_default = 0 WHERE user_id = NEW.user_id AND is_default = 1;
END;
