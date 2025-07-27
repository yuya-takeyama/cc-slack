-- Add working_directory column to threads table
ALTER TABLE threads ADD COLUMN working_directory TEXT;

-- Update existing threads with working_directory from their sessions
-- This uses a subquery to get the working_directory from the most recent session
UPDATE threads 
SET working_directory = (
    SELECT s.working_directory 
    FROM sessions s 
    WHERE s.thread_id = threads.id 
    ORDER BY s.started_at DESC 
    LIMIT 1
)
WHERE EXISTS (
    SELECT 1 
    FROM sessions s 
    WHERE s.thread_id = threads.id
);

-- Set default for threads without sessions (if any)
UPDATE threads 
SET working_directory = '/tmp'
WHERE working_directory IS NULL;

-- Make the column NOT NULL after populating data
-- SQLite doesn't support ALTER COLUMN, so we need to recreate the table
CREATE TABLE threads_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    working_directory TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);

-- Copy data to new table
INSERT INTO threads_new (id, channel_id, thread_ts, working_directory, created_at, updated_at)
SELECT id, channel_id, thread_ts, working_directory, created_at, updated_at
FROM threads;

-- Drop old table and rename new one
DROP TABLE threads;
ALTER TABLE threads_new RENAME TO threads;

-- Recreate indexes
CREATE INDEX idx_threads_channel_thread ON threads(channel_id, thread_ts);