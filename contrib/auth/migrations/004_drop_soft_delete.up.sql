-- Remove soft-delete columns and indexes from all auth tables.

DROP INDEX IF EXISTS idx_users_deleted_at;
DROP INDEX IF EXISTS idx_credentials_deleted_at;
DROP INDEX IF EXISTS idx_recovery_codes_deleted_at;
DROP INDEX IF EXISTS idx_email_verification_tokens_deleted_at;
DROP INDEX IF EXISTS idx_invites_deleted_at;

ALTER TABLE users DROP COLUMN deleted_at;
ALTER TABLE credentials DROP COLUMN deleted_at;
ALTER TABLE recovery_codes DROP COLUMN deleted_at;
ALTER TABLE email_verification_tokens DROP COLUMN deleted_at;
ALTER TABLE invites DROP COLUMN deleted_at;
