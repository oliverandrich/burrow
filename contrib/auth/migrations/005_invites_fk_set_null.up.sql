-- Add ON DELETE SET NULL to invites foreign keys (used_by, created_by).
-- SQLite does not support ALTER TABLE ... ADD CONSTRAINT, so we recreate the table.

CREATE TABLE invites_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    token_hash TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    used_by INTEGER,
    created_by INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (used_by) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO invites_new (id, email, label, token_hash, expires_at, used_at, used_by, created_by, created_at)
    SELECT id, email, label, token_hash, expires_at, used_at, used_by, created_by, created_at FROM invites;

DROP TABLE invites;
ALTER TABLE invites_new RENAME TO invites;

CREATE INDEX IF NOT EXISTS idx_invites_token_hash ON invites(token_hash);
CREATE INDEX IF NOT EXISTS idx_invites_email ON invites(email);
