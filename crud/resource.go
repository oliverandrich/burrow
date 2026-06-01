package crud

import (
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// Action names accepted by [Only] and [Except].
const (
	ActionList   = "list"
	ActionGet    = "get"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

// Resource exposes a Den document type T as a set of JSON CRUD endpoints.
// Build it with [NewResource]; mount it with chi's r.Mount or register it into
// a route group with [Resource.Routes].
type Resource[T any] struct {
	db           *den.DB
	scope        func(*http.Request) []where.Condition
	decodeCreate func(*http.Request) (*T, error)
	applyUpdate  func(*http.Request, *T) error
	present      func(*T) any
	sortField    string
	sortDir      den.SortDirection
	enabled      map[string]bool

	once    sync.Once
	handler chi.Router
}

// Option configures a [Resource]. See [WithScope], [WithCreate], [WithUpdate],
// [WithPresenter], [WithSort], [Only], and [Except].
type Option[T any] func(*Resource[T])

// NewResource builds a Resource for the Den document type T backed by db.
// Without options it exposes all five actions, binds request bodies directly
// onto T (no separate write model), returns the stored document as JSON, and
// orders lists by creation time descending. Options refine each of those.
func NewResource[T any](db *den.DB, opts ...Option[T]) *Resource[T] {
	rs := &Resource[T]{
		db:        db,
		sortField: den.FieldCreatedAt,
		sortDir:   den.Desc,
		enabled: map[string]bool{
			ActionList:   true,
			ActionGet:    true,
			ActionCreate: true,
			ActionUpdate: true,
			ActionDelete: true,
		},
	}
	// Defaults bind the request body directly onto T; WithCreate/WithUpdate
	// replace these with DTO-backed mappers.
	rs.decodeCreate = func(r *http.Request) (*T, error) {
		var v T
		if err := bindValidate(r, &v); err != nil {
			return nil, err
		}
		return &v, nil
	}
	rs.applyUpdate = func(r *http.Request, dst *T) error { return bindValidate(r, dst) }
	for _, o := range opts {
		o(rs)
	}
	return rs
}

// Routes registers the resource's enabled actions on r at its current mount
// path. Use it inside an r.Route group when you want custom sibling routes
// next to the generated ones.
func (rs *Resource[T]) Routes(r chi.Router) { rs.register(r) }

// ServeHTTP lets a Resource be mounted directly with chi's r.Mount. The
// internal router is built on first use, so a resource used only via
// [Resource.Routes] never allocates one.
func (rs *Resource[T]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rs.once.Do(func() {
		rs.handler = chi.NewRouter()
		rs.register(rs.handler)
	})
	rs.handler.ServeHTTP(w, r)
}

func (rs *Resource[T]) register(r chi.Router) {
	if rs.enabled[ActionList] {
		r.Get("/", burrow.Handle(rs.handleList))
	}
	if rs.enabled[ActionCreate] {
		r.Post("/", burrow.Handle(rs.handleCreate))
	}
	if rs.enabled[ActionGet] {
		r.Get("/{id}", burrow.Handle(rs.handleGet))
	}
	if rs.enabled[ActionUpdate] {
		// Update is a partial merge (PATCH): the request applies its provided
		// fields onto the loaded record. PUT (full replace) is intentionally
		// not generated — it can't reset omitted fields while preserving the
		// server-owned ID for an arbitrary T. Write a custom action if you
		// need replace semantics.
		r.Patch("/{id}", burrow.Handle(rs.handleUpdate))
	}
	if rs.enabled[ActionDelete] {
		r.Delete("/{id}", burrow.Handle(rs.handleDelete))
	}
}

// scopeConds returns the request's ownership/tenancy conditions, or nil.
func (rs *Resource[T]) scopeConds(r *http.Request) []where.Condition {
	if rs.scope == nil {
		return nil
	}
	return rs.scope(r)
}

// view renders a document for output, applying the presenter when set.
func (rs *Resource[T]) view(doc *T) any {
	if rs.present != nil {
		return rs.present(doc)
	}
	return doc
}

// find loads a single document by its path id, narrowed by the scope so one
// owner cannot reach another's row by guessing an id (it 404s instead).
func (rs *Resource[T]) find(r *http.Request) (*T, error) {
	q := den.NewQuery[T](rs.db, where.Field(den.FieldID).Eq(chi.URLParam(r, "id")))
	if conds := rs.scopeConds(r); len(conds) > 0 {
		q = q.Where(conds...)
	}
	return q.First(r.Context())
}
