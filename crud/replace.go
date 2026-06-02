package crud

import (
	"net/http"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/den"
)

// handleReplace serves PUT: a full replace of the stored document. The body is
// decoded the same way create decodes it (so the resource's write model — and
// its mass-assignment posture — carries over), producing a fresh T whose
// omitted fields are zero. The server-owned id, creation time, and revision are
// then copied from the stored record so a replace can't move the row, reset its
// creation time, or skip the optimistic-concurrency check; Den re-stamps
// _updated_at itself.
//
// This preserves the standard document.Base server-owned fields. A type that
// also embeds document.SoftDelete / document.Tracked carries extra server-owned
// state those embeds manage — full replace doesn't special-case it (tracked via
// den-cczd upstream). In practice it's fine: a soft-deleted row isn't loadable
// to replace, and Den repopulates the change snapshot on save.
func (rs *Resource[T]) handleReplace(w http.ResponseWriter, r *http.Request) error {
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
	rs.copyFieldAt(doc, existing, rs.baseID)
	rs.copyFieldAt(doc, existing, rs.baseCreated)
	rs.copyFieldAt(doc, existing, rs.baseRev)

	if err := den.Save(r.Context(), rs.db, doc); err != nil {
		return rs.fail(w, r, err)
	}
	rs.setETag(w, doc)
	return burrow.JSON(w, http.StatusOK, rs.view(doc))
}
