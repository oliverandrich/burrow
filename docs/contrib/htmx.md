# htmx

Serves the [htmx](https://htmx.org/) JavaScript library as a static asset and provides Go helpers for htmx request detection and response headers. Inspired by [django-htmx](https://django-htmx.readthedocs.io/).

**Package:** `github.com/oliverandrich/burrow/contrib/htmx`

**Depends on:** `staticfiles`

## Setup

```go
srv := burrow.NewServer(
    htmx.New(),
    staticApp, // staticfiles.New(myStaticFS) — returns (*App, error)
    // ... other apps
)
```

The htmx app embeds `htmx.min.js` and the [SSE extension](https://htmx.org/extensions/sse/) (`ext/sse.min.js`), serving both via the `staticfiles` app under the `"htmx"` prefix. It also provides a `htmx/config` template with sensible defaults. Include both in your layout template:

```html
{{ template "htmx/js" . }}
{{ template "htmx/config" . }}
```

The `htmx/config` template renders a `<meta>` tag that configures htmx to swap `422 Unprocessable Entity` responses. This is the correct HTTP status for form validation errors, and allows handlers to return 422 consistently for both htmx and non-htmx requests.

!!! note "Included in the Bootstrap layout"
    If you use the `bootstrap` app, `htmx/config` is already included in the default layout.

## Templates

The htmx app implements `HasTemplates` and contributes these templates:

| Template | Description |
|----------|-------------|
| `htmx/js` | `<script defer>` tag for htmx JS |
| `htmx/config` | `<meta>` tag with htmx response handling config (swaps 422 responses) |

## Request Detection

Parse htmx-specific request headers with `htmx.Request()`:

```go
import "github.com/oliverandrich/burrow/contrib/htmx"

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    hx := htmx.Request(r)

    if hx.IsHTMX() {
        // Partial response — just the fragment
        return burrow.Render(w, r, http.StatusOK, "notes/list-fragment", data)
    }

    // Full page response
    return burrow.Render(w, r, http.StatusOK, "notes/list", data)
}
```

!!! tip "Automatic layout detection"
    `burrow.Render()` already skips layout wrapping when it detects an `HX-Request` header. You typically don't need to check `hx.IsHTMX()` manually unless you want to return completely different content for htmx requests.

### Available Request Methods

| Method | Header | Description |
|--------|--------|-------------|
| `IsHTMX()` | `HX-Request` | Request was made by htmx |
| `IsBoosted()` | `HX-Boosted` | Request is via an `hx-boost` element |
| `Target()` | `HX-Target` | ID of the target element |
| `Trigger()` | `HX-Trigger` | ID of the triggered element |
| `TriggerName()` | `HX-Trigger-Name` | Name of the triggered element |
| `Prompt()` | `HX-Prompt` | User response to `hx-prompt` |
| `CurrentURL()` | `HX-Current-URL` | Current browser URL |
| `HistoryRestore()` | `HX-History-Restore-Request` | History restoration after cache miss |

## Response Helpers

### Smart Helpers

These helpers handle the common pattern of branching between htmx and non-htmx requests:

#### SmartRedirect

Issues an `HX-Redirect` for htmx requests or a standard 303 redirect for normal requests.

```go
// Before:
if htmx.Request(r).IsHTMX() {
    htmx.Redirect(w, "/dashboard")
    w.WriteHeader(http.StatusOK)
    return nil
}
http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
return nil

// After:
htmx.SmartRedirect(w, r, "/dashboard")
return nil
```

#### RenderOrRedirect

Renders a template fragment for htmx requests, or issues a 303 redirect for standard requests.

```go
// Before:
if htmx.Request(r).IsHTMX() {
    return burrow.Render(w, r, http.StatusOK, "notes/create_response", data)
}
http.Redirect(w, r, "/notes", http.StatusSeeOther)
return nil

// After:
return htmx.RenderOrRedirect(w, r, "/notes", "notes/create_response", data)
```

### Header Setters

Set htmx response headers to control client-side behaviour:

```go
import "github.com/oliverandrich/burrow/contrib/htmx"

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) error {
    // ... delete resource ...

    // Redirect the browser (client-side, no full page reload)
    htmx.Redirect(w, "/notes")
    return nil
}
```

| Function | Header | Description |
|----------|--------|-------------|
| `Redirect(w, url)` | `HX-Redirect` | Client-side redirect |
| `Refresh(w)` | `HX-Refresh` | Full page refresh |
| `Trigger(w, event)` | `HX-Trigger` | Trigger a client-side event |
| `TriggerAfterSettle(w, event)` | `HX-Trigger-After-Settle` | Trigger event after settle |
| `TriggerAfterSwap(w, event)` | `HX-Trigger-After-Swap` | Trigger event after swap |
| `PushURL(w, url)` | `HX-Push-Url` | Push URL to history stack |
| `ReplaceURL(w, url)` | `HX-Replace-Url` | Replace current URL |
| `Reswap(w, strategy)` | `HX-Reswap` | Override swap strategy |
| `Retarget(w, selector)` | `HX-Retarget` | Change target element |
| `Reselect(w, selector)` | `HX-Reselect` | Change content selection |
| `Location(w, url)` | `HX-Location` | Navigate without full reload |

### Constants

| Constant | Value | Description |
|----------|-------|-------------|
| `StatusStopPolling` | `286` | HTTP status code that instructs htmx to stop polling |

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()` |
| `HasStaticFiles` | Contributes embedded `htmx.min.js` under `"htmx"` prefix |
| `HasTemplates` | Contributes `htmx/js` and `htmx/config` templates |
| `HasDependencies` | Requires `staticfiles` |
