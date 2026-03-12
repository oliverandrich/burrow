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

The htmx app embeds `htmx.min.js` and serves it via the `staticfiles` app under the `"htmx"` prefix. It also provides a `htmx/config` template with sensible defaults. Include both in your layout template:

```html
<script src="{{ staticURL "htmx/htmx.min.js" }}" defer></script>
{{ template "htmx/config" . }}
```

The `htmx/config` template renders a `<meta>` tag that configures htmx to swap `422 Unprocessable Entity` responses. This is the correct HTTP status for form validation errors, and allows handlers to return 422 consistently for both htmx and non-htmx requests.

!!! note "Included in the Bootstrap layout"
    If you use the `bootstrap` app, `htmx/config` is already included in the default layout.

## Request Detection

Parse htmx-specific request headers with `htmx.Request()`:

```go
import "github.com/oliverandrich/burrow/contrib/htmx"

func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    hx := htmx.Request(r)

    if hx.IsHTMX() {
        // Partial response — just the fragment
        return burrow.RenderTemplate(w, r, http.StatusOK, "notes/list-fragment", data)
    }

    // Full page response
    return burrow.RenderTemplate(w, r, http.StatusOK, "notes/list", data)
}
```

!!! tip "Automatic layout detection"
    `burrow.RenderTemplate()` already skips layout wrapping when it detects an `HX-Request` header. You typically don't need to check `hx.IsHTMX()` manually unless you want to return completely different content for htmx requests.

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

### Available Response Functions

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
| `Location(w, url)` | `HX-Location` | Navigate without full reload |

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasStaticFiles` | Contributes embedded `htmx.min.js` under `"htmx"` prefix |
| `HasTemplates` | Contributes `htmx/config` template with response handling config |
| `HasDependencies` | Requires `staticfiles` |
