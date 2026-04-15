package notes

import (
	"context"
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
	"github.com/oliverandrich/burrow/contrib/auth/authtest"
	"github.com/oliverandrich/burrow/contrib/messages"
	"github.com/oliverandrich/burrow/contrib/session"
	"github.com/oliverandrich/den"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasDocuments    = (*App)(nil)
	_ burrow.HasRoutes       = (*App)(nil)
	_ burrow.HasNavItems     = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.HasAdmin        = (*App)(nil)
	_ burrow.HasTranslations = (*App)(nil)
	_ burrow.HasTemplates    = (*App)(nil)
)

// testUser returns an auth.User with a fixed test ID for use in test contexts.
func testUser() *auth.User {
	u := &auth.User{Username: "testuser"}
	u.ID = "user-42"
	return u
}

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

func TestDocuments(t *testing.T) {
	app := New()
	docs := app.Documents()
	require.NotEmpty(t, docs)
	assert.Len(t, docs, 1, "should have Note document type")
}

// --- Repository tests ---

func openTestDB(t *testing.T) *den.DB {
	t.Helper()

	db := authtest.NewDB(t)

	// Register notes documents on top of auth.
	app := New()
	err := den.Register(t.Context(), db, app.Documents()...)
	require.NoError(t, err)

	// Create default test users (tests use UserID strings throughout).
	authtest.CreateUser(t, db, authtest.WithID("user-default"), authtest.WithUsername("defaultuser"))
	authtest.CreateUser(t, db, authtest.WithID("user-42"), authtest.WithUsername("testuser"))

	return db
}

func TestCreateAndListNotes(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	err := repo.Create(ctx, &Note{Title: "First Note", Content: "Hello", UserID: "user-default"})
	require.NoError(t, err)

	err = repo.Create(ctx, &Note{Title: "Second Note", Content: "World", UserID: "user-default"})
	require.NoError(t, err)

	notes, err := repo.ListByUserID(ctx, "user-default")
	require.NoError(t, err)
	require.Len(t, notes, 2)
	assert.Equal(t, "Second Note", notes[0].Title) // Most recent first.
	assert.Equal(t, "First Note", notes[1].Title)
}

func TestListNotesEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	notes, err := repo.ListByUserID(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestDeleteNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "To Delete", Content: "Bye", UserID: "user-default"}
	err := repo.Create(ctx, note)
	require.NoError(t, err)

	err = repo.Delete(ctx, note.ID, "user-default")
	require.NoError(t, err)

	notes, err := repo.ListByUserID(ctx, "user-default")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestDeleteNoteWrongUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Not Yours", Content: "Nope", UserID: "user-default"}
	err := repo.Create(ctx, note)
	require.NoError(t, err)

	// User 2 can't delete user 1's note.
	err = repo.Delete(ctx, note.ID, "user-other")
	require.NoError(t, err) // No error but nothing happens.

	notes, err := repo.ListByUserID(ctx, "user-default")
	require.NoError(t, err)
	assert.Len(t, notes, 1) // Still there.
}

func TestGetByID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Find Me", Content: "Here", UserID: "user-default"}
	require.NoError(t, repo.Create(ctx, note))

	found, err := repo.GetByID(ctx, note.ID, "user-default")
	require.NoError(t, err)
	assert.Equal(t, "Find Me", found.Title)
	assert.Equal(t, "Here", found.Content)
}

