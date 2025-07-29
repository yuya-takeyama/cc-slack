-- Refactor to config-based repository management

-- Step 1: Create new worktrees table without repository_id
CREATE TABLE worktrees_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_path TEXT NOT NULL,
    repository_name TEXT NOT NULL,
    thread_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    current_branch TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    UNIQUE(thread_id)
);

-- Step 2: Migrate data from old worktrees table
-- Note: We need to join with repositories table to get the path
INSERT INTO worktrees_new (id, repository_path, repository_name, thread_id, path, base_branch, current_branch, status, created_at, deleted_at)
SELECT 
    w.id,
    r.path as repository_path,
    r.name as repository_name,
    w.thread_id,
    w.path,
    w.base_branch,
    w.current_branch,
    w.status,
    w.created_at,
    w.deleted_at
FROM worktrees w
JOIN repositories r ON w.repository_id = r.id;

-- Step 3: Drop old worktrees table
DROP TABLE worktrees;

-- Step 4: Rename new table to worktrees
ALTER TABLE worktrees_new RENAME TO worktrees;

-- Step 5: Create index on repository_path for performance
CREATE INDEX idx_worktrees_repository_path ON worktrees(repository_path);

-- Step 6: Remove repository_id from threads table
CREATE TABLE threads_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);

INSERT INTO threads_new (id, channel_id, thread_ts, working_directory, created_at, updated_at)
SELECT id, channel_id, thread_ts, working_directory, created_at, updated_at
FROM threads;

DROP TABLE threads;
ALTER TABLE threads_new RENAME TO threads;

-- Step 7: Drop repositories table (no longer needed)
DROP TABLE repositories;