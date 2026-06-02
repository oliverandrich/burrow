# JSON CRUD APIs

The `crud` package turns a [Den](database.md) document type into a standard set
of JSON CRUD endpoints. The common 90% — list, get, create, update, delete — is
declared; custom actions stay ordinary [chi](routing.md) routes.

## The simple case

A `crud.Resource` is an `http.Handler`. Store your `*den.DB` on the app in
[`Configure`](configuration.md), then mount a resource the normal chi way with
`r.Mount`:

```go
import (
    "github.com/go-chi/chi/v5"
    "github.com/oliverandrich/burrow"
    "github.com/oliverandrich/burrow/contrib/auth"
    "github.com/oliverandrich/burrow/crud"
    "github.com/oliverandrich/den"
)

type App struct {
    db *den.DB
}

func (a *App) Configure(cfg *burrow.AppConfig, _ *cli.Command) error {
    a.db = cfg.DB
    return nil
}

func (a *App) Routes(r chi.Router) {
    r.Route("/api", func(r chi.Router) {
        r.Use(auth.RequireAuth())                 // gate it like any route
        r.Mount("/notes", crud.NewResource[Note](a.db))
    })
}
```

`Note` is your Den document type (it embeds `document.Base` — see
[Database](database.md)). That serves five endpoints under `/api/notes`:

| Method | Path | Action |
|--------|------|--------|
| `GET` | `/api/notes` | List (paginated) |
| `POST` | `/api/notes` | Create → `201` |
| `GET` | `/api/notes/{id}` | Get |
| `PATCH` | `/api/notes/{id}` | Update (partial merge) |
| `DELETE` | `/api/notes/{id}` | Delete → `204` |

