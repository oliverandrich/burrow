# Multi-Tenant: Database per User

This guide explains how to implement a **database-per-tenant** architecture with burrow, where each user gets their own SQLite database file. This is an application-level pattern — it uses burrow's existing building blocks without requiring framework changes.

## When to Use This

Database-per-tenant is a good fit when:

- Each user's data is completely isolated (no cross-user queries)
- You want per-user backups (`cp user_42.db backup/`)
- A misbehaving user's data can never affect another user
- You need to delete all user data cleanly (GDPR: delete the file)
- You want per-user migration rollout (migrate one user at a time)

It is **not** a good fit when:

- You need cross-user queries (reports, leaderboards, admin dashboards aggregating all users)
- You have millions of users (each DB file consumes OS resources)
- Users share data with each other (collaborative features)

## Architecture Overview

```
data/
├── main.db              ← shared: users, sessions, auth
└── tenants/
    ├── 1.db             ← user 1: notes, projects, files
    ├── 2.db             ← user 2: notes, projects, files
    └── 3.db             ← user 3: notes, projects, files
```

The **main database** holds authentication, sessions, and user accounts — data that must be accessible before you know which tenant DB to open.

Each **tenant database** holds all business data for one user, with its own schema and migrations.

## Implementation

### 1. Tenant DB Pool

A pool manages open database connections, opening them on demand and closing idle ones:

```go
// internal/tenant/pool.go
package tenant

import (
    "fmt"
    "path/filepath"
    "sync"
    "time"

    "github.com/oliverandrich/burrow"
    "github.com/uptrace/bun"
)

// Pool manages per-tenant SQLite database connections.
type Pool struct {
    dir     string         // e.g. "data/tenants"
    entries sync.Map       // map[int64]*entry
    migrate func(*bun.DB) error
}

type entry struct {
    db       *bun.DB
    lastUsed time.Time
}

// NewPool creates a tenant DB pool. The migrate function is called
// once when a tenant DB is first opened.
func NewPool(dir string, migrate func(db *bun.DB) error) *Pool {
    return &Pool{dir: dir, migrate: migrate}
}

// Get returns the database for the given tenant, opening it if needed.
func (p *Pool) Get(tenantID int64) (*bun.DB, error) {
    // Fast path: already open
    if v, ok := p.entries.Load(tenantID); ok {
        e := v.(*entry)
        e.lastUsed = time.Now()
        return e.db, nil
    }

    // Slow path: open and migrate
    dsn := filepath.Join(p.dir, fmt.Sprintf("%d.db", tenantID))
    db, err := burrow.OpenDB(dsn)
    if err != nil {
        return nil, fmt.Errorf("open tenant db %d: %w", tenantID, err)
    }

    if err := p.migrate(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("migrate tenant db %d: %w", tenantID, err)
    }

    p.entries.Store(tenantID, &entry{db: db, lastUsed: time.Now()})
    return db, nil
}

// CloseIdle closes databases that haven't been used for the given duration.
// Call this periodically (e.g. every minute) to free resources.
func (p *Pool) CloseIdle(maxIdle time.Duration) {
    cutoff := time.Now().Add(-maxIdle)
    p.entries.Range(func(key, value any) bool {
        e := value.(*entry)
        if e.lastUsed.Before(cutoff) {
            p.entries.Delete(key)
            e.db.Close()
        }
        return true
    })
}

// Close closes all open tenant databases.
func (p *Pool) Close() {
    p.entries.Range(func(key, value any) bool {
        value.(*entry).db.Close()
        p.entries.Delete(key)
        return true
    })
}
```

### 2. Middleware

A middleware looks up the authenticated user and injects their tenant DB into the request context:

