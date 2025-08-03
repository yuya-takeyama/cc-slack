-- Drop pagination indexes
DROP INDEX IF EXISTS idx_threads_updated_at_desc;
DROP INDEX IF EXISTS idx_sessions_started_at_desc;
DROP INDEX IF EXISTS idx_sessions_thread_started_at;