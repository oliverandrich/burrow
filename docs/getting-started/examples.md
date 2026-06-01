# Examples & Tutorial

Burrow ships with two example applications and a step-by-step tutorial. Each one is a complete, runnable project — read the source to see how the framework patterns fit together in practice.

## Hello World

**Source:** [`example/hello/`](https://github.com/oliverandrich/burrow/tree/main/example/hello)

A single-file application (~150 lines) that serves a "Hello, World!" page with Tailwind v4 styling, `prefers-color-scheme` dark mode, and i18n support. Start here to understand the absolute minimum needed for a burrow app.

**What you can learn from it:**

- The `burrow.App` interface — `Name()`, and optional interfaces (`HasRoutes`, `HasTemplates`, `HasTranslations`, `HasFuncMap`, `Configurable`)
- Server setup with `burrow.NewServer()` and `srv.SetLayout()`
- Rendering pages with `burrow.Render()`
- Embedding templates and translations with `//go:embed`
- CLI wiring with `urfave/cli`

```bash
go run ./example/hello
# → http://localhost:8080
```

## Notes

**Source:** [`example/notes/`](https://github.com/oliverandrich/burrow/tree/main/example/notes)

A full-featured notes application with authentication, an admin panel, and HTMX-powered interactions. This is the closest thing to a real-world burrow app and demonstrates how multiple contrib apps work together.

**What you can learn from it:**

- **Multi-app architecture** — session, CSRF, auth, admin, jobs, messages, htmx, and humanize all wired together in `main.go`
- **Repository pattern** with Den/SQLite — CRUD operations, offset-based pagination
- **Declarative JSON API** — `/api/notes` built with the [`crud` package](../guide/crud-api.md): an owner-scoped resource with a write DTO, authenticated by API-key bearer tokens (managed at the self-service `/auth/api-keys` page) alongside the session-based HTML UI
- **FTS5 full-text search** — migration with triggers, `SearchByUserID` repository method, HTMX search form
- **Admin views** — hand-written admin handlers with search, pagination, and htmx actions
- **Custom layout** — navbar with navigation items, icon helpers, theme switcher
- **HTMX patterns** — infinite scroll (`hx-trigger="revealed"`), form submission with OOB swap, search with `hx-push-url`
- **i18n** — English and German translations, translated form labels and flash messages

```bash
go run ./example/notes/cmd/server
# → http://localhost:8080
```

## Tutorial: Polls App

**Source:** [`tutorial/`](https://github.com/oliverandrich/burrow/tree/main/tutorial) (one directory per step)

A guided, seven-part tutorial that builds a survey/voting application from scratch. Each part has its own complete, compilable project in `tutorial/stepNN/`.

| Part | Topic | Key Concepts |
|------|-------|--------------|
| [Part 1](../tutorial/part1.md) | Setup & First View | Project scaffolding, `HandlerFunc`, server lifecycle |
| [Part 2](../tutorial/part2.md) | Database & Models | `burrow.App` interface, Den/SQLite, documents |
| [Part 3](../tutorial/part3.md) | Templates & Layouts | `html/template`, layouts, `Render` |
| [Part 4](../tutorial/part4.md) | Forms, CRUD & Validation | Form binding, CSRF, flash messages, validation errors |
| [Part 5](../tutorial/part5.md) | Authentication | Auth middleware, user context, passkeys |
| [Part 6](../tutorial/part6.md) | Admin Panel | `HasAdmin` interface, admin routes, admin nav |
| [Part 7](../tutorial/part7.md) | HTMX, Charts & Pagination | htmx helpers, Chart.js, offset-based infinite scroll |

Unlike the examples (which show final results), the tutorial shows the *process* — how to start from zero and incrementally add features. If you're new to burrow, start with the tutorial and use the examples as reference for patterns not covered there.

```bash
cd tutorial/step07
go run .
# → http://localhost:8080
```
