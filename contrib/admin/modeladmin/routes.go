package modeladmin

import (
	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
)

// Routes mounts all CRUD routes for this ModelAdmin on the given router.
// Routes are mounted under /{slug} so the caller should mount this
// within the /admin route group.
func (ma *ModelAdmin[T]) Routes(r chi.Router) {
	r.Route("/"+ma.Slug, func(r chi.Router) {
		r.Get("/", burrow.Handle(ma.HandleList))

		if ma.CanCreate {
			r.Get("/new", burrow.Handle(ma.HandleNew))
			r.Post("/", burrow.Handle(ma.HandleCreate))
		}

		r.Get("/{id}", burrow.Handle(ma.HandleDetail))

		if ma.CanEdit {
			r.Post("/{id}", burrow.Handle(ma.HandleUpdate))
		}

		if ma.CanDelete {
			r.Delete("/{id}", burrow.Handle(ma.HandleDelete))
		}

		for _, action := range ma.RowActions {
			switch action.method() {
			case "DELETE":
				r.Delete("/{id}/"+action.Slug, burrow.Handle(action.Handler))
			default:
				r.Post("/{id}/"+action.Slug, burrow.Handle(action.Handler))
			}
		}
	})
}
