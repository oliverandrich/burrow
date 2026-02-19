package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = sql.ErrNoRows

// Repository provides data access for auth models.
type Repository struct {
	db *bun.DB
}

// NewRepository creates a new auth Repository.
func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

// --- User methods ---

// CreateUser creates a new user with a username and optional name.
func (r *Repository) CreateUser(ctx context.Context, username, name string) (*User, error) {
	user := &User{Username: username, Name: name}
	if _, err := r.db.NewInsert().Model(user).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create user %q: %w", username, err)
	}
	return user, nil
}

// CreateUserWithEmail creates a new user with email and optional name.
func (r *Repository) CreateUserWithEmail(ctx context.Context, email, name string) (*User, error) {
	user := &User{
		Username: email,
		Email:    &email,
		Name:     name,
	}
	if _, err := r.db.NewInsert().Model(user).Exec(ctx); err != nil {
		return nil, fmt.Errorf("create user with email %q: %w", email, err)
	}
	return user, nil
}

// GetUserByID retrieves a user by ID.
func (r *Repository) GetUserByID(ctx context.Context, id int64) (*User, error) {
	var user User
	if err := r.db.NewSelect().Model(&user).Where("u.id = ?", id).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}
	return &user, nil
}

// GetUserByIDWithCredentials retrieves a user by ID with preloaded credentials.
func (r *Repository) GetUserByIDWithCredentials(ctx context.Context, id int64) (*User, error) {
	var user User
	if err := r.db.NewSelect().Model(&user).Relation("Credentials").Where("u.id = ?", id).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get user by id %d with credentials: %w", id, err)
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	if err := r.db.NewSelect().Model(&user).Where("username = ?", username).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get user by username %q: %w", username, err)
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	if err := r.db.NewSelect().Model(&user).Where("email = ?", email).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get user by email %q: %w", email, err)
	}
	return &user, nil
}

// UpdateUser updates a user record.
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	if _, err := r.db.NewUpdate().Model(user).WherePK().Exec(ctx); err != nil {
		return fmt.Errorf("update user %d: %w", user.ID, err)
	}
	return nil
}

// SetUserRole updates a user's role.
func (r *Repository) SetUserRole(ctx context.Context, userID int64, role string) error {
	if _, err := r.db.NewUpdate().Model((*User)(nil)).
		Set("role = ?", role).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", userID).
		Exec(ctx); err != nil {
		return fmt.Errorf("set role for user %d: %w", userID, err)
	}
	return nil
}

// MarkEmailVerified marks a user's email as verified.
func (r *Repository) MarkEmailVerified(ctx context.Context, userID int64) error {
	now := time.Now()
	if _, err := r.db.NewUpdate().Model((*User)(nil)).
		Set("email_verified = ?", true).
		Set("email_verified_at = ?", now).
		Where("id = ?", userID).
		Exec(ctx); err != nil {
		return fmt.Errorf("mark email verified for user %d: %w", userID, err)
	}
	return nil
}

// UserExists checks if a user with the given username exists.
func (r *Repository) UserExists(ctx context.Context, username string) (bool, error) {
	count, err := r.db.NewSelect().Model((*User)(nil)).Where("username = ?", username).Count(ctx)
	if err != nil {
		return false, fmt.Errorf("check user exists %q: %w", username, err)
	}
	return count > 0, nil
}

// EmailExists checks if a user with the given email exists.
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	count, err := r.db.NewSelect().Model((*User)(nil)).Where("email = ?", email).Count(ctx)
	if err != nil {
		return false, fmt.Errorf("check email exists %q: %w", email, err)
	}
	return count > 0, nil
}

// ListUsers returns all non-deleted users ordered by creation date descending.
func (r *Repository) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := r.db.NewSelect().Model(&users).Order("created_at DESC", "id DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// CountUsers returns the total number of non-deleted users.
func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	count, err := r.db.NewSelect().Model((*User)(nil)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

// --- Credential methods ---

// CreateCredential creates a new WebAuthn credential.
func (r *Repository) CreateCredential(ctx context.Context, cred *Credential) error {
	if _, err := r.db.NewInsert().Model(cred).Exec(ctx); err != nil {
		return fmt.Errorf("create credential for user %d: %w", cred.UserID, err)
	}
	return nil
}

// GetCredentialsByUserID retrieves all credentials for a user.
func (r *Repository) GetCredentialsByUserID(ctx context.Context, userID int64) ([]Credential, error) {
	var creds []Credential
	if err := r.db.NewSelect().Model(&creds).Where("user_id = ?", userID).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get credentials for user %d: %w", userID, err)
	}
	return creds, nil
}

// UpdateCredentialSignCount updates the sign count for a credential.
func (r *Repository) UpdateCredentialSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	if _, err := r.db.NewUpdate().Model((*Credential)(nil)).
		Set("sign_count = ?", signCount).
		Where("credential_id = ?", credentialID).
		Exec(ctx); err != nil {
		return fmt.Errorf("update credential sign count: %w", err)
	}
	return nil
}

