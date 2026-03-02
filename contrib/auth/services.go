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
	"golang.org/x/crypto/bcrypt"
)

// --- Service interfaces ---

// EmailService defines email operations.
type EmailService interface {
	GenerateToken() (string, string, time.Time, error)
	SendVerification(ctx context.Context, toEmail, token string) error
	SendInvite(ctx context.Context, toEmail, token string) error
}

// WebAuthnService defines WebAuthn operations.
type WebAuthnService interface {
	WebAuthn() *gowebauthn.WebAuthn
	StoreRegistrationSession(userID int64, data *gowebauthn.SessionData)
	GetRegistrationSession(userID int64) (*gowebauthn.SessionData, error)
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

// GenerateToken generates a new verification token.
// Returns (plaintext token, SHA256 hash for storage, expiry time, error).
func GenerateToken() (string, string, time.Time, error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", "", time.Time{}, fmt.Errorf("generate random bytes: %w", err)
	}
	plaintext := hex.EncodeToString(b)
	return plaintext, HashToken(plaintext), time.Now().Add(TokenExpiry), nil
}

// GenerateInviteToken generates a random invite token and its SHA256 hash.
func GenerateInviteToken() (plaintext, hash string, err error) {
	b := make([]byte, TokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = hex.EncodeToString(b)
	return plaintext, HashToken(plaintext), nil
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
type RecoveryService struct{}

// NewRecoveryService creates a new recovery service.
func NewRecoveryService() *RecoveryService {
	return &RecoveryService{}
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

		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcryptCost)
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
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = recoveryAlphabet[int(b[i])%len(recoveryAlphabet)]
	}
	return string(b), nil
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

type webauthnService struct { //nolint:govet // fieldalignment: readability over optimization
	wa    *gowebauthn.WebAuthn
	mu    sync.Mutex
	store map[string]*webauthnSessionEntry
	done  chan struct{} // closed when cleanup goroutine exits
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

func (s *webauthnService) StoreRegistrationSession(userID int64, data *gowebauthn.SessionData) {
	s.put(fmt.Sprintf("registration:%d", userID), data)
}

func (s *webauthnService) GetRegistrationSession(userID int64) (*gowebauthn.SessionData, error) {
	return s.pop(fmt.Sprintf("registration:%d", userID))
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
	s.store[key] = &webauthnSessionEntry{
		data:      data,
		expiresAt: time.Now().Add(webauthnSessionTTL),
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
			s.mu.Lock()
			now := time.Now()
			for key, entry := range s.store {
				if now.After(entry.expiresAt) {
					delete(s.store, key)
				}
			}
			s.mu.Unlock()
		}
	}
}
