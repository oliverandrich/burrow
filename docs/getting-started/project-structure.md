# Project Structure

A recommended directory layout for applications using the framework.

## Minimal

```
myapp/
├── main.go           # Server setup and app registration
├── go.mod
└── go.sum
```

For small projects, everything fits in a single `main.go`.

## Recommended

```
myapp/
├── cmd/
│   └── server/
│       └── main.go           # Server setup, app registration, layouts
├── internal/
│   ├── notes/                # Larger app (split by purpose)
│   │   ├── app.go            # App struct, routes, framework wiring
│   │   ├── context.go        # Context helpers
│   │   ├── handlers.go       # HTTP handlers
│   │   ├── models.go         # Domain models
│   │   ├── repository.go     # Data access layer
│   │   ├── templates/        # HTML template files
│   │   │   ├── list.html     # {{ define "notes/list" }}
│   │   │   └── detail.html   # {{ define "notes/detail" }}
│   │   └── migrations/
│   │       └── 001_create_notes.up.sql
│   └── pages/                # Small app (single file is fine)
│       └── app.go
├── templates/                # Shared layout templates
│   └── layout.html           # {{ define "app/layout" }}
├── static/                   # CSS, JS, images
│   ├── styles.css
│   └── app.js
├── config.toml               # Optional TOML config
├── go.mod
└── go.sum
```

## Key Conventions

**Apps live in `internal/`** — each app is a self-contained package with its own model, repository, handlers, and migrations.

**One file per app is fine** — for small apps, put everything in `app.go`. Larger apps split files by purpose: `context.go`, `handlers.go`, `models.go`, `repository.go`, etc. (see the [notes example](../guide/creating-an-app.md)).

**Templates are `.html` files** — each app embeds its own templates via `//go:embed templates/*.html` and implements `HasTemplates`. Templates use `{{ define "appname/templatename" }}` to namespace them within the global template set.

**Migrations are embedded** — each app embeds its own SQL files with `//go:embed migrations`. The framework runs them automatically at startup.

**Layouts are separate from apps** — layout templates live at the project level since they're shared across all apps. Set them via `srv.SetLayout()` in `main.go`, or use the [`bootstrap` contrib app](../contrib/bootstrap.md) which provides a ready-made layout.

**Static files are optional** — use the [staticfiles contrib app](../contrib/staticfiles.md) if you need content-hashed URLs and cache headers.
