# Changelog

All notable changes to Burrow are documented here. The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Unreleased

### Added

- **`mise run clean` and `mise run clean-all` tasks** in burrow itself and the project scaffold. `clean` removes build artifacts and generated files (coverage outputs, `bin/`, `tmp/`, `site/`, docs build output, `.tailwind/` source dirs, generated example static bundles); `clean-all` additionally wipes every gitignored `data/` runtime dir and every `*.db*` SQLite file anywhere in the tree. `clean-all` depends on `clean` so there's no copy-paste drift.
- **`docs/getting-started/scaffold.md`** documents the output of `burrow new` — annotated file tree, full mise-task reference, dev/release loops, and what the scaffold deliberately omits. The Quick Start tip box, the index Minimal Example, and the Tooling CLI section now cross-link to it; `tooling.md` keeps the CLI-flag reference, the new page covers the generated project.

### Fixed

- **`burrow dev` no longer kills a freshly-restarted child on a stale exit observation.** When the app exited (compile error, panic) and the user saved a file before the supervisor's exit-monitor goroutine had finished its cleanup, the cleanup would cancel the brand-new child's context, killing it. The dev server recovered on the next save but the experience looked like "save → crash → nothing rebuilds." The exit handler now refuses to act on observations from a previous generation (`internal/dev/supervisor.go`).

## 0.25.2 — 2026-05-24

### Changed

