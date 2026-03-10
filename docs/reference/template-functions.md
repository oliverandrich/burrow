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
| `csrfToken` | Request | `<input name="gorilla.csrf.Token" value="{{ csrfToken }}">` | Returns the CSRF token for the current request. |

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
| `isAdminEditSelf` | Request | `{{ if isAdminEditSelf }}...{{ end }}` | Returns `true` if the admin is editing their own account. |
| `isAdminEditLastAdmin` | Request | `{{ if isAdminEditLastAdmin }}...{{ end }}` | Returns `true` if the admin is editing the last remaining admin. |
| `authLogo` | Request | `{{ authLogo }}` | Returns the auth logo HTML. |

### bootstrap

| Function | Type | Example | Description |
|----------|------|---------|-------------|
| `add` | Static | `{{ add .Page 1 }}` | Integer addition. |
| `sub` | Static | `{{ sub .Total 1 }}` | Integer subtraction. |
| `pageURL` | Static | `{{ pageURL .BaseURL .Page .Limit }}` | Builds a pagination URL with `page` and `limit` query parameters. |
| `pageLimit` | Static | `{{ pageLimit .Page }}` | Derives the per-page size from a `PageResult`. |
| `pageNumbers` | Static | `{{ range pageNumbers .Current .Total }}` | Generates a slice of page numbers for pagination controls. |

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
