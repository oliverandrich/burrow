package notes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/where"
)

// createNoteInput is the create write model. It omits UserID on purpose —
// ownership comes from the authenticated user, never the client.
type createNoteInput struct {
	Title   string `json:"title" validate:"required"`
	Content string `json:"content"`
}

// updateNoteInput is the update write model. Update is a partial merge (PATCH),
// so its fields are pointers: only the ones the client actually sends are
// applied, leaving the rest untouched.
type updateNoteInput struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
}

// apiRoutes mounts the JSON CRUD API for notes under /api/notes. It is the
// machine-facing counterpart to the HTML UI: gated by RequireAuth (which now
// also accepts API-key bearer tokens) and scoped to the current user, so each
// caller only ever sees and edits their own notes. For a custom action
// alongside these (e.g. POST /api/notes/{id}/publish), swap r.Mount for
// crud.NewResource(...).Routes(r) and add the route as a sibling — see
// docs/guide/crud-api.md.
func (a *App) apiRoutes(r chi.Router) {
	// Name the resource once, then mount it AND describe it through the same
	// crud.API collector — the path "/notes" is written a single time.
	notes := crud.NewResource[Note](a.db,
		// Scope narrows every action to the caller's own notes.
		crud.WithScope[Note](func(req *http.Request) []where.Condition {
			return []where.Condition{where.Field("user_id").Eq(currentUserID(req))}
		}),
		// WithCreate stamps the owner on new records from the auth context.
		crud.WithCreate(func(in createNoteInput, req *http.Request) (*Note, error) {
			return &Note{UserID: currentUserID(req), Title: in.Title, Content: in.Content}, nil
		}),
		// WithUpdate applies only the fields the PATCH actually carried.
		crud.WithUpdate(func(in updateNoteInput, n *Note, _ *http.Request) error {
			if in.Title != nil {
				n.Title = *in.Title
			}
			if in.Content != nil {
				n.Content = *in.Content
			}
			return nil
		}),
		// Client-driven list query, all narrowed by the scope above.
		// Note's title/content are den:"fts", so ?search= runs a relevance-
		// ranked full-text search over them; ?ordering=-title,_created_at
		// sorts the non-search listing by the named fields.
		crud.WithFullTextSearch[Note](),
		crud.WithOrdering[Note]("title", den.FieldCreatedAt),
		// OpenAPI prose so the generated spec (and any viewer) is self-describing.
		crud.WithTag[Note]("notes", "CRUD operations for the authenticated user's own notes. Every request is scoped to the caller — you can only see and edit your own notes."),
		crud.WithActionDoc[Note](crud.ActionList, "List your notes", "Returns a page of the caller's notes. Supports `?search=` (full-text over title and content), `?ordering=`, and pagination."),
		crud.WithActionDoc[Note](crud.ActionCreate, "Create a note", "Creates a note owned by the authenticated user. The owner is taken from the auth context, never the request body."),
	)

	api := crud.NewAPI(crud.APIInfo{
		Title:       "Notes API",
		Version:     "1.0",
		BaseURL:     "/api",
		Description: "JSON API for personal notes. Authenticate with a personal access token (`Authorization: Bearer brw_…`); every request is scoped to your own notes.",
	})
	// Document the bearer auth the API routes enforce (descriptive only — the
	// gate is auth.RequireAuth below). Create tokens at /auth/api-keys.
	api.AddSecurityScheme("bearerAuth", crud.BearerAuth("personal access token"))
	api.Secured("bearerAuth")

	r.Route("/api", func(r chi.Router) {
		// The OpenAPI spec is public; the resource itself is gated below.
		r.Get("/openapi.json", burrow.Handle(api.SpecHandler()))
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth())
			api.Mount(r, "/notes", notes)
		})
	})
}

// CSRFExemptPaths exempts the bearer-authenticated JSON API from CSRF — it
// carries no cookie, so there is no CSRF token to check. The csrf app discovers
// this method automatically at boot. The trailing slash makes "/api/" a prefix
// match (covers /api/notes, /api/notes/{id}); "/api" with no slash would match
// nothing, since no route is literally /api.
func (a *App) CSRFExemptPaths() []string { return []string{"/api/"} }

// currentUserID returns the authenticated user's id; the API routes are behind
// RequireAuth, so a user is always present.
func currentUserID(r *http.Request) string {
	return auth.MustCurrentUser[Profile](r.Context()).ID
}