// DeleteCredential soft-deletes a credential.
func (r *Repository) DeleteCredential(ctx context.Context, credID, userID int64) error {
	if _, err := r.db.NewDelete().Model((*Credential)(nil)).
		Where("id = ? AND user_id = ?", credID, userID).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete credential %d for user %d: %w", credID, userID, err)
	}
	return nil
}

// CountUserCredentials counts the number of credentials for a user.
func (r *Repository) CountUserCredentials(ctx context.Context, userID int64) (int64, error) {
	count, err := r.db.NewSelect().Model((*Credential)(nil)).Where("user_id = ?", userID).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count credentials for user %d: %w", userID, err)
	}
	return int64(count), nil
}

// --- Recovery code methods ---

// CreateRecoveryCodes creates recovery codes for a user.
func (r *Repository) CreateRecoveryCodes(ctx context.Context, userID int64, codeHashes []string) error {
	codes := make([]RecoveryCode, len(codeHashes))
	for i, hash := range codeHashes {
		codes[i] = RecoveryCode{
			UserID:   userID,
			CodeHash: hash,
		}
	}
	if _, err := r.db.NewInsert().Model(&codes).Exec(ctx); err != nil {
		return fmt.Errorf("create recovery codes for user %d: %w", userID, err)
	}
	return nil
}

// GetUnusedRecoveryCodes retrieves unused recovery codes for a user.
func (r *Repository) GetUnusedRecoveryCodes(ctx context.Context, userID int64) ([]RecoveryCode, error) {
	var codes []RecoveryCode
	if err := r.db.NewSelect().Model(&codes).Where("user_id = ? AND used = ?", userID, false).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get unused recovery codes for user %d: %w", userID, err)
	}
	return codes, nil
}

// GetUnusedRecoveryCodeCount returns the count of unused recovery codes.
func (r *Repository) GetUnusedRecoveryCodeCount(ctx context.Context, userID int64) (int64, error) {
	count, err := r.db.NewSelect().Model((*RecoveryCode)(nil)).Where("user_id = ? AND used = ?", userID, false).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unused recovery codes for user %d: %w", userID, err)
	}
	return int64(count), nil
}

// MarkRecoveryCodeUsed marks a recovery code as used.
func (r *Repository) MarkRecoveryCodeUsed(ctx context.Context, codeID int64) error {
	now := time.Now()
	if _, err := r.db.NewUpdate().Model((*RecoveryCode)(nil)).
		Set("used = ?", true).
		Set("used_at = ?", now).
		Where("id = ?", codeID).
		Exec(ctx); err != nil {
		return fmt.Errorf("mark recovery code %d as used: %w", codeID, err)
	}
	return nil
}

// DeleteRecoveryCodes hard-deletes all recovery codes for a user.
func (r *Repository) DeleteRecoveryCodes(ctx context.Context, userID int64) error {
	if _, err := r.db.NewDelete().Model((*RecoveryCode)(nil)).
		Where("user_id = ?", userID).
		ForceDelete().
		Exec(ctx); err != nil {
		return fmt.Errorf("delete recovery codes for user %d: %w", userID, err)
	}
	return nil
}

// HasRecoveryCodes checks if a user has any recovery codes.
func (r *Repository) HasRecoveryCodes(ctx context.Context, userID int64) (bool, error) {
	count, err := r.db.NewSelect().Model((*RecoveryCode)(nil)).Where("user_id = ?", userID).Count(ctx)
	if err != nil {
		return false, fmt.Errorf("check has recovery codes for user %d: %w", userID, err)
	}
	return count > 0, nil
}

