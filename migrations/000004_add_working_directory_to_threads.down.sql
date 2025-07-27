-- Recreate threads table without working_directory column
CREATE TABLE threads_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);

-- Copy data to new table (excluding working_directory)
INSERT INTO threads_new (id, channel_id, thread_ts, created_at, updated_at)
SELECT id, channel_id, thread_ts, created_at, updated_at
FROM threads;

-- Drop old table and rename new one
DROP TABLE threads;
ALTER TABLE threads_new RENAME TO threads;