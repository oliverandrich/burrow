package burrow

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oliverandrich/den"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/oliverandrich/den/validate"
)

// TestDB returns a file-backed SQLite database wrapped in a [den.DB] for
// testing. Struct-tag validation is enabled by default to match [OpenDB].
// The database is created in [testing.T.TempDir] and closed automatically
// when the test finishes.
func TestDB(t *testing.T) *den.DB {
	t.Helper()
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := den.OpenURL(dsn, validate.WithValidation())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestErrorExecContext returns a context with a minimal [TemplateExecutor] that
// renders error templates as "<code>: <message>". Use this in tests that
// trigger error responses through [Handle] or [RenderError].
func TestErrorExecContext(ctx context.Context) context.Context {
	exec := TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if strings.HasPrefix(name, "error/") {
			return template.HTML(fmt.Sprintf("%d: %s", data["Code"], data["Message"])), nil //nolint:gosec // test helper
		}
		return "", fmt.Errorf("template %q not found", name)
	})
	return WithTemplateExecutor(ctx, exec)
}

// TestErrorExecMiddleware is an HTTP middleware that injects [TestErrorExecContext]
// into the request context. Use this in tests that need error rendering support.
func TestErrorExecMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(TestErrorExecContext(r.Context())))
	})
}
