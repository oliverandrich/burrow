// Package uploads provides file upload storage as a burrow contrib app.
// It offers a pluggable Store interface with a local filesystem
// implementation and content-hashed filenames for deduplication.
package uploads

import (
	"context"
)

type ctxKeyStorage struct{}
type ctxKeyAllowedTypes struct{}

// WithStorage returns a new context carrying the given Store.
func WithStorage(ctx context.Context, s Store) context.Context {
	return context.WithValue(ctx, ctxKeyStorage{}, s)
}

// Storage returns the Store from the context, or nil.
func Storage(ctx context.Context) Store {
	s, _ := ctx.Value(ctxKeyStorage{}).(Store)
	return s
}

// GetStorage is a deprecated alias for [Storage].
//
//go:fix inline
func GetStorage(ctx context.Context) Store {
	return Storage(ctx)
}

// StorageFromContext is a deprecated alias for [Storage].
//
//go:fix inline
func StorageFromContext(ctx context.Context) Store {
	return Storage(ctx)
}

// withAllowedTypes returns a new context carrying the default allowed MIME types.
func withAllowedTypes(ctx context.Context, types []string) context.Context {
	return context.WithValue(ctx, ctxKeyAllowedTypes{}, types)
}

// allowedTypesFromContext returns the default allowed MIME types, or nil.
func allowedTypesFromContext(ctx context.Context) []string {
	types, _ := ctx.Value(ctxKeyAllowedTypes{}).([]string)
	return types
}

// URL returns the public URL for a storage key. If no Store is in the
// context, it returns the key as-is (safe fallback for templates).
func URL(ctx context.Context, key string) string {
	s := Storage(ctx)
	if s == nil {
		return key
	}
	return s.URL(key)
}
