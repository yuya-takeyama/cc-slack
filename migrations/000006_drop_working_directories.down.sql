-- Recreate the working_directories table
CREATE TABLE working_directories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, name)
);

-- Recreate index
CREATE INDEX idx_working_directories_channel_id ON working_directories(channel_id);