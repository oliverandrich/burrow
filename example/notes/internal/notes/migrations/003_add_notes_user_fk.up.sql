-- Add ON DELETE CASCADE foreign key from notes.user_id to users.id.
-- SQLite does not support ALTER TABLE ADD CONSTRAINT, so we recreate the table.

-- Remove orphaned notes (user_id referencing non-existent users).
DELETE FROM notes WHERE user_id NOT IN (SELECT id FROM users);

-- Temporarily drop FTS triggers that reference the notes table.
DROP TRIGGER IF EXISTS notes_ai;
DROP TRIGGER IF EXISTS notes_ad;
DROP TRIGGER IF EXISTS notes_au;

-- Recreate with FK constraint.
CREATE TABLE notes_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

INSERT INTO notes_new (id, user_id, title, content, created_at, deleted_at)
    SELECT id, user_id, title, content, created_at, deleted_at FROM notes;

DROP TABLE notes;
ALTER TABLE notes_new RENAME TO notes;

CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes (user_id);

-- Recreate FTS triggers.
CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
    INSERT INTO notes_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;

-- Rebuild FTS index to reattach to the new table.
INSERT INTO notes_fts(notes_fts) VALUES('rebuild');