func TestGetByIDWrongUser(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Not Yours", Content: "Nope", UserID: "user-default"}
	require.NoError(t, repo.Create(ctx, note))

	_, err := repo.GetByID(ctx, note.ID, "user-other")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestUpdateNote(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Original", Content: "Old", UserID: "user-default"}
	require.NoError(t, repo.Create(ctx, note))

	note.Title = "Updated"
	note.Content = "New"
	require.NoError(t, repo.Update(ctx, note))

	found, err := repo.GetByID(ctx, note.ID, "user-default")
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
	assert.Equal(t, "New", found.Content)
}

// --- Handler tests ---

// testTemplateExecutor builds a real template executor from the notes templates
// plus a minimal app/alerts_oob stub for handler tests.
func testTemplateExecutor(t *testing.T) burrow.TemplateExecutor {
	t.Helper()

	app := New()
	// Stubs for request-scoped functions, icons, and functions from other apps.
	fm := template.FuncMap{
		"t":               func(key string) string { return key },
		"csrfToken":       func() string { return "test-token" },
		"staticURL":       func(name string) string { return "/static/" + name },
		"iconTrash":       func(class ...string) template.HTML { return "<svg>trash</svg>" },
		"iconPlusLg":      func(class ...string) template.HTML { return "<svg>plus</svg>" },
		"iconPencil":      func(class ...string) template.HTML { return "<svg>pencil</svg>" },
		"iconJournalText": func(class ...string) template.HTML { return "<svg>journal</svg>" },
		"alertClass":      func(level messages.Level) string { return string(level) },
		"add":             func(a, b int) int { return a + b },
		"sub":             func(a, b int) int { return a - b },
	}

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

	return func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
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

	require.NoError(t, repo.Create(ctx, &Note{Title: "Test", Content: "Content", UserID: "user-42"}))

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
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

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Test", Content: "Content", UserID: "user-42"}))

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = injectTemplateExecutor(t, req)
	// HTMX nav request (no page param) → should use Render → fragment only.
	req.Header.Set("HX-Request", "true")

	ctx := burrow.WithLayout(req.Context(), "test-layout")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Test")
	assert.NotContains(t, rec.Body.String(), "<layout>")
}

func TestListNotesNormalRequestUsesLayout(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Test", Content: "Content", UserID: "user-42"}))

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))

	layoutCalled := false
	exec := burrow.TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if name == "test-layout" {
			layoutCalled = true
			assert.Equal(t, "Notes", data["Title"])
			return template.HTML("<layout>" + string(data["Content"].(template.HTML)) + "</layout>"), nil
		}
		return template.HTML("<rendered:" + name + ">"), nil
	})
	ctx := burrow.WithTemplateExecutor(req.Context(), exec)
	ctx = burrow.WithLayout(ctx, "test-layout")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, layoutCalled, "layout should be called for normal request")
	assert.Contains(t, rec.Body.String(), "<layout>")
}

func TestListNotesUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.List(rec, req)
	})
}

// --- New handler ---

func TestNewNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/new", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
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

func TestNewNoteUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/new", nil)
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.New(rec, req)
	})
}

// --- Create handler ---

func TestCreateNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
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

	notes, err := repo.ListByUserID(context.Background(), "user-42")
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "My Note", notes[0].Title)
}

func TestCreateNoteNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/notes", rec.Header().Get("Location"))

	notes, err := repo.ListByUserID(context.Background(), "user-42")
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "My Note", notes[0].Title)
}

func TestCreateNoteValidationErrorHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// HTMX gets 422 — htmx/config enables swapping on 422.
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-new-title")
	assert.Contains(t, body, `action="/notes"`)
	assert.Contains(t, body, "is-invalid")

	// No note should have been created.
	notes, err := repo.ListByUserID(context.Background(), "user-42")
	require.NoError(t, err)
	assert.Empty(t, notes)
}

func TestCreateNoteValidationErrorNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-new-title")
	assert.Contains(t, body, `action="/notes"`)
}

func TestCreateNoteUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	form := strings.NewReader("title=Test&content=Content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.Create(rec, req)
	})
}

// --- Edit handler ---

func TestEditNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Edit Me", Content: "Original", UserID: "user-42"}
	require.NoError(t, repo.Create(t.Context(), note))

	h := &App{repo: repo}

	r := chi.NewRouter()
	r.Get("/notes/{id}/edit", func(w http.ResponseWriter, r *http.Request) {
		err := h.Edit(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/"+note.ID+"/edit", nil)
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-edit-title")
	assert.Contains(t, body, "Edit Me")
	assert.Contains(t, body, "Original")
	assert.Contains(t, body, `action="/notes/`+note.ID+`"`)
	assert.Contains(t, body, `hx-post="/notes/`+note.ID+`"`)
}

func TestEditNoteUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/1/edit", nil)
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.Edit(rec, req)
	})
}

func TestEditNoteNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Update handler ---

func TestUpdateNoteHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: "user-42"}
	require.NoError(t, repo.Create(t.Context(), note))

	h := &App{repo: repo}

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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/"+note.ID, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
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

	found, err := repo.GetByID(context.Background(), note.ID, "user-42")
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
	assert.Equal(t, "New content", found.Content)
}

func TestUpdateNoteNonHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: "user-42"}
	require.NoError(t, repo.Create(t.Context(), note))

	h := &App{repo: repo}

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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/"+note.ID, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/notes", rec.Header().Get("Location"))

	found, err := repo.GetByID(context.Background(), note.ID, "user-42")
	require.NoError(t, err)
	assert.Equal(t, "Updated", found.Title)
}

func TestUpdateNoteValidationErrorHTMX(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	note := &Note{Title: "Original", Content: "Old", UserID: "user-42"}
	require.NoError(t, repo.Create(t.Context(), note))

	h := &App{repo: repo}

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
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/"+note.ID, form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// HTMX gets 422 — htmx/config enables swapping on 422.
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "notes-edit-title")
	assert.Contains(t, body, "is-invalid")

	// Note should be unchanged.
	found, err := repo.GetByID(context.Background(), note.ID, "user-42")
	require.NoError(t, err)
	assert.Equal(t, "Original", found.Title)
}

func TestUpdateNoteUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	form := strings.NewReader("title=Test&content=Content")
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes/1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.Update(rec, req)
	})
}

func TestUpdateNoteNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// --- Delete handler ---

func TestDeleteNoteHandler(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: "user-42"}
	require.NoError(t, repo.Create(ctx, note))

	h := &App{repo: repo}

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
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = session.Inject(req, map[string]any{})
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `hx-swap-oob="true"`)
	assert.Contains(t, rec.Body.String(), "notes-deleted")
}

func TestDeleteNoteUnauthenticatedPanics(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/1", nil)
	rec := httptest.NewRecorder()

	assert.Panics(t, func() {
		_ = h.Delete(rec, req)
	})
}

func TestDeleteNoteNonExistentIDIsNoOp(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	h := &App{repo: repo}

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

	// With string IDs, "abc" matches the route. Deleting a non-existent
	// note is a no-op, and the handler redirects.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/abc", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = session.Inject(req, map[string]any{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
}

// --- ModelAdmin integration tests ---

func TestAdminRoutes_RegistersWithoutPanic(t *testing.T) {
	db := openTestDB(t)

	app := New()
	require.NoError(t, app.Configure(&burrow.AppConfig{DB: db}, nil))

	r := chi.NewRouter()
	app.AdminRoutes(r) // Should not panic.
}

func TestAdminRoutes_Delete(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	note := &Note{Title: "Delete Me", Content: "Bye", UserID: "user-42"}
	require.NoError(t, repo.Create(ctx, note))

	app := New()
	require.NoError(t, app.Configure(&burrow.AppConfig{DB: db}, nil))

	r := chi.NewRouter()
	r.Use(burrow.TestErrorExecMiddleware)
	app.AdminRoutes(r)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/"+note.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/notes", rec.Header().Get("Location"))

	// Verify deletion.
	count, err := den.NewQuery[Note](ctx, db).Count()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
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

