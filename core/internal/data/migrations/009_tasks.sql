-- Task management tables (CR-007)
-- Integrates with existing Cortex SQLite database

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'todo',  -- todo, doing, blocked, done
    priority TEXT DEFAULT 'medium',        -- low, medium, high, critical
    project_id TEXT,                       -- Links to projects table
    parent_id TEXT,                        -- Subtask support
    estimate INTEGER DEFAULT 0,            -- Estimated time in minutes
    actual_time INTEGER DEFAULT 0,         -- Actual time spent in minutes
    tags TEXT DEFAULT '',                  -- Comma-separated tags
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    due_date DATETIME,

    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL,
    FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id TEXT NOT NULL,
    depends_on_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (task_id, depends_on_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_deps_task ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_deps_depends ON task_dependencies(depends_on_id);
