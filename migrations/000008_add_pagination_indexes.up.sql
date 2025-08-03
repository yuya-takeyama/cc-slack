-- Indexes for pagination performance
-- Threads pagination (ORDER BY updated_at DESC)
CREATE INDEX IF NOT EXISTS idx_threads_updated_at_desc ON threads(updated_at DESC);

-- Sessions pagination (ORDER BY started_at DESC)
CREATE INDEX IF NOT EXISTS idx_sessions_started_at_desc ON sessions(started_at DESC);

-- Sessions by thread pagination (thread_id filter + ORDER BY started_at DESC)
CREATE INDEX IF NOT EXISTS idx_sessions_thread_started_at ON sessions(thread_id, started_at DESC);