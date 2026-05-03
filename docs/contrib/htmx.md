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

!!! note "Included in the µCSS and Bootstrap layouts"
    If you use the `mucss` or `bootstrap` app, `htmx/config` is already included in the default layout.

## Templates

The htmx app implements `HasTemplates` and contributes these templates:

| Template | Description |
|----------|-------------|
| `htmx/js` | `<script defer>` tag for htmx JS |
| `htmx/config` | `<meta>` tag with htmx response handling config (swaps 422 responses) |
| `htmx/dialog_script` | Client-side listeners for `htmx.OpenDialog` / `htmx.CloseDialog` server triggers, plus delegated close on `[rel="prev"]` clicks. Included automatically by the `mucss` and `bootstrap` `nav_layout`s. |

## HTMX-driven dialogs

A standard pattern for opening forms (or any other content) inside a `<dialog>` element via HTMX. The server returns content swapped into a permanent modal container plus a trigger header; the client opens or closes the dialog accordingly.

### Setup

The `mucss` and `bootstrap` `nav_layout` templates ship a permanent empty `<dialog id="modal"></dialog>` container plus the `htmx/dialog_script` listener — no app-side wiring needed. If you build a custom layout, include both yourself:

```html
<dialog id="modal"></dialog>
{{ template "htmx/dialog_script" . }}
```

The dialog is a structural container only — each view that opens the dialog renders its own `<article>` (or framework-equivalent element) as the swapped content. This lets the view choose its own width class (e.g. µCSS's `modal-lg`), classes, or even bypass the card chrome entirely.

### Trigger buttons

Point the trigger at `#modal` with `innerHTML` swap. The handler returns the article element, the server sets the open header:

```html
<a href="/notes/new" role="button"
   hx-get="/notes/new"
   hx-target="#modal"
   hx-swap="innerHTML">
    Add note
</a>
```

```html
{{ define "notes/form" -}}
<article>
    <header>
        <button aria-label="Close" rel="prev"></button>
        <strong>Add note</strong>
    </header>
    <form ... hx-target="#modal" hx-swap="innerHTML">...</form>
</article>
{{- end }}
```

```go
func (a *App) New(w http.ResponseWriter, r *http.Request) error {
    // ... build form data ...
    htmx.OpenDialog(w, "modal", "modal-lg") // optional class on the dialog
    return burrow.Render(w, r, http.StatusOK, "notes/form", data)
}
```

The optional third argument to `OpenDialog` replaces the dialog element's `className` before opening — useful for µCSS's size variants (`modal-sm`, `modal-lg`, `modal-fullscreen`), which target the dialog itself rather than its content. Pass `""` to clear a previously applied class.

### Validation errors

Forms inside the dialog post back to `#modal`. On a validation error, the handler returns the form (wrapped in its `<article>`) with errors filled in — content swaps in place, dialog stays open:

```go
func (a *App) Create(w http.ResponseWriter, r *http.Request) error {
    f := forms.New[Note]()
    if !f.Bind(r) {
        // No OpenDialog needed — dialog is already open.
        return burrow.Render(w, r, http.StatusUnprocessableEntity, "notes/form", errorData)
    }
    // ... persist ...
    htmx.CloseDialog(w, "modal")
    return htmx.RenderOrRedirect(w, r, "/notes", "notes/create_response", successData)
}
```

### Closing

The server tells the client to close via `htmx.CloseDialog(w, id)`. Users can also close manually by clicking any element with `rel="prev"` *or* `data-close-dialog` inside the dialog (works without server roundtrip):

```html
<header>
    <button aria-label="Close" rel="prev"></button>
    <strong>Edit note</strong>
</header>

<footer class="form-actions">
    <button type="submit">Save</button>
    <button type="button" class="secondary outline" data-close-dialog>Cancel</button>
</footer>
```

**Why two attributes?** `rel="prev"` carries the visual side-effect that µCSS (and any other framework using the same selector) styles it as a 1rem X-icon close button. `data-close-dialog` is purely behavioral — use it on cancel buttons, "Not now" links, or any other element that should close the dialog without inheriting close-icon styling.

### How it works

`htmx.OpenDialog(w, id)` sets `HX-Trigger-After-Swap: {"openDialog":"<id>"}`. After the swap completes, htmx fires an `openDialog` event on `<body>` with `event.detail.value === "<id>"`. The `htmx/dialog_script` listener finds the matching `<dialog>` and calls `showModal()`. `CloseDialog` is the symmetric mirror with `closeDialog`.

The script also installs a delegated `click` listener: any element with `rel="prev"` *inside an open dialog* closes that dialog client-side (no server roundtrip). This intentionally swallows the click — including links — so place `<a rel="prev" href="...">` *inside* an open dialog only if you mean "close the dialog, do not navigate." Outside dialogs the listener does nothing.

### Multiple dialogs on one page

The default `nav_layout` ships a single `<dialog id="modal">` container, which is why the docs use `htmx.OpenDialog(w, "modal")` everywhere. For a second dialog on the same page, render an additional `<dialog id="other-modal">` element somewhere in your layout (or an OOB swap) and pass that id to the helpers:

```go
htmx.OpenDialog(w, "confirm-delete")
htmx.CloseDialog(w, "confirm-delete")
```

### Header composition

`HX-Trigger`, `HX-Trigger-After-Swap`, and `HX-Trigger-After-Settle` are single-slot headers. `htmx.OpenDialog` / `htmx.CloseDialog` both write `HX-Trigger-After-Swap`, as does `htmx.TriggerAfterSwap`. A handler that calls more than one of these for the same response will only ship the last value. If you need to fire multiple after-swap events, build the JSON object yourself with `w.Header().Set("HX-Trigger-After-Swap", ...)` containing all events.

## Request Detection

Parse htmx-specific request headers with `htmx.Request()`:

```go
import "github.com/oliverandrich/burrow/contrib/htmx"

func (a *App) List(w http.ResponseWriter, r *http.Request) error {
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

func (a *App) Delete(w http.ResponseWriter, r *http.Request) error {
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
