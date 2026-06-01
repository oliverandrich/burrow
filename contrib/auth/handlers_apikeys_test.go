package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeysPageListsKeys(t *testing.T) {
	h, repo := setupTestApp(t)
	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)
	_, _, err = repo.CreateAPIKey(context.Background(), user.ID, "ci-deploy", nil)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/api-keys", nil)
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	require.NoError(t, h.APIKeysPage(rec, req))
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "api-keys-title")
	assert.Contains(t, body, "ci-deploy", "the key's label is listed")
}

func TestAPIKeysPageShowsNewKeyOnce(t *testing.T) {
	h, repo := setupTestApp(t)
	user, err := repo.CreateUser(context.Background(), "alice")
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/api-keys", nil)
	req = session.Inject(req, map[string]any{sessionKeyNewAPIKey: "brw_secret-token"})
	ctx := burrow.WithTemplateExecutor(req.Context(), rendererTestExecutor())
	req = req.WithContext(WithUser(ctx, user))
	rec := httptest.NewRecorder()

	require.NoError(t, h.APIKeysPage(rec, req))
	assert.Contains(t, rec.Body.String(), "brw_secret-token", "the freshly-created key is shown once")
	assert.Empty(t, session.GetString(req, sessionKeyNewAPIKey), "and cleared from the session afterwards")
}

func TestCreateAPIKeyHandler(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)

	form := url.Values{"label": {"deploy"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/api-keys", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithSession(req, user)
	rec := httptest.NewRecorder()

	require.NoError(t, h.CreateAPIKeyHandler(rec, req))
	assert.Equal(t, http.StatusSeeOther, rec.Code, "redirects back to the list")

	keys, err := repo.ListAPIKeysByUser(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "deploy", keys[0].Label)
	assert.NotEmpty(t, session.GetString(req, sessionKeyNewAPIKey), "plaintext stashed for one-time display")
}

func TestRevokeAPIKeyHandler(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	_, key, err := repo.CreateAPIKey(ctx, user.ID, "doomed", nil)
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/auth/api-keys/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		_ = h.RevokeAPIKeyHandler(w, requestWithSession(r, user))
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/api-keys/"+key.ID+"/delete", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	keys, err := repo.ListAPIKeysByUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, keys, "the key is revoked")
}

func TestRevokeAPIKeyHandlerScopedToOwner(t *testing.T) {
	h, repo := setupTestApp(t)
	ctx := context.Background()
	alice, err := repo.CreateUser(ctx, "alice")
	require.NoError(t, err)
	bob, err := repo.CreateUser(ctx, "bob")
	require.NoError(t, err)
	_, key, err := repo.CreateAPIKey(ctx, alice.ID, "alice-key", nil)
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Post("/auth/api-keys/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
		_ = h.RevokeAPIKeyHandler(w, requestWithSession(r, bob)) // authenticated as bob
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/auth/api-keys/"+key.ID+"/delete", nil)
	router.ServeHTTP(httptest.NewRecorder(), req)

	keys, err := repo.ListAPIKeysByUser(ctx, alice.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 1, "bob cannot revoke alice's key")
}
