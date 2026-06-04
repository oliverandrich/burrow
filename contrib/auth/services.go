package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	gowebauthn "github.com/go-webauthn/webauthn/webauthn"
	"github.com/oliverandrich/burrow/internal/bgloop"
	"golang.org/x/crypto/bcrypt"
)

// --- Service interfaces ---

// EmailService defines email operations.
type EmailService interface {
	SendVerification(ctx context.Context, toEmail, verifyURL string) error
	SendInvite(ctx context.Context, toEmail, inviteURL string) error
}

// WebAuthnService defines WebAuthn operations.
type WebAuthnService interface {
	WebAuthn() *gowebauthn.WebAuthn
	StoreRegistrationSession(userID string, data *gowebauthn.SessionData)
	GetRegistrationSession(userID string) (*gowebauthn.SessionData, error)
	StoreDiscoverableSession(sessionID string, data *gowebauthn.SessionData)
	GetDiscoverableSession(sessionID string) (*gowebauthn.SessionData, error)
}

// --- Token utilities ---

const (
	// TokenLength is the number of random bytes for verification tokens.
	TokenLength = 32
	// TokenExpiry is how long verification tokens are valid.
	TokenExpiry = 24 * time.Hour
	// InviteExpiry is how long an invite token is valid.
	InviteExpiry = 7 * 24 * time.Hour
)

// HashToken computes the SHA256 hash of a token.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateToken generates TokenLength random bytes as a hex string with an
// optional prefix, plus its SHA256 hash for storage. The plaintext is the
// only recoverable form; only the hash is ever persisted.
func generateToken(prefix string) (plaintext, hash string, err error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	plaintext = prefix + hex.EncodeToString(b)
	return plaintext, HashToken(plaintext), nil
}

// GenerateToken generates a new verification token.
// Returns (plaintext token, SHA256 hash for storage, expiry time, error).
func GenerateToken() (string, string, time.Time, error) {
	plaintext, hash, err := generateToken("")
	if err != nil {
		return "", "", time.Time{}, err
	}
	return plaintext, hash, time.Now().Add(TokenExpiry), nil
}

// GenerateInviteToken generates a random invite token and its SHA256 hash.
func GenerateInviteToken() (plaintext, hash string, err error) {
	return generateToken("")
}

// APIKeyPrefix is prepended to generated API-key plaintext tokens. The
// fixed prefix makes leaked keys recognisable to secret scanners.
const APIKeyPrefix = "brw_"

// GenerateAPIKey generates a random API-key token and its SHA256 hash. The
// plaintext is prefixed with [APIKeyPrefix] and must be shown to the user
// exactly once — only the returned hash is ever persisted.
func GenerateAPIKey() (plaintext, hash string, err error) {
	return generateToken(APIKeyPrefix)
}

// --- Recovery service ---

const (
	// CodeLength is the length of each recovery code (without dashes).
	CodeLength = 12
	// CodeCount is the default number of recovery codes to generate.
	CodeCount = 8
	// bcryptCost is the cost factor for bcrypt hashing of recovery codes.
	bcryptCost = 12
)

// alphabet for recovery codes (lowercase + digits, excluding confusing chars: 0, o, l, 1).
const recoveryAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// RecoveryService handles recovery code generation.
type RecoveryService struct {
	// BcryptCost overrides the default bcrypt cost. Use bcrypt.MinCost in tests.
	BcryptCost int
}

// NewRecoveryService creates a new recovery service.
func NewRecoveryService() *RecoveryService {
	return &RecoveryService{BcryptCost: bcryptCost}
}

// GenerateCodes generates recovery codes and their bcrypt hashes.
// Returns (plaintext codes for display, hashed codes for storage, error).
func (s *RecoveryService) GenerateCodes(count int) ([]string, []string, error) {
	if count <= 0 {
		count = CodeCount
	}

	plaintexts := make([]string, count)
	hashes := make([]string, count)

	for i := range count {
		code, err := generateRecoveryCode(CodeLength)
		if err != nil {
			return nil, nil, fmt.Errorf("generate code: %w", err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(code), s.BcryptCost)
		if err != nil {
			return nil, nil, fmt.Errorf("hash code: %w", err)
		}

		plaintexts[i] = formatRecoveryCode(code)
		hashes[i] = string(hash)
	}

	return plaintexts, hashes, nil
}

// NormalizeCode removes dashes and converts to lowercase for comparison.
func NormalizeCode(code string) string {
	return strings.ToLower(strings.ReplaceAll(code, "-", ""))
}

func generateRecoveryCode(length int) (string, error) {
	alphabetLen := len(recoveryAlphabet)
	// maxValid is the largest multiple of alphabetLen that fits in a byte.
	// Bytes >= maxValid are rejected to avoid modulo bias.
	maxValid := 256 - (256 % alphabetLen) // 256 - (256 % 30) = 240

	result := make([]byte, length)
	buf := make([]byte, length)
	filled := 0
	for filled < length {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if int(b) >= maxValid {
				continue
			}
			result[filled] = recoveryAlphabet[int(b)%alphabetLen]
			filled++
			if filled == length {
				break
			}
		}
	}
	return string(result), nil
}

