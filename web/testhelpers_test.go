package web

import (
	"context"
	"fmt"
	"html/template"
	"strings"

	"github.com/oliverandrich/burrow/app"
)

// testErrorExecContext returns a context with a minimal TemplateExecutor
// that renders the "error/" templates used by HTTPError responses. Mirrors
// internal_test_helpers_test.go in package burrow — duplicated rather than
// imported because burrowtest would create an import cycle (burrowtest
// imports burrow, and burrow imports web via the wrapper functions).
func testErrorExecContext(ctx context.Context) context.Context {
	exec := app.TemplateExecutor(func(_ context.Context, name string, data map[string]any) (template.HTML, error) {
		if strings.HasPrefix(name, "error/") {
			return template.HTML(fmt.Sprintf("%d: %s", data["Code"], data["Message"])), nil //nolint:gosec // test helper
		}
		return "", fmt.Errorf("template %q not found", name)
	})
	return app.WithTemplateExecutor(ctx, exec)
}
