package auth

import (
	"encoding/binary"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/uptrace/bun"
)

// Role constants.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User represents an authenticated user with WebAuthn credentials.
type User struct {
	bun.BaseModel   `bun:"table:users,alias:u"`
	DeletedAt       time.Time    `bun:",soft_delete,nullzero" json:"-" form:"-"`
	UpdatedAt       time.Time    `bun:",nullzero" json:"updated_at" form:"-"`
	CreatedAt       time.Time    `bun:",nullzero,notnull,default:current_timestamp" json:"created_at" form:"-" verbose:"Created at"`
	EmailVerifiedAt *time.Time   `json:"email_verified_at,omitempty" form:"-"`
	Email           *string      `bun:",unique" json:"email,omitempty" form:"-" verbose:"Email"`
	Name            string       `bun:",nullzero" json:"name,omitempty" verbose:"Name"`
	Bio             string       `bun:",nullzero" json:"bio,omitempty"`
	Role            string       `bun:",notnull,default:'user'" json:"role" verbose:"Role"`
	Username        string       `bun:",unique,notnull" json:"username" form:"-" verbose:"Username"`
	Credentials     []Credential `bun:"rel:has-many,join:id=user_id" json:"credentials,omitempty" form:"-"`
	ID              int64        `bun:",pk,autoincrement" json:"id" verbose:"ID"`
	EmailVerified   bool         `bun:",notnull,default:false" json:"email_verified" form:"-"`
	IsActive        bool         `bun:",notnull,default:true" json:"is_active" form:"-" verbose:"Active"`
}

// IsAdmin returns true if the user has the admin role.
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// WebAuthnID returns the user ID as bytes for the WebAuthn protocol.
func (u *User) WebAuthnID() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(u.ID)) //nolint:gosec // ID is always positive
	return b
}

// WebAuthnName returns the username.
func (u *User) WebAuthnName() string { return u.Username }

// WebAuthnDisplayName returns the display name or falls back to username.
func (u *User) WebAuthnDisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	return u.Username
}

// WebAuthnCredentials returns the user's WebAuthn credentials.
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.Credentials))
	for i := range u.Credentials {
		creds[i] = u.Credentials[i].ToWebAuthn()
	}
	return creds
}

// WebAuthnIcon returns an empty string (deprecated by the spec).
func (u *User) WebAuthnIcon() string { return "" }

// Credential stores a WebAuthn credential for a user.
type Credential struct {
	bun.BaseModel   `bun:"table:credentials,alias:c"`
	DeletedAt       time.Time `bun:",soft_delete,nullzero" json:"-"`
	CreatedAt       time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	AttestationType string    `json:"-"`
	Transports      string    `json:"-"`
	Name            string    `bun:",notnull" json:"name"`
	CredentialID    []byte    `bun:",unique,notnull" json:"-"`
	PublicKey       []byte    `bun:",notnull" json:"-"`
	AAGUID          []byte    `json:"-"`
	ID              int64     `bun:",pk,autoincrement" json:"id"`
	UserID          int64     `bun:",notnull" json:"user_id"`
	SignCount       uint32    `bun:",default:0" json:"-"`
	BackupState     bool      `bun:",default:false" json:"-"`
	BackupEligible  bool      `bun:",default:false" json:"-"`
}

// ToWebAuthn converts the stored credential to the WebAuthn library type.
func (c *Credential) ToWebAuthn() webauthn.Credential {
	var transports []protocol.AuthenticatorTransport
	if c.Transports != "" {
		for t := range strings.SplitSeq(c.Transports, ",") {
			transports = append(transports, protocol.AuthenticatorTransport(t))
		}
	}
	return webauthn.Credential{
		ID:              c.CredentialID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Transport:       transports,
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			BackupEligible: c.BackupEligible,
			BackupState:    c.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    c.AAGUID,
			SignCount: c.SignCount,
		},
	}
}

// NewCredentialFromWebAuthn creates a Credential from a WebAuthn registration result.
func NewCredentialFromWebAuthn(userID int64, cred *webauthn.Credential) *Credential {
	return &Credential{
		UserID:          userID,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		Transports:      TransportsFromWebAuthn(cred.Transport),
		Name:            "Passkey",
		BackupEligible:  cred.Flags.BackupEligible,
		BackupState:     cred.Flags.BackupState,
		AttestationType: cred.AttestationType,
	}
}

// TransportsFromWebAuthn converts WebAuthn transports to a comma-separated string.
func TransportsFromWebAuthn(transports []protocol.AuthenticatorTransport) string {
	strs := make([]string, len(transports))
	for i, t := range transports {
		strs[i] = string(t)
	}
	return strings.Join(strs, ",")
}

// RecoveryCode stores a hashed recovery code for account recovery.
type RecoveryCode struct {
	bun.BaseModel `bun:"table:recovery_codes,alias:rc"`
	CreatedAt     time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	DeletedAt     time.Time  `bun:",soft_delete,nullzero" json:"-"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	CodeHash      string     `bun:",notnull" json:"-"`
	ID            int64      `bun:",pk,autoincrement" json:"id"`
	UserID        int64      `bun:",notnull" json:"user_id"`
	Used          bool       `bun:",notnull,default:false" json:"used"`
}

// EmailVerificationToken stores a hashed token for email verification.
type EmailVerificationToken struct {
	bun.BaseModel `bun:"table:email_verification_tokens,alias:evt"`
	ExpiresAt     time.Time `bun:",notnull" json:"expires_at"`
	CreatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	DeletedAt     time.Time `bun:",soft_delete,nullzero" json:"-"`
	TokenHash     string    `bun:",unique,notnull" json:"-"`
	ID            int64     `bun:",pk,autoincrement" json:"id"`
	UserID        int64     `bun:",notnull" json:"user_id"`
}

// Invite represents an invitation to register.
type Invite struct {
	bun.BaseModel `bun:"table:invites,alias:inv"`
	ExpiresAt     time.Time  `bun:",notnull" json:"expires_at" form:"-" verbose:"Expires at"`
	CreatedAt     time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at" form:"-" verbose:"Created at"`
	DeletedAt     time.Time  `bun:",soft_delete,nullzero" json:"-" form:"-"`
	UsedAt        *time.Time `json:"used_at,omitempty" form:"-"`
	UsedBy        *int64     `json:"used_by,omitempty" form:"-"`
	CreatedBy     *int64     `json:"created_by,omitempty" form:"-"`
	Email         string     `bun:",notnull" json:"email" verbose:"Email"`
	Label         string     `bun:",notnull,default:''" json:"label" verbose:"Label"`
	TokenHash     string     `bun:",unique,notnull" json:"-" form:"-"`
	ID            int64      `bun:",pk,autoincrement" json:"id" verbose:"ID"`
}

// IsUsed returns true if the invite has been used.
func (i *Invite) IsUsed() bool { return i.UsedAt != nil }

// IsExpired returns true if the invite has expired.
func (i *Invite) IsExpired() bool { return time.Now().After(i.ExpiresAt) }

// IsValid returns true if the invite is neither used nor expired.
func (i *Invite) IsValid() bool { return !i.IsUsed() && !i.IsExpired() }
