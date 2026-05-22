# app — App interface, capability interfaces, context helpers

The `burrow/app` package holds the App-domain types every burrow app touches: the `App` interface itself, every optional capability interface (`HasRoutes`, `HasMiddleware`, `HasMigrations`, `Configurable`, …), the `AppConfig` struct passed into `Configure`, navigation primitives (`NavItem`, `NavLink`), the framework `Config` plus its sub-configs, and the context helpers used by middleware and templates.

The burrow root re-exports the everyday names as type aliases — `burrow.App`, `burrow.AppConfig`, `burrow.HasRoutes`, `burrow.Configurable` etc. — and the helpers as wrapper functions, so most application code can stay on `burrow.X`. Reach into `burrow/app` directly when you want one of the less-common context helpers or sub-config types in a file that already imports the package.

Full signatures live on [pkg.go.dev](https://pkg.go.dev/github.com/oliverandrich/burrow/app). This page is a symbol index.

## Required interface

| Symbol | Notes |
|---|---|
| `App` | Every app implements `Name() string`. Alias of `registry.App`. |

## Capability interfaces

Apps gain features by implementing any subset of these. Each is optional; pick what your app needs.

| Symbol | Triggers |
|---|---|
| `Configurable` | `Configure(*AppConfig, *cli.Command) error` runs at boot |
| `PostConfigurable` | Second-pass configure after every Configure has run |
| `HasDependencies` | Auto-sorted boot order; missing deps panic at startup (lives in `burrow/registry`) |
| `HasFlags` | CLI flags merged into the framework's flag set |
| `HasCLICommands` | App ships its own `*cli.Command` subcommands |
| `HasRoutes` | App registers HTTP routes on the chi router |
| `HasMiddleware` | App contributes global HTTP middleware |
| `HasNavItems` | App contributes main-nav entries |
| `HasAdmin` | App contributes admin routes + nav |
| `AdminAuth` | App provides admin-panel auth middleware (`contrib/auth` implements this) |
| `HasMigrations` | App ships versioned, run-once Den migrations |
| `HasDocuments` | App registers Den document types at boot |
| `HasStaticFiles` | App contributes embedded static assets |
| `HasTranslations` | App contributes i18n TOML files |
| `HasTemplates` | App contributes HTML templates |
| `HasFuncMap` | App contributes static template functions |
| `HasRequestFuncMap` | App contributes per-request template functions |
| `HasShutdown` | App's `Shutdown(ctx)` runs during graceful shutdown |
| `ReadinessChecker` | App contributes to the `/healthz/ready` probe |
| `Startable` | App's `Start(*Server)` runs after boot completes (lives in `burrow/server`) |

## Data types

| Symbol | Notes |
|---|---|
| `AppConfig` | DB, Registry, Config, WithLocale — passed into every Configure |
| `NavItem` | Label, URL, Icon, Position, AuthOnly/StaffOnly/AdminOnly |
| `NavLink` | Template-ready nav item with pre-computed active state |
| `NamedMigration` | Version + `migrate.Migration` |
| `TemplateExecutor` | Function type for executing a named template |
| `AuthChecker` | Auth-state closure bag — `IsAuthenticated` / `IsStaff` / `IsAdmin` |

## Framework configuration

| Symbol | Notes |
|---|---|
| `Config` | Top-level framework config — bundles `ServerConfig`, `DatabaseConfig`, `StorageConfig`, `TLSConfig` |
| `ServerConfig` / `DatabaseConfig` / `StorageConfig` / `TLSConfig` | Sub-configs read from CLI/ENV/TOML at boot |
| `Config.IsHTTPS` / `Config.ResolveBaseURL` / `Config.ValidateTLS` / `Config.ResolvedTLSMode` | Helpers used during boot |
| `NewConfig(cmd)` | Builds Config from a parsed `*cli.Command` |
| `CoreFlags(configSource)` | The framework's own CLI flag set |
| `FlagSources(configSource, envVar, tomlKey)` | Standard env+TOML chain for contrib-app flags |
| `IsLocalhost(host)` | Reports whether the host string is a localhost address |

## Context helpers

| Symbol | Notes |
|---|---|
| `WithLayout` / `Layout` | Layout template name in/out of context |
| `WithNavItems` / `NavItems` | Filtered nav items in/out of context |
| `WithTemplateExecutor` / `TemplateExec` | Template executor in/out of context |
| `WithAuthChecker` | Auth-state closure bag into context (contrib/auth's middleware sets this) |
| `WithRequestPath` / `RequestPath` | Request path in/out of context (for nav-link highlighting outside HTTP) |
| `IsAuthenticated` / `IsStaff` / `IsAdmin` | Read auth state from context — returns false when no AuthChecker is set |
| `WithContextValue` / `ContextValue[T]` | Generic typed context bag for app-specific values |

See [Inter-App Communication](../guide/inter-app-communication.md) for usage patterns and [Core Interfaces](interfaces.md) for the long-form descriptions of each capability interface.
