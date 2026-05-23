package burrow

import (
	"context"

	"github.com/oliverandrich/burrow/server"
	"github.com/oliverandrich/den"
)

// NewServer creates a Server and registers the given apps. Apps are
// auto-sorted to satisfy [HasDependencies] declarations. Wrapper around
// [server.New].
func NewServer(apps ...App) *Server { return server.New(apps...) }

// OpenDB opens a database from a URL-style DSN. Wrapper around
// [server.OpenDB].
func OpenDB(ctx context.Context, dsn string, opts ...den.Option) (*den.DB, error) {
	return server.OpenDB(ctx, dsn, opts...)
}
