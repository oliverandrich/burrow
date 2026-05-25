# Tailwind CSS

Burrow projects style their HTML with [Tailwind CSS](https://tailwindcss.com/) via the **standalone Rust CLI** — no npm, no PostCSS, no plugins. The CLI is pinned and installed via [mise](https://mise.jdx.dev/) alongside the Go toolchain.

Dark mode follows the browser's `prefers-color-scheme` setting; no user-toggleable theme machinery is shipped by Burrow. If you want a toggle, write it yourself.

!!! info "Coming from `cmd/burrow-tailwind`?"
    The standalone `cmd/burrow-tailwind` tool was removed in v0.22 (after a one-cycle deprecation shim in v0.21). Replace the `tool` directive in `go.mod` and the invocations in `.mise.toml` / `.air.toml`:

    ```
    - tool github.com/oliverandrich/burrow/cmd/burrow-tailwind
    + tool github.com/oliverandrich/burrow/cmd/burrow
    ```

    ```
    - go tool burrow-tailwind -i tailwind.css -o ...
    + go tool burrow tailwind -i tailwind.css -o ...
    ```

## What you ship

A realistic Burrow project keeps the binary in `cmd/server/` and every app — including the project shell that owns the shared layout — under `internal/`. There is no project-root package:

```
myapp/
├── go.mod
├── .mise.toml                    # pins Go + tailwindcss
├── .gitignore                    # /.tailwind/, bin/
├── tailwind.css                  # @import tailwindcss + @import sources.css
│
├── cmd/server/
│   └── main.go                   # server config + app registration
│
└── internal/
    ├── app/                      # shell app: layout + project static assets
    │   ├── app.go                # HasTemplates + HasStaticFiles
    │   ├── templates/app/layout.html
    │   └── static/app.min.css    # built CSS, embedded
    ├── pages/                    # small app
    │   ├── app.go
    │   └── templates/pages/home.html
    └── notes/                    # CRUD app split by purpose
        ├── app.go, handlers.go, models.go, repository.go
        └── templates/notes/list.html
```

A flat single-file project (one `main.go`, one `templates/`, one `static/` at the root) is also fine for demos and prototypes. Both layouts use the same `tailwind.css` and the same toolchain — `burrow tailwind` auto-discovers either one (see below).

## Why the shell is in `internal/app/`

Go's `//go:embed` directive can only descend from the file containing it — `../../templates` is not allowed. Two consequences flow from that constraint:

- **Every embedded asset lives inside the package that owns it.** Project-root `templates/` and `static/` directories would either need a root-level package (an extra import, an extra file) or have to move into some package's subtree. Putting them inside `internal/app/` keeps the project root flat and treats the shell as a regular Burrow app.
- **`main.go` stays thin.** No `//go:embed` directives, no asset wiring. Just `staticfiles.New(emptyFS)` for the framework's static root, then `srv := burrow.NewServer(staticApp, htmx.New(), app.New(), pages.New(), notes.New())`.

The shell's `app.go`:

```go
package app

import (
    "embed"
    "io/fs"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

func New() *App { return &App{} }

type App struct{}

func (a *App) Name() string { return "app" }

func (a *App) TemplateFS() fs.FS {
    sub, _ := fs.Sub(templateFS, "templates")
    return sub
}

// StaticFS serves the embedded `static/` under the URL prefix `/static/app/`.
// `fs.Sub` strips the `static/` directory from the embedded paths so the
// actual file resolves at `app.min.css`, served as `/static/app/<hash>.app.min.css`.
func (a *App) StaticFS() (string, fs.FS) {
    sub, _ := fs.Sub(staticFS, "static")
    return "app", sub
}
```

The layout template then references the CSS as:

```html
<link rel="stylesheet" href="{{ staticURL "app/app.min.css" }}">
```

The `app/` prefix mirrors how every Burrow contrib's assets are namespaced (htmx serves at `/static/htmx/...`, admin at `/static/admin/...`, etc.).

## Quick start

!!! tip "Or: scaffold with `burrow new`"
    The six steps below produce — by hand — the same layout that ships pre-wired in `burrow new`: `cmd/<name>/`, `internal/app/` shell, `.mise.toml` with the toolchain pinned, `.air.toml` integration, a buildable `tailwind.css`. For a fresh project, run `go install github.com/oliverandrich/burrow/cmd/burrow@latest && burrow new myapp --module github.com/you/myapp`; see [Tooling](../getting-started/tooling.md) for all flags. Reading the steps below is still worthwhile to understand *what* the scaffold wires up.

### 1. Pin tooling with mise

```toml
# .mise.toml
[tools]
go = "1.26"
"github:tailwindlabs/tailwindcss" = "4"
```

```bash
mise install
```

### 2. Add `burrow` as a Go tool

```bash
go get -tool github.com/oliverandrich/burrow/cmd/burrow
```

### 3. Create your `tailwind.css`

```css
@import "tailwindcss";
@import "./.tailwind/sources.css";
```

Two lines. The `./.tailwind/sources.css` import is regenerated on every `burrow tailwind` invocation; it carries `@source` directives for both Burrow's contribs and your project's own templates. **You don't list contribs or internal apps manually.**

### 4. Add `.tailwind/` and the build output to `.gitignore`

```
.tailwind/
```

Whether you also gitignore `internal/app/static/app.min.css` depends on your deployment style (see Production build below).

### 5. Build CSS

```bash
go tool burrow tailwind -i tailwind.css -o internal/app/static/app.min.css --minify
```

### 6. Link it from your layout

```html
<link rel="stylesheet" href="{{ staticURL "app/app.min.css" }}">
```

## What `burrow tailwind` does

`burrow tailwind` is a thin sub-command that wraps `tailwindcss`. On every invocation it writes `.tailwind/sources.css` with `@source "<absolute path>";` lines for:

1. Every `<burrow>/contrib/<app>/templates/` directory (resolved via `go list -m`).
2. The project's `./templates/` if it exists (flat layout).
3. Every `./internal/<app>/templates/` directory (project's structured layout).

Then it forwards every argument verbatim to `tailwindcss`. New contribs in future Burrow releases — or new internal apps in your own project — get picked up automatically on the next invocation. No per-project source-list maintenance.

The two examples in this repo show both layouts in action: [`example/hello`](https://github.com/oliverandrich/burrow/tree/main/example/hello) is the flat single-file app; [`example/notes`](https://github.com/oliverandrich/burrow/tree/main/example/notes) is the realistic `cmd/server` + `internal/<app>/` layout with a Den-backed CRUD use case.

## Customizing contrib styles

Tailwind utility classes are baked into each contrib's templates (e.g. `contrib/admin/templates/dashboard.html` uses `class="rounded-lg bg-white shadow"`). To change how a contrib looks:

- **Color tweaks via theme tokens** — your `tailwind.css` can override Tailwind's design tokens with `@theme`:

    ```css
    @import "tailwindcss";
    @import "./.tailwind/sources.css";

    @theme {
        --color-blue-500: oklch(0.65 0.18 250);  /* your brand blue */
        --font-sans: "Inter", system-ui, sans-serif;
    }
    ```

    Every contrib using `bg-blue-500`, `text-blue-500` etc. picks up the new value.

- **Structural overrides** — redefine the contrib's template name in your own `internal/<app>/templates/` directory. Burrow's `html/template` uses last-define-wins, but **registration order matters**: your override-app must be registered *after* the contrib it overrides, either by listing it later in `burrow.NewServer(...)` or by declaring the contrib as a `Dependencies()` of your app. See [Layouts → Overriding Contrib Templates](layouts.md#overriding-contrib-templates) for the full pattern with worked examples.

## Development workflow with `burrow dev`

`burrow dev` watches the project, rebuilds CSS with `burrow tailwind`, and restarts the Go app on every change — sequentially, in one process.

```toml
# .mise.toml
[tools]
go = "1.26"
"github:tailwindlabs/tailwindcss" = "4"

[tasks.dev]
description = "Run app with hot-reload (burrow dev)"
run = "go tool burrow dev"
```

```bash
mise run dev
```

On any `.go`, `.html`, `.css`, `.toml`, `.yml`, or `.yaml` edit inside the project:

1. `burrow dev` SIGTERMs the running app.
2. It re-runs `burrow tailwind` once (no `--watch`), regenerating `internal/app/static/app.min.css`.
3. It re-runs `go run`. Go rebuilds the binary, which re-embeds the freshly-written CSS via `//go:embed`, then starts.

The Tailwind step is *sequential, not parallel*: a long-running `tailwindcss --watch` would write the CSS to disk while the running Go binary still serves the previously-embedded copy — `//go:embed` is a compile-time directive. Only a Go rebuild picks up new CSS, so there's no point running Tailwind in `--watch` mode alongside the app. Go's build cache keeps warm restarts fast.

Auto-discovery resolves the entry-point (single `cmd/<name>/main.go`) and the Tailwind paths (`tailwind.css` plus the single `internal/<app>/static/`). Override with `--app`, `--css-in`, `--css-out`, or disable Tailwind via `--no-tailwind`.

> Note: `.tailwind/sources.css` is regenerated on every `burrow tailwind` run. A new Burrow contrib that lands via `go get -u burrow` is picked up on the next file-change rebuild — same for new internal apps you add.

## Production build

```bash
go tool burrow tailwind -i tailwind.css -o internal/app/static/app.min.css --minify
```

The output is minified. Combined with `contrib/staticfiles` content-hashing and `Cache-Control: immutable`, the CSS is downloaded once per release.

Whether to **commit `internal/app/static/app.min.css`** depends on your deployment style:

- **Commit it** — convenient for `go run ./cmd/server` to work without first invoking the build. Right call for example apps and demos.
- **Generate at CI/build time** — leaner repo, no risk of stale committed CSS. Right call for production projects with a real build pipeline.

## See also

- [Tailwind CSS documentation](https://tailwindcss.com/docs)
- [contrib/staticfiles](../contrib/staticfiles.md) — content-hashing of CSS and other assets
- Starter `tailwind.css`: [`docs/guide/tailwind.example.css`](https://github.com/oliverandrich/burrow/blob/main/docs/guide/tailwind.example.css)
