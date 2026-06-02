package crud_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// revItem is a revision-tracked document: DenSettings opts it into Den's
// optimistic-concurrency revision token (`_rev`), which crud maps to ETags.
type revItem struct {
	document.Base
	Name string `json:"name" validate:"required"`
}

func (revItem) DenSettings() den.Settings { return den.Settings{UseRevision: true} }

func newRevDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrowtest.DB(t)
	require.NoError(t, den.Register(t.Context(), db, &revItem{}))
	return db
}

func saveRev(t *testing.T, db *den.DB) *revItem {
	t.Helper()
	it := &revItem{Name: "a"}
	require.NoError(t, den.Save(t.Context(), db, it))
	return it
}

func mountRev(rs *crud.Resource[revItem]) chi.Router {
	r := chi.NewRouter()
	r.Mount("/items", rs)
	return r
}

func ocResource(db *den.DB) *crud.Resource[revItem] {
	return crud.NewResource[revItem](db, crud.WithOptimisticConcurrency[revItem]())
}

func TestConcurrencyGetEmitsETag(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rec := do(t, mountRev(ocResource(db)), http.MethodGet, "/items/"+it.ID, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("ETag"), "GET emits an ETag for a revisioned doc")
}

func TestConcurrencyNoETagWhenOptionOff(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	// Same revisioned type, but the resource didn't opt into concurrency.
	rs := crud.NewResource[revItem](db)
	rec := do(t, mountRev(rs), http.MethodGet, "/items/"+it.ID, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("ETag"), "no ETag without WithOptimisticConcurrency")
}

func TestConcurrencyPatchWithoutIfMatchIs428(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rec := do(t, mountRev(ocResource(db)), http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`, nil)
	assert.Equal(t, http.StatusPreconditionRequired, rec.Code)
	assert.Contains(t, rec.Body.String(), "precondition_required")
}

func TestConcurrencyPatchStaleIfMatchIs412(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rec := do(t, mountRev(ocResource(db)), http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`,
		map[string]string{"If-Match": `"deadbeef"`})
	assert.Equal(t, http.StatusPreconditionFailed, rec.Code)
	assert.Contains(t, rec.Body.String(), "precondition_failed")
}

func TestConcurrencyPatchMatchingIfMatchSucceeds(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)
	h := mountRev(ocResource(db))

	etag := do(t, h, http.MethodGet, "/items/"+it.ID, "", nil).Header().Get("ETag")
	require.NotEmpty(t, etag)

	rec := do(t, h, http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`,
		map[string]string{"If-Match": etag})
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var got revItem
	decode(t, rec, &got)
	assert.Equal(t, "b", got.Name)
	assert.NotEqual(t, etag, rec.Header().Get("ETag"), "update returns a fresh ETag")
}

func TestConcurrencyDeleteWithoutIfMatchIs428(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rec := do(t, mountRev(ocResource(db)), http.MethodDelete, "/items/"+it.ID, "", nil)
	assert.Equal(t, http.StatusPreconditionRequired, rec.Code)
}

func TestConcurrencyDeleteMatchingIfMatchSucceeds(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)
	h := mountRev(ocResource(db))

	etag := do(t, h, http.MethodGet, "/items/"+it.ID, "", nil).Header().Get("ETag")
	rec := do(t, h, http.MethodDelete, "/items/"+it.ID, "",
		map[string]string{"If-Match": etag})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// TestConcurrencyLostUpdatePrevented is the core scenario: two clients read the
// same version, the first write wins, the second (still holding the old ETag)
// is rejected instead of silently clobbering.
func TestConcurrencyLostUpdatePrevented(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)
	h := mountRev(ocResource(db))

	etag := do(t, h, http.MethodGet, "/items/"+it.ID, "", nil).Header().Get("ETag")

	first := do(t, h, http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`,
		map[string]string{"If-Match": etag})
	require.Equal(t, http.StatusOK, first.Code)

	second := do(t, h, http.MethodPatch, "/items/"+it.ID, `{"name":"c"}`,
		map[string]string{"If-Match": etag}) // stale now
	assert.Equal(t, http.StatusPreconditionFailed, second.Code)

	// The losing write didn't land.
	final := do(t, h, http.MethodGet, "/items/"+it.ID, "", nil)
	var got revItem
	decode(t, final, &got)
	assert.Equal(t, "b", got.Name)
}

// TestConcurrencyStarMatchesAnyExisting pins that If-Match: * succeeds against
// an existing row.
func TestConcurrencyStarMatchesAnyExisting(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rec := do(t, mountRev(ocResource(db)), http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`,
		map[string]string{"If-Match": "*"})
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}

// TestRevisionConflictWithoutOptionIs412 pins the defensive net: even a resource
// that did NOT opt into WithOptimisticConcurrency surfaces Den's revision
// conflict as a 412 rather than a 500, as long as the type is revisioned. The
// race between the handler's load and its save is forced deterministically by a
// competing write inside the update hook, after the handler loaded the row.
func TestRevisionConflictWithoutOptionIs412(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)

	rs := crud.NewResource[revItem](db, crud.WithUpdate(
		func(in revItem, dst *revItem, r *http.Request) error {
			dst.Name = in.Name
			// A competing writer bumps the stored _rev out from under the
			// in-flight document, so the handler's own Save is now stale.
			other, err := den.NewQuery[revItem](db, where.Field(den.FieldID).Eq(dst.ID)).First(r.Context())
			if err != nil {
				return err
			}
			other.Name = "concurrent"
			return den.Save(r.Context(), db, other)
		}))

	rec := do(t, mountRev(rs), http.MethodPatch, "/items/"+it.ID, `{"name":"b"}`, nil)
	assert.Equal(t, http.StatusPreconditionFailed, rec.Code,
		"a revision conflict is a 412, not a 500")
	assert.Contains(t, rec.Body.String(), "precondition_failed")
}
