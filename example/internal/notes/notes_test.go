package notes

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.Migratable      = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasNavItems     = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
)

func TestAppName(t *testing.T) {
	app := New()
	assert.Equal(t, "notes", app.Name())
}

func TestNavItems(t *testing.T) {
	app := New()
	items := app.NavItems()
	require.Len(t, items, 1)
	assert.Equal(t, "Notes", items[0].Label)
	assert.Equal(t, "/notes", items[0].URL)
	assert.True(t, items[0].AuthOnly)
}

func TestMigrationFS(t *testing.T) {
	app := New()
	fsys := app.MigrationFS()
	require.NotNil(t, fsys)
}

// --- Repository tests ---

func openTestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	t.Cleanup(func() { sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())

	// Run notes migration.
	app := New()
	err = burrow.RunAppMigrations(t.Context(), db, app.Name(), app.MigrationFS())
	require.NoError(t, err)

	return db
}

func TestCreateAndListNotes(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	err := repo.Create(ctx, &Note{Title: "First Note", Content: "Hello", UserID: 1})
	require.NoError(t, err)

	err = repo.Create(ctx, &Note{Title: "Second Note", Content: "World", UserID: 1})
	require.NoError(t, err)

	notes, err := repo.ListByUserID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, notes, 2)
	assert.Equal(t, "Second Note", notes[0].Title) // Most recent first.
	assert.Equal(t, "First Note", notes[1].Title)
}

func TestListNotesEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	notes, err := repo.ListByUserID(ctx, 999)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestDeleteNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "To Delete", Content: "Bye", UserID: 1}
	err := repo.Create(ctx, note)
	require.NoError(t, err)

	err = repo.Delete(ctx, note.ID, 1)
	require.NoError(t, err)

	notes, err := repo.ListByUserID(ctx, 1)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestDeleteNoteWrongUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Not Yours", Content: "Nope", UserID: 1}
	err := repo.Create(ctx, note)
	require.NoError(t, err)

	// User 2 can't delete user 1's note.
	err = repo.Delete(ctx, note.ID, 2)
	require.NoError(t, err) // No error but nothing happens.

	notes, err := repo.ListByUserID(ctx, 1)
	require.NoError(t, err)
	assert.Len(t, notes, 1) // Still there.
}

// --- Handler tests ---

func TestListNotesHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Test", Content: "Content", UserID: 42}))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	auth.SetUser(c, &auth.User{ID: 42})

	h := NewHandlers(repo)
	err := h.List(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test")
}

func TestListNotesUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewHandlers(repo)
	err := h.List(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestCreateNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	e := echo.New()
	form := strings.NewReader("title=My+Note&content=Some+content")
	req := httptest.NewRequest(http.MethodPost, "/notes", form)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	auth.SetUser(c, &auth.User{ID: 42})

	h := NewHandlers(repo)
	err := h.Create(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)

	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "My Note", notes[0].Title)
}

func TestCreateNoteEmptyTitle(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	e := echo.New()
	form := strings.NewReader("title=&content=Some+content")
	req := httptest.NewRequest(http.MethodPost, "/notes", form)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	auth.SetUser(c, &auth.User{ID: 42})

	h := NewHandlers(repo)
	err := h.Create(c)

	require.Error(t, err)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestDeleteNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: 42}
	require.NoError(t, repo.Create(ctx, note))

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/notes/1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	auth.SetUser(c, &auth.User{ID: 42})
	c.SetPathValues(echo.PathValues{{Name: "id", Value: "1"}})

	h := NewHandlers(repo)
	err := h.Delete(c)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}
