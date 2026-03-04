-- Initial schema: jobs queue table

CREATE TABLE IF NOT EXISTS _jobs (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    type         TEXT NOT NULL,
    payload      TEXT NOT NULL DEFAULT '{}',
    status       TEXT NOT NULL DEFAULT 'pending',
    attempts     INTEGER NOT NULL DEFAULT 0,
    max_retries  INTEGER NOT NULL DEFAULT 3,
    run_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    locked_at    DATETIME,
    completed_at DATETIME,
    failed_at    DATETIME,
    last_error   TEXT,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME
);

CREATE INDEX IF NOT EXISTS idx_jobs_status_run_at ON _jobs(status, run_at);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON _jobs(type);
