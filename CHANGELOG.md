# Changelog

All notable changes to Burrow are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Unreleased

### Breaking Changes

- **Forms: `Cleanable.Clean()` signature changed** — `Clean() error` is now `Clean(ctx context.Context) error`; existing implementations must add the `context.Context` parameter
- **Auth: `isAdminEditSelf` and `isAdminEditLastAdmin` template functions removed** — the custom user admin detail page is replaced by generic ModelAdmin forms; `IsAdminEditSelf()`, `IsAdminEditLastAdmin()`, and `emailValue` are no longer available
- **Auth: `User` model form tags changed** — `Email`, `Username`, `IsActive`, `Name`, `Bio`, and `Role` now have explicit `form` tags instead of `form:"-"`; code that relied on these fields being excluded from forms must use `WithExclude` or `WithReadOnly`
- **Pagination: cursor-based pagination removed** — `ApplyCursor()`, `TrimCursorResults()`, `CursorResult()`, `PageRequest.Cursor`, `PageResult.NextCursor`, and `PageResult.PrevCursor` have been removed; use `ApplyOffset()` + `OffsetResult()` for all pagination
- **`Render()` renamed** — `RenderTemplate()` is now `Render()`; the old `Render(w, r, code, template.HTML)` raw-HTML wrapper has been removed (use `HTML()` directly); `RenderTemplate()` remains as a deprecated shim with `//go:fix inline` for automatic migration via `go fix` (Go 1.26+)

### Added

- **ModelAdmin: `fmt.Stringer` support in list views** — when a list field value implements `fmt.Stringer` (e.g. an eager-loaded FK relation), the list view renders `String()` instead of the raw struct
- **ModelAdmin: computed list columns (`ListDisplay`)** — new `ListDisplay map[string]func(T) template.HTML` field on `ModelAdmin[T]` allows custom computed columns in list views that are not direct struct fields
- **`auth.User` implements `fmt.Stringer`** — returns the user's `Name` if set, otherwise falls back to `Username`
- **ModelAdmin: read-only fields (`ReadOnlyFields`)** — new `ReadOnlyFields []string` field on `ModelAdmin[T]` renders specified fields as plain text in create/edit forms; values are preserved from the model instance and cannot be modified by the user
- **ModelAdmin: CSV/JSON export (`CanExport`)** — new `CanExport bool` field on `ModelAdmin[T]` adds export dropdown to list views; exports respect current filters, search, and sorting; downloads as `{slug}-{date}.csv` or `.json`
- **ModelAdmin: delete confirmation page with cascade impact** — when `CanDelete` is true, the delete button now navigates to a dedicated confirmation page instead of using an inline `hx-confirm` dialog; ON DELETE CASCADE foreign keys are auto-detected at boot time via SQLite PRAGMAs, and affected row counts are shown on the confirmation page
- **Forms: `WithReadOnly` option** — new `forms.WithReadOnly[T](fields...)` option marks fields as read-only; read-only fields skip validation and restore their original value after bind
- **Forms: `Clean(ctx)` and `WithCleanFunc`** — `Cleanable.Clean()` now receives a `context.Context` for request-scoped data; new `WithCleanFunc[T]` option adds closure-based cross-field validation for logic that needs external dependencies (DB, repo); both run after per-field validation and errors are merged
- **ModelAdmin: `FormOptions`** — new `FormOptions []forms.Option[T]` field on `ModelAdmin[T]` allows passing additional form options (e.g. `WithCleanFunc`) to create/edit forms
- **Auth: user admin uses generic ModelAdmin** — user detail/edit/delete now use ModelAdmin with `WithCleanFunc` for last-admin demotion protection; custom handlers, `isAdminEditSelf`/`isAdminEditLastAdmin` context helpers, and custom template removed
- **Auth: `authtest` package** — new `contrib/auth/authtest` package provides `NewDB` (in-memory DB with auth migrations) and `CreateUser` (with functional options) for tests that depend on the auth app
- **ModelAdmin: bulk actions (`BulkActions`)** — new `BulkActions []BulkAction` field on `ModelAdmin[T]` enables multi-select checkbox operations in list views; includes `DeleteBulkAction[T]()` convenience constructor for bulk delete; toolbar with action dropdown, select-all checkbox, and JS confirm for destructive actions

