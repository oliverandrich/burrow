package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when a record is not found.
var ErrNotFound = den.ErrNotFound

// Repository provides data access for auth models.
type Repository struct {
	db *den.DB
}

// NewRepository creates a new auth Repository.
func NewRepository(db *den.DB) *Repository {
	return &Repository{db: db}
}

// --- User methods ---

// CreateUser creates a new user with a username and optional name.
func (r *Repository) CreateUser(ctx context.Context, username, name string) (*User, error) {
	user := &User{Username: username, Name: name, Role: RoleUser, IsActive: true}
	if err := den.Insert(ctx, r.db, user); err != nil {
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
		Role:     RoleUser,
		IsActive: true,
	}
	if err := den.Insert(ctx, r.db, user); err != nil {
		return nil, fmt.Errorf("create user with email %q: %w", email, err)
	}
	return user, nil
}

// GetUserByID retrieves a user by ID.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user, err := den.FindByID[User](ctx, r.db, id)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id %s: %w", id, err)
	}
	return user, nil
}

// GetUserByIDWithCredentials retrieves a user by ID with preloaded credentials.
func (r *Repository) GetUserByIDWithCredentials(ctx context.Context, id string) (*User, error) {
	user, err := r.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	creds, err := r.GetCredentialsByUserID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id %s with credentials: %w", id, err)
	}
	user.Credentials = creds
	return user, nil
}

// GetUserByUsername retrieves a user by username.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user, err := den.NewQuery[User](r.db, where.Field("username").Eq(username)).First(ctx)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by username %q: %w", username, err)
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email.
func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user, err := den.NewQuery[User](r.db, where.Field("email").Eq(email)).First(ctx)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email %q: %w", email, err)
	}
	return user, nil
}

// UpdateUser updates a user record.
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	if err := den.Update(ctx, r.db, user); err != nil {
		return fmt.Errorf("update user %s: %w", user.ID, err)
	}
	return nil
}

// SetUserRole updates a user's role.
func (r *Repository) SetUserRole(ctx context.Context, userID string, role string) error {
	_, err := den.FindOneAndUpdate[User](ctx, r.db,
		den.SetFields{"role": role},
		where.Field("_id").Eq(userID),
	)
	if err != nil {
		return fmt.Errorf("set role for user %s: %w", userID, err)
	}
	return nil
}

// SetUserActive sets a user's is_active flag.
func (r *Repository) SetUserActive(ctx context.Context, userID string, active bool) error {
	_, err := den.FindOneAndUpdate[User](ctx, r.db,
		den.SetFields{"is_active": active},
		where.Field("_id").Eq(userID),
	)
	if err != nil {
		return fmt.Errorf("set active for user %s: %w", userID, err)
	}
	return nil
}

// MarkEmailVerified marks a user's email as verified.
func (r *Repository) MarkEmailVerified(ctx context.Context, userID string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[User](ctx, r.db,
		den.SetFields{"email_verified": true, "email_verified_at": &now},
		where.Field("_id").Eq(userID),
	)
	if err != nil {
		return fmt.Errorf("mark email verified for user %s: %w", userID, err)
	}
	return nil
}

// UserExists checks if a user with the given username exists.
func (r *Repository) UserExists(ctx context.Context, username string) (bool, error) {
	exists, err := den.NewQuery[User](r.db, where.Field("username").Eq(username)).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check user exists %q: %w", username, err)
	}
	return exists, nil
}

// EmailExists checks if a user with the given email exists.
func (r *Repository) EmailExists(ctx context.Context, email string) (bool, error) {
	exists, err := den.NewQuery[User](r.db, where.Field("email").Eq(email)).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check email exists %q: %w", email, err)
	}
	return exists, nil
}

// ListUsers returns all users ordered by creation time ascending.
func (r *Repository) ListUsers(ctx context.Context) ([]User, error) {
	users, err := den.NewQuery[User](r.db).Sort("_created_at", den.Asc).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	result := make([]User, len(users))
	for i, u := range users {
		result[i] = *u
	}
	return result, nil
}

// CountUsers returns the total number of users.
func (r *Repository) CountUsers(ctx context.Context) (int, error) {
	count, err := den.NewQuery[User](r.db).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return int(count), nil
}

// CountAdminUsers returns the number of users with the admin role.
func (r *Repository) CountAdminUsers(ctx context.Context) (int, error) {
	count, err := den.NewQuery[User](r.db, where.Field("role").Eq(RoleAdmin)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count admin users: %w", err)
	}
	return int(count), nil
}

// DeleteUser permanently deletes a user by ID.
func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	user, err := den.FindByID[User](ctx, r.db, id)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil // already deleted
		}
		return fmt.Errorf("delete user %s: %w", id, err)
	}
	if err := den.Delete(ctx, r.db, user); err != nil {
		return fmt.Errorf("delete user %s: %w", id, err)
	}
	return nil
}

