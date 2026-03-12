package notes

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
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
	_ burrow.HasTemplates    = (*App)(nil)
	_ burrow.HasFuncMap      = (*App)(nil)
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

func TestGetByID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Find Me", Content: "Here", UserID: 1}
	require.NoError(t, repo.Create(ctx, note))

	found, err := repo.GetByID(ctx, note.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "Find Me", found.Title)
	assert.Equal(t, "Here", found.Content)
}

func TestGetByIDWrongUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Not Yours", Content: "Nope", UserID: 1}
	require.NoError(t, repo.Create(ctx, note))

	_, err := repo.GetByID(ctx, note.ID, 2)
	require.Error(t, err)
}

func TestUpdateNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Original", Content: "Old", UserID: 1}
	require.NoError(t, repo.Create(ctx, note))

	note.Title = "Updated"
	note.Content = "New"
	require.NoError(t, repo.Update(ctx, note))

	found, err := repo.GetByID(ctx, note.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
	assert.Equal(t, "New", found.Content)
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

// testTemplateExecutor builds a real template executor from the notes templates
// plus a minimal app/alerts_oob stub for handler tests.
func testTemplateExecutor(t *testing.T) burrow.TemplateExecutor {
	t.Helper()

	app := New()
	fm := app.FuncMap()
	// Add stubs for request-scoped functions and functions provided by other apps.
	fm["t"] = func(key string) string { return key }
	fm["csrfToken"] = func() string { return "test-token" }
	fm["staticURL"] = func(name string) string { return "/static/" + name }
	fm["itoa"] = func(id int64) string { return fmt.Sprintf("%d", id) }
	fm["iconTrash"] = func(class ...string) template.HTML { return "<svg>trash</svg>" }
	fm["alertClass"] = func(level messages.Level) string { return string(level) }

	tmpl := template.New("").Funcs(fm)

	// Parse notes templates.
	fsys := app.TemplateFS()
	entries, err := fs.ReadDir(fsys, "notes")
	require.NoError(t, err)
	for _, e := range entries {
		data, readErr := fs.ReadFile(fsys, "notes/"+e.Name())
		require.NoError(t, readErr)
		_, parseErr := tmpl.Parse(string(data))
		require.NoError(t, parseErr)
	}

	// Add a minimal app/alerts_oob template for create/delete/update responses.
	_, err = tmpl.Parse(`{{ define "app/alerts_oob" -}}
<div id="alerts" hx-swap-oob="true">
{{ range .Messages -}}
<div class="alert alert-{{ .Level }}">{{ .Text }}</div>
{{- end }}
</div>
{{- end }}`)
	require.NoError(t, err)

	return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
		var buf strings.Builder
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // test helper
	}
}

// injectTemplateExecutor adds a test template executor to the request context.
func injectTemplateExecutor(t *testing.T, req *http.Request) *http.Request {
	t.Helper()
	exec := testTemplateExecutor(t)
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	return req.WithContext(ctx)
}

func TestListNotesHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Test", Content: "Content", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Test")
	assert.Contains(t, body, `id="note-form"`)
	assert.Contains(t, body, `hx-get="/notes/new"`)
}

func TestListNotesHTMXNavReturnsFragment(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Test", Content: "Content", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	// HTMX nav request (no cursor) → should use RenderTemplate → fragment only.
	req.Header.Set("HX-Request", "true")

	layoutCalled := false
	layout := burrow.LayoutFunc(func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, _ map[string]any) error {
		layoutCalled = true
		return burrow.HTML(w, code, "<layout>"+string(content)+"</layout>")
	})
	ctx := burrow.WithLayout(req.Context(), layout)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, layoutCalled, "layout should not be called for HTMX nav request")
	assert.Contains(t, rec.Body.String(), "Test")
	assert.NotContains(t, rec.Body.String(), "<layout>")
}

func TestListNotesNormalRequestUsesLayout(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Test", Content: "Content", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)

	layoutCalled := false
	layout := burrow.LayoutFunc(func(w http.ResponseWriter, _ *http.Request, code int, content template.HTML, data map[string]any) error {
		layoutCalled = true
		assert.Equal(t, "Notes", data["Title"])
		return burrow.HTML(w, code, "<layout>"+string(content)+"</layout>")
	})
	ctx := burrow.WithLayout(req.Context(), layout)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, layoutCalled, "layout should be called for normal request")
	assert.Contains(t, rec.Body.String(), "<layout>")
}

func TestListNotesUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

// --- New handler ---

func TestNewNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/new", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	// HTMX request: returns form fragment for inline insertion.
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	err := h.New(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-new-title")
	assert.Contains(t, body, `action="/notes"`)
	assert.Contains(t, body, `hx-post="/notes"`)
	assert.Contains(t, body, `name="title"`)
	assert.Contains(t, body, `name="content"`)
}

func TestNewNoteUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/new", nil)
	rec := httptest.NewRecorder()

	err := h.New(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

// --- Create handler ---

func TestCreateNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=My+Note&content=Some+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = requestWithUser(req, &auth.User{ID: 42})
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// OOB: new card prepended to grid.
	assert.Contains(t, body, "My Note")
	assert.Contains(t, body, `hx-swap-oob="afterbegin"`)
	// OOB: form cleared.
	assert.Contains(t, body, `id="note-form"`)
	// OOB: flash message.
	assert.Contains(t, body, "notes-created")

	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "My Note", notes[0].Title)
}

func TestCreateNoteNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=My+Note&content=Some+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/notes", rec.Header().Get("Location"))

	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "My Note", notes[0].Title)
}

func TestCreateNoteValidationErrorHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Empty title should fail validation.
	form := strings.NewReader("title=&content=Some+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// HTMX gets 200 so the response is swapped into #note-form.
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-new-title")
	assert.Contains(t, body, `action="/notes"`)
	assert.Contains(t, body, "is-invalid")

	// No note should have been created.
	notes, err := repo.ListByUserID(context.Background(), 42)
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestCreateNoteValidationErrorNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes", func(w http.ResponseWriter, r *http.Request) {
		err := h.Create(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=&content=Some+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-new-title")
	assert.Contains(t, body, `action="/notes"`)
}

func TestCreateNoteUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	form := strings.NewReader("title=Test&content=Content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	err := h.Create(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

// --- Edit handler ---

func TestEditNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Edit Me", Content: "Original", UserID: 42}
	require.NoError(t, repo.Create(t.Context(), note))

	h := NewHandlers(repo)

	r := chi.NewRouter()
	r.Get("/notes/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		err := h.Edit(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/notes/%d/edit", note.ID), nil)
	req.Header.Set("HX-Request", "true")
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-edit-title")
	assert.Contains(t, body, "Edit Me")
	assert.Contains(t, body, "Original")
	assert.Contains(t, body, fmt.Sprintf(`action="/notes/%d"`, note.ID))
	assert.Contains(t, body, fmt.Sprintf(`hx-post="/notes/%d"`, note.ID))
}

func TestEditNoteUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/1/edit", nil)
	rec := httptest.NewRecorder()

	err := h.Edit(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestEditNoteNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	r := chi.NewRouter()
	r.Get("/notes/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		err := h.Edit(w, r)
		if err != nil {
			var httpErr *burrow.HTTPError
			if assert.ErrorAs(t, err, &httpErr) {
				http.Error(w, httpErr.Message, httpErr.Code)
			}
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/999/edit", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Update handler ---

func TestUpdateNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: 42}
	require.NoError(t, repo.Create(t.Context(), note))

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Update(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=Updated&content=New+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/notes/%d", note.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = requestWithUser(req, &auth.User{ID: 42})
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// OOB: updated card replaces existing.
	assert.Contains(t, body, "Updated")
	assert.Contains(t, body, `hx-swap-oob="outerHTML"`)
	// OOB: form cleared.
	assert.Contains(t, body, `id="note-form"`)
	// OOB: flash message.
	assert.Contains(t, body, "notes-updated")

	found, err := repo.GetByID(context.Background(), note.ID, 42)
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
	assert.Equal(t, "New content", found.Content)
}

func TestUpdateNoteNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: 42}
	require.NoError(t, repo.Create(t.Context(), note))

	h := NewHandlers(repo)

	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Post("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Update(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=Updated&content=New+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/notes/%d", note.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/notes", rec.Header().Get("Location"))

	found, err := repo.GetByID(context.Background(), note.ID, 42)
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
}

func TestUpdateNoteValidationErrorHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: 42}
	require.NoError(t, repo.Create(t.Context(), note))

	h := NewHandlers(repo)

	exec := testTemplateExecutor(t)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Post("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Update(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	form := strings.NewReader("title=&content=New+content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("/notes/%d", note.ID), form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// HTMX gets 200 so the response is swapped.
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-edit-title")
	assert.Contains(t, body, "is-invalid")

	// Note should be unchanged.
	found, err := repo.GetByID(context.Background(), note.ID, 42)
	require.NoError(t, err)
	assert.Equal(t, "Original", found.Title)
}

func TestUpdateNoteUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	form := strings.NewReader("title=Test&content=Content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	err := h.Update(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestUpdateNoteNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	r := chi.NewRouter()
	r.Post("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Update(w, r)
		if err != nil {
			var httpErr *burrow.HTTPError
			if assert.ErrorAs(t, err, &httpErr) {
				http.Error(w, httpErr.Message, httpErr.Code)
			}
		}
	})

	form := strings.NewReader("title=Test&content=Content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/999", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Delete handler ---

func TestDeleteNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: 42}
	require.NoError(t, repo.Create(ctx, note))

	h := NewHandlers(repo)

	// Use chi router to inject URL params; include messages middleware for store.
	exec := testTemplateExecutor(t)
	msgMW := messages.New().Middleware()[0]
	r := chi.NewRouter()
	r.Use(msgMW)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := burrow.WithTemplateExecutor(r.Context(), exec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Delete("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Delete(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/1", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `hx-swap-oob="true"`)
	assert.Contains(t, rec.Body.String(), "notes-deleted")
}

func TestDeleteNoteUnauthenticated(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/1", nil)
	rec := httptest.NewRecorder()

	err := h.Delete(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestDeleteNoteInvalidID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)

	r := chi.NewRouter()
	r.Delete("/notes/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := h.Delete(w, r)
		if err != nil {
			var httpErr *burrow.HTTPError
			if assert.ErrorAs(t, err, &httpErr) {
				http.Error(w, httpErr.Message, httpErr.Code)
			}
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/abc", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- ModelAdmin integration tests ---

func TestModelAdminRoutes_List(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Admin View", Content: "Visible", UserID: 42}))

	app := New()
	require.NoError(t, app.Register(&burrow.AppConfig{DB: db}))

	r := chi.NewRouter()
	app.AdminRoutes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Admin View")
}

func TestModelAdminRoutes_Delete(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: 42}
	require.NoError(t, repo.Create(ctx, note))

	app := New()
	require.NoError(t, app.Register(&burrow.AppConfig{DB: db}))

	r := chi.NewRouter()
	app.AdminRoutes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, fmt.Sprintf("/notes/%d", note.ID), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/admin/notes", rec.Header().Get("HX-Redirect"))

	// Verify deletion.
	count, err := db.NewSelect().Model((*Note)(nil)).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestAdminNavItems(t *testing.T) {
	app := New()
	items := app.AdminNavItems()

	require.Len(t, items, 1)
	assert.Equal(t, "Notes", items[0].Label)
	assert.Equal(t, "admin-nav-notes", items[0].LabelKey)
	assert.Equal(t, "/admin/notes", items[0].URL)
	assert.True(t, items[0].AdminOnly)
	assert.NotNil(t, items[0].Icon)
	assert.Equal(t, 30, items[0].Position)
}

func TestDependencies(t *testing.T) {
	app := New()
	deps := app.Dependencies()
	require.Len(t, deps, 1)
	assert.Equal(t, "auth", deps[0])
}

func TestRoutesNilHandlers(t *testing.T) {
	// Before Register is called, handlers is nil — Routes should be a no-op.
	app := New()
	r := chi.NewRouter()
	app.Routes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminRoutesNilNotesAdmin(t *testing.T) {
	// Before Register is called, notesAdmin is nil — AdminRoutes should be a no-op.
	app := New()
	r := chi.NewRouter()
	app.AdminRoutes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFuncMapIconFunctions(t *testing.T) {
	app := New()
	fm := app.FuncMap()

	plusLgFn, ok := fm["iconPlusLg"].(func(class ...string) template.HTML)
	require.True(t, ok)
	result := plusLgFn()
	assert.NotEmpty(t, result)
	assert.Contains(t, string(result), "<svg")

	pencilFn, ok := fm["iconPencil"].(func(class ...string) template.HTML)
	require.True(t, ok)
	result = pencilFn()
	assert.NotEmpty(t, result)
	assert.Contains(t, string(result), "<svg")

	journalTextFn, ok := fm["iconJournalText"].(func(class ...string) template.HTML)
	require.True(t, ok)
	result = journalTextFn("my-class")
	assert.NotEmpty(t, result)
	assert.Contains(t, string(result), "<svg")
}

func TestListNotesNoTemplateExecutor(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	// No template executor in context.
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.Error(t, err)
	var httpErr *burrow.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestListNotesHTMXScrollReturnsFragment(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Scroll Note", Content: "Content", UserID: 42}))

	h := NewHandlers(repo)
	// HTMX request with cursor → triggers the infinite scroll branch.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes?cursor=9999&limit=10", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Scroll Note")
}

// --- Pagination & search tests ---

func TestListByUserIDPaged(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create enough notes to test pagination.
	for i := range 5 {
		require.NoError(t, repo.Create(ctx, &Note{
			Title:   fmt.Sprintf("Note %d", i),
			Content: "Content",
			UserID:  1,
		}))
	}

	// First page with limit 3.
	pr := burrow.PageRequest{Limit: 3}
	notes, page, err := repo.ListByUserIDPaged(ctx, 1, pr)
	require.NoError(t, err)
	assert.Len(t, notes, 3)
	assert.True(t, page.HasMore)
	assert.NotEmpty(t, page.NextCursor)

	// Second page using cursor.
	pr2 := burrow.PageRequest{Limit: 3, Cursor: page.NextCursor}
	notes2, page2, err := repo.ListByUserIDPaged(ctx, 1, pr2)
	require.NoError(t, err)
	assert.Len(t, notes2, 2)
	assert.False(t, page2.HasMore)
}

func TestListByUserIDPagedEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	pr := burrow.PageRequest{Limit: 10}
	notes, page, err := repo.ListByUserIDPaged(t.Context(), 999, pr)
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.False(t, page.HasMore)
	assert.Empty(t, page.NextCursor)
}

func TestSearchByUserID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Golang Tutorial", Content: "Learn Go basics", UserID: 1}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Python Guide", Content: "Learn Python", UserID: 1}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Golang Advanced", Content: "Concurrency in Go", UserID: 1}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Other User Note", Content: "Golang stuff", UserID: 2}))

	t.Run("matches word in title", func(t *testing.T) {
		notes, page, err := repo.SearchByUserID(ctx, 1, "Golang", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 2)
		assert.False(t, page.HasMore)
	})

	t.Run("matches word in content", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, 1, "Python", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Python Guide", notes[0].Title)
	})

	t.Run("does not return other user's notes", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, 2, "Golang", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Other User Note", notes[0].Title)
	})

	t.Run("empty query returns empty results", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, 1, "", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, 1, "Rust", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("syntax error returns empty results", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, 1, `"unclosed`, burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("pagination with cursor", func(t *testing.T) {
		notes, page, err := repo.SearchByUserID(ctx, 1, "Learn", burrow.PageRequest{Limit: 1})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.True(t, page.HasMore)
		assert.NotEmpty(t, page.NextCursor)

		notes2, page2, err := repo.SearchByUserID(ctx, 1, "Learn", burrow.PageRequest{Limit: 1, Cursor: page.NextCursor})
		require.NoError(t, err)
		assert.Len(t, notes2, 1)
		assert.False(t, page2.HasMore)
		assert.NotEqual(t, notes[0].ID, notes2[0].ID)
	})
}

func TestListNotesHandlerWithSearch(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &Note{Title: "Searchable Note", Content: "Find me", UserID: 42}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Other Note", Content: "Not this", UserID: 42}))

	h := NewHandlers(repo)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes?q=Searchable", nil)
	req = requestWithUser(req, &auth.User{ID: 42})
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Searchable Note")
	assert.NotContains(t, rec.Body.String(), "Other Note")
}
