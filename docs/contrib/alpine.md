# Alpine.js

Serves the [Alpine.js](https://alpinejs.dev/) JavaScript library as a static asset with content-hashed URLs and immutable caching.

**Package:** `github.com/oliverandrich/burrow/contrib/alpine`

**Depends on:** `staticfiles`

## Setup

```go
srv := burrow.NewServer(
    alpine.New(),
    staticApp, // staticfiles.New(myStaticFS) — returns (*App, error)
    // ... other apps
)
```

The alpine app embeds `alpine.min.js` and serves it via the `staticfiles` app under the `"alpine"` prefix. Include the script tag in your layout template:

```html
{{ template "alpine/js" . }}
```

!!! tip "Optional by default"
    The core framework defines an empty stub for `alpine/js`, so layout templates can reference it even when the alpine app is not registered — the stub renders nothing.

## Templates

The alpine app implements `HasTemplates` and contributes this template:

| Template | Description |
|----------|-------------|
| `alpine/js` | `<script defer>` tag for Alpine.js |

## Usage

Alpine.js works entirely through HTML attributes — no Go helpers are needed:

```html
<div x-data="{ open: false }">
    <button @click="open = !open">Toggle</button>
    <div x-show="open" x-transition>
        Hello from Alpine.js!
    </div>
</div>
```

See the [Alpine.js documentation](https://alpinejs.dev/) for the full API reference.

## Interfaces Implemented

| Interface | Description |
|-----------|-------------|
| `burrow.App` | Required: `Name()`, `Register()` |
| `HasStaticFiles` | Contributes embedded `alpine.min.js` under `"alpine"` prefix |
| `HasTemplates` | Contributes `alpine/js` template |
| `HasDependencies` | Requires `staticfiles` |
