package burrowtest

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/oliverandrich/burrow"
)

// ErrorExecContext returns a context with a minimal [burrow.TemplateExecutor]
// that renders error templates as "<code>: <message>". Use this in tests
// that trigger error responses through [burrow.Handle] or [burrow.RenderError].
func ErrorExecContext(ctx context.Context) context.Context {
	exec := burrow.TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if strings.HasPrefix(name, "error/") {
			return template.HTML(fmt.Sprintf("%d: %s", data["Code"], data["Message"])), nil //nolint:gosec // test helper
		}
		return "", fmt.Errorf("template %q not found", name)
	})
	return burrow.WithTemplateExecutor(ctx, exec)
}

// ErrorExecMiddleware is an HTTP middleware that injects [ErrorExecContext]
// into the request context. Use this in tests that need error rendering
// support.
func ErrorExecMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(ErrorExecContext(r.Context())))
	})
}
