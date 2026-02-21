// Package staticfiles provides static file serving as a burrow contrib app.
// It computes content hashes at startup and serves files under hashed URLs
// for cache busting, similar to Django's ManifestStaticFilesStorage.
package staticfiles

import (
	"context"
)

type ctxKeyApp struct{}

// URL returns the hashed URL for a static file. It reads the App from context
// (injected by the middleware) to resolve the content-hashed path.
// If no App is in context, it returns the name as-is (safe fallback).
func URL(ctx context.Context, name string) string {
	a, ok := ctx.Value(ctxKeyApp{}).(*App)
	if !ok || a == nil {
		return name
	}
	if hashed, exists := a.manifest[name]; exists {
		return a.prefix + hashed
	}
	return a.prefix + name
}
