package healthcheck

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
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

// Compile-time assertions.
var (
	_ burrow.App       = (*App)(nil)
	_ burrow.HasRoutes = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := &App{}
	assert.Equal(t, "healthcheck", app.Name())
}

func TestAppRegister(t *testing.T) {
	app := &App{}
	db := testDB(t)
	cfg := &burrow.AppConfig{DB: db}

	err := app.Register(cfg)
	require.NoError(t, err)
	assert.Equal(t, db, app.db)
}

func TestHealthEndpoint(t *testing.T) {
	app := &App{}
	db := testDB(t)
	app.db = db

	r := chi.NewRouter()
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)
}

func TestHealthEndpointDBCheck(t *testing.T) {
	app := &App{}
	db := testDB(t)
	app.db = db

	r := chi.NewRouter()
	app.Routes(r)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"database":"ok"`)
}
