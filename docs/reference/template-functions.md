# Template Functions

This page lists all template functions available in Burrow templates. Functions come from two sources:

- **Static** (`HasFuncMap`) — registered at parse time, available in all templates
- **Request-scoped** (`HasRequestFuncMap`) — cloned per request, can access the current request context

## Core Functions

Provided by the framework itself. Always available.

**Static:**

| Function | Example | Description |
|----------|---------|-------------|
| `safeHTML` | `{{ safeHTML .RawHTML }}` | Mark a string as safe HTML (no escaping) |
| `safeURL` | `{{ safeURL .Link }}` | Mark a string as safe URL |
| `safeAttr` | `{{ safeAttr .Attr }}` | Mark a string as safe HTML attribute |
| `itoa` | `{{ itoa .ID }}` | Convert `int64` to string |
| `add` | `{{ add .Page 1 }}` | Integer addition |
| `sub` | `{{ sub .Total 1 }}` | Integer subtraction |
| `pageURL` | `{{ pageURL .BasePath .RawQuery 3 }}` | Builds a pagination URL preserving existing query parameters |
| `pageNumbers` | `{{ range pageNumbers .Current .Total }}` | Generates a slice of page numbers with ellipsis gaps (`-1`) |

**Navigation (request-scoped, provided by the framework):**

| Function | Example | Description |
|----------|---------|-------------|
| `navItems` | `{{ range navItems }}` | Raw `[]NavItem` from all `HasNavItems` apps. No filtering or active state. |
| `navLinks` | `{{ range navLinks }}` | Filtered `[]NavLink` with `IsActive` computed from the request path. Hides `AuthOnly`/`AdminOnly` items based on the `AuthChecker` in context. |

**i18n (request-scoped, provided by the core `i18n` sub-package):**

| Function | Example | Description |
|----------|---------|-------------|
| `lang` | `<html lang="{{ lang }}">` | Current locale (e.g., `"en"`, `"de"`). Returns the locale detected from the `Accept-Language` header. |
| `t` | `{{ t "welcome-title" }}` | Simple translation lookup by message ID. |
| `tData` | `{{ tData "greeting" .Data }}` | Translation with interpolation data (`map[string]any`). |
| `tPlural` | `{{ tPlural "items-count" .Count }}` | Pluralised translation. |

## Contrib App Functions

### staticfiles

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `staticURL` | Static | `{{ staticURL "bootstrap/bootstrap.min.css" }}` | Returns content-hashed URL for a static file. Paths are prefixed by the contrib app's prefix (e.g., `bootstrap/filename`). |

### csrf

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `csrfToken` | Request | `{{ csrfToken }}` | Returns the raw CSRF token string. |
| `csrfField` | Request | `{{ csrfField }}` | Renders a complete `<input type="hidden">` element with the CSRF token. |

### auth

**Static** (`HasFuncMap`):

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `credName` | Static | `{{ credName .Credential }}` | Returns a human-readable name for a WebAuthn credential. |
| `emailValue` | Static | `{{ emailValue .User }}` | Returns the user's email or empty string if nil. |
| `deref` | Static | `{{ deref .StringPtr }}` | Dereferences a `*string`, returns empty string if nil. |

**Request-scoped** (`HasRequestFuncMap`):

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `currentUser` | Request | `{{ if $u := currentUser }}{{ $u.Email }}{{ end }}` | Returns the authenticated `*auth.User` or `nil`. |
| `isAuthenticated` | Request | `{{ if isAuthenticated }}...{{ end }}` | Returns `true` if a user is logged in. |
| `authLogo` | Request | `{{ authLogo }}` | Returns the auth logo HTML. |

### Icons (via RegisterIconFunc)

Icon template functions are registered by apps in their `Register()` method via [`cfg.RegisterIconFunc()`](interfaces.md#registericonfunc). They are not tied to any specific contrib app. The `IconFunc` signature is `func(...string) template.HTML` — the variadic string parameter accepts optional CSS classes.

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `icon<Name>` | Static | `{{ iconSunFill }}` | Icon SVG. Name depends on which icons are registered by apps. |
| `icon<Name>` (with class) | Static | `{{ iconSunFill "fs-1" }}` | Icon SVG with CSS class applied. |

## How It Works

At startup, the framework:

1. Creates a base `template.FuncMap` with the core functions
2. Merges static `FuncMap` entries from all `HasFuncMap` apps
3. Parses all `.html` template files from `HasTemplates` apps into one `*template.Template`

Per request:

1. Clones the parsed template set
2. Collects request-scoped functions from all `HasRequestFuncMap` apps
3. Injects them into the clone via `template.Funcs()`

This means static functions are resolved at parse time (fast), while request-scoped functions are resolved per request (slightly slower but can access `*http.Request`).

!!! warning "Name collisions"
    The framework panics at startup if two apps register the same function name. See the [HasFuncMap reference](interfaces.md#hasfuncmap) for the full list of reserved names.
