package modeladmin

import (
	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

// Routes mounts all CRUD routes for this ModelAdmin on the given router.
// Routes are mounted under /{slug} so the caller should mount this
// within the /admin route group.
func (ma *ModelAdmin[T]) Routes(r chi.Router) {
	// Auto-detect FTS5 table at boot time.
	if tbl := tableName[T](); tbl != "" && len(ma.SearchFields) > 0 {
		ma.ftsTable = detectFTS(ma.DB, tbl)
	}

	r.Route("/"+ma.Slug, func(r chi.Router) {
		r.Get("/", burrow.Handle(ma.HandleList))

		if ma.CanExport {
			r.Get("/export.csv", burrow.Handle(ma.HandleExportCSV))
			r.Get("/export.json", burrow.Handle(ma.HandleExportJSON))
		}

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
