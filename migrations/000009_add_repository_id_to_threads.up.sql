ALTER TABLE threads ADD COLUMN repository_id INTEGER REFERENCES repositories(id);

-- Index for faster lookups
CREATE INDEX idx_threads_repository_id ON threads(repository_id);