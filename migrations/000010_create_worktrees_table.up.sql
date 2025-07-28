CREATE TABLE worktrees (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    current_branch TEXT,
    status TEXT NOT NULL DEFAULT 'active', -- active, archived, deleted
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (repository_id) REFERENCES repositories(id),
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    UNIQUE(thread_id)
);

-- Indexes for performance
CREATE INDEX idx_worktrees_repository_id ON worktrees(repository_id);
CREATE INDEX idx_worktrees_thread_id ON worktrees(thread_id);
CREATE INDEX idx_worktrees_status ON worktrees(status);
CREATE INDEX idx_worktrees_created_at ON worktrees(created_at);