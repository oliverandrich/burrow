package notes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/contrib/auth"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den/where"
)

// noteInput is the API write model. It omits UserID on purpose — ownership
// comes from the authenticated user, never the client (mass-assignment guard).
type noteInput struct {
	Title   string `json:"title" validate:"required"`
	Content string `json:"content"`
}

// apiRoutes mounts the JSON CRUD API for notes under /api/notes. It is the
// machine-facing counterpart to the HTML UI: gated by RequireAuth (which now
// also accepts API-key bearer tokens) and scoped to the current user, so each
// caller only ever sees and edits their own notes. For a custom action
// alongside these (e.g. POST /api/notes/{id}/publish), swap r.Mount for
// crud.NewResource(...).Routes(r) and add the route as a sibling — see
// docs/guide/crud-api.md.
func (a *App) apiRoutes(r chi.Router) {
	r.Route("/api", func(r chi.Router) {
		r.Use(auth.RequireAuth())
		r.Mount("/notes", crud.NewResource[Note](a.db,
			// Scope narrows every action to the caller's own notes.
			crud.WithScope[Note](func(req *http.Request) []where.Condition {
				return []where.Condition{where.Field("user_id").Eq(currentUserID(req))}
			}),
			// WithCreate stamps the owner on new records from the auth context.
			crud.WithCreate(func(in noteInput, req *http.Request) (*Note, error) {
				return &Note{UserID: currentUserID(req), Title: in.Title, Content: in.Content}, nil
			}),
			crud.WithUpdate(func(in noteInput, n *Note, _ *http.Request) error {
				n.Title, n.Content = in.Title, in.Content
				return nil
			}),
		))
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