// ValidateAndUseRecoveryCode validates and marks a recovery code as used.
func (r *Repository) ValidateAndUseRecoveryCode(ctx context.Context, userID int64, code string) (bool, error) {
	codes, err := r.GetUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, c := range codes {
		if bcrypt.CompareHashAndPassword([]byte(c.CodeHash), []byte(code)) == nil {
			if markErr := r.MarkRecoveryCodeUsed(ctx, c.ID); markErr != nil {
				return false, markErr
			}
			return true, nil
		}
	}
	return false, nil
}

// --- Email verification methods ---

// CreateEmailVerificationToken creates a new email verification token.
func (r *Repository) CreateEmailVerificationToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	token := &EmailVerificationToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if _, err := r.db.NewInsert().Model(token).Exec(ctx); err != nil {
		return fmt.Errorf("create email verification token for user %d: %w", userID, err)
	}
	return nil
}

// GetEmailVerificationToken retrieves a token by hash.
func (r *Repository) GetEmailVerificationToken(ctx context.Context, tokenHash string) (*EmailVerificationToken, error) {
	var token EmailVerificationToken
	if err := r.db.NewSelect().Model(&token).Where("token_hash = ?", tokenHash).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get email verification token: %w", err)
	}
	return &token, nil
}

// DeleteEmailVerificationToken hard-deletes a token.
func (r *Repository) DeleteEmailVerificationToken(ctx context.Context, tokenID int64) error {
	if _, err := r.db.NewDelete().Model((*EmailVerificationToken)(nil)).
		Where("id = ?", tokenID).
		ForceDelete().
		Exec(ctx); err != nil {
		return fmt.Errorf("delete email verification token %d: %w", tokenID, err)
	}
	return nil
}

// DeleteUserEmailVerificationTokens hard-deletes all tokens for a user.
func (r *Repository) DeleteUserEmailVerificationTokens(ctx context.Context, userID int64) error {
	if _, err := r.db.NewDelete().Model((*EmailVerificationToken)(nil)).
		Where("user_id = ?", userID).
		ForceDelete().
		Exec(ctx); err != nil {
		return fmt.Errorf("delete email verification tokens for user %d: %w", userID, err)
	}
	return nil
}

// DeleteExpiredEmailVerificationTokens hard-deletes expired tokens.
func (r *Repository) DeleteExpiredEmailVerificationTokens(ctx context.Context) error {
	if _, err := r.db.NewDelete().Model((*EmailVerificationToken)(nil)).
		Where("expires_at < ?", time.Now()).
		ForceDelete().
		Exec(ctx); err != nil {
		return fmt.Errorf("delete expired email verification tokens: %w", err)
	}
	return nil
}

// --- Invite methods ---

// CreateInvite creates a new invite record.
func (r *Repository) CreateInvite(ctx context.Context, invite *Invite) error {
	if _, err := r.db.NewInsert().Model(invite).Exec(ctx); err != nil {
		return fmt.Errorf("create invite for %q: %w", invite.Email, err)
	}
	return nil
}

// GetInviteByTokenHash retrieves an invite by its token hash.
func (r *Repository) GetInviteByTokenHash(ctx context.Context, tokenHash string) (*Invite, error) {
	var invite Invite
	if err := r.db.NewSelect().Model(&invite).Where("token_hash = ?", tokenHash).Scan(ctx); err != nil {
		return nil, fmt.Errorf("get invite by token hash: %w", err)
	}
	return &invite, nil
}

// ListInvites returns all invites ordered by creation date descending.
func (r *Repository) ListInvites(ctx context.Context) ([]Invite, error) {
	var invites []Invite
	if err := r.db.NewSelect().Model(&invites).Order("created_at DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	return invites, nil
}

// MarkInviteUsed marks an invite as used by the given user.
func (r *Repository) MarkInviteUsed(ctx context.Context, inviteID, userID int64) error {
	now := time.Now()
	if _, err := r.db.NewUpdate().Model((*Invite)(nil)).
		Set("used_at = ?", now).
		Set("used_by = ?", userID).
		Where("id = ?", inviteID).
		Exec(ctx); err != nil {
		return fmt.Errorf("mark invite %d as used: %w", inviteID, err)
	}
	return nil
}

// DeleteInvite hard-deletes an invite (revoke).
func (r *Repository) DeleteInvite(ctx context.Context, inviteID int64) error {
	if _, err := r.db.NewDelete().Model((*Invite)(nil)).
		Where("id = ?", inviteID).
		ForceDelete().
		Exec(ctx); err != nil {
		return fmt.Errorf("delete invite %d: %w", inviteID, err)
	}
	return nil
}
