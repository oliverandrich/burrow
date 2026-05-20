# Auth: Extending the User

Companion page to [Auth](auth.md). The auth contrib ships a minimal User: email, username, role, password, credentials, recovery codes. Real applications usually want more — a display name, bio, avatar URL, social links, department, custom roles. The `Profile` type parameter on `auth.User[P]` is how you add those without forking auth.

The Profile is **stored inline** as a nested JSON object inside the user document. One query loads the whole record. No 1:1 Profile table, no join, no eventual-consistency window.

!!! warning "Profile must be a plain struct, not a Den document"
    Profile is a plain Go struct. Don't embed `document.Base`, don't register it with the server, don't try to wire `Link` relations from `User[P]` to it. The Profile value is serialised inline as JSON inside the user document — it doesn't get its own table. `auth.Configure` enforces this at startup: passing a `document.Base`-embedding type as `P` fails fast with `auth: Profile type … must not embed document.Base`. If you need a separate row with its own indexes or lifecycle, that's the [separate-document pattern](#when-to-use-a-separate-document-instead) instead: a sibling document keyed by `UserID` with a proper `Link`.

## The minimal extension

```go
package myapp

type Profile struct {
    Name      string `form:"name"       verbose:"Name"`
    Bio       string `form:"bio"        verbose:"Bio" widget:"textarea"`
    AvatarURL string `form:"avatar_url" verbose:"Avatar URL"`
}
```

Register the auth contrib with your Profile type:

```go
authApp := auth.New[myapp.Profile]()
srv.Register(authApp)
```

Apps that don't need extensions use `auth.EmptyProfile`:

```go
authApp := auth.New[auth.EmptyProfile]()
```

## Accessing Profile data

Inside handlers, parametrise `auth.CurrentUser`:

```go
func (a *App) profilePage(w http.ResponseWriter, r *http.Request) error {
    u := auth.CurrentUser[myapp.Profile](r.Context())
    return burrow.HTML(w, 200, "<h1>"+u.Profile.Name+"</h1>")
}
```

In templates, the user is available via the standard `currentUser` template function and the `.User.Profile.X` path works directly:

```html
<header>
    Welcome, {{ with currentUser }}{{ if .Profile.Name }}{{ .Profile.Name }}{{ else }}{{ .Username }}{{ end }}{{ end }}!
</header>
```

The `{{ if .Profile.Name }}{{ .Profile.Name }}{{ else }}{{ .Username }}{{ end }}` fallback is the recommended pattern: auth-core only knows the Username; the richer display lives in your Profile.

## Admin form integration

The auth admin's user-edit page (`/admin/users/{id}`) is built generically. Since `User[Profile]` has a `Profile` field of struct type, the forms package renders it automatically as a [subform](../guide/forms.md#nested-struct-fields-subforms): one section with Name/Bio/AvatarURL inputs, no template work needed.

Auth's admin user *list* (`/admin/users`) only shows auth-core columns (Username, Email, Role, Active). To add Profile-aware columns or filters, override the `auth/admin_users` template in your app — last-define-wins resolves to yours.

## Registration flow

Default `auth.New[P]()` collects auth essentials only at registration (email/username, no Profile fields). The Profile starts at its zero value; users fill it in via the admin user-edit page or a custom settings page.

Apps with onboarding flows that need richer registration (e.g. "what's your display name?" required at sign-up) override the registration handler and template — that's app code, not framework. The default flow stays minimal to avoid registration friction.

## Indexes on Profile fields

`den:` tags flow through into Profile and produce real database indexes on the JSON path. As of Den 0.14, the schema walker descends into named struct fields and emits dotted paths like `$.profile.slug`:

```go
type Profile struct {
    Slug       string `json:"slug"       den:"unique"`
    Department string `json:"department" den:"index"`
    Bio        string `json:"bio"        den:"fts"`
}
```

`unique` enforces a real uniqueness constraint, `index` accelerates equality lookups, `fts` registers the field with the SQLite FTS5 index. `index_together` / `unique_together` groups can mix flat User fields and nested Profile fields.

Index names dot-flatten the path (`idx_user_profile_slug`); the expression keeps the dots so Den resolves the nested JSON value at runtime. See Den's [Nested Field Indexes](https://github.com/oliverandrich/den/blob/main/docs/guide/documents.md#nested-field-indexes) docs for the generated SQL on both SQLite and PostgreSQL.

## Limitations to know

### Search by Profile field

`Repository[P].SearchUsers` queries Username and Email only — it does not search inside Profile. Apps that want users to be findable by display name should add a domain-specific query helper (e.g. `repo.SearchAuthorsByName` that queries the JSON path on the Profile column). If the field is tagged `den:"index"` the lookup uses the nested index; otherwise it's an un-indexed JSON scan. For full-text search on a Bio-like field, tag it `den:"fts"` and Den registers an FTS5 index — no separate document needed.

### WebAuthn display name

`User[P].WebAuthnDisplayName()` returns the Username, regardless of Profile contents. The WebAuthn UI hint is not customisable from Profile in v1; if you need to override, configure the WebAuthn relying party directly.

## Migrating from pre-Profile auth

If you're updating an app from a burrow version where `auth.User` was non-generic:

| Old | New |
|---|---|
| `auth.New(opts...)` | `auth.New[auth.EmptyProfile](opts...)` |
| `auth.Repository` | `auth.Repository[auth.EmptyProfile]` |
| `auth.NewRepository(db)` | `auth.NewRepository[auth.EmptyProfile](db)` |
| `auth.CurrentUser(ctx)` | `auth.CurrentUser[auth.EmptyProfile](ctx)` |
| `auth.MustCurrentUser(ctx)` | `auth.MustCurrentUser[auth.EmptyProfile](ctx)` |
| `*auth.User` | `*auth.User[auth.EmptyProfile]` |
| `user.Name` / `user.Bio` (in templates or code) | Move to Profile if you want them; auth-core no longer carries Name/Bio |
| `repo.CreateUser(ctx, username, name)` | `repo.CreateUser(ctx, username)` — Name lived on User; set Profile.Name post-create if needed |

Existing user records in your database load fine: missing `profile` keys unmarshal to the zero value of P. No data migration is required.

## When to use a separate document instead

The inline-Profile pattern is great when extension data is small and always-loaded with the user. Reach for a separate document when:

- Profile data is large (avatar files, big bio markdown blobs) — pay for the second query only when the profile view is rendered.
- The data has its own lifecycle (versioned profile history, multi-profile-per-user, audit trail).
- You need to query the data without loading the user (e.g. background jobs that scan profiles directly).

Indexes alone are no longer a reason — `den:"index"` / `unique` / `fts` work directly on Profile fields. A typical hybrid: keep small display info (Name, AvatarURL) inline on Profile for fast hot-path rendering, store heavier data (long bio, settings JSON) as siblings keyed by UserID.
