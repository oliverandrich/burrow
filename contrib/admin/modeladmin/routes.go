package modeladmin

import (
	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

// Init runs boot-time detection (FTS5, cascade foreign keys) for this ModelAdmin.
// Called automatically by Routes(). Call manually when registering routes without Routes().
func (ma *ModelAdmin[T]) Init() {
	tbl := tableName[T]()

	// Register table → display name for cascade impact labels.
	RegisterTableDisplayName(tbl, ma.DisplayPluralName)

	// Auto-add DeleteBulkAction when CanDelete is true and no "delete" bulk action exists.
	if ma.CanDelete && !ma.hasBulkAction("delete") {
		ma.BulkActions = append(ma.BulkActions, DeleteBulkAction[T]())
	}

	if ma.DB == nil {
		return
	}

	// Auto-detect FTS5 table at boot time.
	if tbl != "" && len(ma.SearchFields) > 0 {
		ma.ftsTable = detectFTS(ma.DB, tbl)
	}

	// Auto-detect ON DELETE CASCADE foreign keys at boot time.
	if tbl != "" && ma.CanDelete {
		ma.cascades = detectCascades(ma.DB, tbl)
	}
}

// hasBulkAction returns true if a bulk action with the given slug is already configured.
func (ma *ModelAdmin[T]) hasBulkAction(slug string) bool {
	for _, a := range ma.BulkActions {
		if a.Slug == slug {
			return true
		}
	}
	return false
}

// Routes mounts all CRUD routes for this ModelAdmin on the given router.
// Routes are mounted under /{slug} so the caller should mount this
// within the /admin route group.
func (ma *ModelAdmin[T]) Routes(r chi.Router) {
	ma.Init()

	r.Route("/"+ma.Slug, func(r chi.Router) {
		r.Get("/", burrow.Handle(ma.HandleList))

		if len(ma.BulkActions) > 0 {
			r.Post("/bulk/{action}", burrow.Handle(ma.HandleBulkAction))
		}

		if ma.CanExport {
			r.Get("/export.csv", burrow.Handle(ma.HandleExportCSV))
			r.Get("/export.json", burrow.Handle(ma.HandleExportJSON))
		}

		if ma.CanCreate {
			r.Get("/new", burrow.Handle(ma.HandleNew))
			r.Post("/", burrow.Handle(ma.HandleCreate))
		}

		if ma.CanDelete {
			r.Get("/bulk/delete", burrow.Handle(ma.HandleConfirmDelete))
			r.Post("/bulk/delete", burrow.Handle(ma.HandleDelete))
		}

		r.Get("/{id}", burrow.Handle(ma.HandleDetail))

		if ma.CanEdit {
			r.Post("/{id}", burrow.Handle(ma.HandleUpdate))
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
