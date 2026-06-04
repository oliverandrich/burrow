# API Docs

Self-hosted, interactive OpenAPI documentation UI. Vendors the
[Scalar](https://github.com/scalar/scalar) API reference and serves a page that
points it at your OpenAPI spec — by default the spec produced by the
[crud](../guide/crud-api.md) package's `API.SpecHandler`.

**Package:** `github.com/oliverandrich/burrow/contrib/apidocs`

**Depends on:** [staticfiles](staticfiles.md)

The Scalar bundle is vendored and served content-hashed through `staticfiles`,
and web fonts are disabled, so the documentation works fully offline with no CDN
dependency at runtime.

## Setup

apidocs renders a viewer; it does not produce the spec. You serve the OpenAPI
JSON yourself (see the [crud guide](../guide/crud-api.md#openapi-spec) —
`api.SpecHandler()` mounted at a route of your choice), then point apidocs at
that URL. It depends on [staticfiles](staticfiles.md), which serves the vendored
bundle:

```go
import (
    "embed"

    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/apidocs"
    "github.com/oliverandrich/burrow/contrib/staticfiles"
)

var assetsFS embed.FS // your app's static assets (may be empty)

staticApp, err := staticfiles.New(assetsFS)
if err != nil {
    log.Fatal(err)
}

srv := burrow.NewServer(
    staticApp,
    // ... your app that mounts api.SpecHandler() at /api/openapi.json
    apidocs.New(
        apidocs.WithSpecURL("/api/openapi.json"), // the route you mounted SpecHandler at
        apidocs.WithTitle("My API"),
    ),
)
```

The page is served at `/api/docs` by default. Open it in a browser to browse the
endpoints and try requests against the live API.

The documentation page is **public** — it exposes the same information as your
spec route, with no gating of its own. Keep your spec non-sensitive, or place the
whole API surface (spec route + `/api/docs`) behind external auth (e.g. a reverse
proxy). The spec describes shapes, not data, so it is usually safe to expose.

## Options

| Option | Default | Description |
| --- | --- | --- |
| `WithSpecURL(url)` | `/api/openapi.json` | URL the viewer loads the spec from — the route you mounted `api.SpecHandler()` at. |
| `WithRoute(route)` | `/api/docs` | Route the documentation page is served at. |
| `WithTitle(title)` | `API Documentation` | HTML page title. |

## Content Security Policy

If you also use the [secure](secure.md) contrib with a Content-Security-Policy:

- The vendored bundle is same-origin, so `script-src 'self'` covers it.
- Scalar injects inline styles — allow `style-src 'unsafe-inline'` (or a nonce).
- Web fonts are disabled, so no external `font-src` is needed.

## Updating Scalar

The bundle is vendored at `contrib/apidocs/static/scalar.standalone.js`, pinned
to `@scalar/api-reference` **1.58.0**. To bump it, replace the file with a newer
[`@scalar/api-reference`](https://www.npmjs.com/package/@scalar/api-reference)
standalone build (`dist/browser/standalone.js`) and re-run the tests.
