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
		r.Get("/", burrow.Handle(ma.handleList))

		if ma.CanCreate {
			r.Get("/new", burrow.Handle(ma.handleNew))
			r.Post("/", burrow.Handle(ma.handleCreate))
		}

		r.Get("/{id}", burrow.Handle(ma.handleDetail))

		if ma.CanEdit {
			r.Post("/{id}", burrow.Handle(ma.handleUpdate))
		}

		if ma.CanDelete {
			r.Delete("/{id}", burrow.Handle(ma.handleDelete))
		}
	})
}
