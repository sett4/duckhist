CREATE TABLE IF NOT EXISTS history (
    id VARCHAR(26) PRIMARY KEY,
    command TEXT,
    executed_at TIMESTAMP,
    executing_host TEXT,
    executing_dir TEXT,
    executing_user TEXT
);