// PurgeOrphanedUsers deletes users with zero credentials that were
// created more than the given duration ago. These are leftover from abandoned
// WebAuthn registration flows where the client never called RegisterFinish.
func (r *Repository) PurgeOrphanedUsers(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoffStr := time.Now().Add(-olderThan).Format(time.RFC3339Nano)

	// Find old users only — abandoned registrations are typically very few.
	oldUsers, err := den.NewQuery[User](r.db, where.Field("_created_at").Lt(cutoffStr)).All(ctx)
	if err != nil {
		return 0, fmt.Errorf("purge orphaned users: list old users: %w", err)
	}

	var purged int
	for _, u := range oldUsers {
		hasCredentials, err := den.NewQuery[Credential](r.db, where.Field("user_id").Eq(u.ID)).Exists(ctx)
		if err != nil {
			return purged, fmt.Errorf("purge orphaned users: check credentials for %s: %w", u.ID, err)
		}
		if !hasCredentials {
			if err := den.Delete(ctx, r.db, u); err != nil {
				return purged, fmt.Errorf("purge orphaned users: delete user %s: %w", u.ID, err)
			}
			purged++
		}
	}
	return purged, nil
}

// --- Paginated query methods ---

// ListUsersPaged returns users with pagination and optional role filter, ordered by created_at desc.
func (r *Repository) ListUsersPaged(ctx context.Context, pr burrow.PageRequest, role string) ([]User, burrow.PageResult, error) {
	conditions := []where.Condition{}
	if role != "" {
		conditions = append(conditions, where.Field("role").Eq(role))
	}

	ptrs, count, err := den.NewQuery[User](r.db, conditions...).
		Sort("_created_at", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list users paged: %w", err)
	}

	users := make([]User, len(ptrs))
	for i, p := range ptrs {
		users[i] = *p
	}
	return users, burrow.OffsetResult(pr, int(count)), nil
}

// SearchUsers searches users by username, name, or email with pagination and optional role filter.
func (r *Repository) SearchUsers(ctx context.Context, query string, pr burrow.PageRequest, role string) ([]User, burrow.PageResult, error) {
	searchCond := where.Or(
		where.Field("username").StringContains(query),
		where.Field("name").StringContains(query),
		where.Field("email").StringContains(query),
	)

	var cond where.Condition
	if role != "" {
		cond = where.And(searchCond, where.Field("role").Eq(role))
	} else {
		cond = searchCond
	}

	ptrs, count, err := den.NewQuery[User](r.db, cond).
		Sort("_created_at", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("search users: %w", err)
	}

	users := make([]User, len(ptrs))
	for i, p := range ptrs {
		users[i] = *p
	}
	return users, burrow.OffsetResult(pr, int(count)), nil
}

// GetInviteByID retrieves an invite by its ID.
func (r *Repository) GetInviteByID(ctx context.Context, id string) (*Invite, error) {
	invite, err := den.FindByID[Invite](ctx, r.db, id)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get invite by id %s: %w", id, err)
	}
	return invite, nil
}

// ListInvitesPaged returns invites with pagination, ordered by created_at desc.
func (r *Repository) ListInvitesPaged(ctx context.Context, pr burrow.PageRequest) ([]Invite, burrow.PageResult, error) {
	ptrs, count, err := den.NewQuery[Invite](r.db).
		Sort("_created_at", den.Desc).
		Limit(pr.Limit).
		Skip(pr.Offset()).
		AllWithCount(ctx)
	if err != nil {
		return nil, burrow.PageResult{}, fmt.Errorf("list invites paged: %w", err)
	}

	invites := make([]Invite, len(ptrs))
	for i, p := range ptrs {
		invites[i] = *p
	}
	return invites, burrow.OffsetResult(pr, int(count)), nil
}

// --- Credential methods ---

// CreateCredential creates a new WebAuthn credential.
func (r *Repository) CreateCredential(ctx context.Context, cred *Credential) error {
	if err := den.Insert(ctx, r.db, cred); err != nil {
		return fmt.Errorf("create credential for user %s: %w", cred.UserID, err)
	}
	return nil
}

// GetCredentialsByUserID retrieves all credentials for a user.
func (r *Repository) GetCredentialsByUserID(ctx context.Context, userID string) ([]Credential, error) {
	creds, err := den.NewQuery[Credential](r.db, where.Field("user_id").Eq(userID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get credentials for user %s: %w", userID, err)
	}
	result := make([]Credential, len(creds))
	for i, c := range creds {
		result[i] = *c
	}
	return result, nil
}

// UpdateCredentialSignCount updates the sign count for a credential.
func (r *Repository) UpdateCredentialSignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	// Encode as base64 to match JSON serialization of []byte fields.
	credIDBase64 := base64.StdEncoding.EncodeToString(credentialID)
	_, err := den.FindOneAndUpdate[Credential](ctx, r.db,
		den.SetFields{"sign_count": signCount},
		where.Field("credential_id").Eq(credIDBase64),
	)
	if err != nil {
		return fmt.Errorf("update credential sign count: %w", err)
	}
	return nil
}

