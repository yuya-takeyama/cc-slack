-- Add working_directory column back to sessions table
CREATE TABLE sessions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id INTEGER NOT NULL,
    session_id TEXT NOT NULL UNIQUE,
    working_directory TEXT NOT NULL,
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

-- Copy data back with working_directory from threads table
INSERT INTO sessions_new (
    id, thread_id, session_id, working_directory, started_at, ended_at, status,
    model, total_cost_usd, input_tokens, output_tokens, duration_ms, num_turns
)
SELECT 
    s.id, s.thread_id, s.session_id, t.working_directory, s.started_at, s.ended_at, s.status,
    s.model, s.total_cost_usd, s.input_tokens, s.output_tokens, s.duration_ms, s.num_turns
FROM sessions s
JOIN threads t ON s.thread_id = t.id;

-- Drop old table and rename new one
DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;

-- Recreate indexes
CREATE INDEX idx_sessions_thread_id ON sessions(thread_id);
CREATE INDEX idx_sessions_status ON sessions(status);