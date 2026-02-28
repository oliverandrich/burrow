package notes

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codeberg.org/oliverandrich/burrow"
	"codeberg.org/oliverandrich/burrow/contrib/auth"
	"github.com/go-chi/chi/v5"
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
	_ burrow.HasAdmin        = (*App)(nil)
	_ burrow.HasTranslations = (*App)(nil)
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

func TestTranslationFS(t *testing.T) {
	app := New()
	fsys := app.TranslationFS()
	require.NotNil(t, fsys)

	matches, err := fs.Glob(fsys, "translations/*.toml")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(matches), 2, "expected at least en and de translation files")
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

// requestWithUser creates a request with the given user set in the context.
func requestWithUser(req *http.Request, user *auth.User) *http.Request {
	if user != nil {
		ctx := auth.WithUser(req.Context(), user)
		return req.WithContext(ctx)
	}
	return req
}

func TestListNotesHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Test", Content: "Content", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test")
}

func TestListNotesUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestCreateNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	form := strings.NewReader("title=My+Note&content=Some+content")
	req := httptest.NewRequest(http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()

	err := h.Create(rec, req)

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

	h := NewHandlers(repo)
	form := strings.NewReader("title=&content=Some+content")
	req := httptest.NewRequest(http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()

	err := h.Create(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestDeleteNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: 42}
	require.NoError(t, repo.Create(ctx, note))

	h := NewHandlers(repo)

	// Use chi router to inject URL params.
	r := chi.NewRouter()
	r.Delete("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Delete(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	req := httptest.NewRequest(http.MethodDelete, "/notes/1", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Repository admin tests ---

func TestListAll(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create notes for different users.
	require.NoError(t, repo.Create(ctx, &Note{Title: "User1 Note", Content: "A", UserID: 1}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "User2 Note", Content: "B", UserID: 2}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "User1 Second", Content: "C", UserID: 1}))

	notes, err := repo.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, notes, 3)
	// Most recent first.
	assert.Equal(t, "User1 Second", notes[0].Title)
	assert.Equal(t, "User2 Note", notes[1].Title)
	assert.Equal(t, "User1 Note", notes[2].Title)
}

func TestListAllEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	notes, err := repo.ListAll(t.Context())
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestAdminDelete(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Admin Delete", Content: "Gone", UserID: 1}
	require.NoError(t, repo.Create(ctx, note))

	// Admin deletes without user ownership check.
	err := repo.AdminDelete(ctx, note.ID)
	require.NoError(t, err)

	notes, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

// --- Admin handler tests ---

func TestAdminListHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Admin View", Content: "Visible", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/notes", nil)
	rec := httptest.NewRecorder()

	err := h.AdminList(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Admin View")
}

func TestAdminDeleteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: 42}
	require.NoError(t, repo.Create(ctx, note))

	h := NewHandlers(repo)

	r := chi.NewRouter()
	r.Delete("/admin/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.AdminDelete(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	req := httptest.NewRequest(http.MethodDelete, "/admin/notes/1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"deleted"`)

	// Verify it's actually deleted.
	notes, err := repo.ListAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestAdminNavItems(t *testing.T) {
	app := New()
	items := app.AdminNavItems()

	require.Len(t, items, 1)
	assert.Equal(t, "Notes", items[0].Label)
	assert.Equal(t, "admin-nav-notes", items[0].LabelKey)
	assert.Equal(t, "/admin/notes", items[0].URL)
	assert.True(t, items[0].AdminOnly)
	assert.Equal(t, "bi bi-journal-text", items[0].Icon)
	assert.Equal(t, 30, items[0].Position)
}
