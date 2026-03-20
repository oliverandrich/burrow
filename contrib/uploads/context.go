// Package uploads provides file upload storage as a burrow contrib app.
// It offers a pluggable Storage interface with a local filesystem
// implementation and content-hashed filenames for deduplication.
package uploads

import (
	"context"
)

type ctxKeyStorage struct{}
type ctxKeyAllowedTypes struct{}

// WithStorage returns a new context carrying the given Storage.
func WithStorage(ctx context.Context, s Storage) context.Context {
	return context.WithValue(ctx, ctxKeyStorage{}, s)
}

// GetStorage returns the Storage from the context, or nil.
func GetStorage(ctx context.Context) Storage {
	s, _ := ctx.Value(ctxKeyStorage{}).(Storage)
	return s
}

// StorageFromContext is a deprecated alias for [GetStorage].
//
//go:fix inline
func StorageFromContext(ctx context.Context) Storage {
	return GetStorage(ctx)
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

// URL returns the public URL for a storage key. If no Storage is in the
// context, it returns the key as-is (safe fallback for templates).
func URL(ctx context.Context, key string) string {
	s := GetStorage(ctx)
	if s == nil {
		return key
	}
	return s.URL(key)
}