// DeleteCredential deletes a credential.
func (r *Repository) DeleteCredential(ctx context.Context, credID, userID string) error {
	cred, err := den.NewQuery[Credential](r.db,
		where.Field("_id").Eq(credID),
		where.Field("user_id").Eq(userID),
	).First(ctx)
	if err != nil {
		return fmt.Errorf("delete credential %s for user %s: %w", credID, userID, err)
	}
	if err := den.Delete(ctx, r.db, cred); err != nil {
		return fmt.Errorf("delete credential %s for user %s: %w", credID, userID, err)
	}
	return nil
}

// CountUserCredentials counts the number of credentials for a user.
func (r *Repository) CountUserCredentials(ctx context.Context, userID string) (int64, error) {
	count, err := den.NewQuery[Credential](r.db, where.Field("user_id").Eq(userID)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count credentials for user %s: %w", userID, err)
	}
	return count, nil
}

// --- Recovery code methods ---

// CreateRecoveryCodes creates recovery codes for a user.
func (r *Repository) CreateRecoveryCodes(ctx context.Context, userID string, codeHashes []string) error {
	codes := make([]*RecoveryCode, len(codeHashes))
	for i, hash := range codeHashes {
		codes[i] = &RecoveryCode{
			UserID:   userID,
			CodeHash: hash,
		}
	}
	if err := den.InsertMany(ctx, r.db, codes); err != nil {
		return fmt.Errorf("create recovery codes for user %s: %w", userID, err)
	}
	return nil
}

// GetUnusedRecoveryCodes retrieves unused recovery codes for a user.
func (r *Repository) GetUnusedRecoveryCodes(ctx context.Context, userID string) ([]RecoveryCode, error) {
	codes, err := den.NewQuery[RecoveryCode](r.db,
		where.Field("user_id").Eq(userID),
		where.Field("used").Eq(false),
	).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("get unused recovery codes for user %s: %w", userID, err)
	}
	result := make([]RecoveryCode, len(codes))
	for i, c := range codes {
		result[i] = *c
	}
	return result, nil
}

// GetUnusedRecoveryCodeCount returns the count of unused recovery codes.
func (r *Repository) GetUnusedRecoveryCodeCount(ctx context.Context, userID string) (int64, error) {
	count, err := den.NewQuery[RecoveryCode](r.db,
		where.Field("user_id").Eq(userID),
		where.Field("used").Eq(false),
	).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count unused recovery codes for user %s: %w", userID, err)
	}
	return count, nil
}

// MarkRecoveryCodeUsed marks a recovery code as used.
func (r *Repository) MarkRecoveryCodeUsed(ctx context.Context, codeID string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[RecoveryCode](ctx, r.db,
		den.SetFields{"used": true, "used_at": &now},
		where.Field("_id").Eq(codeID),
	)
	if err != nil {
		return fmt.Errorf("mark recovery code %s as used: %w", codeID, err)
	}
	return nil
}

// DeleteRecoveryCodes deletes all recovery codes for a user.
func (r *Repository) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	_, err := den.DeleteMany[RecoveryCode](ctx, r.db,
		[]where.Condition{where.Field("user_id").Eq(userID)},
	)
	if err != nil {
		return fmt.Errorf("delete recovery codes for user %s: %w", userID, err)
	}
	return nil
}

// HasRecoveryCodes checks if a user has any recovery codes.
func (r *Repository) HasRecoveryCodes(ctx context.Context, userID string) (bool, error) {
	exists, err := den.NewQuery[RecoveryCode](r.db, where.Field("user_id").Eq(userID)).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check has recovery codes for user %s: %w", userID, err)
	}
	return exists, nil
}

// ValidateAndUseRecoveryCode validates and marks a recovery code as used.
// It always iterates all codes to prevent timing attacks that could reveal
// which code position matched.
func (r *Repository) ValidateAndUseRecoveryCode(ctx context.Context, userID string, code string) (bool, error) {
	codes, err := r.GetUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return false, err
	}

	var matchedID string
	found := false
	for _, c := range codes {
		if bcrypt.CompareHashAndPassword([]byte(c.CodeHash), []byte(code)) == nil {
			matchedID = c.ID
			found = true
			// Continue iterating to prevent timing side-channel.
		}
	}

	if !found {
		return false, nil
	}
	if err := r.MarkRecoveryCodeUsed(ctx, matchedID); err != nil {
		return false, err
	}
	return true, nil
}

