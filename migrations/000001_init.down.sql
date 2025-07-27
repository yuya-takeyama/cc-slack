-- インデックスを削除
DROP INDEX IF EXISTS idx_sessions_status;
DROP INDEX IF EXISTS idx_sessions_thread_id;

-- テーブルを削除
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS threads;