```go
// internal/tenant/middleware.go
package tenant

import (
    "context"
    "net/http"

    "github.com/oliverandrich/burrow/contrib/auth"
    "github.com/uptrace/bun"
)

type ctxKeyTenantDB struct{}

// WithDB stores a tenant DB in the context.
func WithDB(ctx context.Context, db *bun.DB) context.Context {
    return context.WithValue(ctx, ctxKeyTenantDB{}, db)
}

// DB retrieves the tenant DB from the context.
func DB(ctx context.Context) *bun.DB {
    db, _ := ctx.Value(ctxKeyTenantDB{}).(*bun.DB)
    return db
}

// Middleware injects the tenant DB for the authenticated user.
// Must run after auth middleware (needs CurrentUser in context).
func Middleware(pool *Pool) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := auth.CurrentUser(r.Context())
            if user == nil {
                // Not authenticated — no tenant DB needed
                next.ServeHTTP(w, r)
                return
            }

            db, err := pool.Get(user.ID)
            if err != nil {
                http.Error(w, "failed to open tenant database", http.StatusInternalServerError)
                return
            }

            ctx := WithDB(r.Context(), db)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### 3. Wiring in main.go

```go
func main() {
    // Shared apps (auth, session, etc.) use the main DB
    srv := burrow.NewServer(
        session.New(),
        csrf.New(),
        staticApp,
        auth.New(),
        bootstrap.New(),
        htmx.New(),
        notes.New(), // your app — uses tenant DB
    )

    srv.SetLayout(bootstrap.NavLayout())

    // Create the tenant pool with a migration function
    notesApp := notes.New()
    pool := tenant.NewPool("data/tenants", func(db *bun.DB) error {
        return burrow.RunAppMigrations(context.Background(), db, notesApp.Name(), notesApp.MigrationFS())
    })
    defer pool.Close()

    // Start idle DB cleanup
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            pool.CloseIdle(10 * time.Minute)
        }
    }()

    // Add tenant middleware (must run after auth middleware)
    srv.Use(tenant.Middleware(pool))

    // ... run server
}
```

### 4. Using the Tenant DB in Handlers

Your handlers and repositories work exactly as before — the only difference is where the `*bun.DB` comes from:

```go
// internal/notes/handlers.go
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) error {
    // Get the tenant DB from context (injected by middleware)
    db := tenant.DB(r.Context())

    // Create a repository scoped to this user's DB
    repo := NewRepository(db)

    notes, err := repo.ListAll(r.Context())
    if err != nil {
        return err
    }

    return burrow.Render(w, r, http.StatusOK, "notes/list", map[string]any{
        "Notes": notes,
    })
}
```

Alternatively, if you prefer to keep the repository on the handler struct, create it once per request in a middleware:

```go
// Middleware that creates a request-scoped repository
func RepoMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        db := tenant.DB(r.Context())
        if db != nil {
            repo := NewRepository(db)
            ctx := WithRepo(r.Context(), repo)
            r = r.WithContext(ctx)
        }
        next.ServeHTTP(w, r)
    })
}
```

### 5. Migrations

Tenant migrations are regular burrow migrations. They run automatically when a tenant DB is first opened (via the pool's migrate function):

```
internal/notes/migrations/
├── 001_create_notes.up.sql
├── 002_add_tags.up.sql
└── 003_add_full_text_search.up.sql
```

Each tenant DB has its own `_migrations` table tracking which migrations have been applied. New migrations are applied lazily — the next time the user logs in.

### 6. Admin Considerations

The admin panel (ModelAdmin) works against a single `*bun.DB`. For a multi-tenant setup, you have a few options:

- **No admin for tenant data** — admin only manages users, jobs, and shared data in the main DB
- **Impersonation** — admin selects a user, the tenant middleware opens that user's DB, and the admin sees their data
- **Custom admin handlers** — build dedicated admin views that open specific tenant DBs on demand

## Operational Benefits

**Backup:** `cp data/tenants/42.db backups/` — one file per user, no export needed.

**GDPR deletion:** `rm data/tenants/42.db` — complete data removal, no orphaned rows.

**Migration rollout:** Migrate one tenant at a time. If a migration breaks, only that user is affected.

**Performance isolation:** A user with millions of rows doesn't slow down other users.

**Monitoring:** `ls -lhS data/tenants/` — instantly see which users consume the most storage.

## Scaling Limits

Each open SQLite database consumes one file descriptor and some memory for the WAL. Practical limits:

| Open DBs | Memory overhead | File descriptors |
|----------|----------------|------------------|
| 100 | ~50 MB | ~300 |
| 1,000 | ~500 MB | ~3,000 |
| 10,000 | ~5 GB | ~30,000 |

The `CloseIdle` mechanism keeps this manageable — only active users hold open connections. A system with 10,000 registered users but 200 concurrent might only have 200 open DBs at any time.

Set the OS file descriptor limit accordingly:

```bash
# /etc/security/limits.conf
myapp soft nofile 65536
myapp hard nofile 65536
```