### Removed

- **Auth: soft-delete removed from all models** — `deleted_at` columns and `bun:",soft_delete"` tags removed from User, Credential, RecoveryCode, EmailVerificationToken, and Invite; all deletes are now permanent; includes migration `004_drop_soft_delete`

### Fixed

- **SQLite PRAGMAs now applied per-connection** — per-connection PRAGMAs (foreign_keys, busy_timeout, synchronous, etc.) are now set via `_pragma` DSN parameters instead of one-shot `db.Exec()` calls, ensuring they are active on every connection in the pool; this fixes `ON DELETE CASCADE` not firing when a different pool connection handled the DELETE
- **ModelAdmin: ambiguous column name with relations** — `getItem` and list queries now qualify column names with the table alias to prevent SQLite "ambiguous column name" errors when eager-loading relations that share column names (e.g. `id`, `created_at`)
- **Auth: invites FK missing ON DELETE** — `used_by` and `created_by` in the `invites` table now have `ON DELETE SET NULL`, preventing user deletion from failing due to FK constraint violations (migration `005_invites_fk_set_null`)
- **Auth: swallowed errors in handlers** — `SetUserRole` (first-user admin promotion), `DeleteEmailVerificationToken`, and `DeleteUserEmailVerificationTokens` errors are now logged via `slog.Error` instead of silently discarded

## 0.4.1 — 2026-03-13

### Added

- **`--ratelimit-max-clients` flag** — caps the number of tracked client buckets (default 10,000) to prevent memory exhaustion; when the limit is reached, the oldest entry is evicted
- **Periodic cleanup of expired email verification tokens** — the auth background cleanup now also deletes expired tokens, preventing unbounded accumulation
- **Ratelimit configuration validation** — `Configure()` now rejects zero/negative values for rate, burst, and cleanup interval, and negative max-clients

### Changed

- Add secret key configuration section to deployment guide covering `SESSION_HASH_KEY`, `SESSION_BLOCK_KEY`, and `CSRF_KEY`

### Security

- **Fix rate limit bypass via X-Forwarded-For spoofing** — ratelimit now uses only `X-Real-IP` when `--ratelimit-trust-proxy` is enabled; `X-Forwarded-For` is no longer used because its multi-value format is trivially spoofed
- **Fix timing attack on recovery code validation** — `ValidateAndUseRecoveryCode` now always iterates all codes to prevent timing side-channel that revealed code position via early return
- **Fix user enumeration via registration endpoint** — `RegisterBegin` now returns HTTP 200 for both new and existing accounts, preventing attackers from probing which usernames or emails are registered
- **Verify WebAuthn sign count to detect cloned credentials** — login now rejects authentication attempts where the sign count does not increase, indicating a potentially cloned authenticator; software authenticators (always 0) are unaffected
- **Fix invite token race condition** — `MarkInviteUsed` now uses `WHERE used_at IS NULL` to ensure only the first concurrent registration consumes an invite; subsequent attempts fail atomically
- **Hide CSRF failure reasons from clients** — custom error handler returns generic "Forbidden" instead of detailed failure reasons; failure details are logged server-side via slog

## 0.4.0 — 2026-03-13

### Breaking Changes

- **`/healthz` endpoint removed** — replaced by `/healthz/live` (liveness) and `/healthz/ready` (readiness). Update load balancer and monitoring configurations accordingly.
- **`LayoutFunc` removed**: The `LayoutFunc` type and `SetLayout(fn LayoutFunc)` are gone. Layouts are now template name strings: call `srv.SetLayout("myapp/layout")` with a template name, and `RenderTemplate` wraps content automatically. Layout templates receive the rendered fragment as `.Content` and access dynamic data (navigation, user, etc.) via template functions instead of Go code passing data maps. See the [Layouts & Rendering guide](guide/layouts.md).
- **ModelAdmin migrated to `forms` package**: `Renderer[T].Form()` now takes `[]forms.BoundField` instead of `[]FormField` and `*ValidationError` (errors are on each `BoundField`). `FormField`, `Choice`, `AutoFields`, `PopulateFromForm` removed from modeladmin — use `forms.FromModel`, `forms.BoundField`, `forms.Choice` instead. `ChoicesFunc` and `FilterDef.Choices` now use `forms.Choice`.

