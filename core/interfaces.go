package core

import (
	"context"
	"io/fs"

	"github.com/labstack/echo/v5"
	"github.com/urfave/cli/v3"
)

// Migratable is implemented by apps that provide database migrations.
type Migratable interface {
	MigrationFS() fs.FS
}

// HasMiddleware is implemented by apps that contribute Echo middleware.
type HasMiddleware interface {
	Middleware() []echo.MiddlewareFunc
}

// HasNavItems is implemented by apps that contribute navigation items.
type HasNavItems interface {
	NavItems() []NavItem
}

// Configurable is implemented by apps that define CLI flags
// and need to read their configuration from the CLI command.
type Configurable interface {
	Flags() []cli.Flag
	Configure(cmd *cli.Command) error
}

// HasCLICommands is implemented by apps that contribute subcommands.
type HasCLICommands interface {
	CLICommands() []*cli.Command
}

// HasRoutes is implemented by apps that register HTTP routes.
type HasRoutes interface {
	Routes(e *echo.Echo)
}

// Seedable is implemented by apps that can seed the database
// with initial data.
type Seedable interface {
	Seed(ctx context.Context) error
}
