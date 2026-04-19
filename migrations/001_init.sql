CREATE TABLE IF NOT EXISTS snapshots (
    taken_at         TEXT PRIMARY KEY,   -- ISO8601
    block_started_at TEXT NOT NULL,
    block_ended_at   TEXT,
    tokens_used      INTEGER NOT NULL,
    usage_ratio      REAL    NOT NULL    -- tokens_used / plan_limit
);