Lists use offset [pagination](pagination.md#json-api) and return a
`burrow.PageResponse[T]` (`{"items": […], "pagination": {…}}`); single resources
return the document as JSON. Responses are always JSON.

!!! warning "Without a write model, every field is client-settable"
    The simple case above binds the request body directly onto `Note`, so a
    client can set *any* field (including `id` or an owner column). That is fine
    for trusted callers; for untrusted input, add a [write model](#write-models).

## Custom actions

When a resource needs a custom action alongside the generated ones (the 10%),
register it into a route group with `Routes` and add the extra routes as
ordinary siblings:

```go
r.Route("/api/notes", func(r chi.Router) {
    r.Use(auth.RequireAuth())
    crud.NewResource[Note](a.db).Routes(r)                // generated: /, /{id}
    r.Post("/{id}/publish", burrow.Handle(a.publishNote)) // your own route
})
```

`res.Routes(r)` reads just like your app's own `Routes` method — no new routing
concept. (`r.Mount` is the shorthand for the no-custom-actions case; because a
mounted handler is a sealed subtree, sibling routes must be registered next to
it with `Routes` instead.)

## Ownership and tenancy

`WithScope` narrows **every** action to the conditions you return for the
request, using Den's [`where`](database.md) package
(`github.com/oliverandrich/den/where`). It applies to list, get, update, and
delete alike, so a client cannot reach another user's row by guessing its id —
single-row lookups `404` instead of leaking:

```go
crud.NewResource[Note](a.db,
    crud.WithScope[Note](func(r *http.Request) []where.Condition {
        u := auth.MustCurrentUser[Profile](r.Context())
        return []where.Condition{where.Field("author_id").Eq(u.ID)}
    }),
)
```

(`Profile` is your [auth profile type](../contrib/auth.md), or
`auth.EmptyProfile`.)

## Write models

`WithCreate` / `WithUpdate` take a typed write model: only its fields are
accepted (validated against their [`validate`](validation.md) tags), and you map
it onto the document yourself — so clients can't set server-owned fields:

```go
type createNote struct {
    Title string `json:"title" validate:"required"`
    Body  string `json:"body"`
}

// Update is a partial merge — pointer fields distinguish "sent" from "absent".
type updateNote struct {
    Title *string `json:"title"`
    Body  *string `json:"body"`
}

crud.NewResource[Note](a.db,
    crud.WithCreate(func(in createNote, r *http.Request) (*Note, error) {
        u := auth.MustCurrentUser[Profile](r.Context())
        return &Note{AuthorID: u.ID, Title: in.Title, Body: in.Body}, nil
    }),
    crud.WithUpdate(func(in updateNote, n *Note, r *http.Request) error {
        if in.Title != nil { n.Title = *in.Title } // only fields the client sent
        if in.Body != nil { n.Body = *in.Body }
        return nil
    }),
)
```

Without these, the body binds directly onto the document (the simple case).

**Update is a partial merge** — the route is `PATCH` (there is no generated `PUT`). The request applies its provided fields onto the loaded record. With a write DTO, give the update model **pointer fields** and apply them conditionally (above) so an omitted field isn't reset to its zero value; the no-DTO path is already partial because JSON decodes onto the loaded document.

## Output shaping

By default the stored document is marshalled as-is — which exposes every field,
including ones you add later. `WithPresenter` maps each document to an explicit
output shape:

```go
crud.WithPresenter(func(n *Note) any {
    return map[string]any{"id": n.ID, "title": n.Title}
})
```

## Filtering, ordering & search

The list endpoint can take query params from the client, but only for fields you
allowlist — an undeclared field is never filterable, sortable, or searchable, so
clients can't probe columns you didn't mean to expose. You allowlist by JSON
field name (the `json:"…"` tag); Den's built-in columns have underscore-prefixed
names, so reach them through the constants (`den.FieldCreatedAt`,
`den.FieldID`) rather than typing the string. Every clause is ANDed with
[the scope](#ownership-and-tenancy), so it only ever narrows what a caller
already sees.

```go
crud.NewResource[Note](db,
    crud.WithFilter[Note]("status"),                 // ?status=active (&status=draft -> IN)
    crud.WithOrdering[Note]("title", den.FieldCreatedAt), // ?ordering=-title,_created_at
    crud.WithSearch[Note]("title", "content"),       // ?search=foo (substring, ORed)
)
```

- **`WithFilter(fields…)`** — exact-match filters. Repeating a param matches any
  of the values (`?status=active&status=draft`). Values are coerced to each
  field's Go type, so numeric and boolean fields filter correctly; an uncoercible
  value (`?price=abc`) is a `400`. Unknown params are ignored.
- **`WithOrdering(fields…)`** — `?ordering=field`, a leading `-` for descending,
  commas for tie-breakers. Unknown tokens are silently dropped; when the param is
  absent or names no allowlisted field, `WithSort` applies.
- **`WithSearch(fields…)`** — a `?search=` term matched as a SQL `LIKE` substring
  across the listed fields (ORed). Case-sensitivity follows the database
  (case-insensitive on SQLite, case-sensitive on PostgreSQL); it is a plain
  substring scan and does **not** use Den's full-text (`den:"fts"`) index.

A bad filter *value* (`?price=abc` on an int field) is rejected with a `400`,
but an unknown ordering or search field is ignored, not an error. Field names
that collide with the reserved params (`limit`, `page`, `ordering`, `search`)
are dropped from the filter allowlist.

## Optimistic concurrency (ETag / If-Match)

`WithOptimisticConcurrency` stops concurrent writes from silently clobbering
each other. It maps Den's revision token (`_rev`) to a strong HTTP ETag: `GET`,
create, and update responses carry an `ETag`, and `PATCH`/`DELETE` require a
matching `If-Match` — a missing header is `428 Precondition Required`, a stale
one is `412 Precondition Failed`. A client that edited a stale copy is rejected
instead of overwriting whoever wrote first.

It needs the document type to opt into Den's revision tracking, so each row
carries a `_rev` (`document` is `github.com/oliverandrich/den/document`):

```go
type Note struct {
    document.Base
    Title string `json:"title"`
}

// Den maintains _rev on every save and rejects a save whose _rev is stale.
func (Note) DenSettings() den.Settings { return den.Settings{UseRevision: true} }

crud.NewResource[Note](db, crud.WithOptimisticConcurrency[Note]())
```

The flow is the standard HTTP one — read the ETag, send it back on write:

```bash
# GET hands back the current version in the ETag header.
curl -i .../api/notes/abc123
# → ETag: "7f3a…"

# The write echoes it; a stale token gets 412 instead of clobbering.
curl -X PATCH -H 'If-Match: "7f3a…"' -d '{"title":"Renamed"}' .../api/notes/abc123
```

`If-Match: *` matches any existing row (a missing id is still a `404`, not a
precondition error). A `412` means the row moved on since you read it — re-`GET`
to pick up the new `ETag` and retry, even if the `If-Match` you sent looked
current. Without this option crud ignores ETags entirely.

Two caveats:

- **Enabling `UseRevision` on a populated table needs care.** Rows written
  before it was on have an empty `_rev`, so every `If-Match` against them is a
  `412` until they're rewritten — backfill or migrate as Den's revision docs
  describe.
- **Revision tracking applies to *every* writer of the type.** crud only
  enforces `If-Match` on its own routes; if the same model is edited by a
  form-based HTML handler, that handler must read `_rev` from the loaded record,
  round-trip it (e.g. a hidden field), and handle `den.ErrRevisionConflict`
  itself — otherwise a stale form post becomes a 500. Optimistic concurrency
  fits machine clients more naturally than form posts.

## Other options

- `WithSort(field, den.Asc|den.Desc)` — default list ordering (creation time
  descending when unset). `field` is the JSON field name, e.g. `"title"`.
- `Only(crud.ActionList, crud.ActionGet)` / `Except(crud.ActionDelete)` — expose
  a subset of the five actions. Disable a standard action and write your own
  sibling route to fully replace it.

## Authentication and CSRF

`crud` is auth-agnostic — gate a resource with ordinary middleware
([`auth.RequireAuth`](../contrib/auth.md), `RequireStaff`, …) via `r.Use`,
exactly like any route. Mounting without a gate leaves the endpoints open, just
as a hand-written route would.

API clients authenticate with
[API-key bearer tokens](../contrib/auth.md#api-key-authentication) rather than
cookies, so they must send `Accept: application/json` (it makes an
unauthenticated request return `401` instead of a login redirect) and the API
prefix needs a [CSRF exemption](../contrib/csrf.md#exempting-webhook-paths) for
unsafe methods. Returning the prefix from `CSRFExemptPaths` is all it takes —
the csrf app discovers the method automatically; the trailing slash makes it a
prefix match:

```go
func (a *App) CSRFExemptPaths() []string { return []string{"/api/"} }
```

```bash
curl -H "Authorization: Bearer brw_…" -H "Accept: application/json" \
     -H "Content-Type: application/json" -d '{"title":"Hello"}' \
     https://example.com/api/notes
```

## Errors

Failures use one envelope, always JSON:

```json
{ "error": { "code": "validation_failed", "fields": { "title": "is required" } } }
```

`code` is `validation_failed` (400, with localized `fields`), `invalid_request`
(400, malformed body), `not_found` (404), or `internal` (500). With
[optimistic concurrency](#optimistic-concurrency-etag-if-match) enabled you may
also see `precondition_required` (428, `If-Match` missing) and
`precondition_failed` (412, `If-Match` stale).
