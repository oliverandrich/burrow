package auth

import (
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/oliverandrich/den/document"
)

// Role constants.
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// EmptyProfile is the default Profile type for [User]. Apps that don't
// need user-extension fields parameterise with EmptyProfile; apps that do
// define their own Profile struct (see docs/contrib/auth-profile.md).
type EmptyProfile struct{}

// User represents an authenticated user with WebAuthn credentials. The
// Profile type parameter holds app-specific extension fields (display
// name, bio, avatar, social links, etc.) stored inline as a nested JSON
// object. Apps without extensions use [EmptyProfile].
//
// den: tags on Profile fields (index, unique, fts, index_together) are
// honoured — Den walks named struct fields and emits dotted JSON-path
// indexes (e.g. $.profile.slug). See docs/contrib/auth-profile.md.
type User[P any] struct {
	document.Base
	EmailVerifiedAt *time.Time   `json:"email_verified_at,omitempty" form:"-"`
	Email           *string      `json:"email,omitempty" den:"unique" form:"email" verbose:"Email"`
	Role            string       `json:"role" den:"index" form:"role" verbose:"Role"`
	Username        string       `json:"username" den:"unique" form:"username" verbose:"Username"`
	Profile         P            `json:"profile,omitempty" form:"profile" verbose:"Profile"`
	Credentials     []Credential `json:"credentials,omitempty" form:"-"` // populated by separate query, not embedded
	EmailVerified   bool         `json:"email_verified" form:"-"`
	IsActive        bool         `json:"is_active" form:"is_active" verbose:"Active"`
}

// String returns the username. For a richer display name, read the
// Profile fields directly — see docs/contrib/auth-profile.md.
func (u User[P]) String() string {
	return u.Username
}

// IsAdmin returns true if the user has the admin role.
func (u *User[P]) IsAdmin() bool { return u.Role == RoleAdmin }

// WebAuthnID returns the user ID as bytes for the WebAuthn protocol.
// The ULID string is unique and stable, so we use it directly.
func (u *User[P]) WebAuthnID() []byte {
	return []byte(u.ID)
}

// WebAuthnName returns the username.
func (u *User[P]) WebAuthnName() string { return u.Username }

// WebAuthnDisplayName returns the username. Apps that want a richer
// display name in WebAuthn dialogs should override the relying-party
// configuration; auth-core does not know about app-specific Profile
// fields and the WebAuthn UX impact of switching display names is small.
func (u *User[P]) WebAuthnDisplayName() string { return u.Username }

// WebAuthnCredentials returns the user's WebAuthn credentials.
func (u *User[P]) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.Credentials))
	for i := range u.Credentials {
		creds[i] = u.Credentials[i].ToWebAuthn()
	}
	return creds
}

// WebAuthnIcon returns an empty string (deprecated by the spec).
func (u *User[P]) WebAuthnIcon() string { return "" }

// Credential stores a WebAuthn credential for a user.
type Credential struct {
	document.Base
	CredentialID    []byte `json:"credential_id" den:"unique"`
	PublicKey       []byte `json:"public_key"`
	AAGUID          []byte `json:"aaguid"`
	AttestationType string `json:"attestation_type"`
	Transports      string `json:"transports"`
	Name            string `json:"name"`
	UserID          string `json:"user_id" den:"index"`
	SignCount       uint32 `json:"sign_count"`
	BackupState     bool   `json:"backup_state"`
	BackupEligible  bool   `json:"backup_eligible"`
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
func NewCredentialFromWebAuthn(userID string, cred *webauthn.Credential) *Credential {
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
	document.Base
	UsedAt   *time.Time `json:"used_at,omitempty"`
	CodeHash string     `json:"code_hash"`
	UserID   string     `json:"user_id" den:"index,index_together:recovery_status"`
	Used     bool       `json:"used" den:"index_together:recovery_status"`
}

// EmailVerificationToken stores a hashed token for email verification.
type EmailVerificationToken struct {
	document.Base
	ExpiresAt time.Time `json:"expires_at"`
	TokenHash string    `json:"token_hash" den:"unique"`
	UserID    string    `json:"user_id" den:"index"`
}

// Invite represents an invitation to register.
type Invite struct {
	document.Base
	ExpiresAt time.Time  `json:"expires_at" form:"-" verbose:"Expires At"`
	UsedAt    *time.Time `json:"used_at,omitempty" form:"-"`
	UsedBy    *string    `json:"used_by,omitempty" form:"-"`
	CreatedBy *string    `json:"created_by,omitempty" form:"-"`
	Email     string     `json:"email" verbose:"Email"`
	Label     string     `json:"label" verbose:"Label"`
	TokenHash string     `json:"token_hash" den:"unique" form:"-"`
}

// IsUsed returns true if the invite has been used.
func (i *Invite) IsUsed() bool { return i.UsedAt != nil }

// IsExpired returns true if the invite has expired.
func (i *Invite) IsExpired() bool { return time.Now().After(i.ExpiresAt) }

// IsValid returns true if the invite is neither used nor expired.
func (i *Invite) IsValid() bool { return !i.IsUsed() && !i.IsExpired() }
