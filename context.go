package burrow

import (
	"context"

	"github.com/oliverandrich/burrow/app"
)

// WithLayout stores the layout template name in the context. Wrapper around
// [app.WithLayout].
func WithLayout(ctx context.Context, name string) context.Context {
	return app.WithLayout(ctx, name)
}

// Layout retrieves the layout template name from the context. Wrapper around
// [app.Layout].
func Layout(ctx context.Context) string { return app.Layout(ctx) }

// WithNavItems stores navigation items in the context. Wrapper around
// [app.WithNavItems].
func WithNavItems(ctx context.Context, items []NavItem) context.Context {
	return app.WithNavItems(ctx, items)
}

// NavItems retrieves the navigation items from the context. Wrapper around
// [app.NavItems].
func NavItems(ctx context.Context) []NavItem { return app.NavItems(ctx) }

// WithTemplateExecutor stores the template executor in the context. Wrapper
// around [app.WithTemplateExecutor].
func WithTemplateExecutor(ctx context.Context, exec TemplateExecutor) context.Context {
	return app.WithTemplateExecutor(ctx, exec)
}

// TemplateExec retrieves the template executor from the context. Wrapper
// around [app.TemplateExec].
func TemplateExec(ctx context.Context) TemplateExecutor { return app.TemplateExec(ctx) }

// TemplateExecutorFromContext is a deprecated alias for [TemplateExec].
//
//go:fix inline
func TemplateExecutorFromContext(ctx context.Context) TemplateExecutor {
	return app.TemplateExec(ctx)
}

// WithAuthChecker stores an AuthChecker in the context. Wrapper around
// [app.WithAuthChecker].
func WithAuthChecker(ctx context.Context, checker AuthChecker) context.Context {
	return app.WithAuthChecker(ctx, checker)
}

// WithRequestPath stores the request path in the context. Wrapper around
// [app.WithRequestPath].
func WithRequestPath(ctx context.Context, path string) context.Context {
	return app.WithRequestPath(ctx, path)
}

// RequestPath retrieves the request path from the context. Wrapper around
// [app.RequestPath].
func RequestPath(ctx context.Context) string { return app.RequestPath(ctx) }

// IsAuthenticated returns true if the AuthChecker in context reports
// authentication. Wrapper around [app.IsAuthenticated].
func IsAuthenticated(ctx context.Context) bool { return app.IsAuthenticated(ctx) }

// IsStaff returns true if the AuthChecker in context reports staff status.
// Wrapper around [app.IsStaff].
func IsStaff(ctx context.Context) bool { return app.IsStaff(ctx) }

// IsAdmin returns true if the AuthChecker in context reports admin status.
// Wrapper around [app.IsAdmin].
func IsAdmin(ctx context.Context) bool { return app.IsAdmin(ctx) }

// WithContextValue stores a value in the context under the given key. Wrapper
// around [app.WithContextValue].
func WithContextValue(ctx context.Context, key, val any) context.Context {
	return app.WithContextValue(ctx, key, val)
}

// ContextValue retrieves a typed value from the context. Wrapper around
// [app.ContextValue].
func ContextValue[T any](ctx context.Context, key any) (T, bool) {
	return app.ContextValue[T](ctx, key)
}
