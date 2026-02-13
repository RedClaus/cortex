-- Migration 013: Lessons for Lingo
-- Stores Spanish learning lessons with message history for context recall

-- Lessons table (conversation sessions for learning)
CREATE TABLE IF NOT EXISTS lessons (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    persona_id TEXT NOT NULL,
    title TEXT,
    summary TEXT,
    status TEXT DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    message_count INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Lesson messages
CREATE TABLE IF NOT EXISTS lesson_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    lesson_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (lesson_id) REFERENCES lessons(id) ON DELETE CASCADE
);

-- Indexes for fast retrieval
CREATE INDEX IF NOT EXISTS idx_lessons_user ON lessons(user_id);
CREATE INDEX IF NOT EXISTS idx_lessons_persona ON lessons(persona_id);
CREATE INDEX IF NOT EXISTS idx_lessons_user_persona ON lessons(user_id, persona_id);
CREATE INDEX IF NOT EXISTS idx_lessons_updated ON lessons(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_lessons_status ON lessons(status);
CREATE INDEX IF NOT EXISTS idx_lesson_messages_lesson ON lesson_messages(lesson_id);

-- FTS for searching lesson content
CREATE VIRTUAL TABLE IF NOT EXISTS lessons_fts USING fts5(
    title,
    summary,
    content='lessons',
    content_rowid='rowid',
    tokenize='porter'
);

-- FTS for searching message content
CREATE VIRTUAL TABLE IF NOT EXISTS lesson_messages_fts USING fts5(
    content,
    content='lesson_messages',
    content_rowid='id',
    tokenize='porter'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS lessons_ai AFTER INSERT ON lessons BEGIN
    INSERT INTO lessons_fts(rowid, title, summary) VALUES (NEW.rowid, NEW.title, NEW.summary);
END;

CREATE TRIGGER IF NOT EXISTS lessons_ad AFTER DELETE ON lessons BEGIN
    INSERT INTO lessons_fts(lessons_fts, rowid, title, summary) VALUES ('delete', OLD.rowid, OLD.title, OLD.summary);
END;

CREATE TRIGGER IF NOT EXISTS lessons_au AFTER UPDATE ON lessons BEGIN
    INSERT INTO lessons_fts(lessons_fts, rowid, title, summary) VALUES ('delete', OLD.rowid, OLD.title, OLD.summary);
    INSERT INTO lessons_fts(rowid, title, summary) VALUES (NEW.rowid, NEW.title, NEW.summary);
END;

CREATE TRIGGER IF NOT EXISTS lesson_messages_ai AFTER INSERT ON lesson_messages BEGIN
    INSERT INTO lesson_messages_fts(rowid, content) VALUES (NEW.id, NEW.content);
    UPDATE lessons SET message_count = message_count + 1, updated_at = CURRENT_TIMESTAMP WHERE id = NEW.lesson_id;
END;

CREATE TRIGGER IF NOT EXISTS lesson_messages_ad AFTER DELETE ON lesson_messages BEGIN
    INSERT INTO lesson_messages_fts(lesson_messages_fts, rowid, content) VALUES ('delete', OLD.id, OLD.content);
END;