// --- Email verification methods ---

// CreateEmailVerificationToken creates a new email verification token.
func (r *Repository) CreateEmailVerificationToken(ctx context.Context, userID string, tokenHash string, expiresAt time.Time) error {
	token := &EmailVerificationToken{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if err := den.Insert(ctx, r.db, token); err != nil {
		return fmt.Errorf("create email verification token for user %s: %w", userID, err)
	}
	return nil
}

// GetEmailVerificationToken retrieves a token by hash.
func (r *Repository) GetEmailVerificationToken(ctx context.Context, tokenHash string) (*EmailVerificationToken, error) {
	token, err := den.NewQuery[EmailVerificationToken](r.db, where.Field("token_hash").Eq(tokenHash)).First(ctx)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get email verification token: %w", err)
	}
	return token, nil
}

// DeleteEmailVerificationToken deletes a token.
func (r *Repository) DeleteEmailVerificationToken(ctx context.Context, tokenID string) error {
	token, err := den.FindByID[EmailVerificationToken](ctx, r.db, tokenID)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("delete email verification token %s: %w", tokenID, err)
	}
	if err := den.Delete(ctx, r.db, token); err != nil {
		return fmt.Errorf("delete email verification token %s: %w", tokenID, err)
	}
	return nil
}

// DeleteUserEmailVerificationTokens deletes all tokens for a user.
func (r *Repository) DeleteUserEmailVerificationTokens(ctx context.Context, userID string) error {
	_, err := den.DeleteMany[EmailVerificationToken](ctx, r.db,
		[]where.Condition{where.Field("user_id").Eq(userID)},
	)
	if err != nil {
		return fmt.Errorf("delete email verification tokens for user %s: %w", userID, err)
	}
	return nil
}

// DeleteExpiredEmailVerificationTokens deletes expired tokens.
func (r *Repository) DeleteExpiredEmailVerificationTokens(ctx context.Context) error {
	_, err := den.DeleteMany[EmailVerificationToken](ctx, r.db,
		[]where.Condition{where.Field("expires_at").Lt(time.Now().Format(time.RFC3339Nano))},
	)
	if err != nil {
		return fmt.Errorf("delete expired email verification tokens: %w", err)
	}
	return nil
}

// --- Invite methods ---

// CreateInvite creates a new invite record.
func (r *Repository) CreateInvite(ctx context.Context, invite *Invite) error {
	if err := den.Insert(ctx, r.db, invite); err != nil {
		return fmt.Errorf("create invite for %q: %w", invite.Email, err)
	}
	return nil
}

// GetInviteByTokenHash retrieves an invite by its token hash.
func (r *Repository) GetInviteByTokenHash(ctx context.Context, tokenHash string) (*Invite, error) {
	invite, err := den.NewQuery[Invite](r.db, where.Field("token_hash").Eq(tokenHash)).First(ctx)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get invite by token hash: %w", err)
	}
	return invite, nil
}

// ListInvites returns all invites ordered by creation date descending.
func (r *Repository) ListInvites(ctx context.Context) ([]Invite, error) {
	invites, err := den.NewQuery[Invite](r.db).Sort("_created_at", den.Desc).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	result := make([]Invite, len(invites))
	for i, inv := range invites {
		result[i] = *inv
	}
	return result, nil
}

// ErrInviteAlreadyUsed is returned when an invite has already been consumed.
var ErrInviteAlreadyUsed = errors.New("invite already used")

// MarkInviteUsed atomically marks an invite as used by the given user.
// The condition used_at IS NULL ensures only the first caller succeeds,
// preventing a race condition where two registrations consume the same invite.
func (r *Repository) MarkInviteUsed(ctx context.Context, inviteID, userID string) error {
	now := time.Now()
	_, err := den.FindOneAndUpdate[Invite](ctx, r.db,
		den.SetFields{"used_at": &now, "used_by": &userID},
		where.Field("_id").Eq(inviteID),
		where.Field("used_at").IsNil(),
	)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return ErrInviteAlreadyUsed
		}
		return fmt.Errorf("mark invite %s as used: %w", inviteID, err)
	}
	return nil
}

// DeleteInvite deletes an invite (revoke).
func (r *Repository) DeleteInvite(ctx context.Context, inviteID string) error {
	invite, err := den.FindByID[Invite](ctx, r.db, inviteID)
	if err != nil {
		if errors.Is(err, den.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("delete invite %s: %w", inviteID, err)
	}
	if err := den.Delete(ctx, r.db, invite); err != nil {
		return fmt.Errorf("delete invite %s: %w", inviteID, err)
	}
	return nil
}
