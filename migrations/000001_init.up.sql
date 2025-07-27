-- Slackスレッドとセッションの関係を管理
CREATE TABLE threads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    channel_id TEXT NOT NULL,
    thread_ts TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(channel_id, thread_ts)
);

-- Claude Code セッション情報
CREATE TABLE sessions (
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

-- インデックス
CREATE INDEX idx_sessions_thread_id ON sessions(thread_id);
CREATE INDEX idx_sessions_status ON sessions(status);