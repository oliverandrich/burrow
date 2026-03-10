# Changelog

All notable changes to Burrow are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Unreleased

### Breaking Changes

- **Template engine migration**: Replaced [Templ](https://templ.guide/) with Go's standard `html/template`. All `templ.Component` types are replaced by `template.HTML`. Apps now contribute templates via `HasTemplates`, static functions via `HasFuncMap`, and request-scoped functions via `HasRequestFuncMap`.
- **LayoutFunc signature changed**: `func(title string, content templ.Component) templ.Component` is now `func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error`.
- **NavItem.Icon type changed**: `templ.Component` is now `template.HTML`.
- **`burrow.Render()` replaced by `burrow.RenderTemplate()`**: Handlers now call `RenderTemplate(w, r, statusCode, "app/template", data)` instead of `Render(w, r, statusCode, component)`.

### Added

- `contrib/secure` — security response headers middleware (X-Content-Type-Options, X-Frame-Options, Referrer-Policy, HSTS, CSP, Permissions-Policy, COOP) using [unrolled/secure](https://github.com/unrolled/secure).
- `contrib/htmx` — dedicated contrib app with request detection and response helpers, inspired by django-htmx.
- `contrib/jobs` — in-process SQLite-backed job queue with worker pool, retry logic, and admin UI via ModelAdmin.
- `contrib/uploads` — pluggable file upload storage with local filesystem backend and content-hashed serving.
- `contrib/ratelimit` — per-client rate limiting middleware using token bucket algorithm.
- `contrib/authmail` — pluggable email renderer interface with SMTP implementation (`authmail/smtpmail`).
- `contrib/admin/modeladmin` — generic Django-style CRUD admin with list fields, filters, search, row actions, and i18n.
- `HasShutdown` interface for graceful app cleanup (called in reverse registration order).
- `HasTemplates` interface for apps to contribute `.html` template files.
- `HasFuncMap` interface for apps to contribute static template functions.
- `HasRequestFuncMap` interface for apps to contribute request-scoped template functions.
- Auto-sorting of apps by `HasDependencies` declarations in `NewServer`.
- Form binding with validation via `burrow.Bind()` and `burrow.Validate()`.
- i18n-aware validation error translation via `i18n.TranslateValidationErrors()`.
- Graceful restart via SIGHUP using tableflip.
- TLS/ACME support for standalone deployment.
- Dark mode toggle with theme persistence in the Bootstrap app.
- Offset and cursor-based pagination helpers.
- Flash messages via `contrib/messages` with Bootstrap alert templates.

### Changed

- Migrated all contrib apps from Templ to `html/template`.
- Bootstrap Icons are now inline SVG functions returning `template.HTML` instead of `templ.Component`.
- Admin panel uses HTMX with explicit `hx-get`/`hx-target` instead of `hx-boost`.
- Replaced `Registry.Bootstrap()` with `Registry.RegisterAll()`.
- Options pattern adopted for `auth.New()`, `admin.New()`, `jobs.New()`, `uploads.New()`, and `ratelimit.New()`.
- Unified auth context helpers to `context.Context` pattern.
- SQLite connection defaults aligned with [dj-lite](https://github.com/adamghill/dj-lite/) recommendations: added `busy_timeout=5000`, `temp_store=MEMORY`, `mmap_size=128MB`, `journal_size_limit=26MB`, `cache_size=2000`, and `IMMEDIATE` transaction mode for better production concurrency.
- Rewritten project description (README and docs index) with clear positioning, target audience, and API-only disclaimer.
- Simplified Quick Start to a minimal app without layout, session, or healthcheck.
- New guides: [Database](guide/database.md), [TLS](guide/tls.md), [Routing](guide/routing.md), [Contributing](contributing.md).
- New reference page: [Core Functions](reference/core-functions.md) documenting all exported functions and types.
- Added code examples to every interface in the [Core Interfaces](reference/interfaces.md) reference.
- Added dependency declarations (`Depends on:`) to all contrib app docs.
- Reorganized guide sidebar into Core, Templates & UI, Advanced, and Deployment groups.
- Added copyright footer to documentation site.
- Expanded auth [Renderer](contrib/auth.md#renderer) and [Auth Layout](contrib/auth.md#auth-layout) documentation with usage examples.
- **Default email renderer moved** from `authmail/smtpmail/templates` to `auth.DefaultEmailRenderer()`. The `authmail` package keeps the `Renderer` interface only.
- Auth app now declares `i18n` as a dependency alongside `session`.
- Added `i18n.NewTestApp()` helper for creating a minimal i18n setup in tests.

### Fixed

- Auth emails (verification, invite) are now rendered in the user's locale. Previously, emails were always in English because goroutines used `context.Background()`, losing the request locale.
- Auth pages now render with a minimal layout instead of full app chrome.
- WebAuthn cleanup goroutine uses context-based cancellation.
- `buildManifest` errors are propagated instead of silently discarded.
- `Seed` is called on `Seedable` apps during server bootstrap.
- Fixed broken cross-links and removed redundant content across docs.

## 2026-02-19 — Initial Release

- App-based architecture with `burrow.App` interface and optional interfaces.
- Pure Go SQLite via `modernc.org/sqlite` (no CGO required).
- Per-app SQL migration runner with `_migrations` tracking table.
- Chi v5 router integration with `burrow.HandlerFunc` error-returning handlers.
- Cookie-based sessions via `gorilla/sessions`.
- WebAuthn/passkey authentication with recovery codes.
- CSRF protection via `gorilla/csrf`.
- i18n with Accept-Language detection and `go-i18n` translations.
- Content-hashed static file serving.
- Admin panel coordinator.
- CLI configuration via `urfave/cli` with flag, env var, and TOML support.
- CSS-agnostic layout system.