### Added

- **`ReadinessChecker` interface** — apps can implement `ReadinessCheck(ctx) error` to contribute to the readiness probe
- **Healthcheck liveness and readiness endpoints** — `/healthz/live` (always 200) and `/healthz/ready` (database + all `ReadinessChecker` apps, 200/503 with details)
- **`NavLink` type and `navLinks` template function** — core framework now provides filtered, template-ready navigation with automatic `AuthOnly`/`AdminOnly` filtering and active-state highlighting. Apps only need to implement `HasNavItems`; no manual filtering or `RequestFuncMap` required for navigation.
- **`AuthChecker` context type** — allows core framework to read auth state without importing `contrib/auth`. The `auth` middleware injects it automatically. Custom auth systems can use `burrow.WithAuthChecker()`.
- **`forms` package** — generic, type-safe form handling with `Form[T]`, `BoundField`, `Choice`, struct tag-driven field extraction (`form`, `verbose_name`, `widget`, `choices`, `help_text`, `validate`), request binding via `burrow.Bind`, cross-field validation via `Cleanable` interface, and dynamic choices via `ChoiceProvider`/`WithChoicesFunc`
- `forms.WithExclude` option — excludes fields by Go struct field name from form rendering
- **`contrib/htmx` config template** — `htmx/config` template configures htmx to swap `422 Unprocessable Entity` responses, enabling consistent status codes for form validation errors across htmx and non-htmx requests
- **Reusable asset templates** — `bootstrap/css`, `bootstrap/js`, and `htmx/js` templates for including CSS/JS assets in layouts without hardcoding `staticURL` calls

## 0.3.0 — 2026-03-11

### Breaking Changes

- **`contrib/jobs` `Repository.Fail()` signature changed**: Added `baseDelay time.Duration` parameter for configurable exponential backoff. Default backoff changed from `2^attempts` seconds (2s, 4s, 8s) to `baseDelay * 2^(attempts-1)` with a 30s default (30s, 1m, 2m, 4m).

### Added

- `csrfField` template function — renders a complete `<input type="hidden">` element with the CSRF token, reducing form boilerplate
- `FieldChoices` on `ModelAdmin` — dynamic `<select>` dropdowns for foreign key fields, loaded from the database at request time
- `--jobs-retry-base-delay` flag — configurable base delay for exponential retry backoff (default: 30s)
- `RenderTemplate()` now applies the layout for `hx-boost` requests (`HX-Boosted` header), fixing navbar disappearing on boosted navigation
- `RequireAuth()` middleware uses `Referer` header for post-login redirect on non-GET requests, fixing 405 errors on POST-protected routes
- `slog.Info` logging for applied database migrations

### Changed

- Restructure deployment guide: add intro with deployment options table, reorder sections (bare metal → systemd → Docker → graceful restart)
- Add logging guide explaining slog configuration responsibility and handler options
- Add custom LayoutFunc example to layouts guide
- Add template function availability note to i18n guide
- Add working example reference (notes app) to FTS5 guide
- Link to urfave/cli flag documentation in configuration guide
- Add all missing contrib app flags (Jobs, Uploads, Rate Limit, Secure, SMTP Mail) to configuration reference
- Add FieldChoices documentation to admin contrib docs
- Comprehensive tutorial review and fixes (parts 1–7): explicit file paths, missing imports, `go mod tidy` in run sections
- Add missing setup steps (`go mod init`, `go get`, `go mod tidy`) to quick start examples in README, index, installation, and quickstart pages
- Split notes example app into standard file layout (`models.go`, `repository.go`, `handlers.go`, `app.go`)
- Add missing `HasJobs` interface to all interface tables in docs
- Remove internal "Updating Icons" section from Bootstrap Icons docs

### Fixed

