package burrow

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// TestDB returns an in-memory SQLite database wrapped in a [bun.DB].
// The database is automatically closed when the test finishes.
func TestDB(t *testing.T) *bun.DB {
	t.Helper()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	return bun.NewDB(sqldb, sqlitedialect.New())
}

// TestErrorExecContext returns a context with a minimal [TemplateExecutor] that
// renders error templates as "<code>: <message>". Use this in tests that
// trigger error responses through [Handle] or [RenderError].
func TestErrorExecContext(ctx context.Context) context.Context {
	exec := TemplateExecutor(func(_ *http.Request, name string, data map[string]any) (template.HTML, error) {
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
