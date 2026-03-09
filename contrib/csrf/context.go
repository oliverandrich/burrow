// Package csrf provides CSRF protection as a burrow contrib app.
// It wraps gorilla/csrf and provides context helpers for reading
// the CSRF token in templates and handlers.
//
// The middleware stores the token in the request context. Use [Token]
// to retrieve it in handlers, or the "csrfToken" template function
// in HTML templates.
package csrf

import "context"

// ctxKeyCSRFToken is the context key for the CSRF token.
type ctxKeyCSRFToken struct{}

// WithToken stores a CSRF token in the context.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFToken{}, token)
}

// Token retrieves the CSRF token from the context.
func Token(ctx context.Context) string {
	if token, ok := ctx.Value(ctxKeyCSRFToken{}).(string); ok {
		return token
	}
	return ""
}
