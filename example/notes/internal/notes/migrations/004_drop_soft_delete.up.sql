-- Remove soft-delete column from notes table.

ALTER TABLE notes DROP COLUMN deleted_at;