func formatRecoveryCode(code string) string {
	var parts []string
	for i := 0; i < len(code); i += 4 {
		end := min(i+4, len(code))
		parts = append(parts, code[i:end])
	}
	return strings.Join(parts, "-")
}

// --- WebAuthn service ---

const webauthnSessionTTL = 2 * time.Minute

// maxWebAuthnSessions caps the in-memory session store to prevent
// denial-of-service via unauthenticated registration/login endpoints.
const maxWebAuthnSessions = 10000

type webauthnService struct {
	wa    *gowebauthn.WebAuthn
	store map[string]*webauthnSessionEntry
	done  chan struct{}
	mu    sync.Mutex
}

type webauthnSessionEntry struct {
	data      *gowebauthn.SessionData
	expiresAt time.Time
}

// NewWebAuthnService creates a new WebAuthn service with the given RP configuration.
// The context controls the lifetime of the background cleanup goroutine.
func NewWebAuthnService(ctx context.Context, rpDisplayName, rpID, rpOrigin string) (WebAuthnService, error) {
	wa, err := gowebauthn.New(&gowebauthn.Config{
		RPDisplayName: rpDisplayName,
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	})
	if err != nil {
		return nil, err
	}
	svc := &webauthnService{
		wa:    wa,
		store: make(map[string]*webauthnSessionEntry),
		done:  make(chan struct{}),
	}
	go svc.cleanup(ctx)
	return svc, nil
}

func (s *webauthnService) WebAuthn() *gowebauthn.WebAuthn { return s.wa }

func (s *webauthnService) StoreRegistrationSession(userID string, data *gowebauthn.SessionData) {
	s.put("registration:"+userID, data)
}

func (s *webauthnService) GetRegistrationSession(userID string) (*gowebauthn.SessionData, error) {
	return s.pop("registration:" + userID)
}

func (s *webauthnService) StoreDiscoverableSession(sessionID string, data *gowebauthn.SessionData) {
	s.put("discoverable:"+sessionID, data)
}

func (s *webauthnService) GetDiscoverableSession(sessionID string) (*gowebauthn.SessionData, error) {
	return s.pop("discoverable:" + sessionID)
}

func (s *webauthnService) put(key string, data *gowebauthn.SessionData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict oldest entries when the store is at capacity.
	if len(s.store) >= maxWebAuthnSessions {
		s.evictOldestLocked()
	}

	s.store[key] = &webauthnSessionEntry{
		data:      data,
		expiresAt: time.Now().Add(webauthnSessionTTL),
	}
}

// evictOldestLocked removes the entry with the earliest expiry.
// Must be called with s.mu held.
func (s *webauthnService) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range s.store {
		if oldestKey == "" || v.expiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.expiresAt
		}
	}
	if oldestKey != "" {
		delete(s.store, oldestKey)
	}
}

func (s *webauthnService) pop(key string) (*gowebauthn.SessionData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.store[key]
	if !ok {
		return nil, errors.New("session not found")
	}
	delete(s.store, key)

	if time.Now().After(entry.expiresAt) {
		return nil, errors.New("session expired")
	}
	return entry.data, nil
}

func (s *webauthnService) cleanup(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			func() {
				defer bgloop.Recover("auth.webauthnSessionCleanup")
				s.sweepExpiredSessions()
			}()
		}
	}
}

// sweepExpiredSessions removes all expired entries. The deferred unlock
// guarantees the mutex is released even if iteration panics.
func (s *webauthnService) sweepExpiredSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.store {
		if now.After(entry.expiresAt) {
			delete(s.store, key)
		}
	}
}
