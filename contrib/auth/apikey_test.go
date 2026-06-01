package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKey(t *testing.T) {
	plaintext, hash, err := GenerateAPIKey()
	require.NoError(t, err)

	assert.Greater(t, len(plaintext), len(APIKeyPrefix), "plaintext should carry the prefix and random body")
	assert.Equal(t, APIKeyPrefix, plaintext[:len(APIKeyPrefix)])
	assert.Equal(t, HashToken(plaintext), hash, "stored hash must be the SHA256 of the plaintext")

	other, _, err := GenerateAPIKey()
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, other, "generated tokens must be unique")
}

func TestAPIKeyIsExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	assert.False(t, (&APIKey{}).IsExpired(), "key without expiry never expires")
	assert.False(t, (&APIKey{ExpiresAt: &future}).IsExpired())
	assert.True(t, (&APIKey{ExpiresAt: &past}).IsExpired())
}

// --- Repository tests ---

func TestCreateAndGetAPIKey(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	plaintext, key, err := repo.CreateAPIKey(ctx, user.ID, "ci-token", nil)
	require.NoError(t, err)
	require.NotEmpty(t, plaintext)
	assert.Equal(t, user.ID, key.UserID)
	assert.Equal(t, "ci-token", key.Label)
	assert.NotEqual(t, plaintext, key.Hash, "the record stores the hash, not the plaintext")

	got, err := repo.GetAPIKeyByHash(ctx, HashToken(plaintext))
	require.NoError(t, err)
	assert.Equal(t, key.ID, got.ID)
}

func TestGetAPIKeyByHashNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)

	_, err := repo.GetAPIKeyByHash(context.Background(), HashToken("brw_nope"))
	require.ErrorIs(t, err, ErrNotFound)
}

func TestListAPIKeysByUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	bob, err := repo.CreateUser(ctx, "bob")
	require.NoError(t, err)

	_, _, err = repo.CreateAPIKey(ctx, alice.ID, "one", nil)
	require.NoError(t, err)
	_, _, err = repo.CreateAPIKey(ctx, alice.ID, "two", nil)
	require.NoError(t, err)
	_, _, err = repo.CreateAPIKey(ctx, bob.ID, "bob-key", nil)
	require.NoError(t, err)

	keys, err := repo.ListAPIKeysByUser(ctx, alice.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 2, "only the owner's keys are listed")
}

func TestDeleteAPIKeyScopedToUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository[EmptyProfile](db)
	ctx := context.Background()

	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	bob, err := repo.CreateUser(ctx, "bob")
	require.NoError(t, err)

	_, key, err := repo.CreateAPIKey(ctx, alice.ID, "alice-key", nil)
	require.NoError(t, err)

	// Bob cannot delete Alice's key by guessing its ID.
	require.NoError(t, repo.DeleteAPIKey(ctx, key.ID, bob.ID))
	keys, err := repo.ListAPIKeysByUser(ctx, alice.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 1, "wrong-owner delete is a no-op")

	// The owner can delete it.
	require.NoError(t, repo.DeleteAPIKey(ctx, key.ID, alice.ID))
	keys, err = repo.ListAPIKeysByUser(ctx, alice.ID)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

// --- Bearer authentication tests ---
//
// Bearer tokens are resolved by the same automatic authMiddleware as session
// cookies, so the standard gates (RequireAuth / RequireStaff) work for API
// clients with no extra middleware. These tests drive authMiddleware + a gate.

// bearerRouter wires the automatic user-loading middleware plus the given
// gates in front of a handler that echoes the authenticated username.
func bearerRouter(app *App[EmptyProfile], gates ...func(http.Handler) http.Handler) chi.Router {
	r := chi.NewRouter()
	r.Use(app.authMiddleware)
	for _, g := range gates {
		r.Use(g)
	}
	r.Get("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		user := CurrentUser[EmptyProfile](r.Context())
		_, _ = w.Write([]byte(user.Username))
	})
	return r
}

// bearerRequest builds an API request (Accept: application/json, empty
// session) optionally carrying a bearer token.
func bearerRequest(ctx context.Context, token string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/ping", nil)
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return session.Inject(req, nil)
}

func TestBearerAuthAllowsValidKey(t *testing.T) {
	app, repo := newTestApp(t)

	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)
	plaintext, _, err := repo.CreateAPIKey(context.Background(), user.ID, "key", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireAuth()).ServeHTTP(rec, bearerRequest(t.Context(), plaintext))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice", rec.Body.String(), "the key's owner is the authenticated user")
}

func TestBearerAuthRejectsMissingCredential(t *testing.T) {
	app, _ := newTestApp(t)

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireAuth()).ServeHTTP(rec, bearerRequest(t.Context(), ""))

	assert.Equal(t, http.StatusUnauthorized, rec.Code, "API client gets 401, not a redirect")
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
	assert.Contains(t, rec.Body.String(), "error")
}

func TestBearerAuthRejectsUnknownToken(t *testing.T) {
	app, _ := newTestApp(t)

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireAuth()).ServeHTTP(rec, bearerRequest(t.Context(), "brw_unknown"))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBearerAuthRejectsExpiredKey(t *testing.T) {
	app, repo := newTestApp(t)

	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)
	past := time.Now().Add(-time.Hour)
	plaintext, _, err := repo.CreateAPIKey(context.Background(), user.ID, "expired", &past)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireAuth()).ServeHTTP(rec, bearerRequest(t.Context(), plaintext))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBearerAuthRejectsInactiveUser(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	plaintext, _, err := repo.CreateAPIKey(ctx, user.ID, "key", nil)
	require.NoError(t, err)
	require.NoError(t, repo.SetUserActive(ctx, user.ID, false))

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireAuth()).ServeHTTP(rec, bearerRequest(t.Context(), plaintext))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBearerAuthComposesWithRequireStaff(t *testing.T) {
	app, repo := newTestApp(t)
	ctx := context.Background()

	// Non-staff user is authenticated by the key but blocked by RequireStaff.
	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	plaintext, _, err := repo.CreateAPIKey(ctx, user.ID, "key", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	bearerRouter(app, RequireStaff()).ServeHTTP(rec, bearerRequest(t.Context(), plaintext))
	assert.Equal(t, http.StatusForbidden, rec.Code, "valid key, insufficient role → 403")

	// Promote to staff and retry.
	require.NoError(t, repo.SetUserRole(ctx, user.ID, RoleStaff))
	rec = httptest.NewRecorder()
	bearerRouter(app, RequireStaff()).ServeHTTP(rec, bearerRequest(t.Context(), plaintext))
	assert.Equal(t, http.StatusOK, rec.Code, "staff key passes the role gate")
}
