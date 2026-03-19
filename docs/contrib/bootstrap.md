# Bootstrap

Design system using [Bootstrap 5](https://getbootstrap.com/) and [htmx](https://htmx.org/). Ships three color themes (blue, purple, gray) compiled from Sass, a base layout, a navbar layout with overridable slots, and a dark mode theme switcher. For icons, see [`bsicons`](bsicons.md). For htmx helpers, see [`htmx`](htmx.md).

**Package:** `github.com/oliverandrich/burrow/contrib/bootstrap`

**Depends on:** `staticfiles`, `htmx`

## Setup

```go
srv := burrow.NewServer(
    session.New(),
    csrf.New(),
    auth.New(),
    bootstrap.New(),                    // purple theme, htmx enabled (defaults)
    htmx.New(),
    healthcheck.New(),
    admin.New(),
    staticApp,
)
```

### Options

```go
// Use a different color theme
bootstrap.New(bootstrap.WithColor(bootstrap.Blue))
bootstrap.New(bootstrap.WithColor(bootstrap.Gray))
bootstrap.New(bootstrap.WithColor(bootstrap.Default)) // vanilla Bootstrap

```

## Layouts

The app provides two layouts:

### Base Layout (`bootstrap/layout`)

A minimal HTML shell with themed CSS, Bootstrap JS, theme script, and htmx. Content is rendered directly in `<body>` — apps control their own `<main>`, containers, etc.

```go
bootstrap.Layout() // returns "bootstrap/layout"
```

This is the default layout injected by middleware when no other layout is set.

### Nav Layout (`bootstrap/nav_layout`)

Extends the base layout with overridable navbar and alerts slots:

```go
srv.SetLayout(bootstrap.NavLayout()) // returns "bootstrap/nav_layout"
```

The nav layout renders:

1. `{{ template "bootstrap/navbar" . }}` — empty by default, override in your app
2. `{{ template "bootstrap/alerts" . }}` — empty by default, override for flash messages
3. `{{ .Content }}` — page content
4. `{{ template "bootstrap/nav_scripts" . }}` — HTMX active-link script (overridable)

To use it, define `bootstrap/navbar` and optionally `bootstrap/alerts` in your app's templates:

```html
{{ define "bootstrap/navbar" -}}
<nav class="navbar navbar-expand-lg bg-body sticky-top">
    <div class="container">
        <a class="navbar-brand" href="/">My App</a>
        <!-- your navbar content -->
    </div>
</nav>
{{- end }}

{{ define "bootstrap/alerts" -}}
<div id="alerts">
    {{ range messages -}}
    <div class="alert alert-{{ alertClass .Level }} alert-dismissible fade show" role="alert">
        {{ .Text }}
        <button type="button" class="btn-close" data-bs-dismiss="alert" aria-label="Close"></button>
    </div>
    {{- end }}
</div>
{{- end }}
```

## Middleware Behaviour

The bootstrap middleware injects `bootstrap/layout` **only when no layout is already set** in the request context:

- `srv.SetLayout(bootstrap.NavLayout())` is called — nav layout wins
- `srv.SetLayout()` is NOT called — base layout takes effect
- Admin `/admin` route group always overrides unconditionally

## Color Themes

Three Sass-compiled themes are available, each setting a different primary color while sharing common overrides (border radius, spacing scale, navbar padding, link decoration):

| Theme | Primary Color | Constant | Description |
|-------|--------------|----------|-------------|
| Purple (default) | `#6f42c1` | `bootstrap.Purple` | Burrow theme with purple primary |
| Blue | `#0d6efd` | `bootstrap.Blue` | Burrow theme with Bootstrap's blue |
| Gray | `#6c757d` | `bootstrap.Gray` | Burrow theme with neutral gray |
| Default | — | `bootstrap.Default` | Vanilla Bootstrap, no customization |

All Burrow themes share common overrides: tighter border radius, extended spacing scale (6–8), more navbar padding, and no link underlines. Use `Default` for unmodified Bootstrap.

Themes are compiled from `contrib/bootstrap/scss/` using `just sass`.

### Custom Theme

You can build your own Sass theme and use it instead of the built-in ones:

1. Install Bootstrap Sass source and the Dart Sass compiler:

    ```bash
    npm install --save-dev bootstrap@5.3.8
    brew install sass/sass/sass  # or see https://sass-lang.com/install/
    ```

2. Create your theme file (e.g. `scss/mytheme.scss`):

    ```scss
    // Variable overrides (before Bootstrap import)
    $primary: #e91e63;
    $min-contrast-ratio: 3;
    $border-radius: 0.25rem;

    // Import Bootstrap
    @import "../node_modules/bootstrap/scss/bootstrap";
    ```

3. Compile it:

    ```bash
    sass scss/mytheme.scss static/mytheme.min.css --style=compressed --no-source-map
    ```

4. Embed the CSS in your app's static files and reference it:

    ```go
    bootstrap.New(bootstrap.WithCustomCSS("myapp/mytheme.min.css"))
    ```

The custom CSS path is resolved via `staticURL`, so the file must be served by a registered `staticfiles` app.

## Templates

| Template | Description |
|----------|-------------|
| `bootstrap/css` | `<link>` tag for the selected color theme CSS |
| `bootstrap/js` | `<script defer>` tag for Bootstrap JS bundle |
| `bootstrap/layout` | Base HTML page shell |
| `bootstrap/nav_layout` | Layout with navbar and alerts slots |
| `bootstrap/navbar` | Empty navbar slot (override in your app) |
| `bootstrap/alerts` | Empty alerts slot (override in your app) |
| `bootstrap/nav_scripts` | HTMX active-link script for navbar (overridable) |
| `bootstrap/pagination` | Offset-based pagination nav component |
| `bootstrap/theme_script` | Inline script for dark mode persistence |
| `bootstrap/theme_switcher` | Theme toggle button component |

## Static Files

| File | Description |
|------|-------------|
| `bootstrap.min.css` | Vanilla Bootstrap 5 CSS |
| `bootstrap.bundle.min.js` | Bootstrap 5 JS bundle (includes Popper) |
| `theme-blue.min.css` | Blue color theme (Sass-compiled) |
| `theme-purple.min.css` | Purple color theme (Sass-compiled) |
| `theme-gray.min.css` | Gray color theme (Sass-compiled) |

## Icon Registration

The bootstrap app registers the following icons via `cfg.RegisterIconFunc()` in its `Register()` method:

- `iconSunFill`, `iconMoonStarsFill`, `iconCircleHalf` — used by the theme switcher

## Extended Spacing

The theme extends Bootstrap's spacing scale (0–5) with three additional levels:

| Level | Size |
|-------|------|
| 6 | 4.5rem |
| 7 | 6rem |
| 8 | 9rem |

Use with standard Bootstrap spacing utilities: `py-6`, `mb-7`, `mt-8`, etc.

## Sass Build

Themes are compiled from Sass. To rebuild after changing `contrib/bootstrap/scss/*.scss`:

```bash
just sass        # compile all themes
just sass-setup  # install Bootstrap Sass source (run once after clone)
```

A pre-commit hook automatically recompiles when `.scss` files change.

## Dark Mode

The layout includes a theme switcher toggle that persists the user's preference in `localStorage`. It uses Bootstrap's `data-bs-theme` attribute to switch between light and dark modes.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasStaticFiles` | Contributes embedded Bootstrap assets under `"bootstrap"` prefix |
| `HasMiddleware` | Injects bootstrap layout when no layout is set in context |
| `HasTemplates` | Contributes layouts, pagination, and theme templates |
| `HasFuncMap` | Contributes icon, theme, and utility template functions |
| `HasDependencies` | Requires `staticfiles` and `htmx` |