- `RenderTemplate()` skipped layout for `hx-boost` requests, causing navbar to disappear on boosted navigation
- `RequireAuth()` redirected to POST URL after login, causing 405 Method Not Allowed
- Tutorial Part 5 showed `<nil>` in navbar — `User.Email` (`*string`) replaced with `User.Username` (`string`)
- Tutorial Part 6 used manual route registration with wrong HTTP methods, causing 405 on admin delete
- Replace broken Codeberg URLs with GitHub URLs across documentation
- Convert and resize cover image from PNG (2.9MB) to JPEG (352KB)

## 0.2.0 — 2026-03-10

### Breaking Changes

- **Template engine migration**: Replaced [Templ](https://templ.guide/) with Go's standard `html/template`. All `templ.Component` types are replaced by `template.HTML`. Apps now contribute templates via `HasTemplates`, static functions via `HasFuncMap`, and request-scoped functions via `HasRequestFuncMap`.
- **LayoutFunc signature changed**: `func(title string, content templ.Component) templ.Component` is now `func(w http.ResponseWriter, r *http.Request, code int, content template.HTML, data map[string]any) error`.
- **NavItem.Icon type changed**: `templ.Component` is now `template.HTML`.
- **`burrow.Render()` replaced by `burrow.RenderTemplate()`**: Handlers now call `RenderTemplate(w, r, statusCode, "app/template", data)` instead of `Render(w, r, statusCode, component)`.
- **`contrib/jobs` handler signature changed**: `HandlerFunc` changed from `func(ctx context.Context, job *Job) error` to `func(ctx context.Context, payload []byte) error`. Handlers now receive raw JSON payload bytes instead of the full `*Job` struct.
- **`contrib/jobs` `Enqueue`/`EnqueueAt` return type changed**: Now returns `(string, error)` instead of `(*Job, error)`. The string is an opaque job ID.
- **`contrib/auth` email delivery via jobs**: Auth emails are delivered via the job queue when available, with automatic fallback to direct sending. Register a `burrow.Queue` implementation (e.g., `jobs.New()`) to enable retries and persistence.

### Added

- `burrow.Queue` interface — core abstraction for job queues with `Handle()`, `Enqueue()`, `EnqueueAt()`, and `Dequeue()` methods.
- `burrow.HasJobs` interface — apps implement `RegisterJobs(q Queue)` to declare job handlers, discovered automatically by the queue during `Configure()`.
- `burrow.JobHandlerFunc`, `burrow.JobOption`, `burrow.JobConfig`, `burrow.WithMaxRetries()` — core job handler types and options.
- `contrib/jobs.Dequeue()` — cancel a pending job by its ID.
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
- i18n-aware validation error translation via `ve.Translate(ctx, i18n.TData)`.
- Graceful restart via SIGHUP using tableflip.
- TLS/ACME support for standalone deployment.
- Dark mode toggle with theme persistence in the Bootstrap app.
- Offset and cursor-based pagination helpers.
- Flash messages via `contrib/messages` with Bootstrap alert templates.

### Changed

- ModelAdmin search auto-detects FTS5 tables at boot time. If a `{tablename}_fts` virtual table exists, search uses FTS5 `MATCH` instead of `LIKE`, with automatic fallback on syntax errors.
- Auth email delivery (verification, invite) now uses the job queue with retries and persistence instead of fire-and-forget goroutines.
- `contrib/jobs` implements `burrow.Queue` interface; apps register handlers via `HasJobs` instead of manual `Registry.Get()` lookups.
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
- New guide: [Full-Text Search](guide/fts5.md) covering FTS5 virtual tables, triggers, sanitization, highlighting, and performance.
- Added copyright footer to documentation site.
- New guide: [Coming from Django](getting-started/coming-from-django.md) mapping Django concepts to Burrow equivalents with side-by-side code examples.
- Added request lifecycle diagram to the [Routing](guide/routing.md) guide.
- Added "Why urfave/cli?" section to [Server & Registry](reference/server.md) reference.
- New guide: [Testing](guide/testing.md) covering test helpers and patterns.
- New pages: [Examples & Tutorial](getting-started/examples.md) overview, seven-part [Tutorial](tutorial/index.md).
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

## 0.1.0 — 2026-02-19

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
