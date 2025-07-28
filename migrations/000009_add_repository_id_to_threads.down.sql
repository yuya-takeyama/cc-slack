DROP INDEX IF EXISTS idx_threads_repository_id;
ALTER TABLE threads DROP COLUMN repository_id;