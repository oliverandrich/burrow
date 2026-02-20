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
│   ├── notes/                # Custom app
│   │   ├── notes.go          # Model, repository, handlers, App struct
│   │   └── migrations/
│   │       └── 001_create_notes.up.sql
│   └── pages/                # Another custom app
│       ├── pages.go
│       └── migrations/
│           └── 001_create_pages.up.sql
├── templates/                # Templ layout templates
│   ├── layouts/
│   │   ├── app.templ
│   │   └── admin.templ
│   └── auth/                 # Auth renderer templates
│       ├── login.templ
│       └── register.templ
├── static/                   # CSS, JS, images
│   ├── styles.css
│   └── app.js
├── config.toml               # Optional TOML config
├── go.mod
└── go.sum
```

## Key Conventions

**Apps live in `internal/`** — each app is a self-contained package with its own model, repository, handlers, and migrations.

**One file per app is fine** — for small apps, put everything in a single `.go` file (see the [notes example](../guide/creating-an-app.md)).

**Migrations are embedded** — each app embeds its own SQL files with `//go:embed migrations`. The framework runs them automatically at startup.

**Layouts are separate from apps** — layout templates live at the project level since they're shared across all apps. Pass them to `srv.SetLayouts()` in `main.go`.

**Static files are optional** — use the [staticfiles contrib app](../contrib/staticfiles.md) if you need content-hashed URLs and cache headers.
