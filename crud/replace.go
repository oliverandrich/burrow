package crud

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
)

// handleReplace serves PUT: a full replace of the stored document. The body is
// decoded the same way create decodes it (so the resource's write model — and
// its mass-assignment posture — carries over), producing a fresh T whose
// omitted fields are zero. Den's server-owned fields (id, timestamps, revision,
// and any soft-delete audit fields) are then carried over from the stored
// record via [den.PreserveServerFields], so a replace can't move the row, reset
// its creation time, skip the optimistic-concurrency check, or resurrect a
// soft-deleted row.
func (rs *Resource[T]) handleReplace(w http.ResponseWriter, r *http.Request) error {
	// Scoped load (not den.Replace, which loads by id alone): the find applies
	// the resource's row scope, so one owner can't replace another's row.
	existing, err := rs.find(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	if err := rs.requirePrecondition(r, existing); err != nil {
		return rs.fail(w, r, err)
	}

	doc, err := rs.decodeCreate(r)
	if err != nil {
		return rs.fail(w, r, err)
	}
	if err := den.PreserveServerFields(rs.db, doc, existing); err != nil {
		return rs.fail(w, r, err)
	}

	if err := den.Save(r.Context(), rs.db, doc); err != nil {
		return rs.fail(w, r, err)
	}
	rs.setETag(w, doc)
	return burrow.JSON(w, http.StatusOK, rs.view(doc))
}
