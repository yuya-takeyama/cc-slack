-- Remove working_directory column from sessions table
-- SQLite doesn't support DROP COLUMN, so we need to recreate the table
CREATE TABLE sessions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL,
    session_id TEXT NOT NULL UNIQUE,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    status TEXT CHECK(status IN ('active', 'completed', 'failed', 'timeout')) DEFAULT 'active',
    model TEXT,
    total_cost_usd REAL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    duration_ms INTEGER,
    num_turns INTEGER,
    FOREIGN KEY (thread_id) REFERENCES threads(id)
);

-- Copy data to new table (excluding working_directory)
INSERT INTO sessions_new (
    id, thread_id, session_id, started_at, ended_at, status, 
    model, total_cost_usd, input_tokens, output_tokens, duration_ms, num_turns
)
SELECT 
    id, thread_id, session_id, started_at, ended_at, status,
    model, total_cost_usd, input_tokens, output_tokens, duration_ms, num_turns
FROM sessions;

-- Drop old table and rename new one
DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;

-- Recreate indexes
CREATE INDEX idx_sessions_thread_id ON sessions(thread_id);
CREATE INDEX idx_sessions_status ON sessions(status);