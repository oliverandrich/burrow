# Bootstrap

Swappable design system using [Bootstrap 5](https://getbootstrap.com/) and [htmx](https://htmx.org/). Provides static assets and a base HTML layout for all pages. For icons, see [`bsicons`](bsicons.md).

**Package:** `codeberg.org/oliverandrich/burrow/contrib/bootstrap`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(authRenderer),
    bootstrap.New(),                    // provides base layout + Bootstrap/htmx assets
    healthcheck.New(),
    admin.New(admintpl.Layout(), admintpl.DefaultDashboardRenderer()),
    staticfiles.New(myStaticFS),
)
```

The `bootstrap` app must be registered before apps that reference its assets (like `admintpl.Layout()`). The `staticfiles` app must also be registered to serve the embedded CSS and JS.

## Layout

`Layout()` returns a `LayoutFunc` that renders a base HTML page with Bootstrap CSS, Bootstrap JS bundle (includes Popper), and htmx:

```go
bootstrap.Layout() // returns burrow.LayoutFunc
```

The layout renders a responsive container — no navigation, no sidebar. It is intended as a clean base for user-facing pages like login, register, and standalone forms.

## Middleware Behavior

The bootstrap middleware injects its layout **only when no layout is already set** in the request context:

- `srv.SetLayout(custom)` is called → custom layout wins, bootstrap skips
- `srv.SetLayout()` is NOT called → bootstrap layout takes effect
- Admin `/admin` route group always overrides unconditionally

This makes bootstrap batteries-included by default without fighting custom layouts.

## Static Files

The bootstrap app embeds these static assets and implements `HasStaticFiles` to contribute them under the `"bootstrap"` prefix:

| File | Description |
|------|-------------|
| `bootstrap.min.css` | Bootstrap 5 CSS |
| `bootstrap.bundle.min.js` | Bootstrap 5 JS bundle (includes Popper) |
| `htmx.min.js` | htmx library for progressive enhancement |

These are served at `/static/bootstrap/bootstrap.min.css`, etc. when the `staticfiles` app is registered.

## Pagination Component

The `templates` sub-package provides a reusable Bootstrap 5 pagination nav for offset-based pagination:

```go
import bstpl "codeberg.org/oliverandrich/burrow/contrib/bootstrap/templates"
```

```templ
@bstpl.Pagination(page, "/admin/notes")
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `page` | `burrow.PageResult` | Pagination metadata from `burrow.OffsetResult()` |
| `baseURL` | `string` | Path without query params (e.g., `/admin/notes`) |

**Behavior:**

- Renders nothing when `TotalPages <= 1` (single page or empty)
- Shows previous/next buttons (`«`/`»`) with disabled state on first/last page
- Displays page numbers with ellipsis (`…`) when there are more than 7 pages
- Current page highlighted with Bootstrap's `active` class
- Links include `?page=N&limit=N` query parameters

For full pagination documentation, see the [Pagination Guide](../guide/pagination.md).

## Swapping Design Systems

The `bootstrap` package is intentionally self-contained. To use a different CSS framework, create a new contrib app (e.g., `contrib/pico` or `contrib/tailwind`) that provides its own assets and layout, and register it instead of `bootstrap.New()`.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasStaticFiles` | Contributes embedded Bootstrap + htmx assets under `"bootstrap"` prefix |
| `HasMiddleware` | Injects bootstrap layout when no layout is set in context |
