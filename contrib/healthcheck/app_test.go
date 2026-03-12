package healthcheck

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })
	return bun.NewDB(sqldb, sqlitedialect.New())
}

func setupRouter(t *testing.T, apps ...burrow.App) chi.Router {
	t.Helper()
	db := testDB(t)
	reg := burrow.NewRegistry()
	for _, a := range apps {
		reg.Add(a)
	}

	app := New()
	reg.Add(app)
	require.NoError(t, reg.RegisterAll(db))

	r := chi.NewRouter()
	app.Routes(r)
	return r
}

// Compile-time assertions.
var (
	_ burrow.App       = (*App)(nil)
	_ burrow.HasRoutes = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "healthcheck", app.Name())
}

func TestAppRegister(t *testing.T) {
	app := New()
	db := testDB(t)
	reg := burrow.NewRegistry()
	cfg := &burrow.AppConfig{DB: db, Registry: reg}

	err := app.Register(cfg)
	require.NoError(t, err)
	assert.Equal(t, db, app.db)
	assert.Equal(t, reg, app.registry)
}

func TestLivenessEndpoint(t *testing.T) {
	r := setupRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz/live", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestReadinessEndpointAllHealthy(t *testing.T) {
	r := setupRouter(t)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok","database":"ok","checks":{}}`, rec.Body.String())
}

// mockReadinessApp is a test app that implements ReadinessChecker.
type mockReadinessApp struct {
	err  error
	name string
}

func (m *mockReadinessApp) Name() string                           { return m.name }
func (m *mockReadinessApp) Register(_ *burrow.AppConfig) error     { return nil }
func (m *mockReadinessApp) ReadinessCheck(_ context.Context) error { return m.err }

// Compile-time assertion.
var _ burrow.ReadinessChecker = (*mockReadinessApp)(nil)

func TestReadinessEndpointWithHealthyChecker(t *testing.T) {
	mock := &mockReadinessApp{name: "mockapp", err: nil}
	r := setupRouter(t, mock)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"status":"ok","database":"ok","checks":{"mockapp":"ok"}}`, rec.Body.String())
}

func TestReadinessEndpointWithUnhealthyChecker(t *testing.T) {
	mock := &mockReadinessApp{name: "mockapp", err: errors.New("not connected")}
	r := setupRouter(t, mock)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"status":"unavailable","database":"ok","checks":{"mockapp":"not connected"}}`, rec.Body.String())
}

func TestReadinessEndpointMultipleCheckers(t *testing.T) {
	healthy := &mockReadinessApp{name: "cache", err: nil}
	unhealthy := &mockReadinessApp{name: "queue", err: errors.New("queue down")}
	r := setupRouter(t, healthy, unhealthy)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz/ready", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{"status":"unavailable","database":"ok","checks":{"cache":"ok","queue":"queue down"}}`, rec.Body.String())
}