- **Scaffold + examples: enable `@tailwindcss/typography` + `@tailwindcss/forms`** via `@plugin` directives in `tailwind.css`. Both plugins ship bundled with the standalone Tailwind CLI (no JS toolchain or `node_modules` involved). Tree-shaken when unused — zero CSS cost for projects that don't touch `.prose` or form controls. `@tailwindcss/aspect-ratio` is also bundled but intentionally left off: Tailwind v4's built-in `aspect-*` utilities supersede it.
- **`docs/guide/releases.md` refreshed.** Removed stale mentions of `just`, links to the archived `go-burrow-template` repo, and an inline Homebrew Cask recipe the scaffold never shipped. Inline goreleaser + CI snippets replaced by pointers to the scaffold's own `.goreleaser.yaml` and `.github/workflows/ci.yml` so the page stops drifting from the source of truth.
- **Scaffold: release job moved into `ci.yml` as a tag-only stage** (`if: startsWith(github.ref, 'refs/tags/v')`, `needs: [check, zizmor]`). Standalone `release.yml` is removed. Matches burrow's own workflow shape and ensures release builds run against the same CI gate as `main` pushes.
- **Scaffold: goreleaser builds FreeBSD and OpenBSD binaries** for `amd64` and `arm64`, alongside the existing Linux / macOS / Windows targets. Possible because the scaffold is consistently `CGO_ENABLED=0`. (NetBSD was evaluated but `modernc.org/sqlite` upstream has a compile bug for `netbsd/amd64` and ships no source for `netbsd/arm64`; left out until that's fixed.)

## 0.25.1 — 2026-05-24

### Fixed

- **`contrib/selfupdate` archive extraction** now picks tar.gz vs zip from the asset's actual filename instead of `runtime.GOOS`. The previous heuristic assumed the burrow-scaffold convention (zip on darwin/windows) and broke `WithAssetMatcher` users whose archives don't follow it — `goreleaser_Darwin_all.tar.gz`, for example, was opened as a zip and failed with "not a valid zip file". Burrow-scaffold-shaped releases are unaffected.

## 0.25.0 — 2026-05-24

> **Upgrading from v0.24?** See the [v0.25 migration guide](migration/v0.25.md).

### Added

- **`contrib/selfupdate` — in-app `update` command.** Adds an `update` sub-command to burrow binaries that fetches the latest release from GitHub, Codeberg, or any Forgejo instance, verifies the archive against the release's `checksums.txt` SHA256, extracts the binary, and atomically replaces the running executable via [`minio/selfupdate`](https://github.com/minio/selfupdate). Anonymous public-API access only — no tokens. Defaults match the scaffold's `.goreleaser.yaml` asset layout; override via `WithAssetPattern` (template) or `WithAssetMatcher` (predicate). `WithBinaryName` covers the case where the archive's binary doesn't match the repo name. Verifies the SHA256 of the archive **before** parsing it, skips tar symlink entries (won't get tricked into writing a 0-byte binary), enforces a per-binary `flock` against concurrent updates, caps download sizes, escapes URL path segments, and surfaces rate-limit responses with the reset timestamp. Linux is the tested target; macOS works as a side effect, Windows is best-effort (no flock).
- **`burrow dev` — integrated dev server.** New CLI sub-command runs the app via `go run` under an `fsnotify`-backed file watcher. On each debounced file change it sequentially rebuilds the Tailwind CSS bundle (via `burrow tailwind`, when configured) and restarts the app — the Go rebuild re-embeds the fresh CSS via `//go:embed`. Replaces the Air-based loop in the project scaffold; auto-discovers the entry-point and Tailwind paths from the conventional burrow layout. Detects app crashes (build errors, panics, port-in-use) and logs them. Linux is the tested target; macOS works as a side effect, Windows is best-effort.
- **`burrow dev` auto-creates `.env`** with `SESSION_HASH_KEY` and `CSRF_KEY` (mode 0600) on first run when the file is missing; existing files are read verbatim via [godotenv](https://github.com/joho/godotenv) and never patched. Shell env vars win over `.env` values (standard 12-factor precedence). Pass `--no-env-file` to skip both.

### Changed

- **Project scaffold no longer ships `.air.toml` or pins `air`** in `.mise.toml`. `mise run dev` becomes `go tool burrow dev`; `mise run setup` now calls `go tool burrow dev --init-env` to materialise `.env`. `.gitignore` tracks `.env` instead of `.dev-keys`.
- **Den bumped to v0.16.1.** Bugfix release: top-level `Or()` no longer swallows sibling AND-predicates (SQL precedence fix); SQLite `time.Time`, `[]byte`, and `json.RawMessage` comparison operators now match the JSON storage encoding.

## 0.24.0 — 2026-05-23

> **Upgrading from v0.23?** See the [v0.24 migration guide](migration/v0.24.md).

### Breaking Changes

- **`Registry` split into storage and lifecycle.** Storage (`Add`, `Get`, `Apps`, name lookup) moves into a new `github.com/oliverandrich/burrow/registry` package as free functions; the lifecycle orchestration (`ConfigureAll`, `Configure`, `RegisterMiddleware`, `RegisterRoutes`, `RunMigrations`, `RegisterDocuments`, `AllFlags`, `AllNavItems`, `AllAdminNavItems`, `AllCLICommands`, `Shutdown`) is no longer exported — `Server` orchestrates them internally. Construct registries with `registry.New()`, add apps with `registry.Add(reg, app)`. `burrow.App`, `burrow.Registry`, and `burrow.HasDependencies` remain available as type aliases pointing at the new package, so existing `burrow.App` references compile unchanged.

### Added

- **Typed registry lookup.** `registry.Get[T](reg)` returns `(T, bool)` for Optional-Service lookups (graceful degradation when the provider may be missing). `registry.MustGet[T](reg)` returns `T` and panics on a missing or ambiguous type — the idiomatic shape for Hard-Dependency apps that declare the provider in `Dependencies()`. `registry.GetByName(reg, name)` and `registry.MustGetByName(reg, name)` cover the string-keyed Soft-Discovery case (used by `contrib/admin` to discover its `AdminAuth` provider).

### Changed

- **`docs/guide/inter-app-communication.md` documents three lookup patterns** — Hard-Dependency, Optional-Service, Soft-Discovery — with the registry function each one uses and the failure mode it carries.
- **`sse.BrokerFromRegistry` simplified to `registry.Get[*App]`.** Behaviour unchanged (still returns nil when SSE is not registered); the old name-lookup-plus-type-assert is gone.
- **`contrib/jobs` docs corrected to "Den-backed (SQLite + Postgres)".** Doc comments and READMEs called it "SQLite-backed" — that was true in v0.1 but the repository has been Postgres-aware since Den arrived. Code behaviour unchanged.
- **`burrow` package split into themed public sub-packages.** Implementation moves into `burrow/app` (App interface, capability interfaces, AppConfig, NavItem, Config and sub-configs, context helpers), `burrow/server` (Server, boot sequence, templates, listener, TLS, OpenDB), `burrow/web` (HandlerFunc, HTTPError, Handle, JSON, Render, Bind, Validate), `burrow/tasks` (Queue, Task, Result, JobConfig), `burrow/pagination` (PageRequest, PageURL, OffsetResult). The burrow root retains type aliases and wrapper functions for the everyday API — `burrow.App`, `burrow.AppConfig`, `burrow.HasRoutes`, `burrow.NewServer`, `burrow.Handle`, `burrow.JSON`, `burrow.Render`, `burrow.Bind`, `burrow.Validate`, `burrow.DefineTask`, `burrow.ParsePageRequest` and so on continue to work via aliases. Less-frequent operations are also reachable via direct sub-package import (`tasks.DefineTask[P]`, `pagination.ParsePageRequest`, `app.Config` sub-configs, `web.HandlerFunc`). `Config.resolvedTLSMode` is now exported as `ResolvedTLSMode` because `tls.go` lives in `burrow/server` and calls it across the package boundary. No behaviour changes.
- **Den bumped to v0.16.0.** Internal refactor in Den (its `internal/core` split into public sub-packages); the `den.X` alias surface burrow uses is API-compatible — no changes required.
- **`burrow.go` split into themed root files for readability**, with smoke tests added to the root package. No API change.

## 0.23.0 — 2026-05-21

> **Upgrading from v0.22?** See the [v0.23 migration guide](migration/v0.23.md).

### Breaking Changes

- **Three-tier role model in `contrib/auth`.** `RoleStaff = "staff"` joins `RoleUser` and `RoleAdmin`; admins are implicit staff (`User.IsStaff()` true for both). The `/admin/` frame now opens to staff and admins via `RequireAuth + RequireStaff`.
- **`HasAdmin` apps self-gate admin-only routes.** Wrap them in `r.Group(func(r chi.Router) { r.Use(auth.RequireAdmin()); … })` inside `AdminRoutes`. Built-in `contrib/auth` and `contrib/jobs` already do this.
- **`burrow.AdminAuth` interface gains `RequireStaff()`.** Custom providers must add the method (one-line stub: return `RequireAdmin()` to preserve v0.22 frame semantics).
- **`auth promote` / `auth demote` CLI removed.** Replaced by `auth set-role <username> <user|staff|admin>` with role validation.

### Added

- **`burrow.NavItem.StaffOnly` flag** sitting next to `AuthOnly`/`AdminOnly`.
- **`burrow.IsAuthenticated` / `IsStaff` / `IsAdmin` helpers** plus `AuthChecker.IsStaff` so app code can query auth state without importing `contrib/auth`.
- **`auth.RequireStaff()` middleware** for staff-only routes outside `/admin/` (e.g. a `/studio` shell).

## 0.22.1 — 2026-05-21

### Fixed

- **`contrib/auth` recovery flow auto-regenerates on last-code consumption.** When `RecoveryLogin` consumes the user's final unused code, the handler now routes through the `/auth/recovery-codes` ack flow so passkey-only accounts can't end up at `remaining_codes: 0` without a fresh safety net. Happy path with `remaining > 0` is unchanged.

## 0.22.0 — 2026-05-20

> **Migrating from v0.21.x?** See the [v0.22 migration guide](migration/v0.22.md) for the concrete before/after patterns.

### Breaking Changes

- **`Seedable` interface and `--seed` flag removed; replaced by `HasMigrations` wired to Den's migrate package.** Apps that previously implemented `Seed(ctx) error` now implement `Migrations() []burrow.NamedMigration`, returning versioned migrations whose `Forward` functions take a `*den.Tx`. The server applies them automatically at boot — each migration runs exactly once across processes (tracked in the `_den_migrations` collection), namespaced by app name (`{app}/{version}`). Migration recipe: move the old `Seed` body into a `migrate.Migration{Forward: func(ctx, tx) error { ... }}` and use `den.Save(ctx, tx, doc)` / `den.NewQuery[T](tx, ...)` directly inside Forward — repos keep their `*den.DB` shape; migrations are setup code that operates in the framework-persistence layer. Drop the `--seed`/`SEED=true` invocation — booting is enough. All six tutorial-step polls apps migrated alongside.

- **`--i18n-default-language` and `--i18n-supported-languages` flags removed.** The supported locale set is now derived from each `HasTranslations` app's `TranslationFS()`: `Bundle.AddTranslations` extends the matcher with every locale it loads from `active.<locale>.toml` filenames. The bundle's source-language tag is hardcoded to English — with the Label-as-key convention, the literal you write in `{{ t "..." }}` IS the source string, so a German-first project just writes German keys, ships only `active.de.toml`, and needs no framework configuration. `i18n.NewBundle()` and `i18n.NewTestBundle(translationFSes...)` lose their leading args. Apps that previously set `--i18n-*` flags fail loudly with urfave/cli's unknown-flag error — migrate by deleting them.

- **`NavItem.LabelKey` and `Choice.LabelKey` removed.** Labels now double as the i18n message ID — `navLinks` pipes `NavItem.Label` through `i18n.T` directly, so contribute translations keyed by the English Label (e.g. `Users = "Benutzer"`) instead of an opaque `admin-nav-users` key. On a translation miss the raw Label is rendered. Built-in contribs (`auth`, `jobs`) and the `example/notes` app migrate their translation TOMLs accordingly. See [navigation](guide/navigation.md#translation) and [i18n: Labels as Keys](guide/i18n.md#labels-as-keys).
- **`auth.User` is now generic — `auth.User[P any]`.** The `Profile` type parameter holds app-specific extension fields (display name, bio, avatar, social links, …) stored inline as a nested JSON object on the user document. Apps without extensions parametrise with `auth.EmptyProfile`. `auth.New()` → `auth.New[auth.EmptyProfile]()`; `auth.Repository` → `auth.Repository[auth.EmptyProfile]`; `auth.CurrentUser(ctx)` → `auth.CurrentUser[auth.EmptyProfile](ctx)`. The `Name` and `Bio` fields are removed from User core — apps that want a display name define a `Profile.Name` and render `{{ .User.Profile.Name }}` with `{{ .User.Username }}` as a fallback. `Repository.CreateUser` and `CreateUserWithEmail` no longer take a `name` parameter; `Repository.SearchUsers` searches Username and Email only (Profile-field search lives in app code). The `authtest` package hard-codes `auth.EmptyProfile`; apps with a custom Profile maintain their own seeding helpers. Existing user records in the database load fine — missing `profile` keys unmarshal to the zero value of P. See [Extending the User](contrib/auth-profile.md) for the full migration recipe. `den:` tags on Profile fields (index, unique, fts, index_together) are honoured via Den 0.14's nested-struct schema walk. `auth.Configure` rejects `document.Base`-embedding Profile types at startup, since Profile is serialised inline and is not a Den document.

### Changed

- **`--auth-webauthn-rp-id` falls back to the base URL's hostname** instead of hardcoding `"localhost"`. Local dev with `host=localhost port=8080` still resolves to `localhost` (port stripped); deployed apps with `--base-url=https://app.example.com` now correctly derive RPID `app.example.com` without needing to set the flag. Pass the flag explicitly to override for registrable-suffix setups (e.g. `--auth-webauthn-rp-id=example.com` to share credentials across subdomains).
- **`forms.BoundField.Label` and `forms.Choice.Label` are now auto-translated** by `extractFields` via the same Label-as-key convention as `NavItem.Label`. Templates render `{{ .Label }}` and get the locale-appropriate string for free — no `{{ t .Label }}` wrapping needed. Contribute translations keyed by the English `verbose:` / `Choice.Label` value. Added `Form.WithContext(ctx)` for building forms outside the request lifecycle (background jobs, CLI). See [forms: Translating field labels](guide/forms.md#translating-field-labels).
- **`contrib/{admin,auth,jobs}` short structured-key labels migrated to Label-as-key** in line with the new convention. Filter tabs, action buttons, table headers, field labels, status badges — i.e. anything that's a single-word or short-phrase UI label — now use bare English keys (`{{ t "Active" }}` instead of `{{ t "admin-users-active" }}`). Full sentences, prompts, confirmations, and plural variants keep their structured keys. Translation TOMLs now have a "Field labels" block at the top followed by structured "Messages" — see [i18n: Labels vs. Messages](guide/i18n.md#labels-vs-messages) for the rule.
- **Den bumped to v0.14.0 — `den:` tags now flow through nested struct fields.** Profile-style extensions on `auth.User[P]` (and any other named-struct embedder) can declare `den:"index"` / `unique` / `fts` / `index_together` directly on Profile fields; Den emits JSON-path indexes like `$.profile.slug` on both SQLite and PostgreSQL. See Den's [Nested Field Indexes](https://github.com/oliverandrich/den/blob/main/docs/guide/documents.md#nested-field-indexes). Heads-up: `den.Revert` now zeroes the doc before decoding the snapshot — burrow doesn't call `Revert`, so the behaviour change only matters for app code that does.
- **Den bumped to v0.15.0 — internal cleanup, no API change.** Dropped `goccy/go-json` and `oklog/ulid/v2` from the transitive graph; ULIDs are now produced by Den's in-tree monotonic generator (fixes a latent intra-millisecond ordering bug for `Sort("_id")` and cursor pagination) and JSON encoding uses stdlib with a pooled `Encoder.SetEscapeHTML(false)` for JSONB columns. Wire-compatible. The removed `storage/s3` backend was not used by any burrow contrib.
- **`burrow new` scaffold template synced with Den 0.15.** The embedded `go.mod.tmpl` pins `oliverandrich/den v0.15.0` and drops the `goccy/go-json` and `oklog/ulid/v2` indirect entries; scaffolded projects build cleanly without a manual reconcile. The `internal/app/app.go.tmpl` package doc loses the "Pattern B" jargon, matching the docs cleanup from PR #33.

### Added

- **`--app-name` global flag** (`APP_NAME` env, `server.app_name` toml) sets a human-readable application name available as `cfg.Config.Server.AppName`. Defaults to the binary basename (e.g. `notes`, `myapp`) so it's non-empty out of the box. Three consumers wired:
    - **WebAuthn**: `--auth-webauthn-rp-display-name` loses its `"Web App"` default and falls back to `--app-name` when not set explicitly. The browser passkey dialog now reads "Save passkey for **Notes**" instead of "Save passkey for **Web App**" by default.
    - **SMTP From**: a bare `--smtp-from=noreply@example.com` is decorated as `"Notes <noreply@example.com>"` automatically. Pre-formatted `--smtp-from="Acme <noreply@example.com>"` is left alone.
    - **Template func**: `{{ appName }}` returns the configured name. Use it in layout titles, email subjects, footer copy, etc.
- **`forms` package: nested-struct subform support.** Struct-typed form fields now render as **subforms** — `extractFields` recurses one level into the nested struct and exposes its fields under the parent's `BoundField.SubFields`. Nested `FormName` values follow the `parent.child` convention (e.g. `profile.name`) so `burrow.Bind` decodes them and validation errors route to the correct nested field. `time.Time` is excluded (keeps the `"date"` widget); pointer-to-struct is dereferenced (nil → zero-value sub-fields); recursion is capped at one level. Templates dispatch on `Type == "subform"` and iterate `.SubFields` — see [forms: Nested struct fields](guide/forms.md#nested-struct-fields-subforms).
- **Translation guard test** (`TestTranslationsLoadInGoI18n`) loads every contrib's `active.*.toml` through a real go-i18n bundle, so reserved-key collisions (`id`, `hash`, `description`, `leftdelim`, `rightdelim`, plural keys) surface at test time instead of at server boot.

### Removed

- **`cmd/burrow-tailwind` deprecation shim deleted.** The standalone binary printed a deprecation notice and delegated to `burrow tailwind` in v0.21; v0.22 removes it. Existing projects: replace the `tool github.com/oliverandrich/burrow/cmd/burrow-tailwind` directive in `go.mod` with `tool github.com/oliverandrich/burrow/cmd/burrow`, and rewrite `.mise.toml` / `.air.toml` invocations from `go tool burrow-tailwind ...` to `go tool burrow tailwind ...`. See the [Tailwind guide](guide/tailwind.md).

### Fixed

- **`contrib/jobs` translation bundle no longer collides with go-i18n reserved keys.** The Label-as-key migration had introduced `ID = "ID"` in `active.{en,de}.toml`; `id` is one of go-i18n's reserved top-level fields (alongside `hash`, `description`, plural keys), so the loader rejected the file at boot. Renamed the label to "Job ID" in both TOMLs and `templates/admin_detail.html`.
- **`contrib/auth` admin user-edit page renders Profile as a real subform.** `forms.extractFields` already emitted `Type: "subform"` with populated `SubFields` for the `User[P].Profile` struct field, but `templates/admin_user_form.html` had no subform branch — the field fell through to the text-input default and rendered as a literal `{}` placeholder. The template now extracts the per-field markup into a shared `auth/admin_user_field` block and adds a `<fieldset>`-based subform branch that recurses into `.SubFields`.
- **`contrib/auth` admin user-edit Role dropdown matches the surrounding design.** The native `<select>` previously kept the browser's default chevron and styling, visibly out of place next to the Tailwind-styled inputs. The select branch now wraps the element in a `grid` container, applies `appearance-none`, reserves `pr-8` for a custom inline chevron SVG positioned via `justify-self-end` (`pointer-events-none` so clicks still reach the select), and tints `<option>` backgrounds in dark mode via the `dark:*:` variant.

## 0.21.2 — 2026-05-18

### Changed

- **Dropped "Django-inspired" framing from current-state copy.** Scaffolded project's homepage and README, plus the `forms` package doc, now describe Burrow on its own terms. The broader marketing comparison in burrow's own README and `docs/index.md` ("for Go developers who want something like Django, Rails, or Flask") is kept intentionally.
- **Scaffolded project's README rewritten for post-scaffold reality.** Quick Start leads with `mise run setup && mise run dev` instead of the obsolete `gohatch` invocation; the gohatch requirement and the "Template Variables" metadata section are gone; the Development table uses `burrow tailwind`.

## 0.21.1 — 2026-05-18

### Changed

- **`burrow new` bootstrap UX.** The destination is now auto-initialized as a git repo when `git` is on `PATH` (no initial commit — that's the user's choice). The printed Next-steps switches to a mise-aware two-liner (`mise run setup` + `mise run dev`) when mise is detected, falling back to `go mod tidy && go run` when it isn't. The scaffolded `.mise.toml`'s `[tasks.setup]` is now a one-stop bootstrap: `mise install` + `go mod tidy` + dev-key generation + `pre-commit install` (when `.git/` exists). Docs and README updated to lead with the mise path. No `--no-git` flag — second `git init` is a harmless no-op for users in CI.

## 0.21.0 — 2026-05-18

### Added

- **`cmd/burrow` CLI** with three sub-commands: `burrow new <dir> --module <path>` scaffolds a new burrow project from an embedded template (the former go-burrow-template repo, now bundled); `burrow generate app <name>` scaffolds a contrib-style app stub inside an existing project, default path `./internal/<name>`; `burrow tailwind <args...>` consolidates the standalone `cmd/burrow-tailwind` tool. Install via `go install github.com/oliverandrich/burrow/cmd/burrow@latest`. Scaffolded projects auto-pin to the burrow version that generated them (via `runtime/debug.ReadBuildInfo` with git-tag fallback).

### Changed

- **Docs reflect the new CLI.** Quickstart and installation pages lead with `burrow new`; `docs/guide/tailwind.md` uses `burrow tailwind` throughout (with a migration callout at the top); new `docs/reference/cli.md` documents `new`, `generate app`, and `tailwind` sub-commands.

### Deprecated

- **`cmd/burrow-tailwind` is deprecated** in favour of `burrow tailwind`. The standalone binary now prints a one-line deprecation notice and delegates to the new sub-command. Migrate by replacing `go tool burrow-tailwind ...` with `go tool burrow tailwind ...` in `.mise.toml`, `.air.toml`, and any other build scripts. Scheduled for removal in v0.22.

## 0.20.0 — 2026-05-17

> **Migrating from v0.18.x?** See the [v0.20 migration guide](migration/v0.20.md) for the concrete before/after patterns.

### Breaking Changes

- **CSS stack swap: Tailwind v4 in, mucss / bootstrap / bsicons / alpine out.** Burrow ships no CSS bundle of its own anymore — every in-tree contrib (`admin`, `auth`, `jobs`) and example uses Tailwind v4 utilities. Apps consume them via the new [`cmd/burrow-tailwind`](guide/tailwind.md) wrapper, which auto-discovers every contrib's template directory so utility scanning Just Works. Theme-switcher machinery is gone; dark mode follows `prefers-color-scheme`. Apps still on the deleted contribs must either migrate to Tailwind (see `example/notes` and `docs/guide/tailwind.md`) or vendor the old CSS themselves.
- **`NavItem.Icon` and `NavLink.Icon` are now `string`** (a template-define name) instead of `template.HTML`. Layouts render icons via the new `{{ icon .Icon }}` template function. Move icon SVGs into `templates/icons.html` with `{{ define "<app>/icon_<name>" }}` blocks and set `Icon: "<app>/icon_<name>"`. `contrib/admin.DashboardItem.Icon` follows suit. See `docs/guide/navigation.md`.
- **`HasDocuments.Documents()` returns `[]document.Document` instead of `[]any`.** Den 0.13.1 tightened `den.Register` to take `...document.Document`; burrow's interface follows so the document-type contract is enforced at compile time end-to-end. Types embedding `document.Base` satisfy the marker automatically. Migration: replace `return []any{&Foo{}, &Bar{}}` with `return []document.Document{&Foo{}, &Bar{}}` and add the `github.com/oliverandrich/den/document` import.
- **Silent template-func stubs removed; contrib `Dependencies()` reflect real coupling.** The core base FuncMap no longer stubs `csrfToken` / `csrfField` / `csrfHxHeaders` / `lang`; templates using them without the providing contrib now fail at parse time instead of rendering empty. `auth.Dependencies()` is now `[session, csrf, staticfiles]`; `admin.Dependencies()` adds `csrf` and `auth`; `jobs.Dependencies()` is now `[admin]`. Host apps registering `auth.New()` without `csrf` or `staticfiles` will fail at startup — register the missing contribs.
- **`auth.DefaultAuthLayout()` returns `"auth/layout"` instead of `"mucss/layout"`.** `contrib/auth` now ships its own Tailwind-styled, navbar-less default layout. Hosts get a clean login page out of the box (no leaked host navbar). The layout links `{{ staticURL "app/app.min.css" }}` (Pattern B convention); hosts on a different CSS path override via `auth.WithAuthLayout("...")` or pass `""` to inherit the host's `srv.SetLayout` layout.
- **Den bumped to v0.13.1 — top-level CRUD wrappers collapsed; struct-tag validation always-on.** `den.Insert` / `den.Update` → `den.Save` (branches on `doc.ID`); `den.InsertMany` → `den.SaveAll`; `den.DeleteMany[T](ctx, db, conds)` → `den.NewQuery[T](db, conds...).Delete(ctx)`; `den.FindOneAndUpdate[T](...)` → `den.NewQuery[T](db, conds...).UpdateOne(ctx, fields)`. `burrow.OpenDBWithoutValidation` removed (no escape hatch left to opt out of). See the [Den changelog](https://github.com/oliverandrich/den/blob/main/CHANGELOG.md) for the full table.

### Added

- **Tailwind v4 toolchain.** `cmd/burrow-tailwind` wraps the upstream `tailwindcss` standalone CLI, auto-generating `@source` directives for every contrib's template directory. Install via `go get -tool github.com/oliverandrich/burrow/cmd/burrow-tailwind`, then `go tool burrow-tailwind -i tailwind.css -o static/app.min.css [--watch | --minify]`. `.mise.toml` pins both the Go toolchain and `tailwindcss` so `mise install` is the one-shot setup. New contribs are picked up automatically on `go get -u` — no per-project source-list maintenance. See [`docs/guide/tailwind.md`](guide/tailwind.md).
- **`{{ icon "name" }}` template function** in the core FuncMap. Looks up the named template define and returns its rendered HTML, used by layouts to render `NavItem.Icon` / `NavLink.Icon` / `admin.DashboardItem.Icon` dynamically. Necessary because Go's built-in `{{ template "name" . }}` action requires a string literal at parse time.
- **`Server.CLICommands()`** returns the same subcommands as `Server.Registry().AllCLICommands()` but wraps each `Action` to open the DB and run `Configure()` on every app first. Use this when wiring contrib subcommands like `auth promote` — without it, the subcommand fires against uninitialised apps. `Server.TemplateExecutor()` exposes the framework's executor for non-HTTP rendering (background jobs, SSE broadcasts) after boot.
- **`burrowtest.StubApp(name)`** helper for tests that need to satisfy a contrib's `Dependencies()` declaration without actually exercising the depended-on contrib.

### Changed

- **Dev tooling migrated from `justfile` to mise.** `.mise.toml` pins the Go toolchain and dev tools (`golangci-lint`, `tparse`, `goimports`, `govulncheck`, `go-licenses`, `go-ignore-cov`, `pre-commit`); tasks live in `[tasks.*]` blocks or `mise-tasks/<name>` scripts. `just <recipe>` → `mise run <task>`. Run `mise install` after cloning.
- **`contrib/jobs` `RescueStale` runs as a single bulk update** instead of an `All` + per-row `UpdateOne` loop, closing the race window between snapshot and rescue — the `status=running` guard is now re-evaluated at write time. No public API change.
- **Documentation refreshed end-to-end for v0.20.** `docs/guide/testing.md` rewritten against the current `burrowtest` API (replaces removed `burrow.TestDB` / `burrow.TestErrorExec*`); `docs/reference/server.md` boot sequence rewritten to match `Server.Run` reality plus new sections for `Server.CLICommands` / `Server.TemplateExecutor`; `docs/reference/core-functions.md` gains a `burrow.OpenDB` reference; type / flag / dep references aligned across `interfaces.md`, `core-functions.md`, `context-helpers.md`, `configuration.md`, `template-functions.md`, and every contrib page. Tutorial parts 1-8 prose reconciled with the actual `tutorial/stepNN/` source; `tutorial/step06..08/main.go` gain `Commands: srv.CLICommands()`.

### Removed

- **`contrib/mucss`, `contrib/bootstrap`, `contrib/bsicons`, `contrib/alpine`** — all four former design / icon / JS-helper contribs deleted alongside the `AppConfig.RegisterIconFunc` / `IconFuncs` / `IconFunc` API surface, the µCSS/Bootstrap CSS bundles, the `update-icons` / `update-mucss` / `update-bootstrap` / `sass` mise tasks, and the vendored Bootstrap-Icons + Alpine.js + theme-switcher assets. `example/themes` (a µCSS accent-variant preview) is also gone.

### Fixed

- **`admin/pagination` template lifted into `contrib/admin`.** The three near-identical `{{ define "<app>/pagination" }}` blocks (`jobs/pagination`, `auth/pagination`, `notes/pagination`) collapsed into a single canonical `admin/pagination` with the same `(dict "Page" .Page "BasePath" "..." "RawQuery" .RawQuery)` contract. Downstream apps that override `jobs/pagination` / `auth/pagination` / `notes/pagination` must switch to overriding `admin/pagination` instead.
- **`tutorial/step03..08` migrated off mucss.** The `internal/pages` package was renamed to `internal/app` and now ships a hand-written `app.css` via `HasStaticFiles` instead of depending on the deleted `contrib/mucss`. Each step's `main.go` drops the `mucss.New()` registration; the layout templates link `{{ staticURL "app/app.css" }}`.
- **CLI subcommands now run with apps `Configure()`'d.** `Server.CLICommands()` (see Added) replaces `Server.Registry().AllCLICommands()` as the recommended wiring; without it, `auth promote` and friends failed with `auth app not initialized`. **Migration:** in your `cli.Command{...}`, replace `Commands: srv.Registry().AllCLICommands()` with `Commands: srv.CLICommands()`.
- **`{{ icon "..." }}` no longer poisons the global template tree.** `Server` now keeps a separate `iconTemplates` Clone for icon execution so executing icon defines doesn't mark the main `templates` tree as "executed". Before the fix, any page that rendered an icon followed by a layout wrap returned `HTTP 500: html/template: cannot Clone "" after it has executed`. Regression test: `TestExecuteTemplateUsingIconDoesNotPoisonClone`.
- **`example/notes` no longer renders the empty-state placeholder alongside created notes.** `notes/create_response` OOB-deletes the `#notes-empty` placeholder when prepending the first note, and a new `notes/delete_response` re-injects it when the last note is deleted.

## 0.18.1 — 2026-05-06

### Fixed

- **Vendored Alpine.js and µCSS attributions added to `THIRD_PARTY_LICENSES.md`.** Alpine.js v3.15.8 (MIT, Copyright (c) 2019-2025 Caleb Porzio and contributors) under `contrib/alpine/static/` and µCSS v1.4.8 (MIT, Copyright (c) 2026 Digicreon) under `contrib/mucss/static/` were both shipped without an entry in the third-party listing.

## 0.18.0 — 2026-05-05

### Breaking Changes

- **`contrib/admin` ported to µCSS, sidebar dropped.** Admin uses a top-nav layout with a card-grid dashboard at `/admin/`; per-page breadcrumbs replace the sidebar, and the admin-area templates in `contrib/auth`, `contrib/jobs`, and `example/notes` were rewritten in µCSS. **Public-API:** `SidebarGroup`/`SidebarItem`/`PrepareSidebar` → `DashboardGroup`/`DashboardItem`/`PrepareDashboard`; the `Active` field, `sidebarLinkClass`, `isActivePath`, and the `adminSidebar` template func are gone (now `adminDashboard`). `contrib/admin` now declares `staticfiles`, `htmx`, `mucss`, `messages` as Dependencies. Apps with custom admin extensions emitting Bootstrap markup must port to µCSS.
- **`contrib/auth` user-facing templates ported to µCSS.** `auth.DefaultAuthLayout()` now returns `"mucss/layout"`; element IDs unchanged so `webauthn.js` keeps working. Inline JS visibility toggles switched from `classList.add/remove("d-none")` to native `element.hidden = true/false` — custom auth templates copying that pattern must follow. Apps still on Bootstrap must set `auth.New(auth.WithAuthLayout("bootstrap/layout"), auth.WithRenderer(...))`.

### Added

- **`contrib/mucss`** — new default design contrib providing [µCSS v1.x](https://mucss.org/) (PicoCSS v2 derivative with upstream bugs fixed and richer components: Hero, Alert, Toast, Modal, Pagination, Badge, Tabs). 20 named accent variants (`WithColor`), `WithCompactType()` for app/admin UIs, customisable via CSS custom properties (`WithCustomCSS`). The app implements `Configurable` to register `iconSunFill`, `iconMoonStarsFill`, and `iconCircleHalf` template functions used by the theme switcher.
- **`contrib/mucss` ships Burrow UX extras** (`mu-extras.min.css`, always loaded unless `WithCustomCSS` is set): navbar-dropdown min-width, navbar-icon vertical centering, dialog header flex layout, dialog form-footer button layout, and a `.field-error` rule for inline form errors.
- **HTMX-driven dialog pattern.** `htmx.OpenDialog(w, id [, class])` and `htmx.CloseDialog(w, id)` emit events that the `htmx/dialog_script` template turns into `dialog.showModal()` / `close()` calls. The `mucss` and `bootstrap` nav layouts ship a permanent `<dialog id="modal">` container and include the script. The optional class arg switches the µCSS modal size variant. Inside an open dialog, clicking any element with `rel="prev"` (µCSS close-icon button convention) or the behavioral `data-close-dialog` attribute closes the dialog client-side without a round-trip. See `example/notes` for the full create + edit flow.
- **`example/themes`** — new demo app (`just example-themes`) previewing every µCSS accent variant side-by-side with the canonical button/card/alert/badge layout. Useful for picking a `WithColor(...)` value for your project.
- **`uploader.ServeHandler` supports Range and conditional GET** when the underlying Storage implements `den.SeekableStorage` (file backend). Browsers can now stream video Range-by-Range, and clients with cached copies get 304 Not Modified instead of a full re-download. ETag is the storage path (content-addressed, so stable). Non-seekable backends (S3) keep the existing `io.Copy` fallback — Range support there belongs at the URL layer (pre-signed URLs).
- **`burrow.CacheControlImmutable`** constant for the year-long immutable Cache-Control value used by content-addressed responses. `contrib/staticfiles` and `uploader` both consume it; downstream apps that serve content-addressed bytes can use it too instead of inlining the literal.

### Changed

- **Den bumped to v0.11.2.** `OpenDB` and `OpenDBWithoutValidation` now wrap the typed `den.ErrUnsupportedScheme` sentinel, so callers can match unsupported-scheme errors via `errors.Is` instead of substring-matching the message. `uploader.ServeHandler` consumes the new `den.SeekableStorage` capability for Range / conditional-GET support (see Added).
- **`contrib/mucss` updated to µCSS 1.4.8.** Upstream fix for `<details class="dropdown">` summaries inside coloured navbars — the same pattern Burrow's theme switcher uses. No API or token change.
- **`example/hello` ported to µCSS** with `WithCompactType`, a `.hero` block, and a native `<dialog>` demo. The `example/hello-pico` evaluation example was removed.
- **`example/notes` user-facing pages ported to µCSS.** `bootstrap.New()` stays alongside `mucss.New(WithCompactType())` until the admin-area auth templates migrate.
- **Docs lead with µCSS as the default design contrib.** README, getting-started, contrib/admin, tutorial part 6, and reference docs updated; the "CSS framework default" Open Question is resolved (µCSS default, Bootstrap deprecated).
- **Tutorial Part 8: Custom Admin Views.** New `tutorial/step08` extends the polls app with `burrow.HasAdmin` and full Question/Choice CRUD admin views; new `docs/tutorial/part8.md` walks through it. Index, mkdocs nav, and Part 7's "Next" pointer updated.
- **`tutorial/step03`–`step07` ported to µCSS.** Each step registers `mucss.New()`. The user menu evolves: step05/06 use a simple inline sign-out form; step07 — once htmx is loaded — refactors into a `<details class="dropdown">` with `hx-post`. Polls app gains `Seedable` (`Seed(ctx)` for `--seed`). Tutorial docs `part3.md`–`part7.md` updated to match.

### Deprecated

- **`contrib/bootstrap`** is deprecated, removal in v0.20. Critical fixes only. New projects should use `contrib/mucss` — the port is mostly mechanical (`<div class="card">` → `<article>`, `alert-danger` → `alert-error`). `contrib/bsicons` (not Bootstrap-coupled) stays.

### Fixed

- **`mucss/theme_switcher`** marks the currently active option with `aria-current="true"` (rendered bold via `mu-extras.min.css`). Previously all three options looked identical when the dropdown was open.

## 0.17.0 — 2026-04-28

### Breaking Changes

- **Den backend imports moved out of `db.go`** — every binary using `burrow.OpenDB` must now blank-import the Den backend that matches its DSN in its own `main.go`. Burrow no longer pulls in either backend by default, so production binaries only link the engine they actually use (sqlite-only deployments drop ~3 MB; postgres-only deployments drop ~9 MB). Migration:

    ```go
    // main.go
    import (
        _ "github.com/oliverandrich/den/backend/sqlite"   // for sqlite:// DSNs
        _ "github.com/oliverandrich/den/backend/postgres" // for postgres:// DSNs
    )
    ```

    `OpenDB` and `OpenDBWithoutValidation` wrap Den's "unsupported database scheme" error with the exact import path to add. The `den/storage/file` blank-import stays in `db.go` (negligible weight, no native deps); `s3://` is already opt-in via `den/storage/s3`.

- **`burrow.TestDB`, `burrow.TestErrorExecContext`, `burrow.TestErrorExecMiddleware` moved to new sub-package `burrow/burrowtest`** and renamed (Test prefix dropped, matching the `httptest` / `fstest` convention). Migration:

    ```go
    // before
    db := burrow.TestDB(t)
    ctx := burrow.TestErrorExecContext(ctx)
    handler := burrow.TestErrorExecMiddleware(next)
    // after
    import "github.com/oliverandrich/burrow/burrowtest"
    db := burrowtest.DB(t)
    ctx := burrowtest.ErrorExecContext(ctx)
    handler := burrowtest.ErrorExecMiddleware(next)
    ```

    The sub-package blank-imports SQLite for `DB(t)`. Production binaries that do not depend on `burrowtest` therefore do not link SQLite via this path.

## 0.16.0 — 2026-04-28

### Breaking Changes

- **`--media-url-prefix` flag removed** — the public URL prefix for locally served attachments now lives inside `--storage-dsn` as a `?url_prefix=…` query parameter. Default DSN is `file:///data/media?url_prefix=/media/`, so out-of-the-box behavior is unchanged. Migrate explicit configs by folding the prefix into the DSN, for example `STORAGE_DSN='file:///data/media?url_prefix=/uploads/'`. The `MEDIA_URL_PREFIX` env var, the `storage.url_prefix` TOML key, and the `StorageConfig.URLPrefix` Go field are gone with the flag.
- **Den upgraded to v0.11.0** — see the [den v0.11.0 CHANGELOG](https://github.com/oliverandrich/den/blob/main/CHANGELOG.md#0110--2026-04-27) for the full list. The two changes that surface in burrow's contract: the single-arg `storage.OpenURL` (covered above) and the `[]where.Condition` slice argument on `FindOneAndUpdate` (callers that pass conditions to it must wrap them in a slice).

## 0.15.0 — 2026-04-19

### Breaking Changes

- **`contrib/uploads` → `uploader`** — moved out of `contrib/` because it is no longer a `burrow.App`. New import path: `github.com/oliverandrich/burrow/uploader`. Construct the `*uploader.Uploader` in your domain app's `Configure` (`u := uploader.NewUploader(cfg.DB)`) and register the serving route in `Routes` (`uploader.Mount(r, cfg.DB.Storage())`). File ingress is `u.Store(r, "file", opts)`. Gone: `App`, `Store` interface, `LocalStorage`, all `With*` options, all `--uploads-*` flags and `UPLOADS_*` env vars, the middleware, the context helpers, `StoreFile`, `ErrNoStorage`, `StoreOptions.Storage`, and `StoreOptions.Prefix`. `NewUploader` panics if the DB has no Storage. Orphan-bytes cleanup on `Insert`/`Update` failure is the caller's responsibility (`_ = u.Storage().Delete(ctx, att)`); an offline sweeper is planned. See [`docs/guide/uploader.md`](guide/uploader.md) for the new shape.

### Added

- **Built-in Storage configuration** — two new core flags construct a `den.Storage` at boot and install it on the opened `*den.DB`:
    - `--storage-dsn` (`STORAGE_DSN`, default `file:///data/media`) — Storage URL. Schemes: `file://` (SQLAlchemy/JDBC-style: `file:///relative` or `file:////absolute`; one leading slash is stripped on parse). Set to an empty string to disable Storage.
    - `--media-url-prefix` (`MEDIA_URL_PREFIX`, default `/media/`) — public URL prefix for locally served attachments.

    Domain apps receive the configured Storage via `cfg.DB.Storage()`; no `main.go` setup is required. Mirrors the `--database-dsn` pattern so the full data layer is flag-driven.
- **Built-in `mediaURL` template function** — when a Storage is installed on the DB, Burrow auto-registers `mediaURL` (an alias for `den.Storage.URL`) in the global `FuncMap`. Templates can render attachments with `{{ mediaURL .Hero }}` without any per-app `FuncMap` wiring. When Storage is disabled, the function is not registered, so any reference fails at template-parse time — the right signal for an unwired dependency.
- **`burrow.OpenDB` accepts `den.Option` variadics** — `OpenDB(ctx, dsn, opts...)` forwards options to `den.OpenURL` after applying `validate.WithValidation()`. Used internally to layer in `den.WithStorage`; callers who open the DB themselves (tests, tools) can layer in their own options.
- **Tooling page in Getting Started docs** — new `getting-started/tooling.md` documents the two companion projects: the [`go-burrow-template`](https://github.com/oliverandrich/go-burrow-template) project template (scaffolds a runnable Burrow app with contrib stack, air live reload, goreleaser config, and CI) and the [`burrow-claude-plugin`](https://github.com/oliverandrich/burrow-claude-plugin) Claude Code plugin (specialized agents and commands for feature development, architecture, and review). Linked from `quickstart.md` as a "faster start" entry point.
- **`zizmor` workflow audit in CI** — new `zizmor` job in `.github/workflows/ci.yml` runs the [zizmor](https://docs.zizmor.sh/) static analyzer against all workflow files on every PR and push. A `.github/zizmor.yml` config documents the one accepted risk (the `workflow_run` trigger in `release.yml`, gated on branch-prefix + CI success).

### Changed

- **Default `--database-dsn` is now `sqlite:///data/app.db`** — colocated with the attachment storage root at `./data/media`, so a fresh project keeps all its local state in one directory. Den v0.10.1's SQLite backend auto-creates the parent directory on `Open`, so first-run setup needs no manual `mkdir`.
- **Den upgraded to v0.10.1** — picks up the new `document.Attachment` embed, `den.Storage` interface, pluggable storage-backend registry (`storage.OpenURL` + `storage/file` sub-package), the `URLPrefix()` accessor the `uploader` package uses to derive its serving route, and SQLite auto-mkdir on open.
- **CI and release workflows hardened** — all `actions/*` and `golangci/*` uses are now pinned to commit SHAs with version comments. `persist-credentials: false` on every `actions/checkout`. `actions/setup-go` runs with `cache: false` to prevent cache-poisoning on tag pushes. `release.yml` routes `github.event.workflow_run.head_branch` through a `VERSION` env var instead of direct `${{ … }}` interpolation inside `run:` blocks, closing the template-injection vector. The manual `actions/cache` steps were removed.

## 0.14.0 — 2026-04-19

### Breaking Changes

- **Den upgraded to v0.8.0** — picks up the sealed `Scope` unification (Tx\* CRUD variants removed), ctx-on-terminals for `QuerySet`, ctx on `Open`/`OpenURL`, renamed change-tracking `Rollback` → `Revert`, composable `document.SoftDelete`/`document.Tracked` embeds (replacing `SoftBase`/`TrackedBase`/`TrackedSoftBase`), non-blocking PostgreSQL index creation, GIN-friendly `Eq` predicates, and batched `WithFetchLinks`. See the [den v0.8.0 CHANGELOG](https://github.com/oliverandrich/den/blob/main/CHANGELOG.md#080--2026-04-18) for the full list.
- **`burrow.OpenDB` and `burrow.OpenDBWithoutValidation` take a leading `context.Context`** — mirroring den's new `Open`/`OpenURL` signature. A cancelled context aborts the open cleanly; callers with a startup deadline now get the guarantee they expect. Migration: `burrow.OpenDB(dsn)` → `burrow.OpenDB(ctx, dsn)`. The internal server boot already has `ctx` in scope; tests should pass `t.Context()`.

### Changed

- **`contrib/jobs` claim uses `SELECT ... FOR UPDATE SKIP LOCKED`** — the per-candidate CAS retry loop is replaced by a single locking query inside a transaction. N concurrent workers each receive a disjoint slice of the pending set without blocking each other, removing the O(N²) CAS contention under load. On SQLite the `ForUpdate` modifier is a no-op (IMMEDIATE transactions already serialize writers), so behavior is unchanged there. No schema migration required. Enabled by den v0.8.0's `den.NewQuery[T](tx).ForUpdate(den.SkipLocked())`.

## 0.13.1 — 2026-04-15

### Changed

- **Den upgraded to v0.7.0** — picks up PostgreSQL JSONB comparison fixes, GroupBy SQL pushdown, revision TOCTOU fix, and link validation enforcement

### Fixed

- **Invalid den struct tags in test models** — `den:"name,index"` and `den:"key,unique"` incorrectly included field names in the den tag (field names come from the json tag). Den v0.7.0 now rejects unknown tag options, catching these at registration time

## 0.13.0 — 2026-04-15

### Breaking Changes

- **Auth admin rewritten with hand-written handlers** — `contrib/auth` no longer uses ModelAdmin for its admin views. User list, edit, delete, and invite management are now direct handlers with custom templates. The `HasAdmin` interface (`AdminRoutes`, `AdminNavItems`) is unchanged. Auth admin templates now use globally registered icon functions (`iconSearch`, `iconPlus`, `iconPersonSlash`, `iconPersonCheck`, `iconTrash`, `iconXCircle`).
- **Jobs admin rewritten with hand-written handlers** — `contrib/jobs` no longer uses ModelAdmin. Job list, detail, delete, retry, and cancel are now direct handlers with custom templates including status filter pills and inline action buttons.
- **ModelAdmin package removed** — `contrib/admin/modeladmin` has been deleted entirely. All admin views now use hand-written handlers. The admin coordinator (layout, sidebar, auth middleware, nav groups) is unchanged.

### Added

- **User search in admin** — the user admin list now supports search across username, name, and email fields.
- **Inline invite creation** — the invite admin list now features an htmx-powered inline form that slides open on button click instead of navigating to a separate page.
- **`burrow.DefineTask[P]()`** — type-safe generic task definitions for the jobs system. Wraps `Queue.Handle` and `Queue.Enqueue` with automatic JSON marshalling, ensuring compile-time agreement between producer and consumer payload types. Auth email jobs migrated as first consumer.
- **`burrow.DefineResultTask[P, R]()`** — variant of `DefineTask` for handlers that return both a result and an error. Results are persisted as JSON on the job and visible in the admin detail view.
- **Job result persistence** — completed jobs now store their handler's return value in a `Result` field (JSON). Failed jobs additionally record the Go error type in `ErrorClass` and the timestamp of the last handler invocation in `LastAttemptedAt`. All three fields are displayed in the admin detail view and cleared on retry.
- **Job priority** — jobs now have a `Priority` field (default 0, higher = more urgent). Set per-type via `burrow.WithPriority(n)` at handler registration. The claim query picks highest-priority jobs first, with FIFO ordering within the same priority level.

### Changed

- **Seed requires `--seed` flag** — `Seedable.Seed()` no longer runs unconditionally on every server start. Pass `--seed` (or set `SEED=true`) to run seed functions. This prevents non-idempotent seeders from creating duplicates on restart.
- **Auth handlers split into focused files** — the monolithic `handlers.go` (714 lines, 25 functions) has been split into `handlers_registration.go`, `handlers_login.go`, `handlers_credentials.go`, `handlers_recovery.go`, `handlers_email.go`, and `handlers_helpers.go`. No API or behavior changes — pure file reorganization for better navigability.
- **Admin decoupled from auth package** — `contrib/admin` no longer imports `contrib/auth` directly. Instead, it discovers auth middleware via the new `burrow.AdminAuth` interface from the registry. `contrib/auth` implements `AdminAuth` automatically. Custom auth systems can provide their own implementation.
- **`Config.IsHTTPS()` helper** — replaces duplicated `strings.HasPrefix(baseURL, "https://")` checks in csrf, session, and secure apps.
- **WebAuthn flag aliases removed** — the legacy `--webauthn-rp-id`, `--webauthn-rp-display-name`, `--webauthn-rp-origin` aliases (without `auth-` prefix) have been removed. Use the canonical `--auth-webauthn-*` names.
- **`burrow.Queue` split into `Enqueuer` + `Queue`** — new `Enqueuer` interface holds `Enqueue`, `EnqueueAt`, and `Dequeue`. `Queue` embeds `Enqueuer` and adds `Handle`. Code that only submits jobs can now accept `Enqueuer` instead of the full `Queue`. `TaskDefinition` and `ResultTask` store `Enqueuer` internally.
- **Form fields with nil pointers render as zero values** — `forms.extractFields` now returns the element type's zero value (e.g. `""` for `*string`) instead of `nil` when a pointer field is nil. Templates can use `{{ .Value }}` on optional fields without special-casing nil.

### Fixed

- **Job retry backoff capped at 1 hour** — exponential backoff (`baseDelay * 2^(attempts-1)`) now caps at 1 hour. Previously, high attempt counts could overflow `time.Duration` and produce negative or astronomically large delays.
- **Session flush logs encoding errors** — `state.flush()` now logs via `slog.Error` when cookie encoding fails instead of silently swallowing the error.
- **Admin rejects duplicate AdminAuth providers** — `admin.Configure()` returns an error if multiple apps implement `AdminAuth`, instead of silently using the first one.
- **Session deferredWriter supports http.Flusher** — the session middleware's response wrapper now implements `Flush()`, fixing SSE and streaming handlers that use direct `w.(http.Flusher)` type assertions.
- **Session cookies written once per request** — `session.Set()`, `Delete()`, and `Save()` no longer write the `Set-Cookie` header immediately. Instead, the session middleware defers the write until the response is sent, producing exactly one `Set-Cookie` header regardless of how many session mutations occur. Previously, each `Set()` call wrote a separate header, and only the last one survived to the browser.
- **Jobs recover from handler panics** — worker goroutines now recover from panics in job handlers, converting them into failures with a stack trace. The worker stays alive and continues processing other jobs.
- **RenderError falls back to plaintext** — when both `error/{code}` and `error/default` templates are missing, `RenderError` now writes a plaintext HTTP error instead of a blank response.
- **NavItem.LabelKey now translated in navLinks** — `buildNavLinks` now translates `LabelKey` via `i18n.T` at render time, falling back to `Label` when no translation is found. Previously `LabelKey` was silently dropped.
- **Uploads no longer buffer entire file in memory** — `LocalStorage.Store` now streams uploads directly to a temp file while computing the SHA-256 hash, instead of reading the entire file into `[]byte`. Only the first 512 bytes are buffered for MIME detection. MIME validation happens before reading the body, so rejected files are discarded early.
- **Jobs admin UI re-enabled** — `contrib/jobs` `AdminRoutes` and `AdminNavItems` were stubbed out during the Den migration. The ModelAdmin integration is now wired up again, restoring the list/detail/retry/cancel/delete views and the sidebar nav entry.
- **Auth redirects use SmartRedirect** — `Logout`, `RecoveryCodesPage`, and `AcknowledgeRecoveryCodes` now use `htmx.SmartRedirect` instead of `http.Redirect`, fixing redirect behavior when triggered via htmx.
- **Invite creation uses SmartRedirect** — `handleCreateInvite` now uses `htmx.SmartRedirect` instead of `http.Redirect`, fixing redirect behavior when the form is submitted via htmx.

## 0.12.0 — 2026-04-08

### Breaking Changes

- **Den struct-tag validation is now enabled by default** — `burrow.OpenDB()` now enables Den's `validate.WithValidation()` automatically, so any document field tagged with `validate:"..."` (e.g., `validate:"required"`, `validate:"email"`, `validate:"oneof=..."`) is enforced before every Insert and Update. Violations return an error wrapping `den.ErrValidation`.

  Projects that had `validate:` tags on Den documents where the tags previously had no effect will now see those constraints enforced. If the data layer needs to stay lax temporarily during migration, use the new `burrow.OpenDBWithoutValidation()` escape hatch. Remove it once the data is clean.

  `burrow.TestDB()` and `authtest.NewDB()` also enable validation so test code runs with the same constraints as production.

- **Upgraded to Den v0.6.0** — Den now runs mutating hooks (`BeforeInsert`, `BeforeUpdate`, `BeforeSave`) before both struct-tag validation and the `Validator.Validate()` interface. This lets a `BeforeInsert` hook populate a default value for a field that validation then requires, matching the ActiveRecord/Django/SQLAlchemy pattern. See the [Den changelog](https://github.com/oliverandrich/den/blob/main/CHANGELOG.md) for details. If you had custom validation that depended on running before the hooks (unusual), move it into `BeforeInsert` itself.

### Added

- **`burrow.OpenDBWithoutValidation(dsn)`** — opens a database with struct-tag validation disabled. Intended only as a migration escape hatch when moving a project from pre-v0.12.0 behavior.

## 0.11.4 — 2026-04-06

### Changed

- **Den updated to v0.5.0** — adds support for composite indexes via `index_together` and `unique_together` struct tags, and fixes `Settings.Indexes` application during `Register()`
- **Composite indexes for Job model** — added `index_together:claim` (RunAt, Status, WorkerID) and `index_together:stale` (Status, LockedAt) to optimize worker claim and stale-job-rescue queries
- **Composite index for RecoveryCode model** — added `index_together:recovery_status` (UserID, Used) to optimize unused recovery code lookups

## 0.11.3 — 2026-04-06

### Changed

- **Den updated to v0.4.2** — adds PostgreSQL version check (requires PostgreSQL 13+) and LLM documentation

## 0.11.2 — 2026-04-05

### Changed

- **License page** — include full MIT license text, link to third-party licenses
- **Release workflow** — release now waits for CI success before creating a GitHub release

## 0.11.1 — 2026-04-05

### Fixed

- **Social card URLs** — updated `og:image` and `twitter:image` meta tags to point to readthedocs.io

## 0.11.0 — 2026-04-05

### Breaking Changes

- **Database layer replaced: Bun → Den** — the entire database layer has been replaced with [Den](https://github.com/oliverandrich/den), an object-document mapper (ODM) for Go. Same API for SQLite and PostgreSQL via URL-based DSN.
- **All IDs changed from `int64` to `string`** — documents now use ULID-based string IDs via `document.Base`
- **`Migratable` interface replaced by `HasDocuments`** — apps declare document types instead of providing SQL migration files. Den creates tables and indexes automatically from struct tags.
- **DSN requires URL scheme** — `--database-dsn sqlite:///app.db` (default) or `--database-dsn postgres://host/db`. Plain file paths no longer accepted.
- **`--jobs-database` renamed to `--jobs-database-dsn`** — env var `JOBS_DATABASE_DSN`, TOML key `jobs.database_dsn`
- **SQL migration files removed** — schema is managed automatically from document struct definitions
- **License changed from EUPL-1.2 to MIT**

### Added

- **PostgreSQL support** — switch between SQLite and PostgreSQL with a single flag, same code, same API
- **New contrib app: `humanize`** — i18n-aware template functions for human-friendly display of times, numbers, and file sizes, inspired by Django's `django.contrib.humanize`

### Changed

- **Handler pattern simplified** — handlers are now methods on `*App` instead of a separate `Handlers` struct
- **Bun dependency removed** — replaced by `github.com/oliverandrich/den v0.4.0`
- **Job queue: ownership guard** — workers stamp `worker_id` on claimed jobs; `Complete`/`Fail` verify ownership via guarded `FindOneAndUpdate`, preventing double processing under concurrent workers
- **Job queue: `Complete`/`Fail` take `*Job`** — no longer reload from DB, eliminating redundant reads and stale attempt counts

### Fixed

- **Admin HTMX navigation** — boosted requests with a custom `hx-target` (e.g. `#main`) no longer wrap content in the layout, fixing the doubled sidebar
- **`itoa` template function removed** — was a no-op after int64→string migration; templates use `.ID` directly
- **Dead code removed** — `URLParamInt64`, `MustURLParamInt64`, stale `bun:` tags in tests

## 0.10.0 — 2026-03-29

### Breaking Changes

- **App interface simplified**: `App` now only requires `Name() string` — `Register(cfg *AppConfig) error` has been removed. All setup logic moves into `Configure(cfg *AppConfig, cmd *cli.Command) error` via the `Configurable` interface.
- **Configurable signature changed**: `Configure(cmd *cli.Command) error` → `Configure(cfg *AppConfig, cmd *cli.Command) error` — update all implementations to accept the new `cfg` parameter
- **PostConfigurable signature changed**: `PostConfigure(cmd *cli.Command) error` → `PostConfigure(cfg *AppConfig, cmd *cli.Command) error`
- **HasFlags extracted**: `Flags()` is no longer part of `Configurable` — it is now a standalone `HasFlags` interface
- **Registry.RegisterAll removed**: Use `Registry.ConfigureAll(cfg *AppConfig)` instead (calls `Configure(cfg, nil)` on each `Configurable` app)

## 0.9.0 — 2026-03-28

### Breaking Changes

- **HasRequestFuncMap**: Changed signature from `RequestFuncMap(*http.Request)` to `RequestFuncMap(context.Context)` — update all implementations by replacing `r *http.Request` with `ctx context.Context` and `r.Context()` with `ctx` ([#5](https://github.com/oliverandrich/burrow/issues/5))
- **TemplateExecutor**: Changed signature from `func(*http.Request, string, map[string]any)` to `func(context.Context, string, map[string]any)` ([#5](https://github.com/oliverandrich/burrow/issues/5))

### Added

- **`Startable` lifecycle interface** — counterpart to `HasShutdown`, called after the full boot sequence (templates built, middleware and routes registered); receives `*Server` so apps can access server resources like `TemplateExecutor()` ([#11](https://github.com/oliverandrich/burrow/issues/11))
- **jobs: automatic `TemplateExecutor` injection** — the jobs app now implements `Startable` and injects the `TemplateExecutor` into every job handler's context, enabling `RenderFragment` in background jobs without manual setup ([#11](https://github.com/oliverandrich/burrow/issues/11))
- Added `RenderFragment()` for rendering templates outside HTTP handlers (background jobs, SSE, CLI) ([#5](https://github.com/oliverandrich/burrow/issues/5))
- Added `Server.TemplateExecutor()` accessor to obtain the template executor after boot ([#5](https://github.com/oliverandrich/burrow/issues/5))
- Added `WithRequestPath()`/`RequestPath()` context helpers for request path propagation ([#5](https://github.com/oliverandrich/burrow/issues/5))
- **htmx**: Added smart response helpers `SmartRedirect` and `RenderOrRedirect` that handle htmx/non-htmx branching, plus `Reselect` header setter and `StatusStopPolling` constant ([#9](https://github.com/oliverandrich/burrow/issues/9))
- Added `URLParamInt64()` and `MustURLParamInt64()` helpers for parsing numeric URL parameters ([#8](https://github.com/oliverandrich/burrow/issues/8))
- **auth**: Added `MustCurrentUser()` helper that returns the authenticated user or panics — for use behind `RequireAuth` middleware ([#7](https://github.com/oliverandrich/burrow/issues/7))
- **sse**: Added `BrokerFromRegistry()` for package-level access to the SSE broker without type assertions ([#6](https://github.com/oliverandrich/burrow/issues/6))
- **`csrfHxHeaders` template function** — renders `hx-headers='{"X-CSRF-Token":"..."}'` as an HTML attribute when the csrf app is registered, or nothing when it is not. Use `<body {{ csrfHxHeaders }}>` for automatic CSRF protection on all htmx requests.
- **`csrfToken` / `csrfField` / `csrfHxHeaders` fallbacks** — these template functions are always available (return empty values when the csrf app is not registered), so layouts can reference them unconditionally.

### Fixed

- **Bootstrap/admin layouts: automatic CSRF token for htmx** — `bootstrap/layout`, `bootstrap/nav_layout`, and `admin/layout` now use `{{ csrfHxHeaders }}` on the `<body>` tag, so all htmx requests include the CSRF token automatically.

## 0.8.0 — 2026-03-22

### Added

- **htmx: bundle SSE extension** — `htmx/js` template now includes the [htmx SSE extension](https://htmx.org/extensions/sse/) (`htmx-ext-sse` v2.2.4) alongside htmx itself

## 0.7.4 — 2026-03-22

### Added

- **`PostConfigurable` interface** — new optional app interface for second-pass configuration after all `Configure()` calls complete; used by `contrib/jobs` to guarantee that `RegisterJobs()` runs after all apps are fully configured

### Fixed

- **Jobs: `RegisterJobs` timing** — `HasJobs.RegisterJobs()` is now called during `PostConfigure()` instead of `Configure()`, ensuring that apps can safely access state set in their own `Configure()` when registering job handlers (fixes [#4](https://github.com/oliverandrich/burrow/issues/4))

## 0.7.3 — 2026-03-22

### Changed

- **`burrow.TestDB` uses file-backed SQLite** — `TestDB(t)` now creates the database in `t.TempDir()` instead of using `file::memory:`, preventing data loss when the connection pool recycles connections
- **Removed `internal/sqlitetest`** — all internal test code now uses the public `burrow.TestDB(t)` helper

## 0.7.2 — 2026-03-21

### Fixed

- **Jobs: flaky test fix** — `sqlitetest.OpenDB` now uses a file-backed SQLite database in `t.TempDir()` instead of `file::memory:`, preventing data loss when the connection pool recycles connections

## 0.7.1 — 2026-03-21

### Security

- **Uploads: path traversal protection** — `LocalStorage.Open`, `Delete`, and `Path` now validate that the resolved path stays within the root directory; returns `ErrPathTraversal` for keys containing `..` sequences
- **Auth: timing-safe recovery login** — `RecoveryLogin` now runs a dummy bcrypt comparison when the username is not found, preventing timing-based username enumeration
- **Auth: atomic registration** — removed TOCTOU race in `RegisterBegin` by eliminating pre-insert existence checks; UNIQUE constraint violations now return a clean "registration failed" instead of 500
- **Auth: WebAuthn session store cap** — in-memory session store is now capped at 10,000 entries to prevent denial-of-service via unauthenticated endpoints
- **Jobs: atomic Retry/Cancel** — status checks moved into the UPDATE WHERE clause to prevent a race where a running job could be reset to pending and executed twice

### Fixed

- **RenderError: no more empty bodies** — added `error/default` fallback template and specific templates for 401, 422, 429 in both core and bootstrap; `RenderError` now falls back to `error/default` when the code-specific template is missing
- **`journal_size_limit` PRAGMA per-connection** — moved from one-shot `db.Exec` to `withPerConnPragmas` so it is applied to every new pool connection, not just the first
- **Signal registration cleanup** — `signalDone` now calls `signal.Stop` to clean up OS signal registration after the first signal is received
- **Uploads: extension from MIME type** — storage keys now derive the file extension from the detected MIME type instead of the attacker-controlled filename, preventing malicious extensions (e.g., uploading a JPEG as `evil.php`)
- **Auth: admin promotion uses CountAdminUsers** — first-user admin promotion now checks `CountAdminUsers == 0` instead of `CountUsers == 1`, avoiding interference from phantom users created by abandoned registration flows
- **Ratelimit: trust-proxy security warning** — added prominent documentation warning that `--ratelimit-trust-proxy` must only be enabled behind a reverse proxy that overwrites `X-Real-IP`

### Fixed

- **Dead code removed** — unused `db` field on `Registry` struct removed
- **Signal cleanup** — `signalDone` now calls `signal.Stop` after receiving the first signal

## 0.7.0 — 2026-03-21

### Breaking Changes

- **Uploads: flag names renamed** — `--upload-dir` → `--uploads-dir`, `--upload-url-prefix` → `--uploads-url-prefix`, `--upload-allowed-types` → `--uploads-allowed-types`. Environment variables changed accordingly (`UPLOAD_*` → `UPLOADS_*`). This aligns with the `{appname}-{property}` convention used by all other apps.
- **Uploads: `Storage` interface renamed to `Store`** — the context getter is now `uploads.Storage(ctx)` returning `uploads.Store`. The old `GetStorage` and `StorageFromContext` remain as deprecated wrappers.
- **Context getter renames** — `UserFromContext` → `CurrentUser`, `LogoFromContext` → `Logo`, `NavGroupsFromContext` → `NavGroups`, `RequestPathFromContext` → `RequestPath`. Old names remain as deprecated `//go:fix inline` wrappers.
- **Auth: flag prefix corrected** — `webauthn-rp-id` → `auth-webauthn-rp-id`, `webauthn-rp-display-name` → `auth-webauthn-rp-display-name`, `webauthn-rp-origin` → `auth-webauthn-rp-origin`. Old names kept as CLI aliases.
- **Auth: Renderer method renames** — `VerifyEmailSuccess` → `VerifyEmailSuccessPage`, `VerifyEmailError` → `VerifyEmailErrorPage`

### Added

- **SSE contrib app** — new `sse` app providing an in-memory pub/sub broker for Server-Sent Events; supports static and dynamic topics, non-blocking publish, configurable buffer size, 30s keepalive, graceful shutdown, and seamless htmx SSE extension integration via `ContextHandler`
- **Alpine.js contrib app** — new `alpine` app that embeds Alpine.js 3.15.8 and serves it via `staticfiles` with content-hashed URLs; include via `{{ template "alpine/js" . }}` in layout templates
- **Jobs: separate database support** — new `--jobs-database` flag to store the job queue in a dedicated SQLite file, eliminating write contention with the main application database
- **`burrow.OpenDB()`** — exported function to open SQLite databases with the framework's standard PRAGMAs; useful for contrib apps or user code that need dedicated database connections
- **`burrow.RenderContent()`** — renders pre-rendered HTML with layout wrapping and HTMX support; used by modeladmin, replaces duplicated `renderWithLayout` logic
- **Benchmarks** — 20 benchmarks for critical code paths (Handle, Render, JSON, Bind, Pagination, Template Clone)
- **`internal/sqlitetest`** — shared test helper for consistent in-memory SQLite test databases

### Changed

- **Auth repository: explicit ErrNotFound** — all Get* methods now explicitly check `sql.ErrNoRows` and return `ErrNotFound` directly, matching the jobs repository pattern
- **Deployment guide improvements** — hardened systemd unit (dedicated user, RuntimeDirectory, EnvironmentFile), added SQLite production section (file permissions, WAL sidecar files, backup strategies)
- **Error handling consistency** — fixed routing.md to correctly describe `RenderError` behavior; added `ValidationError` documentation to error handling guide
- **Comprehensive documentation improvements** — quickstart clarifications, middleware authoring example, template error behavior, navLinks explanation, migration rollback strategy, TOML config path, ACME rate limits, logging configuration, GoReleaser Homebrew Cask guidance
- **Building Releases guide** — new documentation page covering local builds, cross-compilation with GoReleaser, automated GitHub releases, version injection, and CI workflows
- **Multi-Tenant guide** — new documentation page explaining database-per-user architecture with `burrow.OpenDB()`

### Fixed

- **Empty context.go files removed** — package doc comments moved from `secure/context.go` and `jobs/context.go` to their respective `app.go` files

## 0.6.0 — 2026-03-21

### Breaking Changes

- **ModelAdmin: `Renderer.ConfirmDelete` signature changed** — `ConfirmDelete(w, r, item *T, cfg)` is now `ConfirmDelete(w, r, ids []string, impacts []CascadeImpact, cfg)`. Custom renderers must update their implementation.
- **ModelAdmin: `DeleteImpacts` removed from `RenderConfig`** — cascade impacts are now passed directly to `ConfirmDelete` instead of being embedded in `RenderConfig`.
- **ModelAdmin: delete routes changed** — `GET /{id}/delete` and `DELETE /{id}` replaced by unified `POST /bulk/delete` (confirm page) and `DELETE /bulk/delete` (execute deletion). Both accept `_selected` form parameter with one or more IDs.

### Added

- **ModelAdmin: `ConfirmPage` on `BulkAction`** — bulk actions with `ConfirmPage: true` redirect to a confirm page instead of using a JS `confirm()` dialog; the built-in `DeleteBulkAction` uses this by default
- **Admin layout: `hx-boost` on `<body>`** — replaces manual `hx-get`/`hx-target`/`hx-push-url` on every link; all navigation and form submissions are automatically boosted to swap `<main>` without full page reloads
- **`AppConfig.RegisterIconFunc()`** — apps register icon template functions in their `Register()` method via `cfg.RegisterIconFunc("iconName", bsicons.IconFunc)`; duplicate registrations are silently ignored, allowing multiple apps to depend on the same icon without collisions
- **Bootstrap color themes** — three Sass-compiled color themes (blue, purple, gray) selectable via `bootstrap.WithColor()`; default is purple; vanilla Bootstrap available via `bootstrap.WithColor(bootstrap.Default)`
- **`bootstrap.NavLayout()`** — layout template with empty `bootstrap/navbar`, `bootstrap/alerts`, and `bootstrap/nav_scripts` slots that apps can override via `HasTemplates`
- **Extended spacing utilities** — spacing scale extended with levels 6 (4.5rem), 7 (6rem), and 8 (9rem)
- **`bootstrap.WithCustomCSS()`** — option to use a custom Sass-compiled CSS file instead of the built-in themes
- **`burrow.PageURL()` helper** — builds pagination URLs that preserve existing query parameters (search, filters) while setting the page number; useful for any paginated view that needs to retain query state across page navigation
- **ModelAdmin: search input** — admin list views with `SearchFields` now show a search input with HTMX support; uses FTS5 when available, falls back to LIKE
- **`add`/`sub` template functions in core** — integer arithmetic available in all templates without contrib app registration
- **Core htmx template stubs** — `htmx/js` and `htmx/config` defined as empty stubs in core; htmx app overrides them when registered
- **Sass build pipeline** — `just sass` compiles themes, `just sass-setup` installs Bootstrap Sass source; pre-commit hook auto-compiles on `.scss` changes
- **Alpine.js contrib app** — new `alpine` app that embeds Alpine.js 3.15.8 and serves it via `staticfiles` with content-hashed URLs; include via `{{ template "alpine/js" . }}` in layout templates

### Changed

- **Building Releases guide** — new documentation page covering local builds, cross-compilation with GoReleaser, automated GitHub releases, version injection, and CI workflows
- **Pagination helpers moved to core** — `pageURL`, `pageNumbers` are now core template functions (registered in `baseFuncMap`); `PageResult.PageSize()` method replaces the old `pageLimit` template function; `bootstrap/pagination` template now uses `BasePath`/`RawQuery` instead of `BaseURL` for query-preserving pagination links
- **`themeCSS` template function removed** — the CSS path is now baked into the `bootstrap/css` template at boot time via an overlay FS; this removes a potential FuncMap collision point for alternative CSS framework apps
- **Icon template functions moved to `RegisterIconFunc`** — apps no longer register icon functions via `FuncMap()`; instead they call `cfg.RegisterIconFunc()` in `Register()`, which prevents FuncMap collisions when multiple apps use the same icons
- **Bootstrap layout simplified** — `bootstrap/layout` no longer wraps content in `<main class="container">`; apps control their own container and structure
- **Auth layout removed** — `auth/layout` template removed; auth pages now use `bootstrap/layout` directly
- **Notes example uses `bootstrap.NavLayout()`** — app-specific layout replaced by framework-provided nav layout with slot overrides
- **Notes homepage redesign** — full-width hero section with primary color background, shadow cards

### Fixed

- **Auth middleware: HTMX-aware login redirect** — when a session expires during HTMX navigation (e.g. in the admin), the auth middleware now uses `HX-Redirect` to force a full page navigation to the login page instead of swapping it into `<main>`

## 0.5.0 — 2026-03-15

### Breaking Changes

- **Forms: `Cleanable.Clean()` signature changed** — `Clean() error` is now `Clean(ctx context.Context) error`; existing implementations must add the `context.Context` parameter
- **Auth: `isAdminEditSelf` and `isAdminEditLastAdmin` template functions removed** — the custom user admin detail page is replaced by generic ModelAdmin forms; `IsAdminEditSelf()`, `IsAdminEditLastAdmin()`, and `emailValue` are no longer available
- **Auth: `User` model form tags changed** — `Email`, `Username`, `IsActive`, `Name`, `Bio`, and `Role` now have explicit `form` tags instead of `form:"-"`; code that relied on these fields being excluded from forms must use `WithExclude` or `WithReadOnly`
- **Pagination: cursor-based pagination removed** — `ApplyCursor()`, `TrimCursorResults()`, `CursorResult()`, `PageRequest.Cursor`, `PageResult.NextCursor`, and `PageResult.PrevCursor` have been removed; use `ApplyOffset()` + `OffsetResult()` for all pagination
- **`Render()` renamed** — `RenderTemplate()` is now `Render()`; the old `Render(w, r, code, template.HTML)` raw-HTML wrapper has been removed (use `HTML()` directly); `RenderTemplate()` remains as a deprecated shim with `//go:fix inline` for automatic migration via `go fix` (Go 1.26+)
- **`TemplateExec()` renamed** — `TemplateExecutorFromContext()` is now `TemplateExec()`; the old name remains as a deprecated shim with `//go:fix inline`
- **Minimum Go version raised to 1.26** — required for `reflect.Type.Fields` iterator, `sync.WaitGroup.Go`, and future use of `testing.B.Loop` and `net/http.CrossOriginProtection`
- **`RequireAdmin()` behavior changed** — unauthenticated users are now redirected to `/auth/login` (like `RequireAuth`); previously returned plain-text 403 for all denied users

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
- **Error pages with `RenderError()`** — new `RenderError(w, r, code, message)` renders `error/{code}` templates through the standard `Render` pipeline with i18n-translated messages, layout wrapping, and HTMX fragment support; JSON responses for `Accept: application/json`; default templates for 403, 404, 405, 500 shipped as embedded FS (override by defining `{{ define "error/404" }}` in any app's templates)
- **Chi `NotFound` and `MethodNotAllowed` handlers** — unmatched routes and wrong HTTP methods now render styled error pages instead of plain text
- **Test helpers: `TestDB`, `TestErrorExecContext`, `TestErrorExecMiddleware`** — shared test helpers in the root package for framework and app tests

### Removed

- **Auth: soft-delete removed from all models** — `deleted_at` columns and `bun:",soft_delete"` tags removed from User, Credential, RecoveryCode, EmailVerificationToken, and Invite; all deletes are now permanent; includes migration `004_drop_soft_delete`

### Fixed

- **SQLite PRAGMAs now applied per-connection** — per-connection PRAGMAs (foreign_keys, busy_timeout, synchronous, etc.) are now set via `_pragma` DSN parameters instead of one-shot `db.Exec()` calls, ensuring they are active on every connection in the pool; this fixes `ON DELETE CASCADE` not firing when a different pool connection handled the DELETE
- **ModelAdmin: ambiguous column name with relations** — `getItem`, list, search (FTS5 and LIKE), and filter queries now qualify column names with `?TableAlias` to prevent SQLite "ambiguous column name" errors when eager-loading relations that share column names (e.g. `id`, `created_at`)
- **ModelAdmin: pagination preserves query parameters** — pagination links now retain search terms, filters, and sort parameters when navigating between pages
- **ModelAdmin: `applySort` pointer comparison** — user-requested column sorting was silently overridden by the default `OrderBy` because `applySort` returned the same pointer; now returns `(query, bool)` for reliable detection
- **Admin sidebar: active-link prefix matching** — `path.indexOf(url) === 0` replaced with boundary-aware `startsWith(url + "/")` to prevent false matches when model slugs share a prefix (e.g. `/admin/user` and `/admin/user-role`)
- **ModelAdmin: delete/bulk-delete preserves current page** — after deleting items, the redirect now returns to the current page (clamped to the last available page) instead of always going back to page 1
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
- New guide: *Coming from Django*, mapping Django concepts to Burrow equivalents with side-by-side code examples.
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
