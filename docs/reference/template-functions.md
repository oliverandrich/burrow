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
| `add` | `{{ add .Page 1 }}` | Integer addition |
| `sub` | `{{ sub .Total 1 }}` | Integer subtraction |
| `pageURL` | `{{ pageURL .BasePath .RawQuery 3 }}` | Builds a pagination URL preserving existing query parameters |
| `pageNumbers` | `{{ range pageNumbers .Current .Total }}` | Generates a slice of page numbers with ellipsis gaps (`-1`) |
| `mediaURL` | `{{ mediaURL .Hero }}` | Returns the public URL for a `document.Attachment`. Auto-registered when Den is opened with `den.WithStorage(...)`; unregistered (and a parse-time error) when no Storage is configured. |
| `icon` | `{{ icon "auth/icon_people" }}` | Looks up the named template define and returns its rendered HTML. Returns empty when the name is empty or unknown. Used by layouts to render `NavItem.Icon` / `NavLink.Icon` / `admin.DashboardItem.Icon` (all hold template-define names). See [Navigation › Icon convention](../guide/navigation.md#icon-convention). |
| `dict` | `{{ template "x" (dict "Page" .Page "BasePath" "/x") }}` | Builds a `map[string]any` from alternating key/value pairs. Useful for passing multiple values to a sub-template. Keys must be strings. |
| `appName` | `<title>{{ appName }}</title>` | The configured application name (from `--app-name`, env `APP_NAME`, toml `server.app_name`; defaults to the binary basename). Same source feeds WebAuthn passkey dialogs and SMTP `From`-name decoration. |

**Navigation (request-scoped, provided by the framework):**

| Function | Example | Description |
|----------|---------|-------------|
| `navItems` | `{{ range navItems }}` | Raw `[]NavItem` from all `HasNavItems` apps. No filtering or active state. |
| `navLinks` | `{{ range navLinks }}` | Filtered `[]NavLink` with `IsActive` computed from the request path. Hides `AuthOnly`/`AdminOnly` items based on the `AuthChecker` in context. |

**i18n (request-scoped, always available — the framework's i18n bundle is pre-registered by `Server.boot` and the locale middleware runs on every request):**

| Function | Example | Description |
|----------|---------|-------------|
| `lang` | `<html lang="{{ lang }}">` | Current locale (e.g., `"en"`, `"de"`). Resolved from the `Accept-Language` header by the bundle's locale middleware. |
| `t` | `{{ t "welcome-title" }}` | Simple translation lookup by message ID. Returns the message ID itself when no translation is loaded. |
| `tData` | `{{ tData "greeting" .Data }}` | Translation with interpolation data (`map[string]any`). |
| `tPlural` | `{{ tPlural "items-count" .Count }}` | Pluralised translation. |

Apps contribute translation files via `HasTranslations`; the framework auto-discovers them at boot. No separate `contrib/i18n` app is needed.

## Contrib App Functions

### staticfiles

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `staticURL` | Static | `{{ staticURL "app/app.min.css" }}` | Returns content-hashed URL for a static file. Paths are prefixed by the contributing app's prefix (e.g., `app/filename`). |

### csrf

These funcs **only exist when `contrib/csrf` is registered**. Templates that use them fail to parse at server boot if csrf is missing — there are no silent fallbacks. Apps that ship templates calling `csrfToken` / `csrfField` / `csrfHxHeaders` must declare `csrf` in their `Dependencies()`.

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `csrfToken` | Request | `{{ csrfToken }}` | Returns the raw CSRF token string. |
| `csrfField` | Request | `{{ csrfField }}` | Renders a complete `<input type="hidden">` element with the CSRF token. |
| `csrfHxHeaders` | Request | `<body {{ csrfHxHeaders }}>` | Renders the `hx-headers='{"X-CSRF-Token": "…"}'` attribute so all htmx requests carry the CSRF token. |

### messages

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `messages` | Request | `{{ range messages }}{{ .Text }}{{ end }}` | Returns the flash messages collected during this request (`[]messages.Message`). Registered by `contrib/messages` via `HasRequestFuncMap`. |

### auth

**Static** (`HasFuncMap`):

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `credName` | Static | `{{ credName .Credential }}` | Returns the credential's `Name`, falling back to `"Passkey"` when empty. |
| `deref` | Static | `{{ deref .StringPtr }}` | Dereferences a `*string`, returns empty string if nil. |

**Request-scoped** (`HasRequestFuncMap`):

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `currentUser` | Request | `{{ if $u := currentUser }}{{ $u.Email }}{{ end }}` | Returns the authenticated `*auth.User[auth.EmptyProfile]` or `nil`. |
| `isAuthenticated` | Request | `{{ if isAuthenticated }}...{{ end }}` | Returns `true` if a user is logged in. |
| `authLogo` | Request | `{{ authLogo }}` | Returns the auth logo HTML. |

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
