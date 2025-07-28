CREATE TABLE repositories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    path TEXT NOT NULL,
    default_branch TEXT DEFAULT 'main',
    slack_channel_id TEXT,
    slack_username TEXT,
    slack_icon_emoji TEXT,
    slack_icon_url TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for faster lookups by channel
CREATE INDEX idx_repositories_slack_channel_id ON repositories(slack_channel_id);