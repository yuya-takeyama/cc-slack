-- Rollback to database-based repository management

-- Step 1: Recreate repositories table
CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    default_branch TEXT DEFAULT 'main',
    slack_channel_id TEXT,
    slack_username TEXT,
    slack_icon_emoji TEXT,
    slack_icon_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Insert unique repositories from worktrees
INSERT INTO repositories (name, path, default_branch)
SELECT DISTINCT repository_name, repository_path, 'main'
FROM worktrees;

-- Step 3: Add repository_id back to threads table
CREATE TABLE threads_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    repository_id INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (repository_id) REFERENCES repositories(id),
    UNIQUE(channel_id, thread_ts)
);

INSERT INTO threads_new (id, channel_id, thread_ts, working_directory, created_at, updated_at)
SELECT id, channel_id, thread_ts, working_directory, created_at, updated_at
FROM threads;

DROP TABLE threads;
ALTER TABLE threads_new RENAME TO threads;

-- Step 4: Recreate old worktrees table with repository_id
CREATE TABLE worktrees_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id INTEGER NOT NULL,
    thread_id INTEGER NOT NULL,
    path TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    current_branch TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (repository_id) REFERENCES repositories(id),
    FOREIGN KEY (thread_id) REFERENCES threads(id),
    UNIQUE(thread_id)
);

-- Step 5: Migrate data back with repository_id
INSERT INTO worktrees_new (id, repository_id, thread_id, path, base_branch, current_branch, status, created_at, deleted_at)
SELECT 
    w.id,
    r.id as repository_id,
    w.thread_id,
    w.path,
    w.base_branch,
    w.current_branch,
    w.status,
    w.created_at,
    w.deleted_at
FROM worktrees w
JOIN repositories r ON w.repository_path = r.path;

-- Step 6: Drop and rename
DROP TABLE worktrees;
ALTER TABLE worktrees_new RENAME TO worktrees;