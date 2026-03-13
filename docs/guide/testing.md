# Testing

This guide covers the patterns used throughout burrow for testing handlers, middleware, migrations, and templates. All examples use [testify](https://github.com/stretchr/testify) for assertions and Go's standard `net/http/httptest` package.

## Common Test Imports

Most test files use a subset of these imports:

```go
import (
    "context"
    "database/sql"
    "html/template"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "testing/fstest"

    "github.com/go-chi/chi/v5"
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/auth"
    "github.com/oliverandrich/burrow/contrib/session"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/uptrace/bun"
    "github.com/uptrace/bun/dialect/sqlitedialect"
    "modernc.org/sqlite/lib/sqliteshim"
)
```

## Test Database Setup

Create an in-memory SQLite database for each test. Use `t.Cleanup()` to close it automatically:

```go
func testDB(t *testing.T) *bun.DB {
    t.Helper()
    sqldb, err := sql.Open(sqliteshim.ShimName, ":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { sqldb.Close() })
    return bun.NewDB(sqldb, sqlitedialect.New())
}
```

If your tests need migrations (most app tests do), run them after opening the database:

```go
func testDB(t *testing.T) *bun.DB {
    t.Helper()
    sqldb, err := sql.Open(sqliteshim.ShimName,
        "file::memory:?cache=shared&_pragma=foreign_keys(1)")
    require.NoError(t, err)
    t.Cleanup(func() { sqldb.Close() })
    db := bun.NewDB(sqldb, sqlitedialect.New())

    // New() is your app's constructor — see "Creating an App".
    app := New()
    err = burrow.RunAppMigrations(t.Context(), db, app.Name(), app.MigrationFS())
    require.NoError(t, err)
    return db
}
```

!!! tip "Shared cache for concurrent access"
    Use `file::memory:?cache=shared` when your handler opens transactions or when multiple goroutines access the same database. Plain `:memory:` creates a private database per connection.

## Testing Handlers

Burrow handlers return errors (`burrow.HandlerFunc`). Use `httptest.NewRecorder` and `httptest.NewRequestWithContext` to test them:

```go
func TestGreetHandler(t *testing.T) {
    handler := burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.Text(w, http.StatusOK, "hello")
    })

    req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Equal(t, "hello", rec.Body.String())
}
```

### Testing Error Responses

Handlers signal errors by returning a `*burrow.HTTPError`. When wrapped with `burrow.Handle()`, the error is converted into the appropriate HTTP status code:

```go
func TestNotFound(t *testing.T) {
    handler := burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
        return burrow.NewHTTPError(http.StatusNotFound, "note not found")
    })

    req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes/999", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusNotFound, rec.Code)
    assert.Contains(t, rec.Body.String(), "note not found")
}
```

### Form Submissions

Set the `Content-Type` header and pass form data as the request body:

```go
func TestBindForm(t *testing.T) {
    body := strings.NewReader("title=My+Note&content=Some+text")
    req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", body)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    var form struct {
        Title   string `form:"title"`
        Content string `form:"content"`
    }
    err := burrow.Bind(req, &form)

    require.NoError(t, err)
    assert.Equal(t, "My Note", form.Title)
}
```

### JSON Requests

For JSON APIs, set the content type and pass a JSON body:

```go
func TestBindJSON(t *testing.T) {
    body := strings.NewReader(`{"title":"My Note","content":"Some text"}`)
    req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/notes", body)
    req.Header.Set("Content-Type", "application/json")

    var payload struct {
        Title   string `json:"title"`
        Content string `json:"content"`
    }
    err := burrow.Bind(req, &payload)

    require.NoError(t, err)
    assert.Equal(t, "My Note", payload.Title)
}
```

### URL Parameters

Chi URL parameters require a real chi router to populate the context:

```go
func TestDeleteNote(t *testing.T) {
    r := chi.NewRouter()
    r.Delete("/notes/{id}", burrow.Handle(handler.Delete))

    req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/notes/42", nil)
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}
```

## Testing with Authentication

Use `auth.WithUser()` to inject an authenticated user into the request context:

```go
func requestWithUser(req *http.Request, user *auth.User) *http.Request {
    ctx := auth.WithUser(req.Context(), user)
    return req.WithContext(ctx)
}
```

Use `session.Inject()` when your handler reads or writes session data:

```go
req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
req = requestWithUser(req, &auth.User{ID: 42, Role: auth.RoleUser})
req = session.Inject(req, map[string]any{})
```

`session.Inject` sets up an in-memory session store — no cookie middleware needed. Pass initial session values via the map:

```go
// Pre-populate session with flash messages, return URL, etc.
req = session.Inject(req, map[string]any{
    "return_to": "/dashboard",
})
```

## Testing HTMX Responses

Burrow automatically detects HTMX requests via the `HX-Request` header and skips the layout, returning only the template fragment. Test both paths to verify layout behaviour:

```go
func TestListNotes(t *testing.T) {
    db := testDB(t)
    repo := NewRepository(db)
    require.NoError(t, repo.Create(t.Context(), &Note{Title: "Test", UserID: 42}))
    h := NewHandlers(repo)

    setup := func(t *testing.T) *http.Request {
        t.Helper()
        req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
        req = requestWithUser(req, &auth.User{ID: 42})
        req = injectTemplateExecutor(t, req)
        return req
    }

    t.Run("full page includes layout", func(t *testing.T) {
        req := setup(t)
        ctx := burrow.WithLayout(req.Context(), "test-layout")
        req = req.WithContext(ctx)

        rec := httptest.NewRecorder()
        err := h.List(rec, req)

        require.NoError(t, err)
        assert.Contains(t, rec.Body.String(), "<html")
    })

    t.Run("htmx returns fragment only", func(t *testing.T) {
        req := setup(t)
        req.Header.Set("HX-Request", "true")
        ctx := burrow.WithLayout(req.Context(), "test-layout")
        req = req.WithContext(ctx)

        rec := httptest.NewRecorder()
        err := h.List(rec, req)

        require.NoError(t, err)
        assert.NotContains(t, rec.Body.String(), "<html")
        assert.Contains(t, rec.Body.String(), "Test")
    })
}
```

For HTMX responses with out-of-band swaps (e.g., flash messages after creating a record), check for the `hx-swap-oob` attribute:

```go
assert.Contains(t, rec.Body.String(), `hx-swap-oob="true"`)
```

### Testing Redirects

Non-HTMX form submissions typically redirect. Check the status code and `Location` header:

```go
assert.Equal(t, http.StatusSeeOther, rec.Code)
assert.Equal(t, "/notes", rec.Header().Get("Location"))
```

## Testing Migrations

Use `fstest.MapFS` to create migration files in memory:

```go
func TestMigrations(t *testing.T) {
    db := testDB(t)

    migrations := fstest.MapFS{
        "001_create_items.up.sql": &fstest.MapFile{
            Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);"),
        },
    }

    err := burrow.RunAppMigrations(t.Context(), db, "myapp", migrations)
    require.NoError(t, err)

    // Verify the table was created.
    _, err = db.NewInsert().
        Model(&struct{ Name string }{Name: "test"}).
        TableExpr("items").
        Exec(t.Context())
    require.NoError(t, err)
}
```

Test that failed migrations don't leave partial state:

```go
func TestMigrationRollback(t *testing.T) {
    db := testDB(t)

    migrations := fstest.MapFS{
        "001_create_items.up.sql": &fstest.MapFile{
            Data: []byte("CREATE TABLE items (id INTEGER PRIMARY KEY);"),
        },
        "002_bad.up.sql": &fstest.MapFile{
            Data: []byte("THIS IS NOT SQL;"),
        },
    }

    err := burrow.RunAppMigrations(t.Context(), db, "myapp", migrations)
    require.Error(t, err)

    // First migration should be committed, second should not.
    var count int
    err = db.NewRaw("SELECT COUNT(*) FROM _migrations WHERE app = ?", "myapp").
        Scan(t.Context(), &count)
    require.NoError(t, err)
    assert.Equal(t, 1, count)
}
```

## Testing Middleware

Use a chi router to test middleware with the full request pipeline:

```go
func TestRequireAuth(t *testing.T) {
    t.Run("redirects unauthenticated", func(t *testing.T) {
        r := chi.NewRouter()
        r.Use(auth.RequireAuth())
        r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        })

        req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected", nil)
        req = session.Inject(req, map[string]any{})
        rec := httptest.NewRecorder()
        r.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusSeeOther, rec.Code)
        assert.Equal(t, "/auth/login", rec.Header().Get("Location"))
    })

    t.Run("allows authenticated", func(t *testing.T) {
        r := chi.NewRouter()
        // Inject user before the middleware.
        r.Use(func(next http.Handler) http.Handler {
            return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                ctx := auth.WithUser(r.Context(), &auth.User{ID: 1})
                next.ServeHTTP(w, r.WithContext(ctx))
            })
        })
        r.Use(auth.RequireAuth())
        r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
        })

        req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/protected", nil)
        rec := httptest.NewRecorder()
        r.ServeHTTP(rec, req)

        assert.Equal(t, http.StatusOK, rec.Code)
    })
}
```

Use table-driven tests for middleware that checks roles or permissions:

```go
func TestRequireAdmin(t *testing.T) {
    tests := []struct {
        name       string
        role       string
        wantStatus int
    }{
        {"forbids non-admin", auth.RoleUser, http.StatusForbidden},
        {"allows admin", auth.RoleAdmin, http.StatusOK},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := chi.NewRouter()
            r.Use(func(next http.Handler) http.Handler {
                return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                    ctx := auth.WithUser(r.Context(), &auth.User{ID: 1, Role: tt.role})
                    next.ServeHTTP(w, r.WithContext(ctx))
                })
            })
            r.Use(auth.RequireAdmin())
            r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
            })

            req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin", nil)
            rec := httptest.NewRecorder()
            r.ServeHTTP(rec, req)

            assert.Equal(t, tt.wantStatus, rec.Code)
        })
    }
}
```

## Testing Validation

`burrow.Validate()` returns a `*burrow.ValidationError` with per-field errors:

```go
func TestValidation(t *testing.T) {
    form := struct {
        Email string `validate:"required,email"`
        Age   int    `validate:"min=18"`
    }{
        Email: "",
        Age:   15,
    }

    err := burrow.Validate(form)
    require.Error(t, err)

    var ve *burrow.ValidationError
    require.ErrorAs(t, err, &ve)
    assert.Len(t, ve.Errors, 2)
    assert.True(t, ve.HasField("Email"))
    assert.True(t, ve.HasField("Age"))
}
```

## Integration Tests

The examples above test individual pieces in isolation. A complete integration test wires up the chi router with middleware, database, and handler — similar to how the real server processes a request:

```go
func TestCreateNoteIntegration(t *testing.T) {
    db := testDB(t)
    repo := NewRepository(db)
    h := NewHandlers(repo)

    r := chi.NewRouter()
    r.Use(session.New().Middleware()...)
    r.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := auth.WithUser(r.Context(), &auth.User{ID: 1})
            ctx = burrow.WithTemplateExecutor(ctx, testTemplateExecutor(t))
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    })
    r.Post("/notes", burrow.Handle(h.Create))

    body := strings.NewReader("title=Integration+Test&content=Works")
    req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/notes", body)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("HX-Request", "true")
    req = session.Inject(req, map[string]any{})
    rec := httptest.NewRecorder()
    r.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    assert.Contains(t, rec.Body.String(), "Integration Test")

    // Verify the note was persisted.
    notes, err := repo.ListByUserID(t.Context(), 1)
    require.NoError(t, err)
    assert.Len(t, notes, 1)
}
```

## Testing Templates

At runtime, the server parses all app templates into a single global `*template.Template` and injects a `TemplateExecutor` function into every request context. In tests, you must build this yourself because there is no running server. You only need this when testing handlers that call `burrow.RenderTemplate` and you want to verify the rendered HTML.

Build a `TemplateExecutor` from your app's template files with stub functions for dependencies from other apps (like `csrfToken` from CSRF or `t` from i18n):

```go
func testTemplateExecutor(t *testing.T) burrow.TemplateExecutor {
    t.Helper()

    // New() is your app's constructor. FuncMap() and TemplateFS() come from
    // the HasFuncMap and HasTemplates interfaces — see the Interfaces reference.
    app := New()
    fm := app.FuncMap()
    // Stub request-scoped functions provided by other apps at runtime.
    fm["t"] = func(key string) string { return key }
    fm["csrfToken"] = func() string { return "test-token" }
    fm["staticURL"] = func(name string) string { return "/static/" + name }

    tmpl := template.New("").Funcs(fm)

    // Parse templates from the app's embedded FS (the templates/ directory).
    fsys := app.TemplateFS()
    err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return err
        }
        data, readErr := fs.ReadFile(fsys, path)
        if readErr != nil {
            return readErr
        }
        _, parseErr := tmpl.Parse(string(data))
        return parseErr
    })
    require.NoError(t, err)

    return func(r *http.Request, name string, data map[string]any) (template.HTML, error) {
        var buf strings.Builder
        if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
            return "", err
        }
        return template.HTML(buf.String()), nil
    }
}
```

Inject it into the request context:

```go
func injectTemplateExecutor(t *testing.T, req *http.Request) *http.Request {
    t.Helper()
    exec := testTemplateExecutor(t)
    ctx := burrow.WithTemplateExecutor(req.Context(), exec)
    return req.WithContext(ctx)
}
```

If your templates reference templates from other apps (e.g., `{{ template "app/alerts_oob" . }}`), add stubs for those too:

```go
_, err = tmpl.Parse(`{{ define "app/alerts_oob" }}{{ end }}`)
require.NoError(t, err)
```

## Test Helpers (`authtest`)

The `contrib/auth/authtest` package provides shared helpers for tests that depend on the auth app — following the convention of `net/http/httptest`.

### Database with Auth Migrations

`authtest.NewDB` returns an in-memory SQLite database with all auth migrations already applied:

```go
import "github.com/oliverandrich/burrow/contrib/auth/authtest"

func testDB(t *testing.T) *bun.DB {
    t.Helper()
    db := authtest.NewDB(t)

    // Run your app's own migrations on top.
    app := New()
    err := burrow.RunAppMigrations(t.Context(), db, app.Name(), app.MigrationFS())
    require.NoError(t, err)
    return db
}
```

### Creating Test Users

`authtest.CreateUser` inserts a user with sensible defaults (unique username, role `"user"`, active). Use functional options to override:

```go
// Default user — unique username auto-generated.
user := authtest.CreateUser(t, db)

// Fully customised user.
admin := authtest.CreateUser(t, db,
    authtest.WithID(1),
    authtest.WithUsername("admin"),
    authtest.WithEmail("admin@example.com"),
    authtest.WithName("Admin"),
    authtest.WithRole("admin"),
)
```

Available options: `WithID`, `WithUsername`, `WithEmail`, `WithName`, `WithRole`, `WithActive`.

### Testing with Authentication

Inject a user into the request context with `auth.WithUser`:

```go
req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/notes", nil)
ctx := auth.WithUser(req.Context(), &auth.User{ID: 42, Role: auth.RoleUser})
req = req.WithContext(ctx)
```

Use `session.Inject` when your handler reads or writes session data:

```go
req = session.Inject(req, map[string]any{})
```

## Summary

| What to test | Key tools |
|---|---|
| Database queries | `sql.Open(sqliteshim.ShimName, ":memory:")`, `RunAppMigrations` |
| Auth test DB | `authtest.NewDB(t)` — in-memory DB with auth migrations |
| Test users | `authtest.CreateUser(t, db, ...options)` |
| Handlers | `httptest.NewRecorder`, `httptest.NewRequestWithContext(t.Context(), ...)` |
| Auth context | `auth.WithUser(ctx, user)`, `session.Inject(req, values)` |
| HTMX responses | `req.Header.Set("HX-Request", "true")`, check `hx-swap-oob` |
| Migrations | `fstest.MapFS`, `RunAppMigrations` |
| Middleware | Chi router with `r.Use()`, table-driven tests |
| Validation | `burrow.Validate()`, `errors.As(err, &ve)`, `ve.HasField()` |
| Templates | `TemplateExecutor` with stubbed functions, `WithTemplateExecutor` |