func TestListNotesHTMXScrollReturnsFragment(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	require.NoError(t, repo.Create(t.Context(), &Note{Title: "Scroll Note", Content: "Content", UserID: "user-42"}))

	h := &App{repo: repo}
	// HTMX request with page > 1 → triggers the infinite scroll branch.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes?page=2&limit=10", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = injectTemplateExecutor(t, req)
	req.Header.Set("HX-Request", "true")

	rec := httptest.NewRecorder()
	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
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
			UserID:  "user-default",
		}))
	}

	// First page with limit 3.
	pr := burrow.PageRequest{Limit: 3, Page: 1}
	notes, page, err := repo.ListByUserIDPaged(ctx, "user-default", pr)
	require.NoError(t, err)
	assert.Len(t, notes, 3)
	assert.True(t, page.HasMore)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 2, page.TotalPages)
	assert.Equal(t, 5, page.TotalCount)

	// Second page.
	pr2 := burrow.PageRequest{Limit: 3, Page: 2}
	notes2, page2, err := repo.ListByUserIDPaged(ctx, "user-default", pr2)
	require.NoError(t, err)
	assert.Len(t, notes2, 2)
	assert.False(t, page2.HasMore)
}

func TestListByUserIDPagedEmpty(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)

	pr := burrow.PageRequest{Limit: 10, Page: 1}
	notes, page, err := repo.ListByUserIDPaged(t.Context(), "nonexistent", pr)
	require.NoError(t, err)
	assert.Empty(t, notes)
	assert.False(t, page.HasMore)
	assert.Equal(t, 0, page.TotalCount)
}

func TestSearchByUserID(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	authtest.CreateUser(t, db, authtest.WithID("user-other"), authtest.WithUsername("otheruser"))

	require.NoError(t, repo.Create(ctx, &Note{Title: "Golang Tutorial", Content: "Learn Go basics", UserID: "user-default"}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Python Guide", Content: "Learn Python", UserID: "user-default"}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Golang Advanced", Content: "Concurrency in Go", UserID: "user-default"}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Other User Note", Content: "Golang stuff", UserID: "user-other"}))

	t.Run("matches word in title", func(t *testing.T) {
		notes, page, err := repo.SearchByUserID(ctx, "user-default", "Golang", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 2)
		assert.False(t, page.HasMore)
	})

	t.Run("matches word in content", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, "user-default", "Python", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Python Guide", notes[0].Title)
	})

	t.Run("does not return other user's notes", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, "user-other", "Golang", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.Equal(t, "Other User Note", notes[0].Title)
	})

	t.Run("empty query returns empty results", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, "user-default", "", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, "user-default", "Rust", burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("syntax error returns empty results", func(t *testing.T) {
		notes, _, err := repo.SearchByUserID(ctx, "user-default", `"unclosed`, burrow.PageRequest{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, notes)
	})

	t.Run("pagination with offset", func(t *testing.T) {
		notes, page, err := repo.SearchByUserID(ctx, "user-default", "Learn", burrow.PageRequest{Limit: 1, Page: 1})
		require.NoError(t, err)
		assert.Len(t, notes, 1)
		assert.True(t, page.HasMore)
		assert.Equal(t, 2, page.TotalCount)

		notes2, page2, err := repo.SearchByUserID(ctx, "user-default", "Learn", burrow.PageRequest{Limit: 1, Page: 2})
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

	require.NoError(t, repo.Create(ctx, &Note{Title: "Searchable Note", Content: "Find me", UserID: "user-42"}))
	require.NoError(t, repo.Create(ctx, &Note{Title: "Other Note", Content: "Not this", UserID: "user-42"}))

	h := &App{repo: repo}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes?q=Searchable", nil)
	req = req.WithContext(auth.WithUser(req.Context(), testUser()))
	req = injectTemplateExecutor(t, req)
	rec := httptest.NewRecorder()

	err := h.List(rec, req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Searchable Note")
	assert.NotContains(t, rec.Body.String(), "Other Note")
}
